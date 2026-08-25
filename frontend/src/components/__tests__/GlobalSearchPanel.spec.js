import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import GlobalSearchPanel from '../GlobalSearchPanel.vue';
import { globalSearch } from '@/api/search';
import { useOnboardingStore } from '@/stores/onboarding';

// Панель показывает найденное, ввод идёт в поле меню и приходит сюда строкой. Проверяем
// то, что ломается незаметно: разделы находятся без обращения к серверу, пустая строка
// закрывает панель, а переход закрывает её раньше навигации -- иначе подтверждение о
// несохранённой форме окажется под панелью.

vi.mock('@/api/search', () => ({ globalSearch: vi.fn() }));
// settingsAllowed управляет только ключом page.admin.settings - остальные разделы
// остаются доступны всем прочим тестам файла без явной настройки.
let settingsAllowed = true;
vi.mock('@/composables/usePermission', () => ({
  usePermission: () => ({ can: (key) => (key === 'page.admin.settings' ? settingsAllowed : true) }),
}));

const push = vi.fn();

function mountPanel(query = '', show = true) {
  return mount(GlobalSearchPanel, {
    props: { show, query },
    global: {
      mocks: { $router: { push } },
      stubs: {
        NavIcon: true,
        SkeletonLine: true,
        teleport: true,
      },
    },
    attachTo: document.body,
  });
}

let wrapper;

beforeEach(() => {
  setActivePinia(createPinia());
  vi.useFakeTimers();
  localStorage.clear();
  globalSearch.mockResolvedValue({ groups: [], total: 0 });
  settingsAllowed = true;
});

afterEach(() => {
  vi.useRealTimers();
  wrapper?.unmount();
  vi.clearAllMocks();
});

