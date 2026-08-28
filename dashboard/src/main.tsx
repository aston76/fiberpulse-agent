import { render } from "preact";
import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";
import brandMark from "./assets/fiberpulse-mark.png";
import "./styles.css";

type Consent = { scope: string; granted: boolean; policy_version: string; occurred_at?: string };
type Network = { connection_type: string; wifi_quality?: number; metered: boolean; roaming: boolean; vpn_suspected: boolean; proxy_suspected: boolean; online: boolean };
type Health = { at?: string; state: string; category: string; dns_configured: boolean; dns_ok: boolean; probe_configured: boolean; probe_ok: boolean; probe_rtt_us: number; network: Network };
type Measurement = { id: string; started_at: string; provider: string; server_fqdn?: string; download_bps: number; upload_bps: number; min_rtt_us: number; status: string; confidence_score: number; confidence_level: string; confidence_reasons?: string[]; public_eligible: boolean };
type Baseline = { maturity: string; count: number; days: number; download_median_bps: number; download_mad_bps: number; upload_median_bps: number; min_rtt_median_us: number };
type Incident = { id: string; category: string; state: string; suspected_at: string; active_at?: string; recovering_at?: string; resolved_at?: string; updated_at: string };
type Report = { id: string; format: "pdf" | "csv"; state: string; period_start: string; period_end: string; byte_count: number; error_code?: string; created_at: string; updated_at: string };
type Status = { version: string; state: string; test_state: string; scheduler_state: string; paused: boolean; next_automatic_test?: string; provider: { name: string; enabled: boolean }; mlab_consent: Consent; sharing_consent: Consent; sharing_available: boolean; last_health: Health; measurements: Measurement[]; share_queue_count: number; baseline: Baseline; incidents: Incident[]; reports: Report[]; last_error?: string };
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
    const plot = new uPlot({ width: Math.max(240, host.current.clientWidth), height: 220, cursor: { drag: { x: true, y: false } }, scales: { x: { time: true }, y: { range: (_u, min, max) => [0, Math.max(10, max * 1.15)] } }, axes: [{ stroke: "#94a3b8", grid: { stroke: "#172b46" } }, { stroke: "#94a3b8", grid: { stroke: "#172b46" }, label: "Mbps" }], series: [{}, { label: "Download", stroke: "#087cff", width: 2 }, { label: "Upload", stroke: "#08e889", width: 2 }] }, data, host.current);
    const observer = new ResizeObserver(entries => plot.setSize({ width: Math.max(240, entries[0].contentRect.width), height: 220 })); observer.observe(host.current);
    return () => { observer.disconnect(); plot.destroy(); };
  }, [ordered]);
  if (ordered.length < 2) return <div class="empty">Run at least two complete tests to display the performance history.</div>;
  return <div ref={host} class="chart" aria-label="Download and upload performance history chart" />;
}

type ConsentReview = { heading: string; intro: string; points: string[]; acknowledgement: string; confirmLabel: string };

function ConsentCard({ scope, consent, title, children, review, grantDisabled = false, disabledReason = "", onChange }: { scope: string; consent: Consent; title: string; children: preact.ComponentChildren; review: ConsentReview; grantDisabled?: boolean; disabledReason?: string; onChange: (granted: boolean) => Promise<void> }) {
  const [busy, setBusy] = useState(false); const [message, setMessage] = useState(""); const [reviewing, setReviewing] = useState(false); const [acknowledged, setAcknowledged] = useState(false); const cancelRef = useRef<HTMLButtonElement>(null);
  const closeReview = () => { if (!busy) { setReviewing(false); setAcknowledged(false); } };
  useEffect(() => {
    if (!reviewing) return;
    cancelRef.current?.focus();
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === "Escape") closeReview(); };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [reviewing, busy]);
  const change = async (granted: boolean) => {
    setBusy(true); setMessage("");
    try { await onChange(granted); if (granted) { setReviewing(false); setAcknowledged(false); } }
    catch (cause) { setMessage(cause instanceof Error ? cause.message : "Unable to save consent"); }
    finally { setBusy(false); }
  };
  const dialogTitle = `${scope}-consent-title`;
  return <article class="consent-card"><div><p class="eyebrow">Independent consent</p><h3>{title}</h3></div><div class="consent-copy">{children}</div><div class="consent-actions"><span class={`pill ${consent.granted ? "ok" : "muted"}`}>{consent.granted ? "Granted" : "Not granted"}</span><button class="button secondary" disabled={busy || (grantDisabled && !consent.granted)} onClick={() => consent.granted ? void change(false) : setReviewing(true)}>{consent.granted ? "Withdraw" : "Review and grant"}</button></div>{grantDisabled && <p class="muted-copy">{disabledReason}</p>}{message && <p class="error" role="alert">{message}</p>}
    {reviewing && <div class="modal-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) closeReview(); }}><section class="consent-dialog" role="dialog" aria-modal="true" aria-labelledby={dialogTitle}><p class="eyebrow">Explicit consent</p><h2 id={dialogTitle}>{review.heading}</h2><p>{review.intro}</p><ul>{review.points.map(point => <li key={point}>{point}</li>)}</ul><label class="acknowledgement"><input type="checkbox" checked={acknowledged} onChange={event => setAcknowledged(event.currentTarget.checked)} /><span>{review.acknowledgement}</span></label><div class="dialog-actions"><button ref={cancelRef} class="button ghost" disabled={busy} onClick={closeReview}>Cancel</button><button class="button primary" disabled={busy || !acknowledged} onClick={() => void change(true)}>{busy ? "Saving…" : review.confirmLabel}</button></div></section></div>}
  </article>;
}

