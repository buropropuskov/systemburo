import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CarsView from '../CarsView.vue';
import EmployeeView from '../EmployeeView.vue';
import { usePermissionsStore } from '@/stores/permissions';

const requests = vi.hoisted(() => ({ cars: vi.fn(), employees: vi.fn() }));
vi.mock('@/api/client', () => ({ apiRequest: vi.fn(async () => ({ ok: false, json: async () => [] })), apiRequestRaw: vi.fn() }));
vi.mock('@/api/cars', () => ({ getUniqueCarsPaginated: (...args) => requests.cars(...args) }));
vi.mock('@/api/employees', () => ({ getUniqueEmployeesPaginated: (...args) => requests.employees(...args) }));
vi.mock('@/api/blacklist', () => ({ listVehicleBlacklist: vi.fn(async () => []), listPersonBlacklist: vi.fn(async () => []) }));
vi.mock('@/utils/viewportScale', () => ({ getViewportZoom: () => 1 }));

const observers = [];
let wrapper;
const originalWidth = window.innerWidth;
const originalHeight = window.innerHeight;

function resize(width) {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: width });
  Object.defineProperty(window, 'innerHeight', { configurable: true, value: 900 });
  window.dispatchEvent(new Event('resize'));
}

beforeEach(() => {
  setActivePinia(createPinia());
  usePermissionsStore().$patch({ mode: 'super', loaded: true });
  requests.cars.mockReset().mockResolvedValue({ items: [{ id: 1, license_plate: 'А123АА 77', mark: 'Учебная машина', status: true }], meta: { total: 60 } });
  requests.employees.mockReset().mockResolvedValue({ items: [{ id: 1, last_name: 'Учебный', first_name: 'Сотрудник', status: true }], meta: { total: 60 } });
  observers.length = 0;
  vi.stubGlobal('IntersectionObserver', class {
    constructor(callback, options) { this.options = options; this.disconnect = vi.fn(); observers.push(this); }
    observe() {}
    unobserve() {}
  });
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  vi.unstubAllGlobals();
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: originalWidth });
  Object.defineProperty(window, 'innerHeight', { configurable: true, value: originalHeight });
});

for (const [name, component, bodyRef, request] of [
  ['cars', CarsView, 'carsBody', 'cars'],
  ['employees', EmployeeView, 'employeesBody', 'employees'],
]) {
  async function open(width) {
    resize(width);
    wrapper = mount(component, { global: {
      stubs: { teleport: true, PassageRegistrySummary: { template: '<header><slot /></header>' }, SearchComponent: true, RefreshButton: true, LoaderSpinner: true, StatusBadge: true, ConfirmationModal: true, VehicleDetailsModal: true, EmployeeDetailsModal: true, ApplicationDetail: true },
      mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn(async () => {}) } },
    } });
    await flushPromises();
  }

  describe(`${name}: tablet scroll and pagination root`, () => {
    for (const width of [768, 1024, 1366, 1440]) {
      it(`${width}px uses the matching scroll container`, async () => {
        await open(width);
        expect(wrapper.element.style.height).toBe(width <= 1366 ? '' : '900px');
        expect(observers.at(-1).options.root).toBe(width <= 1366 ? null : wrapper.vm.$refs[bodyRef]);
        expect(requests[request]).toHaveBeenCalledTimes(1);
      });
    }
    it('reconnects pagination on tablet/desktop resize without fetching the next page', async () => {
      await open(1024);
      const initial = observers.at(-1);
      resize(1440);
      await flushPromises();
      expect(initial.disconnect).toHaveBeenCalled();
      expect(observers.at(-1).options.root).toBe(wrapper.vm.$refs[bodyRef]);
      expect(wrapper.element.style.height).toBe('900px');
      resize(1024);
      await flushPromises();
      expect(observers.at(-1).options.root).toBe(null);
      expect(wrapper.element.style.height).toBe('');
      expect(requests[request]).toHaveBeenCalledTimes(1);
    });
  });
}
