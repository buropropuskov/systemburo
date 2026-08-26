import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import CarsView from '../CarsView.vue';

// #1097 w11: у сотрудников привязка к организации и к компании независимы (обе
// галки можно включить сразу), а у машин было явное ограничение "или-или" -
// :disabled на чекбоксах + watch, сбрасывающий второй при включении первого.
// Владелец попросил снять ограничение, чтобы машина могла быть привязана и к
// организации, и к компании одновременно, как у сотрудников. Бэкенд (models/car.go,
// unique_car_service.go, миграции) никакой XOR-валидации не несёт - ограничение было
// чисто фронтовым.
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
  apiRequestRaw: vi.fn().mockResolvedValue({
    ok: false,
    json: vi.fn().mockResolvedValue({ success: false, error: 'x' }),
  }),
}));
vi.mock('@/api/blacklist', () => ({ listVehicleBlacklist: vi.fn().mockResolvedValue([]) }));

const stubs = {
  teleport: true,
  SearchComponent: true,
  RefreshButton: true,
  LoaderSpinner: true,
  StatusBadge: true,
  ConfirmationModal: true,
  VehicleDetailsModal: true,
  ApplicationDetail: true,
};

function mountView() {
  return mount(CarsView, {
    global: {
      stubs,
      mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } },
    },
  });
}

let wrapper;

describe('CarsView — привязка машины к организации и компании независима', () => {
  beforeEach(async () => {
    setActivePinia(createPinia());
    wrapper = mountView();
    await flushPromises();

    wrapper.vm.ownershipInfo = {
      has_organization: true,
      has_company: true,
      user_id: 1,
      organization_id: 10,
      organization_name: 'ООО Ромашка',
      company_id: 20,
      company_name: 'ИП Иванов',
    };
    wrapper.vm.showAddCarModal();
    await wrapper.vm.$nextTick();
  });

  afterEach(() => {
    wrapper?.unmount();
  });

  function checkboxes() {
    const inputs = wrapper.findAll('.binding-option input[type="checkbox"]');
    expect(inputs).toHaveLength(2);
    return { orgBox: inputs[0], companyBox: inputs[1] };
  }

  it('чекбоксы не несут disabled - ни один не блокирует другой', () => {
    const { orgBox, companyBox } = checkboxes();
    expect(orgBox.attributes('disabled')).toBeUndefined();
    expect(companyBox.attributes('disabled')).toBeUndefined();
  });

  it('включение организации не сбрасывает уже включённую компанию', async () => {
    const { orgBox, companyBox } = checkboxes();

    await companyBox.setValue(true);
    expect(wrapper.vm.bindToCompany).toBe(true);

    await orgBox.setValue(true);
    // Раньше здесь стоял watch(bindToOrganization), гасивший bindToCompany.
    expect(wrapper.vm.bindToOrganization).toBe(true);
    expect(wrapper.vm.bindToCompany).toBe(true);
  });

  it('включение компании не сбрасывает уже включённую организацию', async () => {
    const { orgBox, companyBox } = checkboxes();

    await orgBox.setValue(true);
    expect(wrapper.vm.bindToOrganization).toBe(true);

    await companyBox.setValue(true);
    // Раньше здесь стоял watch(bindToCompany), гасивший bindToOrganization.
    expect(wrapper.vm.bindToOrganization).toBe(true);
    expect(wrapper.vm.bindToCompany).toBe(true);
  });

  it('обе галки одновременно уходят в payload сохранения (organization_id и company_id заполнены)', async () => {
    const { orgBox, companyBox } = checkboxes();
    await orgBox.setValue(true);
    await companyBox.setValue(true);

    expect(wrapper.vm.bindToOrganization ? wrapper.vm.ownershipInfo.organization_id : null).toBe(10);
    expect(wrapper.vm.bindToCompany ? wrapper.vm.ownershipInfo.company_id : null).toBe(20);
  });
});
