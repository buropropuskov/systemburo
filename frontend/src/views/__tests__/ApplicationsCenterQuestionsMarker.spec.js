import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import ApplicationsCenter from '../ApplicationsCenter.vue';
import { usePermissionsStore } from '@/stores/permissions';
import { buildApplicationTags, layoutApplicationTags } from '@/utils/applicationTags';

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

function seedPerms() {
  const perms = usePermissionsStore();
  perms.mode = 'normal';
  perms.effective = {};
}

function mountCenter() {
  return mount(ApplicationsCenter, {
    global: {
      stubs,
      mocks: { $route: { query: {} }, $router: { push: vi.fn(), replace: vi.fn(() => Promise.resolve()) } },
    },
  });
}

let wrapper;

describe('ApplicationsCenter — маркер обсуждения (#973)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    seedPerms();
  });
  afterEach(() => wrapper?.unmount());

  it('openApplication НЕ гасит маркер обсуждения при открытии (#973: гасит прочтение тем)', async () => {
    wrapper = mountCenter();
    const app = { id: 1, is_read: true, has_unseen_questions: true, application_number: 'A-1', organization_name: 'Орг' };
    wrapper.vm.applications = [app];
    await wrapper.vm.openApplication(app);
    // Открытие заявки больше не снимает иконку - её гасит прочтение топиков в детали,
    // список обновится при следующей загрузке.
    expect(app.has_unseen_questions).toBe(true);
  });

  it('onQuestionsRead гасит маркер заявки в списке (всё обсуждение прочитано в детали)', async () => {
    wrapper = mountCenter();
    const app = { id: 7, is_read: true, has_unseen_questions: true, application_number: 'A-7', organization_name: 'Орг' };
    wrapper.vm.applications = [app];
    wrapper.vm.onQuestionsRead(7);
    expect(app.has_unseen_questions).toBe(false);
  });

  it('маркер обсуждения рендерится, даже если у заявки нет других тегов', async () => {
    wrapper = mountCenter();
    wrapper.vm.loading = false;
    wrapper.vm.applications = [{
      id: 1, is_read: true, has_unseen_questions: true,
      application_number: 'A-1', organization_name: 'Орг', sender_name: 'И',
      sending_datetime: '2026-01-01T10:00:00Z', status: 'Согласование', confirmation: 'Согласование',
    }];
    await wrapper.vm.$nextTick();

    expect(wrapper.find('.application-tags').exists()).toBe(true);
    expect(wrapper.find('[data-testid="center-questions-badge-1"]').exists()).toBe(true);
  });

  // Тег, не учтённый в раскладке колонки, не даёт соседям свернуться и вылезает
  // поверх колонки действий (#1315 S2). Раскладку считает layoutApplicationTags по
  // реальной ширине колонки, поэтому проверяем участие тега в ней.
  it('маркер обсуждения участвует в раскладке колонки и сворачивается вместе с соседями', () => {
    const tags = buildApplicationTags({ has_unseen_questions: true, sender_is_important: true });
    expect(tags.map(t => t.key)).toContain('questions');

    const narrow = layoutApplicationTags(tags, 90);
    expect(narrow.visible.every(e => e.mode !== 'text')).toBe(true);

    // Одинокий тег возвращает подпись, когда колонке есть где развернуться. Ширину
    // берём с широкого края диапазона колонки (90-170px): подпись «Обсуждение» стоит
    // 93px и в самую узкую колонку не влезает - там же остаётся иконочным и «Дополнение».
    const alone = layoutApplicationTags(buildApplicationTags({ has_unseen_questions: true }), 170);
    expect(alone.visible[0].mode).toBe('text');
  });

  it('deep-link ?open открывает заявку из списка и чистит query', async () => {
    wrapper = mountCenter();
    const push = wrapper.vm.$router;
    wrapper.vm.$route.query = { open: '5' };
    const app = { id: 5, is_read: true, application_number: 'A-5', organization_name: 'Орг' };
    wrapper.vm.applications = [app];
    wrapper.vm.openFromDeepLink();
    expect(wrapper.vm.selectedApplication).toBe(app);
    expect(push.replace).toHaveBeenCalled();
  });
});
