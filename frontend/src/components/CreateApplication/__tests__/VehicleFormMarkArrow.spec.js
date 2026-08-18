import { describe, it, expect, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { mount, flushPromises } from '@vue/test-utils';
import VehicleForm from '../VehicleForm.vue';

/**
 * Стрелка поля "Марка Т/С" была растровым arrow.png, зазубренным на Retina -
 * заменили на inline SVG-шеврон (см. VehicleForm.vue). Следующий круг замечаний
 * владельца: стрелка та же, но текст и стрелка сходились вплотную (не было gap
 * между ними) и длинная марка обрезалась впритык к стрелке. Замок держит обе части:
 * рендер (нет растровой иконки в этом блоке) и исходники CSS (scoped-стили Vue SFC
 * в jsdom не применяются - getComputedStyle тут ничего не покажет, см.
 * stickyHeadersOpaque.spec.js).
 */

vi.mock('@/api/client', () => ({ apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: async () => [] })) }));
vi.mock('@/api/blacklist', () => ({ checkVehicleBlacklist: vi.fn().mockResolvedValue(null) }));
vi.mock('@/stores/auth', () => ({ useAuthStore: vi.fn(() => ({ token: 'test-token' })) }));
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: vi.fn(() => ({ notify: vi.fn(), enqueue: vi.fn() })) }));
vi.mock('@/api/marks', () => ({ listMarks: vi.fn().mockResolvedValue([]) }));

const FIELD_CFG = {
  number: { visible: true, required: false },
  mark: { visible: true, required: false },
  unloading_places: { visible: true, required: false },
  passage_tables: { visible: true, required: false },
};

const filePath = resolve(__dirname, '../VehicleForm.vue');
const source = readFileSync(filePath, 'utf8');

function cssRule(selector) {
  const re = new RegExp(`(?:^|\\n)\\s*${selector.replace(/[.[\]]/g, '\\$&')}\\s*\\{([^}]*)\\}`);
  const match = source.match(re);
  if (!match) throw new Error(`правило ${selector} не найдено в исходнике`);
  return match[1];
}

describe('VehicleForm - стрелка "Марка Т/С" не растровая, с отступом от текста', () => {
  it('в разметке дропдауна марки нет img/*.png - только inline svg', async () => {
    const w = mount(VehicleForm, { props: { fieldConfig: FIELD_CFG }, attachTo: document.body });
    await flushPromises();

    const markButton = w.get('.mark__dropdown-button');
    expect(markButton.find('img').exists()).toBe(false);
    expect(markButton.html()).not.toMatch(/\.png/);

    const arrow = markButton.get('svg.mark__button-arrow');
    expect(arrow.attributes('viewBox')).toBe('0 0 10 6');
    expect(arrow.find('path').exists()).toBe(true);
  });

  it('стрелка марки совпадает по размеру и толщине линии с эталонным BaseDropdown', () => {
    const baseDropdown = readFileSync(resolve(__dirname, '../../ui/BaseDropdown.vue'), 'utf8');
    const baseArrowRule = baseDropdown.match(/\.base-dropdown__arrow\s*\{([^}]*)\}/)[1];
    const markArrowRule = cssRule('.mark__button-arrow');

    for (const prop of ['width: 10px', 'height: 10px']) {
      expect(baseArrowRule).toContain(prop);
      expect(markArrowRule).toContain(prop);
    }

    const markPath = source.match(/mark__button-arrow[\s\S]*?<path[\s\S]*?stroke-width="([\d.]+)"/)[1];
    const basePath = baseDropdown.match(/base-dropdown__arrow[\s\S]*?<path[\s\S]*?stroke-width="([\d.]+)"/)[1];
    expect(markPath).toBe(basePath);
  });

  it('между текстом и стрелкой задан gap - иначе они сходятся вплотную', () => {
    const contentRule = cssRule('.mark__button-content');
    expect(contentRule).toMatch(/gap:\s*\d+px/);
  });

  it('правый внутренний отступ кнопки больше левого - у стрелки есть запас от текста', () => {
    const buttonRule = cssRule('.mark__dropdown-button');
    const padding = buttonRule.match(/padding:\s*([^;]+);/)[1].trim().split(/\s+/).map(v => parseFloat(v));
    // Сокращённая запись top right bottom left.
    const [, right, , left] = padding;
    expect(right).toBeGreaterThan(left);
  });

  it('шеврон повёрнут в сторону меню: оно раскрывается вправо', () => {
    // Шеврон нарисован остриём вниз, поэтому знак поворота решает, куда он смотрит.
    // Пришёл он сюда из растровой стрелки, у которой остриё было вправо, и знак
    // достался от неё - значок при этом стал показывать в сторону, противоположную
    // раскрытию (см. .mark__dropdown-menu: left: 100%).
    const base = cssRule('.mark__button-arrow');
    const open = cssRule('.mark__button-arrow--open');
    const menu = cssRule('.mark__dropdown-menu');

    expect(menu, 'меню раскрывается вбок - иначе поворот стрелки надо считать заново')
      .toMatch(/left:\s*100%/);
    expect(base, 'остриё вниз + rotate(90deg) смотрит влево, от меню')
      .toMatch(/transform:\s*rotate\(-90deg\)/);
    expect(open, 'раскрытое состояние отражает стрелку в противоположную сторону')
      .toMatch(/transform:\s*rotate\(90deg\)/);
  });
});
