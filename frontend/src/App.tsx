import { useEffect, useMemo, useState, type ComponentType } from "react";
import { Button, KIND, SIZE } from "baseui/button";
import { Card } from "baseui/card";
import { Input } from "baseui/input";
import { Select } from "baseui/select";

type Metric = "moisture" | "ph" | "ec" | "light";
type Range = "24h" | "7d" | "30d";
type Route = "overview" | "plants" | "devices" | "activity" | "settings";
type Option = { id: string; label: string };
type Plant = {
  id: string;
  label: string;
  zone: string;
  attention: boolean;
  pinned: boolean;
  lastSeen: string;
  series: Record<Metric, number[]>;
};
type DashboardReading = {
  measured_at: string;
  ph: number;
  ec_ms_per_cm: number;
  light_lux: number;
  soil_moisture_percent: number;
};
type DashboardResponse = {
  plants: Array<{ id: string; label: string; readings: DashboardReading[] }>;
};

type LegacyInputProps = {
  id?: string;
  value?: string;
  placeholder?: string;
  "aria-label"?: string;
  onChange?: (event: { currentTarget: { value: string } }) => void;
};

const BaseInput = Input as unknown as ComponentType<LegacyInputProps>;

const metrics: Array<{
  id: Metric;
  label: string;
  unit: string;
  precision: number;
}> = [
  { id: "moisture", label: "土壤含水率", unit: "%", precision: 1 },
  { id: "ph", label: "pH", unit: "pH", precision: 1 },
  { id: "ec", label: "EC", unit: "mS/cm", precision: 2 },
  { id: "light", label: "光照", unit: "lux", precision: 0 },
];
const navigation: Array<{ id: Route; label: string; hint: string }> = [
  { id: "overview", label: "總覽", hint: "全域趨勢" },
  { id: "plants", label: "盆栽", hint: "6 個受監測單位" },
  { id: "devices", label: "裝置", hint: "2 個閘道" },
  { id: "activity", label: "活動紀錄", hint: "系統與人工操作" },
  { id: "settings", label: "設定", hint: "通知與資料偏好" },
];
const seriesColors = [
  "#276EF1",
  "#00A86B",
  "#E11900",
  "#8B5CF6",
  "#C66A00",
  "#007C91",
];
const rangeHours: Record<Range, number> = {
  "24h": 24,
  "7d": 24 * 7,
  "30d": 24 * 30,
};

const wave = (index: number, period: number, offset: number) =>
  Math.sin((index + offset) / period);
function createSeries(
  seed: number,
  base: Record<Metric, number>,
): Record<Metric, number[]> {
  const length = rangeHours["30d"] + 1;
  return {
    moisture: Array.from({ length }, (_, i) =>
      Number(
        (
          base.moisture -
          i * 0.018 +
          wave(i, 5.7, seed) * 1.8 +
          wave(i, 17, seed) * 1.2
        ).toFixed(1),
      ),
    ),
    ph: Array.from({ length }, (_, i) =>
      Number(
        (base.ph + wave(i, 12, seed) * 0.13 + wave(i, 49, seed) * 0.08).toFixed(
          2,
        ),
      ),
    ),
    ec: Array.from({ length }, (_, i) =>
      Number(
        (
          base.ec +
          wave(i, 8.5, seed) * 0.06 +
          wave(i, 37, seed) * 0.04
        ).toFixed(2),
      ),
    ),
    light: Array.from({ length }, (_, i) =>
      Math.round(
        base.light * Math.max(0, Math.sin((((i % 24) - 6) / 12) * Math.PI)) +
          Math.max(0, wave(i, 3.8, seed)) * 95,
      ),
    ),
  };
}
const fixturePlants: Plant[] = [
  {
    id: "pothos",
    label: "黃金葛",
    zone: "東窗",
    attention: true,
    pinned: true,
    lastSeen: "剛才",
    series: createSeries(1, { moisture: 44, ph: 6.3, ec: 1.12, light: 980 }),
  },
  {
    id: "ficus",
    label: "琴葉榕",
    zone: "南側",
    attention: false,
    pinned: true,
    lastSeen: "剛才",
    series: createSeries(4, { moisture: 58, ph: 6.6, ec: 1.34, light: 1420 }),
  },
  {
    id: "monstera",
    label: "龜背芋",
    zone: "北窗",
    attention: true,
    pinned: true,
    lastSeen: "5 分鐘前",
    series: createSeries(7, { moisture: 36, ph: 6.1, ec: 0.92, light: 680 }),
  },
  {
    id: "fern",
    label: "波士頓腎蕨",
    zone: "西側",
    attention: true,
    pinned: false,
    lastSeen: "5 分鐘前",
    series: createSeries(10, { moisture: 31, ph: 6.5, ec: 1.08, light: 760 }),
  },
  {
    id: "calathea",
    label: "竹芋",
    zone: "書房",
    attention: false,
    pinned: false,
    lastSeen: "剛才",
    series: createSeries(13, { moisture: 63, ph: 6.4, ec: 1.22, light: 540 }),
  },
  {
    id: "herbs",
    label: "香草盆",
    zone: "陽台",
    attention: false,
    pinned: false,
    lastSeen: "剛才",
    series: createSeries(16, { moisture: 49, ph: 6.8, ec: 1.46, light: 1670 }),
  },
];

