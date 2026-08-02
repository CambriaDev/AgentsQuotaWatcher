package web

const dashboardHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Agents Quota Watcher</title>
  <style>
    :root { color-scheme: dark; --bg:#0c1020; --panel:#151b31; --line:#2d3659; --text:#f1f4ff; --muted:#9ba7ca; --accent:#7c8cff; --warn:#f2c96d; --danger:#ef7188; }
    * { box-sizing:border-box } body { margin:0; font:15px/1.5 Inter,ui-sans-serif,system-ui,sans-serif; background:radial-gradient(circle at top right,#20285a,var(--bg) 55%); color:var(--text) }
    main { max-width:1100px; margin:auto; padding:48px 24px 72px } header { display:flex; align-items:flex-start; justify-content:space-between; gap:20px; margin-bottom:32px }
    h1 { margin:0; font-size:clamp(28px,5vw,42px); letter-spacing:-.04em } h2 { margin:0; font-size:20px } p { color:var(--muted) } button { border:0; border-radius:8px; color:var(--text); background:var(--accent); padding:9px 13px; cursor:pointer; font:inherit } button.secondary { background:#252e4e } button.danger { background:#5a2436 } button:disabled { opacity:.6; cursor:wait }
    #cards { display:grid; grid-template-columns:repeat(auto-fit,minmax(320px,1fr)); gap:18px } .card { background:color-mix(in srgb,var(--panel) 94%,transparent); border:1px solid var(--line); border-radius:14px; padding:22px; box-shadow:0 18px 48px #0003 }
    .topline { display:flex; justify-content:space-between; align-items:center; gap:12px }.badge { color:var(--muted); border:1px solid var(--line); border-radius:100px; padding:3px 9px; font-size:12px }.badge.ok { color:#8fe4bc; border-color:#33795a }.badge.error { color:#ff9aaa; border-color:#9a4052 }
    .metric { margin:22px 0 }.metric:first-child { margin-top:16px }.metric-row { display:flex; justify-content:space-between; gap:12px; color:var(--muted) }.metric-row strong { color:var(--text); font-size:17px }.bar { height:8px; border-radius:8px; background:#252c47; overflow:hidden; margin-top:8px }.bar span { display:block; height:100%; background:linear-gradient(90deg,#69d6aa,#9ccf8e); border-radius:inherit }.bar.warn span { background:var(--warn) }.bar.danger span { background:var(--danger) }
    .details { margin:7px 0 0; color:var(--muted); font-size:13px }.model-list { display:grid; gap:6px; margin-top:12px; padding-top:12px; border-top:1px solid var(--line) }.model-row { display:flex; justify-content:space-between; gap:12px; color:var(--muted); font-size:13px }.model-row strong { color:var(--text); font-weight:600 }.error { border-left:3px solid var(--danger); padding:10px; margin-top:18px; background:#3b1c2a; color:#ffb4c0 }.footer { color:var(--muted); font-size:12px; margin:20px 0 0 }.actions { display:flex; flex-wrap:wrap; gap:8px; margin-top:20px }
    dialog { width:min(500px,calc(100% - 32px)); background:var(--panel); color:var(--text); border:1px solid var(--line); border-radius:14px; padding:24px; box-shadow:0 24px 90px #0008 } dialog::backdrop { background:#050712bb } label { display:block; margin:16px 0 6px; color:var(--muted) } input,select { width:100%; padding:10px; color:var(--text); background:#0d1224; border:1px solid var(--line); border-radius:8px; font:inherit }.dialog-actions { display:flex; justify-content:flex-end; gap:8px; margin-top:22px } #notice { min-height:22px; color:var(--muted); margin:0 0 18px }
    @media (max-width:600px) { main { padding:30px 16px } header { display:block } header button { margin-top:16px } }
  </style>
</head>
<body>
  <main>
    <header>
      <div><h1>Agents Quota Watcher</h1><p>Claude 与 GitHub Copilot 剩余额度概览。</p></div>
      <div class="actions"><button id="add-account">添加账户</button><button class="secondary" id="refresh">刷新全部</button></div>
    </header>
    <p id="notice" aria-live="polite"></p>
    <section id="cards" aria-live="polite"></section>
  </main>
  <dialog id="credentials">
    <form id="credential-form">
      <h2 id="dialog-title">连接账户</h2>
      <p>令牌仅保存到此机器的本地凭据文件中，文件权限为仅当前用户可读写。</p>
      <label for="provider">服务</label><select id="provider" name="provider"><option value="copilot">GitHub Copilot</option><option value="claude">Claude</option></select>
      <label for="alias">卡片别名</label><input id="alias" name="alias" type="text" maxlength="80" placeholder="例如：工作账号" required>
      <div id="username-field" hidden><label for="username">GitHub 用户名</label><input id="username" name="username" type="text" autocomplete="username"></div>
      <div id="credit-limit-field" hidden><label for="credit_limit">每月 credits 上限</label><input id="credit_limit" name="credit_limit" type="number" min="1" step="1" value="1500"><p>默认 1500，可按你的 Copilot 计划调整。</p></div>
      <label for="access_token">访问令牌</label><input id="access_token" name="access_token" type="password" autocomplete="off" required>
      <label for="refresh_token">刷新令牌（Claude OAuth 可选）</label><input id="refresh_token" name="refresh_token" type="password" autocomplete="off">
      <label for="expires_at">过期时间（可选，RFC 3339）</label><input id="expires_at" name="expires_at" type="text" placeholder="2026-08-01T00:00:00Z">
      <div class="dialog-actions"><button type="button" class="secondary" id="cancel">取消</button><button type="submit">保存并验证</button></div>
    </form>
  </dialog>
  <script>
    const cards = document.querySelector('#cards'), notice = document.querySelector('#notice'), dialog = document.querySelector('#credentials'), form = document.querySelector('#credential-form');
    const escapeHTML = value => String(value).replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#039;'}[char]));
    const relativeTime = value => { if (!value) return '尚未刷新'; const seconds = Math.max(0, Math.floor((Date.now() - new Date(value)) / 1000)); if (seconds < 10) return '刚刚刷新'; if (seconds < 60) return seconds + ' 秒前刷新'; if (seconds < 3600) return Math.floor(seconds / 60) + ' 分钟前刷新'; return Math.floor(seconds / 3600) + ' 小时前刷新'; };
    const resetTime = value => value ? '重置：' + new Date(value).toLocaleString() : '';
    function metricHTML(metric) {
      const percent = Math.min(100, Math.max(0, Number(metric.percent || 0))), tone = percent >= 80 ? 'danger' : percent >= 50 ? 'warn' : '';
      const formatCredits = value => Number(value).toLocaleString(undefined, {maximumFractionDigits: 2});
      const amount = metric.limit ? formatCredits(metric.used) + ' / ' + formatCredits(metric.limit) : percent.toFixed(0) + '%';
      const breakdown = (metric.breakdown || []).map(item => '<div class="model-row"><span>' + escapeHTML(item.name) + '</span><strong>' + formatCredits(item.used) + ' credits</strong></div>').join('');
      const modelList = breakdown ? '<div class="model-list">' + breakdown + '</div>' : '';
      const unit = metric.id === 'ai-credits' ? ' credits' : '';
      return '<div class="metric"><div class="metric-row"><span>' + escapeHTML(metric.label) + '</span><strong>' + amount + unit + '</strong></div><div class="bar ' + tone + '"><span style="width:' + percent + '%"></span></div><div class="details">' + percent.toFixed(1) + '% 已使用 ' + resetTime(metric.reset_at) + '</div>' + modelList + '</div>';
    }
    function cardHTML(provider) {
      const state = provider.error ? 'error' : 'ok';
      const label = provider.error ? '连接异常' : '已连接';
      const metrics = provider.metrics?.length ? provider.metrics.map(metricHTML).join('') : '<p>正在读取额度…</p>';
      const error = provider.error ? '<div class="error">' + escapeHTML(provider.error) + '</div>' : '';
      return '<article class="card"><div class="topline"><h2>' + escapeHTML(provider.name) + '</h2><span class="badge ' + state + '">' + label + '</span></div><p class="footer">' + escapeHTML(provider.provider) + '</p>' + metrics + error + '<p class="footer">' + relativeTime(provider.updated_at) + '</p><div class="actions"><button class="secondary" data-refresh="' + provider.account_id + '">刷新</button><button class="danger" data-disconnect="' + provider.account_id + '">删除</button></div></article>';
    }
    async function request(url, options = {}) { const response = await fetch(url, options); const body = await response.json().catch(() => ({})); if (!response.ok) throw new Error(body.error || '请求失败'); return body; }
    async function load() { try { const accounts = await request('/api/usage'); cards.innerHTML = accounts.length ? accounts.map(cardHTML).join('') : '<p>尚未添加账户。点击“添加账户”以开始监控。</p>'; notice.textContent = ''; } catch (error) { notice.textContent = error.message; } }
    document.querySelector('#refresh').onclick = async () => { notice.textContent = '正在刷新…'; try { await request('/api/refresh', {method:'POST'}); await load(); } catch (error) { notice.textContent = error.message; } };
    function updateAccountForm() {
      const isCopilot = document.querySelector('#provider').value === 'copilot', username = document.querySelector('#username'), usernameField = document.querySelector('#username-field'), creditLimit = document.querySelector('#credit_limit'), creditLimitField = document.querySelector('#credit-limit-field');
      usernameField.hidden = !isCopilot; username.required = isCopilot;
      creditLimitField.hidden = !isCopilot; creditLimit.required = isCopilot;
    }
    document.querySelector('#add-account').onclick = () => { form.reset(); document.querySelector('#provider').value = 'copilot'; document.querySelector('#credit_limit').value = '1500'; updateAccountForm(); dialog.showModal(); };
    document.querySelector('#provider').onchange = updateAccountForm;
    cards.onclick = async event => {
      const target = event.target, id = target.dataset.refresh || target.dataset.disconnect;
      if (!id) return;
      try { target.disabled = true; if (target.dataset.disconnect) await request('/api/accounts/' + encodeURIComponent(id), {method:'DELETE'}); else await request('/api/accounts/' + encodeURIComponent(id) + '/refresh', {method:'POST'}); await load(); } catch (error) { notice.textContent = error.message; } finally { target.disabled = false; }
    };
    document.querySelector('#cancel').onclick = () => dialog.close();
    form.onsubmit = async event => { event.preventDefault(); const data = Object.fromEntries(new FormData(form)), provider = data.provider; if (provider === 'copilot') data.credit_limit = Number(data.credit_limit); else { delete data.username; delete data.credit_limit; } const credentials = {access_token:data.access_token, username:data.username, credit_limit:data.credit_limit, refresh_token:data.refresh_token, expires_at:data.expires_at}; try { await request('/api/accounts', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({provider,alias:data.alias,credentials})}); dialog.close(); await load(); } catch (error) { notice.textContent = error.message; } };
    load(); setInterval(load, 60000);
  </script>
</body>
</html>`
