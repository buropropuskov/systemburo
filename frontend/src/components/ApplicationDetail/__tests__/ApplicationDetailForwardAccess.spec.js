import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// #1948: переслать заявку вправе любой, у кого есть к ней доступ - супер-админ,
// принимающий, отправитель, ответственный, согласующий и читатель. Читатель при этом
// передаёт заявку ТОЛЬКО на просмотр: назначение роли сервер отбивает 403.
// Получатели приезжают из двух источников: носителю page.admin.users - полный
// /users/all, остальным - неадминские кандидаты (на /users/all у них 403).

import { usePermissionsStore } from '@/stores/permissions';
import { useAuthStore } from '@/stores/auth';

const okJson = (data) => ({ ok: true, json: async () => data });

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/api/applications', () => ({
  markAsRead: vi.fn().mockResolvedValue(undefined),
  getApplicationSupplements: vi.fn().mockResolvedValue([]),
}));

import { apiRequest } from '@/api/client';
import ApplicationDetail from '../ApplicationDetail.vue';
import ForwardModal from '../ForwardModal.vue';

const FORWARD = '[data-testid="app-detail-button-forward"]';

// Ответ эндпоинта кандидатов - поля из models.RecipientCandidate. Организация названа
// как в /users/all: оба ответа приезжают в один и тот же проп allUsers.
const CANDIDATES = [
  {
    id: 21,
    username: 'petrov',
    last_name: 'Петров',
    first_name: 'Пётр',
    middle_name: 'Петрович',
    position: 'Кладовщик',
    organization: 'ООО Ромашка',
    company: 'Ромашка-Сервис',
    pd_hidden: false,
  },
  {
    id: 22,
    username: 'hidden',
    last_name: null,
    first_name: null,
    middle_name: null,
    position: null,
    // Согласие на обработку ПД закрывает ФИО, но не организацию работодателя.
    organization: 'ООО Ромашка',
    company: 'Ромашка-Сервис',
    pd_hidden: true,
  },
];

// Ответ /users/all шире по составу людей: администратору доступны и чужие организации.
const ALL_USERS = [
  ...CANDIDATES,
  {
    id: 33,
    username: 'sidorov',
    last_name: 'Сидоров',
    first_name: 'Сидор',
    middle_name: null,
    position: 'Логист',
    organization: 'Чужая организация',
  },
];

/** JWT-подобный токен: стор читает is_super_admin из payload. */
function tokenWith(payload) {
  const body = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600, ...payload }));
  return `header.${body}.signature`;
}

/** Право пересылки выдано - тесты проверяют именно гейт доступа к заявке. */
function grant(...keys) {
  usePermissionsStore().effective = Object.fromEntries(
    ['action.forward.application', ...keys].map(k => [k, { value: 'allow', source: 'role' }])
  );
}

async function mountDetail({ props = {}, data = {}, superAdmin = false, allow = [] } = {}) {
  grant(...allow);
  if (superAdmin) useAuthStore().token = tokenWith({ is_super_admin: true });

  const wrapper = shallowMount(ApplicationDetail, {
    props: {
      application: { id: 7, application_number: 'A-7', status: 'Непрочитано', sender_user_id: 99 },
      currentUserId: 1,
      mode: 'center',
      ...props,
    },
  });
  // Роли ставим после загрузки детали: она сама пишет viewers/responsibleUsers из
  // ответа, и без ожидания её ответ затёр бы данные теста.
  await flushPromises();
  await wrapper.setData({ responsibleUsers: [], approvers: [], viewers: [], ...data });
  await wrapper.vm.$nextTick();
  return wrapper;
}

