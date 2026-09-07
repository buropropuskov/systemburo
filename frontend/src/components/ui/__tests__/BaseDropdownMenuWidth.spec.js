import { mount } from '@vue/test-utils';
import { describe, it, expect, vi } from 'vitest';
import BaseDropdown from '../BaseDropdown.vue';

vi.mock('@/utils/viewportScale', () => ({ getViewportZoom: () => 1 }));

/**
 * menuMinWidth уширяет меню, когда триггер узкий, а пункты содержательные. Ширина
 * живёт в inline-стиле, который считает updateMenuPosition, а тот работает ТОЛЬКО в
 * режиме телепорта.
 *
 * Замок на эту связку: в кабинете `menu-min-width` выставили без `teleport`, и пункт
 * «Заявки организации» продолжал обрезаться - 146px текста в 107px поля, при том что
 * проп выглядел заданным (#2339).
 */
function mountDd(props) {
  const w = mount(BaseDropdown, {
    props: { options: [{ id: 1, name: 'Мои заявки' }, { id: 2, name: 'Заявки организации' }], ...props },
  });
  w.vm.$refs.dropdown.getBoundingClientRect = () => ({ top: 100, bottom: 130, left: 50, width: 147, height: 30 });
  return w;
}

describe('BaseDropdown — ширина меню', () => {
  it('menuMinWidth уширяет меню сверх узкого триггера', async () => {
    const w = mountDd({ teleport: true, menuMinWidth: 210 });
    await w.find('.base-dropdown__button').trigger('click');
    expect(parseFloat(w.vm.menuStyle.width)).toBeGreaterThanOrEqual(210);
    w.unmount();
  });

  it('без телепорта menuMinWidth не применяется - inline-стиля нет вовсе', async () => {
    // Не «работает хуже», а не работает совсем: меню остаётся шириной кнопки, и
    // длинный пункт режется многоточием. Значит проп без телепорта - ложное обещание.
    const w = mountDd({ teleport: false, menuMinWidth: 210 });
    await w.find('.base-dropdown__button').trigger('click');
    expect(w.find('.base-dropdown__menu').attributes('style')).toBeUndefined();
    w.unmount();
  });

  it('без menuMinWidth меню повторяет ширину триггера', async () => {
    const w = mountDd({ teleport: true });
    await w.find('.base-dropdown__button').trigger('click');
    expect(parseFloat(w.vm.menuStyle.width)).toBe(147);
    w.unmount();
  });
});
