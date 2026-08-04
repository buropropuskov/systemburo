import { describe, it, expect, vi } from 'vitest';
import { mount, shallowMount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false, json: async () => [] }) }));
vi.mock('@/api/applications', () => ({ createSupplement: vi.fn() }));
vi.mock('@/api/attachment-templates', () => ({ getFieldConfig: vi.fn().mockResolvedValue({ base: [] }) }));

import ExistingCarsModal from '../ExistingCarsModal.vue';
import ExistingEmployeesModal from '../ExistingEmployeesModal.vue';
import SupplementModal from '../SupplementModal.vue';

/**
 * Окно дополнения открывается ПОВЕРХ панели детали заявки (10002), а поиск существующих
 * машин/людей открывается уже из него. У этих модалок слой зашит в scoped-CSS (1000) -
 * без подъёма пропом они уезжают ЗА собственного родителя, и человек видит пустое
 * затемнение вместо списка. jsdom этого не показывает, поэтому держим замок на цепочке.
 */

// teleport стабим везде: и окна поиска, и само окно дополнения рендерят содержимое
// в body, откуда обёртка теста его не видит.
const stubs = { teleport: true, SearchComponent: true, LoaderSpinner: true };

const CARS_ATTACHMENT = {
  id: 12,
  attachment_type: 'cars',
  attachment_name: 'Автотранспорт',
  unique_attachment_id: 101,
  entry_date_to: '2099-12-31',
};

const PEOPLE_ATTACHMENT = {
  id: 13,
  attachment_type: 'people',
  attachment_name: 'Сотрудники',
  unique_attachment_id: 102,
  entry_date_to: '2099-12-31',
};

function overlayZIndex(wrapper) {
  return Number(wrapper.find('.modal-overlay').attributes('style').match(/z-index:\s*(\d+)/)[1]);
}

function supplementOverlayLayer() {
  const source = readFileSync(resolve(__dirname, '../SupplementModal.vue'), 'utf8');
  return Number(source.match(/\.supp-overlay\s*\{[^}]*z-index:\s*(\d+)/)[1]);
}

async function mountSupplement(attachments) {
  setActivePinia(createPinia());
  const wrapper = shallowMount(SupplementModal, {
    props: { show: true, application: { id: 1, application_number: 'A-1' }, attachments },
    global: { stubs: { teleport: true } },
  });
  await wrapper.vm.$nextTick();
  return wrapper;
}

describe('Слой окна поиска существующих (#1685)', () => {
  it('ExistingCarsModal кладёт слой из пропа на оверлей', () => {
    const wrapper = mount(ExistingCarsModal, {
      props: { visible: true, zIndex: 10012 },
      global: { stubs },
    });
    expect(overlayZIndex(wrapper)).toBe(10012);
  });

  it('ExistingEmployeesModal кладёт слой из пропа на оверлей', () => {
    const wrapper = mount(ExistingEmployeesModal, {
      props: { visible: true, zIndex: 10012 },
      global: { stubs },
    });
    expect(overlayZIndex(wrapper)).toBe(10012);
  });

  it('дефолт остаётся 1000 - подача заявки не задета', () => {
    const cars = mount(ExistingCarsModal, { props: { visible: true }, global: { stubs } });
    const people = mount(ExistingEmployeesModal, { props: { visible: true }, global: { stubs } });
    expect(overlayZIndex(cars)).toBe(1000);
    expect(overlayZIndex(people)).toBe(1000);
  });

  it('VehicleForm и EmployeeForm получают слой выше оверлея самого окна', async () => {
    const layer = supplementOverlayLayer();

    const cars = await mountSupplement([CARS_ATTACHMENT]);
    const vehicleLayer = cars.findComponent({ name: 'VehicleForm' }).props('existingModalZIndex');
    expect(vehicleLayer).toBeGreaterThan(layer);

    const people = await mountSupplement([PEOPLE_ATTACHMENT]);
    const employeeLayer = people.findComponent({ name: 'EmployeeForm' }).props('existingModalZIndex');
    expect(employeeLayer).toBeGreaterThan(layer);
  });

  it('меню дропдауна вложений тоже выше оверлея окна', async () => {
    const layer = supplementOverlayLayer();
    const wrapper = await mountSupplement([CARS_ATTACHMENT]);
    const dropdown = wrapper.findComponent({ name: 'BaseDropdown' });
    expect(dropdown.props('teleport')).toBe(true);
    expect(dropdown.props('menuZIndex')).toBeGreaterThan(layer);
  });

  it('оверлей окна выше панели детали заявки, из которой оно открывается', () => {
    const detail = readFileSync(
      resolve(__dirname, '../../ApplicationDetail/ApplicationDetail.vue'),
      'utf8',
    );
    const detailLayer = Number(detail.match(/\.application-detail-overlay\s*\{[^}]*z-index:\s*(\d+)/)[1]);
    expect(supplementOverlayLayer()).toBeGreaterThan(detailLayer);
  });
});
