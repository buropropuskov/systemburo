import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import ApplicationsCenter from '../ApplicationsCenter.vue';
import { usePermissionsStore } from '@/stores/permissions';
import { useAuthStore } from '@/stores/auth';
import { getApplicationsPaginated } from '@/api/applications';

/**
 * Пара переключателей «Новые»/«Обновления» в шапке Центра. Замок держит три вещи,
 * которые ломаются молча: кнопки не исчезают при нулевом счётчике (пассивный бейдж
 * прятался по v-if и сосед прыгал на его место), «Новые» ведут тот же псевдо-статус,
 * что дропдаун «Статус» (два состояния разъехались бы), и вместе фильтры не
 * включаются - на бэке unread требует отсутствия записи в application_reads, а
 * status_updated в Центре её наличия, так что пара давала бы пустой список всегда.
 */

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/api/applications', () => ({
  getApplicationsPaginated: vi.fn().mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } }),
  getApplicationById: vi.fn(),
}));
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn(), SOUND_PRESETS: [] }));

const stubs = {
  teleport: true,
  RefreshButton: true,
  ApplicationDetail: true,
  DateFilter: true,
  FilterTabs: true,
  SkeletonTransition: { template: '<div><slot /></div>' },
  SkeletonTable: true,
  LoaderSpinner: true,
  DownloadBlanksModal: true,
  Badge: true,
  BaseDropdown: true,
  ApplicationsFilterModal: true,
};

function mountCenter() {
  const perms = usePermissionsStore();
  perms.mode = 'normal';
  perms.effective = {};
  return mount(ApplicationsCenter, {
    global: {
      stubs,
      mocks: {
        $route: { query: {}, path: '/center' },
        $router: { push: vi.fn(), replace: vi.fn(() => Promise.resolve()) },
        $bus: { emit: vi.fn(), on: vi.fn(), off: vi.fn() },
      },
    },
  });
}

function app(over = {}) {
  return {
    id: 1, is_read: true, application_number: 'A-1', organization_name: 'Орг',
    sender_name: 'И', sending_datetime: '2026-01-01T10:00:00Z',
    status: 'В работе', confirmation: 'Согласовано',
    ...over,
  };
}

const unreadBtn = (w) => w.find('[data-testid="center-button-unread"]');
const updatesBtn = (w) => w.find('[data-testid="center-button-updates"]');

let wrapper;

