import { chromium } from '/home/washka/project/systemburo/frontend/node_modules/playwright/index.mjs';
const b = await chromium.launch();
const p = await b.newPage({ viewport: { width: 794, height: 1123 } });
await p.goto('file://' + process.cwd() + '/facts.html');
console.log(await p.evaluate(() => {
  const out = [];
  document.querySelectorAll('.page').forEach((pg, i) => {
    const r = pg.getBoundingClientRect();
    pg.querySelectorAll('.card, .grid, .lead, h1').forEach(el => {
      const bb = el.getBoundingClientRect();
      if (bb.bottom > r.bottom - 30) out.push(`стр ${i+1}: "${(el.textContent||'').trim().slice(0,28)}" ниже границы (низ ${Math.round(bb.bottom - r.top)} из 1123)`);
    });
  });
  return out.length ? out.join('\n') : 'всё внутри страниц';
}));
await b.close();
