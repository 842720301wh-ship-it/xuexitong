const { chromium } = require('playwright-core');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const net = require('node:net');
const { spawn } = require('node:child_process');
const out = path.resolve('docs/design/task-v2');
fs.mkdirSync(out, { recursive: true });
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
async function until(test) { const deadline = Date.now() + 10000; while (!test()) { if (Date.now() > deadline) throw new Error('Timed out waiting for test condition'); await sleep(20); } }
async function port() { const socket = net.createServer(); await new Promise(resolve => socket.listen(0, '127.0.0.1', resolve)); const p = socket.address().port; await new Promise(resolve => socket.close(resolve)); return p; }
const accounts = [{ id: 'account-a', name: '张三', username: '13811112222', status: 'connected' }, { id: 'account-b', name: '李四', username: '13833334444', status: 'connected' }];
const models = [{ id: 'model-a', name: 'DeepSeek 日常学习', endpoint: 'https://api.deepseek.com', models: ['deepseek-chat', 'deepseek-reasoner'], primary: 'deepseek-chat', fallback: 'deepseek-reasoner', has_key: true }, { id: 'model-b', name: '通义学习助手', endpoint: 'https://api.example.com/v1', models: ['qwen-plus', 'qwen-turbo'], primary: 'qwen-plus', fallback: 'qwen-turbo', has_key: true }];
(async () => {
  const listenPort = await port(), base = `http://127.0.0.1:${listenPort}`;
  const service = spawn(path.resolve('dist/backend/tiantianxiangshang.exe'), ['-addr', `127.0.0.1:${listenPort}`, '-data', path.resolve('.cache/task-qa', String(Date.now())), '-web', path.resolve('dist/web')], { windowsHide: true, stdio: ['ignore', 'ignore', 'pipe'] });
  let startupError = '';
  service.stderr.on('data', b => startupError += b.toString());
  service.on('error', e => startupError += e.message);
  let browser;
  try {
    let ready = false;
    for (let i = 0; i < 100; i++) { try { const r = await fetch(base + '/api/tasks'); if (r.ok) { assert.deepEqual(await r.json(), []); ready = true; break; } } catch {} await sleep(50); }
    assert(ready, 'Local task API did not start: ' + startupError);
    browser = await chromium.launch({ executablePath: process.env.BROWSER_PATH || 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', headless: true });
    const context = await browser.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2 });
    const page = await context.newPage(), errors = [];
    page.on('pageerror', error => errors.push(error.message));
    let config = { accounts: structuredClone(accounts), models: structuredClone(models), default_account_id: 'account-a' };
    let tasks = [], submitted = [], holdB = false, releaseB, finishedB, progress = {};
    await page.route('**/api/**', async route => {
      const req = route.request(), p = new URL(req.url()).pathname.replace('/api', ''), method = req.method(), body = req.postDataJSON();
      let response;
      if (p === '/settings') response = config;
      else if (p === '/records') response = [];
      else if (p === '/task-progress') response = progress;
      else if (p === '/task-defaults') response = { account_id: config.default_account_id };
      else if (p === '/accounts/default') { config.default_account_id = body.id; response = config; }
      else if (/\/accounts\/[^/]+\/courses$/.test(p)) {
        if (p.includes('account-b') && holdB) { holdB = false; await new Promise(resolve => { releaseB = resolve; }); }
        response = { courses: p.includes('account-a') ? [{ id: '247370145', name: '工程检测加固', teacher: '王老师' }, { id: '24810562', name: '大学英语', teacher: '陈老师' }] : [{ id: '25888888', name: '高等数学', teacher: '李老师' }] };
      } else if (p === '/tasks' && method === 'GET') response = tasks;
      else if (p === '/tasks' && method === 'POST') { assert(!('password' in body) && !('api_key' in body)); const saved = { ...body, id: 'task-1', created_at: new Date().toISOString(), updated_at: new Date().toISOString() }; tasks.push(saved); submitted.push(structuredClone(saved)); response = saved; }
      else if (p === '/tasks/task-1' && method === 'PUT') { const saved = { ...body, id: 'task-1' }; tasks[0] = saved; submitted.push(structuredClone(saved)); response = saved; }
      else if (p === '/tasks/task-1' && method === 'DELETE') { tasks = []; response = { deleted: true }; }
      else throw new Error(`Unexpected task test API: ${method} ${p}`);
      try { await route.fulfill({ contentType: 'application/json', body: JSON.stringify(response) }); } finally { if (p.includes('account-b') && releaseB) finishedB?.(); }
    });
    async function select(id, text) {
      await page.locator('#' + id).click();
      const title = id === 'task-account' ? '选择学习账号' : '选择 AI 配置';
      await page.getByRole('radiogroup', { name: title, exact: true }).getByRole('radio').filter({ hasText: text }).click();
    }
    async function settle() { await page.evaluate(async () => { await Promise.all(document.getAnimations().filter(a => a.effect?.getTiming().iterations !== Infinity).map(a => a.finished.catch(() => {}))); }); }
    async function taskAction(label) {
      await page.waitForFunction(() => [...document.querySelectorAll('.el-overlay')].every(el => getComputedStyle(el).display === 'none' || el.getBoundingClientRect().height === 0));
      const card = page.locator('.saved-task-card').first();
      await card.scrollIntoViewIfNeeded();
      const bounds = await card.boundingBox();
      await page.mouse.move(bounds.x + 24, bounds.y + 48);
      await page.mouse.down();
      await page.getByRole('dialog', { name: '任务操作', exact: true }).waitFor({ state: 'visible' });
      await page.mouse.up();
      if (label) await page.getByRole('dialog', { name: '任务操作', exact: true }).getByRole('button', { name: label, exact: true }).click();
    }
    const accountBox = () => page.locator('#task-account');
    async function checkInvalid(button, id) {
      assert(await page.getByRole('button', { name: button, exact: true }).isEnabled());
      await page.getByRole('button', { name: button, exact: true }).click();
      assert.equal(await page.locator('#' + id).getAttribute('aria-invalid'), 'true');
      assert.equal(await page.evaluate(() => document.activeElement.id), id);
    }
    await page.goto(base + '/#home');
    await page.getByRole('button', { name: '新建学习任务', exact: true }).click();
    await checkInvalid('下一步 · AI 模型', 'task-name');
    await settle();
    await page.screenshot({ path: path.join(out, 'validation-name.png') });
    await page.getByLabel('任务名称', { exact: true }).fill('本学期学习计划');
    await page.getByRole('checkbox', { name: /工程检测加固/ }).waitFor();
    assert.match(await accountBox().innerText(), /张三/);
    assert.equal(await page.locator('.task-page input[type=password]').count(), 0);
    await page.getByRole('checkbox', { name: /工程检测加固/ }).check();
    await page.screenshot({ path: path.join(out, 'account-course.png') });
    await select('task-account', '李四');
    await page.getByRole('checkbox', { name: /高等数学/ }).waitFor();
    assert.match(await page.locator('.selected-count').innerText(), /0/);
    await checkInvalid('下一步 · AI 模型', 'task-courses');
    await page.getByRole('checkbox', { name: /高等数学/ }).check();
    holdB = true;
    await page.getByRole('button', { name: '刷新课程', exact: true }).click();
    await until(() => !!releaseB);
    await select('task-account', '张三');
    await page.getByRole('checkbox', { name: /工程检测加固/ }).waitFor();
    const lateFinished = new Promise(resolve => finishedB = resolve);
    releaseB();
    await lateFinished;
    await page.evaluate(() => new Promise(requestAnimationFrame));
    assert.equal(await page.getByRole('checkbox', { name: /高等数学/ }).count(), 0, 'stale response mixed courses from another account');
    await page.getByRole('checkbox', { name: /工程检测加固/ }).check();
    await accountBox().click();
    await page.getByRole('radiogroup', { name: '选择学习账号', exact: true }).waitFor({ state: 'visible' });
    await settle();
    await page.screenshot({ path: path.join(out, 'account-drawer.png') });
    await page.keyboard.press('Escape');
    await select('task-account', '李四');
    await page.getByRole('checkbox', { name: /高等数学/ }).check();
    await page.getByRole('button', { name: '下一步 · AI 模型', exact: true }).click();
    await checkInvalid('下一步 · 运行设置', 'task-model-config');
    assert.equal(await page.getByLabel('API Key', { exact: true }).count(), 0);
    await page.getByRole('button', { name: '管理配置', exact: true }).click();
    await page.getByRole('heading', { name: '你的模型配置' }).waitFor();
    await page.getByRole('button', { name: '返回任务', exact: true }).click();
    await select('task-model-config', '通义学习助手');
    await page.locator('.task-model-summary').getByText('qwen-plus', { exact: true }).waitFor();
    await page.screenshot({ path: path.join(out, 'model-config.png') });
    await page.locator('#task-model-config').click();
    await page.getByRole('radiogroup', { name: '选择 AI 配置', exact: true }).waitFor({ state: 'visible' });
    await settle();
    await page.screenshot({ path: path.join(out, 'model-drawer.png') });
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: '下一步 · 运行设置', exact: true }).click();
    await page.getByLabel('最多重试（次）', { exact: true }).fill('');
    await page.getByRole('button', { name: '保存任务', exact: true }).click();
    await page.locator('#error-task-retries').waitFor({ state:'visible' });
    assert.equal(await page.evaluate(() => document.activeElement.id), 'task-retries');
    assert.equal(submitted.length, 0);
    await page.getByLabel('最多重试（次）', { exact: true }).fill('2');
    await page.getByLabel('视频速度', { exact: true }).fill('1.5');
    assert(await page.getByLabel('章节并发数', { exact: true }).isDisabled());
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({ path: path.join(out, 'run-settings.png') });
    const centers = await page.locator('.step-index').evaluateAll(elements => elements.map(el => { const b = el.getBoundingClientRect(); return { x:b.x+b.width/2, y:b.y+b.height/2 }; }));
    assert(Math.abs((centers[1].x-centers[0].x)-(centers[2].x-centers[1].x)) < 1, 'step spacing differs');
    assert(centers.every(c => Math.abs(c.y-centers[0].y)<1), 'step circles are misaligned');
    await page.getByRole('button', { name: '保存任务', exact: true }).click();
    await page.locator('.saved-task-card h3').filter({ hasText: '本学期学习计划' }).waitFor();
    assert.equal(submitted[0].account_id, 'account-b');
    assert.equal(submitted[0].model_config_id, 'model-b');
    assert.equal(submitted[0].courses[0].id, '25888888');
    assert.equal(config.default_account_id, 'account-a');
    await page.waitForFunction(() => !document.querySelector('.el-message'));
    await page.screenshot({ path: path.join(out, 'home.png') });
    const compactCard = page.locator('.saved-task-card').first();
    assert((await compactCard.boundingBox()).height < 215, 'task card did not become shorter');
    assert.match(await compactCard.innerText(), /高等数学/);
    assert(!/课程 ID|25888888/.test(await compactCard.innerText()), 'course ID still shown instead of its name');
    assert.equal(await compactCard.getByRole('button', { name: '编辑任务', exact: true }).count(), 0);
    await compactCard.click({ position: { x: 25, y: 48 } });
    assert.equal(await page.getByRole('dialog', { name: '任务操作', exact: true }).count(), 0, 'short tap opened task actions');
    const compactBounds = await compactCard.boundingBox();
    await page.mouse.move(compactBounds.x + 24, compactBounds.y + 48);
    await page.mouse.down();
    await page.mouse.move(compactBounds.x + 24, compactBounds.y + 80);
    await page.mouse.up();
    await page.waitForTimeout(550);
    assert.equal(await page.getByRole('dialog', { name: '任务操作', exact: true }).count(), 0, 'moving to scroll triggered a long press');
    await taskAction();
    await settle();
    await page.screenshot({ path: path.join(out, 'task-actions.png') });
    await page.getByRole('dialog', { name: '任务操作', exact: true }).getByRole('button', { name: '取消', exact: true }).click();
    progress['task-1'] = { state: 'running', course_name: '高等数学', chapter_name: '第 2 节', position_seconds: 6, duration_seconds: 357 };
    await page.locator('.learning-latest-progress').filter({ hasText: '00:06 / 05:57' }).waitFor();
    progress['task-1'] = { ...progress['task-1'], position_seconds: 12 };
    await page.locator('.learning-latest-progress').filter({ hasText: '00:12 / 05:57' }).waitFor();
    assert(!/00:06/.test(await page.locator('.learning-latest-progress').innerText()), 'old progress lines accumulated on the card');
    await page.screenshot({ path: path.join(out, 'home-progress.png') });
    await page.reload();
    await taskAction('编辑任务');
    assert.match(await accountBox().innerText(), /李四/, 'editing replaced selected account with the global default');
    await page.getByRole('checkbox', { name: /高等数学/ }).waitFor();
    assert(await page.getByRole('checkbox', { name: /高等数学/ }).isChecked());
    await page.getByRole('button', { name: '手动填写 ID', exact: true }).click();
    await page.getByLabel('课程 ID', { exact: true }).fill('bad-id');
    await checkInvalid('下一步 · AI 模型', 'task-courses');
    await page.getByRole('button', { name: '课程列表', exact: true }).click();
    await checkInvalid('下一步 · AI 模型', 'task-courses');
    await page.getByRole('button', { name: '手动填写 ID', exact: true }).click();
    await page.getByLabel('课程 ID', { exact: true }).fill('25888888, 25888888 25999999');
    assert.match(await page.locator('.selected-count').innerText(), /2/);
    await page.getByRole('button', { name: '下一步 · AI 模型', exact: true }).click();
    assert.match(await page.locator('.task-model-summary').innerText(), /qwen-plus/);
    await page.getByRole('button', { name: '下一步 · 运行设置', exact: true }).click();
    await page.getByRole('button', { name: '保存任务', exact: true }).click();
    await page.locator('.saved-task-card').waitFor();
    assert.equal(tasks.length, 1);
    assert.equal(tasks[0].courses.length, 2);
    config.models = [config.models[0]];
    await page.reload();
    await page.getByText('模型配置已移除，请重新选择', { exact: true }).waitFor();
    await taskAction('编辑任务');
    await page.getByRole('button', { name: '下一步 · AI 模型', exact: true }).click();
    await page.getByText('原模型配置已被移除，请重新选择。', { exact: true }).waitFor();
    await checkInvalid('下一步 · 运行设置', 'task-model-config');
    await select('task-model-config', 'DeepSeek 日常学习');
    await page.getByRole('button', { name: '上一步', exact: true }).click();
    for (const width of [320, 390, 480, 1024]) {
      await page.setViewportSize({ width, height: 844 });
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `horizontal overflow at ${width}`);
    }
    await page.setViewportSize({ width: 390, height: 844 });
    await page.getByRole('button', { name: '返回首页', exact: true }).click();
    await taskAction('删除任务');
    await page.getByRole('dialog', { name: '删除任务', exact: true }).getByRole('button', { name: '取消', exact: true }).click();
    assert.equal(tasks.length, 1, 'cancelling deletion removed the task');
    await taskAction('删除任务');
    await page.getByRole('dialog', { name: '删除任务', exact: true }).getByRole('button', { name: '删除', exact: true }).click();
    await page.getByText('创建你的第一个学习任务', { exact: true }).waitFor();
    assert.equal(tasks.length, 0);
    config.accounts = [];
    config.models = [];
    config.default_account_id = '';
    await page.goto(base + '/#task');
    await page.reload();
    await page.getByRole('button', { name: '先去添加账号', exact: true }).waitFor();
    await page.getByRole('button', { name: '先去添加账号', exact: true }).click();
    await page.getByRole('button', { name: '连接其他账号', exact: true }).waitFor();
    config.accounts = Array.from({ length: 16 }, (_, i) => ({ id: 'account-extra-' + i, name: '示例账号 ' + (i + 1), username: '1390000' + String(1000 + i), status: 'connected' }));
    config.models = Array.from({ length: 12 }, (_, i) => ({ ...models[0], id: 'model-extra-' + i, name: '示例模型 ' + (i + 1) }));
    config.default_account_id = 'account-extra-15';
    await page.reload();
    await page.locator('#task-account').click();
    await page.waitForFunction(() => document.querySelector('.task-picker-sheet .picker-list')?.scrollTop > 0);
    const visiblePicker = () => page.locator('.task-picker-sheet:visible');
    const accountFooterBefore = await visiblePicker().locator('.picker-footer').boundingBox();
    assert(await visiblePicker().locator('.picker-list').evaluate(el => el.scrollHeight > el.clientHeight));
    await visiblePicker().locator('.picker-list').hover();
    await page.mouse.wheel(0, -800);
    const accountFooterAfter = await visiblePicker().locator('.picker-footer').boundingBox();
    assert.equal(accountFooterBefore.y, accountFooterAfter.y);
    assert(accountFooterAfter.y + accountFooterAfter.height <= 844);
    await visiblePicker().getByRole('textbox').fill('示例账号 12');
    assert.equal(await visiblePicker().getByRole('radio').count(), 1);
    await visiblePicker().getByRole('radio').click();
    assert.match(await accountBox().innerText(), /示例账号 12/);
    assert.equal(config.default_account_id, 'account-extra-15');
    await page.getByLabel('任务名称', { exact: true }).fill('长列表选择测试');
    await page.getByRole('checkbox', { name: /高等数学/ }).check();
    await page.getByRole('button', { name: '下一步 · AI 模型', exact: true }).click();
    await page.locator('#task-model-config').click();
    assert(await visiblePicker().locator('.picker-list').evaluate(el => el.scrollHeight > el.clientHeight));
    await visiblePicker().getByRole('textbox').fill('示例模型 12');
    await visiblePicker().getByRole('radio').click();
    assert.match(await page.locator('#task-model-config').innerText(), /示例模型 12/);
    assert.deepEqual(errors, []);
    console.log('PASS: account/model drawer selection, defaults, stale request isolation, management return, task CRUD, reload, missing refs, responsive and empty states.');
    // Review board uses actual screenshots with isolated sample data.
    const board = await context.newPage();
    await board.setViewportSize({ width: 1710, height: 1070 });
    const panels = [['home.png', '01 学习首页'], ['account-drawer.png', '02 账号抽屉单选'], ['model-drawer.png', '03 AI 配置抽屉单选'], ['run-settings.png', '04 运行设置']];
    const content = panels.map(([file, title]) => `<section><h2>${title}</h2><img src="data:image/png;base64,${fs.readFileSync(path.join(out, file)).toString('base64')}"></section>`).join('');
    await board.setContent(`<html lang="zh-CN"><meta charset="utf-8"><style>*{box-sizing:border-box}body{margin:0;background:#eef0f3;color:#292e36;font-family:'Microsoft YaHei',sans-serif;padding:36px}h1{font-size:27px;margin:0 0 8px}p{font-size:12px;color:#8793a3;margin:0 0 24px}.panels{display:grid;grid-template-columns:repeat(4,1fr);gap:20px}h2{font-size:14px;margin:0 0 13px}img{width:100%;border:1px solid #dde3eb;border-radius:22px;box-shadow:0 10px 30px #24304408}footer{margin-top:18px;font-size:11px;color:#8c98a7}</style><h1>天天向上 · 任务配置 UI 03</h1><p>选择已有账号与 AI 配置，统一在“我的”管理。</p><div class="panels">${content}</div><footer>真实页面截图 · 使用隔离的模拟账号与课程数据</footer></html>`);
    await board.locator('img').last().waitFor();
    await board.evaluate(() => Promise.all([...document.images].map(img => img.decode())));
    await board.screenshot({ path: path.join(out, 'task-ui-v2.png'), fullPage: true });
    await context.close();
  } finally { if (browser) await browser.close(); service.kill(); }
})().catch(e => { console.error(e); process.exitCode = 1; });

