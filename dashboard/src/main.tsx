import { render } from "preact";
import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";
import "./styles.css";

type Consent = { scope: string; granted: boolean; policy_version: string; occurred_at?: string };
type Network = { connection_type: string; wifi_quality?: number; metered: boolean; roaming: boolean; vpn_suspected: boolean; proxy_suspected: boolean; online: boolean };
type Health = { at?: string; state: string; category: string; dns_ok: boolean; probe_ok: boolean; probe_rtt_us: number; network: Network };
type Measurement = { id: string; started_at: string; provider: string; server_fqdn?: string; download_bps: number; upload_bps: number; min_rtt_us: number; status: string; confidence_score: number; confidence_level: string; confidence_reasons?: string[]; public_eligible: boolean };
type Status = { version: string; state: string; test_state: string; paused: boolean; next_automatic_test?: string; provider: { name: string; enabled: boolean }; mlab_consent: Consent; sharing_consent: Consent; last_health: Health; measurements: Measurement[]; share_queue_count: number; last_error?: string };
type Envelope = { csrf_token: string; data: Status };

const fmtMbps = (value = 0) => `${(value / 1_000_000).toFixed(1)} Mbps`;
const fmtRTT = (value = 0) => value > 0 ? `${(value / 1000).toFixed(1)} ms` : "—";
const fmtDate = (value?: string) => value ? new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(value)) : "Not scheduled";

