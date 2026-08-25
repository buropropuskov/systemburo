import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import EmployeeView from '../EmployeeView.vue';
import { usePermissionsStore } from '@/stores/permissions';

// View на mounted дёргает реестр/ownership/ЧС - глушим API; проверяем только
// гейтинг вкладок реестра (section.registry.*) и кнопок изменения (entity.employees.write).
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
  apiRequestRaw: vi.fn().mockResolvedValue({
    ok: false,
    json: vi.fn().mockResolvedValue({ success: false, error: 'x' }),
  }),
}));
vi.mock('@/api/blacklist', () => ({ listPersonBlacklist: vi.fn().mockResolvedValue([]) }));

const stubs = {
  teleport: true,
  SearchComponent: true,
  RefreshButton: true,
  LoaderSpinner: true,
  StatusBadge: true,
  EmployeeEditModal: true,
  EmployeeDetailsModal: true,
  ApplicationDetail: true,
};

function seedPerms({ mode = 'normal', allow = [] } = {}) {
  const perms = usePermissionsStore();
  perms.mode = mode;
  perms.effective = Object.fromEntries(allow.map(k => [k, { value: 'allow', source: 'role' }]));
}

function mountView() {
  return mount(EmployeeView, {
    global: { stubs, mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } } },
  });
}

/** Открывает реестр с владельческой строкой (минуя API). */
async function withOwnedRow(w, filter = 'user') {
  w.vm.loading = false;
  w.vm.currentFilter = filter;
  w.vm.ownershipInfo = {
    has_organization: true,
    has_company: true,
    user_id: 1,
    organization_id: 10,
    company_id: 20,
  };
  w.vm.employeesData = [{
    id: 1, last_name: 'Иванов', first_name: 'Иван', position: 'Слесарь',
    status: true, user_id: 1, organization_id: 10, company_id: 20,
  }];
  await w.vm.$nextTick();
}

const orgTab = w => w.find('[data-testid="filter-tab-organization"]');
const companyTab = w => w.find('[data-testid="filter-tab-company"]');
const allSystemTab = w => w.find('[data-testid="filter-tab-all-system"]');
const addBtn = w => w.find('[data-testid="ob-employees-add-button"]');
const editBtn = w => w.find('.edit-btn');
const deleteBtn = w => w.find('.delete-btn');
const readOnly = w => w.find('.read-only-text');

let wrapper;

describe('EmployeeView — гейтинг реестра по правам (срез 6)', () => {
  beforeEach(() => setActivePinia(createPinia()));
  afterEach(() => wrapper?.unmount());

  describe('Вкладки разделов реестра (section.registry.*)', () => {
    it('вкладка организации видна при section.registry.organization', async () => {
      seedPerms({ allow: ['section.registry.organization'] });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(orgTab(wrapper).exists()).toBe(true);
    });

    it('вкладка организации скрыта без права (хотя ownershipInfo.has_organization=true)', async () => {
      seedPerms({ allow: [] });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(orgTab(wrapper).exists()).toBe(false);
    });

    it('вкладка компании видна при section.registry.company', async () => {
      seedPerms({ allow: ['section.registry.company'] });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(companyTab(wrapper).exists()).toBe(true);
    });

    it('вкладка компании скрыта без права', async () => {
      seedPerms({ allow: [] });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(companyTab(wrapper).exists()).toBe(false);
    });

    it('вкладка «Все системы» скрыта у обычного юзера без section.registry.all_system', async () => {
      seedPerms({ allow: ['section.registry.organization'] });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(allSystemTab(wrapper).exists()).toBe(false);
    });

    it('вкладка «Все системы» видна при section.registry.all_system', async () => {
      seedPerms({ allow: ['section.registry.all_system'] });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(allSystemTab(wrapper).exists()).toBe(true);
    });

    it('супер-админ видит все вкладки', async () => {
      seedPerms({ mode: 'super' });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(orgTab(wrapper).exists()).toBe(true);
      expect(companyTab(wrapper).exists()).toBe(true);
      expect(allSystemTab(wrapper).exists()).toBe(true);
    });
  });

  describe('Кнопки изменения реестра (entity.employees.write)', () => {
    it('кнопка «Добавить» видна при entity.employees.write', async () => {
      seedPerms({ allow: ['entity.employees.write'] });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(addBtn(wrapper).exists()).toBe(true);
    });

    it('кнопка «Добавить» скрыта без entity.employees.write', async () => {
      seedPerms({ allow: ['section.registry.organization'] });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(addBtn(wrapper).exists()).toBe(false);
    });

    it('кнопка «Редактировать» владельческой строки видна при entity.employees.write', async () => {
      seedPerms({ allow: ['entity.employees.write'] });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(editBtn(wrapper).exists()).toBe(true);
    });

    it('кнопка «Редактировать» скрыта без entity.employees.write (даже для владельца)', async () => {
      seedPerms({ allow: [] });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(editBtn(wrapper).exists()).toBe(false);
    });
  });

  describe('Кнопка удаления реестра (entity.employees.delete)', () => {
    it('кнопка «Удалить» владельческой строки видна при entity.employees.delete', async () => {
      seedPerms({ allow: ['entity.employees.delete'] });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(deleteBtn(wrapper).exists()).toBe(true);
    });

    it('кнопка «Удалить» скрыта без entity.employees.delete (даже для владельца)', async () => {
      seedPerms({ allow: ['entity.employees.write'] });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(deleteBtn(wrapper).exists()).toBe(false);
    });

    it('супер-админ видит кнопку «Удалить»', async () => {
      seedPerms({ mode: 'super' });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(deleteBtn(wrapper).exists()).toBe(true);
    });
  });

  describe('Плашка «Только просмотр» (консистентность при отзыве write/delete)', () => {
    it('плашка скрыта, пока доступно изменение (write есть, delete нет)', async () => {
      seedPerms({ allow: ['entity.employees.write'] });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(readOnly(wrapper).exists()).toBe(false);
    });

    it('плашка скрыта, пока доступно удаление (delete есть, write нет)', async () => {
      seedPerms({ allow: ['entity.employees.delete'] });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(readOnly(wrapper).exists()).toBe(false);
    });

    it('плашка видна у владельца, когда отозваны и write, и delete', async () => {
      seedPerms({ allow: ['section.registry.organization'] });
      wrapper = mountView();
      await withOwnedRow(wrapper);
      expect(editBtn(wrapper).exists()).toBe(false);
      expect(deleteBtn(wrapper).exists()).toBe(false);
      expect(readOnly(wrapper).exists()).toBe(true);
    });

    it('плашка видна на чужой строке (all_system), даже при наличии прав', async () => {
      seedPerms({ allow: ['entity.employees.write', 'entity.employees.delete', 'section.registry.all_system'] });
      wrapper = mountView();
      await withOwnedRow(wrapper, 'all_system');
      expect(readOnly(wrapper).exists()).toBe(true);
    });
  });
});
