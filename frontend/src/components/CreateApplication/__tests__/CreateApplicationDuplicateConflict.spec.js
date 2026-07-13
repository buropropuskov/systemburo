import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';

// #952: дубль из ЛК приходит во временный pendingDuplicateState. Если в форме уже есть
// черновик - модалка "Заменить / Объединить / Отмена"; если пусто - берётся сразу.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn().mockReturnValue({ token: 'test-token' }),
}));

const EXISTING = {
  message: 'мой черновик',
  attachments: [{ local_id: 'a1', attachment_type: 'cars', display_name: 'Мои машины' }],
  vehiclesByAttachment: { a1: [{ id: 1, plateNumber: 'A001' }] },
};
const PENDING = {
  message: 'дубль',
  attachments: [{ local_id: 'd1', attachment_type: 'people', display_name: 'Дубль-люди' }],
  employeesByAttachment: { d1: [{ id: 1, lastName: 'Иванов' }] },
  attachmentDatesByAttachment: { d1: { isOneDay: true, singleDate: '02.07.2026' } },
};

beforeEach(() => {
  setActivePinia(createPinia());
  localStorage.clear();
});

async function mountApp() {
  const w = shallowMount(CreateApplication);
  await flushPromises();
  return w;
}

const draft = () => JSON.parse(localStorage.getItem('draftApplicationState') || 'null');
const localIds = w => w.vm.attachments.map(a => a.local_id);

describe('CreateApplication - конфликт дублирования (#952)', () => {
  it('mergeDrafts: существующее сохраняется, вложения дубля добавляются', async () => {
    const w = await mountApp();
    const merged = w.vm.mergeDrafts(EXISTING, PENDING);
    expect(merged.message).toBe('мой черновик'); // шапка/сообщение не трогаются
    expect(merged.attachments.map(a => a.local_id)).toEqual(['a1', 'd1']);
    expect(merged.vehiclesByAttachment).toEqual({ a1: [{ id: 1, plateNumber: 'A001' }] });
    expect(merged.employeesByAttachment).toEqual({ d1: [{ id: 1, lastName: 'Иванов' }] });
  });

  it('пустая форма + дубль -> берётся сразу, без модалки', async () => {
    localStorage.setItem('pendingDuplicateState', JSON.stringify(PENDING));
    const w = await mountApp();
    expect(w.vm.showDuplicateConflict).toBe(false);
    expect(localIds(w)).toEqual(['d1']);
    expect(localStorage.getItem('pendingDuplicateState')).toBeNull();
  });

  it('есть черновик + дубль -> показывается модалка, данные пока прежние', async () => {
    localStorage.setItem('draftApplicationState', JSON.stringify(EXISTING));
    localStorage.setItem('pendingDuplicateState', JSON.stringify(PENDING));
    const w = await mountApp();
    expect(w.vm.showDuplicateConflict).toBe(true);
    expect(localIds(w)).toEqual(['a1']); // пока показан существующий черновик
  });

  it('Заменить -> данные целиком из дубля', async () => {
    localStorage.setItem('draftApplicationState', JSON.stringify(EXISTING));
    localStorage.setItem('pendingDuplicateState', JSON.stringify(PENDING));
    const w = await mountApp();

    w.vm.onDuplicateConflictReplace();
    await flushPromises();

    expect(localIds(w)).toEqual(['d1']);
    expect(w.vm.showDuplicateConflict).toBe(false);
    expect(localStorage.getItem('pendingDuplicateState')).toBeNull();
    expect(draft().attachments.map(a => a.local_id)).toEqual(['d1']);
  });

  it('Объединить -> существующие + вложения дубля', async () => {
    localStorage.setItem('draftApplicationState', JSON.stringify(EXISTING));
    localStorage.setItem('pendingDuplicateState', JSON.stringify(PENDING));
    const w = await mountApp();

    w.vm.onDuplicateConflictMerge();
    await flushPromises();

    expect(localIds(w)).toEqual(['a1', 'd1']);
    expect(w.vm.message).toBe('мой черновик'); // сообщение осталось прежним
    expect(w.vm.showDuplicateConflict).toBe(false);
    expect(localStorage.getItem('pendingDuplicateState')).toBeNull();
  });

  it('Отмена -> остаётся прежний черновик, дубль отброшен', async () => {
    localStorage.setItem('draftApplicationState', JSON.stringify(EXISTING));
    localStorage.setItem('pendingDuplicateState', JSON.stringify(PENDING));
    const w = await mountApp();

    w.vm.onDuplicateConflictCancel();
    await flushPromises();

    expect(localIds(w)).toEqual(['a1']);
    expect(w.vm.showDuplicateConflict).toBe(false);
    expect(localStorage.getItem('pendingDuplicateState')).toBeNull();
    expect(draft().attachments.map(a => a.local_id)).toEqual(['a1']);
  });
});
