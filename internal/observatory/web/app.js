const state={page:1,pages:0,q:"",country:"",provider:"",countries:new Map(),providers:new Set()};
const $=id=>document.getElementById(id);
const number=new Intl.NumberFormat(undefined,{maximumFractionDigits:1});
const integer=new Intl.NumberFormat();
const countryNames=typeof Intl.DisplayNames==="function"?new Intl.DisplayNames([navigator.language||"en"],{type:"region"}):null;

function flag(code){if(!/^[A-Z]{2}$/.test(code||""))return "🌐";return String.fromCodePoint(...[...code].map(char=>127397+char.charCodeAt(0)))}
function text(value,fallback="Not provided"){return value&&String(value).trim()?String(value):fallback}
function place(item){return [item.city,item.region].filter(Boolean).join(", ")||text(item.country_name,item.country_code)}
function date(value){return new Intl.DateTimeFormat(undefined,{year:"numeric",month:"short",day:"numeric",hour:"2-digit",minute:"2-digit",timeZoneName:"short"}).format(new Date(value))}
function escapeHTML(value){return String(value??"").replace(/[&<>'"]/g,char=>({"&":"&amp;","<":"&lt;",">":"&gt;","'":"&#39;",'"':"&quot;"}[char]))}

function resultRow(item){
  const location=escapeHTML(place(item));
  const country=escapeHTML(item.country_name||countryNames?.of(item.country_code)||item.country_code);
  const plan=[item.isp,item.offer_name].filter(Boolean).map(escapeHTML);
  const advertised=item.advertised_download_mbps?`Advertised up to ${integer.format(item.advertised_download_mbps)} Mbps`:"Advertised speed not provided";
  return `<article class="result-row">
    <div class="place"><span class="flag" aria-label="${country}">${flag(item.country_code)}</span><div><b title="${location}">${location}</b><small>${country} · ${escapeHTML(date(item.timestamp))}</small></div></div>
    <div class="plan"><b title="${plan.join(" — ")}">${plan.length?plan.join(" — "):"Subscription not provided"}</b><small>${escapeHTML(advertised)}</small>${item.subscription_type?`<span class="type">${escapeHTML(item.subscription_type)}</span>`:""}</div>
    <div class="metric download"><b>${number.format(item.download_mbps)}</b><small>Mbps down</small></div>
    <div class="metric upload"><b>${number.format(item.upload_mbps)}</b><small>Mbps up</small></div>
    <div class="metric"><b>${number.format(item.latency_ms)}</b><small>ms latency</small></div>
    <div class="context"><span class="connection">${escapeHTML(item.connection_type)}</span><small>${escapeHTML(item.confidence_level)} confidence · ${item.confidence_score}/100</small></div>
  </article>`;
}

function renderFilters(){
  const country=$("country"),provider=$("provider"),selectedCountry=country.value,selectedProvider=provider.value;
  country.innerHTML='<option value="">All countries</option>'+[...state.countries].sort((a,b)=>a[1].localeCompare(b[1])).map(([code,name])=>`<option value="${escapeHTML(code)}">${flag(code)} ${escapeHTML(name)}</option>`).join("");
  provider.innerHTML='<option value="">All providers</option>'+[...state.providers].sort((a,b)=>a.localeCompare(b)).map(name=>`<option value="${escapeHTML(name)}">${escapeHTML(name)}</option>`).join("");
  country.value=selectedCountry;provider.value=selectedProvider;
}

async function loadFacets(){
  try{
    const response=await fetch("/api/v1/public/facets",{headers:{Accept:"application/json"}});if(!response.ok)return;
    const data=await response.json();
    data.countries.forEach(item=>state.countries.set(item.code,item.name||item.code));
    data.providers.forEach(item=>state.providers.add(item.name));renderFilters();
  }catch(error){console.error("measurement filters failed",error)}
}

async function load(){
  $("results").setAttribute("aria-busy","true");
  const params=new URLSearchParams({page:String(state.page),limit:"25"});
  if(state.q)params.set("q",state.q);if(state.country)params.set("country",state.country);if(state.provider)params.set("provider",state.provider);
  try{
    const response=await fetch(`/api/v1/public/measurements?${params}`,{headers:{Accept:"application/json"}});
    if(!response.ok)throw new Error(`HTTP ${response.status}`);
    const data=await response.json();state.pages=data.pages||0;
    $("stat-measurements").textContent=integer.format(data.summary.measurements||0);
    $("stat-countries").textContent=integer.format(data.summary.countries||0);
    $("stat-providers").textContent=integer.format(data.summary.providers||0);
    $("stat-download").textContent=number.format(data.summary.avg_download_mbps||0);
    $("result-count").textContent=`${integer.format(data.total)} result${data.total===1?"":"s"}`;
    $("results").innerHTML=data.items.map(resultRow).join("");
    $("empty").hidden=data.items.length!==0;
    $("page-label").textContent=state.pages?`Page ${data.page} of ${state.pages}`:"No pages";
    $("previous").disabled=data.page<=1;$("next").disabled=!state.pages||data.page>=state.pages;
  }catch(error){
    console.error("measurement explorer failed",error);
    $("results").innerHTML='<div class="empty"><span class="empty-mark">!</span><h3>Data is temporarily unavailable</h3><p>Please retry in a moment.</p></div>';
    $("result-count").textContent="Unavailable";
  }finally{$("results").removeAttribute("aria-busy")}
}

$("search-form").addEventListener("submit",event=>{event.preventDefault();state.q=$("search").value.trim();state.country=$("country").value;state.provider=$("provider").value;state.page=1;void load()});
$("previous").addEventListener("click",()=>{if(state.page>1){state.page--;void load();scrollTo({top:document.querySelector(".explorer").offsetTop,behavior:"smooth"})}});
$("next").addEventListener("click",()=>{if(state.page<state.pages){state.page++;void load();scrollTo({top:document.querySelector(".explorer").offsetTop,behavior:"smooth"})}});
void Promise.all([loadFacets(),load()]);