function useStatus() {
  const [envelope, setEnvelope] = useState<Envelope>();
  const [error, setError] = useState("");
  const refresh = async () => {
    try {
      const response = await fetch("/api/v1/status", { credentials: "same-origin", cache: "no-store" });
      if (!response.ok) throw new Error(`Status request failed (${response.status})`);
      setEnvelope(await response.json()); setError("");
    } catch (cause) { setError(cause instanceof Error ? cause.message : "Dashboard connection failed"); }
  };
  useEffect(() => { void refresh(); const timer = window.setInterval(refresh, 10_000); return () => window.clearInterval(timer); }, []);
  const action = async (name: string, body: unknown = {}) => {
    if (!envelope) return;
    const response = await fetch(`/api/v1/actions/${name}`, { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json", "X-CSRF-Token": envelope.csrf_token }, body: JSON.stringify(body) });
    if (!response.ok) { const result = await response.json().catch(() => ({})); throw new Error(result?.error?.detail || `Action rejected (${response.status})`); }
    await refresh();
  };
  return { envelope, error, refresh, action };
}

function PerformanceChart({ measurements }: { measurements: Measurement[] }) {
  const host = useRef<HTMLDivElement>(null);
  const ordered = useMemo(() => [...measurements].filter(item => item.status === "complete").reverse(), [measurements]);
  useEffect(() => {
    if (!host.current || ordered.length < 2) return;
    const data: uPlot.AlignedData = [ordered.map(item => new Date(item.started_at).getTime() / 1000), ordered.map(item => item.download_bps / 1e6), ordered.map(item => item.upload_bps / 1e6)];
    const plot = new uPlot({ width: Math.max(280, host.current.clientWidth), height: 250, cursor: { drag: { x: true, y: false } }, scales: { x: { time: true }, y: { range: (_u, min, max) => [0, Math.max(10, max * 1.15)] } }, axes: [{ stroke: "#91a8b7", grid: { stroke: "#183544" } }, { stroke: "#91a8b7", grid: { stroke: "#183544" }, label: "Mbps" }], series: [{}, { label: "Download", stroke: "#38d6ad", width: 2 }, { label: "Upload", stroke: "#67a8ff", width: 2 }] }, data, host.current);
    const observer = new ResizeObserver(entries => plot.setSize({ width: Math.max(280, entries[0].contentRect.width), height: 250 })); observer.observe(host.current);
    return () => { observer.disconnect(); plot.destroy(); };
  }, [ordered]);
  if (ordered.length < 2) return <div class="empty">Run at least two complete tests to display the performance history.</div>;
  return <div ref={host} class="chart" aria-label="Download and upload performance history chart" />;
}

function ConsentCard({ consent, title, children, onChange }: { consent: Consent; title: string; children: preact.ComponentChildren; onChange: (granted: boolean) => Promise<void> }) {
  const [busy, setBusy] = useState(false); const [message, setMessage] = useState("");
  const change = async (granted: boolean) => { setBusy(true); setMessage(""); try { await onChange(granted); } catch (cause) { setMessage(cause instanceof Error ? cause.message : "Unable to save consent"); } finally { setBusy(false); } };
  return <article class="consent-card"><div><p class="eyebrow">Independent consent</p><h3>{title}</h3></div><div class="consent-copy">{children}</div><div class="consent-actions"><span class={`pill ${consent.granted ? "ok" : "muted"}`}>{consent.granted ? "Granted" : "Not granted"}</span><button class="button secondary" disabled={busy} onClick={() => void change(!consent.granted)}>{consent.granted ? "Withdraw" : "Review and grant"}</button></div>{message && <p class="error" role="alert">{message}</p>}</article>;
}

function App() {
  const { envelope, error, action } = useStatus(); const [actionError, setActionError] = useState("");
  if (!envelope) return <main class="shell"><div class="loading"><span class="pulse" />{error || "Connecting to the local FiberPulse agent…"}</div></main>;
  const status = envelope.data; const health = status.last_health || { state: "unknown", category: "unknown", network: {} as Network } as Health; const latest = status.measurements[0];
  const run = async (name: string, body: unknown = {}) => { setActionError(""); try { await action(name, body); } catch (cause) { setActionError(cause instanceof Error ? cause.message : "Action failed"); } };
  return <main class="shell">
    <header class="topbar"><div class="brand"><span class="brand-mark" aria-hidden="true"><i /><i /><i /></span><div><strong>FiberPulse</strong><span>Measured Internet performance</span></div></div><div class="top-actions"><span class={`live ${health.network?.online ? "online" : "offline"}`}><i />{health.network?.online ? "Online" : "Offline"}</span><button class="button ghost" onClick={() => void run("quit")}>Quit completely</button></div></header>
    <section class="hero"><div><p class="eyebrow">Local-first network evidence</p><h1>Your connection, measured over time.</h1><p>FiberPulse separates observed performance from assumptions. Results describe a test path at a point in time — never guaranteed line capacity or proof of ISP responsibility.</p></div><div class="hero-actions"><button class="button primary" disabled={!status.mlab_consent.granted || !status.provider.enabled || status.test_state !== "idle"} onClick={() => void run("test")}>{status.test_state === "idle" ? "Run manual test" : `Test: ${status.test_state}`}</button><button class="button secondary" onClick={() => void run("pause", { paused: !status.paused })}>{status.paused ? "Resume automatic tests" : "Pause automatic tests"}</button></div></section>
    {(error || actionError || status.last_error) && <div class="notice error" role="alert">{actionError || error || status.last_error}</div>}
    <section class="metrics" aria-label="Latest measurement">
      <article><span>Download</span><strong>{latest ? fmtMbps(latest.download_bps) : "—"}</strong><small>Observed toward test server</small></article>
      <article><span>Upload</span><strong>{latest ? fmtMbps(latest.upload_bps) : "—"}</strong><small>Observed toward test server</small></article>
      <article><span>Minimum NDT7 RTT</span><strong>{latest ? fmtRTT(latest.min_rtt_us) : "—"}</strong><small>Not universal Internet ping</small></article>
      <article><span>Confidence</span><strong class={`confidence ${latest?.confidence_level || "low"}`}>{latest ? `${latest.confidence_score}/100` : "—"}</strong><small>{latest?.confidence_level || "No completed test"}</small></article>
    </section>
    <section class="grid">
      <article class="panel wide"><div class="panel-head"><div><p class="eyebrow">13-month local history</p><h2>Performance trend</h2></div><a class="button ghost" href="/api/v1/export/csv">Export CSV</a></div><PerformanceChart measurements={status.measurements} /></article>
      <article class="panel"><div class="panel-head"><div><p class="eyebrow">Current context</p><h2>Network validation</h2></div><span class={`pill ${health.state === "internet_usable" ? "ok" : "warn"}`}>{health.state?.replaceAll("_", " ")}</span></div><dl class="facts"><div><dt>Connection</dt><dd>{health.network?.connection_type || "unknown"}</dd></div><div><dt>DNS check</dt><dd>{health.dns_ok ? "Healthy" : "Unavailable"}</dd></div><div><dt>Probe check</dt><dd>{health.probe_ok ? "Healthy" : "Unavailable"}</dd></div><div><dt>VPN</dt><dd>{health.network?.vpn_suspected ? "Suspected" : "No local signal detected"}</dd></div><div><dt>Proxy</dt><dd>{health.network?.proxy_suspected ? "Suspected" : "No local signal detected"}</dd></div><div><dt>Metered</dt><dd>{health.network?.metered ? "Yes — automatic tests blocked" : "No local signal detected"}</dd></div></dl></article>
      <article class="panel"><div class="panel-head"><div><p class="eyebrow">Scheduler</p><h2>Automatic testing</h2></div><span class={`pill ${status.paused ? "warn" : "ok"}`}>{status.paused ? "Paused" : "Active"}</span></div><p class="schedule-time">{fmtDate(status.next_automatic_test)}</p><p class="muted-copy">Default rate: two randomized tests per day. Hard limit: four automatic and eight total starts in every rolling 24-hour window.</p></article>
    </section>
    <section class="consents"><div class="section-head"><p class="eyebrow">Privacy controls</p><h2>Nothing is assumed.</h2><p>M-Lab testing and FiberPulse sharing are separate choices. Both can be withdrawn locally.</p></div><div class="consent-grid">
      <ConsentCard consent={status.mlab_consent} title="M-Lab measurements" onChange={granted => action("consent", { scope: "mlab", granted, language: "en" })}><p>NDT7 sends synthetic traffic directly to M-Lab. M-Lab publishes the public IP and measurement data and may retain them indefinitely. FiberPulse cannot erase M-Lab’s historical public data.</p><p><a href="https://www.measurementlab.net/privacy/" target="_blank" rel="noreferrer">Read M-Lab privacy policy</a></p></ConsentCard>
      <ConsentCard consent={status.sharing_consent} title="Minimal FiberPulse sharing" onChange={granted => action("consent", { scope: "fiberpulse", granted, language: "en" })}><p>Share only the versioned measurement fields, coarse network context and confidence reasons. No exact IP, SSID, BSSID, hostname, account, email, GPS or local logs are stored by FiberPulse.</p><p>Queued events on this machine: {status.share_queue_count}</p></ConsentCard>
    </div></section>
    <section class="panel table-panel"><div class="panel-head"><div><p class="eyebrow">Evidence log</p><h2>Recent measurements</h2></div><a class="button secondary" href="/api/v1/export/pdf">Generate factual PDF</a></div><div class="table-scroll"><table><thead><tr><th>Date</th><th>Download</th><th>Upload</th><th>Min RTT</th><th>Context</th><th>Confidence</th></tr></thead><tbody>{status.measurements.length === 0 ? <tr><td colSpan={6} class="empty-cell">No measurements yet. Consent and a valid unmetered network are required.</td></tr> : status.measurements.map(item => <tr key={item.id}><td>{fmtDate(item.started_at)}</td><td>{fmtMbps(item.download_bps)}</td><td>{fmtMbps(item.upload_bps)}</td><td>{fmtRTT(item.min_rtt_us)}</td><td>{item.status}</td><td><span class={`pill ${item.public_eligible ? "ok" : "muted"}`}>{item.confidence_score} · {item.confidence_level}</span></td></tr>)}</tbody></table></div></section>
    <footer><span>FiberPulse {status.version}</span><span>All history shown here is stored locally.</span></footer>
  </main>;
}

render(<App />, document.getElementById("app")!);
