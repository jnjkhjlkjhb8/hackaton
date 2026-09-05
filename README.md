# Farm Host Backend

Go 後端，負責接收 ESP32 Master Node 經 MQTT over WSS 上傳的盆栽感測資料，並保存至 PostgreSQL。

Dashboard 前端位於 `frontend/`，目前以 fixture 資料提供全域趨勢判讀；AI、OTA 與遠端 Wi-Fi 設定不在本階段範圍。

## 架構

```text
Slave Nodes
    │ local forwarding
    ▼
Master Node ── MQTT over WSS ── Cloudflare Tunnel ── Mosquitto
                                                         │
                                                         ▼
                                                    Go Host ── PostgreSQL
```

- Master 每五分鐘傳送一則 telemetry 批次。
- 每筆 Slave 讀值包含 pH、EC（`mS/cm`）、光照（lux）、土壤濕度（%）、
  `calibration_version` 與 `firmware_version`。
- MQTT 使用 QoS 1；資料庫以 Master 與 `message_id` 去重。
- Master、Slave 與盆栽關係由資料庫管理，未註冊或已停用的 Slave 無法寫入資料。

完整領域用語請見 [CONTEXT.md](./CONTEXT.md)，裝置身分決策請見
[ADR 0001](./docs/adr/0001-mqtt-device-identity.md)，ESP32 Master 實作請見
[ESP32 Master 與後端對接指南](./docs/ESP32_MASTER_BACKEND_INTEGRATION.md)。

## 需求

- Go 1.25+
- PostgreSQL（可由既有 `DATABASE_URL` 連線）
- Docker Compose（部署 Mosquitto 與 Cloudflare Tunnel 時需要）
- 已建立的 Cloudflare Tunnel 與兩個公開 hostname：
  - `mqtt.example.com` → `http://mosquitto:9001`
  - `api.example.com` → `http://host:8080`
  - `hackathon.example.com` → `http://frontend:80`

Cloudflare 必須允許 WebSocket。外部 ESP32 使用 `wss://mqtt.example.com`；
VPS 內的 Go 服務使用 `tcp://mosquitto:1883`。

## 設定

複製範本後，僅在 VPS 上填入真實值：

```sh
cp .env.example .env
```

Go 服務啟動時會自動讀取專案根目錄的 `.env`；若檔案不存在，則改讀 VPS
提供的系統環境變數，方便 Docker 與正式部署使用。

| 變數 | 用途 |
| --- | --- |
| `DATABASE_URL` | PostgreSQL connection string |
| `MQTT_BROKER_URL` | Go 服務連線至 Broker 的 URL，例如 `tcp://mosquitto:1883` |
| `MQTT_USERNAME` | Go telemetry consumer 的 Dynamic Security 帳號 |
| `MQTT_PASSWORD` | Go telemetry consumer 的密碼 |
| `DYNSEC_MQTT_*` | 專供 Host 自動建立 Master MQTT 身分的 Dynamic Security 管理帳號 |
| `DYNSEC_CA_FILE` | Host 容器內驗證 `mosquitto:8883` 的 CA 憑證位置 |
| `CLOUDFLARE_ACCESS_TEAM_DOMAIN` | Cloudflare Access team domain，例如 `team.cloudflareaccess.com` |
| `CLOUDFLARE_ACCESS_AUDIENCE` | 管理 API Access application 的 AUD tag |
| `CLOUDFLARE_TUNNEL_TOKEN` | Cloudflare Tunnel token |
| `HTTP_ADDR` | 選用，預設 `:8080` |

`.env` 已被 `.gitignore` 排除，請勿提交 connection string、Tunnel token 或 MQTT 密碼。

## 資料庫 migration

先對既有 PostgreSQL 執行 migration：

```sh
psql "$DATABASE_URL" -f migrations/000001_initial.up.sql
```

此 migration 會啟用 `pgcrypto` 擴充，並建立 Master、Slave、盆栽、
telemetry batch 與 measurement 資料表。

## Mosquitto Dynamic Security

`deploy/mosquitto/mosquitto.conf` 使用 Dynamic Security Plugin。首次啟動前，
需在 Mosquitto 資料目錄初始化 `dynamic-security.json`，再以
`mosquitto_ctrl dynsec` 建立 Go consumer 帳號及其訂閱權限。

先執行 `sh scripts/init-mosquitto-tls.sh` 建立只供容器內 Dynamic Security
管理用的 TLS listener 憑證。管理時請連至 `mosquitto:8883` 並指定
`--cafile /mosquitto/certs/ca.crt`；不要使用未加密的 `1883` 執行
`mosquitto_ctrl`。

