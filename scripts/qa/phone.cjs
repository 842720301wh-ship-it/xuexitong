// Uses the debug WebView on the explicitly connected test phone; never reads keys.
const { chromium } = require('playwright-core');
const fs = require('node:fs');
const path = require('node:path');
const deadline = setTimeout(() => { console.error('Phone QA timed out'); process.exit(1); }, 60000);
(async () => {
  const version = await (await fetch('http://127.0.0.1:19223/json/version')).json();
  const targets = await (await fetch('http://127.0.0.1:19223/json/list')).json();
  const visible = targets.find(t => { try { return JSON.parse(t.description).visible; } catch { return false; } });
  const browser = await chromium.connectOverCDP(version.webSocketDebuggerUrl);
  try {
    let page;
    for (const candidate of browser.contexts()[0].pages()) {
      if (!/^http:\/\/127\.0\.0\.1:/.test(candidate.url()) || !await candidate.locator('h1').count()) continue;
      page ||= candidate;
      if (visible) { const session = await candidate.context().newCDPSession(candidate); const info = await session.send('Target.getTargetInfo'); await session.detach(); if (info.targetInfo.targetId === visible.id) { page = candidate; break; } }
    }
    if (!page) throw Error('App WebView unavailable');
    const action = process.argv[2] || 'inspect';
    if (action === 'runtime-start' || action === 'runtime-stop') {
      console.log(JSON.stringify(await page.evaluate(async action => {
        const tasks = await (await fetch('/api/tasks')).json();
        if (!tasks.length) return {error:'没有已保存任务'};
        if (action === 'runtime-start') window.TiantianAndroid.startTask(tasks[0].id);
        else window.TiantianAndroid.stopTask(tasks[0].id);
        return {task_id:tasks[0].id,native:JSON.parse(window.TiantianAndroid.status())};
      }, action), null, 2));
      return;
    }
    if (action === 'model-diagnose') {
      console.log(JSON.stringify(await page.evaluate(async () => {
        const settings = await (await fetch('/api/settings')).json(), model = settings.models[0];
        if (!model) return { error:'没有已保存的模型配置' };
        const results = [];
        for (const protocol of [...new Set([model.protocol || 'chat', 'chat'])]) {
          const response = await fetch(`/api/models/${model.id}/test`, { method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({protocol}) });
          results.push({ protocol, status:response.status, result:await response.json() });
        }
        return { endpoint:model.endpoint, primary:model.primary, results };
      }), null, 2));
      return;
    }
    if (action === 'start' || action === 'stop' || action === 'home') {
      await page.evaluate(() => { location.hash = 'home'; });
      await page.locator('.learning-task-card').first().waitFor();
      if (action !== 'home') {
        const button = page.locator('.learning-round-action').first();
        const label = await button.getAttribute('aria-label');
        if ((action === 'start' && !label.startsWith('启动')) || (action === 'stop' && !label.startsWith('停止'))) throw Error('Unexpected task state: ' + label);
        await button.click();
      }
    }
    const directory = path.resolve('docs/design/mobile-v2'); fs.mkdirSync(directory, { recursive: true });
    if (action === 'capture') await page.screenshot({ path: path.join(directory, action + '.png'), timeout:10000 });
    console.log(JSON.stringify(await page.evaluate(async () => {
      const response = await fetch('/api/task-progress');
      return { page: document.querySelector('h1')?.textContent, cards: document.querySelectorAll('.learning-task-card').length,
        editing: [...document.querySelectorAll('.el-drawer')].some(e => getComputedStyle(e).visibility !== 'hidden' && getComputedStyle(e).display !== 'none' && e.getBoundingClientRect().height > 0),
        native: JSON.parse(window.TiantianAndroid.status()), progress: await response.json(), buttons: [...document.querySelectorAll('.learning-round-action')].map(b => ({ label: b.getAttribute('aria-label'), disabled: b.disabled })) };
    }), null, 2));
  } finally { await browser.close(); }
})().catch(e => { console.error(e.message); process.exitCode = 1; }).finally(() => clearTimeout(deadline));
