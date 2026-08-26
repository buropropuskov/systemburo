import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';

import ApplicationConfirmation from '../ApplicationConfirmation.vue';

// Опорный момент - согласующих назначаем относительно него, чтобы "N дней" было детерминированным.
const NOW = new Date(2026, 6, 10, 12, 0, 0);
const daysAgo = (n) => new Date(NOW.getTime() - n * 86_400_000).toISOString();

const SILENCE = '.user-silence-block';

function mountConfirmation(application, responsibleUsers) {
  return mount(ApplicationConfirmation, {
    props: { application, responsibleUsers },
    global: { stubs: { LoaderSpinner: true } },
  });
}

const awaitingApp = { id: 1, confirmation: 'Согласование', status: 'Непрочитано' };

describe('ApplicationConfirmation - метка молчащего согласующего (#1315 S3)', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
  });
  afterEach(() => vi.useRealTimers());

  it('обязательный молчащий N дней с напоминаниями -> "Не отвечает N дней, напомнили K раз"', () => {
    const wrapper = mountConfirmation(awaitingApp, [
      { id: 5, username: 'a', required_approval: true, approval_status: 'pending', created_at: daysAgo(4), reminder_count: 2 },
    ]);
    const labels = wrapper.findAll(SILENCE);
    expect(labels).toHaveLength(1);
    expect(labels[0].text()).toBe('Не отвечает 4 дня, напомнили 2 раза');
  });

  it('без напоминаний (K=0) -> только "Не отвечает N дней"', () => {
    const wrapper = mountConfirmation(awaitingApp, [
      { id: 5, username: 'a', required_approval: true, approval_status: 'pending', created_at: daysAgo(1), reminder_count: 0 },
    ]);
    expect(wrapper.find(SILENCE).text()).toBe('Не отвечает 1 день');
  });

  it('есть обязательные -> метку показываем ТОЛЬКО обязательным, необязательному нет (его голос не нужен)', () => {
    const wrapper = mountConfirmation(awaitingApp, [
      { id: 5, username: 'req', required_approval: true, approval_status: 'pending', created_at: daysAgo(5), reminder_count: 1 },
      { id: 6, username: 'opt', required_approval: false, approval_status: 'pending', created_at: daysAgo(5), reminder_count: 0 },
    ]);
    const labels = wrapper.findAll(SILENCE);
    expect(labels).toHaveLength(1);
    expect(labels[0].text()).toContain('напомнили 1 раз');
  });

  it('обязательных нет -> метку показываем всем pending', () => {
    const wrapper = mountConfirmation(awaitingApp, [
      { id: 6, username: 'o1', required_approval: false, approval_status: 'pending', created_at: daysAgo(3), reminder_count: 0 },
      { id: 7, username: 'o2', required_approval: false, approval_status: 'pending', created_at: daysAgo(3), reminder_count: 0 },
    ]);
    expect(wrapper.findAll(SILENCE)).toHaveLength(2);
  });

  it('уже проголосовавшему (approved) метку не показываем', () => {
    const wrapper = mountConfirmation(awaitingApp, [
      { id: 5, username: 'a', required_approval: true, approval_status: 'approved', created_at: daysAgo(5), reminder_count: 2 },
    ]);
    expect(wrapper.find(SILENCE).exists()).toBe(false);
  });

  it('заявка уже согласована -> метки нет ни у кого', () => {
    const wrapper = mountConfirmation(
      { id: 1, confirmation: 'Согласовано', status: 'В работе' },
      [{ id: 5, username: 'a', required_approval: true, approval_status: 'pending', created_at: daysAgo(9), reminder_count: 3 }],
    );
    expect(wrapper.find(SILENCE).exists()).toBe(false);
  });

  it('назначен сегодня (0 дней) -> метки нет (шум не рисуем)', () => {
    const wrapper = mountConfirmation(awaitingApp, [
      { id: 5, username: 'a', required_approval: true, approval_status: 'pending', created_at: daysAgo(0), reminder_count: 0 },
    ]);
    expect(wrapper.find(SILENCE).exists()).toBe(false);
  });
});
