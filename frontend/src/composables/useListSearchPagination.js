import { ref, computed, watch } from 'vue';

const DEFAULT_PAGE_SIZE = 50;

/**
 * Поиск + постраничный показ для больших клиентских списков (blank-import E1: импорт
 * бланком может завести до 2000 строк). Рендерить весь массив v-for'ом за раз перестаёт
 * быть дёшево по числу DOM-узлов; окно рендера по scroll не подходит, если список живёт
 * в разных скролл-контекстах на десктопе и мобилке (bounded-контейнер vs скролл страницы
 * карточками) - виртуализация потребовала бы разного root для IntersectionObserver в
 * каждом режиме и не проверяется юнитом (jsdom не считает layout и не умеет
 * IntersectionObserver). Постраничный показ - чистое состояние (страница, строка поиска),
 * одинаково работает в обеих раскладках.
 *
 * Тулбар (поиск+пагинация) показывается только когда список ПОЛНОСТЬЮ (без фильтра)
 * больше порога - обычная ручная подача из нескольких строк выглядит как раньше. Если
 * список уменьшился ниже порога во время активного поиска (например, лишние строки
 * удалили после импорта) - тулбар прячется и вместе с ним сбрасывается searchQuery:
 * иначе таблица продолжает молча фильтровать по старому запросу, а снять фильтр нечем
 * (инпут скрыт вместе с тулбаром).
 *
 * @param {() => Array} getItems геттер исходного массива (обычно `() => props.items`)
 * @param {(item: any, query: string) => boolean} matches предикат поиска; query уже
 *   приведён к trim+lowerCase
 * @param {number} [pageSize=50]
 */
export function useListSearchPagination(getItems, matches, pageSize = DEFAULT_PAGE_SIZE) {
  const searchQuery = ref('');
  const currentPage = ref(1);

  const showToolbar = computed(() => getItems().length > pageSize);

  const filteredItems = computed(() => {
    const items = getItems();
    const q = searchQuery.value.trim().toLowerCase();
    if (!q) return items;
    return items.filter((item) => matches(item, q));
  });

  const totalPages = computed(() => Math.max(1, Math.ceil(filteredItems.value.length / pageSize)));

  const pagedItems = computed(() => {
    const start = (currentPage.value - 1) * pageSize;
    return filteredItems.value.slice(start, start + pageSize).map((item, i) => ({
      item,
      number: start + i + 1,
    }));
  });

  function goToPage(page) {
    currentPage.value = Math.min(Math.max(1, page), totalPages.value);
  }

  // Новый поиск - снова с первой страницы; удаление строки или сужение поиска могли
  // увести currentPage за пределы totalPages - клампим на актуальный максимум.
  watch(searchQuery, () => { currentPage.value = 1; });
  watch(totalPages, (max) => {
    if (currentPage.value > max) currentPage.value = max;
  });
  watch(showToolbar, (visible) => {
    if (!visible) searchQuery.value = '';
  });

  return {
    searchQuery,
    currentPage,
    showToolbar,
    filteredItems,
    totalPages,
    pagedItems,
    goToPage,
  };
}
