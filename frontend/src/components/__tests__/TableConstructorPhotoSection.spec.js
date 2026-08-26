import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
import TableConstructorPhotoSection from '../TableConstructorPhotoSection.vue';

function mountPS() {
  setActivePinia(createPinia());
  return mount(TableConstructorPhotoSection, { props: { tableId: 5, photos: [] } });
}

describe('TableConstructorPhotoSection.errorMessage', () => {
  it('берёт message из ответа бэка, а не сырое тело', async () => {
    const wr = mountPS();
    const msg = await wr.vm.errorMessage(
      { json: () => Promise.resolve({ message: 'Формат не поддерживается' }) },
      'фолбэк',
    );
    expect(msg).toBe('Формат не поддерживается');
  });

  it('фолбэк, если message пустой или тело не JSON', async () => {
    const wr = mountPS();
    expect(await wr.vm.errorMessage({ json: () => Promise.resolve({}) }, 'фолбэк')).toBe('фолбэк');
    expect(await wr.vm.errorMessage({ json: () => Promise.reject(new Error('x')) }, 'фолбэк')).toBe('фолбэк');
  });
});
