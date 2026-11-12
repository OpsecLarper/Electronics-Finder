"use strict";
(() => {
const TOKEN = new URLSearchParams(location.search).get("t") || "";
const $ = (s, el = document) => el.querySelector(s);
const $$ = (s, el = document) => [...el.querySelectorAll(s)];
const esc = s => String(s ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

const api = (path, opts = {}) => fetch(path, { ...opts, headers: { "X-Token": TOKEN, "Content-Type": "application/json", ...(opts.headers || {}) } })
  .then(r => { if (!r.ok) throw new Error(r.status); return r.status === 204 ? null : r.json(); });

const SUGGEST = ["ESP32-S3", "ESP32-C3", "CC1101", "N20 gear motor", "SG90 servo", "NRF24L01", "SX1262 LoRa", "TMC2209", "DRV8833", "BME280", "Raspberry Pi Pico 2", "WS2812B"];
const REGIONS = { NA: "North America", UK: "United Kingdom", EU: "Europe", ASIA: "Asia", OTHER: "Other" };
const CURRENCIES = ["USD", "EUR", "GBP", "CAD", "AUD", "JPY", "CNY", "INR", "CHF", "SEK", "NOK", "DKK", "PLN", "NZD", "SGD", "HKD", "MXN", "BRL"];
const WEIGHT_DEFS = [
  ["price", "Price", 35], ["trust", "Trust", 30], ["shipping", "Shipping", 15], ["stock", "In stock", 15], ["support", "Support", 5],
];

const state = {
  meta: null, stores: {}, settings: {}, fx: { USD: 1 },
  query: "", status: {}, offers: [], es: null, searching: false,
  mode: "stores", sort: "overall", stockOnly: false,
  weights: loadLS("ef.weights", Object.fromEntries(WEIGHT_DEFS.map(([k, , v]) => [k, v]))),
};

function loadLS(k, d) { try { const v = JSON.parse(localStorage.getItem(k)); return v ?? d; } catch { return d; } }
function saveLS(k, v) { try { localStorage.setItem(k, JSON.stringify(v)); } catch {} }

// ---------- money ----------
function toUSD(price, cur) { const r = state.fx[cur]; return r ? price / r : price; }
function fmt(usd) {
  const cur = state.settings.currency || "USD";
  const v = usd * (state.fx[cur] || 1);
  try { return new Intl.NumberFormat(undefined, { style: "currency", currency: cur, maximumFractionDigits: v < 10 ? 2 : (v < 1000 ? 2 : 0) }).format(v); }
  catch { return cur + " " + v.toFixed(2); }
}
function fmtOrig(o) {
  try { return new Intl.NumberFormat(undefined, { style: "currency", currency: o.currency }).format(o.price); }
  catch { return o.currency + " " + o.price.toFixed(2); }
}

// ---------- scoring ----------
function scoreAll() {
  const region = state.settings.region || "NA";
  const list = state.offers;
  const inStockPrices = list.filter(o => o.inStock !== false).map(o => o.usd);
  const minP = Math.min(...(inStockPrices.length ? inStockPrices : list.map(o => o.usd)));
  const W = state.weights;
  const wSum = Object.values(W).reduce((a, b) => a + b, 0) || 1;
  for (const o of list) {
    const st = state.stores[o.storeId] || {};
    const s = {};
    s.price = Math.max(0, Math.min(100, 100 * Math.pow(minP / o.usd, 0.55)));
    const trust = o.sellerTrust ?? st.trust ?? 3;
    s.trust = trust / 5 * 100;
    s.shipping = ((st.shipping || {})[region] ?? 3) / 5 * 100;
    s.stock = o.inStock === true ? 100 : o.inStock === false ? 8 : 62;
    s.support = (st.support ?? 3) / 5 * 100;
    let overall = 0;
    for (const [k] of WEIGHT_DEFS) overall += (W[k] || 0) * s[k];
    overall = overall / wSum * (0.85 + 0.15 * (o.relevance ?? 1));
    o.s = s;
    o.overall = Math.round(overall);
  }
}

function visibleOffers() {
  let l = state.offers.slice();
  if (state.stockOnly) l = l.filter(o => o.inStock !== false);
  const key = state.sort;
  const cmp = {
    overall: (a, b) => b.overall - a.overall,
    price: (a, b) => a.usd - b.usd,
    trust: (a, b) => b.s.trust - a.s.trust || b.overall - a.overall,
    shipping: (a, b) => b.s.shipping - a.s.shipping || b.overall - a.overall,
  }[key];
  return l.sort(cmp);
}

// ---------- UI bits ----------
function scoreColor(v) { const h = Math.max(0, Math.min(130, (v - 35) * 2.1)); return `hsl(${h} 85% 62%)`; }
function ring(v, label = "", size = 58) {
  const r = size / 2 - 5, c = 2 * Math.PI * r, off = c * (1 - v / 100);
  return `<div class="ring" style="width:${size}px;height:${size}px;color:${scoreColor(v)}">
    <svg width="${size}" height="${size}"><circle class="bgc" cx="${size / 2}" cy="${size / 2}" r="${r}"/>
    <circle class="fg" cx="${size / 2}" cy="${size / 2}" r="${r}" stroke="currentColor" stroke-dasharray="${c}" stroke-dashoffset="${off}"/></svg>
    <b style="color:var(--text)">${v}</b>${label ? `<small>${label}</small>` : ""}</div>`;
}
const stars = v => `<span class="stars" style="--v:${v}" title="${v} / 5"></span>`;
const bar = (label, v, max = 5) => `<div class="bar"><span>${label}</span><div class="track"><div class="fill" style="width:${v / max * 100}%"></div></div><span class="v">${(+v).toFixed(1).replace(/\.0$/, "")}</span></div>`;
function stockPill(o) {
  if (o.inStock === true) return `<span class="pill good">${o.stockQty > 0 ? (o.stockQty > 999 ? "999+" : o.stockQty) + " in stock" : "In stock"}</span>`;
  if (o.inStock === false) return `<span class="pill bad">Out of stock</span>`;
  return `<span class="pill subtle">Stock unknown</span>`;
}
function thumb(o, cls = "thumb") {
  return o.image ? `<img class="${cls}" loading="lazy" src="${esc(o.image)}" alt="" referrerpolicy="no-referrer" onerror="this.outerHTML='<div class=&quot;${cls} ph&quot;>◇</div>'">`
    : `<div class="${cls} ph">◇</div>`;
}
function priceLine(o) {
  const cur = state.settings.currency || "USD";
  let extra = o.currency !== cur ? `<span class="orig">${esc(fmtOrig(o))}</span>` : "";
  if (o.minQty > 1) extra += `<span class="orig">each · min ${o.minQty}</span>`;
  return `${fmt(o.usd)} ${extra}`;
}
function toast(msg, ms = 2600) {
  const t = $("#toast"); t.textContent = msg; t.classList.remove("hidden");
  clearTimeout(toast.t); toast.t = setTimeout(() => t.classList.add("hidden"), ms);
}
function openExt(url) {
  api("/api/open?url=" + encodeURIComponent(url)).catch(() => window.open(url, "_blank"));
}
const searchURL = (st, q) => st.searchUrl.replace("%s", encodeURIComponent(q));

// ---------- navigation ----------
function setView(v) {
  $$(".view").forEach(el => el.classList.toggle("active", el.id === "view-" + v));
  $$("#nav button").forEach(b => b.classList.toggle("active", b.dataset.view === v));
  moveThumb();
  if (v === "stores") renderDirectory();
  if (v === "settings") renderSettings();
  window.scrollTo({ top: 0, behavior: "smooth" });
}
function moveThumb() {
  const nav = $("#nav"), act = $("#nav button.active"), th = $(".seg-thumb", nav);
  if (!act) return;
  th.style.left = act.offsetLeft + "px"; th.style.width = act.offsetWidth + "px";
}

// ---------- search ----------
function renderChips() {
  const recent = loadLS("ef.recent", []);
  const set = new Set(recent.map(r => r.toLowerCase()));
  const sug = SUGGEST.filter(s => !set.has(s.toLowerCase())).slice(0, Math.max(4, 10 - recent.length));
  $("#chips").innerHTML = recent.slice(0, 6).map(r => `<button class="chip recent" type="button">${esc(r)}</button>`).join("")
    + sug.map(s => `<button class="chip" type="button">${esc(s)}</button>`).join("");
}

function startSearch(q) {
  q = q.trim(); if (!q) return;
  $("#q").value = q;
  state.query = q; state.offers = []; state.status = {};
  const recent = loadLS("ef.recent", []).filter(r => r.toLowerCase() !== q.toLowerCase());
  recent.unshift(q); saveLS("ef.recent", recent.slice(0, 8));
  renderChips();
  if (state.es) state.es.close();
  setView("search");
  document.body.classList.add("has-results");
  $("#results").classList.remove("hidden");
  $("#statusBar").classList.remove("hidden");
  state.searching = true;
  $("#goBtn").disabled = true; $("#goBtn").innerHTML = `<span class="spinner"></span>Searching`;
  const disabled = new Set(state.settings.disabled || []);
  for (const st of Object.values(state.stores)) if (st.live && !disabled.has(st.id)) state.status[st.id] = { status: "loading" };
  renderAll();
  const es = new EventSource(`/api/search?t=${encodeURIComponent(TOKEN)}&q=${encodeURIComponent(q)}`);
  state.es = es;
  es.addEventListener("store", ev => {
    const r = JSON.parse(ev.data);
    state.status[r.storeId] = { status: r.status === "ok" && !r.offers.length ? "empty" : r.status, message: r.message, ms: r.ms, n: r.offers.length };
    for (const o of r.offers) { o.usd = toUSD(o.price, o.currency); state.offers.push(o); }
    renderAll();
  });
  const finish = () => {
    es.close(); state.searching = false;
    $("#goBtn").disabled = false; $("#goBtn").textContent = "Search";
    for (const id in state.status) if (state.status[id].status === "loading") state.status[id] = { status: "error", message: "No response" };
    renderAll();
  };
  es.addEventListener("done", finish);
  es.onerror = () => { if (state.searching) finish(); };
}

function renderStatus() {
  const items = Object.entries(state.status).map(([id, s]) => {
    const st = state.stores[id] || { name: id };
    const tip = s.status === "loading" ? "Searching…" : s.status === "ok" ? `${s.n} matching listing${s.n === 1 ? "" : "s"} · ${s.ms} ms`
      : s.status === "empty" ? "No matching products" : (s.message || s.status);
    return `<span class="src ${s.status}" title="${esc(tip)}"><span class="dot"></span>${esc(st.name)}${s.n ? ` · ${s.n}` : ""}</span>`;
  });
  $("#statusBar").innerHTML = items.join("");
}

function renderAll() {
  if (!state.query) return;
  scoreAll();
  renderStatus();
  const offers = visibleOffers();
  const nStores = new Set(state.offers.map(o => o.storeId)).size;
  $("#summary").textContent = state.offers.length
    ? `${state.offers.length} listings from ${nStores} store${nStores === 1 ? "" : "s"} for “${state.query}”${state.searching ? " · still searching…" : ""}`
    : state.searching ? "Searching stores…" : "";
  renderPicks(offers);
  if (state.mode === "stores") renderByStore(offers); else renderCards(offers);
  renderAlso();
}

function renderPicks(offers) {
  const el = $("#picks");
  if (!offers.length) {
    el.innerHTML = state.searching ? [0, 1, 2, 3].map(() => `<div class="skel" style="height:210px"></div>`).join("")
      : `<div class="glass empty" style="grid-column:1/-1">No live listings matched “${esc(state.query)}”. Try a simpler term (e.g. “CC1101” instead of “CC1101 module 433MHz”), or check the stores below.</div>`;
    return;
  }
  const inStock = offers.filter(o => o.inStock !== false);
  const pool = inStock.length ? inStock : offers;
  const used = new Set();
  const by = f => { const l = pool.slice().sort(f); const o = l.find(x => !used.has(x)) || l[0]; used.add(o); return o; };
  const picks = [
    ["Best overall", "★", by((a, b) => b.overall - a.overall)],
    ["Lowest price", "$", by((a, b) => a.usd - b.usd)],
    ["Most trusted", "✓", by((a, b) => b.s.trust - a.s.trust || a.usd - b.usd)],
    ["Fastest to you", "➜", by((a, b) => b.s.shipping - a.s.shipping || b.overall - a.overall)],
  ];
  el.innerHTML = picks.map(([label, ic, o], i) => {
    const st = state.stores[o.storeId] || {};
    return `<div class="pick glass liquid p${i}" data-url="${esc(o.url)}" style="animation-delay:${i * 60}ms">
      <div class="label"><i>${ic}</i>${label}</div>
      <div class="price">${fmt(o.usd)}</div>
      <div class="title">${esc(o.title)}</div>
      <div class="store-line"><div><div class="store-name">${esc(st.name || o.storeId)}</div>${stars(o.sellerTrust ?? st.trust)}</div>${ring(o.overall, "", 50)}</div>
    </div>`;
  }).join("");
  applyLiquid();
}

function renderByStore(offers) {
  const groups = new Map();
  for (const o of offers) { if (!groups.has(o.storeId)) groups.set(o.storeId, []); groups.get(o.storeId).push(o); }
  const rows = [...groups.entries()];
  if (!rows.length) { $("#list").innerHTML = state.searching ? `<div class="skel"></div><div class="skel"></div>` : ""; return; }
  $("#list").innerHTML = rows.map(([id, list], i) => {
    const st = state.stores[id] || { name: id };
    const best = list[0];
    const region = state.settings.region || "NA";
    const more = list.slice(1, 8);
    return `<div class="row glass" style="animation-delay:${i * 40}ms">
      <div class="rank">#${i + 1}</div>
      <div class="store-cell">
        <div class="name">${esc(st.name)} <span class="pill accent">${esc(st.kind || "")}</span></div>
        <div class="meta">${esc(st.origin || "")} · ${list.length} listing${list.length === 1 ? "" : "s"}${best.sellerName ? ` · seller ${esc(best.sellerName)} (${best.sellerPct}% positive)` : ""}</div>
        <div class="bars">
          ${bar("Trust", best.sellerTrust ?? st.trust)}${bar("Shipping", (st.shipping || {})[region] ?? 3)}
          ${bar("Value", st.price)}${bar("Support", st.support)}
        </div>
      </div>
      <div class="offer-mini" data-url="${esc(best.url)}" style="cursor:pointer">
        ${thumb(best)}
        <div style="min-width:0">
          <div class="p">${priceLine(best)} ${stockPill(best)}</div>
          <div class="t">${esc(best.title)}</div>
          ${more.length ? `<button class="link" data-toggle>+${more.length} more from ${esc(st.name)}</button>` : ""}
        </div>
      </div>
      <div class="row-actions">
        ${ring(best.overall, "score")}
        <button class="btn primary sm" data-url="${esc(best.url)}" style="margin-top:14px">View ↗</button>
      </div>
      ${more.length ? `<div class="more-offers">${more.map(o => `<div class="offer-mini" data-url="${esc(o.url)}">${thumb(o)}
        <div style="min-width:0;flex:1"><div class="p">${priceLine(o)} ${stockPill(o)}</div><div class="t">${esc(o.title)}</div></div>${ring(o.overall, "", 44)}</div>`).join("")}</div>` : ""}
    </div>`;
  }).join("");
}

function renderCards(offers) {
  if (!offers.length) { $("#list").innerHTML = ""; return; }
  $("#list").innerHTML = `<div class="cards">${offers.map((o, i) => {
    const st = state.stores[o.storeId] || { name: o.storeId };
    return `<div class="card glass" data-url="${esc(o.url)}" style="animation-delay:${Math.min(i, 20) * 25}ms">
      <div class="img ${o.image ? "" : "ph"}">${o.image ? `<img loading="lazy" src="${esc(o.image)}" alt="" referrerpolicy="no-referrer" onerror="this.parentNode.classList.add('ph');this.outerHTML='◇'">` : "◇"}</div>
      <div class="tags"><span class="pill accent">${esc(st.name)}</span>${stockPill(o)}${o.condition && o.condition !== "New" ? `<span class="pill warn">${esc(o.condition)}</span>` : ""}</div>
      <div class="t">${esc(o.title)}</div>
      <div class="foot"><div><div class="p">${fmt(o.usd)}</div>${stars(o.sellerTrust ?? st.trust)}</div>${ring(o.overall, "", 46)}</div>
    </div>`;
  }).join("")}</div>`;
}

function storeBaseScore(st) {
  const region = state.settings.region || "NA";
  return st.trust * 0.35 + ((st.shipping || {})[region] ?? 3) * 0.2 + st.price * 0.2 + st.support * 0.15 + st.selection * 0.1;
}

function renderAlso() {
  const withOffers = new Set(state.offers.map(o => o.storeId));
  const list = Object.values(state.stores).filter(st => !withOffers.has(st.id) && st.searchUrl)
    .sort((a, b) => storeBaseScore(b) - storeBaseScore(a));
  $("#alsoHead").classList.toggle("hidden", !list.length);
  $("#also").innerHTML = list.map((st, i) => {
    const s = state.status[st.id];
    let note = "";
    if (s && s.status === "error") note = `<div class="err">Live search failed: ${esc(s.message)}</div>`;
    else if (s && s.status === "skipped") note = `<div class="err">${esc(s.message)}</div>`;
    else if (s && s.status === "empty") note = `<div class="err" style="color:var(--faint)">Live search found no exact match</div>`;
    else if (s && s.status === "loading") note = `<div class="err" style="color:var(--accent)">Searching…</div>`;
    return `<div class="also-card glass" style="animation-delay:${i * 25}ms">
      <div class="top"><span class="n">${esc(st.name)}</span>${stars(st.trust)}</div>
      <div><span class="pill subtle">${esc(st.kind)}</span></div>
      <div class="bf">${esc(st.bestFor)}</div>${note}
      <button class="btn ghost sm" data-url="${esc(searchURL(st, state.query))}">Search ${esc(st.name)} ↗</button>
    </div>`;
  }).join("");
}

// ---------- directory ----------
function renderDirectory() {
  const region = state.settings.region || "NA";
  const list = Object.values(state.stores).sort((a, b) => storeBaseScore(b) - storeBaseScore(a));
  const liveLabel = { live: ["Live search", "good"], api: ["Live with API key", "accent"], beta: ["Live (beta)", "warn"] };
  $("#storeGrid").innerHTML = list.map((st, i) => {
    const lv = liveLabel[st.live] || ["Link only", "subtle"];
    const score = Math.round(storeBaseScore(st) / 5 * 100);
    return `<div class="dir-card glass" style="animation-delay:${i * 25}ms">
      <div class="head"><div><div class="n">${esc(st.name)}</div><div class="muted tiny">${esc(st.domain)} · ships from ${esc(st.origin)}</div>
        <div style="margin-top:8px;display:flex;gap:6px;flex-wrap:wrap"><span class="pill accent">${esc(st.kind)}</span><span class="pill ${lv[1]}">${lv[0]}</span></div></div>
        ${ring(score, "rating")}</div>
      <div class="bf"><b>Best for:</b> ${esc(st.bestFor)}</div>
      <div class="notes">${esc(st.notes)}</div>
      <div class="bars">${bar("Trust", st.trust)}${bar("Value", st.price)}${bar("Selection", st.selection)}${bar("Support", st.support)}${bar("Shipping", (st.shipping || {})[region] ?? 3)}</div>
      <div class="actions"><button class="btn ghost sm" data-url="https://${esc(st.domain)}">Visit ${esc(st.name)} ↗</button></div>
    </div>`;
  }).join("");
}

// ---------- settings ----------
function renderSettings() {
  const s = state.settings;
  $("#setRegion").value = s.region || "NA";
  $("#setCurrency").innerHTML = CURRENCIES.map(c => `<option ${c === (s.currency || "USD") ? "selected" : ""}>${c}</option>`).join("");
  $("#setMouser").value = s.mouserKey || "";
  $("#setDkId").value = s.digikeyId || ""; $("#setDkSecret").value = s.digikeySecret || "";
  $("#setEbayId").value = s.ebayId || ""; $("#setEbaySecret").value = s.ebaySecret || "";
  $("#fxNote").textContent = `Exchange rates: ${state.meta.fxSource}.`;
  const dis = new Set(s.disabled || []);
  $("#srcToggles").innerHTML = Object.values(state.stores).filter(st => st.live).map(st =>
    `<label class="toggle"><input type="checkbox" data-src="${st.id}" ${dis.has(st.id) ? "" : "checked"}><span></span>${esc(st.name)}
     ${state.meta.needsSetup[st.id] ? `<span class="pill subtle">needs key</span>` : ""}</label>`).join("");
}

async function saveSettings() {
  const disabled = $$("#srcToggles input").filter(i => !i.checked).map(i => i.dataset.src);
  const s = {
    region: $("#setRegion").value, currency: $("#setCurrency").value, disabled,
    mouserKey: $("#setMouser").value, digikeyId: $("#setDkId").value, digikeySecret: $("#setDkSecret").value,
    ebayId: $("#setEbayId").value, ebaySecret: $("#setEbaySecret").value,
  };
  try {
    applyMeta(await api("/api/settings", { method: "POST", body: JSON.stringify(s) }));
    renderSettings(); renderAll();
    toast("Settings saved");
  } catch (e) { toast("Couldn't save settings: " + e.message); }
}

function applyMeta(m) {
  state.meta = m; state.settings = m.settings; state.fx = m.fx;
  state.stores = Object.fromEntries(m.stores.map(s => [s.id, s]));
  for (const o of state.offers) o.usd = toUSD(o.price, o.currency);
  $("#regionPill").textContent = `${REGIONS[state.settings.region] || "Region"} · ${state.settings.currency}`;
}

// ---------- weights ----------
function renderWeights() {
  $("#weights").innerHTML = WEIGHT_DEFS.map(([k, label]) =>
    `<label><b>${label}<span>${state.weights[k]}</span></b><input type="range" min="0" max="60" value="${state.weights[k]}" data-w="${k}"></label>`).join("");
}

// ---------- liquid refraction (Chromium: SVG displacement via backdrop-filter) ----------
const liquidSupported = (() => { try { return CSS.supports("backdrop-filter", "url(#x)") && !!window.chrome; } catch { return false; } })();
let liquidId = 0;
function buildDisplacement(w, h, r, bezel) {
  const c = document.createElement("canvas"); c.width = w; c.height = h;
  const ctx = c.getContext("2d"), img = ctx.createImageData(w, h), d = img.data;
  const hw = w / 2, hh = h / 2;
  for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) {
    const px = x + .5 - hw, py = y + .5 - hh;
    const qx = Math.abs(px) - (hw - r), qy = Math.abs(py) - (hh - r);
    let nx, ny, dist;
    if (qx > 0 && qy > 0) { const l = Math.hypot(qx, qy) || 1; dist = r - l; nx = qx / l * Math.sign(px); ny = qy / l * Math.sign(py); }
    else if (qx > qy) { dist = r - qx; nx = Math.sign(px); ny = 0; }
    else { dist = r - qy; nx = 0; ny = Math.sign(py); }
    let m = 0;
    if (dist < bezel) { const t = 1 - Math.max(0, dist) / bezel; m = t * t * (3 - 2 * t); }
    const i = (y * w + x) * 4;
    d[i] = 128 + nx * m * 127; d[i + 1] = 128 + ny * m * 127; d[i + 2] = 128; d[i + 3] = 255;
  }
  ctx.putImageData(img, 0, 0);
  return c.toDataURL();
}
function liquidify(el) {
  const w = Math.round(el.offsetWidth), h = Math.round(el.offsetHeight);
  if (w < 20 || h < 20 || w * h > 700000) return;
  const key = w + "x" + h;
  if (el.dataset.lq === key) return;
  el.dataset.lq = key;
  const r = Math.min(parseFloat(getComputedStyle(el).borderTopLeftRadius) || 20, h / 2, w / 2);
  const id = el.dataset.lqid || ("lq" + (++liquidId)); el.dataset.lqid = id;
  const href = buildDisplacement(w, h, r, Math.min(26, h / 2.2));
  const defs = $("#filters defs");
  let f = document.getElementById(id);
  if (!f) { f = document.createElementNS("http://www.w3.org/2000/svg", "filter"); f.id = id; defs.appendChild(f); }
  f.setAttribute("x", "0"); f.setAttribute("y", "0"); f.setAttribute("width", w); f.setAttribute("height", h);
  f.setAttribute("filterUnits", "userSpaceOnUse"); f.setAttribute("primitiveUnits", "userSpaceOnUse");
  f.setAttribute("color-interpolation-filters", "sRGB");
  f.innerHTML = `<feImage href="${href}" x="0" y="0" width="${w}" height="${h}" result="map" preserveAspectRatio="none"/>
    <feDisplacementMap in="SourceGraphic" in2="map" scale="-48" xChannelSelector="R" yChannelSelector="G"/>`;
  el.style.setProperty("--liquid", `url(#${id})`);
  el.classList.add("liquid-on");
}
const ro = liquidSupported ? new ResizeObserver(es => es.forEach(e => liquidify(e.target))) : null;
function applyLiquid() {
  if (!ro) return;
  $$(".liquid").forEach(el => { if (!el.dataset.lqObs) { el.dataset.lqObs = 1; ro.observe(el); } liquidify(el); });
}

// ---------- events ----------
function bind() {
  $("#searchForm").addEventListener("submit", e => { e.preventDefault(); startSearch($("#q").value); });
  $("#chips").addEventListener("click", e => { const c = e.target.closest(".chip"); if (c) startSearch(c.textContent); });
  $("#nav").addEventListener("click", e => { const b = e.target.closest("button"); if (b) setView(b.dataset.view); });
  $("#modeSeg").addEventListener("click", e => {
    const b = e.target.closest("button"); if (!b) return;
    state.mode = b.dataset.mode; $$("#modeSeg button").forEach(x => x.classList.toggle("active", x === b)); renderAll();
  });
  $("#sortSel").addEventListener("change", e => { state.sort = e.target.value; renderAll(); });
  $("#stockOnly").addEventListener("change", e => { state.stockOnly = e.target.checked; renderAll(); });
  $("#weightsBtn").addEventListener("click", () => $("#weights").classList.toggle("hidden"));
  $("#weights").addEventListener("input", e => {
    const k = e.target.dataset.w; if (!k) return;
    state.weights[k] = +e.target.value; e.target.previousElementSibling.querySelector("span").textContent = e.target.value;
    saveLS("ef.weights", state.weights); renderAll();
  });
  $("#saveBtn").addEventListener("click", saveSettings);
  document.addEventListener("click", e => {
    const ext = e.target.closest("[data-ext]");
    if (ext) { e.preventDefault(); openExt(ext.dataset.ext); return; }
    const tog = e.target.closest("[data-toggle]");
    if (tog) { e.stopPropagation(); tog.closest(".row").classList.toggle("open"); return; }
    const u = e.target.closest("[data-url]");
    if (u) { e.preventDefault(); openExt(u.dataset.url); toast("Opening in your browser…", 1500); }
  });
  document.addEventListener("keydown", e => {
    if ((e.key === "/" || (e.ctrlKey && e.key === "k")) && document.activeElement.tagName !== "INPUT") { e.preventDefault(); setView("search"); $("#q").focus(); }
  });
  window.addEventListener("resize", moveThumb);
}

async function init() {
  bind(); renderChips(); renderWeights();
  try { applyMeta(await api("/api/meta")); }
  catch { toast("Couldn't connect to the app backend. Restart the app.", 8000); return; }
  requestAnimationFrame(() => { moveThumb(); applyLiquid(); });
  setInterval(() => api("/api/ping").catch(() => {}), 10000); api("/api/ping").catch(() => {});
  $("#q").focus();
}
init();
})();
