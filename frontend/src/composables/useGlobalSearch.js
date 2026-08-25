import { ref, shallowRef, onBeforeUnmount } from 'vue';
import { globalSearch } from '@/api/search';

/** Ниже этой длины запрос не уходит: короче трёх символов сервер его и не примет. */
export const MIN_QUERY_LENGTH = 3;
/** Пауза после ввода. Столько же ждут фильтры «Доступных мне». */
const DEBOUNCE_MS = 300;
/** Свежий ответ на тот же запрос переиспользуется: ловит Backspace и повторный набор. */
const CACHE_TTL_MS = 60_000;
const CACHE_MAX_ENTRIES = 20;

/**
 * Сетевая часть сквозного поиска: пауза после ввода, отмена устаревших запросов,
 * короткий кеш ответов.
 *
 * Отмена двухслойная, и оба слоя нужны. AbortController гасит уже улетевший запрос,
 * экономя канал. Порядковый номер защищает от подмены результата: прерывание
 * разрешается асинхронно, и без него медленный ответ на «Рог» мог бы записаться поверх
 * готового ответа на «Роголев».
 */
/**
 * Сколько строк раздела забираем с сервера. Берём его потолок: обрезать выдачу на
 * полпути незачем - панель показывает первые пять и раскрывает остальные на месте,
 * так что найденное доступно целиком, а простыни на экране не возникает.
 */
const SECTION_LIMIT = 20;

export function useGlobalSearch() {
  const groups = shallowRef([]);
  const degraded = ref([]);
  const loading = ref(false);
  const failed = ref(false);
  const lastQuery = ref('');

  const cache = new Map();
  let seq = 0;
  let controller = null;
  let timer = null;

  function readCache(key) {
    const hit = cache.get(key);
    if (!hit) return null;
    if (Date.now() - hit.at > CACHE_TTL_MS) {
      cache.delete(key);
      return null;
    }
    // Перекладываем в конец: порядок вставки Map и есть порядок вытеснения.
    cache.delete(key);
    cache.set(key, hit);
    return hit.data;
  }

  function writeCache(key, data) {
    cache.set(key, { at: Date.now(), data });
    while (cache.size > CACHE_MAX_ENTRIES) {
      cache.delete(cache.keys().next().value);
    }
  }

  function cancel() {
    if (timer) {
      clearTimeout(timer);
      timer = null;
    }
    if (controller) {
      controller.abort();
      controller = null;
    }
  }

  async function run(query) {
    const mySeq = ++seq;
    const key = query.toLowerCase();

    const cached = readCache(key);
    if (cached) {
      groups.value = cached.groups ?? [];
      degraded.value = cached.degraded ?? [];
      loading.value = false;
      failed.value = false;
      return;
    }

    controller = new AbortController();
    loading.value = true;
    failed.value = false;

    try {
      const data = await globalSearch(query, { signal: controller.signal, limit: SECTION_LIMIT });
      if (mySeq !== seq) return; // приехал ответ на устаревший запрос
      groups.value = data.groups ?? [];
      degraded.value = data.degraded ?? [];
      writeCache(key, data);
    } catch (e) {
      if (mySeq !== seq || e?.name === 'AbortError') return; // отменили сами -- не ошибка
      groups.value = [];
      degraded.value = [];
      failed.value = true;
    } finally {
      if (mySeq === seq) loading.value = false;
    }
  }

  /**
   * Запросить выдачу по строке. Короткий запрос очищает результаты, не дёргая сервер.
   * @param {string} raw что ввёл пользователь
   */
  function search(raw) {
    const query = (raw ?? '').trim();
    lastQuery.value = query;
    cancel();

    if (query.length < MIN_QUERY_LENGTH) {
      groups.value = [];
      degraded.value = [];
      loading.value = false;
      failed.value = false;
      return;
    }

    timer = setTimeout(() => {
      timer = null;
      run(query);
    }, DEBOUNCE_MS);
  }

  onBeforeUnmount(cancel);

  // Отдельной очистки кеша нет намеренно: он живёт в замыкании панели, а панель
  // размонтируется при разлогине (v-if по признаку авторизации), унося кеш с собой.
  return { groups, degraded, loading, failed, lastQuery, search, cancel };
}
