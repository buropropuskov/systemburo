import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve([])),
}));
import { apiRequest } from '@/api/client';
import { usePermissionsStore } from '@/stores/permissions';
import TableConstructor from '../TableConstructor.vue';

const shellStub = { template: '<div><slot /></div>' };

/**
 * Монтирует конструктор с застабленными детьми и заданной таблицей в детали.
 * @param {{ isActive: boolean, mode?: string, allow?: boolean, name?: string }} opts
 */
function mountC({ isActive, mode = 'super', allow = false, name = 'cargo_cars', query = {}, setSelected = true }) {
  setActivePinia(createPinia());
  const perms = usePermissionsStore();
  perms.mode = mode;
  perms.effective = allow ? { [`table.${name}.versions`]: { value: 'allow' } } : {};
  const push = vi.fn();
  const replace = vi.fn();
  const wrapper = mount(TableConstructor, {
    global: {
      stubs: {
        AdminPageShell: shellStub,
        SearchComponent: true,
        RefreshButton: true,
        BaseDropdown: true,
        TextConstructor: true,
        WorkScheduleTab: true,
        SystemTableColumnsTab: true,
        SystemTableAppearanceTab: true,
        TableConstructorCreateModal: true,
        TableConstructorPhotoSection: true,
        SystemTableHistoryModal: true,
        ConfirmationModal: true,
      },
      mocks: { $router: { push, replace }, $route: { query } },
    },
  });
  if (setSelected) {
    wrapper.vm.selectedTable = {
      table: { id: 7, name, display_name: 'Грузовые', table_type: 'cars', is_active: isActive },
    };
  }
  return { wrapper, push, replace };
}

describe('TableConstructor — кнопка «Версии» архивной таблицы (#980)', () => {
  beforeEach(() => {
    apiRequest.mockClear();
    apiRequest.mockResolvedValue([]);
  });

  it('архивная таблица + право: кнопка видна, клик ведёт на роут версий', async () => {
    const { wrapper, push } = mountC({ isActive: false, mode: 'super' });
    await wrapper.vm.$nextTick();

    const btn = wrapper.find('[data-testid="table-versions-btn"]');
    expect(btn.exists()).toBe(true);

    await btn.trigger('click');
    // from=admin: "Назад" на странице версий вернёт в конструктор, а не на
    // публичную /table/:name (её для архивной таблицы нет).
    expect(push).toHaveBeenCalledWith('/table/cargo_cars/versions?from=admin');
  });

  it('активная таблица: кнопки «Версии» нет (версии архивных - из Конструктора)', async () => {
    const { wrapper } = mountC({ isActive: true, mode: 'super' });
    await wrapper.vm.$nextTick();

    expect(wrapper.find('[data-testid="table-versions-btn"]').exists()).toBe(false);
  });

  it('архивная таблица без права versions: кнопка скрыта (гейт совпадает с роутом)', async () => {
    const { wrapper } = mountC({ isActive: false, mode: 'normal', allow: false });
    await wrapper.vm.$nextTick();

    expect(wrapper.find('[data-testid="table-versions-btn"]').exists()).toBe(false);
  });

  it('архивная таблица, normal с явным грантом versions: кнопка видна', async () => {
    const { wrapper } = mountC({ isActive: false, mode: 'normal', allow: true });
    await wrapper.vm.$nextTick();

    expect(wrapper.find('[data-testid="table-versions-btn"]').exists()).toBe(true);
  });

  it('возврат со страницы версий (?open=<name>) открывает архив и выбирает ту таблицу', async () => {
    apiRequest.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([
        { table: { id: 7, name: 'cargo_cars', display_name: 'Грузовые', table_type: 'cars', is_active: false } },
      ]),
    });
    const { wrapper, replace } = mountC({ isActive: false, query: { open: 'cargo_cars' }, setSelected: false });
    await flushPromises();

    expect(wrapper.vm.showArchive).toBe(true);
    expect(wrapper.vm.selectedTable?.table.name).toBe('cargo_cars');
    // query чистится, чтобы рефреш страницы не залипал на восстановлении
    expect(replace).toHaveBeenCalledWith({ query: {} });
  });
});

/**
 * Один и тот же `?open=` обслуживает два перехода: возврат со страницы версий несёт
 * ИМЯ архивной таблицы, а сквозной поиск - числовой id. Разбирать их одинаково
 * нельзя: по id включённый архив прячет активную таблицу, и открывать нечего.
 */
describe('TableConstructor - параметр open от поиска и от страницы версий', () => {
  it('числовой open не включает архивный режим', async () => {
    const { wrapper } = mountC({ isActive: true, query: { open: '1' }, setSelected: false });
    await flushPromises();

    expect(wrapper.vm.showArchive).toBe(false);
  });

  it('имя таблицы в open по-прежнему открывает архив', async () => {
    const { wrapper } = mountC({ isActive: false, query: { open: 'cargo_cars' }, setSelected: false });
    await flushPromises();

    expect(wrapper.vm.showArchive).toBe(true);
  });

  it('без параметра архив не включается', async () => {
    const { wrapper } = mountC({ isActive: true, query: {}, setSelected: false });
    await flushPromises();

    expect(wrapper.vm.showArchive).toBe(false);
  });
});
