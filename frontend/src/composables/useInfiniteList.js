import { ref, computed } from 'vue';

/**
 * Бесшовная порционная подгрузка списка (issue #1158): аккумулирует "страницы"
 * сервера, отдаёт hasMore по meta.total, и умеет автодогрузку через
 * IntersectionObserver на sentinel-элементе внизу списка (без кнопки "Показать ещё").
 *
 * fetchPage передаётся не при создании, а при каждом вызове load/reset/loadMore/
 * observeSentinel - это позволяет подключать composable к Options API компонентам
 * (setup() не имеет доступа к this, а fetchPage обычно строится из полей формы/
 * фильтров компонента, доступных только в methods).
 *
 * @param {{perPage?: number, keyFn?: (item: object) => (string|number)}} [options]
 */
export function useInfiniteList({ perPage = 30, keyFn = (item) => item.id } = {}) {
  const items = ref([]);
  const total = ref(0);
  const page = ref(1);
  const loading = ref(false);
  const error = ref(false);

  // seq-guard (#632): смена фильтра/поиска до резолва предыдущего запроса не должна
  // затереть актуальные данные устаревшим ответом.
  let seq = 0;
  let observer = null;
  let sentinelFetchPage = null;

  // hasMore считаем по числу УНИКАЛЬНЫХ элементов (items уже дедуплицированы при
  // append) - offset-пагинация поверх живых данных может отдать пересекающиеся id
  // между страницами (вставка новой заявки сдвигает границы страниц), и без дедупа
  // items.length завышался бы, а hasMore врал (#1158).
  const hasMore = computed(() => items.value.length < total.value);

  /**
   * @param {(page: number, perPage: number) => Promise<{items: object[], total: number}>} fetchPage
   * @param {{reset?: boolean}} [opts]
   */
  async function load(fetchPage, { reset = true } = {}) {
    if (reset) page.value = 1;
    loading.value = true;
    error.value = false;
    const mySeq = ++seq;
    try {
      const result = await fetchPage(page.value, perPage);
      if (mySeq !== seq) return; // устарел - актуальный запрос уже идёт
      const data = (result && result.items) || [];
      if (reset) {
        items.value = data;
      } else {
        // Append с дедупом по ключу: пропускаем элементы, чьи id уже накоплены,
        // иначе дубль -> Vue key-warning на :key и завышенный hasMore (#1158).
        const known = new Set(items.value.map(keyFn));
        const fresh = data.filter((item) => !known.has(keyFn(item)));
        items.value = [...items.value, ...fresh];
      }
      total.value = (result && result.total) || 0;
    } catch (err) {
      if (mySeq !== seq) return;
      error.value = true;
      if (reset) items.value = [];
      throw err;
    } finally {
      if (mySeq === seq) loading.value = false;
    }
  }

  function reset(fetchPage) {
    return load(fetchPage, { reset: true });
  }

  function loadMore(fetchPage) {
    if (loading.value || !hasMore.value) return Promise.resolve();
    page.value += 1;
    return load(fetchPage, { reset: false });
  }

  /**
   * Подключает IntersectionObserver к sentinel-элементу внизу списка: пересечение
   * вызывает loadMore(fetchPage) без кнопки. Повторный вызов (смена el при v-if)
   * переподключает observer; el=null (элемент размонтирован) просто отключает.
   * options.root обычно нужен явно - у вложенных скролл-контейнеров (не документа)
   * дефолтный root (viewport) не заметит пересечение.
   * @param {Element|null} el
   * @param {(page: number, perPage: number) => Promise<{items: object[], total: number}>} fetchPage
   * @param {IntersectionObserverInit} [options]
   */
  function observeSentinel(el, fetchPage, options = {}) {
    disconnectObserver();
    sentinelFetchPage = fetchPage;
    if (!el || typeof IntersectionObserver === 'undefined') return;
    observer = new IntersectionObserver((entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting && sentinelFetchPage) {
          // Ошибку уже фиксирует error.value - здесь только гасим unhandled rejection,
          // повторная попытка произойдёт на следующем пересечении (ре-скролл).
          loadMore(sentinelFetchPage).catch(() => {});
        }
      });
    }, { threshold: 0, rootMargin: '200px', ...options });
    observer.observe(el);
  }

  function disconnectObserver() {
    if (observer) {
      observer.disconnect();
      observer = null;
    }
  }

  return {
    items,
    total,
    page,
    loading,
    error,
    hasMore,
    load,
    reset,
    loadMore,
    observeSentinel,
    disconnectObserver,
  };
}
