# ESP32 Master 與後端對接指南

本文件可獨立交給 ESP32 Master 韌體開發者使用。Master 負責向後端註冊、
保存專屬 MQTT 帳密，並彙整 Slave 感測值後上傳。Slave 不直接連線至網際網路
或 MQTT Broker。

## 1. 正式端點

| 用途 | 端點 |
| --- | --- |
| Master enrollment | `POST https://hackathon.rabbitsayhello.me/v1/device/master-enrollments` |
| Slave enrollment／轉移 | `POST https://hackathon.rabbitsayhello.me/v1/device/slave-enrollments` |
| MQTT telemetry | `wss://mqtt.rabbitsayhello.me/` |
| 寫入驗證 | `GET https://hackathon.rabbitsayhello.me/v1/dashboard/measurements` |

ESP32 不可使用 VPS IP、Docker service name、raw MQTT `1883` 或 `8883`。
MQTT 必須走 port `443` 的 MQTT over Secure WebSocket（WSS）。

## 2. 前置條件

管理員必須先為每台 Master 建立 provisioning，並安全交付下列資料：

- `master_id`：不可變的裝置 ID，例如 `master-001`。
- `enrollment_token`：一次性 token；不可寫入 Git、Serial log 或 telemetry。
- Cloudflare 根憑證：供 ESP32 驗證 HTTPS 和 WSS TLS 憑證。

韌體需要 Wi-Fi、NTP 校時、可持久化設定區（建議 NVS）及 UUID v4 產生器。

## 3. 第一次啟動：取得 MQTT 憑證

連上 Wi-Fi 並完成 NTP 校時後，若 NVS 尚未有 MQTT 憑證，發送：

```http
POST /v1/device/master-enrollments HTTP/1.1
Host: hackathon.rabbitsayhello.me
Content-Type: application/json

{"master_id":"master-001","enrollment_token":"一次性-token"}
```

成功時回傳 HTTP `201 Created`：

```json
{
  "master_id": "master-001",
  "mqtt_username": "master-001",
  "mqtt_password": "只會回傳這一次"
}
```

將 `mqtt_username`、`mqtt_password` 及 `master_id` 儲存在 NVS。token 用過即失效，
不得每次開機重新 enrollment。若 MQTT 密碼遺失，請由管理員重新 provision，而不是
在裝置端猜測或重試舊 token。

## 4. MQTT 連線設定

| MQTT 欄位 | 值 |
| --- | --- |
| Transport | WSS |
| Broker URL | `wss://mqtt.rabbitsayhello.me/` |
| MQTT 版本 | 3.1.1 |
| Client ID | `master_id` |
| Username | enrollment 回傳的 `mqtt_username`（等於 `master_id`） |
| Password | enrollment 回傳的 `mqtt_password` |
| TLS | 必須驗證 Cloudflare CA／憑證，不可設為 insecure |
| QoS | `1` |
| Keepalive | 建議 30 秒 |

Master 只具備下列 topic 的 publish 權限：

```text
farm/v1/masters/{master_id}/telemetry
```

例如 `master-001` 必須 publish 到：

```text
farm/v1/masters/master-001/telemetry
```

不可 publish 到其他 Master 的 topic，也不可使用 wildcard topic。

## 5. Telemetry payload

每五分鐘送出一個 JSON batch。`measured_at` 必須是 UTC RFC 3339；同一 batch
重送時必須保留原本的 `message_id`。

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

| 欄位 | 規則 |
| --- | --- |
| `message_id` | 每批唯一 UUID；重送不可變更。 |
| `measured_at` | UTC RFC 3339 時間。 |
| `firmware_version` | Master 與每個 reading 都必填。 |
| `slave_id` | 必須已註冊且綁定於此 Master。 |
| `ph` | `0` 至 `14`。 |
| `ec_ms_per_cm`、`light_lux` | 不可為負。 |
| `soil_moisture_percent` | `0` 至 `100`。 |
| `calibration_version` | 必填；重新校正感測器後更新。 |

後端以 `master_id + message_id` 去重；以 `(slave_id, measured_at)` 防止 reading
重複。數值驗證失敗、Slave 未註冊或 Slave 未綁定此 Master 時，資料不會進庫。

## 6. 韌體執行流程

```text
connect Wi-Fi
sync NTP time
load master_id and MQTT credentials from NVS
if MQTT credentials are absent:
    POST master enrollment over HTTPS
    save returned credentials to NVS

connect to MQTT broker with WSS and TLS verification
every 5 minutes:
    collect Slave readings
    create UUID message_id and UTC measured_at
    publish JSON batch with QoS 1
    if publish fails, enqueue batch locally

on reconnect:
    resend queued batches oldest first, retaining message_id
```

ESP-IDF 建議使用 `esp_http_client` 處理 enrollment，使用 `esp-mqtt` 的
`MQTT_TRANSPORT_OVER_WSS` 處理 MQTT。斷線時應採指數退避重連，並保存至少 24
小時的未送資料。

## 7. Slave 初次綁定

Master 代送 Slave 資料至 `POST /v1/device/slave-enrollments`：

```json
{
  "slave_id": "slave-001",
  "master_id": "master-001",
  "node_label": "Pot A",
  "transfer_token": "slave-own-secret"
}
```

每個 Slave 的 `transfer_token` 由 Slave 擁有。Master 只能轉送，不能把它放進
telemetry、log 或韌體原始碼。綁定成功後才能上傳該 Slave 的讀值。

## 8. 驗證清單

1. Master enrollment 回傳 HTTP `201` 與 MQTT 帳密。
2. WSS MQTT 連線成功，client ID 等於 `master_id`。
3. QoS 1 publish 到自己的 telemetry topic 成功。
4. 等待數秒後查詢 Dashboard API；回應中的 `plants[].readings[]` 應出現送出的
   `measured_at` 與感測值。
5. 重新送出完全相同的 batch；Dashboard 不應出現重複讀值。

若 Dashboard 回傳 `{"plants":[]}`，代表後端尚未收到有效 telemetry。優先檢查
Master 是否已完成 enrollment、MQTT client ID／topic 是否正確，以及每個 Slave
是否已綁定至該 Master。
