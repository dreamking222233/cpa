package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const requestLogsPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>请求日志</title>
  <style>
    :root { color-scheme: light dark; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    body { margin: 0; background: #f6f8fa; color: #18202a; }
    main { max-width: 1280px; margin: 0 auto; padding: 28px 20px 48px; }
    h1 { margin: 0 0 20px; font-size: 28px; line-height: 1.2; }
    h2 { margin: 0 0 12px; font-size: 18px; }
    section { margin-top: 16px; padding: 16px; background: #fff; border: 1px solid #d8dee4; border-radius: 8px; }
    label { display: block; margin: 10px 0 6px; font-size: 13px; font-weight: 600; }
    input, select { box-sizing: border-box; padding: 9px 10px; border: 1px solid #c7ced6; border-radius: 6px; font: inherit; }
    input { width: 100%; }
    button { border: 1px solid #1f6feb; background: #1f6feb; color: #fff; border-radius: 6px; padding: 9px 12px; font: inherit; cursor: pointer; }
    button.secondary { background: #fff; color: #1f2328; border-color: #c7ced6; }
    .toolbar { display: flex; gap: 8px; flex-wrap: wrap; align-items: end; }
    .summary { display: grid; grid-template-columns: repeat(5, minmax(140px, 1fr)); gap: 10px; }
    .metric { padding: 10px; border: 1px solid #eaeef2; border-radius: 8px; background: #fbfcfd; }
    .metric b { display: block; font-size: 18px; margin-top: 4px; }
    .muted { color: #6b7280; font-size: 13px; }
    .status { min-height: 20px; margin-top: 10px; font-size: 13px; }
    .table-wrap { overflow-x: auto; }
    table { width: 100%; border-collapse: collapse; font-size: 13px; min-width: 1080px; }
    th, td { padding: 8px; border-bottom: 1px solid #eaeef2; text-align: left; vertical-align: top; }
    th { color: #59636e; font-weight: 600; white-space: nowrap; }
    td.num { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
    code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; }
    a { color: #0969da; text-decoration: none; }
    a:hover { text-decoration: underline; }
    @media (max-width: 760px) {
      main { padding: 20px 12px 36px; }
      .summary { grid-template-columns: 1fr 1fr; }
    }
  </style>
</head>
<body>
<main>
  <h1>请求日志</h1>
  <section>
    <h2>访问</h2>
    <div class="toolbar">
      <div style="flex:1 1 320px">
        <label for="key">Management Key</label>
        <input id="key" type="password" autocomplete="current-password">
      </div>
      <div>
        <label for="limit">条数</label>
        <select id="limit">
          <option value="50">50</option>
          <option value="100" selected>100</option>
          <option value="200">200</option>
          <option value="500">500</option>
        </select>
      </div>
      <button id="saveKey">保存 Key</button>
      <button id="load" class="secondary">刷新</button>
    </div>
    <div class="status" id="status"></div>
  </section>
  <section>
    <h2>汇总</h2>
    <div class="summary">
      <div class="metric"><span class="muted">请求数</span><b id="totalRequests">0</b></div>
      <div class="metric"><span class="muted">输入 token</span><b id="totalInput">0</b></div>
      <div class="metric"><span class="muted">输出 token</span><b id="totalOutput">0</b></div>
      <div class="metric"><span class="muted">缓存 token</span><b id="totalCached">0</b></div>
      <div class="metric"><span class="muted">估算费用 USD</span><b id="totalCost">$0.000000</b></div>
    </div>
    <p class="muted" id="pricingNote"></p>
  </section>
  <section>
    <h2>明细</h2>
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>北京时间</th>
            <th>模型</th>
            <th>授权文件</th>
            <th class="num">输入</th>
            <th class="num">输出</th>
            <th class="num">缓存</th>
            <th class="num">推理</th>
            <th class="num">合计</th>
            <th class="num">费用</th>
            <th>请求 ID</th>
            <th>状态</th>
            <th>日志文件</th>
          </tr>
        </thead>
        <tbody id="rows"><tr><td colspan="12" class="muted">暂无数据。</td></tr></tbody>
      </table>
    </div>
  </section>
</main>
<script>
const keyInput = document.getElementById('key');
const statusEl = document.getElementById('status');
const rowsEl = document.getElementById('rows');
keyInput.value = localStorage.getItem('cpa.managementKey') || '';
document.getElementById('saveKey').onclick = () => {
  localStorage.setItem('cpa.managementKey', keyInput.value.trim());
  setStatus('Management key saved.');
};
document.getElementById('load').onclick = loadLogs;

function headers() {
  return {'Content-Type': 'application/json', 'X-Management-Key': keyInput.value.trim()};
}
function setStatus(text, error) {
  statusEl.textContent = text || '';
  statusEl.style.color = error ? '#d1242f' : '#1f883d';
}
async function api(path) {
  const res = await fetch(path, {headers: headers()});
  const text = await res.text();
  let body = {};
  try { body = text ? JSON.parse(text) : {}; } catch (_) { body = {message: text}; }
  if (!res.ok) throw new Error(body.error || body.message || res.statusText);
  return body;
}
async function loadLogs() {
  try {
    const limit = document.getElementById('limit').value;
    const data = await api('/v0/management/openai-request-logs?limit=' + encodeURIComponent(limit));
    renderSummary(data);
    renderRows(data.logs || []);
    setStatus('已加载。');
  } catch (err) {
    setStatus(err.message, true);
  }
}
function renderSummary(data) {
  const t = data.totals || {};
  document.getElementById('totalRequests').textContent = fmtInt(t.requests);
  document.getElementById('totalInput').textContent = fmtInt(t.input_tokens);
  document.getElementById('totalOutput').textContent = fmtInt(t.output_tokens);
  document.getElementById('totalCached').textContent = fmtInt(t.cached_tokens);
  document.getElementById('totalCost').textContent = fmtUSD(t.cost_usd);
  const source = data.pricing_source || 'https://openai.com/api/pricing/';
  document.getElementById('pricingNote').innerHTML = '价格按 OpenAI 官方 API Pricing 估算，检查日期：' + escapeHTML(data.pricing_checked || '') + '，时区：' + escapeHTML(data.timezone || 'Asia/Shanghai') + '。<a href="' + escapeAttr(source) + '" target="_blank" rel="noreferrer">价格来源</a>';
}
function renderRows(logs) {
  if (!logs.length) {
    rowsEl.innerHTML = '<tr><td colspan="12" class="muted">没有找到 request log。请确认 request-log 已启用并已有请求。</td></tr>';
    return;
  }
  rowsEl.innerHTML = logs.map(item => {
    const price = item.price_available ? fmtUSD(item.cost_usd) : '未匹配';
    return '<tr>' +
      '<td>' + escapeHTML(item.time_beijing || '') + '</td>' +
      '<td><code>' + escapeHTML(item.model || 'unknown') + '</code></td>' +
      '<td><code>' + escapeHTML(item.auth_file || item.auth_id || '') + '</code></td>' +
      '<td class="num">' + fmtInt(item.input_tokens) + '</td>' +
      '<td class="num">' + fmtInt(item.output_tokens) + '</td>' +
      '<td class="num">' + fmtInt(item.cached_tokens) + '</td>' +
      '<td class="num">' + fmtInt(item.reasoning_tokens) + '</td>' +
      '<td class="num">' + fmtInt(item.total_tokens) + '</td>' +
      '<td class="num">' + escapeHTML(price) + '</td>' +
      '<td><code>' + escapeHTML(item.request_id || '') + '</code></td>' +
      '<td>' + escapeHTML(item.status || '') + '</td>' +
      '<td><code>' + escapeHTML(item.name || '') + '</code></td>' +
      '</tr>';
  }).join('');
}
function fmtInt(value) { return Number(value || 0).toLocaleString(); }
function fmtUSD(value) { return '$' + Number(value || 0).toFixed(6); }
function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
}
function escapeAttr(value) { return escapeHTML(value); }
</script>
</body>
</html>`

func (s *Server) serveRequestLogsPage(c *gin.Context) {
	if s == nil || s.cfg == nil || s.cfg.Home.Enabled || s.cfg.RemoteManagement.DisableControlPanel {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, requestLogsPageHTML)
}
