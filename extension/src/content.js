const HOST_ID = "yg-wrap";

const PANEL_CSS = `
  :host { all: initial; }
  * { box-sizing: border-box; }
  .panel {
    position: fixed;
    z-index: 2147483000;
    width: 320px;
    padding: 14px;
    border-radius: 20px;
    color: #1d1d1f;
    background: rgba(255, 255, 255, 0.9);
    box-shadow: 0 18px 50px rgba(0, 0, 0, 0.16), inset 0 0 0 1px rgba(0, 0, 0, 0.06);
    backdrop-filter: blur(22px);
    font: 13px/1.4 -apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Tahoma, sans-serif;
  }
  .panel.dark {
    color: #f5f5f7;
    background: rgba(32, 32, 34, 0.92);
    box-shadow: 0 18px 50px rgba(0, 0, 0, 0.45), inset 0 0 0 1px rgba(255, 255, 255, 0.08);
  }
  header { display: flex; align-items: flex-start; justify-content: space-between; gap: 8px; }
  .title {
    margin: 0;
    font-size: 14px;
    font-weight: 650;
    letter-spacing: -0.02em;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  button { font: inherit; cursor: pointer; }
  .icon {
    width: 28px; height: 28px; border: 0; border-radius: 50%;
    background: rgba(120, 120, 128, 0.16); color: inherit;
  }
  label { display: flex; flex-direction: column; gap: 6px; margin: 12px 0; color: #6e6e73; font-size: 12px; }
  .panel.dark label { color: #98989d; }
  select, .primary, .secondary, .ghost {
    border: 0; border-radius: 12px; height: 36px;
  }
  select {
    background: rgba(120, 120, 128, 0.12);
    color: inherit;
    padding: 0 10px;
  }
  .quick, .buttons { display: flex; flex-direction: column; gap: 8px; }
  .quick { margin-top: 12px; }
  .quick button, .buttons button { width: 100%; }
  .primary { background: #007aff; color: white; font-weight: 650; }
  .panel.dark .primary { background: #0a84ff; }
  .secondary, .ghost { background: rgba(120, 120, 128, 0.14); color: inherit; }
  .status, .key { margin: 10px 0 0; min-height: 1.2em; }
  .status { color: #6e6e73; }
  .panel.dark .status { color: #98989d; }
  .key { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .key[hidden] { display: none; }
  .mono { font-variant-numeric: tabular-nums; letter-spacing: 0.01em; }
  button:disabled { opacity: 0.5; cursor: default; }
  .veil {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 12px 4px 2px;
    text-align: center;
  }
  .veil[hidden], .form[hidden] { display: none; }
  .veil p { margin: 0; font-weight: 600; }
  .spin {
    width: 22px;
    height: 22px;
    border: 2.5px solid rgba(0, 122, 255, 0.25);
    border-top-color: #007aff;
    border-radius: 50%;
    animation: yg-spin 0.7s linear infinite;
  }
  .panel.dark .spin { border-top-color: #0a84ff; }
  @keyframes yg-spin { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) {
    .spin { animation: none; }
  }
`;

function t(key) { return globalThis.YG && YG.t ? YG.t(key) : key; }

function videoId() {
  const url = new URL(location.href);
  if (url.pathname === "/watch") return url.searchParams.get("v") || "";
  const shorts = url.pathname.match(/^\/shorts\/([A-Za-z0-9_-]{6,})/);
  return shorts ? shorts[1] : "";
}

function pageURL() {
  const id = videoId();
  if (location.pathname.startsWith("/shorts/")) return "https://www.youtube.com/shorts/" + id;
  return "https://www.youtube.com/watch?v=" + id;
}

function pageTitle() {
  const el = document.querySelector("h1.ytd-watch-metadata yt-formatted-string")
    || document.querySelector("h1.ytd-watch-metadata")
    || document.querySelector("#shorts-title")
    || document.querySelector("h1");
  return (el && el.textContent || "").trim();
}

