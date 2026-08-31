import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// Замок видимости заметки бюро в карточке заявки. Заметку ведут принимающие, и
// заявителю с согласующими её не показывают - блок обязан отсутствовать в разметке,
// а не прятаться стилем: спрятанный блок остаётся в DOM и утечёт в любой выгрузке
// страницы. Настоящий барьер стоит на бэке (заметка не попадает в ответ детали
// непринимающему), этот замок стережёт вторую половину - что фронт её и не рисует.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({
  markAsRead: vi.fn().mockResolvedValue(undefined),
}));

import ApplicationDetail from '../ApplicationDetail.vue';
import ApplicationBureauNote from '../ApplicationBureauNote.vue';

const NOTE = { text: 'Ждём паспорт водителя', author_name: 'Иванов Иван', updated_at: '2026-08-25T10:00:00Z' };

async function mountDetail(data = {}) {
  const wrapper = shallowMount(ApplicationDetail, {
    props: {
      application: { id: 7, application_number: 'A-7', status: 'В обработке', bureau_note: NOTE },
      currentUserId: 1,
      mode: 'center',
    },
  });
  await wrapper.setData({ responsibleUsers: [], approvers: [], isApproverSelf: false, ...data });
  await wrapper.vm.$nextTick();
  return wrapper;
}

describe('ApplicationDetail - заметка бюро видна только принимающему', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('принимающий (ответ /application-approvers/me) блок видит', async () => {
    const wrapper = await mountDetail({ isApproverSelf: true });
    expect(wrapper.vm.isApprover).toBe(true);
    expect(wrapper.findComponent(ApplicationBureauNote).exists()).toBe(true);
  });

  it('принимающий из загруженного состава блок видит', async () => {
    const wrapper = await mountDetail({ approvers: [{ user_id: 1 }] });
    expect(wrapper.vm.isApprover).toBe(true);
    expect(wrapper.findComponent(ApplicationBureauNote).exists()).toBe(true);
  });

  it('не принимающий блока не видит, даже если заметка каким-то образом приехала', async () => {
    const wrapper = await mountDetail({ approvers: [{ user_id: 42 }] });
    expect(wrapper.vm.isApprover).toBe(false);
    expect(wrapper.findComponent(ApplicationBureauNote).exists()).toBe(false);
    // И текста заметки нет нигде в разметке - блок не просто скрыт классом.
    expect(wrapper.html()).not.toContain(NOTE.text);
  });

  it('заметка уходит в блок из данных заявки', async () => {
    const wrapper = await mountDetail({ isApproverSelf: true });
    expect(wrapper.findComponent(ApplicationBureauNote).props('note')).toEqual(NOTE);
  });

  it('сохранение заметки обновляет карточку без перечитывания детали', async () => {
    const wrapper = await mountDetail({ isApproverSelf: true });
    const saved = { text: 'Заявитель дозагрузит доверенность', author_name: 'Иванов Иван', updated_at: '2026-08-26T09:00:00Z' };

    wrapper.findComponent(ApplicationBureauNote).vm.$emit('update', saved);
    await wrapper.vm.$nextTick();

    expect(wrapper.findComponent(ApplicationBureauNote).props('note')).toEqual(saved);
  });

  it('снятая заметка приходит в блок как null', async () => {
    const wrapper = await mountDetail({ isApproverSelf: true });

    wrapper.findComponent(ApplicationBureauNote).vm.$emit('update', null);
    await wrapper.vm.$nextTick();

    expect(wrapper.findComponent(ApplicationBureauNote).props('note')).toBeNull();
  });
});
