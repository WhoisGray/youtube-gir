importScripts("i18n.js", "api.js");

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (!msg || msg.type !== "yg-api") return;
  YG.request(msg.path, { method: msg.method, body: msg.body })
    .then((result) => sendResponse(result))
    .catch((err) => sendResponse({
      ok: false,
      status: 0,
      code: "unreachable",
      error: String(err && err.message || err)
    }));
  return true;
});
