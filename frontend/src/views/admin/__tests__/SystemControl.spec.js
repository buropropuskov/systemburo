import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const apiRequest = vi.fn();
vi.mock('@/api/client', () => ({
  apiRequest: (...a) => apiRequest(...a),
}));

import SystemControl from '../SystemControl.vue';

/** Ответ бэка в форме, которую отдаёт apiRequest после разворота envelope. */
function respond(payload) {
  return { ok: true, json: async () => payload };
}

const DISABLED = {
  enabled: false,
  message: '',
  support_email: 'support@buropropuskov.ru',
};

async function mountView(payload = DISABLED) {
  apiRequest.mockResolvedValue(respond(payload));
  const wrapper = shallowMount(SystemControl);
  await flushPromises();
  apiRequest.mockClear();
  return wrapper;
}

/** Тело последнего PUT-запроса. */
function lastRequestBody() {
  const call = apiRequest.mock.calls.at(-1);
  return JSON.parse(call[1].body);
}

describe('SystemControl - режим технических работ', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    apiRequest.mockReset();
  });

  it('подставляет окно ближайших часов, когда работы ещё не объявлены', async () => {
    const wrapper = await mountView();
    expect(wrapper.vm.draftPlannedStart).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/);
    expect(new Date(wrapper.vm.draftPlannedEnd) > new Date(wrapper.vm.draftPlannedStart)).toBe(true);
  });

  it('показывает сохранённое окно и телефон поддержки', async () => {
    const wrapper = await mountView({
      enabled: true,
      message: 'Миграция базы',
      planned_start: '2026-07-28T10:00:00Z',
      planned_end: '2026-07-28T14:00:00Z',
      support_email: 'help@example.com',
      support_phone: '+7 495 123-45-67',
    });
    expect(wrapper.vm.draftMessage).toBe('Миграция базы');
    expect(wrapper.vm.draftSupportPhone).toBe('+7 495 123-45-67');
    expect(wrapper.vm.draftPlannedStart).not.toBe('');
    expect(wrapper.vm.draftPlannedEnd).not.toBe('');
  });

  it('шлёт окно в ISO и оба контакта', async () => {
    const wrapper = await mountView();
    wrapper.vm.draftMessage = 'Обновление до 1.5.0';
    wrapper.vm.draftPlannedStart = '2026-07-28T10:00';
    wrapper.vm.draftPlannedEnd = '2026-07-28T14:00';
    wrapper.vm.draftSupportEmail = 'help@example.com';
    wrapper.vm.draftSupportPhone = '+7 495 123-45-67';
    apiRequest.mockResolvedValue(respond({ enabled: true }));

    await wrapper.vm.enable();

    const body = lastRequestBody();
    expect(body.enabled).toBe(true);
    expect(body.message).toBe('Обновление до 1.5.0');
    expect(body.support_phone).toBe('+7 495 123-45-67');
    expect(new Date(body.planned_start).toISOString()).toBe(body.planned_start);
    expect(new Date(body.planned_end) > new Date(body.planned_start)).toBe(true);
  });

  it('не шлёт запрос, если окончание не позже начала', async () => {
    const wrapper = await mountView();
    wrapper.vm.draftPlannedStart = '2026-07-28T14:00';
    wrapper.vm.draftPlannedEnd = '2026-07-28T10:00';

    await wrapper.vm.enable();

    expect(apiRequest).not.toHaveBeenCalled();
    expect(wrapper.vm.errorText).toContain('позже начала');
  });

  it('не открывает подтверждение с незаполненным окном', async () => {
    const wrapper = await mountView();
    wrapper.vm.draftPlannedEnd = '';

    wrapper.vm.confirmEnable();

    expect(wrapper.vm.confirmOpen).toBe(false);
    expect(wrapper.vm.errorText).toContain('Укажите начало и окончание');
  });

  it('не показывает пользователю внутренние идентификаторы типов', async () => {
    const wrapper = await mountView();
    expect(wrapper.html()).not.toContain('type_id');
  });
});
