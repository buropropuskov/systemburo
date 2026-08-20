import { describe, it, expect, afterEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { readChartPalette, withAlpha } from '@/utils/chartPalette';

// Холст рисуется цветами темы (#2125, S8): CSS-переменные canvas не понимает,
// значения читаются из стилей. До этого линия, сетка и подписи были зашиты
// литералами и в тёмной теме оставались светлыми.

const CHART = readFileSync(resolve(__dirname, '../../components/RealTimeChart.vue'), 'utf8');
const SCRIPT = CHART.slice(CHART.indexOf('<script>'), CHART.indexOf('</script>'));

afterEach(() => {
  document.documentElement.style.cssText = '';
});

describe('палитра холста', () => {
  it('берёт значения темы с корня документа', () => {
    document.documentElement.style.setProperty('--accent', '#5561B8');
    document.documentElement.style.setProperty('--border', '#3a3f4a');

    const palette = readChartPalette();
    expect(palette.accent).toBe('#5561B8');
    expect(palette.grid).toBe('#3a3f4a');
  });

  it('без объявленных переменных отдаёт запасные цвета, а не пустоту', () => {
    const palette = readChartPalette();
    expect(palette.accent).toBe('#4F5BDF');
    expect(palette.label).toBeTruthy();
  });
});

describe('прозрачность цвета', () => {
  it('шестизначный hex разбирается в rgba', () => {
    expect(withAlpha('#4F5BDF', 0.19)).toBe('rgba(79, 91, 223, 0.19)');
    expect(withAlpha(' #fff ', 0.5)).toBe('rgba(255, 255, 255, 0.5)');
  });

  it('нехексовый цвет не склеивается в мусор', () => {
    // Прежний код добавлял '30' к строке: 'rgb(0,0,0)30' холст не понимает вовсе.
    expect(withAlpha('rgb(10, 20, 30)', 0.35)).toBe('color-mix(in srgb, rgb(10, 20, 30) 35%, transparent)');
  });
});

describe('замок на литералы в графике', () => {
  it('в отрисовке не осталось зашитых цветов', () => {
    expect(SCRIPT).not.toMatch(/#[0-9a-fA-F]{3,8}\b/);
    expect(SCRIPT).not.toMatch(/rgba?\(\s*\d/);
  });
});
