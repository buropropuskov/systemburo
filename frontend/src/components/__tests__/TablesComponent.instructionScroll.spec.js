import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { releaseBodyScrollLock, setBodyScrollLock } from '@/utils/bodyScrollLock';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(async (url) => ({
    ok: true,
    json: async () => url.startsWith('/system-tables/name/')
      ? { table: { id: 42, name: 'kpp_4', display_name: 'КПП №4', table_type: 'cars' } }
      : [],
  })),
}));

import TablesComponent from '../TablesComponent.vue';

describe('блокировка фона инструкции таблицы', () => {
  let wrapper;
  const neighbour = {};

  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.clear();
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
    releaseBodyScrollLock(neighbour);
  });

  async function openInstruction() {
    wrapper = mount(TablesComponent, {
      global: {
        stubs: {
          teleport: true, transition: false, 'transition-group': false,
          RouterLink: true, CarsTable: true, PeopleTable: true, FactTable: true,
          ApplicationDetail: true, TableExportModal: true, ManualAddModal: true,
          BaseDropdown: true, DateFilter: true,
        },
        mocks: {
          $route: { params: { tableName: 'kpp_4' } },
          $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
        },
      },
    });
    await flushPromises();
    await wrapper.get('[data-testid="ob-table-instruction"]').trigger('click');
  }

  it('блокирует документ до закрытия инструкции по Escape', async () => {
    await openInstruction();
    expect(document.body.style.overflow).toBe('hidden');
    expect(document.documentElement.style.overflow).toBe('hidden');
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await flushPromises();
    expect(document.body.style.overflow).toBe('');
    expect(document.documentElement.style.overflow).toBe('');
  });

  it('снимает собственную блокировку при уходе со страницы', async () => {
    await openInstruction();
    expect(document.body.style.overflow).toBe('hidden');
    wrapper.unmount();
    wrapper = null;
    expect(document.body.style.overflow).toBe('');
    expect(document.documentElement.style.overflow).toBe('');
  });

  it('не снимает блокировку другого открытого окна', async () => {
    await openInstruction();
    setBodyScrollLock(neighbour, true);
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await flushPromises();
    expect(document.body.style.overflow).toBe('hidden');
    releaseBodyScrollLock(neighbour);
    expect(document.body.style.overflow).toBe('');
  });
});