function plantFromDashboard(value: DashboardResponse["plants"][number]): Plant {
  const readings = value.readings;
  return {
    id: value.id,
    label: value.label,
    zone: "已註冊裝置",
    attention: false,
    pinned: true,
    lastSeen: readings.length
      ? new Date(readings.at(-1)!.measured_at).toLocaleString("zh-TW")
      : "尚無讀值",
    series: {
      moisture: readings.map((reading) => reading.soil_moisture_percent),
      ph: readings.map((reading) => reading.ph),
      ec: readings.map((reading) => reading.ec_ms_per_cm),
      light: readings.map((reading) => reading.light_lux),
    },
  };
}

const selectPoints = (series: number[], range: Range) =>
  series.slice(-(rangeHours[range] + 1));
const formatValue = (value: number, metric: (typeof metrics)[number]) =>
  `${value.toFixed(metric.precision)} ${metric.unit}`;
function pathFor(values: number[], min: number, max: number) {
  return values
    .map(
      (value, index) =>
        `${index ? "L" : "M"}${((index / Math.max(values.length - 1, 1)) * 1000).toFixed(2)},${(360 - ((value - min) / (max - min || 1)) * 360).toFixed(2)}`,
    )
    .join(" ");
}
function timeLabel(range: Range, position: "start" | "mid" | "end") {
  const date = new Date();
  const h = rangeHours[range];
  date.setHours(
    date.getHours() -
      (position === "start" ? h : position === "mid" ? h / 2 : 0),
  );
  return h <= 24
    ? date.toLocaleTimeString("zh-TW", { hour: "2-digit", minute: "2-digit" })
    : date.toLocaleDateString("zh-TW", { month: "numeric", day: "numeric" });
}
const navIcons: Record<Route, string> = {
  overview: "home",
  plants: "eco",
  devices: "router",
  activity: "history",
  settings: "settings",
};
const statusIcons = {
  healthy: "data_usage",
  attention: "warning",
  offline: "warning",
  neutral: "data_usage",
};
function Icon({ name }: { name: string }) {
  return (
    <span className="material-symbols-outlined" aria-hidden="true">
      {name}
    </span>
  );
}
function Status({
  tone,
  children,
}: {
  tone: "healthy" | "attention" | "offline" | "neutral";
  children: string;
}) {
  return (
    <span className={`status status--${tone}`}>
      <Icon name={statusIcons[tone]} />
      {children}
    </span>
  );
}
function PageHeading({
  title,
  description,
  actions,
}: {
  title: string;
  description: string;
  actions?: React.ReactNode;
}) {
  return (
    <header className="page-heading">
      <div>
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      {actions ? <div className="page-heading__actions">{actions}</div> : null}
    </header>
  );
}

