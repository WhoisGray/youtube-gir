(function (root) {
  const DEFAULTS = {
    serverUrl: "http://127.0.0.1:6543",
    apiKey: ""
  };

  function trimURL(url) {
    return String(url || "").replace(/\/+$/, "");
  }

  async function settings() {
    return chrome.storage.local.get(DEFAULTS);
  }

  async function request(path, options) {
    const cfg = await settings();
    const method = (options && options.method) || "GET";
    const headers = { Accept: "application/json" };
    if (cfg.apiKey) headers.Authorization = "Bearer " + cfg.apiKey;
    let body;
    if (options && options.body != null) {
      headers["Content-Type"] = "application/json";
      body = JSON.stringify(options.body);
    }
    if (!cfg.serverUrl) {
      return { ok: false, status: 0, code: "need_key", error: "missing server" };
    }
    let res;
    try {
      res = await fetch(trimURL(cfg.serverUrl) + path, { method, headers, body });
    } catch (err) {
      return { ok: false, status: 0, code: "unreachable", error: String(err && err.message || err) };
    }
    if (res.status === 204) return { ok: true, status: 204, data: null };
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      return {
        ok: false,
        status: res.status,
        error: data.error || res.statusText,
        code: data.code || "error",
        data
      };
    }
    return { ok: true, status: res.status, data };
  }

  root.YG = root.YG || {};
  root.YG.settings = settings;
  root.YG.request = request;
  root.YG.DEFAULTS = DEFAULTS;
  root.YG.trimURL = trimURL;
})(globalThis);
