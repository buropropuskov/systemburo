import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

import ApplicationAttachmentDetail from '../ApplicationAttachmentDetail.vue';

vi.mock('@/api/applicationAssignments', () => ({
  assignElementTables: vi.fn().mockResolvedValue({}),
  assignCarUnloadPlaces: vi.fn().mockResolvedValue({}),
}));

const origMatchMedia = window.matchMedia;
const origResizeObserver = global.ResizeObserver;

/** Телефон: media-запрос компонента (767.98) отвечает «да». */
function mockNarrowViewport(matches) {
  window.matchMedia = vi.fn().mockImplementation((query) => ({
    matches,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

/**
 * Ширина блока приходит наблюдателем, как в браузере. Отдаём её микрозадачей:
 * синхронный колбэк затёрся бы замером clientWidth сразу после `observe`, а в
 * jsdom он всегда 0.
 */
function mockContainerWidth(width) {
  global.ResizeObserver = class {
    constructor(callback) {
      this.callback = callback;
    }

    observe() {
      Promise.resolve().then(() => this.callback([{ contentRect: { width } }]));
    }

    disconnect() {}
  };
}

function car(over = {}) {
  return {
    id: 1,
    car_number: 'К 050 УА 902',
    car_brand: 'BMW X5',
    unload_places: [{ id: 5, name: 'Склад №1' }],
    target_tables: [{ id: 7, display_name: 'ПОСТ №72' }],
    ...over,
  };
}

/** Ждём кадр после mount: режим ставится в `mounted`, до этого DOM ещё десктопный. */
async function mountDetail(props) {
  const wrapper = mount(ApplicationAttachmentDetail, {
    props: { applicationId: 42, ...props },
    global: { stubs: { ApplicationAssignModal: true, teleport: true } },
  });
  await flushPromises();
  return wrapper;
}

function mountCars(cars = [car()], props = {}) {
  return mountDetail({
    attachment: { id: 1, attachment_type: 'cars', attachment_display_name: 'Машины' },
    cars,
    ...props,
  });
}

describe('ApplicationAttachmentDetail — карточка вложения на телефоне (#1097)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    mockNarrowViewport(true);
    mockContainerWidth(300);
  });

  afterEach(() => {
    window.matchMedia = origMatchMedia;
    global.ResizeObserver = origResizeObserver;
  });

  it('гос. номер и марка идут одной строкой, а не двумя полями карточки', async () => {
    const wrapper = await mountCars();

    // отдельной колонки под марку в карточке нет
    expect(wrapper.find('.c-sub').exists()).toBe(false);

    const key = wrapper.find('.el-cell--key');
    expect(key.find('[data-testid="attachment-element-key"]').text()).toBe('К 050 УА 902');
    expect(key.find('.val-sub').text()).toBe('BMW X5');
  });

  it('без марки подстрока не рисуется: прочерк в карточке - мусор', async () => {
    const wrapper = await mountCars([car({ car_brand: '' })]);
    expect(wrapper.find('.el-cell--key .val-sub').exists()).toBe(false);
  });

  it('у сотрудника той же строкой стоит должность', async () => {
    const wrapper = await mountDetail({
      attachment: { id: 2, attachment_type: 'people', attachment_display_name: 'Люди' },
      employees: [{ id: 3, last_name: 'Иванов', first_name: 'Иван', position: 'Водитель', target_tables: [] }],
    });

    expect(wrapper.find('.el-cell--key .val-sub').text()).toBe('Водитель');
  });

  it('подвал несёт одну кнопку «Назначить всем…» вместо двух длинных подписей', async () => {
    const wrapper = await mountCars([car()], { canAssign: true });

    expect(wrapper.find('[data-testid="attachment-assign-all-open"]').exists()).toBe(true);
    // подписи колонок в подвале не помещались - они уехали в лист
    expect(wrapper.find('.el-foot__bulk-label').exists()).toBe(false);
    expect(wrapper.find('[data-testid="attachment-assign-all-places"]').exists()).toBe(false);
  });

  it('кнопка раскрывает лист с полными подписями действий', async () => {
    const wrapper = await mountCars([car()], { canAssign: true });
    await wrapper.find('[data-testid="attachment-assign-all-open"]').trigger('click');

    const items = wrapper.findAll('.bulk-sheet__item');
    expect(items.map((item) => item.text())).toEqual(['Места разгрузки', 'Посты проезда']);
  });

  it('выбор в листе закрывает лист и открывает назначение по всем строкам', async () => {
    const wrapper = await mountCars([car({ id: 1 }), car({ id: 2 })], { canAssign: true });
    await wrapper.find('[data-testid="attachment-assign-all-open"]').trigger('click');
    await wrapper.find('[data-testid="attachment-assign-all-tables"]').trigger('click');

    expect(wrapper.vm.bulkSheetOpen).toBe(false);
    expect(wrapper.vm.assign.open).toBe(true);
    expect(wrapper.vm.assign.kind).toBe('tables');
    expect(wrapper.vm.assign.elementIds).toEqual([1, 2]);
  });

  it('длинный заголовок блока сокращается: он делит строку с полем поиска', async () => {
    const wrapper = await mountDetail({
      attachment: { id: 3, attachment_type: 'items', attachment_display_name: 'Имущество' },
      items: [{ id: 4, name: 'Ноутбук', count: 1 }],
    });

    expect(wrapper.find('.el-section__head h5').text()).toBe('ТМЦ');
  });
});

describe('ApplicationAttachmentDetail — на широком экране подвал прежний', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    mockNarrowViewport(false);
    mockContainerWidth(1200);
  });

  afterEach(() => {
    window.matchMedia = origMatchMedia;
    global.ResizeObserver = origResizeObserver;
  });

  it('обе кнопки назначения стоят в подвале, свёрнутой кнопки нет', async () => {
    const wrapper = await mountCars([car()], { canAssign: true });

    expect(wrapper.find('[data-testid="attachment-assign-all-places"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="attachment-assign-all-tables"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="attachment-assign-all-open"]').exists()).toBe(false);
  });

  it('марка остаётся своей колонкой, а заголовок - полным', async () => {
    const wrapper = await mountCars();

    expect(wrapper.find('.c-sub').exists()).toBe(true);
    expect(wrapper.find('.el-section__head h5').text()).toBe('Автомобили');
  });
});