function TrendChart({
  visible,
  metric,
  range,
}: {
  visible: Plant[];
  metric: Metric;
  range: Range;
}) {
  const metricMeta = metrics.find((item) => item.id === metric)!;
  const values = visible.flatMap((plant) =>
    selectPoints(plant.series[metric], range),
  );
  const lowest = Math.min(...values);
  const highest = Math.max(...values);
  const padding = (highest - lowest || 1) * 0.12;
  const min = lowest - padding;
  const max = highest + padding;
  const ticks = Array.from(
    { length: 4 },
    (_, i) => max - ((max - min) * i) / 3,
  );
  return (
    <figure className="trend-figure" aria-labelledby="chart-title">
      <div
        className="trend-figure__plot"
        role="img"
        aria-label={`${metricMeta.label}，${range}，${visible.map((plant) => plant.label).join("、")}`}
      >
        <div className="trend-figure__axis" aria-hidden="true">
          {ticks.map((tick) => (
            <span key={tick}>{formatValue(tick, metricMeta)}</span>
          ))}
        </div>
        <svg
          className="trend-figure__svg"
          viewBox="0 0 1000 360"
          preserveAspectRatio="none"
          aria-hidden="true"
        >
          {[0, 1, 2, 3].map((line) => (
            <line
              key={line}
              x1="0"
              x2="1000"
              y1={line * 120}
              y2={line * 120}
              className="trend-figure__gridline"
            />
          ))}
          {visible.map((plant, index) => (
            <path
              key={plant.id}
              d={pathFor(selectPoints(plant.series[metric], range), min, max)}
              className="trend-figure__line"
              style={{ stroke: seriesColors[index % seriesColors.length] }}
            />
          ))}
        </svg>
      </div>
      <figcaption className="trend-figure__caption">
        <span>{timeLabel(range, "start")}</span>
        <span>{timeLabel(range, "mid")}</span>
        <span>{timeLabel(range, "end")}</span>
      </figcaption>
    </figure>
  );
}

