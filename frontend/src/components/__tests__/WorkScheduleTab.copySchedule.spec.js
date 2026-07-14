import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// WorkScheduleTab дёргает apiRequest напрямую для CRUD слотов
// (POST/PUT/DELETE `${resourceUrl}/time-slots[/{id}]`). Мокаем клиент общим
// обработчиком; форма ответа компоненту не важна (читает только response.ok).
const apiClientMock = vi.hoisted(() => ({ apiRequest: vi.fn() }));
vi.mock('@/api/client', () => apiClientMock);

import { useDeletionsStore } from '@/stores/deletions';
import WorkScheduleTab from '../WorkScheduleTab.vue';

// Слот в реальной форме DTO (open_time/close_time c секундами "ЧЧ:ММ:СС").
function slot(id, day, open, close, opts = {}) {
  const pad = (t) => (t.length === 5 ? `${t}:00` : t);
  return {
    id,
    day_of_week: day,
    open_time: pad(open),
    close_time: pad(close),
    is_next_day: opts.is_next_day || false,
    is_active: opts.is_active !== undefined ? opts.is_active : true,
  };
}

function mountTab(timeSlots = [], props = {}) {
  setActivePinia(createPinia());
  apiClientMock.apiRequest.mockReset();
  apiClientMock.apiRequest.mockResolvedValue({ ok: true, json: async () => ({}) });
  return mount(WorkScheduleTab, {
    props: { resourceUrl: '/unload-places/5', timeSlots, readonly: false, ...props },
    global: { stubs: { ConfirmationModal: true } },
  });
}

// Собрать вызовы apiRequest в удобной форме {url, method, body}.
function calls() {
  return apiClientMock.apiRequest.mock.calls.map(([url, opts]) => ({
    url,
    method: opts?.method,
    body: opts?.body ? JSON.parse(opts.body) : null,
  }));
}

