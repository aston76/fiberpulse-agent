import { render } from "preact";
import { useEffect, useMemo, useRef, useState } from "preact/hooks";
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";
import brandMark from "./assets/fiberpulse-mark.png";
import "./styles.css";

type Consent = { granted: boolean; policy_version?: string; occurred_at?: string };
type Network = { connection_type?: string; metered?: boolean; roaming?: boolean; vpn_suspected?: boolean; proxy_suspected?: boolean; online?: boolean };
type Health = { state?: string; category?: string; dns_configured?: boolean; dns_ok?: boolean; probe_configured?: boolean; probe_ok?: boolean; network?: Network };
type Measurement = { id: string; started_at: string; download_bps: number; upload_bps: number; min_rtt_us: number; status: string; confidence_score: number; confidence_level: string; public_eligible: boolean };
type Baseline = { maturity: string; count: number; days: number; download_median_bps: number; upload_median_bps: number; min_rtt_median_us: number };
type Incident = { id: string; category: string; state: string; suspected_at: string; resolved_at?: string };
type PlanOffer = { id: string; isp: string; name: string; download_mbps: number; upload_mbps: number; note?: string };
type PlanVerdict = { level: "on_par" | "below_plan" | "well_below_plan"; download_pct: number; upload_pct?: number; summary: string; advice: string; complaint_worthy: boolean };
type PlanState = { offer: PlanOffer; verdict?: PlanVerdict };
type Status = { version: string; test_state: string; scheduler_state: string; connectivity_state: string; paused: boolean; next_automatic_test?: string; provider: { name: string; enabled: boolean }; mlab_consent: Consent; sharing_consent: Consent; sharing_state: string; sharing_available: boolean; last_health?: Health; measurements?: Measurement[]; share_queue_count: number; baseline?: Baseline; plan?: PlanState | null; plan_catalog?: PlanOffer[]; incidents?: Incident[]; last_error?: string };
type Envelope = { csrf_token: string; data: Status };

const mbps = (value = 0) => value > 0 ? (value / 1_000_000).toFixed(value >= 100_000_000 ? 0 : 1) : "—";
const latency = (value = 0) => value > 0 ? (value / 1000).toFixed(value >= 100_000 ? 0 : 1) : "—";
const date = (value?: string) => value ? new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(value)) : "Not scheduled";
const words = (value = "unknown") => value.replaceAll("_", " ");

