import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { flushPromises } from '@vue/test-utils';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

// Журнал на узком экране (#2125, S9d): строка таблицы становится карточкой
// responsive-tables (rt-*), подписи полей берёт из data-label, а сплошная
// заливка успешных строк снята - помечается только ошибка.

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  apiRequestRaw: vi.fn(),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify: vi.fn() }),
}));

import { apiRequestRaw } from '@/api/client';
import { SORTABLE_COLUMNS } from '@/utils/requestLogsQuery';
import {
  lastJournalCall, logsPage, mountView, pickOption, resetApiMocks, unmountAll,
} from './helpers/requestsView';

const OK_LOG = {
  id: 1,
  url: '/api/applications?status=new',
  method: 'GET',
  response_status: 200,
  duration_us: 900,
  username: 'ivanov',
  user_id: 42,
  created_at: '2026-08-20T10:15:00Z',
};

const FAIL_LOG = { ...OK_LOG, id: 2, response_status: 500, method: 'POST' };

const SRC = readFileSync(
  resolve(__dirname, '../../components/monitoring/JournalTab.vue'), 'utf8'
);
const STYLE = SRC.slice(SRC.indexOf('<style'));

/** Объявления правила по его селектору. */
function rule(selector) {
  const start = STYLE.indexOf(`${selector} {`);
  expect(start, `в стилях есть правило ${selector}`).toBeGreaterThan(-1);
  return STYLE.slice(start, STYLE.indexOf('}', start));
}

/** Блок медиазапроса карточного режима. */
function cardMedia() {
  const start = STYLE.indexOf('@media (max-width: 767.98px)');
  expect(start, 'карточные правила стоят на пороге инфраструктуры rt-*').toBeGreaterThan(-1);
  return STYLE.slice(start);
}

async function mountJournal(logs = [OK_LOG, FAIL_LOG]) {
  apiRequestRaw.mockResolvedValue(logsPage(logs));
  const { wrapper } = await mountView();
  await flushPromises();
  return wrapper;
}

afterEach(() => {
  unmountAll();
});

beforeEach(() => {
  resetApiMocks();
});

describe('Мониторинг запросов, журнал карточками на мобилке', () => {
  it('таблица размечена под карточный режим', async () => {
    const wrapper = await mountJournal();

    expect(wrapper.get('.table-container').classes(), 'контейнер таблицы')
      .toContain('rt-table');
    expect(wrapper.get('.table-header').classes(), 'шапка колонок прячется на мобилке')
      .toContain('rt-head-row');
    wrapper.findAll('.table-row').forEach((row) => {
      expect(row.classes(), 'строка превращается в карточку').toContain('rt-row');
    });
  });

  it('подписи полей карточки совпадают с заголовками колонок', async () => {
    const wrapper = await mountJournal([OK_LOG]);

    const labels = wrapper.get('.table-row').findAll('[data-label]')
      .map(cell => cell.attributes('data-label'));
    // Подпись в карточке и заголовок колонки - одно и то же поле: разойдясь,
    // они назовут одну колонку двумя словами на разных ширинах.
    expect(labels).toEqual(SORTABLE_COLUMNS.map(c => c.label));
  });

  it('успешные строки не залиты, ошибка помечена полосой', async () => {
    const wrapper = await mountJournal();

    const [ok, failed] = wrapper.findAll('.table-row');
    expect(ok.classes(), 'успех больше не красит строку').not.toContain('success-row');
    expect(failed.classes()).toContain('error-row');
    expect(STYLE, 'правило заливки успеха удалено').not.toContain('success-row');

    const error = rule('.table-row.error-row');
    expect(error, 'полоса переживает карточный режим, где фон строки перебит')
      .toContain('box-shadow: inset 3px 0 0 var(--danger)');
    expect(error, 'заливки строки нет').not.toContain('background');
  });

  it('карточка открывает окно деталей', async () => {
    const wrapper = await mountJournal([OK_LOG]);

    await wrapper.get('.table-row.rt-row').trigger('click');
    await flushPromises();

    expect(wrapper.find('.log-details-modal').exists()).toBe(true);
  });

  it('порядок строк на мобилке задаётся списком, а не заголовком', async () => {
    const wrapper = await mountJournal([OK_LOG]);

    await pickOption(wrapper.get('.sort-dd'), 'Отклик');
    expect(lastJournalCall(), 'выбранное поле уходит на сервер').toContain('sort=duration');
    expect(lastJournalCall()).toContain('order=desc');

    await wrapper.get('[data-testid="journal-sort-order"]').trigger('click');
    await flushPromises();
    expect(lastJournalCall(), 'кнопка переворачивает направление').toContain('order=asc');

    const before = lastJournalCall();
    await pickOption(wrapper.get('.sort-dd'), 'Отклик');
    expect(lastJournalCall(), 'повтор того же поля порядок не трогает').toBe(before);
  });

  it('список сортировки живёт только на узком экране', () => {
    // На десктопе сортируют кликом по заголовку: два контрола на одно
    // действие расходятся между собой при первой же правке.
    expect(rule('.sort-bar')).toContain('display: none');
    expect(cardMedia()).toContain('.sort-bar {\n    display: flex;\n  }');
  });

  it('карточки не прилипают к кромкам, адрес переносится', () => {
    const media = cardMedia();
    expect(media, 'у тела таблицы своих полей нет - на десктопе их держала строка')
      .toContain('.table-body {\n    padding: 12px 16px;\n  }');
    expect(media, 'в карточке под адрес есть вся ширина строки')
      .toContain('overflow-wrap: anywhere');
  });
});
