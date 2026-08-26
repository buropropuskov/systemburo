import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { flushPromises } from '@vue/test-utils';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

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

import DateFilter from '@/components/DateFilter.vue';
import { apiRequest, apiRequestRaw } from '@/api/client';
import {
  filterDropdown, journalCalls, mountView, pickOption, resetApiMocks, unmountAll,
} from './helpers/requestsView';

// Разметка раздела разнесена по вкладкам, поэтому замки читают все файлы:
// вернувшийся нативный контрол или литерал цвета в любом из них - тот же дефект.
const FILES = [
  resolve(__dirname, '../RequestsView.vue'),
  resolve(__dirname, '../../components/monitoring/JournalTab.vue'),
  resolve(__dirname, '../../components/monitoring/AnalyticsTab.vue'),
  resolve(__dirname, '../../components/monitoring/LogDetails.vue'),
  resolve(__dirname, '../../components/monitoring/RequestLogBadge.vue'),
  resolve(__dirname, '../../components/monitoring/KpiRow.vue'),
].map(path => readFileSync(path, 'utf8'));

/** Разметка всех файлов раздела одной строкой. */
const TEMPLATE = FILES.map(src => src.slice(0, src.indexOf('</template>'))).join('\n');
/**
 * Стили всех файлов раздела одной строкой, БЕЗ комментариев: запрет касается
 * объявлений, а не пояснений к ним. Номер задачи вида #2216 в комментарии
 * попадает под шаблон шестнадцатеричного цвета и ронял проверку на ровном месте.
 */
const STYLE = FILES.map(src => src.slice(src.indexOf('<style')))
  .join('\n')
  .replace(/\/\*[\s\S]*?\*\//g, '');
const SOURCE = FILES.join('\n');

afterEach(() => {
  unmountAll();
});

beforeEach(() => {
  resetApiMocks();
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
    const { wrapper } = await mountView({ page: '3' });
    await flushPromises();
    apiRequestRaw.mockClear();

    await pickOption(filterDropdown(wrapper, 0), 'POST');

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
    const { wrapper } = await mountView();
    await flushPromises();
    apiRequestRaw.mockClear();

    await pickOption(filterDropdown(wrapper, 2), '@ivanov');
    expect(journalCalls().at(-1)).toContain('user_id=42');
  });

  it('период из календаря уходит в запрос днями и снимает быстрый отбор «последний час»', async () => {
    const { wrapper, router } = await mountView({ since: '2026-08-19T09:00:00.000Z' });
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
    expect(router.currentRoute.value.query.since).toBeUndefined();
  });

  it('сброс отбора очищает и поле периода', async () => {
    const { wrapper } = await mountView({ from: '2026-08-01', to: '2026-08-20' });
    await flushPromises();

    const calendar = wrapper.findAllComponents(DateFilter)[0];
    expect(calendar.props('dateRangeStart')).toBeInstanceOf(Date);

    await wrapper.get('[data-testid="journal-clear"]').trigger('click');
    await flushPromises();

    expect(calendar.props('dateRangeStart')).toBeNull();
    expect(calendar.props('dateRangeEnd')).toBeNull();
  });

  it('список размеров страницы рисуется в body, иначе его режет подвал таблицы', () => {
    // Подвал лежит в контейнере с overflow: hidden: без телепорта нижние пункты
    // выпадали за кромку и выбрать «100 на странице» было нельзя.
    const dropdown = TEMPLATE.slice(TEMPLATE.indexOf('class="page-size-dd"'));
    expect(dropdown.slice(0, dropdown.indexOf('/>'))).toContain('teleport');
  });

  it('размер страницы меняется через общий список', async () => {
    const { wrapper } = await mountView();
    await flushPromises();
    apiRequestRaw.mockClear();

    const pageSize = wrapper.findAll('.page-size-dd').at(-1);
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