function isDark() {
  const root = document.documentElement;
  if (root.hasAttribute("dark")) return true;
  const app = document.querySelector("ytd-app");
  if (app && (app.hasAttribute("dark") || app.hasAttribute("darker-dark-theme"))) return true;
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

function formatBytes(n) {
  const value = Number(n) || 0;
  if (value <= 0) return "";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = value;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  return (v >= 10 ? v.toFixed(0) : v.toFixed(1)) + " " + units[i];
}

function callAPI(path, method, body, timeoutMs) {
  return new Promise((resolve) => {
    let done = false;
    const finish = (res) => {
      if (done) return;
      done = true;
      resolve(res);
    };
    const timer = setTimeout(() => {
      finish({ ok: false, status: 0, code: "unreachable", error: t("unreachable") });
    }, timeoutMs || 15000);
    chrome.runtime.sendMessage({ type: "yg-api", path, method, body }, (res) => {
      clearTimeout(timer);
      if (chrome.runtime.lastError) {
        finish({ ok: false, status: 0, code: "unreachable", error: chrome.runtime.lastError.message });
        return;
      }
      finish(res || { ok: false, status: 0, code: "unreachable", error: "no response" });
    });
  });
}

function findAnchor() {
  return document.querySelector("ytd-watch-metadata #actions #top-level-buttons-computed")
    || document.querySelector("ytd-watch-metadata #actions-inner")
    || document.querySelector("#actions #top-level-buttons-computed");
}

let mountAbort = null;

function ensure() {
  const id = videoId();
  const existing = document.getElementById(HOST_ID);
  if (!id) {
    if (existing) existing.remove();
    if (mountAbort) mountAbort.abort();
    return;
  }
  if (existing && existing.dataset.videoId === id) {
    place(existing);
    return;
  }
  if (existing) existing.remove();
  if (mountAbort) mountAbort.abort();
  mount(id);
}

function place(wrap) {
  const anchor = findAnchor();
  if (anchor) {
    wrap.classList.remove("yg-float");
    if (wrap.parentElement !== anchor) anchor.insertBefore(wrap, anchor.firstChild);
    return;
  }
  wrap.classList.add("yg-float");
  if (wrap.parentElement !== document.body) document.body.appendChild(wrap);
}

function mount(id) {
  mountAbort = new AbortController();
  const signal = mountAbort.signal;
  const wrap = document.createElement("div");
  wrap.id = HOST_ID;
  wrap.dataset.videoId = id;
  const btn = document.createElement("button");
  btn.id = "yg-btn";
  btn.type = "button";
  btn.innerHTML = '<span class="yg-spin" hidden></span><svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path fill="currentColor" d="M11 3a1 1 0 0 1 2 0v9.17l2.3-2.3a1 1 0 0 1 1.4 1.42l-4 4a1 1 0 0 1-1.4 0l-4-4a1 1 0 1 1 1.4-1.42L11 12.17V3zM5 18a1 1 0 0 1 1 1v1h12v-1a1 1 0 1 1 2 0v2a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-2a1 1 0 0 1 1-1z"/></svg><span class="yg-label"></span>';
  btn.querySelector(".yg-label").textContent = t("download");
  const host = document.createElement("div");
  host.id = "yg-panel";
  const shadow = host.attachShadow({ mode: "open" });
  shadow.innerHTML = `
    <style>${PANEL_CSS}</style>
    <div class="panel" hidden dir="${YG.dir ? YG.dir() : "ltr"}">
      <header>
        <p class="title"></p>
        <button class="icon" type="button" data-act="close" aria-label="${t("close")}">×</button>
      </header>
      <div class="quick">
        <button class="primary" type="button" data-act="video" data-quality="best"></button>
        <button class="secondary" type="button" data-act="audio" data-quality="best"></button>
        <button class="secondary" type="button" data-act="video" data-quality="car"></button>
      </div>
      <div class="veil" hidden>
        <span class="spin" aria-hidden="true"></span>
        <p></p>
      </div>
      <div class="form" hidden>
        <label><span></span><select></select></label>
        <div class="buttons">
          <button class="ghost" type="button" data-act="video" data-quality="pick"></button>
          <button class="ghost" type="button" data-act="refresh"></button>
        </div>
      </div>
      <p class="status" role="status"></p>
    </div>`;
  const panel = shadow.querySelector(".panel");
  const title = shadow.querySelector(".title");
  const select = shadow.querySelector("select");
  const label = shadow.querySelector("label span");
  const status = shadow.querySelector(".status");
  const pillLabel = btn.querySelector(".yg-label");
  const pillIcon = btn.querySelector("svg");
  const heldPct = new Map();
  let currentJob = null;
  let formatsTicket = 0;
  let sending = false;

  function paint() {
    panel.classList.toggle("dark", isDark());
    label.textContent = t("quality");
    shadow.querySelector('[data-quality="best"][data-act="video"]').textContent = t("bestVideo");
    shadow.querySelector('[data-act="audio"]').textContent = t("audio");
    shadow.querySelector('[data-quality="car"]').textContent = t("car");
    shadow.querySelector('[data-quality="pick"]').textContent = t("thisQuality");
    shadow.querySelector('[data-act="refresh"]').textContent = t("refresh");
    shadow.querySelector('[data-act="close"]').setAttribute("aria-label", t("close"));
    panel.dir = YG.dir ? YG.dir() : "ltr";
    renderPill();
  }

  const veil = shadow.querySelector(".veil");
  const veilText = shadow.querySelector(".veil p");
  const form = shadow.querySelector(".form");
  const pillSpin = btn.querySelector(".yg-spin");

  function setStatus(text) { status.textContent = text || ""; }

  function shownPct(item) {
    const raw = Math.max(0, Math.min(100, Number(item.progress) || 0));
    if (!item.id || (item.status !== "downloading" && item.status !== "paused" && item.status !== "queued")) {
      if (item.id) heldPct.delete(item.id);
      return raw;
    }
    let held = heldPct.get(item.id) || 0;
    if (raw > held) held = raw;
    if (item.status === "downloading" && held > 99) held = 99;
    heldPct.set(item.id, held);
    return held;
  }

  function renderPill() {
    const item = currentJob;
    const active = item && (item.status === "downloading" || item.status === "queued" || item.status === "paused");
    pillSpin.hidden = !active || item.status === "paused";
    pillIcon.hidden = active;
    btn.classList.toggle("loading", Boolean(active && item.status !== "paused"));
    let text = t("download");
    if (item && item.status === "downloading") text = t("downloading") + " " + Math.round(shownPct(item)) + "%";
    else if (item && item.status === "queued") text = t("queued");
    else if (item && item.status === "paused") text = t("paused") + " " + Math.round(shownPct(item)) + "%";
    else if (item && item.status === "completed") text = t("completed");
    else if (item && item.status === "failed") text = t("failed");
    pillLabel.textContent = text;
    btn.setAttribute("aria-label", text);
  }

  function pickJob(items) {
    const rank = { downloading: 0, queued: 1, paused: 2, failed: 3, completed: 4 };
    const mine = (items || []).filter((item) => {
      if (!item || item.status === "cancelled") return false;
      if (item.video_id === id) return true;
      return String(item.url || "").indexOf(id) !== -1;
    });
    mine.sort((a, b) => (rank[a.status] ?? 9) - (rank[b.status] ?? 9));
    return mine[0] || null;
  }

  async function refreshPill() {
    const res = await callAPI("/api/v1/downloads?limit=30", "GET", null, 8000);
    if (signal.aborted || !res.ok) return;
    currentJob = pickJob(res.data && res.data.items);
    renderPill();
  }

  function addOption(value, text) {
    const opt = document.createElement("option");
    opt.value = value;
    opt.textContent = text;
    select.appendChild(opt);
  }

  function fill(qualities) {
    select.replaceChildren();
    const heights = (qualities || []).filter((q) => q && q.id && q.id !== "best" && q.id !== "car");
    heights.forEach((q) => {
      const size = formatBytes(q.filesize);
      const approx = q.filesize_approx && size ? "~" : "";
      addOption(q.id, (q.label || q.id) + (size ? " · " + approx + size : ""));
    });
    if (!heights.length) {
      ["2160", "1440", "1080", "720", "480", "360"].forEach((h) => addOption(h, h + "p"));
    }
  }

  function position() {
    const rect = btn.getBoundingClientRect();
    const width = 320;
    const left = Math.min(Math.max(8, rect.left), window.innerWidth - width - 8);
    panel.style.top = Math.min(rect.bottom + 8, window.innerHeight - 16) + "px";
    panel.style.left = left + "px";
  }

  async function loadFormats() {
    const ticket = ++formatsTicket;
    veil.hidden = false;
    veilText.textContent = t("fetching");
    form.hidden = true;
    title.textContent = pageTitle() || id;
    const res = await callAPI("/api/v1/formats", "POST", { url: pageURL() }, 30000);
    if (ticket !== formatsTicket || signal.aborted) return;
    veil.hidden = true;
    fill(res.ok ? (res.data && res.data.qualities) : []);
    form.hidden = false;
    if (!res.ok) {
      setStatus(res.status === 401 ? t("needKey") : (res.error || t("unreachable")));
      return;
    }
    title.textContent = (res.data && res.data.title) || pageTitle() || id;
    setStatus("");
  }

  function openPanel() {
    paint();
    panel.hidden = false;
    btn.setAttribute("aria-expanded", "true");
    position();
    if (form.hidden) loadFormats();
  }

  function closePanel() {
    panel.hidden = true;
    btn.setAttribute("aria-expanded", "false");
  }

  async function queueDownload(kind, quality) {
    if (sending) return;
    sending = true;
    setStatus("");
    const res = await callAPI("/api/v1/downloads", "POST", {
      url: pageURL(),
      kind,
      quality,
      title: pageTitle()
    }, 20000);
    sending = false;
    if (signal.aborted) return;
    if (!res.ok) {
      setStatus(res.status === 401 ? t("needKey") : (res.error || t("unreachable")));
      return;
    }
    currentJob = (res.data && res.data.download) || { status: "queued", progress: 0, video_id: id };
    renderPill();
    closePanel();
    refreshPill();
  }

  btn.addEventListener("click", (event) => {
    event.preventDefault();
    event.stopPropagation();
    if (panel.hidden) openPanel();
    else closePanel();
  });

  shadow.addEventListener("click", (event) => {
    const button = event.target.closest("button");
    if (!button || button.disabled) return;
    const act = button.dataset.act;
    if (act === "close") { closePanel(); return; }
    if (act === "refresh") { loadFormats(); return; }
    if (act !== "video" && act !== "audio") return;
    const quality = button.dataset.quality === "pick" ? select.value : button.dataset.quality;
    queueDownload(act, quality || "best");
  });

  document.addEventListener("click", (event) => {
    if (panel.hidden) return;
    if (wrap.contains(event.target)) return;
    closePanel();
  }, { capture: true, signal });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") closePanel();
  }, { signal });

  window.addEventListener("resize", () => { if (!panel.hidden) position(); }, { signal });
  window.addEventListener("scroll", () => { if (!panel.hidden) position(); }, { capture: true, signal });

  wrap.append(btn, host);
  place(wrap);
  paint();
  refreshPill();
  const poll = setInterval(refreshPill, 1200);
  signal.addEventListener("abort", () => clearInterval(poll));
}

ensure();
document.addEventListener("yt-navigate-finish", ensure);
setInterval(ensure, 1500);
