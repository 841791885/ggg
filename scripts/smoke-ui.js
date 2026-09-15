/**
 * 前端冒烟测试：用真实浏览器逐页访问 + 走一遍交易链路，抓 JS 报错。
 *
 * 为什么需要它：本项目历史上一半的 bug 是"接口改了字段、前端还在读旧字段"
 * 或"事件绑错元素"这类只在运行时暴露的问题。curl 测 API 全绿不代表页面能用。
 *
 * 用法：
 *   1. 后端在跑（go run . 或 air）2. Next 在跑（cd web && npm run dev）
 *   3. node scripts/smoke-ui.js（默认连 http://localhost:8088，可用 SMOKE_BASE 覆盖）
 * 依赖 puppeteer：已作为 web/ 的 devDependency 安装（npm i --prefix web 即可）。
 * 若报 Cannot find module 'puppeteer'，在项目根执行：
 *   NODE_PATH=web/node_modules node scripts/smoke-ui.js
 */
const puppeteer = require('puppeteer');
const sleep = ms => new Promise(r => setTimeout(r, ms));
(async () => {
  const b = await puppeteer.launch({ headless: true, executablePath: process.env.CHROME_PATH ?? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' });
  const page = await b.newPage();
  const errors = [];
  page.on('pageerror', e => errors.push('PAGEERROR: ' + e.message));
  page.on('console', m => { if (m.type() === 'error') errors.push('CONSOLE: ' + m.text().slice(0,150)); });
  const log = [];
  const snap = async (name) => {
    const t = await page.evaluate(() => document.body.innerText.replace(/\s+/g,' ').slice(0,150));
    log.push(`[${name}] ${t}`);
  };
  const go = async (path, name) => { await page.goto('http://localhost:8088'+path, {waitUntil:'networkidle0', timeout:30000}); await sleep(1200); await snap(name); };

  await page.goto((process.env.SMOKE_BASE ?? 'http://localhost:8088') + '/', { waitUntil: 'networkidle0', timeout: 30000 });
  await page.evaluate(() => localStorage.clear());
  await page.reload({ waitUntil: 'networkidle0' });
  await sleep(800);
  // 登录
  const btn = await page.$('button[type=submit]');
  if (btn) { await btn.click(); await sleep(2500); }
  await snap('1-登录后');

  await go('/shop', '2-商城');
  await go('/cart', '3-购物车');
  await go('/checkout', '4-结算');
  await go('/orders', '5-订单');
  await go('/coupons', '6-优惠券');
  await go('/addresses', '7-地址');
  await go('/notifications', '8-通知');
  await go('/products', '9-商品管理');
  await go('/tasks', '10-后台任务');
  await go('/dashboard', '11-概览');
  await go('/refunds', '12-退款');

  console.log(log.join('\n'));
  console.log('\n=== JS 错误 ===\n' + (errors.length ? [...new Set(errors)].join('\n') : '无'));
  await b.close();
})();
