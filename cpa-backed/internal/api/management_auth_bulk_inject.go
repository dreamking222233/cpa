package api

import "bytes"

const managementAuthFilesBulkScript = `<script id="cpa-auth-files-bulk-script">
(function(){
  if (window.__cpaAuthFilesBulkLoaded) return;
  window.__cpaAuthFilesBulkLoaded = true;
  var css = document.createElement('style');
  css.textContent = [
    '#cpa-auth-bulk{position:fixed;right:18px;bottom:18px;z-index:2147483000;width:min(420px,calc(100vw - 24px));max-height:72vh;background:var(--bg-primary,#fff);color:var(--text-primary,#1f2328);border:1px solid var(--border-color,#d0d7de);border-radius:10px;box-shadow:0 16px 44px rgba(27,31,36,.18);font:13px/1.45 system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;display:none;overflow:hidden}',
    '#cpa-auth-bulk[data-open="false"] .cpa-auth-bulk-body{display:none}',
    '#cpa-auth-bulk[data-open="false"]{width:auto}',
    '#cpa-auth-bulk .cpa-auth-bulk-head{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:10px 12px;border-bottom:1px solid var(--border-color,#d0d7de)}',
    '#cpa-auth-bulk[data-open="false"] .cpa-auth-bulk-head{border-bottom:0}',
    '#cpa-auth-bulk .cpa-auth-bulk-title{font-weight:700}',
    '#cpa-auth-bulk .cpa-auth-bulk-body{padding:12px;display:flex;flex-direction:column;gap:10px}',
    '#cpa-auth-bulk .cpa-auth-bulk-row{display:flex;gap:8px;align-items:center;flex-wrap:wrap}',
    '#cpa-auth-bulk input[type="password"]{box-sizing:border-box;min-width:0;flex:1 1 160px;border:1px solid var(--border-color,#d0d7de);border-radius:6px;background:var(--bg-primary,#fff);color:inherit;padding:7px 9px;font:inherit}',
    '#cpa-auth-bulk button{border:1px solid var(--border-color,#d0d7de);background:var(--bg-secondary,#f6f8fa);color:inherit;border-radius:6px;padding:7px 10px;font:inherit;cursor:pointer}',
    '#cpa-auth-bulk button.primary{background:var(--primary-color,#0969da);border-color:var(--primary-color,#0969da);color:#fff}',
    '#cpa-auth-bulk button.danger{background:#cf222e;border-color:#cf222e;color:#fff}',
    '#cpa-auth-bulk button:disabled{opacity:.55;cursor:not-allowed}',
    '#cpa-auth-bulk .cpa-auth-bulk-list{border:1px solid var(--border-color,#d0d7de);border-radius:8px;max-height:300px;overflow:auto;background:var(--bg-secondary,#f6f8fa)}',
    '#cpa-auth-bulk .cpa-auth-bulk-item{display:grid;grid-template-columns:24px 1fr auto;gap:8px;align-items:center;padding:8px 10px;border-bottom:1px solid var(--border-color,#d0d7de)}',
    '#cpa-auth-bulk .cpa-auth-bulk-item:last-child{border-bottom:0}',
    '#cpa-auth-bulk .cpa-auth-bulk-name{font-weight:650;word-break:break-all}',
    '#cpa-auth-bulk .cpa-auth-bulk-meta{color:var(--text-secondary,#57606a);font-size:12px;word-break:break-all}',
    '#cpa-auth-bulk .cpa-auth-bulk-pill{border-radius:999px;border:1px solid var(--border-color,#d0d7de);padding:2px 7px;font-size:12px;white-space:nowrap}',
    '#cpa-auth-bulk .cpa-auth-bulk-pill.off{color:#cf222e;background:#ffebe9;border-color:#ff818266}',
    '#cpa-auth-bulk .cpa-auth-bulk-pill.on{color:#1a7f37;background:#dafbe1;border-color:#4ac26b66}',
    '#cpa-auth-bulk .cpa-auth-bulk-status{min-height:18px;color:var(--text-secondary,#57606a);font-size:12px}',
    '@media (max-width:640px){#cpa-auth-bulk{right:12px;bottom:12px;max-height:78vh}}'
  ].join('');
  document.head.appendChild(css);

  var panel = document.createElement('div');
  panel.id = 'cpa-auth-bulk';
  panel.setAttribute('data-open', 'true');
  panel.innerHTML = '<div class="cpa-auth-bulk-head"><span class="cpa-auth-bulk-title">批量启停认证文件</span><button type="button" data-act="toggle">收起</button></div><div class="cpa-auth-bulk-body"><div class="cpa-auth-bulk-row"><input type="password" data-role="key" placeholder="Management Key，可留空使用当前会话"><button type="button" data-act="save-key">保存</button><button type="button" data-act="refresh">刷新</button></div><div class="cpa-auth-bulk-row"><button type="button" data-act="select-all">全选</button><button type="button" data-act="select-none">清空</button><button type="button" class="primary" data-act="enable">批量启用</button><button type="button" class="danger" data-act="disable">批量禁用</button></div><div class="cpa-auth-bulk-status" data-role="status"></div><div class="cpa-auth-bulk-list" data-role="list"></div></div>';
  document.body.appendChild(panel);

  var keyInput = panel.querySelector('[data-role="key"]');
  var statusEl = panel.querySelector('[data-role="status"]');
  var listEl = panel.querySelector('[data-role="list"]');
  keyInput.value = localStorage.getItem('managementKey') || localStorage.getItem('cpa.managementKey') || '';

  function onAuthFilesPage() {
    var locationText = (location.pathname + location.hash).toLowerCase();
    return locationText.indexOf('auth-files') !== -1 && locationText.indexOf('oauth-excluded') === -1 && locationText.indexOf('oauth-model-alias') === -1;
  }
  function setVisible() {
    panel.style.display = onAuthFilesPage() ? 'block' : 'none';
  }
  function setStatus(text, error) {
    statusEl.textContent = text || '';
    statusEl.style.color = error ? '#cf222e' : 'var(--text-secondary,#57606a)';
  }
  function headers() {
    var out = {'Content-Type':'application/json'};
    var key = (keyInput.value || '').trim();
    if (key) out.Authorization = 'Bearer ' + key;
    return out;
  }
  function authLabel(item) {
    return item.email || item.account || item.label || item.auth_index || item.id || '';
  }
  function escapeHTML(value) {
    return String(value == null ? '' : value).replace(/[&<>"']/g, function(c){ return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]; });
  }
  async function api(path, options) {
    var res = await fetch(path, Object.assign({headers: headers()}, options || {}));
    var text = await res.text();
    var body = {};
    try { body = text ? JSON.parse(text) : {}; } catch (_) { body = {message: text}; }
    if (!res.ok && res.status !== 207) throw new Error(body.error || body.message || res.statusText);
    return {status: res.status, body: body};
  }
  async function load() {
    if (!onAuthFilesPage()) return;
    setStatus('加载中...');
    try {
      var result = await api('/v0/management/auth-files');
      var files = (result.body.files || []).filter(function(item){ return item && !item.runtime_only; });
      if (!files.length) {
        listEl.innerHTML = '<div class="cpa-auth-bulk-item"><span></span><span class="cpa-auth-bulk-meta">暂无可批量操作的认证文件</span><span></span></div>';
        setStatus('');
        return;
      }
      listEl.innerHTML = files.map(function(item){
        var name = item.name || item.id || '';
        var disabled = !!item.disabled || String(item.status || '').toLowerCase() === 'disabled';
        return '<label class="cpa-auth-bulk-item"><input type="checkbox" value="' + escapeHTML(name) + '"><span><span class="cpa-auth-bulk-name">' + escapeHTML(name) + '</span><br><span class="cpa-auth-bulk-meta">' + escapeHTML(item.provider || item.type || '') + (authLabel(item) ? ' · ' + escapeHTML(authLabel(item)) : '') + '</span></span><span class="cpa-auth-bulk-pill ' + (disabled ? 'off' : 'on') + '">' + (disabled ? '已禁用' : '已启用') + '</span></label>';
      }).join('');
      setStatus('已加载 ' + files.length + ' 个认证文件。');
    } catch (err) {
      setStatus(err.message || '加载失败', true);
    }
  }
  function selectedNames() {
    return Array.prototype.slice.call(listEl.querySelectorAll('input[type="checkbox"]:checked')).map(function(input){ return input.value; });
  }
  async function patch(disabled) {
    var names = selectedNames();
    if (!names.length) {
      setStatus('请先勾选账号。', true);
      return;
    }
    setStatus((disabled ? '禁用' : '启用') + '中...');
    try {
      var result = await api('/v0/management/auth-files/status/batch', {
        method: 'PATCH',
        body: JSON.stringify({names: names, disabled: disabled})
      });
      var body = result.body || {};
      var failed = Array.isArray(body.failed) ? body.failed.length : 0;
      setStatus('已更新 ' + (body.updated || 0) + ' 个' + (failed ? '，失败 ' + failed + ' 个。' : '。'), failed > 0);
      await load();
      window.dispatchEvent(new CustomEvent('cpa-auth-files-bulk-updated', {detail: body}));
    } catch (err) {
      setStatus(err.message || '批量操作失败', true);
    }
  }
  panel.addEventListener('click', function(event) {
    if (!event.target || !event.target.closest) return;
    var button = event.target.closest('button[data-act]');
    if (!button) return;
    var act = button.getAttribute('data-act');
    if (act === 'toggle') {
      var open = panel.getAttribute('data-open') !== 'false';
      panel.setAttribute('data-open', open ? 'false' : 'true');
      button.textContent = open ? '展开' : '收起';
    } else if (act === 'save-key') {
      localStorage.setItem('cpa.managementKey', (keyInput.value || '').trim());
      localStorage.setItem('managementKey', (keyInput.value || '').trim());
      setStatus('Management Key 已保存。');
    } else if (act === 'refresh') {
      load();
    } else if (act === 'select-all') {
      listEl.querySelectorAll('input[type="checkbox"]').forEach(function(input){ input.checked = true; });
    } else if (act === 'select-none') {
      listEl.querySelectorAll('input[type="checkbox"]').forEach(function(input){ input.checked = false; });
    } else if (act === 'enable') {
      patch(false);
    } else if (act === 'disable') {
      patch(true);
    }
  });
  var lastHref = '';
  setInterval(function(){
    if (location.href === lastHref) return;
    lastHref = location.href;
    setVisible();
    if (onAuthFilesPage()) load();
  }, 500);
  setVisible();
  if (onAuthFilesPage()) load();
})();
</script>`

func injectManagementAuthFilesBulkHTML(data []byte) []byte {
	if len(data) == 0 || bytes.Contains(data, []byte(`id="cpa-auth-files-bulk-script"`)) {
		return data
	}
	marker := []byte("</body>")
	idx := bytes.LastIndex(bytes.ToLower(data), marker)
	if idx < 0 {
		return append(data, []byte(managementAuthFilesBulkScript)...)
	}
	out := make([]byte, 0, len(data)+len(managementAuthFilesBulkScript))
	out = append(out, data[:idx]...)
	out = append(out, []byte(managementAuthFilesBulkScript)...)
	out = append(out, data[idx:]...)
	return out
}