describe('GlobalSearchPanel', () => {
  it('закрытая панель не рисуется', () => {
    wrapper = mountPanel('', false);

    expect(wrapper.find('.gsp').exists()).toBe(false);
  });

  it('открывается без запроса и подсказывает, что можно искать', () => {
    wrapper = mountPanel('');

    expect(wrapper.find('.gsp').exists()).toBe(true);
    expect(wrapper.text()).toContain('начните вводить');
  });

  it('ввод идёт в самой панели', async () => {
    wrapper = mountPanel('');

    await wrapper.find('.gsp__input').setValue('Роголев');

    expect(wrapper.emitted('update:query')[0]).toEqual(['Роголев']);
  });

  it('находит раздел меню без обращения к серверу', async () => {
    wrapper = mountPanel('Автомоб');
    await flushPromises();

    const titles = wrapper.findAll('.gsp__row-title').map((el) => el.text());
    expect(titles).toContain('Автомобили');
    expect(globalSearch).not.toHaveBeenCalled();
  });

  it('короткий запрос не уходит на сервер', async () => {
    wrapper = mountPanel('ро');
    vi.advanceTimersByTime(1000);
    await flushPromises();

    expect(globalSearch).not.toHaveBeenCalled();
  });

  it('запрос уходит один раз после паузы, а не на каждый символ', async () => {
    wrapper = mountPanel('Рог');
    await wrapper.setProps({ query: 'Рогол' });
    await wrapper.setProps({ query: 'Роголев' });
    vi.advanceTimersByTime(400);
    await flushPromises();

    expect(globalSearch).toHaveBeenCalledTimes(1);
    expect(globalSearch).toHaveBeenCalledWith('Роголев', expect.anything());
  });

  it('показывает группы, пришедшие с сервера', async () => {
    globalSearch.mockResolvedValue({
      groups: [{
        type: 'employees',
        title: 'Сотрудники',
        count: 1,
        items: [{ id: 7, type: 'employees', title: 'Роголев Иван', subtitle: 'водитель', target: { entity: 'unique_employee', id: 7 } }],
      }],
      total: 1,
    });
    wrapper = mountPanel('Роголев');
    vi.advanceTimersByTime(400);
    await flushPromises();

    expect(wrapper.text()).toContain('Сотрудники');
    expect(wrapper.text()).toContain('Роголев Иван');
  });

  it('сообщает о разделе, который не ответил', async () => {
    globalSearch.mockResolvedValue({ groups: [], total: 0, degraded: ['applications'] });
    wrapper = mountPanel('Роголев');
    vi.advanceTimersByTime(400);
    await flushPromises();

    expect(wrapper.text()).toContain('Заявки');
  });

  it('закреплённая панель переживает переход по находке', async () => {
    wrapper = mountPanel('Автомоб');
    await flushPromises();

    await wrapper.find('[aria-label="Закрепить панель"]').trigger('click');
    await wrapper.find('.gsp__row').trigger('click');
    await flushPromises();

    expect(push).toHaveBeenCalled();
    expect(wrapper.emitted('close')).toBeFalsy();
  });

  it('закрепление запоминается между заходами', async () => {
    wrapper = mountPanel('Автомоб');
    await flushPromises();
    await wrapper.find('[aria-label="Закрепить панель"]').trigger('click');
    wrapper.unmount();

    wrapper = mountPanel('Автомоб');
    await flushPromises();

    expect(wrapper.find('[aria-label="Открепить панель"]').exists()).toBe(true);
  });

  it('свёрнутая панель показывает столбик с числом найденного', async () => {
    wrapper = mountPanel('Автомоб');
    await flushPromises();

    await wrapper.find('[aria-label="Свернуть в столбик"]').trigger('click');

    expect(wrapper.find('.gsp--collapsed').exists()).toBe(true);
    // По «Автомоб» находятся и раздел «Автомобили», и действие «Добавить автомобиль».
    expect(wrapper.find('.gsp__strip-count').text()).toBe('2');
    // Список в столбике не показывается, иначе он не столбик.
    expect(wrapper.find('.gsp__row').exists()).toBe(false);
  });

  it('новый запрос разворачивает свёрнутую панель', async () => {
    wrapper = mountPanel('Автомоб');
    await flushPromises();
    await wrapper.find('[aria-label="Свернуть в столбик"]').trigger('click');
    expect(wrapper.find('.gsp--collapsed').exists()).toBe(true);

    await wrapper.setProps({ query: 'Сотрудн' });
    await flushPromises();

    expect(wrapper.find('.gsp--collapsed').exists()).toBe(false);
  });

  it('переход сворачивает панель, но не закрывает её', async () => {
    wrapper = mountPanel('Автомоб');
    await flushPromises();

    await wrapper.find('.gsp__row').trigger('click');
    await flushPromises();

    expect(push).toHaveBeenCalledWith({ path: '/carsview' });
    // Запрос и найденное остаются: вернуться к ним можно одним нажатием на столбик.
    expect(wrapper.emitted('close')).toBeFalsy();
    expect(wrapper.find('.gsp--collapsed').exists()).toBe(true);
  });

  it('крестик закрывает совсем -- это единственный способ', async () => {
    wrapper = mountPanel('Автомоб');
    await flushPromises();

    await wrapper.find('[aria-label="Закрыть результаты"]').trigger('click');

    expect(wrapper.emitted('close')).toBeTruthy();
  });

  it('действие находится по обиходному слову, а не только по названию', async () => {
    wrapper = mountPanel('подать');
    await flushPromises();

    const titles = wrapper.findAll('.gsp__row-title').map((el) => el.text());
    expect(titles).toContain('Подать заявку');
    // Действия идут первыми: это намерение, а не место, где слово встречается.
    expect(titles[0]).toBe('Подать заявку');
  });

  it('действие ведёт на страницу оформления', async () => {
    wrapper = mountPanel('отправить');
    await flushPromises();

    await wrapper.find('.gsp__row').trigger('click');
    await flushPromises();

    expect(push).toHaveBeenCalledWith({ path: '/new-application' });
  });

  // #7: «Настройки» гейтится точечным ключом page.admin.settings (не super-only),
  // тем же приёмом, что и любой другой раздел Админки - без личного deny-override
  // раздел найдётся, с ним - нет.
  it('администратор с личным deny-override: раздел «Настройки» не находится поиском', async () => {
    settingsAllowed = false;
    wrapper = mountPanel('Настро');
    await flushPromises();

    const titles = wrapper.findAll('.gsp__row-title').map((el) => el.text());
    expect(titles).not.toContain('Настройки');
  });

  it('администратор с правом page.admin.settings: раздел «Настройки» находится поиском', async () => {
    settingsAllowed = true;
    wrapper = mountPanel('Настро');
    await flushPromises();

    const titles = wrapper.findAll('.gsp__row-title').map((el) => el.text());
    expect(titles).toContain('Настройки');
  });

  /**
   * Онбординг раскрывает панель сам (reveal.open: 'search-panel') и рассказывает про
   * неё отдельным шагом. Окно шага driver.js лежит вне панели, поэтому клик по нему
   * приходил в общий обработчик «мимо панели»: панель сворачивалась в столбик,
   * подсветка слетала, а вырез в затемнении оставался висеть на пустом месте.
   */
  it('панель, раскрытая туром, не сворачивается кликом мимо неё', async () => {
    wrapper = mountPanel('Автомоб');
    await flushPromises();
    useOnboardingStore().setRevealOpen('search-panel');

    document.body.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    await flushPromises();

    expect(wrapper.find('.gsp--collapsed').exists()).toBe(false);
  });

  it('без сигнала тура клик мимо сворачивает панель как прежде', async () => {
    wrapper = mountPanel('Автомоб');
    await flushPromises();
    useOnboardingStore().setRevealOpen(null);

    document.body.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    await flushPromises();

    expect(wrapper.find('.gsp--collapsed').exists()).toBe(true);
  });

  it('тур раскрыл другой узел - панель поиска ведёт себя обычно', async () => {
    wrapper = mountPanel('Автомоб');
    await flushPromises();
    useOnboardingStore().setRevealOpen('admin-column');

    document.body.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    await flushPromises();

    expect(wrapper.find('.gsp--collapsed').exists()).toBe(true);
  });

  // #1097 W4.1: на десктопе прозрачность подпёрта размытием, а на мобилке размытия нет
  // (backdrop-filter рвёт кадры при выезде, #1201) - и текст страницы читался сквозь
  // список находок. Проверяем по исходнику: scoped-CSS в jsdom не применяется.
  it('на мобилке подложка панели почти глухая - находки не сливаются со страницей', () => {
    const sfc = readFileSync(resolve(__dirname, '../GlobalSearchPanel.vue'), 'utf8');
    const mobileBlock = sfc.match(/@media\s*\(max-width:\s*768px\)\s*\{([\s\S]*?)\n\}/);
    expect(mobileBlock).not.toBeNull();

    const surface = mobileBlock[1].match(/background:\s*color-mix\([^)]*var\(--surface\)\s*(\d+)%/);
    expect(surface).not.toBeNull();
    expect(Number(surface[1])).toBeGreaterThanOrEqual(95);
  });
});

