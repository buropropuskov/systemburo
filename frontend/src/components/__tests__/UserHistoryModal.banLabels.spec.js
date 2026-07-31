import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...args) => apiRequest(...args),
}));
// exceljs тяжёлый и нужен только для экспорта - экспорт здесь не тестируем.
vi.mock('exceljs', () => ({ default: { Workbook: class {} } }));

import UserHistoryModal from '../UserHistoryModal.vue';

async function mountWith(history) {
  apiRequest.mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue(history) });
  const wrapper = mount(UserHistoryModal, {
    props: {
      user: { id: 11, username: 'bantarget' },
      currentUserName: 'Админ',
    },
    global: { stubs: { teleport: true } },
    attachTo: document.body,
  });
  await flushPromises();
  return wrapper;
}

function entry(over = {}) {
  return {
    id: 1,
    action_type: 'banned',
    details: { reason: 'Нарушение режима доступа' },
    actor_user_id: 3,
    actor_name: 'Админ А.А.',
    created_at: '2026-05-01T10:00:00Z',
    ...over,
  };
}

describe('UserHistoryModal — лейблы блокировки', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    document.body.innerHTML = '';
  });

  it('banned: "Заблокирован" + причина из details.reason, красный dot', async () => {
    const wrapper = await mountWith([entry()]);

    const item = wrapper.find('.history-item');
    expect(item.find('.action-text').text()).toBe('Заблокирован');
    expect(item.find('.action-comment').text()).toBe('Причина: «Нарушение режима доступа»');
    expect(item.find('.timeline-dot').classes()).toContain('dot-deactivate');
  });

  it('unbanned: срок блокировки + снятая причина, зелёный dot', async () => {
    // banned_at снимок (08:00) до момента разбана created_at (10:00) = 2 часа.
    const wrapper = await mountWith([
      entry({
        id: 2,
        action_type: 'unbanned',
        details: { reason: 'Нарушение режима доступа', banned_at: '2026-05-01T08:00:00Z' },
        created_at: '2026-05-01T10:00:00Z',
      }),
    ]);

    const item = wrapper.find('.history-item');
    expect(item.find('.action-text').text()).toBe('Разблокирован');
    expect(item.find('.action-comment').text()).toBe('Был в блокировке: 2 ч., причина: «Нарушение режима доступа»');
    expect(item.find('.timeline-dot').classes()).toContain('dot-activate');
  });

  it('unbanned: только срок, если причины блокировки не было', async () => {
    const wrapper = await mountWith([
      entry({
        id: 5,
        action_type: 'unbanned',
        details: { banned_at: '2026-04-29T07:00:00Z' },
        created_at: '2026-05-01T10:00:00Z',
      }),
    ]);

    // 2026-04-29 07:00 -> 2026-05-01 10:00 = 2 дня 3 часа.
    expect(wrapper.find('.action-comment').text()).toBe('Был в блокировке: 2 дн. 3 ч.');
  });

  it('unbanned без снимка блокировки: "Разблокирован" без комментария', async () => {
    const wrapper = await mountWith([
      entry({ id: 6, action_type: 'unbanned', details: null }),
    ]);

    const item = wrapper.find('.history-item');
    expect(item.find('.action-text').text()).toBe('Разблокирован');
    expect(item.find('.action-comment').exists()).toBe(false);
  });

  it('banned без причины: лейбл есть, комментарий не рендерится', async () => {
    const wrapper = await mountWith([
      entry({ id: 3, details: { reason: '' } }),
    ]);

    const item = wrapper.find('.history-item');
    expect(item.find('.action-text').text()).toBe('Заблокирован');
    expect(item.find('.action-comment').exists()).toBe(false);
  });

  it('formatBanDuration: минуты при сроке меньше часа, "меньше минуты" при нуле', async () => {
    const wrapper = await mountWith([entry()]);
    const vm = wrapper.vm;
    expect(vm.formatBanDuration('2026-05-01T09:52:00Z', '2026-05-01T10:00:00Z')).toBe('8 мин.');
    expect(vm.formatBanDuration('2026-05-01T10:00:00Z', '2026-05-01T10:00:30Z')).toBe('меньше минуты');
    expect(vm.formatBanDuration('2026-05-01T10:00:00Z', '2026-05-01T09:00:00Z')).toBe('');
  });
});

describe('UserHistoryModal — согласие на обработку данных', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
  });

  it('выдачу согласия подписывает по-русски и показывает редакцию', async () => {
    const wrapper = await mountWith([entry({
      action_type: 'consent_granted',
      details: { consent_type: 'pd_processing', version: 17 },
      actor_name: 'Проверкин П.П.',
    })]);

    const text = wrapper.text();
    expect(text).toContain('Дал согласие на обработку персональных данных');
    expect(text).toContain('Редакция 17');
    // Сырого значения из базы в интерфейсе быть не должно.
    expect(text).not.toContain('consent_granted');

    wrapper.unmount();
  });

  it('отзыв согласия тоже подписан', async () => {
    const wrapper = await mountWith([entry({
      action_type: 'consent_revoked',
      details: { consent_type: 'pd_processing' },
    })]);

    expect(wrapper.text()).toContain('Отозвал согласие на обработку персональных данных');

    wrapper.unmount();
  });
});