function Overview({ plants, isLive }: { plants: Plant[]; isLive: boolean }) {
  const [metric, setMetric] = useState<Metric>("moisture");
  const [range, setRange] = useState<Range>("24h");
  const initial = useMemo(
    () =>
      plants
        .filter((plant) => plant.attention || plant.pinned)
        .map((plant) => ({ id: plant.id, label: plant.label })),
    [plants],
  );
  const [selected, setSelected] = useState<Option[]>(initial);
  useEffect(() => setSelected(initial), [initial]);
  const selectedMetric = metrics.find((item) => item.id === metric)!;
  const visible = useMemo(() => {
    const ids = new Set(selected.map((plant) => plant.id));
    return plants.filter((plant) => ids.has(plant.id));
  }, [selected]);
  const readings = visible.map((plant) => ({
    plant,
    value: selectPoints(plant.series[metric], range).at(-1)!,
  }));
  const low = readings.length
    ? readings.reduce((current, item) =>
        item.value < current.value ? item : current,
      )
    : undefined;
  return (
    <>
      <PageHeading
        title="總覽"
        description="比較多盆植物的感測趨勢，先看變化，再決定是否處理。"
      />
      <p className="fixture-note" role="status">
        <strong>{isLive ? "即時 telemetry 資料" : "Fixture 資料"}</strong> ·
        {isLive ? " 已從 Host 載入最近 30 天讀值。" : " 尚未連上 Host，顯示示意資料。"}
      </p>
      <section className="dashboard-toolbar" aria-label="趨勢篩選條件">
        <div className="dashboard-control">
          <label htmlFor="metric-select">指標</label>
          <Select
            id="metric-select"
            clearable={false}
            searchable={false}
            options={metrics.map(({ id, label }) => ({ id, label }))}
            value={[{ id: metric, label: selectedMetric.label }]}
            onChange={({ value }) => {
              const next = value[0]?.id;
              if (typeof next === "string") setMetric(next as Metric);
            }}
          />
        </div>
        <div className="dashboard-control">
          <label htmlFor="plant-select">顯示盆栽</label>
          <Select
            id="plant-select"
            multi
            clearable={false}
            options={plants.map((plant) => ({
              id: plant.id,
              label: `${plant.label} · ${plant.zone}`,
            }))}
            value={selected}
            onChange={({ value }) => setSelected(value as Option[])}
            placeholder="選擇盆栽"
          />
        </div>
        <div className="dashboard-control dashboard-control--range">
          <span className="dashboard-control__label">範圍</span>
          <div className="range-switcher" aria-label="資料時間範圍">
            {(["24h", "7d", "30d"] as Range[]).map((item) => (
              <Button
                key={item}
                size={SIZE.compact}
                kind={range === item ? KIND.secondary : KIND.tertiary}
                onClick={() => setRange(item)}
                aria-pressed={range === item}
              >
                {item === "24h" ? "24 小時" : item === "7d" ? "7 天" : "30 天"}
              </Button>
            ))}
          </div>
        </div>
      </section>
      <div className="overview-layout">
        <section className="trend-card">
          <Card>
            {visible.length ? (
              <div className="chart-panel">
                <div className="chart-panel__heading">
                  <div>
                    <h2 id="chart-title">{selectedMetric.label}</h2>
                    <p>
                      {range === "24h"
                        ? "最近 24 小時"
                        : range === "7d"
                          ? "最近 7 天"
                          : "最近 30 天"}{" "}
                      · {selectedMetric.unit}
                    </p>
                  </div>
                  <p className="chart-panel__reading-summary">
                    {low
                      ? `${low.plant.label} 目前最低：${formatValue(low.value, selectedMetric)}`
                      : ""}
                  </p>
                </div>
                <TrendChart visible={visible} metric={metric} range={range} />
                <div className="chart-legend" aria-label="目前圖表中的盆栽">
                  {readings.map(({ plant, value }, index) => (
                    <div className="chart-legend__item" key={plant.id}>
                      <span
                        className="chart-legend__swatch"
                        style={{
                          backgroundColor:
                            seriesColors[index % seriesColors.length],
                        }}
                        aria-hidden="true"
                      />
                      <span>{plant.label}</span>
                      <strong>{formatValue(value, selectedMetric)}</strong>
                    </div>
                  ))}
                </div>
              </div>
            ) : (
              <div className="empty-state">
                <div>
                  <h2>尚未選擇盆栽</h2>
                  <p>選擇至少一盆，即可比較 {selectedMetric.label} 趨勢。</p>
                </div>
                <Button onClick={() => setSelected(initial)}>
                  <Icon name="warning" />
                  顯示需注意的盆栽
                </Button>
              </div>
            )}
          </Card>
        </section>
        <aside className="attention-panel" aria-labelledby="attention-title">
          <div className="attention-panel__heading">
            <h2 id="attention-title">需要注意</h2>
            <span>3</span>
          </div>
          {plants
            .filter((plant) => plant.attention)
            .map((plant) => (
              <div className="attention-item" key={plant.id}>
                <div>
                  <strong>{plant.label}</strong>
                  <p>{plant.zone} · 土壤含水率低於最近範圍</p>
                </div>
                <Status tone="attention">待檢查</Status>
              </div>
            ))}
        </aside>
      </div>
    </>
  );
}

