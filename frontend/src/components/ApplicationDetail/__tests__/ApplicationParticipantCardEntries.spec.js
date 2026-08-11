import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

/**
 * Два входа в карточку участника (#1952): строка в окне «Получатели» и согласующий
 * в блоке «Ответственные за согласование». Компонент карточки один, держит её
 * родитель - карточка заявки.
 *
 * Список участников тянется лениво и переживает повторные открытия: из блока
 * согласования контактов нет вовсе, но платить запросом за каждый клик не за что.
 */

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({
  markAsRead: vi.fn().mockResolvedValue(undefined),
  getApplicationSupplements: vi.fn().mockResolvedValue([]),
  getApplicationParticipants: vi.fn(),
}));

import ApplicationDetail from '../ApplicationDetail.vue';
import ApplicationParticipantsModal from '../ApplicationParticipantsModal.vue';
import ApplicationParticipantCard from '../ApplicationParticipantCard.vue';
import ApplicationConfirmation from '../ApplicationConfirmation.vue';
import { getApplicationParticipants } from '@/api/applications';

const APPROVER = {
  user_id: 5,
  username: 'pt_approver',
  full_name: 'Согласуев Семён Семёнович',
  position: 'Инженер',
  organization_name: 'Организация',
  company_name: 'Компания',
  email: 'a@example.com',
  phone: '79100830055',
  roles: ['approver'],
  primary_role: 'approver',
  approval_status: 'approved',
  pd_hidden: false,
};

const READER = {
  user_id: 9,
  username: 'pt_reader',
  full_name: 'Читателев Роман Романович',
  position: null,
  organization_name: null,
  company_name: null,
  email: null,
  phone: null,
  roles: ['reader'],
  primary_role: 'reader',
  approval_status: null,
  pd_hidden: false,
};

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

/** Карточка в состоянии «открыта»: проп show у единственного экземпляра. */
function card(wrapper) {
  return wrapper.findComponent(ApplicationParticipantCard);
}

describe('Карточка участника - вход из окна «Получатели» (#1952)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    getApplicationParticipants.mockResolvedValue([APPROVER, READER]);
  });

  it('клик по строке открывает карточку тем, что окно уже загрузило - без своего запроса', async () => {
    const wrapper = mountDetail();
    await flushPromises();
    vi.clearAllMocks();

    expect(card(wrapper).props('show')).toBe(false);

    wrapper.findComponent(ApplicationParticipantsModal).vm.$emit('select', READER);
    await wrapper.vm.$nextTick();

    expect(card(wrapper).props('show')).toBe(true);
    expect(card(wrapper).props('participant')).toEqual(READER);
    expect(card(wrapper).props('loading')).toBe(false);
    expect(getApplicationParticipants).not.toHaveBeenCalled();
  });

  it('карточка закрывается, окно получателей остаётся открытым', async () => {
    const wrapper = mountDetail();
    await flushPromises();

    wrapper.vm.showParticipantsModal = true;
    wrapper.findComponent(ApplicationParticipantsModal).vm.$emit('select', READER);
    await wrapper.vm.$nextTick();

    card(wrapper).vm.$emit('close');
    await wrapper.vm.$nextTick();

    expect(card(wrapper).props('show')).toBe(false);
    expect(wrapper.findComponent(ApplicationParticipantsModal).props('show')).toBe(true);
  });

  it('строка окна - кликабельная и с клавиатуры', async () => {
    getApplicationParticipants.mockResolvedValue([APPROVER, READER]);
    const modal = mount(ApplicationParticipantsModal, {
      props: { show: true, applicationId: 7 },
      global: { stubs: { BaseModal: { template: '<div><slot /></div>' } } },
    });
    await flushPromises();

    const rows = modal.findAll('[data-testid="app-participants-row"]');
    const row = rows.find((r) => r.text().includes('Читателев'));
    expect(row.attributes('role')).toBe('button');
    expect(row.attributes('tabindex')).toBe('0');

    await row.trigger('click');
    await row.trigger('keydown.enter');
    await row.trigger('keydown.space');

    expect(modal.emitted('select')).toHaveLength(3);
    expect(modal.emitted('select')[0][0].user_id).toBe(READER.user_id);
    modal.unmount();
  });
});

