/**
 * Замки конвенции слоёв поверхностей (#1415 -> разъезд тёмной темы).
 *
 * Слоя три: --bg (фон рабочей области), --surface (карточка на нём), --surface-2
 * (подложка внутри карточки). До этой правки --bg и --surface в светлой теме
 * совпадали, поэтому «страница покрашена цветом карточки» и «карточка без своего
 * фона» выглядели одинаково правильно и копились годами - вылезло всё разом в
 * тёмной теме («Обзор и новости» серый лист, серые плашки кнопок BlankSelector).
 *
 * Отсюда две проверки: палитра обязана держать слои разведёнными в КАЖДОЙ теме,
 * а страница - не красить себя цветом карточки.
 */
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

const ROOT = path.resolve(__dirname, '../../..');
const SRC = path.join(ROOT, 'src');
const TOKENS = fs.readFileSync(path.join(SRC, 'assets/tokens.css'), 'utf8');

/** Значения переменных внутри блока темы. */
function palette(theme) {
  const re = new RegExp(`\\[data-theme="${theme}"\\][^{]*\\{([\\s\\S]*?)\\n\\}`, 'm');
  const block = TOKENS.match(re);
  if (!block) throw new Error(`палитра ${theme} не найдена в tokens.css`);
  const vars = {};
  for (const m of block[1].matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)) vars[m[1]] = m[2].trim();
  return vars;
}

const THEMES = ['light', 'dark'];

