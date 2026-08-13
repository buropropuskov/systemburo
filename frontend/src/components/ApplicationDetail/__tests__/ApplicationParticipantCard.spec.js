import { describe, it, expect } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

import ApplicationParticipantCard from '../ApplicationParticipantCard.vue';

/**
 * Карточка участника (#1952). Своих запросов не делает - получает уже загруженную
 * запись в форме `services.ApplicationParticipant`, поэтому проверяется только показ:
 * контакты кликабельны, пустые честно прочерком, скрытые по ПД названы словами.
 */

const FULL = {
  user_id: 3,
  username: 'pt_approver',
  full_name: 'Согласуев Семён Семёнович',
  position: 'Инженер',
  organization_name: 'Организация',
  company_name: 'Компания',
  email: 'a@example.com',
  phone: '79100830055',
  roles: ['approver', 'reader'],
  primary_role: 'approver',
  approval_status: 'approved',
  approval_comment: 'Согласовано без замечаний',
  approval_datetime: '2026-05-12T09:30:00+03:00',
  pd_hidden: false,
};

const EMPTY_CONTACTS = {
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
};

const HIDDEN = {
  user_id: 4,
  username: 'i.ivanov',
  full_name: '',
  position: 'Кладовщик',
  organization_name: 'Организация',
  company_name: null,
  email: null,
  phone: null,
  roles: ['reader'],
  primary_role: 'reader',
  approval_status: null,
  pd_hidden: true,
};

const BASE_MODAL_STUB = { template: '<div><slot /></div>' };

function mountCard(props = {}, { stubModal = true } = {}) {
  return mount(ApplicationParticipantCard, {
    props: { show: true, participant: FULL, ...props },
    global: stubModal ? { stubs: { BaseModal: BASE_MODAL_STUB } } : {},
  });
}

describe('ApplicationParticipantCard (#1952)', () => {
  it('показывает ФИО, роли и решение согласующего', () => {
    const wrapper = mountCard();

    expect(wrapper.find('[data-testid="app-participant-card-name"]').text())
      .toBe('Согласуев Семён Семёнович');
    const roles = wrapper.findAll('[data-testid="app-participant-card-role"]').map((b) => b.text());
    expect(roles).toEqual(['Согласующий', 'Читатель']);
    expect(wrapper.find('[data-testid="app-participant-card-vote"]').text()).toBe('Согласовано');
    expect(wrapper.find('[data-testid="app-participant-card-comment"]').text())
      .toContain('Согласовано без замечаний');
    expect(wrapper.find('[data-testid="app-participant-card-decided-at"]').text())
      .toContain('12.05.2026');
  });

  // approval_datetime - решение по исходной заявке, а не по текущему раунду
  // дополнения (application_participants.go голоса по раундам не собирает).
  // Подпись называет это явно, иначе при открытом раунде дата читается как
  // ответ по нему.
  it('подпись даты решения называет её решением по заявке, не по раунду', () => {
    const wrapper = mountCard();

    expect(wrapper.find('[data-testid="app-participant-card-decided-at"]').text())
      .toContain('Решение по заявке:');
  });

  it('должность и место работы - из ответа, а не из ФИО', () => {
    const wrapper = mountCard();

    const text = wrapper.text();
    expect(text).toContain('Инженер');
    expect(text).toContain('Организация');
    expect(text).toContain('Компания');
  });

  it('почта - ссылка mailto', () => {
    const link = mountCard().find('[data-testid="app-participant-card-email"] a');

    expect(link.attributes('href')).toBe('mailto:a@example.com');
    expect(link.text()).toBe('a@example.com');
  });

  it('телефон - ссылка tel, показан общей маской проекта', () => {
    const link = mountCard().find('[data-testid="app-participant-card-phone"] a');

    expect(link.text()).toBe('+7 (910) 083 00-55');
    // В href маска не идёт: скобки и пробелы телефону в tel: ни к чему.
    expect(link.attributes('href')).toBe('tel:79100830055');
  });

  it('нероссийский номер не ломается маской - показан как есть', () => {
    const wrapper = mountCard({ participant: { ...FULL, phone: '+1 202 555 0123' } });
    const link = wrapper.find('[data-testid="app-participant-card-phone"] a');

    expect(link.text()).toBe('+1 202 555 0123');
    expect(link.attributes('href')).toBe('tel:+12025550123');
  });

  it('пустые контакты - прочерк, а не пустое место', () => {
    const wrapper = mountCard({ participant: EMPTY_CONTACTS });

    expect(wrapper.find('[data-testid="app-participant-card-email"]').text()).toBe('—');
    expect(wrapper.find('[data-testid="app-participant-card-phone"]').text()).toBe('—');
    expect(wrapper.find('[data-testid="app-participant-card-email"] a').exists()).toBe(false);
    // Незаполненная должность тоже прочерк - строка не исчезает, иначе состав
    // карточки прыгает от человека к человеку.
    expect(wrapper.text()).toContain('Должность');
  });

  it('скрытому по ПД говорим «скрыты», а не рисуем прочерк', () => {
    const wrapper = mountCard({ participant: HIDDEN });

    expect(wrapper.find('[data-testid="app-participant-card-name"]').text()).toBe('Имя скрыто');
    expect(wrapper.find('[data-testid="app-participant-card-pd-note"]').text())
      .toContain('не дал согласия на обработку персональных данных');
    expect(wrapper.find('[data-testid="app-participant-card-contacts-hidden"]').text())
      .toContain('Скрыты');
    // Прочерка у контактов нет вовсе: он читался бы как «не заполнено».
    expect(wrapper.find('[data-testid="app-participant-card-email"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="app-participant-card-phone"]').exists()).toBe(false);
    // Логин - та же фамилия, что и скрытое ФИО, поэтому его тоже нет.
    expect(wrapper.text()).not.toContain('i.ivanov');
  });

  it('пока запись едет - лоадер, отказ виден текстом', async () => {
    const loading = mountCard({ participant: null, loading: true });
    expect(loading.find('[data-testid="app-participant-card-name"]').exists()).toBe(false);
    expect(loading.text()).toContain('Загрузка');

    const failed = mountCard({ participant: null, error: 'Нет доступа к заявке' });
    expect(failed.find('[data-testid="app-participant-card-error"]').text()).toBe('Нет доступа к заявке');
  });

  it('закрывается по Escape', async () => {
    const wrapper = mountCard({}, { stubModal: false });
    await flushPromises();

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await flushPromises();

    expect(wrapper.emitted('close')).toBeTruthy();
    wrapper.unmount();
  });

  it('скругление 30px, как у остальных окон проекта', () => {
    const wrapper = mountCard({}, { stubModal: false });
    const modal = wrapper.findComponent({ name: 'BaseModal' });

    expect(modal.props('radius')).toBe('30px');
    wrapper.unmount();
  });

  it('лежит над окном получателей и под глобальными диалогами', () => {
    const wrapper = mountCard({}, { stubModal: false });
    const layer = wrapper.findComponent({ name: 'BaseModal' }).props('zIndex');

    // окно получателей 12000 - карточка открывается поверх него
    expect(layer).toBeGreaterThan(12000);
    // история заявки 20000 и ConfirmDialog 22000 обязаны оставаться над карточкой
    expect(layer).toBeLessThan(20000);
    wrapper.unmount();
  });
});
