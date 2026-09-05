# ESP32 與 Host 後端對接

本文件定義 Master Node 與 Host 後端的 MQTT 對接。Slave Node 不直接連
Cloudflare 或 MQTT Broker；它只將資料交給其 Master Node。

## 1. 連線資訊

| 項目 | 值 |
| --- | --- |
| 對外協定 | MQTT over WebSocket Secure（WSS） |
| Hostname | `mqtt.<你的網域>` |
| Port | `443` |
| WebSocket path | `/` |
| MQTT version | MQTT 3.1.1 |
| Telemetry QoS | `1` |
| TLS | 必須驗證 Cloudflare 憑證 |

ESP32 不連到 VPS IP、`1883`、`8883` 或 Docker service name。它只能連到：

```text
wss://mqtt.<你的網域>/
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
POST https://api.<你的網域>/v1/device/master-enrollments
```

```json
{"master_id":"master-001","enrollment_token":"one-time-secret"}
```

成功回應只會一次性提供 `mqtt_username` 與 `mqtt_password`。Master 必須將
它們儲存在裝置設定區，並以 `master_id` 作為 MQTT client ID。不可讓 ESP32
使用共用正式 MQTT 帳密。

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
POST https://api.<你的網域>/v1/device/slave-enrollments
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

## 7. 管理命令

Host 對 Master 的設定更新使用另一組 command topic，並採非同步狀態：

```text
pending → applied | failed | expired
```

Master 必須為批量命令回報每個 Slave 的結果。命令 payload 會另行版本化；
ESP32 不應自行猜測或使用未公布的 topic。

## 8. ESP32 端檢查清單

- [ ] 使用 WSS，而非 raw TCP MQTT。
- [ ] 驗證 Cloudflare TLS 憑證。
- [ ] 使用唯一 Master MQTT 帳密。
- [ ] 每五分鐘發送一則 QoS 1 telemetry batch。
- [ ] 以 UTC RFC 3339 傳送 `measured_at`。
- [ ] 所有四種感測讀值與兩個版本欄位皆存在。
- [ ] 斷線時保存 24 小時資料並保持原 `message_id` 補送。
- [ ] 不在 log、payload 或原始碼中洩漏密碼、token。
