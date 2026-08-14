import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import CarsView from '../CarsView.vue';

// Разбор второго круга замечаний владельца по мобильному интерфейсу (#1097, w8):
// бейдж статуса налезал на пунктирную границу format-col - оба делили строку, но
// выравнивались по одной Y-координате через align-self: flex-start, а границу нёс
// только format-col. Бейдж переехал в подвал карточки, в строку с кнопками
// "Изменить"/"Удалить"; номер и марка теперь делят одну строку (освободившееся место).
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
  apiRequestRaw: vi.fn().mockResolvedValue({
    ok: false,
    json: vi.fn().mockResolvedValue({ success: false, error: 'x' }),
  }),
}));
vi.mock('@/api/blacklist', () => ({ listVehicleBlacklist: vi.fn().mockResolvedValue([]) }));

const getUniqueCarsPaginated = vi.fn();
vi.mock('@/api/cars', () => ({
  getUniqueCarsPaginated: (...args) => getUniqueCarsPaginated(...args),
}));

const stubs = {
  teleport: true,
  SearchComponent: true,
  RefreshButton: true,
  LoaderSpinner: true,
  ConfirmationModal: true,
  VehicleDetailsModal: true,
  ApplicationDetail: true,
  BaseModal: true,
};

function mountView() {
  return mount(CarsView, {
    global: {
      stubs,
      mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } },
    },
  });
}

const SFC = readFileSync(resolve(__dirname, '../CarsView.vue'), 'utf8');