function useStatus() {
  const [envelope, setEnvelope] = useState<Envelope>();
  const [error, setError] = useState("");
  const refresh = async () => {
    try {
      const response = await fetch("/api/v1/status", { credentials: "same-origin", cache: "no-store" });
      if (!response.ok) throw new Error(`Status unavailable (${response.status})`);
      setEnvelope(await response.json());
      setError("");
    } catch (cause) { setError(cause instanceof Error ? cause.message : "Unable to reach FiberPulse"); }
  };
  useEffect(() => { void refresh(); const timer = window.setInterval(refresh, 10_000); return () => window.clearInterval(timer); }, []);
  const action = async (name: string, body: unknown = {}) => {
    if (!envelope) return;
    const response = await fetch(`/api/v1/actions/${name}`, { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json", "X-CSRF-Token": envelope.csrf_token }, body: JSON.stringify(body) });
    if (!response.ok) {
      const result = await response.json().catch(() => ({}));
      throw new Error(result?.error?.detail || `Action rejected (${response.status})`);
    }
    await refresh();
  };
  return { envelope, error, refresh, action };
}

function HistoryChart({ measurements }: { measurements: Measurement[] }) {
  const host = useRef<HTMLDivElement>(null);
  const complete = useMemo(() => measurements.filter(item => item.status === "complete").slice().reverse(), [measurements]);
  useEffect(() => {
    if (!host.current || complete.length < 2) return;
    const data: uPlot.AlignedData = [complete.map(item => new Date(item.started_at).getTime() / 1000), complete.map(item => item.download_bps / 1e6), complete.map(item => item.upload_bps / 1e6)];
    const plot = new uPlot({ width: Math.max(260, host.current.clientWidth), height: 230, cursor: { drag: { x: true, y: false } }, scales: { x: { time: true }, y: { range: (_u, _min, max) => [0, Math.max(10, max * 1.15)] } }, axes: [{ stroke: "#71869e", grid: { stroke: "#172b46" } }, { stroke: "#71869e", grid: { stroke: "#172b46" } }], series: [{}, { label: "Download", stroke: "#1687ff", width: 3 }, { label: "Upload", stroke: "#12df91", width: 3 }] }, data, host.current);
    const observer = new ResizeObserver(entries => plot.setSize({ width: Math.max(260, entries[0].contentRect.width), height: 230 }));
    observer.observe(host.current);
    return () => { observer.disconnect(); plot.destroy(); };
  }, [complete]);
  if (complete.length < 2) return <div class="chart-empty"><span>⌁</span><strong>Your history will appear here</strong><small>Run two tests to see the trend.</small></div>;
  return <div ref={host} class="chart" aria-label="Speed history chart" />;
}

function MeasurementPermission({ firstRun, busy, error, onSave, onClose }: { firstRun: boolean; busy: boolean; error: string; onSave: (granted: boolean) => void; onClose: () => void }) {
  const [checked, setChecked] = useState(false);
  const firstButton = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    firstButton.current?.focus();
    const escape = (event: KeyboardEvent) => { if (event.key === "Escape" && !firstRun) onClose(); };
    window.addEventListener("keydown", escape);
    return () => window.removeEventListener("keydown", escape);
  }, [firstRun, onClose]);
  return <div class="modal-backdrop"><section class="modal" role="dialog" aria-modal="true" aria-labelledby="permission-title">
    <div class="modal-icon">↕</div>
    <p class="eyebrow">One-time setup</p>
    <h2 id="permission-title">Allow Internet speed tests?</h2>
    <p class="modal-lead">Choose once. FiberPulse will remember your decision permanently on this Mac.</p>
    <div class="permission-points">
      <p><b>2 automatic tests per day</b><span>Never more than 4 automatic tests in 24 hours.</span></p>
      <p><b>Tests use download and upload data</b><span>Automatic tests stop on metered or roaming networks.</span></p>
      <p><b>M-Lab receives and publishes test data</b><span>This includes your public IP. FiberPulse cannot erase M-Lab history.</span></p>
    </div>
    <label class="check"><input type="checkbox" checked={checked} onChange={event => setChecked(event.currentTarget.checked)} /><span>I understand how the tests work and M-Lab’s data policy.</span></label>
    {error && <p class="inline-error" role="alert">{error}</p>}
    <div class="modal-actions"><button ref={firstButton} class="button quiet" disabled={busy} onClick={() => onSave(false)}>Not now</button><button class="button primary" disabled={busy || !checked} onClick={() => onSave(true)}>{busy ? "Saving…" : "Allow permanently"}</button></div>
    <a class="policy-link" href="https://www.measurementlab.net/privacy/" target="_blank" rel="noreferrer">Read M-Lab privacy policy</a>
  </section></div>;
}

function SharingPermission({ busy, error, onSave, onClose }: { busy: boolean; error: string; onSave: (granted: boolean) => void; onClose: () => void }) {
  const [checked, setChecked] = useState(false);
  return <div class="modal-backdrop"><section class="modal" role="dialog" aria-modal="true" aria-labelledby="sharing-title">
    <div class="modal-icon">♥</div><p class="eyebrow">Optional</p><h2 id="sharing-title">Help improve Internet data?</h2>
    <p class="modal-lead">Share anonymous measurement results with FiberPulse. This is separate from speed testing.</p>
    <div class="permission-points"><p><b>No account or contact details</b><span>No email, hostname, exact IP, SSID, GPS or local logs are stored.</span></p><p><b>You stay in control</b><span>Disable sharing anytime; the local queue is immediately cleared.</span></p></div>
    <label class="check"><input type="checkbox" checked={checked} onChange={event => setChecked(event.currentTarget.checked)} /><span>I understand which minimal fields are shared.</span></label>
    {error && <p class="inline-error" role="alert">{error}</p>}
    <div class="modal-actions"><button class="button quiet" disabled={busy} onClick={onClose}>Cancel</button><button class="button primary" disabled={busy || !checked} onClick={() => onSave(true)}>Enable sharing</button></div>
  </section></div>;
}

