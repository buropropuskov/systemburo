/**
 * Дополнения на телефоне (#1097 W6): блок свёрнут и стоит не сверху.
 *
 * Претензия владельца - «на телефонах Дополнения скрывать в раскрывающиеся элементы
 * и убрать их с самого верха». До правки панель не имела своего `order` в мобильном
 * блоке ApplicationDetail, то есть шла с нулевым (дефолт) и вставала в общей ленте
 * ПЕРЕД сообщением заявки. Порядок задаётся CSS, в jsdom его не проверить - на него
 * стоит статический замок по исходнику.
 */
import fs from 'node:fs';
import path from 'node:path';
import { ref } from 'vue';
import { mount } from '@vue/test-utils';
import { describe, expect, it, beforeEach, vi } from 'vitest';

// jsdom не реализует matchMedia: без мока useNarrowScreen выходит по гарду и isNarrow
// навсегда false, то есть мобильный режим панели не поднять. Ref заводим снаружи -
// setup() зовёт композабл один раз, ширину можно менять и после mount.
const isNarrowRef = ref(true);
vi.mock('@/composables/useNarrowScreen', () => ({
  useNarrowScreen: () => ({ isNarrow: isNarrowRef }),
}));

import SupplementPanel from '../SupplementPanel.vue';

const ME = 7;

function round(overrides = {}) {
  return {
    id: 11,
    number: 2,
    status: 'pending',
    comment: 'Подвозим ещё две машины',
    created_by_name: 'Сидоров Пётр Иванович',
    created_at: '2026-08-05T09:30:00Z',
    counts: { vehicles: 2, employees: 1, items: 0 },
    approvals: [],
    ...overrides,
  };
}

function mountPanel(props = {}) {
  return mount(SupplementPanel, {
    props: { supplements: [round()], currentUserId: ME, ...props },
  });
}

describe('SupplementPanel - сворачивание на мобилке (#1097 W6)', () => {
  beforeEach(() => {
    isNarrowRef.value = true;
  });

  it('на узком экране панель свёрнута сразу после открытия заявки', () => {
    const w = mountPanel();
    expect(w.find('[data-testid="supplement-panel"]').classes()).toContain('is-collapsed');
    expect(w.find('[data-testid="supplement-panel-toggle"]').attributes('aria-expanded')).toBe('false');
  });

  it('заголовок-кнопка раскрывает и сворачивает обратно', async () => {
    const w = mountPanel();
    const toggle = w.find('[data-testid="supplement-panel-toggle"]');

    await toggle.trigger('click');
    expect(w.find('[data-testid="supplement-panel"]').classes()).not.toContain('is-collapsed');
    expect(toggle.attributes('aria-expanded')).toBe('true');

    await toggle.trigger('click');
    expect(w.find('[data-testid="supplement-panel"]').classes()).toContain('is-collapsed');
    expect(toggle.attributes('aria-expanded')).toBe('false');
  });

  it('свёрнутый заголовок говорит, сколько внутри раундов', () => {
    const w = mountPanel({ supplements: [round({ id: 11, number: 2 }), round({ id: 10, number: 1 })] });
    expect(w.find('.supplement-count').text()).toBe('2');
  });

  it('свёрнутая панель прячет раунды стилем, а не выкидывает их из DOM', () => {
    // Иначе шаг тура по [data-testid="supplement-panel"] и поиск по тексту детали
    // перестали бы видеть содержимое, а раскрытие нельзя было бы анимировать.
    const w = mountPanel();
    expect(w.find('[data-testid="supplement-round-11"]').exists()).toBe(true);
    expect(w.text()).toContain('Дополнение №2');
  });

  it('ошибка загрузки видна и в свёрнутом виде - она вне сворачиваемой части', () => {
    const w = mountPanel({ supplements: [], error: 'Не удалось загрузить дополнения заявки' });
    const error = w.find('[data-testid="supplement-panel-error"]');
    expect(error.exists()).toBe(true);
    expect(error.element.closest('.supplement-body')).toBeNull();
  });

  it('на десктопе панель раскрыта и заголовок не кнопка', () => {
    isNarrowRef.value = false;
    const w = mountPanel();
    expect(w.find('[data-testid="supplement-panel"]').classes()).not.toContain('is-collapsed');
    expect(w.find('[data-testid="supplement-panel-toggle"]').exists()).toBe(false);
    expect(w.find('button.supplement-title').exists()).toBe(false);
    expect(w.find('.supplement-count').exists()).toBe(false);
  });
});

describe('порядок секций детали на мобилке (#1097 W6)', () => {
  const SFC = fs.readFileSync(
    path.resolve(__dirname, '../ApplicationDetail.vue'),
    'utf8'
  );

  /** Значение order из мобильного блока по имени класса секции. */
  function order(cls) {
    const m = SFC.match(new RegExp(`\\.${cls}\\s*\\{\\s*order:\\s*(\\d+);`));
    if (!m) throw new Error(`у .${cls} нет order в мобильном блоке ApplicationDetail`);
    return Number(m[1]);
  }

  it('дополнения идут ниже сообщения, вложений и согласования', () => {
    const supplement = order('detail-order-supplement');
    expect(supplement).toBeGreaterThan(order('message-section'));
    expect(supplement).toBeGreaterThan(order('detail-order-picker'));
    expect(supplement).toBeGreaterThan(order('detail-order-selected-attachment'));
    expect(supplement).toBeGreaterThan(order('detail-order-confirmation'));
  });

  it('обёртка дополнений не сжимается в общей ленте', () => {
    // Без flex-shrink: 0 flex-column режет высокий раскрытый блок вместо скролла ленты.
    const shrink = SFC.match(/\.detail-order-supplement,\n\s*\.message-section,/);
    expect(shrink, 'detail-order-supplement нет в списке flex-shrink: 0').not.toBeNull();
  });
});
