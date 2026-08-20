import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { reactive } from 'vue';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createPinia, setActivePinia } from 'pinia';

// Контролы раздела мониторинга (#2125, S8): списки отбора, период и кнопки взяты
// общие - BaseDropdown, DateFilter, .lk-button. До этого экран держал свои
// нативные select и input[type=date], которые в тёмной теме оставались белыми.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  apiRequestRaw: vi.fn(),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify: vi.fn() }),
}));

import RequestsView from '@/views/RequestsView.vue';
import DateFilter from '@/components/DateFilter.vue';
import { apiRequest, apiRequestRaw } from '@/api/client';

const SOURCE = readFileSync(resolve(__dirname, '../RequestsView.vue'), 'utf8');
const TEMPLATE = SOURCE.slice(0, SOURCE.indexOf('</template>'));
const STYLE = SOURCE.slice(SOURCE.indexOf('<style'));

const stubs = {
  AdminPageShell: { template: '<div><slot /></div>' },
  RefreshButton: { template: '<button class="refresh-stub" />' },
  SearchComponent: { template: '<input class="search-stub" />' },
  RealTimeChart: { template: '<div class="chart-stub" />' },
  LoaderSpinner: { template: '<div class="loader-stub" />' },
  AppIcon: { template: '<i />' },
  ToggleSwitch: { template: '<label><slot /></label>' },
};

function logsPage(data = []) {
  return {
    ok: true,
    json: () => Promise.resolve({ success: true, data, meta: { total: 0, page: 1, per_page: 20 } }),
  };
}

/** Адреса, с которыми компонент сходил за списком журнала. */
function journalCalls() {
  return apiRequestRaw.mock.calls.map(([url]) => url);
}

const mounted = [];

function mountView(query = {}) {
  const route = reactive({ query: { ...query } });
  const replace = vi.fn(({ query: next }) => {
    route.query = { ...next };
    return Promise.resolve();
  });
  const wrapper = mount(RequestsView, {
    global: { stubs, mocks: { $route: route, $router: { replace } } },
  });
  mounted.push(wrapper);
  return { wrapper, route };
}

/** Выбирает пункт выпадающего списка по подписи. */
async function pickOption(dropdown, label) {
  await dropdown.get('.base-dropdown__button').trigger('click');
  const item = dropdown.findAll('.base-dropdown__item').find(o => o.text() === label);
  expect(item, `в списке есть пункт «${label}»`).toBeTruthy();
  await item.trigger('click');
  await flushPromises();
}

afterEach(() => {
  mounted.splice(0).forEach(wrapper => wrapper.unmount());
});

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  apiRequest.mockResolvedValue({ ok: true, json: () => Promise.resolve([]) });
  apiRequestRaw.mockResolvedValue(logsPage());
});

describe('Мониторинг запросов, контролы отбора', () => {
  it('нативных select и полей даты на экране не осталось', () => {
    expect(TEMPLATE).not.toContain('<select');
    expect(TEMPLATE).not.toContain('type="date"');
    // Нативную отрисовку темой не покрасить, поэтому проверка стоит замком:
    // вернувшийся <select> в тёмной теме снова будет белым.
    expect(TEMPLATE).toContain('<BaseDropdown');
    expect(TEMPLATE).toContain('<DateFilter');
  });

  it('выбор метода в списке перезапрашивает журнал с этим методом', async () => {
    const { wrapper } = mountView({ page: '3' });
    await flushPromises();
    apiRequestRaw.mockClear();

    await pickOption(wrapper.findAllComponents({ name: 'BaseDropdown' })[1], 'POST');

    const last = journalCalls().at(-1);
    expect(last).toContain('method=POST');
    expect(last).toContain('page=1');
  });

  it('пользователи журнала попадают в список отбора под своими логинами', async () => {
    apiRequest.mockImplementation((path) => {
      if (path === '/request-logs/users') {
        return Promise.resolve({ ok: true, json: () => Promise.resolve([{ id: 42, username: 'ivanov' }]) });
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve([]) });
    });
    const { wrapper } = mountView();
    await flushPromises();
    apiRequestRaw.mockClear();

    await pickOption(wrapper.findAllComponents({ name: 'BaseDropdown' })[3], '@ivanov');
    expect(journalCalls().at(-1)).toContain('user_id=42');
  });

  it('период из календаря уходит в запрос днями и снимает быстрый отбор «последний час»', async () => {
    const { wrapper, route } = mountView({ since: '2026-08-19T09:00:00.000Z' });
    await flushPromises();
    apiRequestRaw.mockClear();

    const calendar = wrapper.findAllComponents(DateFilter)[0];
    calendar.vm.$emit('update:dateRangeStart', new Date(2026, 7, 1));
    calendar.vm.$emit('update:dateRangeEnd', new Date(2026, 7, 20, 23, 59, 59));
    calendar.vm.$emit('apply');
    await flushPromises();

    const last = journalCalls().at(-1);
    // День берётся по локальным частям: toISOString увёл бы 1 августа на 31 июля.
    expect(last).toContain('from_date=2026-08-01');
    expect(last).toContain('to_date=2026-08-20');
    expect(route.query.since).toBeUndefined();
  });

  it('сброс отбора очищает и поле периода', async () => {
    const { wrapper } = mountView({ from: '2026-08-01', to: '2026-08-20' });
    await flushPromises();

    const calendar = wrapper.findAllComponents(DateFilter)[0];
    expect(calendar.props('dateRangeStart')).toBeInstanceOf(Date);

    await wrapper.get('[data-testid="journal-clear"]').trigger('click');
    await flushPromises();

    expect(calendar.props('dateRangeStart')).toBeNull();
    expect(calendar.props('dateRangeEnd')).toBeNull();
  });

  it('размер страницы меняется через общий список', async () => {
    const { wrapper } = mountView();
    await flushPromises();
    apiRequestRaw.mockClear();

    const pageSize = wrapper.findAllComponents({ name: 'BaseDropdown' }).at(-1);
    await pickOption(pageSize, '50 на странице');
    expect(journalCalls().at(-1)).toContain('per_page=50');
  });

  it('кнопки раздела - общие .lk-button, своих больше нет', () => {
    expect(TEMPLATE).toContain('class="lk-button lk-button--primary"');
    expect(TEMPLATE).toContain('class="lk-button lk-button--secondary"');
    ['clear-filters-btn', 'export-btn', 'apply-btn'].forEach((cls) => {
      expect(SOURCE, `класс ${cls} убран`).not.toContain(cls);
    });
  });

  it('чипы быстрого отбора лежат в своей группе', () => {
    const group = TEMPLATE.slice(TEMPLATE.indexOf('class="filter-presets"'));
    expect(group.slice(0, group.indexOf('</div>'))).toContain('v-for="preset in journalPresets"');
    expect(STYLE).toMatch(/\.filter-presets\s*{[^}]*display:\s*flex/);
  });

  it('цвета и радиусы экрана взяты из токенов темы', () => {
    // Хардкод переживает смену темы и остаётся ярким пятном на тёмном фоне,
    // поэтому литералы цвета в стилях раздела запрещены целиком.
    expect(STYLE).not.toMatch(/#[0-9a-fA-F]{3,8}\b/);
    expect(STYLE).not.toMatch(/rgba?\(\s*\d/);
    expect(STYLE).not.toMatch(/border-radius:\s*\d/);
  });
});
