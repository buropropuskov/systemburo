import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

/**
 * Волна 5 мобильной раскладки, экран «Доступные мне» (мокап docs/mockups/mobile-ux.html).
 *
 * Претензия владельца: шапка слишком высокая, а срок со статусом ломали строку
 * организации. Перенос сделан через v-if (элемент физически один), поэтому проверяем
 * не «срок где-то есть», а ГДЕ он лежит и что в DOM он один: скрытая копия задвоила бы
 * data-testid и сломала якорь онбординг-тура.
 *
 * Геометрию (48px шапка, полоса 36px, снятая рамка панели) юнит не видит - jsdom не
 * считает ни каскад, ни медиа-запросы, поэтому её стережём чтением самого SFC, а
 * подтверждаем замером в браузере.
 */

vi.mock('@/api/applications', () => ({
  getAccessibleAttachments: vi.fn(),
  getAccessibleAttachmentDetail: vi.fn(),
}));

vi.mock('@/api/organizations', () => ({
  getOrganizations: vi.fn(),
  getCompanies: vi.fn(),
}));

vi.mock('@/api/attachment-templates', () => ({
  previewBlank: vi.fn(),
}));

vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify: vi.fn() }),
}));

vi.mock('@/services/eventStream', () => ({
  default: {
    connect: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(() => vi.fn()),
    onStatus: vi.fn(() => vi.fn()),
  },
}));

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace: vi.fn(() => Promise.resolve()) }),
  createRouter: () => ({ beforeEach: vi.fn(), afterEach: vi.fn(), push: vi.fn(), replace: vi.fn() }),
  createWebHistory: () => ({}),
}));

import AccessibleAttachmentsView from '@/views/AccessibleAttachmentsView.vue';
import { getAccessibleAttachments } from '@/api/applications';
import { getOrganizations, getCompanies } from '@/api/organizations';

const SOURCE = readFileSync(
  path.resolve(__dirname, '../AccessibleAttachmentsView.vue'),
  'utf8',
);

const stubs = {
  AdminPageShell: { template: '<div><slot /></div>' },
  RefreshButton: { template: '<button class="refresh-stub" />' },
  ApplicationAttachmentDetail: { template: '<div class="detail-stub" />' },
  BaseModal: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
  XlsxViewer: true,
};

const origMatchMedia = window.matchMedia;
function setNarrow(matches) {
  window.matchMedia = () => ({
    matches,
    addEventListener() {},
    removeEventListener() {},
    addListener() {},
    removeListener() {},
  });
}

function makeItem(id) {
  return {
    attachment_id: id,
    attachment_type: 'items',
    attachment_display_name: `Заявка на ввоз ${id}`,
    application_number: '№ 20260812/003',
    organization_name: 'р-н Мегобари',
    company_name: 'ООО «Победа»',
    sender_full_name: 'Пхакадзе В.',
    entry_date_from: '2026-08-13',
    entry_date_to: '2026-08-15',
    places: 'Дебаркадер №1, №2',
    status: 'В работе',
  };
}

async function mountNarrow(matches) {
  setNarrow(matches);
  getAccessibleAttachments.mockResolvedValue({
    items: [makeItem(1)],
    meta: { total: 24, page: 1, per_page: 30 },
  });
  const wrapper = mount(AccessibleAttachmentsView, { global: { stubs } });
  await flushPromises();
  return wrapper;
}

let wrapper;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  getOrganizations.mockResolvedValue([]);
  getCompanies.mockResolvedValue([]);
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  window.matchMedia = origMatchMedia;
});

