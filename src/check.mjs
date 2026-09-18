import { chromium } from '/home/washka/project/systemburo/frontend/node_modules/playwright/index.mjs';
const b = await chromium.launch();
const p = await b.newPage({ viewport: { width: 794, height: 1123 } });
await p.goto('file://' + process.cwd() + '/report.html');
console.log(await p.evaluate(() => {
  const out = [];
  document.querySelectorAll('.page').forEach((pg, i) => {
    if (pg.scrollHeight > pg.clientHeight + 1) out.push(`стр ${i+1}: переполнение на ${pg.scrollHeight - pg.clientHeight}px`);
    const r = pg.getBoundingClientRect();
    pg.querySelectorAll('.note, table, .verdict, .bars, .kpi, h2').forEach(el => {
      const bb = el.getBoundingClientRect();
      if (bb.bottom > r.bottom - 26) out.push(`стр ${i+1}: ${el.className || el.tagName} ниже границы (${Math.round(bb.bottom - r.top)})`);
    });
  });
  return out.length ? out.join('\n') : 'переполнений нет';
}));
await b.close();
