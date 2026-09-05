# ESP32 與 Host 後端對接

本文件定義 Master Node 與 Host 後端的 MQTT 對接。Slave Node 不直接連
Cloudflare 或 MQTT Broker；它只將資料交給其 Master Node。

## 部署中的端點

目前正式環境使用下列端點。HTTPS enrollment 走 Dashboard 的同網域反向代理，
因此 ESP32 不需要依賴另一個 API hostname。

| 用途 | 正式端點 |
| --- | --- |
| Master enrollment | `https://hackathon.rabbitsayhello.me/v1/device/master-enrollments` |
| Slave enrollment／轉移 | `https://hackathon.rabbitsayhello.me/v1/device/slave-enrollments` |
| MQTT over WSS | `wss://mqtt.rabbitsayhello.me/` |
| Dashboard 驗證 API | `https://hackathon.rabbitsayhello.me/v1/dashboard/measurements` |

不要在 ESP32 使用 Docker service name、VPS IP、`1883` 或 `8883`。這些只供
VPS 內部服務使用。

## 1. 連線資訊

| 項目 | 值 |
| --- | --- |
| 對外協定 | MQTT over WebSocket Secure（WSS） |
| Hostname | `mqtt.rabbitsayhello.me`（其他環境替換為自己的 hostname） |
| Port | `443` |
| WebSocket path | `/` |
| MQTT version | MQTT 3.1.1 |
| Telemetry QoS | `1` |
| TLS | 必須驗證 Cloudflare 憑證 |

ESP32 不連到 VPS IP、`1883`、`8883` 或 Docker service name。它只能連到：

```text
wss://mqtt.rabbitsayhello.me/
```

Cloudflare Tunnel 將此 WSS 連線轉發至 VPS 內部的 Mosquitto WebSocket
listener（`http://mosquitto:9001`）。

## 2. Master 身分與帳密

每台 Master 使用自己的 MQTT 帳號與密碼；不可共用帳密。

- `master_id`：不可變的系統識別碼。
- `node_label`：可修改的顯示名稱，不能取代 `master_id`。
- 正式帳密只允許 publish 到此 Master 的 telemetry topic。

首次部署的 Master 必須先完成 provisioning，取得 `master_id` 與一次性
`enrollment_token`。它透過 HTTPS 呼叫：

```text
POST https://hackathon.rabbitsayhello.me/v1/device/master-enrollments
```

```json
{"master_id":"master-001","enrollment_token":"one-time-secret"}
```

成功回應只會一次性提供 `mqtt_username` 與 `mqtt_password`。Master 必須將
它們儲存在裝置設定區，並以 `master_id` 作為 MQTT client ID。不可讓 ESP32
使用共用正式 MQTT 帳密。

```json
{
  "master_id": "master-001",
  "mqtt_username": "master-001",
  "mqtt_password": "only-returned-once"
}
```

此請求成功時為 HTTP `201 Created`；一次性 enrollment token 使用過後不可重用。
正式部署前，由管理員先呼叫受 Cloudflare Access 保護的
`POST /v1/admin/master-enrollments`，再以安全方式將 token 交給該台 ESP32。

## 3. Telemetry topic

Master 僅能 publish 到：

```text
farm/v1/masters/{master_id}/telemetry
```

範例：

```text
farm/v1/masters/master-001/telemetry
```

每個 Master 每五分鐘送出一則批次。一般上限為每分鐘 12 批、每批最大
64 KiB。

## 4. Telemetry payload

`measured_at` 必須是 Master 彙整資料的 UTC RFC 3339 時間。後端自行產生
`received_at`，ESP32 不需傳送它。

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

欄位規則：

| 欄位 | 規則 |
| --- | --- |
| `message_id` | 每個 telemetry batch 唯一的 UUID；重送同一批時不得改變。 |
| `measured_at` | UTC RFC 3339；Master 需透過 NTP 校時。 |
| `firmware_version` | Master 與每台 Slave 都必填。 |
| `slave_id` | 不可變識別碼；必須已屬於 topic 中的 Master。 |
| `ph` | 必填，範圍 `0–14`。 |
| `ec_ms_per_cm` | 必填，單位 `mS/cm`，不可為負。 |
| `light_lux` | 必填，單位 lux，不可為負。 |
| `soil_moisture_percent` | 必填，校正後百分比，範圍 `0–100`。 |
| `calibration_version` | 必填；感測器重新校正後應更新。 |

