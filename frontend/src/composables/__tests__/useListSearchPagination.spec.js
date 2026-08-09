import { describe, it, expect } from 'vitest';
import { nextTick, ref } from 'vue';
import { useListSearchPagination } from '../useListSearchPagination';

function makeItems(n) {
  return Array.from({ length: n }, (_, i) => ({ id: i + 1, name: `Имя${i + 1}` }));
}

function byName(item, q) {
  return item.name.toLowerCase().includes(q);
}

describe('useListSearchPagination', () => {
  it('делит список на страницы по 50 и нумерует строки сквозным номером', () => {
    const items = ref(makeItems(120));
    const { pagedItems, totalPages, currentPage, goToPage } = useListSearchPagination(() => items.value, byName);

    expect(totalPages.value).toBe(3);
    expect(pagedItems.value.length).toBe(50);
    expect(pagedItems.value[0].number).toBe(1);

    goToPage(2);
    expect(currentPage.value).toBe(2);
    expect(pagedItems.value[0].number).toBe(51);
    expect(pagedItems.value[0].item.name).toBe('Имя51');
  });

  it('goToPage клампит запрошенную страницу в границы [1, totalPages]', () => {
    const items = ref(makeItems(120));
    const { currentPage, goToPage } = useListSearchPagination(() => items.value, byName);

    goToPage(999);
    expect(currentPage.value).toBe(3);

    goToPage(-5);
    expect(currentPage.value).toBe(1);
  });

  it('поиск фильтрует список и сбрасывает текущую страницу на первую', async () => {
    const items = ref(makeItems(120));
    const { searchQuery, filteredItems, currentPage, goToPage } = useListSearchPagination(() => items.value, byName);

    goToPage(3);
    searchQuery.value = 'Имя119';
    await nextTick();

    expect(filteredItems.value.length).toBe(1);
    expect(filteredItems.value[0].name).toBe('Имя119');
    expect(currentPage.value).toBe(1);
  });

  it('список сузился без поиска - currentPage клампится на новый totalPages', async () => {
    const items = ref(makeItems(120));
    const { currentPage, totalPages, goToPage } = useListSearchPagination(() => items.value, byName);

    goToPage(3);
    items.value = makeItems(10);
    await nextTick();

    expect(totalPages.value).toBe(1);
    expect(currentPage.value).toBe(1);
  });

  it('showToolbar включается только когда исходный список ПОЛНОСТЬЮ (без фильтра) больше порога', () => {
    const items = ref(makeItems(50));
    const { showToolbar } = useListSearchPagination(() => items.value, byName);
    expect(showToolbar.value).toBe(false);

    items.value = makeItems(51);
    expect(showToolbar.value).toBe(true);
  });

  // Регресс, ради которого список переводили на composable: активный поиск сузил
  // видимую часть, а исходный список ЦЕЛИКОМ упал ниже порога тулбара (например,
  // после импорта бланком лишние строки удалили) - тулбар прячется, и вместе с ним
  // обязан сброситься searchQuery, иначе фильтр продолжает молча действовать без
  // видимого инпута, чтобы его снять.
  it('список уменьшился ниже порога тулбара во время активного поиска - сбрасывает searchQuery', async () => {
    const items = ref(makeItems(120));
    const { searchQuery, showToolbar, filteredItems } = useListSearchPagination(() => items.value, byName);

    searchQuery.value = 'Имя119';
    expect(showToolbar.value).toBe(true);
    expect(filteredItems.value.length).toBe(1);

    items.value = makeItems(10);
    await nextTick();

    expect(showToolbar.value).toBe(false);
    expect(searchQuery.value).toBe('');
    expect(filteredItems.value.length).toBe(10);
  });
});
