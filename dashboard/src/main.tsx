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
type PlanOffer = { id: string; country_code: string; country_name: string; isp: string; name: string; download_mbps: number; upload_mbps: number; price_amount?: number; currency_code?: string; price_php?: number; price_period?: string; category?: string; note?: string; source_url?: string; verified_at?: string; custom?: boolean };
type PlanVerdict = { level: "on_par" | "below_plan" | "well_below_plan"; download_pct: number; upload_pct?: number; advertised_download_mbps: number; summary: string; advice: string; complaint_worthy: boolean };
type PlanState = { offer: PlanOffer; verdict?: PlanVerdict };
type PlanSelection = { offer_id: string; custom?: Pick<PlanOffer, "country_code" | "country_name" | "isp" | "name" | "download_mbps" | "upload_mbps"> };
type TestProgress = { phase: string; bytes: number; elapsed_us: number; estimated_bps: number };
type Status = { version: string; test_state: string; scheduler_state: string; connectivity_state: string; paused: boolean; next_automatic_test?: string; provider: { name: string; enabled: boolean }; mlab_consent: Consent; sharing_consent: Consent; sharing_state: string; sharing_available: boolean; last_health?: Health; measurements?: Measurement[]; share_queue_count: number; baseline?: Baseline; plan?: PlanState | null; test_progress?: TestProgress; plan_catalog?: PlanOffer[]; incidents?: Incident[]; last_error?: string };
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
  const testing = !!envelope && envelope.data.test_state !== "idle";
  useEffect(() => {
    if (!testing) return;
    const timer = window.setInterval(refresh, 1500);
    return () => window.clearInterval(timer);
  }, [testing]);
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

function TestPreflightNotice({ mode, busy, onContinue, onClose }: { mode: "vpn" | "wifi"; busy: boolean; onContinue: () => void; onClose: () => void }) {
  const vpn = mode === "vpn";
  return <div class="modal-backdrop"><section class="modal" role="dialog" aria-modal="true" aria-labelledby="test-preflight-title">
    <div class={`modal-icon ${vpn ? "warning" : ""}`}>{vpn ? "!" : "⌁"}</div>
    <p class="eyebrow">Before the speed test</p>
    <h2 id="test-preflight-title">{vpn ? "Disconnect your VPN first" : "Improve your Wi-Fi measurement"}</h2>
    <p class="modal-lead">{vpn ? "A VPN changes the route and can limit the speed. The result would measure the VPN path, not the connection delivered by your provider." : "FiberPulse detected that this Mac is using Wi-Fi. You can continue, but the result may include Wi-Fi distance, walls and interference."}</p>
    <div class="permission-points">
      {vpn ? <>
        <p><b>Turn off the VPN application</b><span>Also disconnect any system VPN profile currently active.</span></p>
        <p><b>Then start the test again</b><span>FiberPulse checks the active route again and refuses the test if the VPN is still detected.</span></p>
      </> : <>
        <p><b>Best method: Ethernet cable</b><span>Connect this Mac directly to the router supplied by your Internet provider.</span></p>
        <p><b>If you stay on Wi-Fi</b><span>Move as close as possible to the provider router before starting the measurement.</span></p>
        <p><b>Pause other traffic</b><span>Stop large downloads, cloud backups and streaming during the test.</span></p>
      </>}
    </div>
    <div class="modal-actions"><button class="button quiet" disabled={busy} onClick={onClose}>Cancel</button><button class="button primary" disabled={busy} onClick={onContinue}>{busy ? "Checking…" : vpn ? "VPN is off — start test" : "Continue on Wi-Fi"}</button></div>
  </section></div>;
}