數值必須先由 ESP 韌體校正。後端只驗證與保存，不會替 ESP32 校正 pH、EC
或土壤濕度。

## 5. 重連與離線補送

1. MQTT 使用 QoS 1。
2. 網路中斷時，Master 保存最多 24 小時的批次資料。
3. 重新連線後，依 `measured_at` 由舊到新補送。
4. 補送必須沿用原本的 `message_id`。
5. 後端以 `master_id + message_id` 去重，重送不會建立重複 batch。

Cloudflare 可能因網路維護中斷 WebSocket，因此 ESP32 必須實作自動重連與
MQTT keepalive。

## 6. Slave 與 Master 關係

- 每個 Slave 對應一盆盆栽。
- Slave 只經由 Master 上傳資料。
- Slave 初次建立或改綁新 Master 時，必須使用自己的 `transfer_token`。
- Slave 改綁成功後，舊 Master 不能再上傳該 Slave 的讀值。

Slave 初次連到 Master、或使用者要求改綁時，Master 以 HTTPS 代送：

```text
POST https://hackathon.rabbitsayhello.me/v1/device/slave-enrollments
```

```json
{
  "slave_id": "slave-001",
  "master_id": "master-001",
  "node_label": "Pot A",
  "transfer_token": "slave-owned-secret"
}
```

第一次呼叫會建立一盆預設盆栽；之後以同一 transfer token 呼叫並更換
`master_id`，即可改綁 Master。Host 只允許綁定已完成 enrollment 且未停用的
Master。

`transfer_token`、MQTT 密碼與 Wi-Fi 密碼均不可放在 telemetry payload、
Serial log 或 Git repository。

## 7. ESP32 Master 實作順序

1. 連上 Wi-Fi，透過 NTP 將系統時間校正為 UTC。
2. 若 NVS 尚無 MQTT 帳密，以 HTTPS `POST` 送出 `master_id` 與
   `enrollment_token`；只在 NVS 保存回應中的 MQTT 帳密，不輸出到 Serial log。
3. 使用 `wss://mqtt.rabbitsayhello.me/` 建立 MQTT 3.1.1 連線；設定
   client ID 和 username 都為 `master_id`，password 為 enrollment 回應的
   `mqtt_password`，並驗證 Cloudflare 的根憑證。
4. 每五分鐘彙整各 Slave 讀值，產生一個 UUID `message_id` 與 UTC RFC 3339
   `measured_at`，以 QoS 1 publish 到自己的 telemetry topic。
5. publish 失敗時先寫入本機佇列；重連後由舊到新補送，且不可更換原本的
   `message_id`。

推薦使用 ESP-IDF 的 `esp_http_client`（enrollment）與 `esp-mqtt`
（`MQTT_TRANSPORT_OVER_WSS`）。兩個 client 都必須指定 CA 憑證；測試階段也
不可關閉 TLS 憑證驗證。

### 最小流程偽碼

```text
connect_wifi()
sync_ntp()
credentials = load_credentials_from_nvs()
if credentials missing:
    credentials = POST enrollment_url({master_id, enrollment_token})
    save_credentials_to_nvs(credentials)

connect_mqtt_wss(url, client_id=master_id,
                 username=credentials.mqtt_username,
                 password=credentials.mqtt_password, ca_cert)
every 5 minutes:
    batch = build_batch(uuid_v4(), utc_rfc3339_now(), slave_readings)
    publish("farm/v1/masters/" + master_id + "/telemetry", batch, qos=1)
```

## 8. 管理命令

Host 對 Master 的設定更新使用另一組 command topic，並採非同步狀態：

```text
pending → applied | failed | expired
```

Master 必須為批量命令回報每個 Slave 的結果。命令 payload 會另行版本化；
ESP32 不應自行猜測或使用未公布的 topic。

## 9. ESP32 端檢查清單

- [ ] 使用 WSS，而非 raw TCP MQTT。
- [ ] 驗證 Cloudflare TLS 憑證。
- [ ] 使用唯一 Master MQTT 帳密。
- [ ] 每五分鐘發送一則 QoS 1 telemetry batch。
- [ ] 以 UTC RFC 3339 傳送 `measured_at`。
- [ ] 所有四種感測讀值與兩個版本欄位皆存在。
- [ ] 斷線時保存 24 小時資料並保持原 `message_id` 補送。
- [ ] 不在 log、payload 或原始碼中洩漏密碼、token。