function App() {
  const { envelope, error, refresh, action } = useStatus(); const [actionError, setActionError] = useState(""); const [exporting, setExporting] = useState("");
  if (!envelope) return <main class="shell"><div class="loading"><span class="pulse" />{error || "Connecting to the local FiberPulse agent…"}</div></main>;
  const status = envelope.data; const health = status.last_health || { state: "unknown", category: "unknown", network: {} as Network } as Health; const measurements = status.measurements || []; const latest = measurements[0]; const personalBaseline = status.baseline || { maturity: "insufficient", count: 0, days: 0, download_median_bps: 0, download_mad_bps: 0, upload_median_bps: 0, min_rtt_median_us: 0 }; const incidents = status.incidents || []; const reports = status.reports || [];
  const providerLabel = status.provider.name === "development_fake" ? "Local simulation" : status.provider.name;
  const connectionLabel = health.state === "internet_usable" ? "Internet usable" : health.state === "local_only" ? "Local link only" : health.state === "offline" ? "Offline" : "Validation pending";
  const schedulerLabel = status.paused ? "Paused" : !status.mlab_consent.granted ? "Consent required" : status.scheduler_state === "blocked_metered" ? "Metered network" : status.scheduler_state === "blocked_quota" ? "Quota reached" : status.scheduler_state === "deferred_busy" ? "Deferred" : status.scheduler_state === "running" ? "Testing" : "Active";
  const schedulerWarning = status.paused || !status.mlab_consent.granted || ["blocked_metered", "blocked_quota", "deferred_busy", "disabled"].includes(status.scheduler_state);
  const checkLabel = (configured: boolean, healthy: boolean) => !configured ? "Not configured" : healthy ? "Healthy" : "Unavailable";
  const run = async (name: string, body: unknown = {}) => { setActionError(""); try { await action(name, body); } catch (cause) { setActionError(cause instanceof Error ? cause.message : "Action failed"); } };
  const downloadReport = async (format: "pdf" | "csv") => {
    setActionError(""); setExporting(format);
    try {
      const response = await fetch(`/api/v1/export/${format}`, { method: "POST", credentials: "same-origin", headers: { "X-CSRF-Token": envelope.csrf_token } });
      if (!response.ok) throw new Error(`Report generation failed (${response.status})`);
      const objectURL = URL.createObjectURL(await response.blob());
      const link = document.createElement("a"); link.href = objectURL; link.download = `fiberpulse-report.${format}`; document.body.appendChild(link); link.click(); link.remove();
      window.setTimeout(() => URL.revokeObjectURL(objectURL), 1000);
      await refresh();
    } catch (cause) { setActionError(cause instanceof Error ? cause.message : "Report generation failed"); }
    finally { setExporting(""); }
  };
  return <main class="shell">
    <header class="topbar"><a class="brand" href="#overview" aria-label="FiberPulse overview"><img src={brandMark} alt="" /><span class="wordmark"><strong><b>Fiber</b>Pulse</strong><small>Know your real Internet performance</small></span></a><nav aria-label="Dashboard sections"><a href="#overview">Overview</a><a href="#history">History</a><a href="#network">Network</a><a href="#incidents">Insights</a><a href="#reports">Reports</a><a href="#privacy">Privacy</a></nav><div class="top-actions"><span class={`live ${health.network?.online ? "online" : "offline"}`}><i />{health.network?.online ? "Online" : "Offline"}</span><button class="button ghost quit" onClick={() => void run("quit")}>Quit completely</button></div></header>
    {(error || actionError || status.last_error) && <div class="notice error" role="alert">{actionError || error || status.last_error}</div>}
    <section class="hero" id="overview">
      <div class="hero-copy"><p class="eyebrow">Local-first network evidence</p><h1>Know Your Real Internet <span>Performance.</span></h1><p>FiberPulse documents the performance you actually observe over time. No inflated promises, no claim of guaranteed line capacity, and no account required.</p><div class="hero-actions"><button class="button primary" disabled={!status.mlab_consent.granted || !status.provider.enabled || status.test_state !== "idle"} onClick={() => void run("test")}>{status.test_state === "idle" ? "Run manual test" : `Test: ${status.test_state}`}</button><button class="button secondary" onClick={() => void run("pause", { paused: !status.paused })}>{status.paused ? "Resume automatic tests" : "Pause automatic tests"}</button></div><div class="trust-row"><span><i>✓</i><b>Local first</b><small>History stays on this device</small></span><span><i>⌁</i><b>Privacy first</b><small>Sharing is a separate opt-in</small></span><span><i>↗</i><b>Transparent</b><small>Versioned methodology</small></span></div></div>
      <article class="measurement-console" aria-label="Current FiberPulse measurement dashboard"><div class="console-head"><div class="console-provider"><img src={brandMark} alt="" /><span><small>Measurement provider</small><strong>{providerLabel}</strong></span></div><div class={`connection-state ${health.state === "internet_usable" ? "connected" : "disconnected"}`}><i />{connectionLabel}</div></div><section class="metrics" aria-label="Latest measurement"><article><span>Download</span><strong>{latest ? fmtMbps(latest.download_bps) : "—"}</strong><small>Measured performance</small></article><article><span>Upload</span><strong>{latest ? fmtMbps(latest.upload_bps) : "—"}</strong><small>Measured performance</small></article><article><span>Minimum RTT</span><strong>{latest ? fmtRTT(latest.min_rtt_us) : "—"}</strong><small>Toward the test server</small></article></section><div class="console-chart" id="history"><div class="panel-head"><div><p class="eyebrow">13-month local history</p><h2>Performance trend</h2></div><button class="button compact" disabled={!!exporting} onClick={() => void downloadReport("csv")}>{exporting === "csv" ? "Generating…" : "Export CSV"}</button></div><PerformanceChart measurements={measurements} /></div><div class="console-foot"><span><small>Context</small><b>{health.network?.connection_type || "unknown"}</b></span><span><small>Confidence</small><b class={`confidence ${latest?.confidence_level || "low"}`}>{latest ? `${latest.confidence_score}/100 · ${latest.confidence_level}` : "Awaiting test"}</b></span><span><small>Tests stored</small><b>{measurements.length}</b></span></div></article>
    </section>
    <section class="insight-grid" id="network">
      <article class="panel"><div class="panel-head"><div><p class="eyebrow">Network status</p><h2>Connection context</h2></div><span class={`pill ${health.state === "internet_usable" ? "ok" : "warn"}`}>{health.state?.replaceAll("_", " ")}</span></div><dl class="facts"><div><dt>Connection</dt><dd>{health.network?.connection_type || "unknown"}</dd></div><div><dt>DNS check</dt><dd>{checkLabel(health.dns_configured, health.dns_ok)}</dd></div><div><dt>Probe check</dt><dd>{checkLabel(health.probe_configured, health.probe_ok)}</dd></div><div><dt>VPN</dt><dd>{health.network?.vpn_suspected ? "Suspected" : "No local signal"}</dd></div><div><dt>Proxy</dt><dd>{health.network?.proxy_suspected ? "Suspected" : "No local signal"}</dd></div><div><dt>Metered</dt><dd>{health.network?.metered ? "Yes — tests blocked" : "No local signal"}</dd></div></dl></article>
      <article class="panel"><div class="panel-head"><div><p class="eyebrow">Automatic monitoring</p><h2>Randomized, never excessive</h2></div><span class={`pill ${schedulerWarning ? "warn" : "ok"}`}>{schedulerLabel}</span></div><p class="schedule-label">Next eligible automatic test</p><p class="schedule-time">{fmtDate(status.next_automatic_test)}</p><div class="limit-bar"><span style={{ width: "50%" }} /></div><p class="muted-copy">Default: two randomized tests per day. Hard limit: four automatic and eight total starts in every rolling 24-hour window. Metered and roaming networks are blocked.</p></article>
      <article class="panel evidence-card"><div class="panel-head"><div><p class="eyebrow">Transparent result</p><h2>Confidence and limitations</h2></div><span class={`score-ring ${latest?.confidence_level || "empty"}`}>{latest ? latest.confidence_score : "—"}<small>{latest?.confidence_level || "No test"}</small></span></div><p>{latest ? "This rule-based score explains whether the result is suitable for local interpretation. It is not a probability." : "Run a complete measurement to calculate a confidence score and its reason codes."}</p><dl class="mini-facts"><div><dt>Provider</dt><dd>{providerLabel}</dd></div><div><dt>M-Lab consent</dt><dd>{status.mlab_consent.granted ? "Granted" : "Not granted"}</dd></div><div><dt>Public eligible</dt><dd>{latest?.public_eligible ? "Yes" : "No"}</dd></div></dl></article>
    </section>
    <section class="local-intelligence" id="incidents">
      <article class="panel baseline-panel"><div class="panel-head"><div><p class="eyebrow">Personal baseline</p><h2>Median and variability</h2></div><span class={`pill ${personalBaseline.maturity === "insufficient" ? "muted" : "ok"}`}>{personalBaseline.maturity.replaceAll("_", " ")}</span></div><p class="muted-copy">The baseline uses only complete, high-confidence local measurements. Median and MAD resist isolated spikes better than a simple average.</p><dl class="baseline-facts"><div><dt>Qualified tests</dt><dd>{personalBaseline.count} across {personalBaseline.days} day{personalBaseline.days === 1 ? "" : "s"}</dd></div><div><dt>Median download</dt><dd>{personalBaseline.count ? fmtMbps(personalBaseline.download_median_bps) : "Awaiting data"}</dd></div><div><dt>Download MAD</dt><dd>{personalBaseline.count ? fmtMbps(personalBaseline.download_mad_bps) : "Awaiting data"}</dd></div><div><dt>Median upload</dt><dd>{personalBaseline.count ? fmtMbps(personalBaseline.upload_median_bps) : "Awaiting data"}</dd></div><div><dt>Median minimum RTT</dt><dd>{personalBaseline.count ? fmtRTT(personalBaseline.min_rtt_median_us) : "Awaiting data"}</dd></div></dl></article>
      <article class="panel incidents-panel"><div class="panel-head"><div><p class="eyebrow">Incident history</p><h2>Observed degradations</h2></div><span class={`pill ${incidents.some(item => item.state === "active") ? "warn" : "muted"}`}>{incidents.length} recorded</span></div>{incidents.length === 0 ? <p class="incident-empty">No incident has met the confirmation rules. A single abnormal observation is never presented as a confirmed outage.</p> : <ol class="incident-list">{incidents.slice(0, 6).map(item => <li class="incident-row" key={item.id}><div><strong>{item.category.replaceAll("_", " ")}</strong><span class={`pill ${item.state === "active" ? "warn" : item.state === "resolved" ? "ok" : "muted"}`}>{item.state}</span><small>Suspected {fmtDate(item.suspected_at)}{item.resolved_at ? ` · Resolved ${fmtDate(item.resolved_at)}` : ""}</small></div>{item.state === "active" && <button class="button compact" onClick={() => void run("incident-dismiss", { id: item.id })}>Dismiss locally</button>}</li>)}</ol>}</article>
    </section>
    <section class="consents" id="privacy"><div class="section-head"><p class="eyebrow">Privacy controls</p><h2>Your data, your choice.</h2><p>M-Lab testing and minimal FiberPulse sharing are separate decisions. Neither is preselected, and both can be withdrawn locally.</p></div><div class="consent-grid">
      <ConsentCard scope="mlab" consent={status.mlab_consent} title="M-Lab measurements" review={{ heading: "Review M-Lab measurement consent", intro: "No NDT7 test starts until you confirm this separate consent.", points: ["NDT7 generates significant synthetic download and upload traffic directly between this device and M-Lab. The default is two randomized automatic tests per day; hard limits are four automatic and eight total starts in any rolling 24 hours.", "Traffic volume varies with line speed and test duration. A plan-based estimate must be shown when an Internet plan is configured; automatic tests remain blocked on metered or roaming networks.", "M-Lab receives the public IP and measurement result, publishes measurement data under its policy, and may retain it indefinitely. FiberPulse cannot delete M-Lab’s historical data.", "You can stop future FiberPulse-initiated tests at any time by withdrawing this consent."], acknowledgement: "I understand the traffic, publication, retention and erasure limitations described above.", confirmLabel: "Grant M-Lab consent" }} onChange={granted => action("consent", { scope: "mlab", granted, language: "en" })}><p>NDT7 sends synthetic traffic directly to M-Lab. M-Lab publishes the public IP and measurement data and may retain them indefinitely. FiberPulse cannot erase M-Lab’s historical public data.</p><p><a href="https://www.measurementlab.net/privacy/" target="_blank" rel="noreferrer">Read M-Lab privacy policy</a></p></ConsentCard>
      <ConsentCard scope="fiberpulse" consent={status.sharing_consent} title="Minimal FiberPulse sharing" grantDisabled={!status.sharing_available} disabledReason="Sharing transport is intentionally disabled in this development build. No FiberPulse cloud request will be sent." review={{ heading: "Review FiberPulse sharing consent", intro: "This choice is independent from M-Lab testing and is disabled by default.", points: ["Shared fields are limited to rounded time, declared ISP and plan, coarse optional location, measured download/upload/RTT, coarse connection context, confidence reasons and version identifiers.", "FiberPulse does not accept exact IP addresses as stored measurement data, SSID, BSSID, hostname, hardware identifiers, accounts, email, GPS, local interface details or local logs.", "The source IP is processed transiently for ASN and abuse prevention. Accepted pseudonymous measurements are retained for 13 months.", "Withdrawal stops new sharing immediately, purges the local queue and requests central deletion when the signed production transport is configured; M-Lab data is outside FiberPulse’s control."], acknowledgement: "I understand exactly what is shared, why it is processed, and how withdrawal works.", confirmLabel: "Enable minimal sharing" }} onChange={granted => action("consent", { scope: "fiberpulse", granted, language: "en" })}><p>Share only the versioned measurement fields, coarse network context and confidence reasons. No exact IP, SSID, BSSID, hostname, account, email, GPS or local logs are stored by FiberPulse.</p><p>Queued events on this machine: {status.share_queue_count}</p></ConsentCard>
    </div></section>
    <section class="panel table-panel" id="reports"><div class="panel-head"><div><p class="eyebrow">Detailed reports</p><h2>Recent measurements</h2></div><button class="button secondary" disabled={!!exporting} onClick={() => void downloadReport("pdf")}>{exporting === "pdf" ? "Generating…" : "Generate factual PDF"}</button></div><div class="table-scroll"><table><thead><tr><th>Date</th><th>Download</th><th>Upload</th><th>Min RTT</th><th>Context</th><th>Confidence</th></tr></thead><tbody>{measurements.length === 0 ? <tr><td colSpan={6} class="empty-cell">No measurements yet. Consent and a valid unmetered network are required.</td></tr> : measurements.map(item => <tr key={item.id}><td>{fmtDate(item.started_at)}</td><td>{fmtMbps(item.download_bps)}</td><td>{fmtMbps(item.upload_bps)}</td><td>{fmtRTT(item.min_rtt_us)}</td><td>{item.status}</td><td><span class={`pill ${item.public_eligible ? "ok" : "muted"}`}>{item.confidence_score} · {item.confidence_level}</span></td></tr>)}</tbody></table></div><div class="report-history"><div class="panel-head"><div><p class="eyebrow">Local export history</p><h3>Generated exports</h3></div><span class="pill muted">{reports.filter(item => item.state !== "deleted").length} retained</span></div><p class="muted-copy">This history contains metadata only. Downloaded PDF and CSV files remain in the folder selected by your browser.</p>{reports.length === 0 ? <p class="incident-empty">No report has been generated on this device.</p> : <ol class="report-list">{reports.slice(0, 8).map(item => <li key={item.id}><div><strong>{item.format.toUpperCase()} · {item.state}</strong><small>{fmtDate(item.created_at)}{item.byte_count ? ` · ${(item.byte_count / 1024).toFixed(1)} KiB` : ""}{item.error_code ? ` · ${item.error_code}` : ""}</small></div>{["ready", "exported", "failed"].includes(item.state) && <button class="button compact" onClick={() => void run("report-delete", { id: item.id })}>Remove entry</button>}</li>)}</ol>}</div></section>
    <footer><span class="footer-brand"><img src={brandMark} alt="" />FiberPulse {status.version}</span><span>All history shown here is stored locally.</span></footer>
  </main>;
}

render(<App />, document.getElementById("app")!);
