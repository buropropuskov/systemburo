import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import DateRangeSection from '../DateRangeSection.vue';

// Меню "Быстрый выбор" телепортится в body и позиционируется от триггера.
// Замок на две вещи разом:
// 1) ширину задаёт содержимое (CSS width:max-content), а JS её НЕ фиксирует -
//    строка длинного месяца ("На сентябрь 01.09.2026 - 30.09.2026") в прежние
//    230px не влезала и вылезала за границу меню на 11px;
// 2) rect (device-px) приводится к layout-px делением на zoom - иначе на
//    мониторах >1440 inline-координаты домножаются на zoom второй раз.
const zoom = { value: 1 };
vi.mock('@/utils/viewportScale', () => ({
  getViewportZoom: () => zoom.value,
}));

function openMenu(w, rect) {
  w.vm.$refs.qdTrigger.getBoundingClientRect = () => rect;
  w.vm.toggleQuickMenu();
  return w.vm.qdMenuStyle;
}

describe('DateRangeSection - меню "Быстрый выбор"', () => {
  beforeEach(() => {
    zoom.value = 1;
    Object.defineProperty(window, 'innerWidth', { value: 1440, configurable: true });
  });

  it('ширина не фиксируется в JS: меню растёт по содержимому', () => {
    const w = mount(DateRangeSection);
    const style = openMenu(w, { right: 700, bottom: 300 });
    expect(style.width).toBeUndefined();
    expect(style.maxWidth).toBeDefined();
  });

  it('крепится правым краем к правому краю триггера', () => {
    const w = mount(DateRangeSection);
    const style = openMenu(w, { right: 700, bottom: 300 });
    expect(style.left).toBe('auto');
    expect(style.right).toBe('740px'); // 1440 - 700
    expect(style.top).toBe('306px'); // bottom(300) + 6
    // до левого края экрана: 1440 - 740 - 8
    expect(style.maxWidth).toBe('692px');
  });

  it('zoom=1.6: rect и innerWidth делятся на zoom (меню под триггером, не в углу)', () => {
    zoom.value = 1.6;
    // экран 2304 device-px = 1440 layout-px, триггер правым краем на 1000 layout
    Object.defineProperty(window, 'innerWidth', { value: 2304, configurable: true });
    const w = mount(DateRangeSection);
    const style = openMenu(w, { right: 1600, bottom: 480 });
    expect(style.right).toBe('440px'); // 1440 - 1000
    expect(style.top).toBe('306px'); // round(480/1.6) + 6
    expect(style.maxWidth).toBe('992px'); // 1440 - 440 - 8
  });

  it('триггер у самого края экрана: меню не уезжает за кромку', () => {
    const w = mount(DateRangeSection);
    const style = openMenu(w, { right: 1438, bottom: 300 });
    expect(style.right).toBe('8px'); // клэмп на отступ от края
  });
});