/**
 * Выдача читается без знания устройства поиска: у раздела написано, сколько в нём
 * нашлось, видно первые пять, а остальные раскрываются на месте - уходить со
 * страницы за собственными результатами не нужно.
 */
describe('GlobalSearchPanel - сколько нашлось и где остальное', () => {
  const users = (n) => ({
    groups: [{
      type: 'users',
      title: 'Пользователи',
      count: n,
      items: Array.from({ length: n }, (_, i) => ({
        id: i + 1, type: 'users', title: `Шумилин ${i + 1}`, subtitle: '@user', target: { entity: 'user', id: i + 1 },
      })),
    }],
    total: n,
  });

  const showResults = async (n) => {
    globalSearch.mockResolvedValue(users(n));
    wrapper = mountPanel('Шумилин');
    vi.advanceTimersByTime(400);
    await flushPromises();
  };

  it('раздел говорит, сколько в нём нашлось', async () => {
    await showResults(12);

    expect(wrapper.find('.gsp__group-count').text()).toBe('12');
  });

  it('сразу показаны первые пять, остальные - за кнопкой с числом', async () => {
    await showResults(12);

    expect(wrapper.findAll('.gsp__row')).toHaveLength(5);
    expect(wrapper.find('[data-testid="global-search-expand"]').text()).toContain('7');
  });

  it('раскрытие показывает остальные тут же, без перехода', async () => {
    await showResults(12);

    await wrapper.find('[data-testid="global-search-expand"]').trigger('click');
    await flushPromises();

    expect(wrapper.findAll('.gsp__row')).toHaveLength(12);
    expect(wrapper.find('[data-testid="global-search-expand"]').exists()).toBe(false);
    expect(push).not.toHaveBeenCalled();
  });

  it('когда результатов мало, ни счётчика лишнего, ни кнопки', async () => {
    await showResults(1);

    expect(wrapper.find('.gsp__group-count').exists()).toBe(false);
    expect(wrapper.find('[data-testid="global-search-expand"]').exists()).toBe(false);
  });

  it('новый запрос сворачивает раскрытое обратно', async () => {
    await showResults(12);
    await wrapper.find('[data-testid="global-search-expand"]').trigger('click');
    await flushPromises();
    expect(wrapper.findAll('.gsp__row')).toHaveLength(12);

    globalSearch.mockResolvedValue(users(9));
    await wrapper.setProps({ query: 'Шумил' });
    vi.advanceTimersByTime(400);
    await flushPromises();

    expect(wrapper.findAll('.gsp__row')).toHaveLength(5);
  });
});
