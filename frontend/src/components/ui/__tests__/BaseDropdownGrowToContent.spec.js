import { mount } from '@vue/test-utils';
import { describe, it, expect, vi } from 'vitest';
import BaseDropdown from '../BaseDropdown.vue';

vi.mock('@/utils/viewportScale', () => ({ getViewportZoom: () => 1 }));

/**
 * Телепортнутое меню расширяется до подписей пунктов (#2431 продолжение).
 *
 * Ширину там задаёт позиционер по кнопке, а кнопку фильтра зажимает колонка таблицы:
 * в разделе таблиц «Все компании» кнопка и меню выходили по 152px, и названия компаний
 * резались многоточием.
 *
 * jsdom раскладку не считает, поэтому ширины подписей подставляются вручную - проверяется
 * сама формула: на сколько расширить и не уехать ли за край.
 */
async function поставить(w, { нехватка, ширинаКнопки = 152, левыйКрай = 100 }) {
  w.vm.$refs.dropdown.getBoundingClientRect = () => ({
    top: 100, bottom: 130, left: левыйКрай, width: ширинаКнопки, height: 30,
  });
  // Меню появляется в разметке только открытым - иначе ссылки на него нет.
  await w.find('.base-dropdown__button').trigger('click');
  await w.vm.$nextTick();

  // Подписи: одной не хватает `нехватка` пикселей. jsdom ширины не считает, задаём сами.
  const подписи = [
    { scrollWidth: 100 + нехватка, clientWidth: 100 },
    { scrollWidth: 80, clientWidth: 80 },
  ];
  const узел = w.vm.$refs.menu;
  узел.querySelectorAll = () => подписи;
  узел.getBoundingClientRect = () => ({ width: ширинаКнопки });
  w.vm.menuStyle = { ...w.vm.menuStyle, width: `${ширинаКнопки}px`, left: `${левыйКрай}px` };
}

describe('BaseDropdown — телепортнутое меню растёт до подписей', () => {
  const смонтировать = () => mount(BaseDropdown, {
    props: { options: [{ id: 1, name: 'ООО «Очень длинное название компании»' }], teleport: true },
  });

  it('меню расширяется ровно на нехватку', async () => {
    const w = смонтировать();
    await поставить(w, { нехватка: 40 });
    w.vm.growMenuToContent(1600, 8, 100);
    expect(parseFloat(w.vm.menuStyle.width)).toBe(192);
    w.unmount();
  });

  it('подписи помещаются - ширина не меняется', async () => {
    const w = смонтировать();
    await поставить(w, { нехватка: 0 });
    w.vm.growMenuToContent(1600, 8, 100);
    expect(w.vm.menuStyle.width).toBe('152px');
    w.unmount();
  });

  it('за край экрана не уходит - меню сдвигается влево', async () => {
    // Кнопка у правого края: расширенное меню упёрлось бы в границу окна.
    const w = смонтировать();
    await поставить(w, { нехватка: 200, левыйКрай: 1500 });
    w.vm.growMenuToContent(1600, 8, 1500);
    const ширина = parseFloat(w.vm.menuStyle.width);
    const левый = parseFloat(w.vm.menuStyle.left);
    expect(левый + ширина, 'правый край меню за границей окна').toBeLessThanOrEqual(1600 - 8);
    w.unmount();
  });

  it('ширина не превышает экран целиком', async () => {
    const w = смонтировать();
    await поставить(w, { нехватка: 5000 });
    w.vm.growMenuToContent(1600, 8, 100);
    expect(parseFloat(w.vm.menuStyle.width)).toBeLessThanOrEqual(1600 - 16);
    w.unmount();
  });

  it('без телепорта позиционер ширину не трогает - там правило CSS', async () => {
    const w = mount(BaseDropdown, { props: { options: [{ id: 1, name: 'A' }], teleport: false } });
    await поставить(w, { нехватка: 40 });
    const было = w.vm.menuStyle.width;
    w.vm.growMenuToContent(1600, 8, 100);
    expect(w.vm.menuStyle.width, 'в потоке ширину задаёт min-width/max-content').toBe(было);
    w.unmount();
  });
});