describe('слои поверхностей в палитрах', () => {
  it.each(THEMES)('%s: слои объявлены', (theme) => {
    const p = palette(theme);
    for (const name of ['--bg', '--surface', '--surface-2']) {
      expect(p[name], `${theme}: ${name}`).toBeTruthy();
    }
  });

  // В светлой теме фон страницы и карточка намеренно одинаково белые, поэтому
  // разведение требуем только там, где оно есть по замыслу. Ошибку выбора слоя
  // ловит не разница цветов, а статические проверки ниже - они работают в обеих.
  it('тёмная: фон страницы и карточка - разные цвета', () => {
    const p = palette('dark');
    expect(p['--bg']).not.toBe(p['--surface']);
  });

  it.each(THEMES)('%s: подложка внутри карточки отличается от самой карточки', (theme) => {
    const p = palette(theme);
    expect(p['--surface-2']).not.toBe(p['--surface']);
  });

  it('декор полноэкранных экранов в тёмной теме слабее, чем в светлой', () => {
    const light = parseInt(palette('light')['--decor-mix'], 10);
    const dark = parseInt(palette('dark')['--decor-mix'], 10);
    expect(Number.isFinite(light) && Number.isFinite(dark)).toBe(true);
    // Та же примесь на тёмном фоне весит больше и красит экран целиком (замер
    // страницы техработ: синева B-R +27 против +10 у обычного фона).
    expect(dark).toBeLessThan(light);
  });

  it('консоль техработ берёт тон из темы, а не из литерала', () => {
    for (const theme of THEMES) expect(palette(theme)['--console-bg']).toBeTruthy();
    const files = ['views/Maintenance.vue', 'views/Error500.vue'];
    for (const f of files) {
      const css = fs.readFileSync(path.join(SRC, f), 'utf8');
      expect(css, `${f}: тёмно-синий литерал консоли`).not.toMatch(/background:\s*#0f1129/);
    }
  });
});

/** Компоненты, которые роутер рендерит как страницу (а не как карточку внутри shell). */
function routedViews() {
  const router = fs.readFileSync(path.join(SRC, 'router.js'), 'utf8');
  const files = new Set();
  for (const m of router.matchAll(/import\(['"](?:\.|@)\/(views\/[^'"]+\.vue)['"]\)/g)) files.add(m[1]);
  for (const m of router.matchAll(/^import\s+\w+\s+from\s+['"](?:\.|@)\/(views\/[^'"]+\.vue)['"]/gm)) files.add(m[1]);
  return [...files];
}

/** Первый класс корневого элемента шаблона. */
function rootClass(src) {
  const tpl = src.match(/<template>([\s\S]*?)\n<\/template>/);
  if (!tpl) return null;
  const cls = tpl[1].match(/class="([^"{]+)"/);
  return cls ? cls[1].trim().split(/\s+/)[0] : null;
}

/** Значения background в правилах ровно этого класса. */
function rootBackgrounds(src, cls) {
  const out = [];
  const re = new RegExp(`(^|\\n)\\s*\\.${cls}\\s*\\{([^}]*)\\}`, 'g');
  for (const rule of src.matchAll(re)) {
    for (const b of rule[2].matchAll(/background(?:-color)?\s*:\s*([^;]+);/g)) out.push(b[1].trim());
  }
  return out;
}

describe('страницы не красят себя цветом карточки', () => {
  const views = routedViews();

  it('роутер отдаёт непустой список страниц', () => {
    expect(views.length).toBeGreaterThan(10);
  });

  it.each(routedViews())('%s', (file) => {
    const src = fs.readFileSync(path.join(SRC, file), 'utf8');
    // Страницы-карточки внутри AdminPageShell лежат НА фоне, им --surface положен.
    if (src.includes('AdminPageShell')) return;
    const cls = rootClass(src);
    if (!cls) return;
    for (const bg of rootBackgrounds(src, cls)) {
      expect(
        bg,
        `${file}: корень страницы красится цветом карточки - фон страницы даёт body (--bg)`,
      ).not.toMatch(/var\(--surface\)|var\(--color-bg\)|var\(--accent-tint\)/);
    }
  });
});

/*
 * Карточки, у которых рамка была, а фона не было: пока фон страницы был белым, они
 * выглядели белыми и без него. Список собран браузерным сканом (элемент с рамкой и
 * скруглением, лежащий прямо на фоне страницы и не красящий себя).
 */
describe('карточка на фоне страницы несёт свой фон', () => {
  const cards = [
    // Панель бланков: сквозь неё светил --bg, а полоса кнопок со своим --surface
    // читалась серой заплатой поверх панели.
    ['components/BlankSelector.vue', 'selector'],
    ['components/UserProfileHeader.vue', 'account-header'],
    ['views/CarsView.vue', 'carsview__help'],
    ['views/EmployeeView.vue', 'employeesview__help'],
  ];

  it.each(cards)('%s .%s', (file, cls) => {
    const src = fs.readFileSync(path.join(SRC, file), 'utf8');
    expect(rootBackgrounds(src, cls).join(' ')).toMatch(/var\(--surface\)/);
  });
});

/**
 * Подложка под карточками (#1576).
 *
 * У --surface-2 направление в палитрах РАЗНОЕ: в светлой он темнее карточки,
 * в тёмной светлее. Поэтому контейнер-колонка, покрашенная в --surface-2,
 * выглядит правильно в одной теме и переворачивается в другой: подложка
 * выпирает светлым, а карточки внутри неё темнее внешней панели и сливаются
 * с ней. Так и разъехалась деталь заявки. Для подложек есть --surface-sunken
 * с одинаковым направлением в обеих темах - эти проверки держат и сам токен,
 * и переход детали на него.
 */
describe('подложка под карточками темнее карточки', () => {
  const lum = (hex) => {
    const h = hex.replace('#', '');
    const full = h.length === 3 ? h.split('').map((c) => c + c).join('') : h;
    const ch = [0, 2, 4].map((i) => parseInt(full.slice(i, i + 2), 16) / 255);
    const f = (s) => (s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4);
    return 0.2126 * f(ch[0]) + 0.7152 * f(ch[1]) + 0.0722 * f(ch[2]);
  };

  it.each(THEMES)('%s: --surface-sunken объявлен и темнее --surface', (theme) => {
    const p = palette(theme);
    expect(p['--surface-sunken'], `${theme}: --surface-sunken`).toBeTruthy();
    expect(lum(p['--surface-sunken'])).toBeLessThan(lum(p['--surface']));
  });

  const sunken = [
    ['components/ApplicationDetail/ApplicationDetail.vue', 'detail-header'],
    ['components/ApplicationDetail/ApplicationDetail.vue', 'detail-left-column'],
    ['components/ApplicationDetail/ApplicationDetail.vue', 'detail-right-column'],
    // Центральная колонка держала секции без подложки: они ложились прямо на
    // модалку и совпадали с ней цветом, пока боковые выглядели приподнятыми (#1581).
    ['components/ApplicationDetail/ApplicationDetail.vue', 'detail-main-column'],
  ];

  it.each(sunken)('%s .%s берёт --surface-sunken', (file, cls) => {
    const src = fs.readFileSync(path.join(SRC, file), 'utf8');
    expect(rootBackgrounds(src, cls).join(' ')).toMatch(/var\(--surface-sunken\)/);
  });
});
