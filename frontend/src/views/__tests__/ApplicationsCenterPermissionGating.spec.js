import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import ApplicationsCenter from '../ApplicationsCenter.vue';
import { usePermissionsStore } from '@/stores/permissions';

// Центр на mounted дёргает fetch/polling - глушим API; нас интересует гейтинг
// раздела «Архив» (center.archive) и кнопки «Скачать» (action.export.applications).
vi.mock('@/api/client', () => ({ apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }) }));
// Список Центра (#1158) идёт через getApplicationsPaginated, не apiRequest напрямую.
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
  Badge: true,
  BaseDropdown: true,
};

function seedPerms({ mode = 'normal', allow = [] } = {}) {
  const perms = usePermissionsStore();
  perms.mode = mode;
  perms.effective = Object.fromEntries(allow.map(k => [k, { value: 'allow', source: 'role' }]));
}

function mountCenter() {
  return mount(ApplicationsCenter, {
    global: {
      stubs,
      mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } },
    },
  });
}

/** Кладёт одну заявку c бланком в список (минуя API) и снимает скелетон. */
async function withApplication(w) {
  w.vm.loading = false;
  w.vm.applications = [{
    id: 1,
    application_number: 'A-1',
    sending_datetime: '2026-01-01T10:00:00Z',
    status: 'Согласование',
    confirmation: 'Согласование',
    organization_name: 'Орг',
    sender_name: 'Иванов',
    is_read: true,
    has_blank_template: true,
  }];
  await w.vm.$nextTick();
}

const tabs = w => w.find('.center__tabs');
const download = w => w.find('.download-btn');

let wrapper;

describe('ApplicationsCenter — гейтинг центр-действий (срез 5)', () => {
  beforeEach(() => setActivePinia(createPinia()));
  afterEach(() => wrapper?.unmount());

  describe('Раздел «Архив» (center.archive)', () => {
    it('normal с правом: переключатель Активные/Архив виден', () => {
      seedPerms({ allow: ['center.archive'] });
      wrapper = mountCenter();
      expect(tabs(wrapper).exists()).toBe(true);
    });

    it('normal без права: переключатель скрыт', () => {
      seedPerms({ allow: [] });
      wrapper = mountCenter();
      expect(tabs(wrapper).exists()).toBe(false);
    });

    it('без права прямой URL ?archive=true не переключает в архив', () => {
      seedPerms({ allow: [] });
      wrapper = mount(ApplicationsCenter, {
        global: { stubs, mocks: { $route: { query: { archive: 'true' } }, $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) } } },
      });
      expect(wrapper.vm.archiveMode).toBe('active');
    });

    it('super: переключатель виден без явного гранта', () => {
      seedPerms({ mode: 'super' });
      wrapper = mountCenter();
      expect(tabs(wrapper).exists()).toBe(true);
    });
  });

  describe('Экспорт заявок / «Скачать» (action.export.applications)', () => {
    it('normal с правом: кнопка «Скачать» у заявки с бланком видна', async () => {
      seedPerms({ allow: ['action.export.applications'] });
      wrapper = mountCenter();
      await withApplication(wrapper);
      expect(download(wrapper).exists()).toBe(true);
    });

    it('normal без права: кнопка «Скачать» скрыта даже при наличии бланка', async () => {
      seedPerms({ allow: [] });
      wrapper = mountCenter();
      await withApplication(wrapper);
      expect(download(wrapper).exists()).toBe(false);
    });

    it('super: кнопка «Скачать» видна без явного гранта', async () => {
      seedPerms({ mode: 'super' });
      wrapper = mountCenter();
      await withApplication(wrapper);
      expect(download(wrapper).exists()).toBe(true);
    });
  });
});