function PlantsPage({ plants }: { plants: Plant[] }) {
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const matching = plants.filter((plant) =>
    `${plant.label}${plant.zone}`.includes(query.trim()),
  );
  const moisture = metrics[0];
  return (
    <>
      <PageHeading
        title="盆栽"
        description="管理受監測的盆栽、位置與目前讀值。"
        actions={
          <Button onClick={() => setOpen((current) => !current)}>
            <Icon name="add" />
            {open ? "收合新增" : "新增盆栽"}
          </Button>
        }
      />
      {open ? (
        <section className="inline-form" aria-label="新增盆栽">
          <BaseInput placeholder="盆栽名稱" />
          <BaseInput placeholder="位置" />
          <Button size={SIZE.compact}>儲存草稿</Button>
          <p>這是前端流程示意，尚未傳送到後端。</p>
        </section>
      ) : null}
      <section className="table-toolbar">
        <div className="table-search">
          <label htmlFor="plant-search">搜尋盆栽</label>
          <BaseInput
            id="plant-search"
            value={query}
            onChange={(event) => setQuery(event.currentTarget.value)}
            placeholder="輸入盆栽名稱或位置"
            aria-label="搜尋盆栽"
          />
        </div>
        <span aria-live="polite">{matching.length} 個結果</span>
      </section>
      <div className="data-table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>盆栽</th>
              <th>位置</th>
              <th>土壤含水率</th>
              <th>最近上報</th>
              <th>狀態</th>
            </tr>
          </thead>
          <tbody>
            {matching.length ? (
              matching.map((plant) => (
                <tr key={plant.id}>
                  <th scope="row">{plant.label}</th>
                  <td>{plant.zone}</td>
                  <td>{formatValue(plant.series.moisture.at(-1)!, moisture)}</td>
                  <td>{plant.lastSeen}</td>
                  <td>
                    <Status tone={plant.attention ? "attention" : "healthy"}>
                      {plant.attention ? "待檢查" : "正常"}
                    </Status>
                  </td>
                </tr>
              ))
            ) : (
              <tr>
                <td colSpan={5}>
                  <div className="table-empty">
                    <div>
                      <strong>找不到符合的盆栽</strong>
                      <p>請調整搜尋關鍵字，或清除篩選後再試一次。</p>
                    </div>
                    <Button kind={KIND.tertiary} size={SIZE.compact} onClick={() => setQuery("")}>
                      清除搜尋
                    </Button>
                  </div>
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </>
  );
}

function DevicesPage() {
  const [syncTime, setSyncTime] = useState("5 分鐘前");
  return (
    <>
      <PageHeading
        title="裝置"
        description="檢視 Master Node 連線狀態與感測節點覆蓋範圍。"
        actions={
          <Button kind={KIND.secondary} onClick={() => setSyncTime("剛才")}>
            <Icon name="sync" />
            同步狀態
          </Button>
        }
      />
      <section className="operational-summary">
        <div>
          <strong>2</strong>
          <span>Master Node</span>
        </div>
        <div>
          <strong>6</strong>
          <span>感測節點</span>
        </div>
        <div>
          <strong>0</strong>
          <span>離線裝置</span>
        </div>
      </section>
      <div className="data-table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>Master Node</th>
              <th>覆蓋盆栽</th>
              <th>最近上線</th>
              <th>韌體</th>
              <th>狀態</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <th scope="row">master-east-01</th>
              <td>黃金葛、琴葉榕、龜背芋</td>
              <td>{syncTime}</td>
              <td>master-1.0.0</td>
              <td>
                <Status tone="healthy">在線</Status>
              </td>
            </tr>
            <tr>
              <th scope="row">master-west-01</th>
              <td>波士頓腎蕨、竹芋、香草盆</td>
              <td>{syncTime}</td>
              <td>master-1.0.0</td>
              <td>
                <Status tone="healthy">在線</Status>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </>
  );
}

function ActivityPage() {
  const [range, setRange] = useState("今天");
  const events = [
    ["剛才", "master-east-01", "已接收 telemetry 批次", "6 筆有效讀值"],
    ["5 分鐘前", "波士頓腎蕨", "土壤含水率進入注意範圍", "18.2 %"],
    ["10 分鐘前", "管理操作", "更新琴葉榕的顯示名稱", "無資料變更"],
    ["15 分鐘前", "master-west-01", "已接收 telemetry 批次", "3 筆有效讀值"],
  ];
  return (
    <>
      <PageHeading
        title="活動紀錄"
        description="以時間順序檢視系統事件與管理操作。"
      />
      <section className="table-toolbar">
        <Select
          clearable={false}
          searchable={false}
          options={["今天", "最近 7 天", "最近 30 天"].map((label) => ({
            id: label,
            label,
          }))}
          value={[{ id: range, label: range }]}
          onChange={({ value }) => {
            const next = value[0]?.id;
            if (typeof next === "string") setRange(next);
          }}
        />
        <span>{range} · 4 個事件</span>
      </section>
      <ol className="activity-list">
        {events.map(([time, subject, action, detail]) => (
          <li key={`${time}${subject}`}>
            <time>{time}</time>
            <div>
              <strong>{subject}</strong>
              <p>{action}</p>
            </div>
            <span>{detail}</span>
          </li>
        ))}
      </ol>
    </>
  );
}

function SettingsPage() {
  const [email, setEmail] = useState(true);
  const [darkDefault, setDarkDefault] = useState(false);
  return (
    <>
      <PageHeading title="設定" description="調整通知和 dashboard 預設行為。" />
      <div className="settings-layout">
        <section>
          <h2>通知</h2>
          <p>只在狀態改變或需要人工處理時通知，不對每筆 telemetry 發送訊息。</p>
          <label className="setting-row">
            <span>
              <strong>電子郵件通知</strong>
              <small>盆栽進入注意範圍時</small>
            </span>
            <input
              type="checkbox"
              checked={email}
              onChange={(event) => setEmail(event.currentTarget.checked)}
            />
          </label>
        </section>
        <section>
          <h2>顯示</h2>
          <p>淺色主題是明亮工作環境的預設；可在頁首隨時切換。</p>
          <label className="setting-row">
            <span>
              <strong>預設深色主題</strong>
              <small>僅在低光環境使用</small>
            </span>
            <input
              type="checkbox"
              checked={darkDefault}
              onChange={(event) => setDarkDefault(event.currentTarget.checked)}
            />
          </label>
        </section>
        <section>
          <h2>資料</h2>
          <p>原始五分鐘讀值保存十二個月，之後轉為 Asia/Taipei 日曆日的彙總。</p>
          <Button kind={KIND.secondary}>
            <Icon name="download" />
            匯出資料說明
          </Button>
        </section>
      </div>
    </>
  );
}

function PageContent({ route, plants, isLive }: { route: Route; plants: Plant[]; isLive: boolean }) {
  if (route === "plants") return <PlantsPage plants={plants} />;
  if (route === "devices") return <DevicesPage />;
  if (route === "activity") return <ActivityPage />;
  if (route === "settings") return <SettingsPage />;
  return <Overview plants={plants} isLive={isLive} />;
}

export default function App({
  isDark,
  onToggleTheme,
}: {
  isDark: boolean;
  onToggleTheme: () => void;
}) {
  const [route, setRoute] = useState<Route>("overview");
  const [plants, setPlants] = useState<Plant[]>(fixturePlants);
  const [isLive, setIsLive] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    void fetch("/v1/dashboard/measurements", { signal: controller.signal })
      .then((response) => (response.ok ? response.json() : Promise.reject(new Error("dashboard data unavailable"))))
      .then((data: DashboardResponse) => {
        setPlants(Array.isArray(data.plants) ? data.plants.map(plantFromDashboard) : []);
        setIsLive(true);
      })
      .catch(() => undefined);
    return () => controller.abort();
  }, []);
  const active = navigation.find((item) => item.id === route)!;
  return (
    <div className={`saas-shell${isDark ? " saas-shell--dark" : ""}`}>
      <aside className="app-sidebar" aria-label="主導覽">
        <div className="brand-lockup">
          <strong>Plant telemetry</strong>
          <span>Operations</span>
        </div>
        <nav className="primary-nav">
          {navigation.map((item) => (
            <div key={item.id} className="primary-nav__item">
              <Button
                kind={route === item.id ? KIND.secondary : KIND.tertiary}
                size={SIZE.compact}
                onClick={() => setRoute(item.id)}
                aria-current={route === item.id ? "page" : undefined}
              >
                <Icon name={navIcons[item.id]} />
                {item.label}
              </Button>
              {route === item.id ? <span>{item.hint}</span> : null}
            </div>
          ))}
        </nav>
        <div className="sidebar-footer">
          <Status tone="healthy">系統正常</Status>
          <p>最後同步：剛才</p>
        </div>
      </aside>
      <main className="app-main">
        <header className="app-topbar">
          <div className="app-topbar__crumb">
            <span>營運空間</span>
            <strong>{active.label}</strong>
          </div>
          <div className="app-topbar__actions">
            <span>{isLive ? "即時資料" : "Fixture 資料"}</span>
            <Button
              kind={KIND.tertiary}
              size={SIZE.compact}
              onClick={onToggleTheme}
            >
              <Icon name={isDark ? "light_mode" : "dark_mode"} />
            </Button>
          </div>
        </header>
        <div className="page-content">
          <PageContent route={route} plants={plants} isLive={isLive} />
        </div>
      </main>
    </div>
  );
}
