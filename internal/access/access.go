// Package access validates Cloudflare Access JWTs for administrative requests.
package access

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

const AssertionHeader = "Cf-Access-Jwt-Assertion"

type Authorizer struct {
	audience string
	issuer   string
	keysURL  string
	client   *http.Client
	mu       sync.RWMutex
	keys     map[string]*rsa.PublicKey
	expires  time.Time
}

type jwks struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	KeyID string `json:"kid"`
	Key   string `json:"kty"`
	N     string `json:"n"`
	E     string `json:"e"`
}

type tokenHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

type claims struct {
	Audience  json.RawMessage `json:"aud"`
	Issuer    string          `json:"iss"`
	Expires   int64           `json:"exp"`
	NotBefore int64           `json:"nbf"`
	Email     string          `json:"email"`
}

func New(teamDomain, audience string) (*Authorizer, error) {
	teamDomain = strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(teamDomain), "/"), "https://")
	if teamDomain == "" || strings.Contains(teamDomain, "/") || audience == "" {
		return nil, errors.New("Cloudflare Access configuration is incomplete")
	}
	return &Authorizer{
		audience: audience,
		issuer:   "https://" + teamDomain,
		keysURL:  "https://" + teamDomain + "/cdn-cgi/access/certs",
		client:   &http.Client{Timeout: 5 * time.Second},
	}, nil
}

func (a *Authorizer) Authorize(ctx context.Context, header string) (string, error) {
	if header == "" {
		return "", errors.New("Cloudflare Access assertion is required")
	}
	parts := strings.Split(header, ".")
	if len(parts) != 3 {
		return "", errors.New("invalid Cloudflare Access assertion")
	}
	var tokenHeader tokenHeader
	if err := decodePart(parts[0], &tokenHeader); err != nil || tokenHeader.Algorithm != "RS256" || tokenHeader.KeyID == "" {
		return "", errors.New("invalid Cloudflare Access assertion")
	}
	var tokenClaims claims
	if err := decodePart(parts[1], &tokenClaims); err != nil {
		return "", errors.New("invalid Cloudflare Access assertion")
	}
	if err := a.validateClaims(tokenClaims); err != nil {
		return "", err
	}
	key, err := a.key(ctx, tokenHeader.KeyID)
	if err != nil {
		return "", err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", errors.New("invalid Cloudflare Access assertion")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
		return "", errors.New("invalid Cloudflare Access assertion")
	}
	return tokenClaims.Email, nil
}

func (a *Authorizer) validateClaims(tokenClaims claims) error {
	now := time.Now().Unix()
	if tokenClaims.Issuer != a.issuer || tokenClaims.Expires <= now || tokenClaims.NotBefore > now || !hasAudience(tokenClaims.Audience, a.audience) {
		return errors.New("invalid Cloudflare Access assertion")
	}
	return nil
}

func (a *Authorizer) key(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	a.mu.RLock()
	key, valid := a.keys[keyID]
	fresh := time.Now().Before(a.expires)
	a.mu.RUnlock()
	if valid && fresh {
		return key, nil
	}
	if err := a.refresh(ctx); err != nil {
		return nil, err
	}
	a.mu.RLock()
	key = a.keys[keyID]
	a.mu.RUnlock()
	if key == nil {
		return nil, errors.New("Cloudflare Access signing key not found")
	}
	return key, nil
}

func (a *Authorizer) refresh(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.keysURL, nil)
	if err != nil {
		return fmt.Errorf("create Cloudflare Access key request: %w", err)
	}
	response, err := a.client.Do(request)
	if err != nil {
		return fmt.Errorf("fetch Cloudflare Access keys: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch Cloudflare Access keys: unexpected status %d", response.StatusCode)
	}
	var document jwks
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("decode Cloudflare Access keys: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(document.Keys))
	for _, item := range document.Keys {
		key, err := parseRSAKey(item)
		if err == nil {
			keys[item.KeyID] = key
		}
	}
	if len(keys) == 0 {
		return errors.New("Cloudflare Access returned no usable signing keys")
	}
	a.mu.Lock()
	a.keys = keys
	a.expires = time.Now().Add(time.Hour)
	a.mu.Unlock()
	return nil
}

func decodePart(value string, destination any) error {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(decoded, destination)
}

func hasAudience(raw json.RawMessage, audience string) bool {
	var values []string
	if err := json.Unmarshal(raw, &values); err == nil {
		for _, value := range values {
			if value == audience {
				return true
			}
		}
		return false
	}
	var value string
	return json.Unmarshal(raw, &value) == nil && value == audience
}

func parseRSAKey(value jwk) (*rsa.PublicKey, error) {
	if value.Key != "RSA" || value.KeyID == "" {
		return nil, errors.New("unsupported JWK")
	}
	n, err := base64.RawURLEncoding.DecodeString(value.N)
	if err != nil {
		return nil, err
	}
	e, err := base64.RawURLEncoding.DecodeString(value.E)
	if err != nil || len(e) == 0 || len(e) > 4 {
		return nil, errors.New("invalid RSA exponent")
	}
	exponent := 0
	for _, byteValue := range e {
		exponent = exponent<<8 | int(byteValue)
	}
	if exponent < 3 || exponent%2 == 0 {
		return nil, errors.New("invalid RSA exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}, nil
}
