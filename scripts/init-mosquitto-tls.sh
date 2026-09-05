#!/usr/bin/env sh
set -eu

cert_dir="deploy/mosquitto/certs"

if [ -e "$cert_dir/server.crt" ] || [ -e "$cert_dir/server.key" ]; then
  echo "refusing to overwrite existing Mosquitto TLS files" >&2
  exit 1
fi

mkdir -p "$cert_dir"
chmod 700 "$cert_dir"

openssl req -x509 -newkey rsa:4096 -nodes -days 3650 \
  -keyout "$cert_dir/ca.key" \
  -out "$cert_dir/ca.crt" \
  -subj "/CN=Farm Host Mosquitto CA"

openssl req -newkey rsa:2048 -nodes \
  -keyout "$cert_dir/server.key" \
  -out "$cert_dir/server.csr" \
  -subj "/CN=mosquitto"

printf 'subjectAltName=DNS:mosquitto,DNS:localhost\n' > "$cert_dir/server.ext"
openssl x509 -req -days 3650 \
  -in "$cert_dir/server.csr" \
  -CA "$cert_dir/ca.crt" \
  -CAkey "$cert_dir/ca.key" \
  -CAcreateserial \
  -out "$cert_dir/server.crt" \
  -extfile "$cert_dir/server.ext"

rm "$cert_dir/server.csr" "$cert_dir/server.ext" "$cert_dir/ca.srl"
chmod 600 "$cert_dir/ca.key" "$cert_dir/server.key"
chmod 644 "$cert_dir/ca.crt" "$cert_dir/server.crt"

echo "created TLS files in $cert_dir"