describe('Карточка участника - вход из блока согласования (#1952)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    getApplicationParticipants.mockResolvedValue([APPROVER, READER]);
  });

  it('клик по согласующему открывает карточку с контактами из ответа про участников', async () => {
    const wrapper = mountDetail();
    await flushPromises();
    vi.clearAllMocks();

    wrapper.findComponent(ApplicationConfirmation).vm.$emit('select-user', { id: 5, username: 'pt_approver' });
    await wrapper.vm.$nextTick();

    // Карточка открыта сразу, чтобы клик не выглядел провалившимся.
    expect(card(wrapper).props('show')).toBe(true);
    expect(card(wrapper).props('loading')).toBe(true);

    await flushPromises();

    expect(getApplicationParticipants).toHaveBeenCalledTimes(1);
    expect(getApplicationParticipants).toHaveBeenCalledWith(7);
    expect(card(wrapper).props('participant')).toEqual(APPROVER);
    expect(card(wrapper).props('loading')).toBe(false);
    expect(card(wrapper).props('error')).toBe('');
  });

  it('повторное открытие карточки не стоит ни одного запроса', async () => {
    const wrapper = mountDetail();
    await flushPromises();
    vi.clearAllMocks();

    for (const id of [5, 9, 5]) {
      wrapper.findComponent(ApplicationConfirmation).vm.$emit('select-user', { id });
      await flushPromises();
      card(wrapper).vm.$emit('close');
      await wrapper.vm.$nextTick();
    }

    expect(getApplicationParticipants).toHaveBeenCalledTimes(1);
  });

  it('два клика подряд, пока список ещё летит, шлют один запрос и показывают последнего', async () => {
    const wrapper = mountDetail();
    await flushPromises();
    vi.clearAllMocks();

    let resolveList;
    getApplicationParticipants.mockImplementationOnce(
      () => new Promise((resolve) => { resolveList = resolve; })
    );

    wrapper.findComponent(ApplicationConfirmation).vm.$emit('select-user', { id: 5 });
    await wrapper.vm.$nextTick();
    wrapper.findComponent(ApplicationConfirmation).vm.$emit('select-user', { id: 9 });
    await wrapper.vm.$nextTick();

    resolveList([APPROVER, READER]);
    await flushPromises();

    expect(getApplicationParticipants).toHaveBeenCalledTimes(1);
    expect(card(wrapper).props('participant')).toEqual(READER);
  });

  it('деталь перечитали, пока список летел - ответ показан, но в память не осел', async () => {
    const wrapper = mountDetail();
    await flushPromises();
    vi.clearAllMocks();

    let resolveList;
    getApplicationParticipants.mockImplementationOnce(
      () => new Promise((resolve) => { resolveList = resolve; })
    );

    wrapper.findComponent(ApplicationConfirmation).vm.$emit('select-user', { id: 5 });
    await wrapper.vm.$nextTick();

    // Живой сигнал по заявке: голоса согласующих могли смениться.
    wrapper.vm.resetParticipantsCache();
    resolveList([APPROVER, READER]);
    await flushPromises();

    expect(card(wrapper).props('participant')).toEqual(APPROVER);

    // Следующий клик обязан спросить заново, а не показать то, что приехало до
    // обновления детали.
    getApplicationParticipants.mockResolvedValueOnce([{ ...APPROVER, approval_status: 'rejected' }]);
    wrapper.findComponent(ApplicationConfirmation).vm.$emit('select-user', { id: 5 });
    await flushPromises();

    expect(getApplicationParticipants).toHaveBeenCalledTimes(2);
    expect(card(wrapper).props('participant').approval_status).toBe('rejected');
  });

  it('человека нет в списке участников - говорим об этом, а не показываем пустую карточку', async () => {
    const wrapper = mountDetail();
    await flushPromises();

    wrapper.findComponent(ApplicationConfirmation).vm.$emit('select-user', { id: 404 });
    await flushPromises();

    expect(card(wrapper).props('participant')).toBe(null);
    expect(card(wrapper).props('error')).toContain('Не нашли');
  });

  it('отказ бэка виден в карточке текстом', async () => {
    const wrapper = mountDetail();
    await flushPromises();
    getApplicationParticipants.mockRejectedValueOnce(new Error('Нет доступа к заявке'));

    wrapper.findComponent(ApplicationConfirmation).vm.$emit('select-user', { id: 5 });
    await flushPromises();

    expect(card(wrapper).props('error')).toBe('Нет доступа к заявке');
    expect(card(wrapper).props('loading')).toBe(false);
  });

  it('строка согласующего доступна с клавиатуры и объявлена кнопкой', async () => {
    const confirmation = mount(ApplicationConfirmation, {
      props: {
        application: { id: 7, confirmation: 'Согласование', status: 'Непрочитано' },
        responsibleUsers: [
          { id: 5, username: 'pt_approver', last_name: 'Согласуев', first_name: 'Семён', approval_status: 'approved' },
        ],
      },
      global: { stubs: { LoaderSpinner: true } },
    });

    const row = confirmation.find('[data-testid="app-confirmation-user"]');
    expect(row.attributes('role')).toBe('button');
    expect(row.attributes('tabindex')).toBe('0');

    await row.trigger('click');
    await row.trigger('keydown.enter');
    await row.trigger('keydown.space');

    expect(confirmation.emitted('select-user')).toHaveLength(3);
    expect(confirmation.emitted('select-user')[0][0].id).toBe(5);
    confirmation.unmount();
  });
});
