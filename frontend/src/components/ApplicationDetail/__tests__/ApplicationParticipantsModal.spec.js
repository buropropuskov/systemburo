import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

import ApplicationParticipantsModal from '../ApplicationParticipantsModal.vue';
import { getApplicationParticipants } from '@/api/applications';

vi.mock('@/api/applications', () => ({ getApplicationParticipants: vi.fn() }));

const ROW = '[data-testid="app-participants-row"]';
const ROLE = '[data-testid="app-participants-role"]';
const VOTE = '[data-testid="app-participants-vote"]';

/**
 * Фикстура снята с формы `services.ApplicationParticipant`: одна запись на человека,
 * полный набор ролей в `roles`, старшая в `primary_role`, голос заполнен только у
 * согласующего, у скрытого по ПД пусты ФИО и контакты.
 */
const PARTICIPANTS = [
  {
    user_id: 1,
    username: 'pt_sender',
    full_name: 'Отправителев Олег Олегович',
    position: 'Логист',
    organization_name: 'Организация',
    company_name: 'Компания',
    email: 'o@example.com',
    phone: '+7 900 000 00 01',
    roles: ['sender', 'approver'],
    primary_role: 'sender',
    approval_status: 'pending',
    approval_comment: null,
    approval_datetime: null,
    pd_hidden: false,
  },
  {
    user_id: 2,
    username: 'pt_acceptor',
    full_name: 'Бюро пропусков',
    position: null,
    organization_name: null,
    company_name: null,
    email: null,
    phone: null,
    roles: ['acceptor'],
    primary_role: 'acceptor',
    approval_status: null,
    pd_hidden: false,
  },
  {
    user_id: 3,
    username: 'pt_approver',
    full_name: 'Согласуев Семён Семёнович',
    position: 'Инженер',
    organization_name: 'Организация',
    company_name: 'Компания',
    email: 'a@example.com',
    phone: '+7 900 000 00 03',
    roles: ['approver'],
    primary_role: 'approver',
    required_approval: true,
    approval_status: 'approved',
    approval_comment: 'Согласовано без замечаний',
    approval_datetime: '2026-05-12T09:30:00Z',
    pd_hidden: false,
  },
  {
    user_id: 4,
    username: 'i.ivanov',
    full_name: '',
    last_name: null,
    first_name: null,
    middle_name: null,
    position: 'Кладовщик',
    organization_name: 'Организация',
    company_name: null,
    email: null,
    phone: null,
    roles: ['reader'],
    primary_role: 'reader',
    approval_status: null,
    pd_hidden: true,
  },
];

const BASE_MODAL_STUB = { template: '<div><slot /></div>' };

async function mountModal(props = {}) {
  const wrapper = mount(ApplicationParticipantsModal, {
    props: { show: true, applicationId: 7, ...props },
    global: { stubs: { BaseModal: BASE_MODAL_STUB } },
  });
  await flushPromises();
  return wrapper;
}

/**
 * Строка участника по видимому тексту. Ассерты не привязаны к позиции: порядок
 * задаёт сервер, и тест не должен ломаться от смены сортировки в нём.
 * @param {import('@vue/test-utils').VueWrapper} wrapper
 * @param {string} text
 */
function rowWith(wrapper, text) {
  const row = wrapper.findAll(ROW).find((r) => r.text().includes(text));
  expect(row, `нет строки с "${text}"`).toBeTruthy();
  return row;
}