describe('ApplicationDetail - кнопка "Переслать" по доступу к заявке (#1948)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockResolvedValue(okJson([]));
  });

  it('отправитель видит кнопку', async () => {
    const wrapper = await mountDetail({
      props: { application: { id: 7, application_number: 'A-7', status: 'Непрочитано', sender_user_id: 1 } },
    });
    expect(wrapper.find(FORWARD).exists()).toBe(true);
  });

  it('ответственный видит кнопку', async () => {
    const wrapper = await mountDetail({ data: { responsibleUsers: [{ id: 1, approval_status: 'pending' }] } });
    expect(wrapper.find(FORWARD).exists()).toBe(true);
  });

  it('принимающий видит кнопку (ответ /application-approvers/me, без админского состава)', async () => {
    const wrapper = await mountDetail({ data: { isApproverSelf: true } });
    expect(wrapper.vm.approvers).toEqual([]);
    expect(wrapper.find(FORWARD).exists()).toBe(true);
  });

  it('читатель видит кнопку', async () => {
    const wrapper = await mountDetail({ data: { viewers: [{ user_id: 1 }] } });
    expect(wrapper.find(FORWARD).exists()).toBe(true);
  });

  it('супер-админ видит кнопку без роли на заявке', async () => {
    const wrapper = await mountDetail({ superAdmin: true });
    expect(wrapper.vm.hasApplicationAccess).toBe(true);
    expect(wrapper.find(FORWARD).exists()).toBe(true);
  });

  it('у отозванной заявки кнопки нет даже у ответственного', async () => {
    const wrapper = await mountDetail({
      props: { application: { id: 7, application_number: 'A-7', status: 'Отозвана', sender_user_id: 99 } },
      data: { responsibleUsers: [{ id: 1 }] },
    });
    expect(wrapper.find(FORWARD).exists()).toBe(false);
  });

  it('пользователь без доступа к заявке кнопки не видит', async () => {
    const wrapper = await mountDetail({ data: { responsibleUsers: [{ id: 2 }], viewers: [{ user_id: 3 }] } });
    expect(wrapper.vm.hasApplicationAccess).toBe(false);
    expect(wrapper.find(FORWARD).exists()).toBe(false);
  });
});

describe('ApplicationDetail - выбор роли получателя закрыт читателю (#1948)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockResolvedValue(okJson([]));
  });

  const readerOnlyProp = (wrapper) => wrapper.findComponent(ForwardModal).props('readerOnly');

  it('читателю пересылка только на просмотр', async () => {
    const wrapper = await mountDetail({ data: { viewers: [{ user_id: 1 }] } });
    expect(wrapper.vm.isForwardReaderOnly).toBe(true);
    expect(readerOnlyProp(wrapper)).toBe(true);
  });

  it('отправителю выбор роли доступен', async () => {
    const wrapper = await mountDetail({
      props: { application: { id: 7, application_number: 'A-7', status: 'Непрочитано', sender_user_id: 1 } },
      data: { viewers: [{ user_id: 1 }] },
    });
    expect(wrapper.vm.isForwardReaderOnly).toBe(false);
    expect(readerOnlyProp(wrapper)).toBe(false);
  });

  it('ответственному выбор роли доступен', async () => {
    const wrapper = await mountDetail({ data: { responsibleUsers: [{ id: 1 }] } });
    expect(wrapper.vm.isForwardReaderOnly).toBe(false);
  });

  it('принимающему выбор роли доступен', async () => {
    const wrapper = await mountDetail({ data: { isApproverSelf: true, viewers: [{ user_id: 1 }] } });
    expect(wrapper.vm.isForwardReaderOnly).toBe(false);
  });

  it('супер-админу выбор роли доступен', async () => {
    const wrapper = await mountDetail({ superAdmin: true });
    expect(wrapper.vm.isForwardReaderOnly).toBe(false);
  });
});

describe('ApplicationDetail - источник получателей (#1948)', () => {
  const callFor = (path) => apiRequest.mock.calls.find(([p]) => p === path);
  const paths = () => apiRequest.mock.calls.map(([path]) => path);

  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
    apiRequest.mockImplementation((path) => {
      if (path === '/users/recipient-candidates') return Promise.resolve(okJson(CANDIDATES));
      if (path === '/users/all') return Promise.resolve(okJson(ALL_USERS));
      return Promise.resolve(okJson([]));
    });
  });

  it('без права на список пользователей - неадминские кандидаты, уходят в окно пересылки', async () => {
    const wrapper = await mountDetail({ data: { responsibleUsers: [{ id: 1 }] } });

    expect(paths()).toContain('/users/recipient-candidates');
    expect(paths()).not.toContain('/users/all');
    expect(wrapper.vm.allUsers).toEqual(CANDIDATES);
    expect(wrapper.findComponent(ForwardModal).props('allUsers')).toEqual(CANDIDATES);
  });

  it('с page.admin.users - полный список пользователей, отказ по-прежнему молчит', async () => {
    const wrapper = await mountDetail({
      allow: ['page.admin.users'],
      data: { responsibleUsers: [{ id: 1 }] },
    });

    expect(paths()).toContain('/users/all');
    expect(paths()).not.toContain('/users/recipient-candidates');
    expect(callFor('/users/all')[1]).toMatchObject({ silent403: true });
    expect(wrapper.vm.allUsers).toEqual(ALL_USERS);
  });

  it('в личном кабинете получателей не запрашиваем - окна пересылки там нет', async () => {
    await mountDetail({
      props: { mode: 'user' },
      allow: ['page.admin.users'],
      data: { responsibleUsers: [{ id: 1 }] },
    });

    expect(paths()).not.toContain('/users/recipient-candidates');
    expect(paths()).not.toContain('/users/all');
  });
});

