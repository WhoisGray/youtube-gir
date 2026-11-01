const $ = (id) => document.getElementById(id);
const pendingDelete = new Map();
const heldPct = new Map();
let scope = "mine";
let timer = 0;

function applyLocale() {
  document.documentElement.lang = YG.locale() === "fa" ? "fa" : "en";
  document.documentElement.dir = YG.dir();
  $("title").textContent = YG.t("extName");
  $("settings").setAttribute("aria-label", YG.t("settings"));
  $("label-count").textContent = YG.t("downloads");
  $("label-size").textContent = YG.t("totalSize");
  $("label-active").textContent = YG.t("active");
  $("empty-title").textContent = YG.t("emptyTitle");
  $("empty-hint").textContent = YG.t("emptyHint");
  $("scope-mine").textContent = YG.t("mine");
  $("scope-all").textContent = YG.t("everyone");
  $("who").textContent = YG.t("notConnected");
}

function formatBytes(n) {
  const value = Number(n) || 0;
  if (value <= 0) return "0 MB";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = value;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  const digits = v >= 10 || i === 0 ? 0 : 1;
  return v.toFixed(digits) + " " + units[i];
}

function banner(text) {
  const el = $("banner");
  if (!text) {
    el.hidden = true;
    el.textContent = "";
    return;
  }
  el.hidden = false;
  el.textContent = text;
}

function messageFor(res) {
  if (!res) return YG.t("unreachable");
  if (res.code === "unreachable" || res.status === 0) return YG.t("unreachable");
  if (res.status === 401 || res.code === "unauthorized" || res.code === "need_key") return YG.t("needKey");
  if (res.status === 403) return YG.t("denied");
  return res.error || YG.t("unreachable");
}

function actionsFor(item) {
  const acts = [];
  if (item.status === "downloading") acts.push(["pause", "pause"], ["cancel", "cancel"]);
  else if (item.status === "queued") acts.push(["cancel", "cancel"]);
  else if (item.status === "paused" || item.status === "failed") acts.push(["resume", "resume"], ["cancel", "cancel"]);
  if (item.status === "completed") acts.push(["save", "save"], ["link", "copyLink"]);
  if (item.file_token) acts.push(["key", "copyKey"]);
  const confirm = pendingDelete.get(item.id) > Date.now();
  acts.push([confirm ? "confirm-delete" : "delete", confirm ? "confirm" : "remove"]);
  return acts;
}

function buttonClass(act) {
  if (act === "save" || act === "confirm-delete") return "primary";
  if (act === "delete" || act === "cancel") return "danger";
  return "ghost";
}

function shownPercent(item) {
  const raw = Math.max(0, Math.min(100, Number(item.progress) || 0));
  if (item.status !== "downloading" && item.status !== "paused") {
    heldPct.delete(item.id);
    return raw;
  }
  let held = heldPct.get(item.id) || 0;
  if (raw > held) held = raw;
  if (item.status === "downloading" && held > 99) held = 99;
  heldPct.set(item.id, held);
  return held;
}

function itemMeta(item) {
  const bits = [YG.t(item.kind) || item.kind];
  if (item.kind === "video" && item.quality) {
    if (item.quality === "best") bits.push(YG.t("best"));
    else if (item.quality === "car") bits.push(YG.t("car"));
    else if (/^\d{3,4}$/.test(item.quality)) bits.push(item.quality + "p");
    else bits.push(item.quality);
  }
  if (scope === "all" && item.username) bits.push(item.username);
  return bits.filter(Boolean).join(" · ");
}

function itemSub(item, pct) {
  if (item.status === "downloading") {
    const speed = item.speed_bps ? formatBytes(item.speed_bps) + "/s" : "";
    return Math.round(pct) + "%" + (speed ? " · " + speed : "") + (item.eta ? " · " + item.eta : "");
  }
  if (item.status === "completed") return formatBytes(item.file_size);
  if (item.status === "failed" && item.error) return item.error;
  return YG.t(item.status) || item.status;
}

function buildCard() {
  const li = document.createElement("li");
  li.className = "item enter";
  const top = document.createElement("div");
  top.className = "item-top";
  const name = document.createElement("div");
  name.className = "name";
  const pill = document.createElement("span");
  pill.className = "pill";
  top.append(name, pill);
  const sub = document.createElement("p");
  sub.className = "sub";
  const meta = document.createElement("div");
  meta.className = "meta";
  const actions = document.createElement("div");
  actions.className = "actions";
  li.append(top, sub, meta, actions);
  li.addEventListener("animationend", () => li.classList.remove("enter"));
  return li;
}

function syncCard(li, item) {
  li.dataset.id = item.id;
  li.dataset.token = item.file_token || "";
  li.dataset.name = item.file_name || "";

  const title = item.title || item.video_id || item.url || "";
  const top = li.querySelector(".item-top");
  let name = top.querySelector(".name");
  const wantLink = item.status === "completed";
  if ((name.tagName === "A") !== wantLink) {
    const next = document.createElement(wantLink ? "a" : "div");
    next.className = "name";
    name.replaceWith(next);
    name = next;
  }
  if (wantLink) {
    name.href = item.url || "";
    name.target = "_blank";
    name.rel = "noreferrer";
  }
  if (name.textContent !== title) name.textContent = title;

  const pill = top.querySelector(".pill");
  if (pill.dataset.status !== item.status) {
    pill.dataset.status = item.status || "";
    pill.className = "pill" + (item.status ? " " + item.status : "");
    pill.textContent = YG.t(item.status) || item.status || "";
  }

  const pct = shownPercent(item);
  const showBar = item.status === "downloading" || (item.status === "paused" && pct > 0);
  let bar = li.querySelector(".bar");
  if (showBar) {
    if (!bar) {
      bar = document.createElement("div");
      bar.className = "bar";
      bar.setAttribute("aria-hidden", "true");
      bar.appendChild(document.createElement("span"));
      li.querySelector(".sub").before(bar);
    }
    const width = pct + "%";
    if (bar.firstElementChild.style.width !== width) bar.firstElementChild.style.width = width;
  } else if (bar) {
    bar.remove();
  }

  const sub = itemSub(item, pct);
  const subEl = li.querySelector(".sub");
  if (subEl.textContent !== sub) subEl.textContent = sub;

  const meta = itemMeta(item);
  const metaEl = li.querySelector(".meta");
  if (metaEl.textContent !== meta) metaEl.textContent = meta;

  const acts = actionsFor(item);
  const key = acts.map(([act, label]) => act + ":" + label).join("|");
  if (li.dataset.actions !== key) {
    li.dataset.actions = key;
    const box = li.querySelector(".actions");
    box.replaceChildren();
    for (const [act, label] of acts) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = buttonClass(act);
      button.dataset.act = act;
      button.textContent = YG.t(label);
      box.appendChild(button);
    }
  }
}

function render(me, payload) {
  const stats = payload.stats || {};
  $("stat-count").textContent = String(stats.downloads_completed ?? 0);
  $("stat-size").textContent = formatBytes(stats.bytes_downloaded);
  $("stat-active").textContent = String((stats.active || 0) + (stats.queued || 0));
  $("who").textContent = me.username + (me.role === "admin" ? " · " + YG.t("admin") : "");
  $("admin").hidden = me.role !== "admin";

  const items = payload.items || [];
  $("empty").hidden = items.length !== 0;
  $("list").hidden = items.length === 0;
  const list = $("list");
  const byId = new Map();
  for (const li of list.children) {
    if (li.dataset && li.dataset.id) byId.set(li.dataset.id, li);
  }
  const seen = new Set();
  let cursor = null;
  for (const item of items) {
    if (!item || !item.id || seen.has(item.id)) continue;
    seen.add(item.id);
    let li = byId.get(item.id);
    if (!li) {
      li = buildCard();
      byId.set(item.id, li);
    }
    syncCard(li, item);
    const next = cursor ? cursor.nextElementSibling : list.firstElementChild;
    if (next !== li) list.insertBefore(li, next);
    cursor = li;
  }
  for (const [id, li] of byId) {
    if (!seen.has(id)) li.remove();
  }
  for (const id of heldPct.keys()) {
    if (!seen.has(id)) heldPct.delete(id);
  }
}

async function refresh() {
  const me = await YG.request("/api/v1/me");
  if (!me.ok) {
    banner(messageFor(me));
    $("who").textContent = YG.t("notConnected");
    $("empty").hidden = false;
    $("list").hidden = true;
    return;
  }
  banner("");
  const path = "/api/v1/downloads?limit=100" + (scope === "all" ? "&scope=all" : "");
  const list = await YG.request(path);
  if (!list.ok) {
    banner(messageFor(list));
    return;
  }
  render(me.data, list.data);
}

async function copyText(text, button) {
  try {
    await navigator.clipboard.writeText(text);
  } catch (_err) {
    const area = document.createElement("textarea");
    area.value = text;
    document.body.appendChild(area);
    area.select();
    document.execCommand("copy");
    area.remove();
  }
  const previous = button.textContent;
  button.textContent = YG.t("copied");
  setTimeout(() => { button.textContent = previous; }, 1200);
}

$("settings").addEventListener("click", () => chrome.runtime.openOptionsPage());

$("scope-mine").addEventListener("click", () => {
  scope = "mine";
  $("scope-mine").classList.add("on");
  $("scope-all").classList.remove("on");
  refresh();
});

$("scope-all").addEventListener("click", () => {
  scope = "all";
  $("scope-all").classList.add("on");
  $("scope-mine").classList.remove("on");
  refresh();
});

$("list").addEventListener("click", async (event) => {
  const button = event.target.closest("button[data-act]");
  if (!button) return;
  const card = button.closest("[data-id]");
  const id = card.dataset.id;
  const act = button.dataset.act;
  button.disabled = true;
  try {
    if (act === "pause" || act === "resume" || act === "cancel") {
      const res = await YG.request("/api/v1/downloads/" + id + "/" + act, { method: "POST", body: {} });
      if (!res.ok) banner(messageFor(res));
    } else if (act === "delete") {
      pendingDelete.set(id, Date.now() + 4000);
    } else if (act === "confirm-delete") {
      pendingDelete.delete(id);
      const res = await YG.request("/api/v1/downloads/" + id, { method: "DELETE" });
      if (!res.ok) banner(messageFor(res));
    } else if (act === "save") {
      const cfg = await YG.settings();
      await chrome.downloads.download({
        url: YG.trimURL(cfg.serverUrl) + "/api/v1/files/" + card.dataset.token,
        filename: card.dataset.name || undefined,
        saveAs: true
      });
    } else if (act === "link") {
      const cfg = await YG.settings();
      await copyText(YG.trimURL(cfg.serverUrl) + "/api/v1/files/" + card.dataset.token, button);
    } else if (act === "key") {
      await copyText(card.dataset.token, button);
    }
  } finally {
    button.disabled = false;
    refresh();
  }
});

applyLocale();
refresh();
timer = setInterval(refresh, 1200);
window.addEventListener("pagehide", () => clearInterval(timer));