describe('WorkScheduleTab — копирование расписания', () => {
  beforeEach(() => {
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  it('показывает кнопку копирования при !readonly', () => {
    const w = mountTab([]);
    expect(w.find('[data-testid="copy-schedule-btn"]').exists()).toBe(true);
  });

  it('скрывает кнопку копирования в readonly', () => {
    const w = mountTab([], { readonly: true });
    expect(w.find('[data-testid="copy-schedule-btn"]').exists()).toBe(false);
  });

  it('копирует активные окна источника на цель, заменяя ВСЕ существующие окна цели', async () => {
    const slots = [
      slot(1, 0, '09:00', '18:00'), // Пн — источник, активное
      slot(2, 2, '10:00', '12:00'), // Ср — старое активное, будет удалено
      slot(3, 2, '14:00', '16:00', { is_active: false }), // Ср — неактивное, тоже удалить
    ];
    const w = mountTab(slots);
    w.vm.openCopyModal();
    w.vm.copySourceDay = 0;
    w.vm.toggleCopyTargetDay(2);
    await w.vm.performCopySchedule();
    await flushPromises();

    const c = calls();
    const deletes = c.filter((x) => x.method === 'DELETE');
    expect(deletes.map((d) => d.url).sort()).toEqual([
      '/unload-places/5/time-slots/2',
      '/unload-places/5/time-slots/3',
    ]);

    const posts = c.filter((x) => x.method === 'POST');
    expect(posts).toHaveLength(1);
    expect(posts[0].url).toBe('/unload-places/5/time-slots');
    expect(posts[0].body).toMatchObject({
      day_of_week: 2,
      open_time: '09:00',
      close_time: '18:00',
      is_next_day: false,
      is_active: true,
    });

    expect(w.emitted('update')).toBeTruthy();
  });

  it('копирует круглосуточный источник как окно 00:00-23:59', async () => {
    const w = mountTab([slot(1, 0, '00:00', '23:59')]); // Пн круглосуточно
    w.vm.openCopyModal();
    w.vm.toggleCopyTargetDay(1);
    await w.vm.performCopySchedule();
    await flushPromises();

    const posts = calls().filter((x) => x.method === 'POST');
    expect(posts).toHaveLength(1);
    expect(posts[0].body).toMatchObject({ day_of_week: 1, open_time: '00:00', close_time: '23:59' });
  });

  it('сохраняет is_next_day при копировании ночного окна', async () => {
    const w = mountTab([slot(1, 0, '22:00', '02:00', { is_next_day: true })]);
    w.vm.openCopyModal();
    w.vm.toggleCopyTargetDay(4);
    await w.vm.performCopySchedule();
    await flushPromises();

    const posts = calls().filter((x) => x.method === 'POST');
    expect(posts[0].body).toMatchObject({ day_of_week: 4, open_time: '22:00', close_time: '02:00', is_next_day: true });
  });

  it('пустой источник очищает день-цель без создания окон', async () => {
    const w = mountTab([slot(5, 3, '08:00', '10:00')]); // Чт занят, Пн пуст
    w.vm.openCopyModal();
    w.vm.copySourceDay = 0; // Пн — нерабочий
    w.vm.toggleCopyTargetDay(3);
    await w.vm.performCopySchedule();
    await flushPromises();

    const c = calls();
    expect(c.filter((x) => x.method === 'POST')).toHaveLength(0);
    expect(c.filter((x) => x.method === 'DELETE')).toHaveLength(1);
  });

  it('копирует на несколько дней-целей за один вызов', async () => {
    const w = mountTab([slot(1, 0, '09:00', '18:00')]);
    w.vm.openCopyModal();
    w.vm.toggleCopyTargetDay(1);
    w.vm.toggleCopyTargetDay(2);
    await w.vm.performCopySchedule();
    await flushPromises();

    const posts = calls().filter((x) => x.method === 'POST');
    expect(posts.map((p) => p.body.day_of_week).sort()).toEqual([1, 2]);
  });

  it('не даёт выбрать день-источник в качестве цели', () => {
    const w = mountTab([]);
    w.vm.copySourceDay = 2;
    w.vm.toggleCopyTargetDay(2);
    expect(w.vm.copyTargetDays).not.toContain(2);
  });

  it('смена источника снимает его из выбранных целей', () => {
    const w = mountTab([]);
    w.vm.toggleCopyTargetDay(3);
    expect(w.vm.copyTargetDays).toContain(3);
    w.vm.selectCopySourceDay(3);
    expect(w.vm.copyTargetDays).not.toContain(3);
  });

  it('уведомляет об успехе и закрывает модалку', async () => {
    const w = mountTab([slot(1, 0, '09:00', '18:00')]);
    const notifySpy = vi.spyOn(useDeletionsStore(), 'notify');
    w.vm.openCopyModal();
    w.vm.toggleCopyTargetDay(1);
    await w.vm.performCopySchedule();
    await flushPromises();

    expect(w.vm.copyModalOpen).toBe(false);
    expect(notifySpy).toHaveBeenCalledWith(expect.objectContaining({ type: 'success' }));
  });

  it('сбой на первом же create (ничего не замутировано) не эмитит update', async () => {
    const w = mountTab([slot(1, 0, '09:00', '18:00')]);
    const notifySpy = vi.spyOn(useDeletionsStore(), 'notify');
    apiClientMock.apiRequest.mockResolvedValue({ ok: false, json: async () => ({}), text: async () => 'err' });
    w.vm.openCopyModal();
    w.vm.toggleCopyTargetDay(1); // Вт пуст -> первый шаг POST -> ok:false -> throw до мутации
    await w.vm.performCopySchedule();
    await flushPromises();

    expect(notifySpy).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    expect(w.emitted('update')).toBeFalsy();
  });

  it('частичный сбой (create прошёл, delete упал) ЭМИТИТ update, чтобы UI перечитал реальность', async () => {
    // Пн-источник, Ср-цель со старым окном: POST копии проходит, DELETE старого падает.
    const w = mountTab([slot(1, 0, '09:00', '18:00'), slot(2, 2, '10:00', '12:00')]);
    const notifySpy = vi.spyOn(useDeletionsStore(), 'notify');
    apiClientMock.apiRequest.mockImplementation(async (url, opts) => {
      if (opts?.method === 'DELETE') return { ok: false, json: async () => ({}), text: async () => 'err' };
      return { ok: true, json: async () => ({}) };
    });
    w.vm.openCopyModal();
    w.vm.copySourceDay = 0;
    w.vm.toggleCopyTargetDay(2);
    await w.vm.performCopySchedule();
    await flushPromises();

    const c = calls();
    // копия источника создана ДО падения на удалении старого окна
    expect(c.some((x) => x.method === 'POST' && x.body.day_of_week === 2)).toBe(true);
    expect(notifySpy).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
    // update эмитится, несмотря на ошибку - на сервере уже есть изменения
    expect(w.emitted('update')).toBeTruthy();
  });

  it('не выполняет копирование без выбранных дней-целей', async () => {
    const w = mountTab([slot(1, 0, '09:00', '18:00')]);
    w.vm.openCopyModal();
    await w.vm.performCopySchedule();
    await flushPromises();
    expect(apiClientMock.apiRequest).not.toHaveBeenCalled();
  });
});
