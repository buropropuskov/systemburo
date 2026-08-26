import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import EmployeeView from '../EmployeeView.vue';

// Разбор второго круга замечаний владельца по мобильному интерфейсу (#1097, w8):
// бейдж статуса налезал на пунктирную границу position-col - оба делили строку, но
// выравнивались по одной Y-координате через align-self: flex-start, а границу нёс
// только position-col. Бейдж переехал в подвал карточки, в строку с кнопками
// "Изменить"/"Удалить" (тот же фикс, что у CarsView).
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
  apiRequestRaw: vi.fn().mockResolvedValue({
    ok: false,
    json: vi.fn().mockResolvedValue({ success: false, error: 'x' }),
  }),
}));
vi.mock('@/api/blacklist', () => ({ listPersonBlacklist: vi.fn().mockResolvedValue([]) }));

const getUniqueEmployeesPaginated = vi.fn();
vi.mock('@/api/employees', () => ({
  getUniqueEmployeesPaginated: (...args) => getUniqueEmployeesPaginated(...args),
}));

const stubs = {
  teleport: true,
  SearchComponent: true,
  RefreshButton: true,
  LoaderSpinner: true,
  EmployeeEditModal: true,
  EmployeeDetailsModal: true,
  ApplicationDetail: true,
};

function mountView() {
  return mount(EmployeeView, {
    global: {
      stubs,
      mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } },
    },
  });
}

const SFC = readFileSync(resolve(__dirname, '../EmployeeView.vue'), 'utf8');

/** Тело правила для селектора, без учёта переносов (первое совпадение в источнике). */
function rule(src, selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const found = src.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`));
  return found ? found[1].replace(/\s+/g, ' ').trim() : null;
}

/** Содержимое @media-блока по маркеру начала (со сбалансированным подсчётом скобок). */
function mediaBlock(src, marker) {
  const start = src.indexOf(marker);
  if (start === -1) return null;
  let i = src.indexOf('{', start) + 1;
  let depth = 1;
  const bodyStart = i;
  while (depth > 0 && i < src.length) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}') depth--;
    i++;
  }
  return src.slice(bodyStart, i - 1);
}

const MOBILE_76798 = mediaBlock(SFC, '@media (max-width: 767.98px)');

let wrapper;

describe('EmployeeView — карточка сотрудника: подвал не теряет ни бейдж, ни действия', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getUniqueEmployeesPaginated.mockReset();
    getUniqueEmployeesPaginated.mockResolvedValue({
      items: [{ id: 1, last_name: 'Иванов', first_name: 'Иван', position: 'Монтажник', status: true }],
      meta: { total: 1, page: 1, per_page: 30 },
    });
  });

  afterEach(() => {
    wrapper?.unmount();
  });

  it('status-col и actions-col остаются прямыми детьми employee-row', async () => {
    wrapper = mountView();
    await flushPromises();

    const row = wrapper.find('.employee-row');
    expect(row.exists()).toBe(true);
    expect(row.find('.status-col').exists()).toBe(true);
    expect(row.find('.actions-col').exists()).toBe(true);
  });
});

describe('EmployeeView — CSS-контракт подвала карточки на мобилке (чтение SFC)', () => {
  it('бейдж статуса переехал в подвал: своя граница подвала, без выравнивания по чужому пунктиру', () => {
    const statusRule = rule(MOBILE_76798, '.rt-table .employee-row.rt-row > .status-col');
    expect(statusRule, 'правило .status-col не найдено в 767.98-блоке').not.toBeNull();
    expect(statusRule).toMatch(/order:\s*10/);
    expect(statusRule).toMatch(/border-top:\s*1px solid/);
  });

  it('actions-col делит строку с бейджем (не 100% ширины) и идёт следом по order', () => {
    const actionsRule = rule(MOBILE_76798, '.employee-row.rt-row > .actions-col');
    expect(actionsRule, 'правило .actions-col не найдено в 767.98-блоке').not.toBeNull();
    expect(actionsRule).toMatch(/order:\s*11/);
    expect(actionsRule).not.toMatch(/width:\s*100%/);
  });

  // Третий круг замечаний владельца (#1097 w9): бейдж и кнопки "Изменить"/"Удалить"
  // стояли на разной высоте, серая линия подвала обрывалась после бейджа - то же самое,
  // что чинили в CarsView. align-self: flex-start на ОБЕИХ ячейках прижимает их к
  // верхнему краю строки, border-top совпадает независимо от разницы высот контента.
  it('бейдж и кнопки подвала прижаты к верхнему краю строки - border-top не переламывается', () => {
    const statusRule = rule(MOBILE_76798, '.rt-table .employee-row.rt-row > .status-col');
    const actionsRule = rule(MOBILE_76798, '.employee-row.rt-row > .actions-col');
    expect(statusRule).toMatch(/align-self:\s*flex-start/);
    expect(actionsRule).toMatch(/align-self:\s*flex-start/);
  });

  // Четвёртый круг замечаний владельца (#1097 w11): тот же разрыв линии подвала, что
  // чинили в CarsView. border-top рисуют ДВА РАЗНЫХ элемента (.status-col и
  // .actions-col), а между ними column-gap: 8px родителя - пустое место без бордюра,
  // где линия физически прерывается, хотя цвет/толщина границ совпадают. margin-left
  // на actions-col тянет её бокс (а с ним border-top) вплотную к status-col.
  it('actions-col компенсирует column-gap родителя отрицательным margin-left - линия подвала не рвётся', () => {
    const parentRule = rule(MOBILE_76798, '.rt-table .employee-row.rt-row');
    const actionsRule = rule(MOBILE_76798, '.employee-row.rt-row > .actions-col');
    expect(parentRule, 'базовое правило .employee-row.rt-row не найдено в 767.98-блоке').not.toBeNull();
    expect(parentRule).toMatch(/column-gap:\s*8px/);
    expect(actionsRule).toMatch(/margin-left:\s*-8px/);
  });

  it('должность больше не делит базис с бейджем - у неё нет собственного правила флекс-базиса 0', () => {
    // position-col раньше несла flex: 1 1 0, чтобы уступить место бейджу в общей
    // строке; теперь бейдж в подвале, и должность занимает строку по умолчанию (100%).
    const positionRule = rule(MOBILE_76798, '.rt-table .employee-row.rt-row > .position-col');
    expect(positionRule).toBeNull();
  });
});
