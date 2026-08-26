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
    expect(wrapper.vm.startDate).toBeInstanceOf(Date);
    expect(wrapper.vm.startTime).toMatch(/^\d{2}:\d{2}$/);
    expect(new Date(wrapper.vm.plannedEndIso) > new Date(wrapper.vm.plannedStartIso)).toBe(true);
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
    expect(wrapper.vm.draftSupportPhone).toBe('+7 (495) 123 45-67');
    expect(wrapper.vm.plannedStartIso).toBe('2026-07-28T10:00:00.000Z');
    expect(wrapper.vm.plannedEndIso).toBe('2026-07-28T14:00:00.000Z');
  });

  it('накладывает телефонную маску проекта на ввод', async () => {
    const wrapper = await mountView();
    const phone = wrapper.get('[data-testid="support-phone"]');
    await phone.setValue('84951234567');
    expect(wrapper.vm.draftSupportPhone).toBe('+7 (495) 123 45-67');
  });

  it('достраивает неполное время до ЧЧ:ММ', async () => {
    const wrapper = await mountView();
    expect(wrapper.vm.maskTime('0930')).toBe('09:30');
    expect(wrapper.vm.normalizeTime('9')).toBe('09:00');
    expect(wrapper.vm.normalizeTime('930')).toBe('09:30');
    expect(wrapper.vm.normalizeTime('2599')).toBe('23:59');
    expect(wrapper.vm.normalizeTime('')).toBe('');
  });

  it('шлёт окно в ISO и оба контакта', async () => {
    const wrapper = await mountView();
    wrapper.vm.draftMessage = 'Обновление до 1.5.0';
    wrapper.vm.startDate = new Date(2026, 6, 28);
    wrapper.vm.startTime = '10:00';
    wrapper.vm.endDate = new Date(2026, 6, 28);
    wrapper.vm.endTime = '14:00';
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
    wrapper.vm.startDate = new Date(2026, 6, 28);
    wrapper.vm.startTime = '14:00';
    wrapper.vm.endDate = new Date(2026, 6, 28);
    wrapper.vm.endTime = '10:00';

    await wrapper.vm.enable();

    expect(apiRequest).not.toHaveBeenCalled();
    expect(wrapper.vm.errorText).toContain('позже начала');
  });

  it('не открывает подтверждение с незаполненным окном', async () => {
    const wrapper = await mountView();
    wrapper.vm.endDate = null;

    wrapper.vm.confirmEnable();

    expect(wrapper.vm.confirmOpen).toBe(false);
    expect(wrapper.vm.errorText).toContain('Выберите даты');
  });

  it('не показывает пользователю внутренние идентификаторы типов', async () => {
    const wrapper = await mountView();
    expect(wrapper.html()).not.toContain('type_id');
  });
});
