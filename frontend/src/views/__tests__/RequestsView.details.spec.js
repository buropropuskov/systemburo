import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { flushPromises } from '@vue/test-utils';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

// Деталь запроса раздела мониторинга (#2125): вместо колонки в 35% ширины -
// окно по эталону проекта. Замки стерегут контракт окна (крестик, затемнение,
// Escape) и то, что колонки под деталь больше нет, а таблица заняла её место.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  apiRequestRaw: vi.fn(),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify: vi.fn() }),
}));

import { apiRequestRaw } from '@/api/client';
import { logsPage, mountView, resetApiMocks, unmountAll } from './helpers/requestsView';

const LOG = {
  id: 7,
  url: '/api/applications?status=new',
  method: 'GET',
  response_status: 200,
  duration_us: 900,
  username: 'ivanov',
  user_id: 42,
  created_at: '2026-08-18T10:15:00Z',
};

const DETAILS_SRC = readFileSync(
  resolve(__dirname, '../../components/monitoring/LogDetails.vue'), 'utf8'
);
const JOURNAL_SRC = readFileSync(
  resolve(__dirname, '../../components/monitoring/JournalTab.vue'), 'utf8'
);

/** Открывает окно деталей кликом по первой строке журнала. */
async function openDetails() {
  apiRequestRaw.mockResolvedValue(logsPage([LOG]));
  const { wrapper } = await mountView();
  await flushPromises();
  await wrapper.get('.table-row').trigger('click');
  return wrapper;
}

/** Окно деталей в разметке (заглушка teleport оставляет его внутри экрана). */
function modal(wrapper) {
  return wrapper.find('.log-details-modal');
}

afterEach(() => {
  unmountAll();
});

beforeEach(() => {
  resetApiMocks();
});

describe('Мониторинг запросов, окно деталей', () => {
  it('клик по строке открывает окно с данными этого запроса', async () => {
    const wrapper = await openDetails();

    const window = modal(wrapper);
    expect(window.exists(), 'окно деталей открылось').toBe(true);
    expect(window.text()).toContain('Детали запроса');
    expect(window.text()).toContain('/api/applications?status=new');
    expect(window.text()).toContain('ivanov');
    expect(window.text()).toContain('(ID: 42)');
  });

  it('крестик, затемнение и Escape закрывают окно', async () => {
    const wrapper = await openDetails();

    await wrapper.get('[data-testid="modal-button-close"]').trigger('click');
    expect(modal(wrapper).exists(), 'крестик закрыл окно').toBe(false);

    await wrapper.get('.table-row').trigger('click');
    const overlay = wrapper.get('.base-modal-overlay');
    // Затемнение слушает пару mousedown+mouseup: одиночный click закрывал бы
    // окно и при выделении текста внутри с отпусканием снаружи.
    await overlay.trigger('mousedown');
    await overlay.trigger('mouseup');
    expect(modal(wrapper).exists(), 'клик по затемнению закрыл окно').toBe(false);

    await wrapper.get('.table-row').trigger('click');
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await flushPromises();
    expect(modal(wrapper).exists(), 'Escape закрыл окно').toBe(false);
  });

  it('окно берёт эталонное оформление, а не рисует своё', () => {
    expect(DETAILS_SRC, 'окно собрано на BaseModal').toContain('<BaseModal');
    expect(DETAILS_SRC, 'радиус окна эталонный').toContain('radius="30px"');
    // Радиус и слой BaseModal получает пропами: содержимое телепортируется в
    // body, и scoped-правило родителя до него не достаёт.
    expect(DETAILS_SRC).not.toContain('position: fixed');
  });

  it('пустых блоков заголовков и тел в окне не осталось', () => {
    expect(DETAILS_SRC).not.toContain('Заголовки');
    expect(DETAILS_SRC).not.toContain('Тело запроса');
    expect(DETAILS_SRC).not.toContain('Тело ответа');
    // Записей с телами в журнале нет: бэкенд их не пишет, а форматирование
    // JSON держалось только на этих блоках.
    expect(DETAILS_SRC).not.toContain('formatJson');
  });

  it('таблица журнала занимает всю ширину раздела', () => {
    expect(JOURNAL_SRC).not.toContain('logs-table-section');
    expect(JOURNAL_SRC).not.toContain('width: 65%');
  });
});