describe('ApplicationsCenter — переключатели «Новые» и «Обновления» в шапке', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getApplicationsPaginated.mockClear();
    getApplicationsPaginated.mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } });
  });
  afterEach(() => wrapper?.unmount());

  it('обе кнопки стоят в шапке и остаются на месте при нулевых счётчиках', async () => {
    wrapper = mountCenter();
    wrapper.vm.applications = [];
    await wrapper.vm.$nextTick();

    expect(unreadBtn(wrapper).exists()).toBe(true);
    expect(updatesBtn(wrapper).exists()).toBe(true);
    // Без счётчика - только подпись: ноль рядом читался бы как «непрочитанных нет»,
    // хотя на деле их нет лишь в текущей выборке.
    expect(unreadBtn(wrapper).text()).toBe('Новые');
    expect(updatesBtn(wrapper).text()).toBe('Обновления');
  });

  it('счётчики дописываются к подписям', async () => {
    wrapper = mountCenter();
    wrapper.vm.applications = [
      app({ id: 1, is_read: false }),
      app({ id: 2, is_read: false }),
      app({ id: 3, is_read: true, has_status_update: true }),
    ];
    await wrapper.vm.$nextTick();

    expect(unreadBtn(wrapper).text()).toBe('Новые: 2');
    expect(updatesBtn(wrapper).text()).toBe('Обновления: 1');
  });

  it('«Новые» с включёнными «Обновлениями» не пропадают, хотя непрочитанных в выборке нет', async () => {
    wrapper = mountCenter();
    wrapper.vm.statusUpdatedOnly = true;
    wrapper.vm.applications = [app({ id: 4, is_read: true, has_status_update: true })];
    await wrapper.vm.$nextTick();

    expect(wrapper.vm.unreadCount).toBe(0);
    expect(unreadBtn(wrapper).exists()).toBe(true);
  });

  it('клик по «Новые» включает псевдо-статус и шлёт unread=true', async () => {
    wrapper = mountCenter();
    useAuthStore().token = 'tkn';

    await unreadBtn(wrapper).trigger('click');
    expect(wrapper.vm.unreadOnly).toBe(true);
    expect(wrapper.vm.selectedApplicationStatuses).toEqual(['Непрочитано']);
    await wrapper.vm.$nextTick();
    expect(unreadBtn(wrapper).classes()).toContain('status-btn--active');

    await wrapper.vm.buildApplicationsPage(1, 30);
    const params = getApplicationsPaginated.mock.calls.at(-1)[0];
    expect(params.unread).toBe('true');
    // Псевдо-статус не должен уехать в status: колонки с таким значением нет.
    expect(params.status).toBeUndefined();
  });

  it('повторный клик по «Новые» снимает фильтр, не трогая остальные статусы', async () => {
    wrapper = mountCenter();
    useAuthStore().token = 'tkn';
    wrapper.vm.selectedApplicationStatuses = ['Завершено'];

    await unreadBtn(wrapper).trigger('click');
    expect(wrapper.vm.selectedApplicationStatuses).toEqual(['Завершено', 'Непрочитано']);

    await unreadBtn(wrapper).trigger('click');
    expect(wrapper.vm.selectedApplicationStatuses).toEqual(['Завершено']);
    expect(wrapper.vm.unreadOnly).toBe(false);
  });

  it('состояние «Новые» читается из дропдауна «Статус», а не из своего флага', async () => {
    wrapper = mountCenter();
    useAuthStore().token = 'tkn';

    wrapper.vm.setMultiFilter('selectedApplicationStatuses', ['Непрочитано']);
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.unreadOnly).toBe(true);
    expect(unreadBtn(wrapper).classes()).toContain('status-btn--active');

    wrapper.vm.setMultiFilter('selectedApplicationStatuses', []);
    await wrapper.vm.$nextTick();
    expect(wrapper.vm.unreadOnly).toBe(false);
    expect(unreadBtn(wrapper).classes()).not.toContain('status-btn--active');
  });

  it('клик по «Обновления» включает фильтр и гасит «Новые»', async () => {
    wrapper = mountCenter();
    useAuthStore().token = 'tkn';
    wrapper.vm.selectedApplicationStatuses = ['Непрочитано', 'Завершено'];

    await updatesBtn(wrapper).trigger('click');
    expect(wrapper.vm.statusUpdatedOnly).toBe(true);
    expect(wrapper.vm.unreadOnly).toBe(false);
    expect(wrapper.vm.selectedApplicationStatuses).toEqual(['Завершено']);

    await wrapper.vm.buildApplicationsPage(1, 30);
    const params = getApplicationsPaginated.mock.calls.at(-1)[0];
    expect(params.status_updated).toBe('true');
    expect(params.unread).toBeUndefined();
  });

  it('включение «Новые» гасит «Обновления» - вместе они дают пустой список', async () => {
    wrapper = mountCenter();
    useAuthStore().token = 'tkn';
    wrapper.vm.statusUpdatedOnly = true;

    await unreadBtn(wrapper).trigger('click');
    expect(wrapper.vm.unreadOnly).toBe(true);
    expect(wrapper.vm.statusUpdatedOnly).toBe(false);

    await wrapper.vm.buildApplicationsPage(1, 30);
    const params = getApplicationsPaginated.mock.calls.at(-1)[0];
    expect(params.status_updated).toBeUndefined();
  });

  it('выбор «Непрочитано» в дропдауне «Статус» тоже гасит «Обновления»', async () => {
    wrapper = mountCenter();
    useAuthStore().token = 'tkn';
    wrapper.vm.statusUpdatedOnly = true;

    wrapper.vm.setMultiFilter('selectedApplicationStatuses', ['Непрочитано']);
    expect(wrapper.vm.statusUpdatedOnly).toBe(false);
  });

  it('resetFilters гасит оба переключателя', () => {
    wrapper = mountCenter();
    wrapper.vm.selectedApplicationStatuses = ['Непрочитано'];
    wrapper.vm.statusUpdatedOnly = true;

    wrapper.vm.resetFilters();

    expect(wrapper.vm.unreadOnly).toBe(false);
    expect(wrapper.vm.statusUpdatedOnly).toBe(false);
  });

  it('«Обновления» остаются в наборе «есть активные фильтры» - иначе сброс был бы недоступен', () => {
    wrapper = mountCenter();
    wrapper.vm.statusUpdatedOnly = true;
    expect(wrapper.vm.hasActiveFilters).toBe(true);
    // Точку на мобильной кнопке «Фильтр» они больше не зажигают: переключатель уехал
    // из модалки в шапку и виден там сам.
    expect(wrapper.vm.hasModalFilters).toBe(false);
  });

  it('неактивные кнопки подсвечены цветом своей строки, только когда есть что смотреть', async () => {
    wrapper = mountCenter();
    wrapper.vm.applications = [app({ id: 1, is_read: false })];
    await wrapper.vm.$nextTick();
    expect(unreadBtn(wrapper).classes()).toContain('status-btn--unread');

    wrapper.vm.applications = [app({ id: 2, is_read: true })];
    await wrapper.vm.$nextTick();
    expect(unreadBtn(wrapper).classes()).not.toContain('status-btn--unread');
  });
});
