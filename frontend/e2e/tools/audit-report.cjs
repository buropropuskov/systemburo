#!/usr/bin/env node
/**
 * Сборка отчёта аудита раскладки и защита от регрессий (#2473).
 *
 * Делает две вещи с `reports/viewport-audit.json`:
 *  1. печатает человекочитаемый разбор в `reports/viewport-audit.md`;
 *  2. сравнивает находки с эталонным срезом `baselines/viewport-audit.json` и падает,
 *     если на ширинах телефона и десктопа их стало больше.
 *
 * Второе - главное. Эпик правит планшет и админку, а требование владельца звучит
 * как «на телефоне и на десктопе не должно сломаться ничего»; проверять это глазами
 * после каждого из семнадцати срезов нереально, поэтому проверяем числами.
 *
 *   node e2e/tools/audit-report.cjs                 # отчёт + сравнение с эталоном
 *   node e2e/tools/audit-report.cjs --save-baseline # зафиксировать текущее как эталон
 */

const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const REPORT = path.join(ROOT, 'reports', 'viewport-audit.json');
const BASELINE = path.join(ROOT, 'baselines', 'viewport-audit.json');
const MD = path.join(ROOT, 'reports', 'viewport-audit.md');

// Ширины, на которых регрессия недопустима: телефон владельца и его десктоп.
// Планшетные ширины в гейт не входят - там находки как раз и правятся по ходу эпика.
const GUARDED_WIDTHS = [390, 1440, 1920];

const KINDS = [
  ['docOverflow', 'страница шире экрана'],
  ['overflow', 'узлы за правым краем'],
  ['oversized', 'строка шире контейнера'],
  ['overlaps', 'наложение внутри карточки'],
  ['clipped', 'текст обрезан без многоточия'],
  ['small', 'тач-таргет мельче нормы'],
  ['error', 'экран не снялся'],
];

function countOf(entry, kind) {
  const value = entry[kind];
  if (!value) return 0;
  return Array.isArray(value) ? value.length : 1;
}

function loadReport() {
  if (!fs.existsSync(REPORT)) {
    console.error(`нет отчёта ${REPORT} - сначала прогнать viewport-audit.spec.cjs`);
    process.exit(2);
  }
  return JSON.parse(fs.readFileSync(REPORT, 'utf8'));
}

/** Плоская карта «экран|ширина|вид находки» -> сколько штук. */
function fingerprint(report) {
  const map = {};
  for (const entry of report.results) {
    for (const [kind] of KINDS) {
      const n = countOf(entry, kind);
      if (n) map[`${entry.screen}|${entry.width}|${kind}`] = n;
    }
  }
  return map;
}

function renderMarkdown(report) {
  const lines = [];
  lines.push('# Аудит раскладки');
  lines.push('');
  lines.push(`Снято: ${report.createdAt}, стенд ${report.baseURL}`);
  lines.push(`Ширины: ${report.widths.join(', ')}`);
  lines.push('');

  const byScreen = new Map();
  for (const entry of report.results) {
    if (!byScreen.has(entry.screen)) byScreen.set(entry.screen, []);
    byScreen.get(entry.screen).push(entry);
  }

  lines.push('## Сводка');
  lines.push('');
  lines.push(`| Экран | ${report.widths.join(' | ')} |`);
  lines.push(`|---|${report.widths.map(() => '---').join('|')}|`);
  for (const [, entries] of byScreen) {
    const cells = report.widths.map((w) => {
      const entry = entries.find((e) => e.width === w);
      if (!entry) return '-';
      const total = KINDS.reduce((sum, [kind]) => sum + countOf(entry, kind), 0);
      return total === 0 ? 'чисто' : String(total);
    });
    lines.push(`| ${entries[0].name} | ${cells.join(' | ')} |`);
  }
  lines.push('');

  lines.push('## Находки');
  lines.push('');
  for (const [, entries] of byScreen) {
    const dirty = entries.filter((e) => KINDS.some(([kind]) => countOf(e, kind)));
    if (!dirty.length) continue;
    lines.push(`### ${entries[0].name} (${entries[0].area})`);
    for (const entry of dirty) {
      const parts = [];
      for (const [kind, label] of KINDS) {
        const n = countOf(entry, kind);
        if (!n) continue;
        const sample = Array.isArray(entry[kind])
          ? entry[kind].slice(0, 3).map((f) => JSON.stringify(f)).join(' ')
          : JSON.stringify(entry[kind]);
        parts.push(`  - ${label} (${n}): ${sample}`);
      }
      lines.push(`- **${entry.width}px**`);
      lines.push(...parts);
    }
    lines.push('');
  }
  return lines.join('\n');
}

function main() {
  const report = loadReport();
  const current = fingerprint(report);

  fs.mkdirSync(path.dirname(MD), { recursive: true });
  fs.writeFileSync(MD, renderMarkdown(report), 'utf8');
  console.log(`отчёт: ${MD}`);

  if (process.argv.includes('--save-baseline')) {
    fs.mkdirSync(path.dirname(BASELINE), { recursive: true });
    fs.writeFileSync(BASELINE, JSON.stringify(current, null, 2), 'utf8');
    console.log(`эталон записан: ${BASELINE} (${Object.keys(current).length} записей)`);
    return;
  }

  if (!fs.existsSync(BASELINE)) {
    console.log('эталона нет - сравнивать не с чем, прогнать с --save-baseline');
    return;
  }

  const base = JSON.parse(fs.readFileSync(BASELINE, 'utf8'));
  const regressions = [];
  for (const [key, count] of Object.entries(current)) {
    const [screen, width, kind] = key.split('|');
    if (!GUARDED_WIDTHS.includes(Number(width))) continue;
    const before = base[key] || 0;
    if (count > before) regressions.push(`${screen} @${width}: ${kind} ${before} -> ${count}`);
  }

  if (regressions.length) {
    console.error('РЕГРЕССИЯ на защищённых ширинах:');
    for (const line of regressions) console.error(`  ${line}`);
    process.exit(1);
  }
  console.log(`регрессий нет (защищённые ширины: ${GUARDED_WIDTHS.join(', ')})`);
}

main();
