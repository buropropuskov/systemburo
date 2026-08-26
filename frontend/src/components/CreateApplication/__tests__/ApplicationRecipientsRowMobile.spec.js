import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

// На телефоне строка получателей должна оставаться одной строкой: один чип,
// кнопка «Ещё N» и плюс. Раньше в строку выводилось до четырёх чипов и они
// переносились, а выпадающий список открывался фиксированной шириной 240px
// от кнопки и уезжал за правый край.

const zoom = { value: 1 };
vi.mock('@/utils/viewportScale', () => ({
  getViewportZoom: () => zoom.value,
}));
// Кандидат в ответе нужен, чтобы кнопка «+ получатель» отрисовалась: без единого
// кандидата она скрыта (срез fe-recipients).
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({
    ok: true,
    json: vi.fn().mockResolvedValue([
      { id: 9, username: 'cand9', last_name: 'Кандидатов', first_name: 'Кандидат', position: 'Руководитель' },
    ]),
  }),
}));

import ApplicationRecipientsRow from '../ApplicationRecipientsRow.vue';

function mockMatchMedia(matches) {
  window.matchMedia = vi.fn().mockImplementation((query) => ({
    matches,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

const APPROVERS = [
  { user_id: 1, name: 'Первый Получатель Иванович' },
  { user_id: 2, name: 'Второй Получатель Петрович' },
  { user_id: 3, name: 'Третий Получатель Сергеевич' },
];

async function mountRow() {
  const w = mount(ApplicationRecipientsRow, { props: { approvers: APPROVERS, readers: [] } });
  await flushPromises();
  return w;
}

describe('ApplicationRecipientsRow на телефоне', () => {
  let origMatchMedia;

  beforeEach(() => {
    zoom.value = 1;
    origMatchMedia = window.matchMedia;
    Object.defineProperty(window, 'innerWidth', { value: 390, configurable: true });
    Object.defineProperty(window, 'innerHeight', { value: 844, configurable: true });
  });

  afterEach(() => {
    window.matchMedia = origMatchMedia;
  });

  it('в строке один получатель, остальные под кнопкой «Ещё N»', async () => {
    mockMatchMedia(true);
    const w = await mountRow();

    expect(w.vm.visibleChips.length).toBe(1);
    expect(w.vm.overflowChips.length).toBe(2);
    expect(w.find('.recipients-extra__btn').text()).toContain('Ещё 2');
    expect(w.find('.recipients-add__btn').text()).toBe('+');
  });

  it('на десктопе прежний вид: четыре чипа и подписанная кнопка', async () => {
    mockMatchMedia(false);
    const w = await mountRow();

    expect(w.vm.maxVisible).toBe(4);
    expect(w.vm.visibleChips.length).toBe(3);
    expect(w.vm.overflowChips.length).toBe(0);
    expect(w.find('.recipients-add__btn').text()).toBe('+ получатель');
    // Позиционирование считает только мобильная ветка, десктоп остаётся на absolute.
    expect(w.vm.buildPopoverStyle({ getBoundingClientRect: () => ({ left: 0, top: 0, bottom: 30 }) })).toBe(null);
  });

  it('окно списка держится в пределах экрана, даже если кнопка у правого края', async () => {
    mockMatchMedia(true);
    const w = await mountRow();

    const style = w.vm.buildPopoverStyle({
      getBoundingClientRect: () => ({ left: 340, top: 100, bottom: 132 }),
    });
    const left = parseInt(style.left, 10);
    const width = parseInt(style.width, 10);
    expect(style.position).toBe('fixed');
    expect(left).toBeGreaterThanOrEqual(12);
    expect(left + width).toBeLessThanOrEqual(390 - 12);
  });

  it('снизу тесно - список раскрывается вверх и сбрасывает top', async () => {
    mockMatchMedia(true);
    const w = await mountRow();

    const style = w.vm.buildPopoverStyle({
      getBoundingClientRect: () => ({ left: 20, top: 760, bottom: 792 }),
    });
    expect(style.top).toBe('auto');
    expect(style.bottom).toBeDefined();
  });
});
