import { describe, it, expect, afterEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { cssVariable, withAlpha, watchTheme } from '@/utils/chartColors';

// Холст рисуется цветами темы (#2125, S8): CSS-переменные canvas не понимает,
// значения читаются из стилей. До этого линия, сетка и подписи ленты запросов
// были зашиты литералами и в тёмной теме оставались светлыми.

const CHART = readFileSync(resolve(__dirname, '../../components/RealTimeChart.vue'), 'utf8');
const SCRIPT = CHART.slice(CHART.indexOf('<script>'), CHART.indexOf('</script>'));

afterEach(() => {
  document.documentElement.style.cssText = '';
});

describe('цвета темы для холста', () => {
  it('значение переменной снимается с документа элемента', () => {
    document.documentElement.style.setProperty('--accent', '#5561B8');
    expect(cssVariable(document.body, '--accent', '#4F5BDF')).toBe('#5561B8');
  });

  it('без объявленной переменной отдаёт запасной цвет, а не пустоту', () => {
    expect(cssVariable(document.body, '--accent', '#4F5BDF')).toBe('#4F5BDF');
    expect(cssVariable(null, '--accent', '#4F5BDF')).toBe('#4F5BDF');
  });

  it('наблюдатель темы цепляется к корню документа и снимается', () => {
    let hits = 0;
    const observer = watchTheme(document.body, () => { hits += 1; });
    expect(observer).toBeTruthy();
    observer.disconnect();
    expect(hits).toBe(0);
  });
});

describe('прозрачность цвета', () => {
  it('шестизначный и короткий hex разбираются в rgba', () => {
    expect(withAlpha('#4F5BDF', 0.19)).toBe('rgba(79, 91, 223, 0.19)');
    expect(withAlpha(' #fff ', 0.5)).toBe('rgba(255, 255, 255, 0.5)');
  });

  it('нехексовый цвет возвращается как есть, а не склеивается в мусор', () => {
    // Прежний код графика добавлял '30' к строке цвета: 'rgb(0,0,0)30' холст
    // не понимает вовсе, и заливка под кривой пропадала молча.
    expect(withAlpha('rgb(10, 20, 30)', 0.35)).toBe('rgb(10, 20, 30)');
  });
});

describe('замок на литералы в ленте запросов', () => {
  it('в отрисовке не осталось зашитых цветов', () => {
    // Запасные значения внутри cssVariable(...) законны - они нужны там, где
    // темы нет вовсе (юниты, холст вне документа). Всё остальное - хардкод.
    const drawing = SCRIPT.replace(/cssVariable\([^)]*\)/g, '');
    expect(drawing).not.toMatch(/#[0-9a-fA-F]{3,8}\b/);
    expect(drawing).not.toMatch(/rgba?\(\s*\d/);
  });
});
