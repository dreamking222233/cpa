package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const proxyPoolPageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Proxy Pool</title>
  <style>
    :root { color-scheme: light dark; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    body { margin: 0; background: #f6f8fa; color: #18202a; }
    main { max-width: 1120px; margin: 0 auto; padding: 28px 20px 48px; }
    h1 { margin: 0 0 20px; font-size: 28px; line-height: 1.2; }
    h2 { margin: 0 0 12px; font-size: 18px; }
    section { margin-top: 16px; padding: 16px; background: #fff; border: 1px solid #d8dee4; border-radius: 8px; }
    label { display: block; margin: 10px 0 6px; font-size: 13px; font-weight: 600; }
    input { width: 100%; box-sizing: border-box; padding: 9px 10px; border: 1px solid #c7ced6; border-radius: 6px; font: inherit; }
    button { border: 1px solid #1f6feb; background: #1f6feb; color: #fff; border-radius: 6px; padding: 9px 12px; font: inherit; cursor: pointer; }
    button.secondary { background: #fff; color: #1f2328; border-color: #c7ced6; }
    button.danger { background: #d1242f; border-color: #d1242f; }
    button:disabled { opacity: .55; cursor: not-allowed; }
    .toolbar { display: flex; gap: 8px; flex-wrap: wrap; align-items: end; }
    .grid { display: grid; grid-template-columns: 1fr 2fr auto auto; gap: 8px; align-items: center; }
    .row { padding: 10px 0; border-top: 1px solid #eaeef2; }
    .row:first-child { border-top: 0; }
    .muted { color: #6b7280; font-size: 13px; }
    .status { min-height: 20px; margin-top: 10px; font-size: 13px; }
    .table { width: 100%; border-collapse: collapse; font-size: 13px; }
    .table th, .table td { padding: 8px; border-bottom: 1px solid #eaeef2; text-align: left; vertical-align: top; }
    .table th { color: #59636e; font-weight: 600; }
    code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; }
    @media (max-width: 760px) { .grid { grid-template-columns: 1fr; } }
  </style>
</head>
<body>
<main>
  <h1>Proxy Pool</h1>
  <section>
    <h2>Access</h2>
    <div class="toolbar">
      <div style="flex:1 1 320px">
        <label for="key">Management Key</label>
        <input id="key" type="password" autocomplete="current-password">
      </div>
      <button id="saveKey">Save Key</button>
      <button id="load" class="secondary">Refresh</button>
    </div>
    <div class="status" id="status"></div>
  </section>
  <section>
    <h2>Configured Proxies</h2>
    <div id="proxyList" class="muted">No data loaded.</div>
    <div class="row">
      <div class="grid">
        <input id="newName" placeholder="Name, optional">
        <input id="newURL" placeholder="socks5://user:pass@host:1080">
        <label style="margin:0"><input id="newDisabled" type="checkbox" style="width:auto"> Disabled</label>
        <button id="add">Add</button>
      </div>
    </div>
  </section>
  <section>
    <h2>Recent Request Proxy Usage</h2>
    <p class="muted">Request logging must be enabled for upstream proxy details to appear.</p>
    <table class="table">
      <thead><tr><th>Modified</th><th>Request ID</th><th>Proxy</th><th>Log File</th></tr></thead>
      <tbody id="logs"><tr><td colspan="4" class="muted">No data loaded.</td></tr></tbody>
    </table>
  </section>
</main>
<script>
const keyInput = document.getElementById('key');
const statusEl = document.getElementById('status');
const proxyList = document.getElementById('proxyList');
const logsEl = document.getElementById('logs');
let proxies = [];

keyInput.value = localStorage.getItem('cpa.managementKey') || '';
document.getElementById('saveKey').onclick = () => {
  localStorage.setItem('cpa.managementKey', keyInput.value.trim());
  setStatus('Management key saved.');
};
document.getElementById('load').onclick = loadAll;
document.getElementById('add').onclick = addProxy;

function headers() {
  return {'Content-Type': 'application/json', 'X-Management-Key': keyInput.value.trim()};
}
function setStatus(text, error) {
  statusEl.textContent = text || '';
  statusEl.style.color = error ? '#d1242f' : '#1f883d';
}
async function api(path, options = {}) {
  const res = await fetch(path, {...options, headers: {...headers(), ...(options.headers || {})}});
  const text = await res.text();
  let body = {};
  try { body = text ? JSON.parse(text) : {}; } catch (_) { body = {message: text}; }
  if (!res.ok) throw new Error(body.error || body.message || res.statusText);
  return body;
}
async function loadAll() {
  try {
    const data = await api('/v0/management/proxy-pool');
    proxies = data['proxy-pool'] || [];
    renderProxies();
    await loadLogs();
    setStatus('Loaded.');
  } catch (err) {
    setStatus(err.message, true);
  }
}
function renderProxies() {
  if (!proxies.length) {
    proxyList.innerHTML = '<div class="muted">No proxies configured.</div>';
    return;
  }
  proxyList.innerHTML = proxies.map((p, i) =>
    '<div class="row">' +
    '<div class="grid">' +
    '<input data-i="' + i + '" data-k="name" value="' + escapeAttr(p.name || '') + '" placeholder="Name">' +
    '<input data-i="' + i + '" data-k="url" value="' + escapeAttr(p.url || '') + '" placeholder="Proxy URL">' +
    '<label style="margin:0"><input data-i="' + i + '" data-k="disabled" type="checkbox" style="width:auto" ' + (p.disabled ? 'checked' : '') + '> Disabled</label>' +
    '<div>' +
    '<button data-action="save" data-i="' + i + '">Save</button> ' +
    '<button class="danger" data-action="delete" data-i="' + i + '">Delete</button>' +
    '</div>' +
    '</div>' +
    '</div>').join('');
  proxyList.querySelectorAll('button').forEach(btn => btn.onclick = handleProxyButton);
}
async function handleProxyButton(event) {
  const btn = event.currentTarget;
  const index = Number(btn.dataset.i);
  if (btn.dataset.action === 'delete') {
    await patchProxy({action: 'delete', index});
    return;
  }
  const scope = btn.closest('.row');
  const value = {
    name: scope.querySelector('[data-k="name"]').value.trim(),
    url: scope.querySelector('[data-k="url"]').value.trim(),
    disabled: scope.querySelector('[data-k="disabled"]').checked
  };
  await patchProxy({action: 'update', index, value});
}
async function addProxy() {
  const value = {
    name: document.getElementById('newName').value.trim(),
    url: document.getElementById('newURL').value.trim(),
    disabled: document.getElementById('newDisabled').checked
  };
  await patchProxy({action: 'add', value});
  document.getElementById('newName').value = '';
  document.getElementById('newURL').value = '';
  document.getElementById('newDisabled').checked = false;
}
async function patchProxy(payload) {
  try {
    await api('/v0/management/proxy-pool', {method: 'PATCH', body: JSON.stringify(payload)});
    await loadAll();
  } catch (err) {
    setStatus(err.message, true);
  }
}
async function loadLogs() {
  const data = await api('/v0/management/proxy-pool/request-logs?limit=100');
  const logs = data.logs || [];
  if (!logs.length) {
    logsEl.innerHTML = '<tr><td colspan="4" class="muted">No request logs found.</td></tr>';
    return;
  }
  logsEl.innerHTML = logs.map(item =>
    '<tr>' +
    '<td>' + new Date((item.modified || 0) * 1000).toLocaleString() + '</td>' +
    '<td><code>' + escapeHTML(item['request-id'] || '') + '</code></td>' +
    '<td><code>' + escapeHTML(formatProxyAttempts(item)) + '</code></td>' +
    '<td><code>' + escapeHTML(item.name || '') + '</code></td>' +
    '</tr>').join('');
}
function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
}
function formatProxyAttempts(item) {
  const proxies = item.proxies || [];
  if (proxies.length > 1) return proxies.join(' -> ');
  return item.proxy || '<none>';
}
function escapeAttr(value) { return escapeHTML(value); }
</script>
</body>
</html>`

func (s *Server) serveProxyPoolPage(c *gin.Context) {
	if s == nil || s.cfg == nil || s.cfg.Home.Enabled || s.cfg.RemoteManagement.DisableControlPanel {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, proxyPoolPageHTML)
}
