import { vi, expect } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createRouter, createMemoryHistory } from 'vue-router';
import { createPinia, setActivePinia } from 'pinia';
import RequestsView from '@/views/RequestsView.vue';
import { apiRequest, apiRequestRaw } from '@/api/client';

/*
 * Общая оснастка спек раздела мониторинга. Роутер здесь настоящий (memory):
 * вкладки читают отбор через useRoute/useRouter, а подменённый объект $route
 * такие компоненты не видят - и проверка «отбор живёт в адресе» на моке
 * доказывала бы только то, что мок вызван.
 */

export const stubs = {
  AdminPageShell: { template: '<div><slot /></div>' },
  // Окно деталей телепортируется в body: без заглушки его не видит ни wrapper,
  // ни размонтирование спеки - окно осталось бы в документе между тестами.
  teleport: true,
  // Кнопка обновления и тумблер ленты подменяются рабочими заглушками: тесты
  // жмут их как пользователь, поэтому пустой шаблон здесь не годится.
  // Проп loading заглушка держит классом: раздел показывает им обновление, и
  // спеки проверяют именно его, а не факт вызова.
  RefreshButton: {
    props: ['loading'],
    template: '<button class="refresh-stub" :class="{ \'is-loading\': loading }" @click="$emit(\'refresh\')" />',
  },
  SearchComponent: { template: '<input class="search-stub" />' },
  RealTimeChart: { template: '<div class="chart-stub" />' },
  // График по суткам рисует Chart.js на холсте, которого в jsdom нет. Заглушка
  // держит props: спеки проверяют, какой ряд уходит в график, а как он рисуется -
  // дело его собственной спеки.
  DailyRequestsChart: { props: ['points'], template: '<div class="daily-chart-stub" />' },
  LoaderSpinner: { template: '<div class="loader-stub" />' },
  AppIcon: { template: '<i />' },
  ToggleSwitch: {
    props: ['modelValue'],
    template: '<label class="toggle-stub" @click="$emit(\'update:modelValue\', !modelValue)"><slot /></label>',
  },
};

/** Ответ списка журнала в форме конверта, который разворачивает экран. */
export function logsPage(data = [], meta = {}) {
  return {
    ok: true,
    json: () => Promise.resolve({
      success: true,
      data,
      meta: { total: data.length, page: 1, per_page: 20, ...meta },
    }),
  };
}

/** Адреса, с которыми экран сходил за списком журнала. */
export function journalCalls() {
  return apiRequestRaw.mock.calls.map(([url]) => url);
}

/** Последний адрес списка журнала. */
export function lastJournalCall() {
  return journalCalls().at(-1);
}

const mounted = [];

/**
 * Монтирует раздел на заданном отборе в адресе. Переход ждём до монтирования:
 * пока роутер не готов, текущий маршрут пустой, и первая же запись отбора
 * падает с «No match for location».
 * @param {Record<string, string>} query
 */
export async function mountView(query = {}) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/admin/requests', name: 'requests', component: { template: '<div />' } }],
  });
  await router.push({ path: '/admin/requests', query });
  await router.isReady();
  const wrapper = mount(RequestsView, { global: { stubs, plugins: [router] } });
  mounted.push(wrapper);
  return { wrapper, router };
}

/** Отбор, записанный в адресной строке. */
export function currentQuery(router) {
  return { ...router.currentRoute.value.query };
}

/** Размонтирует экраны теста: экран слушает видимость вкладки на общем document. */
export function unmountAll() {
  mounted.splice(0).forEach(wrapper => wrapper.unmount());
}

/** Готовит чистое окружение спеки: пустой стор, моки API без данных. */
export function resetApiMocks() {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  apiRequest.mockResolvedValue({ ok: true, json: () => Promise.resolve([]) });
  apiRequestRaw.mockResolvedValue(logsPage());
}

/** Подменяет видимость вкладки браузера и сообщает об этом странице. */
export function setTabHidden(hidden) {
  Object.defineProperty(document, 'hidden', { configurable: true, get: () => hidden });
  document.dispatchEvent(new Event('visibilitychange'));
}

/** Кнопка панели отбора по подписи. */
export function chip(wrapper, label) {
  const found = wrapper.findAll('button').find(b => b.text() === label);
  expect(found, `на панели есть кнопка «${label}»`).toBeTruthy();
  return found;
}

/**
 * Выбирает пункт выпадающего списка по подписи. Телепортнутое меню живёт в
 * body, поэтому ищем и там: у списка размеров страницы меню вынесено из
 * компонента, иначе его режет подвал таблицы.
 */
export async function pickOption(dropdown, label) {
  await dropdown.get('.base-dropdown__button').trigger('click');
  const inside = dropdown.findAll('.base-dropdown__item').find(o => o.text() === label);
  if (inside) {
    await inside.trigger('click');
  } else {
    const teleported = [...document.body.querySelectorAll('.base-dropdown__item')]
      .find(el => el.textContent.trim() === label);
    expect(teleported, `в списке есть пункт «${label}»`).toBeTruthy();
    teleported.dispatchEvent(new Event('click', { bubbles: true }));
  }
  await flushPromises();
}

/** Выпадающие списки отбора журнала: метод, статус, пользователь. */
export function filterDropdown(wrapper, index) {
  return wrapper.findAll('.filter-dd')[index];
}
