# 農業感測後端

本 context 定義盆栽感測資料的蒐集與保存。它讓無公網 IP 的部署環境能安全接收 ESP32 感測節點的資料。

## Language

**無公網 IP VPS**:
可主動連上網際網路、但無法由網際網路直接連入的 VPS。
_Avoid_: 沒有外網、離線 VPS

**Host**:
本系統中運行 MQTT Broker、Go 後端與 PostgreSQL 的無公網 IP VPS。
_Avoid_: 現場中繼器、Raspberry Pi

**MQTT over WSS**:
ESP32 透過 WebSocket Secure 傳送 MQTT 訊息的連線方式，可由 Cloudflare Tunnel 安全轉送至 VPS。
_Avoid_: 直接 MQTT TCP

**感測節點**:
固定對應一盆盆栽的 ESP32 裝置；其身分以不可變的 `device_id` 識別。
_Avoid_: 主機、從機、裝置群組

**盆栽**:
第一階段中由單一感測節點監測的獨立種植單位。
_Avoid_: 節點、感測器

**感測讀值**:
一盆盆栽在某一時刻由 ESP 韌體校正後的酸鹼值（pH）、導電度（EC）、光照與土壤濕度；四項皆為必填，並帶有 `calibration_version` 與 `firmware_version`。
_Avoid_: 節點資料、環境資料

**EC**:
感測讀值中的導電度，固定以 `mS/cm` 表示。
_Avoid_: 無單位 EC、µS/cm

**土壤濕度**:
經感測器校正後的土壤含水程度，固定以 `0–100` 的百分比表示。
_Avoid_: ADC 原始值、無單位濕度

**Master Node**:
彙整多個 Slave Node 的感測讀值，並將它們轉送至 VPS 的 ESP32 閘道；它不對應盆栽。
_Avoid_: 感測節點、盆栽節點

**Slave Node**:
固定對應一盆盆栽、採集感測讀值並交給 Master Node 的 ESP32 感測節點。
_Avoid_: 主節點、閘道

**MQTT Broker**:
部署於 VPS 的訊息服務，接收 Master Node 的 MQTT 訊息並提供 Go 後端訂閱；它不是業務資料的永久儲存位置。
_Avoid_: Go 後端、資料庫

**採樣時間**:
感測讀值在 Master Node 被採集或彙整的 UTC 時刻，以 `measured_at` 表示。
_Avoid_: 上傳時間、資料庫時間

**接收時間**:
Go 後端接收 MQTT 訊息的 UTC 時刻，以 `received_at` 表示。
_Avoid_: 採樣時間、感測時間

**上報週期**:
Slave Node 產生讀值、並由 Master Node 轉送至後端的固定五分鐘間隔。
_Avoid_: 即時串流、不固定輪詢

**Telemetry 批次**:
Master Node 在一則最多 64 KiB、每分鐘最多 12 批的 MQTT 訊息中彙整上報多筆 Slave Node 感測讀值，以 `message_id` 識別與去重。
_Avoid_: 每個 Slave 各自對 Host 發送 MQTT、無識別的訊息

**離線緩衝**:
Master Node 無法連至 Host 時在本機保存最多 24 小時讀值，連線恢復後按採樣時間補送的機制。
_Avoid_: 離線即丟棄、無上限本機保存

**在線狀態**:
Master Node 由 MQTT Last Will 或 15 分鐘未更新的 `last_seen_at` 判定為離線；Slave Node 依 Master Node 最後回報它的時間判定。
_Avoid_: 僅靠網頁連線、無逾時判定

**無效讀值**:
不符合感測讀值格式或物理範圍的單筆資料；Host 拒絕該筆資料、記錄原因，但仍處理同批次其他有效資料。
_Avoid_: 寫入後修正、拒絕整個批次

**原始讀值保存期限**:
五分鐘粒度感測讀值在 Host 保留 12 個月的期間。
_Avoid_: 無期限保存、未定義保存期限

**每日彙總**:
原始讀值保存期限結束後永久保留的每日最小、最大與平均感測值，以 Asia/Taipei 日曆日切分，用於長期趨勢。
_Avoid_: 永遠保留原始讀值、完全刪除歷史資料

**QoS 1**:
MQTT 訊息的至少送達一次交付方式；同一筆感測讀值可能被重送，後端必須去重。
_Avoid_: 至多送達一次、恰好送達一次

**Master 憑證**:
僅供一個 Master Node 使用的 MQTT 帳號與密碼；其 publish 權限只限於該 Master 的 topic。
_Avoid_: 共用 MQTT 密碼、全域裝置帳號

**憑證輪替**:
由管理者建立新的 Master 憑證並撤銷舊憑證的操作，用於設備遺失、停用或疑似外洩。
_Avoid_: 共用密碼、永久有效憑證

**節點歸屬**:
已註冊的 Master Node、Slave Node 與盆栽之間的對應關係；後端只接受符合此關係的讀值。
_Avoid_: 任意 slave_id、未驗證節點

**節點名稱**:
使用者可修改的 `node_label`，僅供辨識 Master Node 或 Slave Node；它不參與 MQTT 權限或資料關聯。
_Avoid_: master_id、slave_id、裝置身分

**停用**:
撤銷 Master Node 或 Slave Node 的後續上報與管理權限、但保留其盆栽與歷史資料的狀態。
_Avoid_: 實體刪除、清除歷史資料

**設定命令**:
Host 建立、由 Master Node 轉送或套用的非同步設定變更；它有有效期限，並依序處於 `pending`、`applied`、`failed` 或 `expired` 狀態。批量命令必須個別回報每個 Slave Node 的結果。
_Avoid_: 同步設定、即時成功保證

**管理者**:
經 Cloudflare Access 驗證、可呼叫 Host 管理 API 的使用者或服務身分。
_Avoid_: 匿名使用者、MQTT 裝置帳號

**審計紀錄**:
不可修改的管理操作歷程，記錄管理者、時間、目標、操作與結果。
_Avoid_: 可覆寫日誌、未記錄的管理操作

**Slave 轉移碼**:
由 Slave Node 持有的高熵祕密，用於授權將該 Slave Node 歸屬改為新的 Master Node。
_Avoid_: Master 憑證、共用 channel key

**自動註冊**:
Master Node 首次使用受限 bootstrap 憑證與一次性註冊碼建立身分，成功後取得專屬 MQTT 憑證的流程。
_Avoid_: 無條件註冊、共用正式帳密

**Slave 自動建立**:
已註冊 Master Node 首次提供有效 `slave_id` 與 Slave 轉移碼時，Host 建立該 Slave Node、預設盆栽與節點歸屬的流程。
_Avoid_: 未驗證自動建立、手動預建節點

**一次性註冊碼**:
在部署前與 `master_id` 一起寫入 Master Node 的高熵祕密；後端只保存其雜湊並僅接受一次驗證。
_Avoid_: 明文資料庫密碼、可重複使用的註冊碼
