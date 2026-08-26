import { mount } from '@vue/test-utils';
import {
  describe, it, expect, vi, beforeEach,
} from 'vitest';
import BaseDropdown from '../BaseDropdown.vue';

// Меню телепортится в body внутри зазумленного <html> (масштаб под 1440 на
// мониторах >1440). Регресс-замок: updateMenuPosition обязан приводить device-px
// rect и НЕзумленный innerHeight к layout-px делением на getViewportZoom() -
// иначе inline-координаты домножаются на zoom второй раз и меню улетает в угол.
const zoom = { value: 1 };
vi.mock('@/utils/viewportScale', () => ({
  getViewportZoom: () => zoom.value,
}));

function mountDd() {
  return mount(BaseDropdown, {
    props: { options: [{ id: 1, name: 'A' }], teleport: true },
  });
}

describe('BaseDropdown позиционирование меню под корневым zoom', () => {
  beforeEach(() => {
    zoom.value = 1;
    Object.defineProperty(window, 'innerHeight', { value: 2000, configurable: true });
  });

  it('zoom=1: координаты меню = сырой rect (деление на 1, поведение не меняется)', () => {
    const w = mountDd();
    w.vm.$refs.dropdown.getBoundingClientRect = () => ({
      left: 800, top: 600, bottom: 630, width: 200,
    });
    w.vm.updateMenuPosition();
    expect(w.vm.menuStyle.left).toBe('800px');
    expect(w.vm.menuStyle.width).toBe('200px');
    expect(w.vm.menuStyle.top).toBe('635px'); // bottom(630)+gap(5)
  });

  it('zoom=1.6: rect и innerHeight делятся на zoom (меню под триггером, не в углу)', () => {
    zoom.value = 1.6;
    const w = mountDd();
    w.vm.$refs.dropdown.getBoundingClientRect = () => ({
      left: 800, top: 600, bottom: 630, width: 200,
    });
    w.vm.updateMenuPosition();
    expect(w.vm.menuStyle.left).toBe('500px'); // 800/1.6
    expect(w.vm.menuStyle.width).toBe('125px'); // 200/1.6
    expect(w.vm.menuStyle.top).toBe('399px'); // round(630/1.6)+5 = round(393.75)+5
  });
});
