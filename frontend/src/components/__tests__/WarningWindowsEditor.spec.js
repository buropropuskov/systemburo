import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: vi.fn().mockResolvedValue({}) })),
}));
import { apiRequest } from '@/api/client';
import { useDeletionsStore } from '@/stores/deletions';
import WarningWindowsEditor from '../WarningWindowsEditor.vue';

/**
 * Монтирует редактор окон с застабленной ConfirmationModal.
 * @param {Array} windows список окон
 * @returns {{ wrapper: import('@vue/test-utils').VueWrapper, notify: import('vitest').Mock }}
 */
function mountEditor(windows = []) {
  setActivePinia(createPinia());
  const notify = vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {});
  const wrapper = mount(WarningWindowsEditor, {
    props: { resourceUrl: '/unload-places/5', windows },
    global: { stubs: { ConfirmationModal: true, Teleport: true } },
  });
  return { wrapper, notify };
}

// Разбирает тело последнего запроса apiRequest в объект.
function lastBody() {
  const call = apiRequest.mock.calls[apiRequest.mock.calls.length - 1];
  return { url: call[0], method: call[1].method, payload: JSON.parse(call[1].body) };
}

const okResp = () => ({ ok: true, json: vi.fn().mockResolvedValue({}) });

describe('WarningWindowsEditor', () => {
  beforeEach(() => {
    apiRequest.mockClear();
    apiRequest.mockResolvedValue(okResp());
  });

  it('рендерит список окон с условием, временем и текстом; выключенное помечено', () => {
    const { wrapper } = mountEditor([
      { id: 1, day_of_week: 0, time_from: '12:00:00', time_to: '13:00:00', is_next_day: false, message: 'только малогабарит', is_active: true },
      { id: 2, day_of_week: null, time_from: null, time_to: null, is_next_day: false, message: 'проход по пропуску', is_active: false },
    ]);
    const text = wrapper.text();
    expect(text).toContain('Пн');
    expect(text).toContain('12:00–13:00');
    expect(text).toContain('только малогабарит');
    expect(text).toContain('Каждый день');
    expect(text).toContain('весь день');
    expect(text).toContain('проход по пропуску');
    // Выключенное окно помечено и получило класс приглушения.
    expect(text).toContain('выключено');
    expect(wrapper.find('.ww-card--inactive').exists()).toBe(true);
  });

  it('пустой список показывает заглушку', () => {
    const { wrapper } = mountEditor([]);
    expect(wrapper.text()).toContain('Окон-предупреждений пока нет');
  });

  it('добавление окна с интервалом шлёт POST со ВСЕМИ полями, включая is_active и is_next_day', async () => {
    const { wrapper } = mountEditor([]);
    await wrapper.setData({
      modalOpen: true, editingId: null, modalDay: 2,
      modalAllDay: false, modalTimeFrom: '09:00', modalTimeTo: '18:00',
      modalMessage: '  въезд по записи  ', modalActive: true,
    });
    await wrapper.vm.saveWindow();

    const { url, method, payload } = lastBody();
    expect(method).toBe('POST');
    expect(url).toBe('/unload-places/5/warning-windows');
    // Полный набор ключей: частичное тело сбросило бы недосланные поля на бэке.
    expect(Object.keys(payload).sort()).toEqual(
      ['day_of_week', 'is_active', 'is_next_day', 'message', 'time_from', 'time_to'].sort(),
    );
    expect(payload).toMatchObject({
      day_of_week: 2, time_from: '09:00', time_to: '18:00',
      is_next_day: false, message: 'въезд по записи', is_active: true,
    });
    expect(wrapper.emitted('update')).toBeTruthy();
  });

  it('"Каждый день" шлёт day_of_week=null; "Весь день" шлёт time_from/time_to=null', async () => {
    const { wrapper } = mountEditor([]);
    await wrapper.setData({
      modalOpen: true, editingId: null, modalDay: null,
      modalAllDay: true, modalTimeFrom: '', modalTimeTo: '',
      modalMessage: 'нужен пропуск', modalActive: true,
    });
    await wrapper.vm.saveWindow();

    const { payload } = lastBody();
    expect(payload.day_of_week).toBeNull();
    expect(payload.time_from).toBeNull();
    expect(payload.time_to).toBeNull();
    expect(payload.is_next_day).toBe(false);
  });

  it('ночной интервал (конец < начало) выставляет is_next_day=true', async () => {
    const { wrapper } = mountEditor([]);
    await wrapper.setData({
      modalOpen: true, editingId: null, modalDay: 0,
      modalAllDay: false, modalTimeFrom: '22:00', modalTimeTo: '02:00',
      modalMessage: 'ночью тихо', modalActive: true,
    });
    await wrapper.vm.saveWindow();
    expect(lastBody().payload.is_next_day).toBe(true);
  });

  it('редактирование шлёт PUT на /{id} со всеми полями', async () => {
    const win = { id: 7, day_of_week: 1, time_from: '10:00:00', time_to: '11:00:00', is_next_day: false, message: 'старое', is_active: true };
    const { wrapper } = mountEditor([win]);
    wrapper.vm.editWindow(win);
    await wrapper.vm.$nextTick();
    await wrapper.setData({ modalMessage: 'новое' });
    await wrapper.vm.saveWindow();

    const { url, method, payload } = lastBody();
    expect(method).toBe('PUT');
    expect(url).toBe('/unload-places/5/warning-windows/7');
    expect(payload).toMatchObject({
      day_of_week: 1, time_from: '10:00', time_to: '11:00', message: 'новое', is_active: true,
    });
  });

  it('быстрый тумблер is_active шлёт PUT full-replace всеми полями с перевёрнутым is_active (урок S2a)', async () => {
    const win = { id: 9, day_of_week: 3, time_from: '08:00:00', time_to: '20:00:00', is_next_day: false, message: 'важное окно', is_active: true };
    const { wrapper } = mountEditor([win]);
    await wrapper.vm.toggleActive(win);

    const { url, method, payload } = lastBody();
    expect(method).toBe('PUT');
    expect(url).toBe('/unload-places/5/warning-windows/9');
    // Все поля окна сохранены, изменился только is_active - иначе частичный PUT
    // обнулил бы message/time и реактивировал бы окно.
    expect(payload).toEqual({
      day_of_week: 3, time_from: '08:00', time_to: '20:00',
      is_next_day: false, message: 'важное окно', is_active: false,
    });
    expect(wrapper.emitted('update')).toBeTruthy();
  });

  it('ошибка бэка показывается через message из json-ответа, не сырой текст', async () => {
    const { wrapper, notify } = mountEditor([]);
    apiRequest.mockResolvedValueOnce({ ok: false, json: vi.fn().mockResolvedValue({ message: 'окно пересекается' }) });
    await wrapper.setData({
      modalOpen: true, editingId: null, modalDay: null,
      modalAllDay: true, modalMessage: 'текст', modalActive: true,
    });
    await wrapper.vm.saveWindow();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'окно пересекается', type: 'error' }));
  });

  it('toggleActive при ошибке бэка показывает реальное сообщение, а не дженерик', async () => {
    const win = { id: 3, day_of_week: null, time_from: null, time_to: null, is_next_day: false, message: 'm', is_active: true };
    const { wrapper, notify } = mountEditor([win]);
    apiRequest.mockResolvedValueOnce({ ok: false, json: vi.fn().mockResolvedValue({ message: 'окно занято' }) });
    await wrapper.vm.toggleActive(win);
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'окно занято', type: 'error' }));
  });

  it('не-JSON тело ошибки (шлюз 502) не роняет обработку: дефолтное сообщение, не SyntaxError', async () => {
    const { wrapper, notify } = mountEditor([]);
    apiRequest.mockResolvedValueOnce({ ok: false, json: vi.fn().mockRejectedValue(new Error('Unexpected token <')) });
    await wrapper.setData({
      modalOpen: true, editingId: null, modalDay: null,
      modalAllDay: true, modalMessage: 'текст', modalActive: true,
    });
    await wrapper.vm.saveWindow();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'ошибка сервера', type: 'error' }));
  });
});