/** Тело правила для селектора, без учёта переносов (первое совпадение в источнике). */
function rule(src, selector) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const found = src.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`));
  return found ? found[1].replace(/\s+/g, ' ').trim() : null;
}

/** Содержимое @media-блока по маркеру начала (со сбалансированным подсчётом скобок -
 *  jsdom не считает медиазапросы вовсе, а rule() без этого нашёл бы ПЕРВОЕ совпадение
 *  селектора в файле, а не мобильный оверрайд. */
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

const MOBILE_768 = mediaBlock(SFC, '@media (max-width: 768px)');
const MOBILE_76798 = mediaBlock(SFC, '@media (max-width: 767.98px)');

let wrapper;

describe('CarsView — карточка машины: подвал не теряет ни бейдж, ни действия', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getUniqueCarsPaginated.mockReset();
    getUniqueCarsPaginated.mockResolvedValue({
      items: [{ id: 1, number: 'А123ВС799', mark: 'Toyota', format_name: 'Российский', status: true }],
      meta: { total: 1, page: 1, per_page: 30 },
    });
  });

  afterEach(() => {
    wrapper?.unmount();
  });

  it('status-col и actions-col остаются прямыми детьми car-row', async () => {
    wrapper = mountView();
    await flushPromises();

    const row = wrapper.find('.car-row');
    expect(row.exists()).toBe(true);
    expect(row.find('.status-col').exists()).toBe(true);
    expect(row.find('.actions-col').exists()).toBe(true);
  });
});

describe('CarsView — CSS-контракт подвала карточки на мобилке (чтение SFC)', () => {
  it('бейдж статуса переехал в подвал: своя граница подвала, без выравнивания по чужому пунктиру', () => {
    const statusRule = rule(MOBILE_76798, '.rt-table .car-row.rt-row > .status-col');
    expect(statusRule, 'правило .status-col не найдено в 767.98-блоке').not.toBeNull();
    expect(statusRule).toMatch(/order:\s*10/);
    expect(statusRule).toMatch(/border-top:\s*1px solid/);
  });

  it('actions-col делит строку с бейджем (не 100% ширины) и идёт следом по order', () => {
    const actionsRule = rule(MOBILE_76798, '.car-row.rt-row > .actions-col');
    expect(actionsRule, 'правило .actions-col не найдено в 767.98-блоке').not.toBeNull();
    expect(actionsRule).toMatch(/order:\s*11/);
    expect(actionsRule).not.toMatch(/width:\s*100%/);
  });

  // Третий круг замечаний владельца (#1097 w9): бейдж и кнопки "Изменить"/"Удалить"
  // стояли на разной высоте, серая линия подвала обрывалась после бейджа. Причина -
  // align-self: center у бейджа против align-self не заданного (стрейч по умолчанию)
  // у actions-col: высоты ячеек по контенту не совпадают ровно, и верхние края (а с
  // ними border-top) расходились. Обе ячейки обязаны стоять на align-self: flex-start -
  // тогда верхние края (и линия подвала) совпадают независимо от разницы высот контента.
  it('бейдж и кнопки подвала прижаты к верхнему краю строки - border-top не переламывается', () => {
    const statusRule = rule(MOBILE_76798, '.rt-table .car-row.rt-row > .status-col');
    const actionsRule = rule(MOBILE_76798, '.car-row.rt-row > .actions-col');
    expect(statusRule).toMatch(/align-self:\s*flex-start/);
    expect(actionsRule).toMatch(/align-self:\s*flex-start/);
  });

  // Четвёртый круг замечаний владельца (#1097 w11): бейдж и кнопки уже выровнены по
  // align-self: flex-start (см. тест выше), но серая линия подвала всё равно рвалась.
  // Причина - border-top рисуют ДВА РАЗНЫХ элемента (.status-col и .actions-col), а
  // между ними column-gap: 8px родителя - пустое место без бордюра, где линия физически
  // прерывается, хотя цвет/толщина границ совпадают. margin-left на actions-col тянет
  // её бокс (а с ним border-top) вплотную к status-col, закрывая зазор.
  it('actions-col компенсирует column-gap родителя отрицательным margin-left - линия подвала не рвётся', () => {
    const parentRule = rule(MOBILE_76798, '.rt-table .car-row.rt-row');
    const actionsRule = rule(MOBILE_76798, '.car-row.rt-row > .actions-col');
    expect(parentRule, 'базовое правило .car-row.rt-row не найдено в 767.98-блоке').not.toBeNull();
    expect(parentRule).toMatch(/column-gap:\s*8px/);
    expect(actionsRule).toMatch(/margin-left:\s*-8px/);
  });

  it('номер и марка делят одну строку карточки (не 100% каждая)', () => {
    const numberRule = rule(MOBILE_76798, '.rt-table .car-row.rt-row > .car-number-col');
    const brandRule = rule(MOBILE_76798, '.rt-table .car-row.rt-row > .brand-col');
    expect(numberRule, 'правило .car-number-col не найдено в 767.98-блоке').not.toBeNull();
    expect(brandRule, 'правило .brand-col не найдено в 767.98-блоке').not.toBeNull();
    expect(numberRule).not.toMatch(/flex:\s*0 0 100%/);
    expect(brandRule).not.toMatch(/flex:\s*0 0 100%/);
    // brand-col делит строку с car-number-col, а не начинает свою - пунктир ей не нужен.
    expect(brandRule).toMatch(/border-top:\s*none/);
  });
});

describe('CarsView — отступы модалки добавления машины и подписи привязки', () => {
  it('data__completion несёт реальный padding (BaseModal.base-modal__body идёт без своих отступов)', () => {
    expect(rule(SFC, '.data__completion')).toMatch(/padding:\s*14px 20px 18px/);
  });

  it('на телефоне подписи чекбоксов привязки крупнее и тач-таргет строки не мельче 36px', () => {
    const bindingRule = rule(MOBILE_768, '.binding-option');
    expect(bindingRule, 'мобильный оверрайд .binding-option не найден').not.toBeNull();
    expect(bindingRule).toMatch(/font-size:\s*14px/);
    expect(bindingRule).toMatch(/min-height:\s*36px/);

    const checkboxRule = rule(MOBILE_768, '.binding-option input[type="checkbox"]');
    expect(checkboxRule, 'мобильный оверрайд чекбокса привязки не найден').not.toBeNull();
    expect(checkboxRule).toMatch(/width:\s*18px/);
    expect(checkboxRule).toMatch(/height:\s*18px/);
  });
});