describe('ApplicationParticipantsModal (#1952)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getApplicationParticipants.mockResolvedValue(PARTICIPANTS);
  });

  it('список рисуется из ответа эндпоинта - по строке на человека', async () => {
    const wrapper = await mountModal();

    expect(getApplicationParticipants).toHaveBeenCalledWith(7);
    expect(wrapper.findAll(ROW)).toHaveLength(4);
    expect(wrapper.text()).toContain('Всего: 4');
  });

  it('роли подписаны по-русски', async () => {
    const wrapper = await mountModal();

    // Принимающему бейдж роли не рисуем: заявка уходит принимающим по умолчанию, и
    // подпись у каждого второго участника ничего не сообщала.
    const roles = wrapper.findAll(ROLE).map((b) => b.text());
    expect(new Set(roles)).toEqual(new Set(['Отправитель', 'Согласующий', 'Читатель']));
    expect(roles).not.toContain('Принимающий');
  });

  it('человек с несколькими ролями - одна строка со старшей ролью, остальные рядом текстом', async () => {
    const wrapper = await mountModal();

    const rows = wrapper.findAll(ROW).filter((r) => r.text().includes('Отправителев'));
    expect(rows).toHaveLength(1);
    expect(rows[0].find(ROLE).text()).toBe('Отправитель');
    expect(rows[0].find('[data-testid="app-participants-extra-roles"]').text()).toContain('согласующий');
  });

  it('голос согласующего подписан общим словарём заявки', async () => {
    const wrapper = await mountModal();

    expect(rowWith(wrapper, 'Согласуев').find(VOTE).text()).toBe('Согласовано');
    // Автор, который ещё и голосует, показывает своё ожидание - вместе со старшей ролью.
    expect(rowWith(wrapper, 'Отправителев').find(VOTE).text()).toBe('Ожидание');
    // Принимающий не голосует: у него бейджа решения нет вовсе.
    expect(rowWith(wrapper, 'Бюро пропусков').find(VOTE).exists()).toBe(false);
  });

  it('скрытый по ПД назван честно: не «пусто», а «нет согласия», и без логина', async () => {
    const wrapper = await mountModal();

    const hidden = rowWith(wrapper, 'Имя скрыто');
    expect(hidden.find('[data-testid="app-participants-hidden-note"]').text())
      .toContain('не дал согласия на обработку персональных данных');
    expect(hidden.text()).not.toContain('i.ivanov');
    // Должность бэкенд не прячет - и строка её показывает.
    expect(hidden.text()).toContain('Кладовщик');
  });

  it('контакты в списке не показываются - их место в карточке участника', async () => {
    const wrapper = await mountModal();

    expect(wrapper.text()).not.toContain('a@example.com');
    expect(wrapper.text()).not.toContain('+7 900 000 00 03');
  });

  it('отказ бэка виден текстом, а не пустым списком', async () => {
    getApplicationParticipants.mockRejectedValue(new Error('Нет доступа к заявке'));

    const wrapper = await mountModal();

    expect(wrapper.find('[data-testid="app-participants-error"]').text()).toBe('Нет доступа к заявке');
    expect(wrapper.findAll(ROW)).toHaveLength(0);
  });

  it('пустой список говорит об этом словами', async () => {
    getApplicationParticipants.mockResolvedValue([]);

    const wrapper = await mountModal();

    expect(wrapper.find('[data-testid="app-participants-empty"]').exists()).toBe(true);
  });

  it('закрытого окна не запрашиваем, при открытии - перезапрашиваем', async () => {
    const wrapper = await mountModal({ show: false });
    expect(getApplicationParticipants).not.toHaveBeenCalled();

    await wrapper.setProps({ show: true });
    await flushPromises();
    expect(getApplicationParticipants).toHaveBeenCalledTimes(1);

    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    await flushPromises();
    expect(getApplicationParticipants).toHaveBeenCalledTimes(2);
  });

  it('ответ прошлого открытия не затирает текущее', async () => {
    let resolveStale;
    getApplicationParticipants
      .mockImplementationOnce(() => new Promise((resolve) => { resolveStale = resolve; }))
      .mockResolvedValueOnce([PARTICIPANTS[2]]);

    const wrapper = await mountModal();
    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    await flushPromises();

    expect(wrapper.findAll(ROW)).toHaveLength(1);

    resolveStale(PARTICIPANTS);
    await flushPromises();

    expect(wrapper.findAll(ROW)).toHaveLength(1);
    expect(wrapper.text()).toContain('Согласуев');
  });

  it('необязательный согласующий назван согласующим, метка «Обязательно» только у обязательного', async () => {
    // Карточка заявки давно зовёт всю таблицу «ответственными за согласование» и метит
    // обязательных подписью. Пока список звал необязательного «Ответственным», один и
    // тот же человек назывался в двух местах по-разному (поймано руками на стенде).
    getApplicationParticipants.mockResolvedValue([
      { ...PARTICIPANTS[2], user_id: 9, full_name: 'Соболева Наталья', required_approval: false, approval_status: 'pending' },
      PARTICIPANTS[2],
    ]);
    const wrapper = await mountModal();

    const roles = wrapper.findAll(ROLE).map((b) => b.text());
    expect(roles).toEqual(['Согласующий', 'Согласующий']);

    const required = wrapper.findAll('[data-testid="app-participants-required"]');
    expect(required, 'подпись «Обязательно» ровно у одного').toHaveLength(1);

    const votes = wrapper.findAll(VOTE).map((b) => b.text());
    expect(votes, 'голос виден у обоих: необязательный тоже голосует').toHaveLength(2);
  });

  it('закрывается по Escape', async () => {
    const wrapper = mount(ApplicationParticipantsModal, {
      props: { show: true, applicationId: 7 },
    });
    await flushPromises();

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await flushPromises();

    expect(wrapper.emitted('close')).toBeTruthy();
    wrapper.unmount();
  });

  it('скругление 30px, как у остальных окон проекта, а не дефолтные 15px', async () => {
    const wrapper = mount(ApplicationParticipantsModal, {
      props: { show: true, applicationId: 7 },
      global: { stubs: { BaseModal: { props: ['radius'], template: '<div :data-radius="radius"><slot /></div>' } } },
    });
    await flushPromises();

    expect(wrapper.find('[data-radius]').attributes('data-radius')).toBe('30px');
  });

  it('окно лежит выше всей стопки заявки и ниже глобальных диалогов', async () => {
    const wrapper = mount(ApplicationParticipantsModal, {
      props: { show: true, applicationId: 7 },
      global: { stubs: { BaseModal: { props: ['zIndex'], template: '<div :data-z="zIndex"><slot /></div>' } } },
    });
    await flushPromises();

    const layer = Number(wrapper.find('[data-z]').attributes('data-z'));
    // деталь заявки 10002, карточки из неё 10003/10005, назначение 10006, дополнение 10010
    expect(layer).toBeGreaterThan(10010);
    // ConfirmDialog 22000 обязан оставаться над окном
    expect(layer).toBeLessThan(22000);
  });
});