describe('ForwardModal - получатели и роль (#1948)', () => {
  async function mountOpened(props = {}) {
    const wrapper = mount(ForwardModal, {
      props: { show: false, allUsers: CANDIDATES, attachments: [], ...props },
      global: { stubs: { teleport: true } },
    });
    await wrapper.setProps({ show: true });
    return wrapper;
  }

  it('рисует получателей из ответа эндпоинта кандидатов', async () => {
    const wrapper = await mountOpened();
    await wrapper.find('[data-testid="forward-modal-search"]').trigger('focus');

    const options = wrapper.findAll('[data-testid="forward-modal-user-option"]');
    expect(options).toHaveLength(2);
    // Порядок опций задаёт сравнение строк, а оно зависит от локали окружения -
    // проверяем состав, а не позиции: с привязкой к индексу тест зеленел локально и
    // краснел в CI, где скрытый работник вставал первым.
    const texts = options.map((o) => o.text());
    const named = texts.find((t) => t.includes('Петров Пётр Петрович'));
    expect(named, `в списке нет получателя по ФИО: ${texts.join(' | ')}`).toBeTruthy();
    expect(named).toContain('Кладовщик');
    // Организация - вторая строка под именем; кандидаты её отдают наравне с /users/all.
    expect(named).toContain('ООО Ромашка');
    // ФИО скрытого работника бэк уже заменил - фронт показывает то, что пришло.
    expect(texts.some((t) => t.includes('hidden'))).toBe(true);
  });

  it('поиск находит получателя по названию организации', async () => {
    const wrapper = await mountOpened();
    await wrapper.find('[data-testid="forward-modal-search"]').setValue('Ромашка');

    const texts = wrapper.findAll('[data-testid="forward-modal-user-option"]').map((o) => o.text());
    expect(texts.some((t) => t.includes('Петров Пётр Петрович'))).toBe(true);
  });

  it('поиск по чужой организации получателя не находит', async () => {
    const wrapper = await mountOpened();
    await wrapper.find('[data-testid="forward-modal-search"]').setValue('Одуванчик');

    expect(wrapper.findAll('[data-testid="forward-modal-user-option"]')).toHaveLength(0);
  });

  it('читателю тумблеры роли не показываются, получатель уходит на просмотр', async () => {
    const wrapper = await mountOpened({ readerOnly: true });
    wrapper.vm.addUser(CANDIDATES[0]);
    await wrapper.vm.$nextTick();

    expect(wrapper.find('[data-testid="forward-modal-reader-note"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="forward-modal-user-settings"]').exists()).toBe(false);

    await wrapper.find('[data-testid="forward-modal-button-send"]').trigger('click');
    expect(wrapper.emitted('send')[0][0].users).toEqual([
      { user_id: 21, required_approval: false, can_view: true },
    ]);
  });

  it('остальным тумблеры роли доступны, согласование уходит на сервер', async () => {
    const wrapper = await mountOpened();
    wrapper.vm.addUser(CANDIDATES[0]);
    await wrapper.vm.$nextTick();

    expect(wrapper.find('[data-testid="forward-modal-reader-note"]').exists()).toBe(false);
    const toggles = wrapper.findAll('[data-testid="forward-modal-user-settings"] input[type="checkbox"]');
    expect(toggles).toHaveLength(2);

    await toggles[0].setValue(true);
    await toggles[1].setValue(true);
    await wrapper.find('[data-testid="forward-modal-button-send"]').trigger('click');

    expect(wrapper.emitted('send')[0][0].users).toEqual([
      { user_id: 21, required_approval: true, can_view: false },
    ]);
  });
});
