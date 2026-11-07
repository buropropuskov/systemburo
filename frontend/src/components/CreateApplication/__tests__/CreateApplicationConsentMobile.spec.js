import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CreateApplication from '../CreateApplication.vue';
import ApplicationConfirmation from '../ApplicationConfirmation.vue';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn().mockReturnValue({ token: 'test-token' }),
}));
vi.mock('pdfjs-dist/build/pdf.worker.min.mjs?worker', () => ({ default: class {} }));

beforeEach(() => {
  setActivePinia(createPinia());
  localStorage.clear();
});

async function mountApp() {
  const w = shallowMount(CreateApplication);
  await flushPromises();
  return w;
}

describe('CreateApplication - редакция документа при подаче', () => {
  it('использует общий компонент и сбрасывает отметку после изменения состава', async () => {
    const w = await mountApp();
    await w.setData({ selectedAttachment: { id: 1, attachment_type: 'people' } });
    w.getComponent(ApplicationConfirmation).vm.$emit('update:modelValue', { version: 2, hash: 'current' });
    expect(w.vm.consentGiven).toEqual({ version: 2, hash: 'current' });
    await w.setData({ employeesByAttachment: { 1: [{ firstName: 'Иван' }] } });
    expect(w.vm.consentGiven).toBeNull();
  });
});
