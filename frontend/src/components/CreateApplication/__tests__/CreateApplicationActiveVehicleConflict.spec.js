import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';

// Сообщение о блокировке подачи должно показывать, В КАКОЙ заявке машина уже
// активна и до какого срока (данные из GET /cars/check-active), а не только
// номер+марку - иначе непонятно, что именно блокирует.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn().mockReturnValue({ token: 'test-token' }),
}));

beforeEach(() => {
  setActivePinia(createPinia());
  localStorage.clear();
});

async function mountApp() {
  const w = shallowMount(CreateApplication);
  await flushPromises();
  return w;
}

describe('CreateApplication - конфликт активной машины при подаче', () => {
  it('строка конфликта содержит номер блокирующей заявки и срок', async () => {
    const w = await mountApp();
    const out = w.vm.formatActiveVehicleConflict({
      plateNumber: 'К 234 ОУ 023',
      mark: 'BMW X5',
      activeInfo: {
        application_number: '№ 20260718/001',
        entry_date_to: '2026-07-31',
        entry_time_to: '20:00:00',
      },
    });
    expect(out).toContain('К 234 ОУ 023 BMW X5');
    expect(out).toContain('№ 20260718/001');
    expect(out).toContain('до 31.07.2026 20:00');
    expect(out).not.toContain('20:00:00'); // время обрезается до HH:MM
  });

  it('без срока показывает только заявку', async () => {
    const w = await mountApp();
    const out = w.vm.formatActiveVehicleConflict({
      plateNumber: 'A001',
      mark: 'Kamaz',
      activeInfo: { application_number: '№ 20260718/002' },
    });
    expect(out).toBe('A001 Kamaz (в заявке № 20260718/002)');
  });

  it('без номера заявки откатывается на номер+марку', async () => {
    const w = await mountApp();
    expect(w.vm.formatActiveVehicleConflict({ plateNumber: 'A001', mark: 'Kamaz' }))
      .toBe('A001 Kamaz');
    expect(w.vm.formatActiveVehicleConflict({ plateNumber: 'A001', mark: 'Kamaz', activeInfo: {} }))
      .toBe('A001 Kamaz');
  });
});