裝置帳密與 `farm/v1/masters/{master_id}/telemetry` 的 publish ACL 應由
Dynamic Security 管理；不可使用匿名或共用正式裝置帳密。

## 裝置 enrollment

Host 現在提供三個 API。管理員先透過 Cloudflare Access 呼叫 Master 預建 API；
Host 產生一次性 token，資料庫只保存其 SHA-256 雜湊。Master 首次以 token
呼叫裝置 API 時，Host 會自動建立或重設該 Master 的 Dynamic Security client、
限制它的 MQTT client ID 與 telemetry topic，並只在 API 回應中回傳一次 MQTT
密碼。Slave 不直接持有 MQTT 帳密；它以自己的 transfer token 建立或改綁 Master。

| Method / path | Access | 用途 |
| --- | --- | --- |
| `POST /v1/admin/master-enrollments` | Cloudflare Access JWT | 預建 Master 並取得一次性 enrollment token |
| `POST /v1/device/master-enrollments` | enrollment token | 取得一次性的 Master MQTT credentials |
| `POST /v1/device/slave-enrollments` | Slave transfer token | 初次建立 Slave 或改綁至已註冊 Master |

Cloudflare Access 必須只保護 `/v1/admin/*`；裝置 enrollment endpoint 需要讓
ESP32 經 HTTPS 存取。首次部署仍需要人工建立一個專用的
`host-dynsec-manager` Dynamic Security 管理帳號，並將其帳密填入
`DYNSEC_MQTT_USERNAME` 與 `DYNSEC_MQTT_PASSWORD`。這是建立 Dynamic
Security 控制權本身所需的一次性 bootstrap，不是裝置的共用帳密。

Master enrollment 請求：

```json
{"master_id":"master-001","enrollment_token":"one-time-secret"}
```

Slave enrollment 或轉移請求：

```json
{"slave_id":"slave-001","master_id":"master-001","node_label":"Pot A","transfer_token":"slave-owned-secret"}
```

Master 的 MQTT username 與 client ID 都必須等於 `master_id`，且只可發布：

```text
farm/v1/masters/{master_id}/telemetry
```

## 啟動

```sh
docker compose up --build -d
```

健康檢查：

```sh
curl http://127.0.0.1:8080/healthz
```

預期回應：

```json
{"status":"healthy"}
```

## Telemetry topic 與 payload

Master 只能 publish 至：

```text
farm/v1/masters/{master_id}/telemetry
```

範例：

```json
{
  "message_id": "e17e9b29-1d03-4b85-a166-c204a2466a09",
  "measured_at": "2026-09-05T13:30:00Z",
  "firmware_version": "master-1.0.0",
  "readings": [
    {
      "slave_id": "slave-001",
      "ph": 6.4,
      "ec_ms_per_cm": 1.2,
      "light_lux": 850,
      "soil_moisture_percent": 42.5,
      "calibration_version": "2026-09-01",
      "firmware_version": "slave-1.0.0"
    }
  ]
}
```

後端拒絕 pH 不在 `0–14`、負 EC／光照、或土壤濕度不在 `0–100` 的 reading。

## 開發與驗證

```sh
gofmt -w cmd internal
go test ./...
go vet ./...
```

## Dashboard 前端

`frontend/` 是以 React、Vite 與 Uber Base Web（`baseui`）建立的 SaaS 前端。它包含全域趨勢總覽、盆栽、裝置、活動紀錄與設定工作區；總覽預設比較最近 24 小時的土壤濕度，並可切換指標、時間範圍、可見盆栽與淺／深主題。fixture 資料刻意獨立於 UI，以便後續接上 telemetry 查詢 API。

```sh
cd frontend
npm install
npm run dev
```

正式驗證 production bundle：

```sh
cd frontend
npm run build
```

所有 Go 程式碼遵循
[`/Users/jnjkhjlkjhb8/Projects/bus/docs/go-style/`](../bus/docs/go-style/README.md)
的規範。

## Dashboard data API

`GET /v1/dashboard/measurements` 回傳最近 30 天、依盆栽分組的 telemetry
讀值。Dashboard 會在載入時使用此 API；若 Host 尚未可用或沒有資料，才會保留
fixture 畫面。

本機開發時，Vite 預設將 `/v1` 代理到 `http://127.0.0.1:8080`。若 Host
使用不同位址，可指定：

```sh
cd frontend
VITE_API_PROXY_TARGET=http://127.0.0.1:8080 npm run dev
```