function TestShow({ state, progress }: { state: string; progress?: TestProgress }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [variant] = useState(() => Math.floor(Math.random() * 4));
  const [startedAt] = useState(() => Date.now());
  const [elapsed, setElapsed] = useState(0);
  const reduced = useMemo(() => window.matchMedia("(prefers-reduced-motion: reduce)").matches, []);

  useEffect(() => {
    const timer = window.setInterval(() => setElapsed(Math.floor((Date.now() - startedAt) / 1000)), 500);
    return () => window.clearInterval(timer);
  }, [startedAt]);

  useEffect(() => {
    if (reduced) return;
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !ctx) return;
    const dpr = window.devicePixelRatio || 1;
    const palette = ["#087cff", "#08c7f5", "#08e889"];
    const resize = () => { const box = canvas.getBoundingClientRect(); canvas.width = Math.max(1, box.width * dpr); canvas.height = Math.max(1, box.height * dpr); };
    resize();
    window.addEventListener("resize", resize);
    const seeds = Array.from({ length: 64 }, (_, i) => ({ a: Math.random() * Math.PI * 2, s: 0.5 + Math.random(), r: 0.35 + 0.6 * ((i % 3) / 2) }));
    let raf = 0;
    const frame = (now: number) => {
      const t = now / 1000;
      const w = canvas.width, h = canvas.height;
      ctx.clearRect(0, 0, w, h);
      ctx.lineCap = "round";
      if (variant === 0) {
        for (let layer = 0; layer < 3; layer++) {
          ctx.beginPath();
          for (let x = 0; x <= w; x += 8 * dpr) {
            const y = h / 2 + Math.sin(x / (90 * dpr) + t * (1.1 + layer * 0.35) + layer * 1.7) * h * (0.13 + layer * 0.07) + Math.sin(x / (31 * dpr) - t * 2.1) * h * 0.05;
            x === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y);
          }
          ctx.strokeStyle = palette[layer] + (layer === 0 ? "" : "99");
          ctx.lineWidth = (2.6 - layer * 0.6) * dpr;
          ctx.stroke();
        }
      } else if (variant === 1) {
        const cx = w / 2, cy = h / 2;
        palette.forEach((color, i) => { ctx.beginPath(); ctx.arc(cx, cy, h * (0.18 + i * 0.16), 0, Math.PI * 2); ctx.strokeStyle = color + "33"; ctx.lineWidth = 1.2 * dpr; ctx.stroke(); });
        seeds.slice(0, 27).forEach((p, i) => {
          const angle = p.a + t * p.s * (i % 2 === 0 ? 1 : -1);
          const radius = h * p.r * 0.75;
          ctx.beginPath(); ctx.arc(cx + Math.cos(angle) * radius * 1.35, cy + Math.sin(angle) * radius, (1.6 + (i % 3)) * dpr, 0, Math.PI * 2);
          ctx.fillStyle = palette[i % 3]; ctx.fill();
        });
      } else if (variant === 2) {
        const count = 42, slot = w / count;
        seeds.slice(0, count).forEach((p, i) => {
          const level = 0.18 + 0.82 * Math.abs(Math.sin(t * (1.2 + p.s) + p.a));
          const bh = h * 0.62 * level, x = i * slot + slot * 0.22;
          const gradient = ctx.createLinearGradient(0, h - bh, 0, h);
          gradient.addColorStop(0, palette[i % 3]); gradient.addColorStop(1, palette[i % 3] + "22");
          ctx.fillStyle = gradient;
          ctx.beginPath(); ctx.roundRect(x, h / 2 + h * 0.31 - bh, slot * 0.56, bh, 3 * dpr); ctx.fill();
        });
      } else {
        const cx = w / 2, cy = h / 2, maxR = h * 0.46;
        for (let i = 0; i < 4; i++) {
          const radius = ((t * 0.24 + i / 4) % 1) * maxR;
          ctx.beginPath(); ctx.arc(cx, cy, radius, 0, Math.PI * 2);
          ctx.strokeStyle = palette[1] + Math.round(200 * (1 - radius / maxR)).toString(16).padStart(2, "0");
          ctx.lineWidth = 2 * dpr; ctx.stroke();
        }
        const sweep = t * 1.4;
        ctx.beginPath(); ctx.moveTo(cx, cy); ctx.arc(cx, cy, maxR, sweep, sweep + 0.6); ctx.closePath();
        ctx.fillStyle = "#08c7f522"; ctx.fill();
      }
      raf = requestAnimationFrame(frame);
    };
    raf = requestAnimationFrame(frame);
    return () => { cancelAnimationFrame(raf); window.removeEventListener("resize", resize); };
  }, [reduced, variant]);

  const phase = progress?.phase || state;
  const label = phase === "download" ? "Measuring download" : phase === "upload" ? "Measuring upload" : phase === "validate" || phase === "persist" || phase === "share_queued" ? "Finishing up" : "Contacting the nearest server";
  const liveMbps = progress && progress.estimated_bps > 0 ? (progress.estimated_bps / 1_000_000).toFixed(1) : null;
  return <div class="test-show" role="status" aria-live="polite">
    <canvas ref={canvasRef} aria-hidden="true" />
    <div class="test-show-overlay">
      <strong>{liveMbps ? liveMbps + " Mbps" : "…"}</strong>
      <span>{label} · {elapsed}s</span>
    </div>
  </div>;
}