describe('«Доступные мне» на телефоне - раскладка по мокапу', () => {
  it('счётчик записей стоит в шапке экрана и только на мобилке', async () => {
    wrapper = await mountNarrow(true);
    const badge = wrapper.find('[data-testid="aa-count-badge"]');
    expect(badge.exists()).toBe(true);
    expect(badge.text()).toBe('24');
    // Счётчик - часть шапки, а не отдельная строка над списком.
    expect(wrapper.find('.management-header [data-testid="aa-count-badge"]').exists()).toBe(true);

    wrapper.unmount();
    wrapper = await mountNarrow(false);
    // На десктопе то же число уже стоит в подвале списка - второго не заводим.
    expect(wrapper.find('[data-testid="aa-count-badge"]').exists()).toBe(false);
    expect(wrapper.find('.list-footer').text()).toContain('24');
  });

  it('срок уходит из шапки карточки в строку меты вместе с местами', async () => {
    wrapper = await mountNarrow(true);
    const card = wrapper.find('[data-testid="aa-card"]');

    // Ровно один узел срока в DOM: скрытая копия задвоила бы data-testid.
    expect(wrapper.findAll('[data-testid="aa-card-date"]')).toHaveLength(1);
    expect(card.find('.attachment-card__head [data-testid="aa-card-date"]').exists()).toBe(false);

    const term = card.find('.attachment-card__meta--term');
    expect(term.exists()).toBe(true);
    expect(term.find('[data-testid="aa-card-date"]').text()).toBe('13.08.2026 - 15.08.2026');
    // Разделитель между сроком и местами - часть строки, а не CSS-декор: без него
    // «15.08.2026Дебаркадер» слипается в одно слово.
    expect(term.text().replace(/\s+/g, ' ')).toBe('13.08.2026 - 15.08.2026 · Дебаркадер №1, №2');
    // Подпись поля («Места:») в карточку не возвращается - значения говорят сами за себя.
    expect(card.find('.attachment-card__places').exists()).toBe(false);
  });

  it('в шапке карточки остаются только тип и организация', async () => {
    wrapper = await mountNarrow(true);
    const head = wrapper.find('.attachment-card__head');
    expect(head.find('.badge').text()).toBe('ТМЦ');
    expect(head.find('.attachment-card__org').text()).toBe('р-н Мегобари / ООО «Победа»');
    expect(head.find('.attachment-card__status').exists()).toBe(false);
  });

  it('статус переезжает в подвал карточки', async () => {
    wrapper = await mountNarrow(true);
    const card = wrapper.find('[data-testid="aa-card"]');
    const foot = card.find('.attachment-card__foot');
    expect(foot.exists()).toBe(true);
    expect(foot.find('.attachment-card__status').exists()).toBe(true);
    expect(wrapper.findAll('.attachment-card__status')).toHaveLength(1);
  });

  it('на десктопе карточка не меняется: срок и статус остаются в шапке', async () => {
    wrapper = await mountNarrow(false);
    const card = wrapper.find('[data-testid="aa-card"]');
    expect(card.find('.attachment-card__head [data-testid="aa-card-date"]').exists()).toBe(true);
    expect(card.find('.attachment-card__head .attachment-card__status').exists()).toBe(true);
    expect(card.find('.attachment-card__foot').exists()).toBe(false);
    expect(card.find('.attachment-card__meta--term').exists()).toBe(false);
    expect(card.find('.attachment-card__places').text()).toContain('Места:');
  });
});

