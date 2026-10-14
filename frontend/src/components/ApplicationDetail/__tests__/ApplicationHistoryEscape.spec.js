import { describe, it, expect, vi } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia } from 'pinia';
import { setModalOpen, releaseModal } from '@/utils/modalStack';
import ApplicationHistory from '../ApplicationHistory.vue';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: true, json: async () => [] }) }));

describe('ApplicationHistory Escape', () => {
  it('closes only when history is the top window', async () => {
    const w = shallowMount(ApplicationHistory, { props: { applicationId: 1 }, global: { plugins: [createPinia()] } });
    const upper = {};
    try {
      w.vm.openModal();
      await flushPromises();
      setModalOpen(upper, true, 21000);
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      await flushPromises();
      expect(w.vm.showModal).toBe(true);
      releaseModal(upper);
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      await flushPromises();
      expect(w.vm.showModal).toBe(false);
    } finally { releaseModal(upper); w.unmount(); }
  });
});