function PlanModal({ catalog, current, busy, onSave, onClose }: { catalog: PlanOffer[]; current?: PlanOffer; busy: boolean; onSave: (selection: PlanSelection) => void; onClose: () => void }) {
  const customChoice = "__custom__";
  const countries = useMemo(() => {
    const names = new Map<string, string>();
    catalog.forEach(offer => names.set(offer.country_code, offer.country_name));
    if (current?.country_code) names.set(current.country_code, current.country_name);
    return [...names].map(([code, name]) => ({ code, name })).sort((a, b) => a.name.localeCompare(b.name));
  }, [catalog, current]);
  const [countryCode, setCountryCode] = useState(() => current?.country_code || countries[0]?.code || "PH");
  const countryOffers = catalog.filter(offer => offer.country_code === countryCode);
  const isps = useMemo(() => [...new Set(catalog.filter(offer => offer.country_code === countryCode).map(offer => offer.isp))].sort((a, b) => a.localeCompare(b)), [catalog, countryCode]);
  const [isp, setIsp] = useState(() => current?.custom ? customChoice : current?.isp || isps[0] || customChoice);
  const offers = countryOffers.filter(offer => offer.isp === isp);
  const [offerId, setOfferId] = useState(() => current && offers.some(offer => offer.id === current.id) ? current.id : offers[0]?.id || "");
  const [customProvider, setCustomProvider] = useState(() => current?.custom ? current.isp : "");
  const [customName, setCustomName] = useState(() => current?.custom ? current.name : "");
  const [customDown, setCustomDown] = useState(() => current?.custom ? String(current.download_mbps) : "");
  const [customUp, setCustomUp] = useState(() => current?.custom && current.upload_mbps ? String(current.upload_mbps) : "");
  const selected = countryOffers.find(offer => offer.id === offerId);
  const selectedCountry = countries.find(country => country.code === countryCode) || { code: countryCode, name: countryCode };
  const pickCountry = (value: string) => {
    const matching = catalog.filter(offer => offer.country_code === value);
    const providers = [...new Set(matching.map(offer => offer.isp))].sort((a, b) => a.localeCompare(b));
    const nextISP = providers[0] || customChoice;
    setCountryCode(value);
    setIsp(nextISP);
    setOfferId(matching.find(offer => offer.isp === nextISP)?.id || "");
  };
  const pickIsp = (value: string) => { setIsp(value); setOfferId(countryOffers.find(offer => offer.isp === value)?.id || ""); };
  const customValid = Boolean(customProvider.trim() && customName.trim() && Number(customDown) >= 1 && Number(customDown) <= 10000 && (!customUp || (Number(customUp) >= 0 && Number(customUp) <= 10000)));
  const submit = () => {
    if (isp === customChoice) {
      onSave({ offer_id: "", custom: { country_code: selectedCountry.code, country_name: selectedCountry.name, isp: customProvider.trim(), name: customName.trim(), download_mbps: Number(customDown), upload_mbps: Number(customUp) || 0 } });
      return;
    }
    onSave({ offer_id: offerId });
  };
  const amount = selected?.price_amount || selected?.price_php || 0;
  const currency = selected?.currency_code || (selected?.price_php ? "PHP" : "");
  const price = amount && currency ? `${new Intl.NumberFormat(undefined, { style: "currency", currency, maximumFractionDigits: 0 }).format(amount)} / ${selected?.price_period || "month"}` : "";
  return <div class="modal-backdrop"><section class="modal settings" role="dialog" aria-modal="true" aria-labelledby="plan-title">
    <button class="modal-close" aria-label="Close plan selection" onClick={onClose}>×</button>
    <p class="eyebrow">Your Internet plan</p><h2 id="plan-title">What do you pay for?</h2>
    <p class="modal-text">FiberPulse compares your measurements with the advertised speed of your offer. Consumer plans advertise "up to" speeds, so the verdict stays conservative.</p>
    <label class="plan-field"><span>Country</span><select aria-label="Country" value={countryCode} onChange={event => pickCountry((event.target as HTMLSelectElement).value)}>{countries.map(country => <option value={country.code}>{country.name} ({country.code})</option>)}</select></label>
    <label class="plan-field"><span>Provider</span><select value={isp} onChange={event => pickIsp((event.target as HTMLSelectElement).value)}>{isps.map(name => <option value={name}>{name}</option>)}<option value={customChoice}>My provider / plan is not listed</option></select></label>
    {isp === customChoice ? <div class="custom-plan-grid">
      <label class="plan-field"><span>Provider name</span><input maxlength={80} value={customProvider} onInput={event => setCustomProvider(event.currentTarget.value)} placeholder="e.g. a regional ISP" /></label>
      <label class="plan-field"><span>Offer name</span><input maxlength={120} value={customName} onInput={event => setCustomName(event.currentTarget.value)} placeholder="As written on your bill" /></label>
      <label class="plan-field"><span>Advertised download (Mbps)</span><input type="number" min="1" max="10000" value={customDown} onInput={event => setCustomDown(event.currentTarget.value)} placeholder="500" /></label>
      <label class="plan-field"><span>Advertised upload (optional)</span><input type="number" min="0" max="10000" value={customUp} onInput={event => setCustomUp(event.currentTarget.value)} placeholder="Leave blank if unknown" /></label>
      <p class="plan-note">Use the speed written on your latest bill or contract. FiberPulse marks this as subscriber-entered in the report.</p>
    </div> : <>
      <label class="plan-field"><span>Offer</span><select value={offerId} onChange={event => setOfferId((event.target as HTMLSelectElement).value)}>{[...new Set(offers.map(offer => offer.category || "Other"))].map(category => <optgroup label={category}>{offers.filter(offer => (offer.category || "Other") === category).map(offer => <option value={offer.id}>{offer.name + " — up to " + offer.download_mbps + " Mbps"}</option>)}</optgroup>)}</select></label>
      {selected && <p class="plan-note"><b>{[selected.country_name, selected.category, price].filter(Boolean).join(" · ")}</b>{selected.note && <span>{selected.note}</span>}{selected.verified_at && <span>Official catalog checked {selected.verified_at}.</span>}{selected.source_url && <a href={selected.source_url} target="_blank" rel="noreferrer">Open provider source ↗</a>}</p>}
    </>}
    <div class="modal-actions">{current && <button class="button quiet" disabled={busy} onClick={() => onSave({ offer_id: "" })}>Remove</button>}<button class="button quiet" disabled={busy} onClick={onClose}>Cancel</button><button class="button primary" disabled={busy || (isp === customChoice ? !customValid : !offerId)} onClick={submit}>Save plan</button></div>
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
  const [testPreflight, setTestPreflight] = useState<"vpn" | "wifi" | null>(null);
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
  const planDetail = !planState ? "Compare your results with the offer you pay for." : !verdict ? planState.offer.country_name + " · " + planState.offer.isp + " · advertised up to " + planState.offer.download_mbps + " Mbps — run a test to compare." : verdict.summary + " · " + verdict.download_pct + "% of advertised " + verdict.advertised_download_mbps + " Mbps";
  const planPerformanceIssue = Boolean(verdict && verdict.level !== "on_par");
  const issueCount = activeIncidents.length + (planPerformanceIssue ? 1 : 0);
  const issueTone = verdict?.level === "well_below_plan" ? "red" : issueCount ? "amber" : "green";
  const issueTitle = planPerformanceIssue ? (activeIncidents.length ? `${issueCount} issues detected` : "Speed below your plan") : activeIncidents.length ? `${activeIncidents.length} detected` : "No problem detected";
  const issueDetail = planPerformanceIssue && verdict
    ? `${mbps(latest?.download_bps)} Mbps measured · ${verdict.download_pct}% of your ${verdict.advertised_download_mbps} Mbps plan${activeIncidents.length ? ` · plus ${activeIncidents.length} network incident${activeIncidents.length === 1 ? "" : "s"}` : ""}.`
    : activeIncidents.length ? `Latest network incident: ${words(activeIncidents[0].category)}` : "Your recent checks and plan performance look normal.";
  const networkTone = network.vpn_suspected || network.proxy_suspected ? "danger" : network.online ? "good" : "muted";
  const confidenceTone = !latest ? "muted" : latest.confidence_score >= 80 ? "good" : latest.confidence_score >= 60 ? "warning" : "danger";
  const confidenceLabel = latest ? `${latest.confidence_score}/100` : "No test";

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
    if (network.vpn_suspected) { setTestPreflight("vpn"); return; }
    if (network.connection_type === "wifi") { setTestPreflight("wifi"); return; }
    void run("test");
  };
  const continueTest = () => { setTestPreflight(null); void run("test"); };
  const savePlan = async (selection: PlanSelection) => {
    setActionError(""); setBusy(true);
    try { await action("plan", selection); setPlanOpen(false); }
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
      {status.test_state === "idle" ? <p class="test-note">Measures download, upload and latency. Results stay on this device.</p> : <TestShow state={status.test_state} progress={status.test_progress} />}
    </section>

    <section class="quick-grid">
      <article class="quick-card"><span class={`quick-icon ${status.paused ? "amber" : "green"}`}>{status.paused ? "Ⅱ" : "✓"}</span><div><small>AUTOMATIC MONITORING</small><h2>{monitoringLabel}</h2><p>{status.paused ? "Automatic tests are paused." : status.mlab_consent.granted ? `Next check: ${date(status.next_automatic_test)}` : "Enable it once in Settings."}</p></div><button class="mini-button" onClick={() => void run("pause", { paused: !status.paused })}>{status.paused ? "Resume" : "Pause"}</button></article>
     <article class="quick-card"><span class={`quick-icon ${issueTone}`}>{issueCount ? "!" : "✓"}</span><div><small>ACTIVE ISSUES</small><h2>{issueTitle}</h2><p>{issueDetail}</p></div></article>
      <article class="quick-card"><span class={`quick-icon ${planTone}`}>{verdict ? (verdict.level === "on_par" ? "✓" : "!") : "◈"}</span><div><small>YOUR PLAN</small><h2>{planTitle}</h2><p>{planDetail}</p></div><button class="mini-button" onClick={() => setPlanOpen(true)}>{planState ? "Change" : "Choose"}</button></article>
    </section>

    <section class="history-card"><div class="section-heading"><div><p class="eyebrow">Your speed over time</p><h2>Simple performance history</h2></div><span>{measurements.length} test{measurements.length === 1 ? "" : "s"}</span></div><HistoryChart measurements={measurements} /></section>

    <details class="details-card reports-hub"><summary>
      <span class="reports-summary-icon" aria-hidden="true">⌁</span>
      <span class="reports-summary-copy"><small>DIAGNOSTICS & EVIDENCE</small><b>Details and reports</b><em>Network context, confidence, incidents and exports</em></span>
      <span class="reports-signals" aria-label="Diagnostics summary"><span class={`signal-pill ${networkTone}`}><i />{words(network.connection_type)}</span><span class={`signal-pill ${confidenceTone}`}><i />{confidenceLabel}</span><span class={`signal-pill ${issueCount ? (verdict?.level === "well_below_plan" ? "danger" : "warning") : "good"}`}><i />{issueCount} issue{issueCount === 1 ? "" : "s"}</span></span>
      <i class="reports-chevron">⌄</i>
    </summary><div class="details-content">
      <div class="details-intro"><div><p class="eyebrow">Connection intelligence</p><h2>Your diagnostic workspace</h2><p>Understand the conditions behind each measurement and keep evidence ready when you need to speak with your provider.</p></div><span class="privacy-seal"><b>LOCAL</b><small>Your history stays on this device</small></span></div>
      <div class="detail-grid">
        <article class="insight-card network-card"><header><span aria-hidden="true">⌁</span><div><small>CONNECTION</small><h3>Network context</h3></div><i class={`health-dot ${networkTone}`} /></header><dl><div><dt>Active link</dt><dd>{words(network.connection_type)}</dd></div><div><dt>VPN / proxy</dt><dd>{network.vpn_suspected || network.proxy_suspected ? "Suspected" : "Not detected"}</dd></div><div><dt>Metered network</dt><dd>{network.metered ? "Yes" : "No"}</dd></div></dl></article>
        <article class="insight-card result-card"><header><span aria-hidden="true">↕</span><div><small>MEASUREMENT QUALITY</small><h3>Latest result</h3></div><i class={`health-dot ${confidenceTone}`} /></header><dl><div><dt>Confidence</dt><dd>{latest ? `${latest.confidence_score}/100 · ${words(latest.confidence_level)}` : "No test"}</dd></div><div><dt>Evidence eligible</dt><dd>{latest?.public_eligible ? "Yes" : "No"}</dd></div><div><dt>Test provider</dt><dd>{status.provider.name === "development_fake" ? "Local simulation" : status.provider.name}</dd></div></dl></article>
        <article class="insight-card baseline-card"><header><span aria-hidden="true">◫</span><div><small>PERSONAL REFERENCE</small><h3>Your baseline</h3></div><i class={`health-dot ${baseline.count >= 10 ? "good" : "muted"}`} /></header><dl><div><dt>Qualified tests</dt><dd>{baseline.count}</dd></div><div><dt>Median download</dt><dd>{baseline.count ? `${mbps(baseline.download_median_bps)} Mbps` : "Collecting data"}</dd></div><div><dt>Maturity</dt><dd>{words(baseline.maturity)}</dd></div><div><dt>Network incidents</dt><dd class={activeIncidents.length ? "text-warning" : "text-good"}>{activeIncidents.length || "None"}</dd></div></dl></article>
        <article class="insight-card plan-card"><header><span aria-hidden="true">◇</span><div><small>SUBSCRIBED OFFER</small><h3>Plan diagnosis</h3></div><i class={`health-dot ${verdict ? (verdict.level === "on_par" ? "good" : verdict.level === "below_plan" ? "warning" : "danger") : "muted"}`} /></header>{planState ? <dl><div><dt>Country</dt><dd>{planState.offer.country_name}</dd></div><div><dt>Your offer</dt><dd>{planState.offer.isp + " · " + planState.offer.name}</dd></div><div><dt>Advertised</dt><dd>{"Up to " + (verdict?.advertised_download_mbps || planState.offer.download_mbps) + " Mbps"}</dd></div><div><dt>Latest comparison</dt><dd class={verdict?.level === "on_par" ? "text-good" : verdict ? "text-warning" : ""}>{verdict ? verdict.download_pct + "% · " + verdict.summary : "Run a test to compare"}</dd></div></dl> : <div class="plan-empty"><p>Select your provider and offer to compare measured performance with what you pay for.</p><button class="mini-button" onClick={() => setPlanOpen(true)}>Choose your plan</button></div>}</article>
      </div>
      <section class="export-panel"><div class="export-copy"><span class="export-mark" aria-hidden="true">▤</span><div><p class="eyebrow">Evidence package</p><h3>Take your results with you</h3><p>Create a polished report for your provider or download the underlying measurements for deeper analysis.</p></div></div><div class="report-actions"><button class="export-button pdf" disabled={!!exporting} onClick={() => void exportReport("pdf")}><span class="file-badge">PDF</span><span><b>{exporting === "pdf" ? "Creating report…" : "Professional report"}</b><small>Branded, readable and ready to share</small></span><i>↓</i></button><button class="export-button csv" disabled={!!exporting} onClick={() => void exportReport("csv")}><span class="file-badge">CSV</span><span><b>{exporting === "csv" ? "Preparing data…" : "Raw measurement data"}</b><small>Complete rows for your own analysis</small></span><i>↓</i></button></div></section>
    </div></details>

    <footer><span>FiberPulse {status.version}</span><span>Private by default · Data stored locally</span></footer>

    {settingsOpen && <div class="modal-backdrop"><section class="modal settings" role="dialog" aria-modal="true" aria-labelledby="settings-title"><button class="modal-close" aria-label="Close settings" onClick={() => setSettingsOpen(false)}>×</button><p class="eyebrow">Settings</p><h2 id="settings-title">Simple controls</h2><div class="setting-row"><div><b>Internet speed tests</b><span>{status.mlab_consent.granted ? "Allowed permanently" : "Disabled"}</span></div>{status.mlab_consent.granted ? <button class="mini-button danger" onClick={() => void saveMeasurementPermission(false)}>Disable</button> : <button class="mini-button" onClick={() => { setSettingsOpen(false); setPermissionOpen(true); }}>Enable</button>}</div><div class="setting-row"><div><b>Automatic monitoring</b><span>{status.paused ? "Paused" : "Running"}</span></div><button class="mini-button" onClick={() => void run("pause", { paused: !status.paused })}>{status.paused ? "Resume" : "Pause"}</button></div><div class="setting-row"><div><b>Anonymous sharing</b><span>{status.sharing_consent.granted ? "Enabled" : status.sharing_available ? "Optional and disabled" : "Unavailable in this build"}</span></div>{status.sharing_consent.granted ? <button class="mini-button danger" onClick={() => void saveSharing(false)}>Disable</button> : <button class="mini-button" disabled={!status.sharing_available} onClick={() => { setSettingsOpen(false); setSharingOpen(true); }}>Enable</button>}</div><div class="setting-row"><div><b>Your Internet plan</b><span>{planState ? planState.offer.country_name + " · " + planState.offer.isp + " · " + planState.offer.name : "Not selected"}</span></div><button class="mini-button" onClick={() => { setSettingsOpen(false); setPlanOpen(true); }}>{planState ? "Change" : "Choose"}</button></div><p class="settings-note">Your choices are saved on this device. FiberPulse will not ask again automatically.</p></section></div>}
   {showPermission && <MeasurementPermission firstRun={firstRun} busy={busy} error={actionError} onSave={granted => void saveMeasurementPermission(granted)} onClose={() => setPermissionOpen(false)} />}
    {planOpen && <PlanModal catalog={status.plan_catalog || []} current={planState?.offer} busy={busy} onSave={selection => void savePlan(selection)} onClose={() => setPlanOpen(false)} />}
    {testPreflight && <TestPreflightNotice mode={testPreflight} busy={busy} onContinue={continueTest} onClose={() => setTestPreflight(null)} />}
    {sharingOpen && <SharingPermission busy={busy} error={actionError} onSave={granted => void saveSharing(granted)} onClose={() => setSharingOpen(false)} />}
  </main>;
}

render(<App />, document.getElementById("app")!);
