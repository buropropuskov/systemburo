import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

// Чип "Обновления" в ЛК (#1349 срез 4): серверный фильтр status_updated=true,
// счётчик из отдельного эндпоинта, оптимистичное гашение при открытии.
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve([]) })),
}));
vi.mock('@/api/applications', () => ({
  getUserApplicationsPaginated: vi.fn(() => Promise.resolve({ items: [], meta: { total: 0, page: 1, per_page: 30 } })),
  getApplicationById: vi.fn(() => Promise.resolve({ message: 'Не найдена' })),
  getUserStatusUpdatesCount: vi.fn(() => Promise.resolve({ status_updates: 0 })),
}));
import UserApplications from '../UserApplications.vue';
import { getUserApplicationsPaginated, getUserStatusUpdatesCount } from '@/api/applications';

function mountUA(props = {}) {
  setActivePinia(createPinia());
  const wrapper = shallowMount(UserApplications, {
    props: { userId: 1, ...props },
    global: { mocks: { $route: { query: {} }, $router: { replace: vi.fn(() => Promise.resolve()), push: vi.fn() } } },
  });
  return { wrapper };
}

describe('UserApplications — чип "Обновления" (#1349 срез 4)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getUserApplicationsPaginated.mockClear();
    getUserStatusUpdatesCount.mockClear();
    getUserStatusUpdatesCount.mockResolvedValue({ status_updates: 0 });
  });

  it('на mount тянет счётчик из эндпоинта ЛК и кладёт в statusUpdateCount', async () => {
    getUserStatusUpdatesCount.mockResolvedValueOnce({ status_updates: 5 });
    const { wrapper } = mountUA();
    await flushPromises();

    expect(getUserStatusUpdatesCount).toHaveBeenCalled();
    expect(wrapper.vm.statusUpdateCount).toBe(5);
  });

  it('пусто под чипом объясняется фильтром, а не отсутствием заявок (#2339)', async () => {
    // «Заявок нет. У вас ещё нет отправленных заявок» под включённым чипом - враньё:
    // заявки есть, просто ни одна не обновлялась. Чип обязан считаться фильтром.
    const { wrapper } = mountUA();
    await flushPromises();
    expect(wrapper.vm.hasActiveFilters, 'без фильтров признак выключен').toBeFalsy();

    wrapper.vm.toggleStatusUpdated();
    await flushPromises();
    expect(wrapper.vm.statusUpdatedOnly).toBe(true);
    expect(wrapper.vm.hasActiveFilters, 'включённый чип - активный фильтр').toBeTruthy();
  });

  it('на узком экране фильтр уходит своей строкой под заголовок и чип (#2339)', () => {
    // Проверяем правило, а не отрисовку: jsdom не считает раскладку, а дефект был
    // именно в ней - в общей строке фильтр жался до 78px и показывал «М...».
    const стили = readFileSync(resolve(__dirname, '..', 'UserApplications.vue'), 'utf8')
      .split('<style')[1] || '';
    const правило = стили.match(/\.cabinet__filter-dropdown\s*\{[^}]*\}/g)?.at(-1) || '';
    expect(правило, 'фильтр занимает всю ширину строки').toMatch(/flex:\s*1 0 100%/);
    expect(правило, 'order уводит его под заголовок и чип').toMatch(/order:\s*3/);
  });

  it('пункты фильтра читаются целиком, а меню не уже их (#2339)', async () => {
    // На стенде фильтр схлопывался до 78px: триггер показывал «М...», пункты -
    // «Мои ...» и «Заяв...». Двух вещей не хватало: короткого лейбла и меню, которое
    // не наследует ширину узкого триггера.
    const { wrapper } = mountUA({ userId: 7, userOrganizationId: 42 });
    await flushPromises();

    const пункты = wrapper.vm.filterOptions.map((o) => o.label);
    expect(пункты).toEqual(['Мои заявки', 'Заявки организации']);
    пункты.forEach((л) => expect(л.length, `«${л}» не влезает в узкий фильтр`).toBeLessThanOrEqual(20));

    const dd = wrapper.findComponent({ name: 'BaseDropdown' });
    expect(dd.exists()).toBe(true);
    expect(dd.props('menuMinWidth'), 'меню должно быть шире узкого триггера')
      .toBeGreaterThanOrEqual(200);
  });

  it('счётчик уходит с той же вкладкой, что и список (#2339)', async () => {
    const { wrapper } = mountUA({ userId: 7, userOrganizationId: 42 });
    await flushPromises();
    await wrapper.setData({ currentFilter: 'my' });

    getUserStatusUpdatesCount.mockClear();
    await wrapper.vm.fetchStatusUpdateCount();
    expect(
      getUserStatusUpdatesCount.mock.calls.at(-1)[0],
      'вкладка «Мои заявки» - счёт только по своим',
    ).toEqual({ sender_user_id: 7 });

    // Скоуп кабинета шире вкладки: без этого параметра чип считал заявки всей
    // организации и обещал больше, чем показывал список по клику.
    getUserStatusUpdatesCount.mockClear();
    wrapper.vm.setFilter('organization');
    await flushPromises();
    expect(
      getUserStatusUpdatesCount.mock.calls.at(-1)[0],
      'вкладка «Организация» - счёт по организации',
    ).toEqual({ organization_id: 42 });
  });

  it('смена вкладки пересчитывает чип, а не только список (#2339)', async () => {
    const { wrapper } = mountUA({ userId: 7, userOrganizationId: 42 });
    await flushPromises();

    getUserStatusUpdatesCount.mockClear();
    wrapper.vm.setFilter('organization');
    await flushPromises();
    expect(getUserStatusUpdatesCount, 'без перезапроса чип показывал бы число прошлой вкладки')
      .toHaveBeenCalledTimes(1);
  });

  it('список и чип берут вкладку из одной точки (#2339)', async () => {
    // Пока параметры вкладки собирались в двух местах, они и разъехались: список
    // сузился до вкладки, счётчик остался на всём скоупе.
    const { wrapper } = mountUA({ userId: 7, userOrganizationId: 42 });
    await flushPromises();
    await wrapper.setData({ currentFilter: 'organization' });

    getUserApplicationsPaginated.mockClear();
    await wrapper.vm.buildUserApplicationsPage(1, 30);
    const параметрыСписка = getUserApplicationsPaginated.mock.calls.at(-1)[0];
    const параметрыЧипа = wrapper.vm.scopeParams();

    expect(параметрыЧипа).toEqual({ organization_id: 42 });
    expect(параметрыСписка.organization_id, 'список и чип сужаются одинаково')
      .toBe(параметрыЧипа.organization_id);
    expect(параметрыСписка.sender_user_id, 'вкладка «Организация» не сужает по автору')
      .toBeUndefined();
  });

  it('чип рисует счётчик, когда обновления есть', async () => {
    const { wrapper } = mountUA();
    await flushPromises();
    wrapper.vm.statusUpdateCount = 3;
    await wrapper.vm.$nextTick();

    const chip = wrapper.find('[data-testid="lk-button-updates"]');
    expect(chip.exists()).toBe(true);
    expect(chip.text()).toContain('Обновления: 3');
  });

  it('toggleStatusUpdated переключает фильтр и уводит status_updated=true в запрос', async () => {
    const { wrapper } = mountUA();
    await flushPromises();

    expect(wrapper.vm.statusUpdatedOnly).toBe(false);
    wrapper.vm.toggleStatusUpdated();
    expect(wrapper.vm.statusUpdatedOnly).toBe(true);

    await wrapper.vm.buildUserApplicationsPage(1, 30);
    const params = getUserApplicationsPaginated.mock.calls.at(-1)[0];
    expect(params.status_updated).toBe('true');
    expect(params.sender_user_id).toBe(1);
  });

  it('без активного чипа фильтр status_updated не уходит в запрос', async () => {
    const { wrapper } = mountUA();
    await flushPromises();

    await wrapper.vm.buildUserApplicationsPage(1, 30);
    const params = getUserApplicationsPaginated.mock.calls.at(-1)[0];
    expect(params.status_updated).toBeUndefined();
  });

  it('openApplication оптимистично гасит флаг и уменьшает счётчик', async () => {
    const { wrapper } = mountUA();
    await flushPromises();
    wrapper.vm.statusUpdateCount = 3;
    const app = { id: 7, has_status_update: true };

    await wrapper.vm.openApplication(app);

    expect(app.has_status_update).toBe(false);
    expect(wrapper.vm.statusUpdateCount).toBe(2);
    expect(wrapper.vm.showDetailModal).toBe(true);
  });

  it('открытие заявки без флага не трогает счётчик', async () => {
    const { wrapper } = mountUA();
    await flushPromises();
    wrapper.vm.statusUpdateCount = 3;
    const app = { id: 8, has_status_update: false };

    await wrapper.vm.openApplication(app);

    expect(wrapper.vm.statusUpdateCount).toBe(3);
  });

  it('сбой загрузки счётчика сохраняет последнее значение, не обнуляет', async () => {
    const { wrapper } = mountUA();
    await flushPromises();
    wrapper.vm.statusUpdateCount = 4;

    getUserStatusUpdatesCount.mockRejectedValueOnce(new Error('boom'));
    await wrapper.vm.fetchStatusUpdateCount();

    expect(wrapper.vm.statusUpdateCount).toBe(4);
  });
});
