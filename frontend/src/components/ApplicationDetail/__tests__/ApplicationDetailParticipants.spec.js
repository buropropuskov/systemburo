import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

/**
 * Кнопка «Получатели» в шапке карточки заявки (#1952). Своего гейта у неё нет:
 * список участников отдаётся тому, кому видна сама заявка, а она уже открыта -
 * поэтому кнопка проверяется и там, где «Переслать» не положена (режим кабинета,
 * пользователь без прав Центра).
 */

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({
  markAsRead: vi.fn().mockResolvedValue(undefined),
  getApplicationSupplements: vi.fn().mockResolvedValue([]),
  getApplicationParticipants: vi.fn().mockResolvedValue([]),
}));

import ApplicationDetail from '../ApplicationDetail.vue';
import ApplicationParticipantsModal from '../ApplicationParticipantsModal.vue';

const PARTICIPANTS_BTN = '[data-testid="app-detail-button-participants"]';
const FORWARD_BTN = '[data-testid="app-detail-button-forward"]';

function mountDetail(props = {}) {
  return shallowMount(ApplicationDetail, {
    props: {
      application: { id: 7, application_number: 'A-7', status: 'Непрочитано' },
      currentUserId: 1,
      mode: 'center',
      ...props,
    },
  });
}

describe('ApplicationDetail - кнопка «Получатели» (#1952)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
  });

  it('кнопка есть у того, кто заявку открыл, даже без права пересылки', async () => {
    const wrapper = mountDetail();
    await wrapper.vm.$nextTick();

    expect(wrapper.find(PARTICIPANTS_BTN).exists()).toBe(true);
    // Контроль: «Переслать» этому пользователю не положена - значит кнопка получателей
    // показана не заодно с ней, а собственным условием.
    expect(wrapper.find(FORWARD_BTN).exists()).toBe(false);
  });

  it('кнопка есть и в кабинете, где режим не «Центр»', async () => {
    const wrapper = mountDetail({ mode: 'my' });
    await wrapper.vm.$nextTick();

    expect(wrapper.find(PARTICIPANTS_BTN).exists()).toBe(true);
  });

  it('до клика окно закрыто, клик его открывает', async () => {
    const wrapper = mountDetail();
    await wrapper.vm.$nextTick();

    const modal = wrapper.findComponent(ApplicationParticipantsModal);
    expect(modal.props('show')).toBe(false);
    expect(modal.props('applicationId')).toBe(7);

    await wrapper.find(PARTICIPANTS_BTN).trigger('click');

    expect(wrapper.findComponent(ApplicationParticipantsModal).props('show')).toBe(true);
  });

  it('окно закрывает себя само - деталь заявки остаётся открытой', async () => {
    const wrapper = mountDetail();
    await wrapper.vm.$nextTick();
    await wrapper.find(PARTICIPANTS_BTN).trigger('click');

    wrapper.findComponent(ApplicationParticipantsModal).vm.$emit('close');
    await wrapper.vm.$nextTick();

    expect(wrapper.findComponent(ApplicationParticipantsModal).props('show')).toBe(false);
    expect(wrapper.emitted('close')).toBeFalsy();
  });
});
