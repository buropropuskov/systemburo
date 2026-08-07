import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import ApplicationsCenter from '@/views/ApplicationsCenter.vue';
import { usePermissionsStore } from '@/stores/permissions';
import { useOnboardingStore } from '@/stores/onboarding';
import { REVEAL_FIRST_APPLICATION } from '@/composables/useRevealFirstApplication';
import { OPEN_TARGETS } from '../reveal';

/**
 * Раскрытие карточки заявки в Центре по сигналу тура (`reveal.open`).
 *
 * Туры согласующего и принимающего целиком идут по Центру, а деталь заявки там -
 * модалка внутри страницы, а не отдельный роут: сам driver.js открыть её не может.
 * Владелец узла - страница со списком, и она же обязана закрыть за собой ровно то,
 * что открыла. Механика общая с личным кабинетом и вынесена в композабл
 * `useRevealFirstApplication`; здесь проверяется её подключение к Центру.
 *
 * Без этой механики сегмент карточки не «иногда пропускался бы», а пропускался бы
 * ВСЕГДА: элементов модалки в DOM нет, шаги помечены optional и молча выпадают - то
 * есть больше половины обоих туров была бы мёртвой, и ни один структурный замок
 * этого не увидел бы.
 */

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn().mockResolvedValue([]) }),
}));
vi.mock('@/api/applications', () => ({
  getApplicationsPaginated: vi.fn().mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 30 } }),
  getApplicationById: vi.fn().mockResolvedValue(null),
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

/**
 * @param {string|null} target значение reveal.open
 * @returns {Promise<void>}
 */
async function signal(target) {
  useOnboardingStore().setRevealOpen(target);
  await wrapper.vm.$nextTick();
}

describe('Центр заявок - карточка по сигналу тура', () => {
  beforeEach(() => setActivePinia(createPinia()));
  afterEach(() => wrapper?.unmount());

  it('сигнал композабла - из числа значений, которые движок вообще шлёт', () => {
    // Строка задублирована в композабле сознательно (импорт reveal.js потянул бы в
    // страницы списков шину событий и хост тура) - от расхождения стережёт этот замок.
    expect(OPEN_TARGETS).toContain(REVEAL_FIRST_APPLICATION);
  });

  it('открывает первую заявку списка и закрывает её, когда сигнал гаснет', async () => {
    wrapper = mountCenter();
    wrapper.vm.applications = [application({ id: 7 }), application({ id: 8 })];
    await wrapper.vm.$nextTick();

    await signal(REVEAL_FIRST_APPLICATION);
    expect(wrapper.vm.selectedApplication?.id).toBe(7);

    await signal(null);
    expect(wrapper.vm.selectedApplication).toBe(null);
  });

  it('на пустом Центре ничего не открывает и не падает', async () => {
    // Именно этот случай шаги сегмента карточки переживают через `optional`:
    // у человека может не быть ни одной заявки в момент прохождения тура.
    wrapper = mountCenter();
    wrapper.vm.applications = [];
    await wrapper.vm.$nextTick();

    await signal(REVEAL_FIRST_APPLICATION);
    expect(wrapper.vm.selectedApplication).toBe(null);
  });

  it('карточку, открытую человеком, тур не закрывает', async () => {
    wrapper = mountCenter();
    wrapper.vm.applications = [application({ id: 7 })];
    await wrapper.vm.$nextTick();

    wrapper.vm.selectedApplication = wrapper.vm.applications[0];
    await signal(REVEAL_FIRST_APPLICATION);
    await signal(null);
    expect(wrapper.vm.selectedApplication?.id).toBe(7);
  });

  it('человек закрыл карточку сам - гашение сигнала не трогает открытую заново', async () => {
    wrapper = mountCenter();
    wrapper.vm.applications = [application({ id: 7 })];
    await wrapper.vm.$nextTick();

    await signal(REVEAL_FIRST_APPLICATION);
    expect(wrapper.vm.selectedApplication?.id).toBe(7);

    // Крестик/Esc/свайп - владение сбрасывается, и заявку человек открывает уже для себя.
    wrapper.vm.closeDetail();
    wrapper.vm.selectedApplication = wrapper.vm.applications[0];
    await wrapper.vm.$nextTick();

    await signal(null);
    expect(wrapper.vm.selectedApplication?.id).toBe(7);
  });

  it('чужой сигнал раскрытия Центр не трогает', async () => {
    // Ось `open` общая на все узлы: колонка Админки и панель поиска ходят по ней же.
    wrapper = mountCenter();
    wrapper.vm.applications = [application({ id: 7 })];
    await wrapper.vm.$nextTick();

    await signal('admin-column');
    expect(wrapper.vm.selectedApplication).toBe(null);
  });
});
