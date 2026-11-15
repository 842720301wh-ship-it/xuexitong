const { chromium } = require('playwright-core');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const base = process.env.TIANTIAN_QA_URL || 'http://127.0.0.1:18081';
const out = path.resolve('docs/design/navigation-v1');
fs.mkdirSync(out, { recursive: true });
(async () => {
  const browser = await chromium.launch({ executablePath: process.env.BROWSER_PATH || 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', headless: true });
  try {
    const context = await browser.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2 });
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', e => errors.push(e.message));
    // Verify the built UI against the real local service before adding isolated fixtures.
    await page.goto(base + '/#my');
    await page.getByRole('navigation', { name: '我的菜单' }).waitFor();
    assert.equal(await page.locator('.my-menu-item').count(), 5);
    await page.getByRole('button', { name: '开源许可 查看源码与许可说明' }).click();
    assert.equal(await page.getByRole('link', { name: '访问开源仓库' }).getAttribute('href'), 'https://github.com/842720301wh-ship-it/xuexitong.git');
    await page.getByRole('button', { name: '返回我的' }).click();
    for (const [name, target] of [['赞助 自愿支持项目维护', '如果它帮到了你'], ['免责声明 免费使用与使用约定', '免费使用，规范使用'], ['联系我们 问题反馈与交流', '一起把它变得更好']]) {
      await page.getByRole('button', { name, exact: true }).click();
      await page.getByRole('heading', { name: target }).waitFor();
      if (await page.locator('img').count()) assert(await page.locator('img').evaluateAll(imgs => imgs.every(i => i.complete && i.naturalWidth > 0)));
      await page.getByRole('button', { name: '返回我的' }).click();
    }
    let config = { accounts: [], default_account_id: '', models: [] };
    let seq = 0, failCheck = false, failModel = false;
    await page.route('**/api/**', async route => {
      const req = route.request(), url = new URL(req.url()), method = req.method(), body = req.postDataJSON();
      const p = url.pathname.replace('/api', '');
      let value, status = 200;
      if (p === '/settings') value = config;
      else if (p === '/records') value = [];
      else if (p === '/models/discover') value = { models: body.endpoint.includes('deepseek') ? ['deepseek-chat', 'deepseek-reasoner'] : ['example-chat'], message: '连接成功，模型列表已获取' };
      else if (p === '/models/probe') value = { message: '主模型调用成功' };
      else if (p === '/task-progress') value = {};
      else if (p === '/tasks' && method === 'GET') value = [];
      else if (p === '/accounts' && method === 'POST') {
        if (body.password === 'wrong') { status = 400; value = { error: '学习平台登录失败，请检查账号密码' }; }
        else { const a = { id: 'account-' + ++seq, name: body.username === '13811112222' ? '张三' : '李四', username: body.username, status: 'connected' }; config.accounts.push(a); config.default_account_id ||= a.id; value = config; }
      } else if (p === '/accounts/default') { config.default_account_id = body.id; value = config; }
      else if (p.endsWith('/check')) { value = failCheck ? { error: '学习平台连接超时' } : config; status = failCheck ? 400 : 200; }
      else if (p.endsWith('/courses')) value = { courses: [{ id: '247370145', name: '工程检测加固', teacher: '示例教师' }] };
      else if (p.startsWith('/accounts/') && method === 'DELETE') { config.accounts = config.accounts.filter(a => a.id !== p.split('/')[2]); if (!config.accounts.some(a => a.id === config.default_account_id)) config.default_account_id = config.accounts[0]?.id || ''; value = config; }
      else if (p.endsWith('/test')) { status = failModel ? 400 : 200; value = failModel ? { error: '模型接口返回 HTTP 401，请检查 Key' } : { message: '连接成功，主模型可用' }; }
      else if (p.startsWith('/models') && ['POST', 'PUT'].includes(method)) {
        const m = { ...body, id: p.split('/')[2] || 'model-' + ++seq, has_key: true }; delete m.api_key;
        const index = config.models.findIndex(x => x.id === m.id); if (index < 0) config.models.push(m); else config.models[index] = m; value = config;
      } else if (p.startsWith('/models/') && method === 'DELETE') { config.models = config.models.filter(m => m.id !== p.split('/')[2]); value = config; }
      else throw new Error('Unexpected route ' + method + ' ' + p);
      await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(value) });
    });
    await page.reload();
    await page.getByRole('button', { name: '账号资料' }).click();
    await page.getByRole('button', { name: '连接其他账号' }).click();
    await page.getByLabel('手机号 / 学号', { exact: true }).fill('13811112222');
    await page.getByLabel('登录密码', { exact: true }).fill('wrong');
    await page.getByRole('button', { name: '连接并保存', exact: true }).click();
    await page.getByText('学习平台登录失败，请检查账号密码', { exact: true }).waitFor();
    assert.equal(config.accounts.length, 0);
    await page.getByLabel('登录密码', { exact: true }).fill('qa-secret');
    await page.getByRole('button', { name: '连接并保存', exact: true }).click();
    await page.locator('.profile-copy strong').filter({ hasText: '张三' }).waitFor();
    const firstID = config.default_account_id;
    await page.getByRole('button', { name: '账号资料' }).click();
    await page.getByRole('button', { name: '连接其他账号' }).click();
    await page.getByLabel('手机号 / 学号', { exact: true }).fill('13833334444');
    await page.getByLabel('登录密码', { exact: true }).fill('qa-secret-2');
    await page.getByRole('button', { name: '连接并保存', exact: true }).click();
    await page.getByText('账号已连接并保存', { exact: true }).last().waitFor();
    assert.equal(config.default_account_id, firstID);
    await page.getByRole('button', { name: '账号资料' }).click();
    await page.getByRole('button', { name: /李四.*138/ }).click();
    await page.locator('.profile-copy strong').filter({ hasText: '李四' }).waitFor();
    await page.reload();
    await page.locator('.profile-copy strong').filter({ hasText: '李四' }).waitFor();
    await page.screenshot({ path: path.join(out, 'my.png') });
    const originalAccounts = config.accounts.slice();
    const originalDefault = config.default_account_id;
    for (let i = 0; i < 14; i++) config.accounts.push({ id: 'extra-' + i, name: '示例同学 ' + (i + 1), username: '1390000' + String(1000 + i), status: 'connected' });
    config.default_account_id = 'extra-13';
    await page.reload();
    await page.getByRole('button', { name: '账号资料' }).click();
    await page.waitForFunction(() => document.querySelector('.account-list')?.scrollTop > 0);
    const listBounds = await page.locator('.account-list').boundingBox();
    assert(listBounds.height <= 240 && listBounds.height < 844 * 0.33);
    const fixedActionBefore = await page.getByRole('button', { name: '连接其他账号', exact: true }).boundingBox();
    assert(fixedActionBefore.y + fixedActionBefore.height < 844);
    await page.screenshot({ path: path.join(out, 'accounts-many.png') });
    await page.locator('.account-list').hover();
    await page.mouse.wheel(0, -800);
    const fixedActionAfter = await page.getByRole('button', { name: '连接其他账号', exact: true }).boundingBox();
    assert.equal(fixedActionBefore.y, fixedActionAfter.y, 'actions moved when scrolling account list');
    config.accounts = originalAccounts;
    config.default_account_id = originalDefault;
    await page.reload();
    await page.getByRole('button', { name: '账号资料' }).click();
    await page.getByRole('button', { name: '检查连接', exact: true }).click();
    await page.getByText('账号连接正常', { exact: true }).waitFor();
    failCheck = true;
    await page.getByRole('button', { name: '检查连接', exact: true }).click();
    await page.getByText('学习平台连接超时', { exact: true }).waitFor();
    await page.waitForFunction(() => !document.querySelector('.el-message'));
    await page.screenshot({ path: path.join(out, 'accounts.png') });
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: 'AI 模型 保存与管理多个模型配置' }).click();
    await page.getByRole('button', { name: '添加模型配置', exact: true }).click();
    await page.getByLabel('配置名称', { exact: true }).fill('DeepSeek 主账号');
    await page.getByLabel('API 接口地址').fill('https://api.deepseek.com');
    await page.getByLabel('API Key', { exact: true }).fill('qa-model-key');
    await page.getByRole('button', { name: '连接测试', exact: true }).click();
    await page.getByText('已获取 2 个模型，可在下方搜索选择', { exact: true }).waitFor();
    await page.locator('.el-select').filter({ has: page.locator('#primary-model') }).click();
    await page.locator('#' + await page.locator('#primary-model').getAttribute('aria-controls')).getByRole('option', { name: 'deepseek-chat', exact: true }).click();
    await page.locator('.el-select').filter({ has: page.locator('#fallback-model') }).click();
    await page.locator('#' + await page.locator('#fallback-model').getAttribute('aria-controls')).getByRole('option', { name: 'deepseek-reasoner', exact: true }).click();
    await page.getByRole('button', { name: '保存配置', exact: true }).click();
    await page.locator('.model-card h3').filter({ hasText: 'DeepSeek 主账号' }).waitFor();
    await page.getByRole('button', { name: '测试连接', exact: true }).click();
    await page.getByText('连接成功，主模型可用', { exact: true }).waitFor();
    failModel = true;
    await page.getByRole('button', { name: '测试连接', exact: true }).click();
    await page.getByText('模型接口返回 HTTP 401，请检查 Key', { exact: true }).waitFor();
    await page.getByRole('button', { name: '编辑', exact: true }).click();
    assert.equal(await page.getByLabel('API Key', { exact: true }).inputValue(), '');
    await page.getByLabel('配置名称', { exact: true }).fill('DeepSeek 日常学习');
    await page.screenshot({ path: path.join(out, 'model-editor.png') });
    await page.getByRole('button', { name: '保存配置', exact: true }).click();
    await page.locator('.model-card h3').filter({ hasText: 'DeepSeek 日常学习' }).waitFor();
    await page.getByRole('button', { name: '添加模型配置', exact: true }).click();
    await page.getByLabel('配置名称', { exact: true }).fill('备用模型配置');
    await page.getByLabel('API 接口地址').fill('https://api.example.com/v1');
    await page.getByLabel('API Key', { exact: true }).fill('qa-second-key');
    await page.getByRole('button', { name: '连接测试', exact: true }).click();
    await page.getByText('已获取 1 个模型，可在下方搜索选择', { exact: true }).waitFor();
    await page.locator('.el-select').filter({ has: page.locator('#primary-model') }).click();
    await page.locator('#' + await page.locator('#primary-model').getAttribute('aria-controls')).getByRole('option', { name: 'example-chat', exact: true }).click();
    await page.getByRole('button', { name: '保存配置', exact: true }).click();
    await page.locator('.model-card h3').filter({ hasText: '备用模型配置' }).waitFor();
    assert.equal(config.models.length, 2);
    await page.waitForFunction(() => !document.querySelector('.el-message'));
    await page.screenshot({ path: path.join(out, 'models.png') });
    await page.locator('.model-card').filter({ hasText: '备用模型配置' }).getByRole('button', { name: '删除', exact: true }).click();
    await page.getByRole('dialog').getByRole('button', { name: '删除', exact: true }).click();
    await page.waitForFunction(() => document.querySelectorAll('.model-card').length === 1);
    await page.locator('.model-card').getByRole('button', { name: '删除', exact: true }).click();
    await page.getByRole('button', { name: '取消', exact: true }).click();
    assert.equal(config.models.length, 1);
    await page.locator('.model-card').getByRole('button', { name: '删除', exact: true }).click();
    await page.getByRole('dialog').getByRole('button', { name: '删除', exact: true }).click();
    await page.getByText('还没有模型配置', { exact: true }).waitFor();
    assert.equal(config.models.length, 0);
    await page.getByRole('button', { name: '返回我的' }).click();
    await page.getByRole('button', { name: '账号资料' }).click();
    await page.getByRole('button', { name: '移除本机账号', exact: true }).click();
    await page.getByRole('button', { name: '移除', exact: true }).click();
    await page.getByText('已移除本机账号', { exact: true }).waitFor();
    assert.equal(config.default_account_id, firstID);
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: '学习', exact: true }).click();
    await page.getByRole('button', { name: '记录', exact: true }).click();
    await page.getByText('暂无运行记录', { exact: true }).waitFor();
    for (const width of [320, 390, 480, 1024]) {
      await page.setViewportSize({ width, height: 844 });
      await page.goto(base + '/#my');
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `horizontal overflow at ${width}`);
    }
    assert.deepEqual(errors, []);
    console.log('PASS: real navigation + isolated multi-account, defaults, reload, errors, model CRUD/testing, deletion, courses and responsive layouts.');
    await context.close();
  } finally { await browser.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });

