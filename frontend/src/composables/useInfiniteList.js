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
 * Устойчивость к ошибкам бэка (issue #1173): ошибка fetchPage (5xx/сеть) НЕ
 * наращивает page и включает circuit-breaker - canLoadMore становится false, пока
 * ошибка активна, и IntersectionObserver/циклы полной подгрузки потребителей
 * перестают автодогружать. Ручной retry() повторяет именно упавшую страницу.
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
  // Последний вызванный fetchPage/опции load - нужны retry() (#1173), чтобы повторить
  // ИМЕННО упавшую страницу тем же режимом (reset/append), не запрашивая её заново
  // у потребителя.
  let lastFetchPage = null;
  let lastLoadOpts = null;

  // hasMore считаем по числу УНИКАЛЬНЫХ элементов (items уже дедуплицированы при
  // append) - offset-пагинация поверх живых данных может отдать пересекающиеся id
  // между страницами (вставка новой заявки сдвигает границы страниц), и без дедупа
  // items.length завышался бы, а hasMore врал (#1158). hasMore намеренно НЕ учитывает
  // error - это чисто "остались ли на сервере ещё страницы", им гейтится видимость
  // sentinel-контейнера у потребителей (внутри него же рисуется error+retry, #1173).
  const hasMore = computed(() => items.value.length < total.value);

  // canLoadMore (#1173) - circuit-breaker для АВТОдогрузки (IntersectionObserver,
  // циклы полной подгрузки потребителей). Пока активна ошибка - автодогрузка стоит,
  // без этого зависший sentinel мог бы лавиной долбить упавший бэк (page 1->36).
  // Возобновляется, как только error гасится успешным load()/retry().
  const canLoadMore = computed(() => hasMore.value && !error.value);

  /**
   * @param {(page: number, perPage: number) => Promise<{items: object[], total: number}>} fetchPage
   * @param {{reset?: boolean}} [opts]
   */
  async function load(fetchPage, { reset = true } = {}) {
    // Страница, которую реально запрашиваем: append (reset=false) метит следующую
    // после последней успешно закоммиченной. page.value коммитится только на успехе
    // (см. ниже) - неудачная попытка не "накручивает" номер страницы (#1173).
    const requestedPage = reset ? 1 : page.value + 1;
    loading.value = true;
    error.value = false;
    lastFetchPage = fetchPage;
    lastLoadOpts = { reset };
    const mySeq = ++seq;
    try {
      const result = await fetchPage(requestedPage, perPage);
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
      page.value = requestedPage;
    } catch (err) {
      if (mySeq !== seq) return;
      error.value = true;
      if (reset) {
        items.value = [];
        page.value = 1;
        // total тоже сбрасываем (#1173): иначе он остаётся СТАРЫМ значением от
        // предыдущего успешного набора, hasMore врёт "есть ещё" по пустому списку -
        // именно так sentinel оставался видимым после упавшей ПЕРВИЧНОЙ загрузки и
        // автодогрузка лавиной долбила бэк (наблюдавшийся page 1->36).
        total.value = 0;
      }
      // append (reset=false): page.value/total НЕ трогаем - остаются на последней
      // успешно закоммиченной странице, иначе неудачная догрузка "накрутила" бы номер
      // и retry() бил бы по несуществующей странице (#1173).
      throw err;
    } finally {
      if (mySeq === seq) loading.value = false;
    }
  }

  function reset(fetchPage) {
    return load(fetchPage, { reset: true });
  }

  function loadMore(fetchPage) {
    if (loading.value || !canLoadMore.value) return Promise.resolve();
    return load(fetchPage, { reset: false });
  }

  /**
   * Ручной повтор запроса, который последним завершился ошибкой (#1173): та же
   * страница и тот же режим (reset/append), что и упавшая попытка - retry не
   * принимает fetchPage заново, переиспользует запомненный из последнего load().
   * Сбрасывает error и, при успехе, снимает circuit-breaker - автодогрузка
   * (observer/loadAllRemaining потребителей) возобновляется.
   */
  function retry() {
    if (loading.value || !lastFetchPage) return Promise.resolve();
    return load(lastFetchPage, lastLoadOpts);
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
        // canLoadMore гейтит и здесь (#1173, circuit-breaker): пока активна ошибка,
        // повторные пересечения зависшего sentinel не шлют запрос заново - без этого
        // бэк лавиной долбился бы (page 1->36) на каждое пересечение.
        if (entry.isIntersecting && sentinelFetchPage && canLoadMore.value) {
          // Ошибку уже фиксирует error.value - здесь только гасим unhandled rejection,
          // повторная попытка - через retry() (ручной) или следующее пересечение
          // после того, как error снят.
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
    canLoadMore,
    load,
    reset,
    loadMore,
    retry,
    observeSentinel,
    disconnectObserver,
  };
}
