import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';

// #952: при переходе на "Оформление и подача заявки" из дубликата открывается
// первое вложение сверху (раньше форма оставалась без выбранного вложения).

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

describe('CreateApplication - открытие первого вложения из черновика дубля (#952)', () => {
  it('черновик с вложениями -> выбрано первое (верхнее)', async () => {
    localStorage.setItem('draftApplicationState', JSON.stringify({
      message: 'дубль',
      attachments: [
        { local_id: 'a1', attachment_type: 'cars', display_name: 'Машины' },
        { local_id: 'a2', attachment_type: 'people', display_name: 'Люди' },
      ],
    }));

    const w = await mountApp();

    expect(w.vm.selectedAttachment).toBeTruthy();
    expect(w.vm.selectedAttachment.local_id).toBe('a1');
  });

  it('без черновика -> вложение не выбрано', async () => {
    const w = await mountApp();
    expect(w.vm.selectedAttachment).toBeNull();
  });

  it('пустой список вложений -> вложение не выбрано', async () => {
    localStorage.setItem('draftApplicationState', JSON.stringify({ message: 'x', attachments: [] }));
    const w = await mountApp();
    expect(w.vm.selectedAttachment).toBeNull();
  });
});
