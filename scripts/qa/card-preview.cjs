const { chromium } = require('playwright-core');
const fs = require('node:fs');
const path = require('node:path');
(async () => {
  const browser = await chromium.launch({ executablePath: process.env.BROWSER_PATH || 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 940, height: 1120 }, deviceScaleFactor: 2 });
    const panels = [['home-progress.png', '01 课程名称与最新进度'], ['task-actions.png', '02 长按卡片显示操作']];
    const content = panels.map(([file, title]) => `<section><h2>${title}</h2><img src="data:image/png;base64,${fs.readFileSync(path.resolve('docs/design/task-v2', file)).toString('base64')}"></section>`).join('');
    await page.setContent(`<html lang="zh-CN"><meta charset="utf-8"><style>*{box-sizing:border-box}body{margin:0;padding:32px;background:#eef0f3;color:#292e36;font-family:'Microsoft YaHei',sans-serif}h1{font-size:25px;margin:0 0 9px}p,footer{font-size:11px;color:#8b97a7;line-height:1.8;margin:0 0 22px}.panels{display:grid;grid-template-columns:1fr 1fr;gap:28px}h2{font-size:13px;margin:0 0 14px}img{display:block;width:100%;border-radius:23px;border:1px solid #dde3eb;box-shadow:0 12px 30px #2430440a}footer{margin:18px 0 0}</style><h1>天天向上 · 任务卡片 UI 04</h1><p>适度收矮，保留留白 · 长按管理 · 只展示最新进度</p><div class="panels">${content}</div><footer>真实界面截图，运行进度为隔离测试数据；实际日志仍需后台执行器提供。</footer></html>`);
    await page.evaluate(() => Promise.all([...document.images].map(img => img.decode())));
    await page.screenshot({ path: path.resolve('docs/design/task-card-v4.png'), fullPage: true });
  } finally { await browser.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
