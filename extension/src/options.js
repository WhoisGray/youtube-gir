const $ = (id) => document.getElementById(id);

function applyLocale() {
  document.documentElement.lang = YG.locale() === "fa" ? "fa" : "en";
  document.documentElement.dir = YG.dir();
  document.title = YG.t("settings") + " · youtube-gir";
  $("title").textContent = YG.t("extName");
  $("note").textContent = YG.t("languageNote");
  $("tab-connection").textContent = YG.t("connection");
  $("tab-admin").textContent = YG.t("admin");
  $("label-server").textContent = YG.t("server");
  $("label-key").textContent = YG.t("apiKey");
  $("test").textContent = YG.t("test");
  $("save").textContent = YG.t("saveSettings");
  $("help").textContent = YG.t("help");
  $("username").placeholder = YG.t("username");
  $("generate").textContent = YG.t("generate");
  $("users-title").textContent = YG.t("users");
  $("label-proxy").textContent = YG.t("proxyLabel");
  $("proxy-hint").textContent = YG.t("proxyHint");
  $("proxy-save").textContent = YG.t("proxySave");
}

function messageFor(res) {
  if (!res || res.code === "unreachable" || res.status === 0) return YG.t("unreachable");
  if (res.status === 401 || res.code === "unauthorized" || res.code === "need_key") return YG.t("needKey");
  if (res.status === 403) return YG.t("denied");
  return res.error || YG.t("unreachable");
}

function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;"
  }[c]));
}

function showTab(name) {
  const admin = name === "admin";
  $("form").hidden = admin;
  $("admin-panel").hidden = !admin;
  $("tab-connection").classList.toggle("on", !admin);
  $("tab-admin").classList.toggle("on", admin);
  $("tab-connection").setAttribute("aria-selected", admin ? "false" : "true");
  $("tab-admin").setAttribute("aria-selected", admin ? "true" : "false");
  if (admin) loadAdmin();
}

function setStatus(text, ok) {
  const el = $("status");
  el.textContent = text || "";
  el.style.color = ok ? "var(--ok)" : text ? "var(--danger)" : "";
}

function parseServer(value) {
  let url;
  try {
    url = new URL(String(value || "").trim());
  } catch (_err) {
    return null;
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") return null;
  return url;
}

function isLocal(url) {
  return url.hostname === "localhost" || url.hostname === "127.0.0.1";
}

async function grant(url) {
  if (isLocal(url)) return true;
  const origin = url.origin + "/*";
  const has = await chrome.permissions.contains({ origins: [origin] });
  if (has) return true;
  return chrome.permissions.request({ origins: [origin] });
}

async function load() {
  const cfg = await YG.settings();
  $("server").value = cfg.serverUrl;
  $("key").value = cfg.apiKey;
}

async function call(serverUrl, apiKey, path) {
  try {
    const res = await fetch(YG.trimURL(serverUrl) + path, {
      headers: apiKey ? { Authorization: "Bearer " + apiKey, Accept: "application/json" } : { Accept: "application/json" }
    });
    const data = await res.json().catch(() => ({}));
    return { ok: res.ok, status: res.status, data, error: data.error };
  } catch (_err) {
    return { ok: false, status: 0 };
  }
}

async function testConnection() {
  const url = parseServer($("server").value);
  if (!url) {
    setStatus(YG.t("badUrl"), false);
    return;
  }
  const allowed = await grant(url);
  if (!allowed) {
    setStatus(YG.t("permission"), false);
    return;
  }
  const health = await call(url.origin, "", "/health");
  if (health.status === 0) {
    setStatus(YG.t("unreachable"), false);
    return;
  }
  const me = await call(url.origin, $("key").value.trim(), "/api/v1/me");
  if (!me.ok) {
    setStatus(me.status === 401 ? YG.t("needKey") : (me.error || YG.t("testFail")), false);
    return;
  }
  const role = me.data.role === "admin" ? YG.t("admin") : me.data.role;
  setStatus(YG.t("testOk") + " · " + me.data.username + " · " + role, true);
  return me.data.role === "admin";
}

async function revealAdmin(isAdmin) {
  $("tabs").hidden = !isAdmin;
  if (!isAdmin) showTab("connection");
}

async function loadUsers() {
  const res = await YG.request("/api/v1/users");
  if (!res.ok) return;
  $("users").innerHTML = (res.data.users || []).map((u) =>
    `<li><span>${esc(u.username)}</span><span class="muted">${esc(u.role)} · ${esc(u.downloads)}</span></li>`
  ).join("");
}

async function loadProxy() {
  const res = await YG.request("/api/v1/system/proxy");
  if (!res.ok) return;
  const input = $("proxy");
  if (document.activeElement !== input) input.value = res.data.proxy || "";
}

async function loadAdmin() {
  await Promise.all([loadUsers(), loadProxy()]);
}

async function refreshRole() {
  const me = await YG.request("/api/v1/me");
  await revealAdmin(me.ok && me.data.role === "admin");
}

$("form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const url = parseServer($("server").value);
  if (!url) {
    setStatus(YG.t("badUrl"), false);
    return;
  }
  const allowed = await grant(url);
  if (!allowed) {
    setStatus(YG.t("permission"), false);
    return;
  }
  await chrome.storage.local.set({
    serverUrl: url.origin,
    apiKey: $("key").value.trim()
  });
  $("server").value = url.origin;
  const isAdmin = await testConnection();
  await revealAdmin(isAdmin === true);
});

$("test").addEventListener("click", () => testConnection());

$("tab-connection").addEventListener("click", () => showTab("connection"));
$("tab-admin").addEventListener("click", () => showTab("admin"));

$("key-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const username = $("username").value.trim();
  if (!username) return;
  $("generate").disabled = true;
  const res = await YG.request("/api/v1/users", { method: "POST", body: { username } });
  $("generate").disabled = false;
  const box = $("new-key");
  box.hidden = false;
  if (!res.ok) {
    box.textContent = messageFor(res);
    return;
  }
  box.textContent = res.data.username + " · " + res.data.token;
  $("username").value = "";
  try { await navigator.clipboard.writeText(res.data.token); } catch (_err) { /* shown on screen */ }
  loadUsers();
});

$("proxy-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const button = $("proxy-save");
  const status = $("proxy-status");
  button.disabled = true;
  status.textContent = "";
  const res = await YG.request("/api/v1/system/proxy", {
    method: "PUT",
    body: { proxy: $("proxy").value.trim() }
  });
  button.disabled = false;
  if (!res.ok) {
    status.textContent = messageFor(res);
    return;
  }
  $("proxy").value = res.data.proxy || "";
  status.textContent = res.data.proxy ? YG.t("proxySaved") : YG.t("proxyCleared");
});

applyLocale();
load().then(refreshRole);
