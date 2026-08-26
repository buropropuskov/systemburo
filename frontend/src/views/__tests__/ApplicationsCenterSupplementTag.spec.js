import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import ApplicationsCenter from '../ApplicationsCenter.vue';
import { usePermissionsStore } from '@/stores/permissions';
import { buildApplicationTags, layoutApplicationTags } from '@/utils/applicationTags';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }) }));
vi.mock('@/api/applications', () => ({
  getApplicationsPaginated: vi.fn().mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } }),
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
  BaseDropdown: true,
};

function mountCenter() {
  const perms = usePermissionsStore();
  perms.mode = 'normal';
  perms.effective = {};

  return mount(ApplicationsCenter, {
    global: {
      stubs,
      mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn(() => Promise.resolve()) } },
    },
  });
}

function application(over = {}) {
  return {
    id: 1,
    is_read: true,
    application_number: 'A-1',
    organization_name: 'Орг',
    sender_name: 'И',
    sending_datetime: '2026-01-01T10:00:00Z',
    status: 'В работе',
    confirmation: 'Согласовано',
    ...over,
  };
}

let wrapper;

describe('ApplicationsCenter — тег «Дополнение» (#1685)', () => {
  beforeEach(() => setActivePinia(createPinia()));
  afterEach(() => wrapper?.unmount());

  it('заявка с открытым раундом получает тег', async () => {
    wrapper = mountCenter();
    wrapper.vm.loading = false;
    wrapper.vm.applications = [application({ has_open_supplement: true })];
    await wrapper.vm.$nextTick();

    const tag = wrapper.find('[data-testid="center-supplement-badge-1"]');
    expect(tag.exists()).toBe(true);
    expect(tag.text()).toContain('Дополнение');
    // По этому классу свёртка --compact прячет текст в иконку: без него тег останется
    // полнотекстовым и вылезет из фиксированной колонки поверх кнопки «Скачать».
    expect(tag.classes()).toContain('rt-tag--supplement');
  });

  it('без открытого раунда тега нет', async () => {
    wrapper = mountCenter();
    wrapper.vm.loading = false;
    wrapper.vm.applications = [application({ has_open_supplement: false })];
    await wrapper.vm.$nextTick();

    expect(wrapper.find('[data-testid="center-supplement-badge-1"]').exists()).toBe(false);
  });

  it('фильтр по тегу «Дополнение» оставляет только заявки с открытым раундом', () => {
    wrapper = mountCenter();
    wrapper.vm.applications = [
      application({ id: 1, has_open_supplement: true }),
      application({ id: 2, has_open_supplement: false }),
      application({ id: 3 }),
    ];
    wrapper.vm.selectedTags = ['supplement'];

    expect(wrapper.vm.filteredApplications.map(a => a.id)).toEqual([1]);
  });

  it('пункт «Дополнение» есть в списке тегов - он же уходит в модалку фильтров', () => {
    wrapper = mountCenter();

    expect(wrapper.vm.tags).toContainEqual({ value: 'supplement', label: 'Дополнение' });
    const tagsFilter = wrapper.vm.stateFilters.find(f => f.field === 'selectedTags');
    expect(tagsFilter.options).toContainEqual({ value: 'supplement', label: 'Дополнение' });
  });

  // Колонка тегов узкая: не учтённый в раскладке тег не даёт соседям свернуться и
  // вылезает поверх колонки действий (#1315 S2).
  it('тег дополнения участвует в раскладке колонки', () => {
    const tags = buildApplicationTags({ has_open_supplement: true, sender_is_important: true });
    expect(tags.map(t => t.key)).toContain('supplement');

    const narrow = layoutApplicationTags(tags, 90);
    expect(narrow.visible.every(e => e.mode !== 'text')).toBe(true);
  });
});