function PlanModal({ catalog, current, busy, onSave, onClose }: { catalog: PlanOffer[]; current?: string; busy: boolean; onSave: (id: string) => void; onClose: () => void }) {
  const isps = useMemo(() => [...new Set(catalog.map(offer => offer.isp))], [catalog]);
  const [isp, setIsp] = useState(() => catalog.find(offer => offer.id === current)?.isp || isps[0] || "");
  const offers = catalog.filter(offer => offer.isp === isp);
  const [offerId, setOfferId] = useState(() => current && offers.some(offer => offer.id === current) ? current : offers[0]?.id || "");
  const selected = catalog.find(offer => offer.id === offerId);
  const pickIsp = (value: string) => { setIsp(value); setOfferId(catalog.find(offer => offer.isp === value)?.id || ""); };
  return <div class="modal-backdrop"><section class="modal settings" role="dialog" aria-modal="true" aria-labelledby="plan-title">
    <button class="modal-close" aria-label="Close plan selection" onClick={onClose}>×</button>
    <p class="eyebrow">Your Internet plan</p><h2 id="plan-title">What do you pay for?</h2>
    <p class="modal-text">FiberPulse compares your measurements with the advertised speed of your offer. Consumer plans advertise "up to" speeds, so the verdict stays conservative.</p>
    <label class="plan-field"><span>Provider</span><select value={isp} onChange={event => pickIsp((event.target as HTMLSelectElement).value)}>{isps.map(name => <option value={name}>{name}</option>)}</select></label>
    <label class="plan-field"><span>Offer</span><select value={offerId} onChange={event => setOfferId((event.target as HTMLSelectElement).value)}>{offers.map(offer => <option value={offer.id}>{offer.name + " — up to " + offer.download_mbps + " Mbps"}</option>)}</select></label>
    {selected?.note && <p class="plan-note">{selected.note}</p>}
    <div class="modal-actions">{current && <button class="button quiet" disabled={busy} onClick={() => onSave("")}>Remove</button>}<button class="button quiet" disabled={busy} onClick={onClose}>Cancel</button><button class="button primary" disabled={busy || !offerId} onClick={() => onSave(offerId)}>Save plan</button></div>
  </section></div>;
}

