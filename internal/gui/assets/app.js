(function () {
  const packs = {
    en: {
      settings: "Settings",
      quit: "Quit",
      link: "YouTube link",
      bestVideo: "Best video",
      audio: "Audio",
      video: "Video",
      best: "Best",
      car: "Car",
      carHint: "Car keeps H.264 up to 1080p and AAC audio in an MP4.",
      quality: "Quality",
      thisQuality: "Download this quality",
      refresh: "Refresh qualities",
      fetching: "Fetching qualities…",
      starting: "Adding to the queue…",
      started: "Added to your queue",
      alreadyQueued: "Already in the queue",
      needLink: "Paste a YouTube link first.",
      empty: "Nothing in the queue. Paste a YouTube link and choose Best video, Audio, or Car.",
      finished: "finished",
      active: "active",
      pause: "Pause",
      resume: "Resume",
      cancel: "Cancel",
      save: "Save",
      remove: "Delete",
      confirm: "Confirm",
      queued: "Queued",
      downloading: "Downloading",
      paused: "Paused",
      completed: "Ready",
      failed: "Failed",
      cancelled: "Cancelled",
      copyKey: "Copy file key",
      copyLink: "Copy link",
      copied: "Copied",
      copy: "Copy",
      show: "Show",
      hide: "Hide",
      admin: "Admin",
      mine: "Mine",
      everyone: "Everyone",
      server: "Server URL",
      apiKey: "API key",
      extensionHelp: "Paste this URL and key into the extension options. The extension then uses this app as its server.",
      proxyTitle: "Download proxy",
      proxyLabel: "SOCKS or HTTP proxy",
      proxyHint: "SOCKS5 or HTTP, for example socks5://127.0.0.1:1080. Used only while fetching and downloading a video. The server itself stays direct.",
      proxySave: "Save proxy",
      proxySaved: "Saved. The next download uses this proxy.",
      proxyCleared: "Cleared. The next download connects directly.",
      people: "People",
      username: "Username",
      generate: "Generate key",
      openFolder: "Open downloads folder",
      folderOpened: "Opened the downloads folder.",
      folderFailed: "Could not open the downloads folder.",
      close: "Close",
      missing: "Install",
      missingNext: "before a download can finish.",
      unreachable: "Cannot reach the server.",
      localOnly: "Open this page from the app on this computer.",
      notConnected: "Not connected",
      noPeople: "No people yet.",
      deleteUser: "Delete this person and their downloads?"
    },
    fa: {
      settings: "تنظیمات",
      quit: "خروج",
      link: "لینک یوتیوب",
      bestVideo: "بهترین ویدیو",
      audio: "صدا",
      video: "ویدیو",
      best: "بهترین",
      car: "خودرو",
      carHint: "خودرو تصویر H.264 تا 1080p و صدای AAC را در یک MP4 نگه می‌دارد.",
      quality: "کیفیت",
      thisQuality: "دانلود این کیفیت",
      refresh: "تازه‌سازی کیفیت‌ها",
      fetching: "در حال دریافت کیفیت‌ها…",
      starting: "در حال افزودن به صف…",
      started: "به صف شما اضافه شد",
      alreadyQueued: "از قبل در صف است",
      needLink: "اول یک لینک یوتیوب بچسبانید.",
      empty: "صف خالی است. یک لینک یوتیوب بچسبانید و بهترین ویدیو، صدا، یا خودرو را انتخاب کنید.",
      finished: "تمام‌شده",
      active: "فعال",
      pause: "مکث",
      resume: "ادامه",
      cancel: "لغو",
      save: "ذخیره",
      remove: "حذف",
      confirm: "تأیید",
      queued: "در صف",
      downloading: "در حال دانلود",
      paused: "مکث",
      completed: "آماده",
      failed: "ناموفق",
      cancelled: "لغو شده",
      copyKey: "کپی کلید فایل",
      copyLink: "کپی لینک",
      copied: "کپی شد",
      copy: "کپی",
      show: "نمایش",
      hide: "پنهان",
      admin: "مدیر",
      mine: "مال من",
      everyone: "همه",
      server: "نشانی سرور",
      apiKey: "کلید API",
      extensionHelp: "این نشانی و کلید را در تنظیمات افزونه بچسبانید. افزونه بعد از آن از همین برنامه به‌عنوان سرور استفاده می‌کند.",
      proxyTitle: "پروکسی دانلود",
      proxyLabel: "پروکسی SOCKS یا HTTP",
      proxyHint: "SOCKS5 یا HTTP، مثلاً socks5://127.0.0.1:1080. فقط هنگام دریافت و دانلود ویدیو استفاده می‌شود. خود سرور مستقیم می‌ماند.",
      proxySave: "ذخیره پروکسی",
      proxySaved: "ذخیره شد. دانلود بعدی از این پروکسی می‌رود.",
      proxyCleared: "پاک شد. دانلود بعدی مستقیم وصل می‌شود.",
      people: "کاربران",
      username: "نام کاربر",
      generate: "ساخت کلید",
      openFolder: "باز کردن پوشهٔ دانلودها",
      folderOpened: "پوشهٔ دانلودها باز شد.",
      folderFailed: "پوشهٔ دانلودها باز نشد.",
      close: "بستن",
      missing: "این‌ها را نصب کنید:",
      missingNext: "وگرنه دانلود تمام نمی‌شود.",
      unreachable: "سرور در دسترس نیست.",
      localOnly: "این صفحه را از خود برنامه روی همین رایانه باز کنید.",
      notConnected: "متصل نیست",
      noPeople: "هنوز کاربری نیست.",
      deleteUser: "این کاربر و دانلودهایش حذف شوند؟"
    }
  };

  const $ = (id) => document.getElementById(id);
  const boot = JSON.parse($("boot").textContent || "{}");
  const key = boot.key || "";
  const pendingDelete = new Map();
  const heldPct = new Map();
  let scope = "mine";
  let refreshing = false;
  let formatTicket = 0;
  let timer = 0;

  function locale() {
    return document.documentElement.lang === "fa" ? "fa" : "en";
  }

  function t(name) {
    const pack = packs[locale()] || packs.en;
    return pack[name] || packs.en[name] || name;
  }

  function applyLocale() {
    $("settings").textContent = t("settings");
    $("quit").textContent = t("quit");
    $("url-label").textContent = t("link");
    $("url").placeholder = "https://www.youtube.com/watch?v=";
    $("best").textContent = t("bestVideo");
    $("audio").textContent = t("audio");
    $("car").textContent = t("car");
    $("car-hint").textContent = t("carHint");
    $("quality-label").textContent = t("quality");
    $("pick").textContent = t("thisQuality");
    $("refresh").textContent = t("refresh");
    $("empty").textContent = t("empty");
    $("scope-mine").textContent = t("mine");
    $("scope-all").textContent = t("everyone");
    $("settings-title").textContent = t("settings");
    $("server-label").textContent = t("server");
    $("key-label").textContent = t("apiKey");
    $("copy-server").textContent = t("copy");
    $("copy-key").textContent = t("copy");
    $("reveal-key").textContent = $("api-key").type === "text" ? t("hide") : t("show");
    $("extension-help").textContent = t("extensionHelp");
    $("proxy-title").textContent = t("proxyTitle");
    $("proxy-label").textContent = t("proxyLabel");
    $("proxy-hint").textContent = t("proxyHint");
    $("proxy-save").textContent = t("proxySave");
    $("users-title").textContent = t("people");
    $("user-label").textContent = t("username");
    $("generate").textContent = t("generate");
    $("open-folder").textContent = t("openFolder");
    $("close-settings").textContent = t("close");
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

  async function api(path, opts) {
    const options = opts || {};
    const timeout = options.timeout || 15000;
    const ctrl = new AbortController();
    const timerId = setTimeout(function () { ctrl.abort(); }, timeout);
    try {
      const headers = { Authorization: "Bearer " + key };
      if (options.body) headers["Content-Type"] = "application/json";
      const res = await fetch(path, {
        method: options.method || "GET",
        headers: headers,
        body: options.body ? JSON.stringify(options.body) : undefined,
        signal: ctrl.signal
      });
      let data = {};
      const text = await res.text();
      if (text) {
        try { data = JSON.parse(text); } catch (_err) { data = {}; }
      }
      return { ok: res.ok, status: res.status, data: data, error: data.error || "" };
    } catch (_err) {
      return { ok: false, status: 0, data: {}, error: "" };
    } finally {
      clearTimeout(timerId);
    }
  }

  function setStatus(text) {
    $("form-status").textContent = text || "";
  }

  async function loadHealth() {
    const el = $("tools");
    try {
      const res = await fetch("/health");
      const data = await res.json();
      $("version").textContent = data.version ? "v" + data.version : "";
      const missing = [];
      if (!data.ytdlp) missing.push("yt-dlp");
      if (!data.ffmpeg) missing.push("ffmpeg");
      if (missing.length) {
        el.hidden = false;
        el.textContent = t("missing") + " " + missing.join(", ") + " " + t("missingNext");
      } else {
        el.hidden = true;
        el.textContent = "";
      }
    } catch (_err) {
      el.hidden = false;
      el.textContent = t("unreachable");
    }
  }

  function actionsFor(item) {
    const acts = [];
    if (item.status === "downloading") acts.push(["pause", "pause"], ["cancel", "cancel"]);
    else if (item.status === "queued") acts.push(["cancel", "cancel"]);
    else if (item.status === "paused" || item.status === "failed") acts.push(["resume", "resume"], ["cancel", "cancel"]);
    if (item.status === "completed" && item.file_token) acts.push(["save", "save"], ["link", "copyLink"]);
    if (item.file_token) acts.push(["key", "copyKey"]);
    const confirm = pendingDelete.get(item.id) > Date.now();
    acts.push([confirm ? "confirm-delete" : "delete", confirm ? "confirm" : "remove"]);
    return acts;
  }

  function buttonClass(act) {
    if (act === "save" || act === "confirm-delete") return "btn primary";
    if (act === "delete" || act === "cancel") return "danger";
    return "";
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
    const bits = [t(item.kind) || item.kind];
    if (item.kind === "video" && item.quality) {
      if (item.quality === "best") bits.push(t("best"));
      else if (item.quality === "car") bits.push(t("car"));
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
    return t(item.status) || item.status;
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
    li.addEventListener("animationend", function () { li.classList.remove("enter"); });
    return li;
  }

  function syncCard(li, item) {
    li.dataset.id = item.id;
    li.dataset.token = item.file_token || "";
    const title = item.title || item.video_id || item.url || "";
    const top = li.querySelector(".item-top");
    let name = top.querySelector(".name");
    const wantLink = item.status === "completed" && !!item.url;
    if ((name.tagName === "A") !== wantLink) {
      const next = document.createElement(wantLink ? "a" : "div");
      next.className = "name";
      name.replaceWith(next);
      name = next;
    }
    if (wantLink) {
      name.href = item.url;
      name.target = "_blank";
      name.rel = "noreferrer";
    }
    if (name.textContent !== title) name.textContent = title;

    const pill = top.querySelector(".pill");
    if (pill.dataset.status !== item.status) {
      pill.dataset.status = item.status || "";
      pill.className = "pill" + (item.status ? " " + item.status : "");
      pill.textContent = t(item.status) || item.status || "";
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
    const sig = acts.map(function (pair) { return pair[0] + ":" + pair[1]; }).join("|");
    if (li.dataset.actions !== sig) {
      li.dataset.actions = sig;
      const box = li.querySelector(".actions");
      box.replaceChildren();
      acts.forEach(function (pair) {
        const act = pair[0];
        const label = pair[1];
        if (act === "save") {
          const link = document.createElement("a");
          link.className = "btn primary";
          link.href = "/api/v1/files/" + encodeURIComponent(item.file_token);
          link.download = item.file_name || "";
          link.textContent = t(label);
          box.appendChild(link);
          return;
        }
        const button = document.createElement("button");
        button.type = "button";
        button.className = buttonClass(act);
        button.dataset.act = act;
        button.textContent = t(label);
        box.appendChild(button);
      });
    }
  }

  function render(me, payload) {
    const stats = payload.stats || {};
    const finished = stats.downloads_completed || 0;
    const active = (stats.active || 0) + (stats.queued || 0);
    $("stats").textContent = finished + " " + t("finished") + " · " + formatBytes(stats.bytes_downloaded) + " · " + active + " " + t("active");
    $("who").textContent = me.username + (me.role === "admin" ? " · " + t("admin") : "");
    const admin = me.role === "admin";
    $("scope").hidden = !admin;
    $("admin-tools").hidden = !admin;

    const items = payload.items || [];
    $("empty").hidden = items.length !== 0;
    $("list").hidden = items.length === 0;
    const list = $("list");
    const byId = new Map();
    Array.from(list.children).forEach(function (li) {
      if (li.dataset && li.dataset.id) byId.set(li.dataset.id, li);
    });
    const seen = new Set();
    let cursor = null;
    items.forEach(function (item) {
      if (!item || !item.id || seen.has(item.id)) return;
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
    });
    byId.forEach(function (li, id) {
      if (!seen.has(id)) li.remove();
    });
    Array.from(heldPct.keys()).forEach(function (id) {
      if (!seen.has(id)) heldPct.delete(id);
    });
  }

  async function refresh() {
    if (!key || refreshing) return;
    refreshing = true;
    try {
      const me = await api("/api/v1/me");
      if (!me.ok) {
        $("banner").hidden = false;
        $("banner").textContent = me.error || t("unreachable");
        $("who").textContent = t("notConnected");
        return;
      }
      $("banner").hidden = true;
      $("banner").textContent = "";
      const path = "/api/v1/downloads?limit=100" + (scope === "all" ? "&scope=all" : "");
      const list = await api(path);
      if (!list.ok) {
        $("banner").hidden = false;
        $("banner").textContent = list.error || t("unreachable");
        return;
      }
      render(me.data, list.data);
    } finally {
      refreshing = false;
    }
  }

  async function queue(kind, quality) {
    const url = $("url").value.trim();
    setStatus(t("starting"));
    const res = await api("/api/v1/downloads", {
      method: "POST",
      timeout: 20000,
      body: { url: url, kind: kind, quality: quality }
    });
    if (!res.ok) {
      setStatus(res.error || t("unreachable"));
      return;
    }
    setStatus(res.data && res.data.duplicate ? t("alreadyQueued") : t("started"));
    refresh();
  }

  function fillQualities(qualities) {
    const sel = $("quality");
    sel.replaceChildren();
    const heights = (qualities || []).filter(function (q) {
      return q && q.id && q.id !== "best" && q.id !== "car";
    });
    heights.forEach(function (q) {
      const opt = document.createElement("option");
      opt.value = q.id;
      const size = formatBytes(q.filesize);
      const approx = q.filesize_approx && size ? "~" : "";
      opt.textContent = (q.label || q.id) + (size ? " · " + approx + size : "");
      sel.appendChild(opt);
    });
    if (!heights.length) {
      ["2160", "1440", "1080", "720", "480", "360"].forEach(function (h) {
        const opt = document.createElement("option");
        opt.value = h;
        opt.textContent = h + "p";
        sel.appendChild(opt);
      });
    }
    $("quality-row").hidden = false;
  }

  async function loadFormats(url, announce) {
    const ticket = ++formatTicket;
    if (announce) setStatus(t("fetching"));
    const res = await api("/api/v1/formats", {
      method: "POST",
      timeout: 90000,
      body: { url: url }
    });
    if (ticket !== formatTicket || $("url").value.trim() !== url) return;
    if (!res.ok) {
      if (announce) setStatus(res.error || t("unreachable"));
      return;
    }
    fillQualities(res.data.qualities || []);
    if (announce && $("form-status").textContent === t("fetching")) setStatus("");
  }

  async function copyText(text, button) {
    try {
      await navigator.clipboard.writeText(text);
    } catch (_err) {
      const area = document.createElement("textarea");
      area.value = text;
      area.setAttribute("readonly", "");
      document.body.appendChild(area);
      area.select();
      document.execCommand("copy");
      area.remove();
    }
    const previous = button.textContent;
    button.textContent = t("copied");
    setTimeout(function () { button.textContent = previous; }, 1200);
  }

  async function loadProxy() {
    const res = await api("/api/v1/system/proxy");
    if (!res.ok) return;
    const input = $("proxy");
    if (document.activeElement !== input) input.value = res.data.proxy || "";
  }

  async function loadUsers() {
    const res = await api("/api/v1/users");
    const list = $("users");
    list.replaceChildren();
    if (!res.ok) {
      const li = document.createElement("li");
      li.textContent = res.error || t("unreachable");
      list.appendChild(li);
      return;
    }
    const users = res.data.users || [];
    if (!users.length) {
      const li = document.createElement("li");
      li.textContent = t("noPeople");
      list.appendChild(li);
      return;
    }
    users.forEach(function (u) {
      const li = document.createElement("li");
      const name = document.createElement("span");
      name.textContent = u.username + " · " + u.role + " · " + u.downloads;
      li.appendChild(name);
      if (u.role !== "admin") {
        const button = document.createElement("button");
        button.type = "button";
        button.className = "danger";
        button.textContent = t("remove");
        button.addEventListener("click", async function () {
          if (!window.confirm(t("deleteUser"))) return;
          const gone = await api("/api/v1/users/" + encodeURIComponent(u.id), { method: "DELETE" });
          $("user-status").textContent = gone.ok ? "" : (gone.error || t("unreachable"));
          if (gone.ok) loadUsers();
        });
        li.appendChild(button);
      }
      list.appendChild(li);
    });
  }

  $("add").addEventListener("submit", async function (event) {
    event.preventDefault();
    const button = event.submitter || $("best");
    if (button.disabled) return;
    const kind = button.dataset.kind || "video";
    let quality = button.dataset.quality || "best";
    if (quality === "pick") {
      quality = $("quality").value;
      if (!quality) return;
    }
    button.disabled = true;
    $("add").setAttribute("aria-busy", "true");
    try {
      await queue(kind, quality);
    } finally {
      button.disabled = false;
      $("add").removeAttribute("aria-busy");
    }
  });

  $("url").addEventListener("change", function () {
    const url = $("url").value.trim();
    if (url) loadFormats(url, false);
  });

  $("refresh").addEventListener("click", function () {
    const url = $("url").value.trim();
    if (!url) {
      setStatus(t("needLink"));
      $("url").focus();
      return;
    }
    loadFormats(url, true);
  });

  $("scope-mine").addEventListener("click", function () {
    scope = "mine";
    $("scope-mine").classList.add("on");
    $("scope-mine").setAttribute("aria-pressed", "true");
    $("scope-all").classList.remove("on");
    $("scope-all").setAttribute("aria-pressed", "false");
    refresh();
  });

  $("scope-all").addEventListener("click", function () {
    scope = "all";
    $("scope-all").classList.add("on");
    $("scope-all").setAttribute("aria-pressed", "true");
    $("scope-mine").classList.remove("on");
    $("scope-mine").setAttribute("aria-pressed", "false");
    refresh();
  });

  $("list").addEventListener("click", async function (event) {
    const button = event.target.closest("button[data-act]");
    if (!button) return;
    const card = button.closest("[data-id]");
    const id = card.dataset.id;
    const act = button.dataset.act;
    button.disabled = true;
    try {
      if (act === "pause" || act === "resume" || act === "cancel") {
        const res = await api("/api/v1/downloads/" + id + "/" + act, { method: "POST", body: {} });
        if (!res.ok) {
          $("banner").hidden = false;
          $("banner").textContent = res.error || t("unreachable");
        }
      } else if (act === "delete") {
        pendingDelete.set(id, Date.now() + 4000);
      } else if (act === "confirm-delete") {
        pendingDelete.delete(id);
        const res = await api("/api/v1/downloads/" + id, { method: "DELETE" });
        if (!res.ok) {
          $("banner").hidden = false;
          $("banner").textContent = res.error || t("unreachable");
        }
      } else if (act === "link") {
        await copyText(location.origin + "/api/v1/files/" + card.dataset.token, button);
      } else if (act === "key") {
        await copyText(card.dataset.token, button);
      }
    } finally {
      button.disabled = false;
      refresh();
    }
  });

  $("settings").addEventListener("click", function () {
    $("server-url").value = location.origin;
    $("api-key").value = key;
    $("settings-dialog").showModal();
    if (!$("admin-tools").hidden) {
      loadProxy();
      loadUsers();
    }
  });

  $("close-settings").addEventListener("click", function () {
    $("settings-dialog").close();
  });

  $("reveal-key").addEventListener("click", function () {
    const input = $("api-key");
    const show = input.type === "password";
    input.type = show ? "text" : "password";
    $("reveal-key").textContent = show ? t("hide") : t("show");
    $("reveal-key").setAttribute("aria-pressed", show ? "true" : "false");
  });

  $("copy-server").addEventListener("click", function () {
    copyText($("server-url").value, $("copy-server"));
  });

  $("copy-key").addEventListener("click", function () {
    copyText($("api-key").value, $("copy-key"));
  });

  $("proxy-form").addEventListener("submit", async function (event) {
    event.preventDefault();
    const button = $("proxy-save");
    button.disabled = true;
    const res = await api("/api/v1/system/proxy", {
      method: "PUT",
      body: { proxy: $("proxy").value.trim() }
    });
    button.disabled = false;
    if (!res.ok) {
      $("proxy-status").textContent = res.error || t("unreachable");
      return;
    }
    $("proxy").value = res.data.proxy || "";
    $("proxy-status").textContent = res.data.proxy ? t("proxySaved") : t("proxyCleared");
  });

  $("user-form").addEventListener("submit", async function (event) {
    event.preventDefault();
    const username = $("username").value.trim();
    if (!username) return;
    $("generate").disabled = true;
    const res = await api("/api/v1/users", { method: "POST", body: { username: username } });
    $("generate").disabled = false;
    const box = $("new-key");
    box.hidden = false;
    if (!res.ok) {
      box.textContent = res.error || t("unreachable");
      return;
    }
    box.textContent = res.data.username + " · " + res.data.token;
    $("username").value = "";
    try { await navigator.clipboard.writeText(res.data.token); } catch (_err) { /* shown on screen */ }
    loadUsers();
  });

  $("open-folder").addEventListener("click", async function () {
    $("folder-status").textContent = "";
    let res;
    try {
      res = await fetch("/app/folder", { method: "POST" });
    } catch (_err) {
      $("folder-status").textContent = t("folderFailed");
      return;
    }
    $("folder-status").textContent = res.ok ? t("folderOpened") : t("folderFailed");
  });

  $("quit").addEventListener("click", async function () {
    $("quit").disabled = true;
    try {
      await fetch("/app/quit", { method: "POST" });
    } catch (_err) {
      $("quit").disabled = false;
    }
  });

  applyLocale();
  if (!key) {
    $("banner").hidden = false;
    $("banner").textContent = t("localOnly");
    $("best").disabled = true;
    $("audio").disabled = true;
    $("car").disabled = true;
    return;
  }
  loadHealth();
  refresh();
  timer = setInterval(refresh, 1200);
  window.addEventListener("pagehide", function () { clearInterval(timer); });
})();