describe('«Доступные мне» - геометрия мобильного экрана (чтение SFC)', () => {
  const mobileBlock = SOURCE.slice(SOURCE.indexOf('@media (max-width: 767.98px)'));

  it('порог разметки и стилей один - 767.98', () => {
    expect(SOURCE).toContain('useNarrowScreen(767.98)');
    expect(mobileBlock).toContain('@media (max-width: 767.98px)');
    // Прежний рассинхрон 768 (разметка) против 767.98 (RefreshButton, карточки).
    expect(SOURCE).not.toContain('@media (max-width: 768px)\n  .filters__search');
  });

  it('шапка экрана - одна строка 48px, полоса поиска - 36px под ней', () => {
    expect(mobileBlock).toMatch(/\.management-header\s*\{[^}]*height:\s*48px/);
    expect(mobileBlock).toMatch(/\.management-header\s*\{[^}]*flex-wrap:\s*nowrap/);
    expect(mobileBlock).toMatch(/\.filters__search-input\s*\{[^}]*height:\s*36px/);
  });

  /*
   * Волна 7: владелец забраковал волну 6 с другой стороны - "шапка закрепляется,
   * за шапкой видно скроллящиеся элементы", "скроллится только список сам".
   * Панель теперь фиксированной высотой вьюпорта (100dvh за вычетом сайтовой
   * шапки и отступов AdminPageShell), overflow: hidden - контент внутри не
   * должен вытекать наружу и тянуть за собой document scroll. Шапка экрана и
   * полоса поиска больше не sticky (скроллить им не от чего убегать - панель
   * сама не скроллится), а просто закреплённый первый ряд flex-колонки.
   * Верхние углы по-прежнему дублирует сама шапка радиусом на 1px меньше
   * панельного - без этого её прямой угол торчал бы из скруглённого угла панели.
   */
  it('панель фиксированной высоты, скругление держит и не отдаёт скролл наружу', () => {
    const panel = mobileBlock.match(/\.admin-page \.accessible-attachments\.dashboard-card\s*\{[^}]*\}/)[0];
    expect(panel).toContain('overflow: hidden');
    expect(panel).toContain('height: calc(100dvh');
    expect(panel).toContain('!important');
    expect(panel).toContain('display: flex');
    expect(panel).not.toContain('border: none');

    // Радиус панели читаем из самого правила, а верхние углы шапки требуем
    // вывести из него же. Поменяют панель - тест упадёт, пока шапку не поправят
    // следом: иначе её прямой угол снова вылезет поверх скруглённого угла панели.
    const panelRadius = panel.match(/border-radius:\s*([^;]+);/)[1].trim();
    expect(panelRadius).not.toBe('0');
    expect(panelRadius).not.toBe('0px');

    const header = mobileBlock.match(/\.management-header\s*\{[^}]*\}/)[0];
    expect(header).not.toContain('position: sticky');
    expect(header).toContain('flex-shrink: 0');
    const corner = `calc(${panelRadius} - 1px)`;
    expect(header).toContain(`border-radius: ${corner} ${corner} 0 0;`);
  });

  it('лента списка и деталь вложения скроллятся сами - панель и документ не скроллятся', () => {
    expect(SOURCE).toMatch(/data-scroll-own[\s\S]{0,40}data-testid="aa-skeleton"/);
    expect(SOURCE).toMatch(/data-scroll-own[\s\S]{0,40}data-testid="aa-list"/);
    expect(SOURCE).toContain('class="detail-scroll"');

    expect(mobileBlock).toMatch(/\.cards-list\s*\{[^}]*overflow-y:\s*auto/);
    expect(mobileBlock).toMatch(/\.detail-scroll\s*\{[^}]*overflow-y:\s*auto/);

    // content-container больше не отдаёт переполнение наружу - AdminPageShell
    // форсит на этом классе overflow:visible!important с той же специфичностью
    // (0,3,0), поэтому переопределение обязано быть составным и тоже !important.
    const container = mobileBlock.match(/\.admin-page \.accessible-attachments \.content-container\s*\{[^}]*\}/)[0];
    expect(container).toContain('overflow: hidden !important');
  });

  /*
   * Волна 8, п.1: «Заявка», «Посмотреть файл» и карточка вложения - три прямых
   * ребёнка .detail-scroll, а gap самого .detail-section разводит только её
   * «шапку» (.detail-back) с .detail-scroll. Без своего flex+gap .detail-scroll
   * был block-контейнером, и блоки стояли встык без зазора. Проверяем оба
   * правила: базовое (15px, действует и на десктопе) и мобильное (12px).
   */
  it('.detail-scroll сама разводит вложенные блоки зазором - и на десктопе, и на мобилке', () => {
    const baseBlock = SOURCE.slice(0, SOURCE.indexOf('@media (max-width: 767.98px)'));
    const baseRule = baseBlock.match(/\.detail-scroll\s*\{[^}]*\}/)[0];
    expect(baseRule).toContain('display: flex');
    expect(baseRule).toMatch(/gap:\s*15px/);

    const mobileRule = mobileBlock.match(/\.detail-scroll\s*\{[^}]*\}/)[0];
    expect(mobileRule).toMatch(/gap:\s*12px/);
  });

  /*
   * Волна 8, п.3: AdminPageShell красит .admin-page только padding'ом - гутер
   * вокруг фикс-высоты панели прозрачный и в тёмной теме показывает фон body
   * (#1f2229 против #272b33 у панели), особенно заметно в прямых углах гутера,
   * где скруглённая панель отступает от его прямоугольной рамки ("чёрные
   * квадратные углы внизу", claim владельца). :has() красит гутер именно этого
   * экрана в --surface, не трогая остальные admin-страницы.
   */
  it('гутер вокруг панели красится в --surface на мобилке - иначе видно фон body по углам', () => {
    expect(mobileBlock).toMatch(
      /\.admin-page:has\(\.accessible-attachments\)\s*\{[^}]*background:\s*var\(--surface\)/,
    );
  });

  /*
   * Волна 9: box-shadow карточек и блока заявки в тёмной теме (--shadow-drop
   * rgba(0,0,0,0.6)) читался чёрным прямоугольником позади блока. Владелец:
   * "убери тени на этой странице". Карточки списка и блок заявки - свои
   * стили этого экрана, тень снята прямо в правиле. Блок вложения -
   * ApplicationAttachmentDetail, общий компонент, поэтому его тень глушится
   * только здесь через :deep, а не в самом компоненте (другие экраны его
   * не просили).
   */
  it('карточки списка, блок заявки и блок вложения - без декоративного box-shadow', () => {
    const cardRule = SOURCE.match(/\.attachment-card\s*\{[^}]*\}/)[0];
    expect(cardRule).not.toMatch(/box-shadow/);

    const cardHoverRule = SOURCE.match(/\.attachment-card:hover\s*\{[^}]*\}/)[0];
    expect(cardHoverRule).not.toMatch(/box-shadow/);

    const cardActiveRule = SOURCE.match(/\.attachment-card--active\s*\{[^}]*\}/)[0];
    expect(cardActiveRule).not.toMatch(/box-shadow/);

    const applicationBlockRule = SOURCE.match(/\.application-block\s*\{[^}]*\}/)[0];
    expect(applicationBlockRule).not.toMatch(/box-shadow/);

    const detailOverrideRule = SOURCE.match(/\.detail-section :deep\(\.attachment-details\)\s*\{[^}]*\}/)[0];
    expect(detailOverrideRule).toMatch(/box-shadow:\s*none/);
  });
});