function App() {
  const { envelope, error, refresh, action } = useStatus();
  const [actionError, setActionError] = useState("");
  const [busy, setBusy] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [permissionOpen, setPermissionOpen] = useState(false);
  const [sharingOpen, setSharingOpen] = useState(false);
  const [planOpen, setPlanOpen] = useState(false);
  const [exporting, setExporting] = useState("");

  if (!envelope) return <main class="loading"><img src={brandMark} alt="" /><span>{error || "Opening FiberPulse…"}</span></main>;

  const status = envelope.data;
  const health = status.last_health || {};
  const network = health.network || {};
  const measurements = status.measurements || [];
  const latest = measurements[0];
  const incidents = status.incidents || [];
  const activeIncidents = incidents.filter(item => ["active", "suspected", "recovering"].includes(item.state));
  const baseline = status.baseline || { maturity: "insufficient", count: 0, days: 0, download_median_bps: 0, upload_median_bps: 0, min_rtt_median_us: 0 };
  const firstRun = !status.mlab_consent.policy_version;
  const showPermission = firstRun || permissionOpen;
  const connectionState = status.connectivity_state || health.state || "unknown";
  const connectionGood = connectionState === "internet_usable";
  const connectionDetected = connectionGood || Boolean(network.online);
  const connectionLabel = connectionGood ? "Your Internet is working" : connectionState === "offline" ? "You are offline" : connectionState === "unstable" ? "Your Internet is unstable" : connectionState === "internet_degraded" ? "Internet performance is degraded" : network.online ? "Internet connection detected" : "Checking your Internet…";
  const monitoringLabel = status.paused ? "Paused" : status.mlab_consent.granted ? "Active" : "Speed tests off";
  const planState = status.plan || null;
  const verdict = planState?.verdict;
  const planTone = !verdict ? "cyan" : verdict.level === "on_par" ? "green" : verdict.level === "below_plan" ? "amber" : "red";
  const planTitle = planState ? planState.offer.name : "Not selected";
  const planDetail = !planState ? "Compare your results with the offer you pay for." : !verdict ? planState.offer.isp + " · advertised up to " + planState.offer.download_mbps + " Mbps — run a test to compare." : verdict.summary + " · " + verdict.download_pct + "% of advertised " + planState.offer.download_mbps + " Mbps";

  const run = async (name: string, body: unknown = {}) => {
    setActionError(""); setBusy(true);
    try { await action(name, body); }
    catch (cause) { setActionError(cause instanceof Error ? cause.message : "Action failed"); }
    finally { setBusy(false); }
  };
  const saveMeasurementPermission = async (granted: boolean) => {
    setActionError(""); setBusy(true);
    try { await action("consent", { scope: "mlab", granted, language: "en" }); setPermissionOpen(false); }
    catch (cause) { setActionError(cause instanceof Error ? cause.message : "Unable to save your choice"); }
    finally { setBusy(false); }
  };
  const saveSharing = async (granted: boolean) => {
    setActionError(""); setBusy(true);
    try { await action("consent", { scope: "fiberpulse", granted, language: "en" }); setSharingOpen(false); }
    catch (cause) { setActionError(cause instanceof Error ? cause.message : "Unable to save your choice"); }
    finally { setBusy(false); }
  };
  const startTest = () => {
    if (!status.mlab_consent.granted) { setPermissionOpen(true); return; }
    void run("test");
  };
  const savePlan = async (offerId: string) => {
    setActionError(""); setBusy(true);
    try { await action("plan", { offer_id: offerId }); setPlanOpen(false); }
    catch (cause) { setActionError(cause instanceof Error ? cause.message : "Unable to save your plan"); }
    finally { setBusy(false); }
  };
  const exportReport = async (format: "pdf" | "csv") => {
    setExporting(format); setActionError("");
    try {
      const response = await fetch(`/api/v1/export/${format}`, { method: "POST", credentials: "same-origin", headers: { "X-CSRF-Token": envelope.csrf_token } });
      if (!response.ok) throw new Error(`Report generation failed (${response.status})`);
      const url = URL.createObjectURL(await response.blob());
      const link = document.createElement("a"); link.href = url; link.download = `fiberpulse-report.${format}`; link.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000); await refresh();
    } catch (cause) { setActionError(cause instanceof Error ? cause.message : "Report generation failed"); }
    finally { setExporting(""); }
  };

  return <main class="shell">
    <header class="topbar"><div class="brand"><img src={brandMark} alt="" /><span><b>Fiber</b>Pulse</span></div><div class="top-actions"><span class={`online-chip ${network.online ? "online" : ""}`}><i />{network.online ? "Online" : "Offline"}</span><button class="icon-button" aria-label="Open settings" onClick={() => setSettingsOpen(true)}>⚙</button><button class="button quit" onClick={() => void run("quit")}>Quit</button></div></header>
    {(error || actionError || status.last_error) && <div class="notice" role="alert">{actionError || error || status.last_error}</div>}

    <section class="status-hero">
      <div class={`status-symbol ${connectionDetected ? "good" : "checking"}`}>{connectionDetected ? "✓" : "⌁"}</div>
      <p class="eyebrow">Live connection status</p><h1>{connectionLabel}</h1>
      <p class="hero-subtitle">{latest ? `Last speed test: ${date(latest.started_at)}` : "Run your first test to measure the real performance."}</p>
      <div class="speed-grid" aria-label="Latest speed test">
        <article><span class="speed-icon down">↓</span><div><small>DOWNLOAD</small><strong>{mbps(latest?.download_bps)}</strong><em>Mbps</em></div></article>
        <article><span class="speed-icon up">↑</span><div><small>UPLOAD</small><strong>{mbps(latest?.upload_bps)}</strong><em>Mbps</em></div></article>
        <article><span class="speed-icon ping">●</span><div><small>LATENCY</small><strong>{latency(latest?.min_rtt_us)}</strong><em>ms</em></div></article>
      </div>
      <button class="test-button" disabled={busy || !status.provider.enabled || status.test_state !== "idle"} onClick={startTest}><span>▶</span>{status.test_state === "idle" ? "Run speed test" : `Test ${words(status.test_state)}…`}</button>
      <p class="test-note">Measures download, upload and latency. Results stay on this device.</p>
    </section>

    <section class="quick-grid">
      <article class="quick-card"><span class={`quick-icon ${status.paused ? "amber" : "green"}`}>{status.paused ? "Ⅱ" : "✓"}</span><div><small>AUTOMATIC MONITORING</small><h2>{monitoringLabel}</h2><p>{status.paused ? "Automatic tests are paused." : status.mlab_consent.granted ? `Next check: ${date(status.next_automatic_test)}` : "Enable it once in Settings."}</p></div><button class="mini-button" onClick={() => void run("pause", { paused: !status.paused })}>{status.paused ? "Resume" : "Pause"}</button></article>
     <article class="quick-card"><span class={`quick-icon ${activeIncidents.length ? "amber" : "green"}`}>{activeIncidents.length ? "!" : "✓"}</span><div><small>ACTIVE ISSUES</small><h2>{activeIncidents.length ? `${activeIncidents.length} detected` : "No problem detected"}</h2><p>{activeIncidents.length ? `Latest: ${words(activeIncidents[0].category)}` : "Your recent checks look normal."}</p></div></article>
      <article class="quick-card"><span class={`quick-icon ${planTone}`}>{verdict ? (verdict.level === "on_par" ? "✓" : "!") : "◈"}</span><div><small>YOUR PLAN</small><h2>{planTitle}</h2><p>{planDetail}</p></div><button class="mini-button" onClick={() => setPlanOpen(true)}>{planState ? "Change" : "Choose"}</button></article>
    </section>

    <section class="history-card"><div class="section-heading"><div><p class="eyebrow">Your speed over time</p><h2>Simple performance history</h2></div><span>{measurements.length} test{measurements.length === 1 ? "" : "s"}</span></div><HistoryChart measurements={measurements} /></section>

    <details class="details-card"><summary><span><b>Details and reports</b><small>Network context, confidence, incidents and exports</small></span><i>⌄</i></summary><div class="details-content">
      <div class="detail-grid"><article><h3>Network</h3><dl><div><dt>Connection</dt><dd>{words(network.connection_type)}</dd></div><div><dt>VPN / proxy</dt><dd>{network.vpn_suspected || network.proxy_suspected ? "Suspected" : "Not detected"}</dd></div><div><dt>Metered</dt><dd>{network.metered ? "Yes" : "No"}</dd></div></dl></article><article><h3>Latest result</h3><dl><div><dt>Confidence</dt><dd>{latest ? `${latest.confidence_score}/100 · ${words(latest.confidence_level)}` : "No test"}</dd></div><div><dt>Public eligible</dt><dd>{latest?.public_eligible ? "Yes" : "No"}</dd></div><div><dt>Provider</dt><dd>{status.provider.name === "development_fake" ? "Local simulation" : status.provider.name}</dd></div></dl></article><article><h3>Personal baseline</h3><dl><div><dt>Qualified tests</dt><dd>{baseline.count}</dd></div><div><dt>Median download</dt><dd>{baseline.count ? `${mbps(baseline.download_median_bps)} Mbps` : "Collecting data"}</dd></div><div><dt>Maturity</dt><dd>{words(baseline.maturity)}</dd></div></dl></article>{planState && <article><h3>Plan check</h3><dl><div><dt>Your offer</dt><dd>{planState.offer.isp + " · " + planState.offer.name}</dd></div><div><dt>Advertised</dt><dd>{"up to " + planState.offer.download_mbps + " Mbps down" + (planState.offer.upload_mbps ? " / " + planState.offer.upload_mbps + " Mbps up" : "")}</dd></div><div><dt>Latest vs plan</dt><dd>{verdict ? verdict.download_pct + "% of advertised · " + verdict.summary : "Run a test to compare"}</dd></div>{verdict && <div><dt>What it means</dt><dd>{verdict.advice}</dd></div>}</dl></article>}</div>
      <div class="report-actions"><button class="button secondary" disabled={!!exporting} onClick={() => void exportReport("pdf")}>{exporting === "pdf" ? "Creating…" : "Download PDF report"}</button><button class="button secondary" disabled={!!exporting} onClick={() => void exportReport("csv")}>{exporting === "csv" ? "Creating…" : "Download CSV data"}</button></div>
    </div></details>

    <footer><span>FiberPulse {status.version}</span><span>Private by default · Data stored locally</span></footer>

    {settingsOpen && <div class="modal-backdrop"><section class="modal settings" role="dialog" aria-modal="true" aria-labelledby="settings-title"><button class="modal-close" aria-label="Close settings" onClick={() => setSettingsOpen(false)}>×</button><p class="eyebrow">Settings</p><h2 id="settings-title">Simple controls</h2><div class="setting-row"><div><b>Internet speed tests</b><span>{status.mlab_consent.granted ? "Allowed permanently" : "Disabled"}</span></div>{status.mlab_consent.granted ? <button class="mini-button danger" onClick={() => void saveMeasurementPermission(false)}>Disable</button> : <button class="mini-button" onClick={() => { setSettingsOpen(false); setPermissionOpen(true); }}>Enable</button>}</div><div class="setting-row"><div><b>Automatic monitoring</b><span>{status.paused ? "Paused" : "Running"}</span></div><button class="mini-button" onClick={() => void run("pause", { paused: !status.paused })}>{status.paused ? "Resume" : "Pause"}</button></div><div class="setting-row"><div><b>Anonymous sharing</b><span>{status.sharing_consent.granted ? "Enabled" : status.sharing_available ? "Optional and disabled" : "Unavailable in this build"}</span></div>{status.sharing_consent.granted ? <button class="mini-button danger" onClick={() => void saveSharing(false)}>Disable</button> : <button class="mini-button" disabled={!status.sharing_available} onClick={() => { setSettingsOpen(false); setSharingOpen(true); }}>Enable</button>}</div><div class="setting-row"><div><b>Your Internet plan</b><span>{planState ? planState.offer.isp + " · " + planState.offer.name : "Not selected"}</span></div><button class="mini-button" onClick={() => { setSettingsOpen(false); setPlanOpen(true); }}>{planState ? "Change" : "Choose"}</button></div><p class="settings-note">Your choices are saved on this device. FiberPulse will not ask again automatically.</p></section></div>}
   {showPermission && <MeasurementPermission firstRun={firstRun} busy={busy} error={actionError} onSave={granted => void saveMeasurementPermission(granted)} onClose={() => setPermissionOpen(false)} />}
    {planOpen && <PlanModal catalog={status.plan_catalog || []} current={planState?.offer.id} busy={busy} onSave={id => void savePlan(id)} onClose={() => setPlanOpen(false)} />}
    {sharingOpen && <SharingPermission busy={busy} error={actionError} onSave={granted => void saveSharing(granted)} onClose={() => setSharingOpen(false)} />}
  </main>;
}

render(<App />, document.getElementById("app")!);
