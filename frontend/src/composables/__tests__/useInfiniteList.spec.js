import { describe, it, expect, vi } from 'vitest';
import { useInfiniteList } from '../useInfiniteList';

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe('useInfiniteList', () => {
  describe('load / reset', () => {
    it('загружает первую порцию и считает hasMore по total', async () => {
      const list = useInfiniteList({ perPage: 2 });
      const fetchPage = vi.fn().mockResolvedValue({ items: [{ id: 1 }, { id: 2 }], total: 5 });

      await list.load(fetchPage);

      expect(fetchPage).toHaveBeenCalledWith(1, 2);
      expect(list.items.value).toEqual([{ id: 1 }, { id: 2 }]);
      expect(list.total.value).toBe(5);
      expect(list.hasMore.value).toBe(true);
      expect(list.loading.value).toBe(false);
    });

    it('reset затирает накопленное и возвращает страницу на 1', async () => {
      const list = useInfiniteList({ perPage: 2 });
      const fetchPage = vi.fn()
        .mockResolvedValueOnce({ items: [{ id: 1 }, { id: 2 }], total: 4 })
        .mockResolvedValueOnce({ items: [{ id: 3 }, { id: 4 }], total: 4 })
        .mockResolvedValueOnce({ items: [{ id: 9 }], total: 1 });

      await list.load(fetchPage);
      await list.loadMore(fetchPage);
      expect(list.items.value).toHaveLength(4);
      expect(list.page.value).toBe(2);

      await list.reset(fetchPage);
      expect(fetchPage).toHaveBeenLastCalledWith(1, 2);
      expect(list.items.value).toEqual([{ id: 9 }]);
      expect(list.page.value).toBe(1);
      expect(list.hasMore.value).toBe(false);
    });

    it('при ошибке на reset очищает items и пробрасывает исключение', async () => {
      const list = useInfiniteList({ perPage: 2 });
      const fetchPage = vi.fn().mockRejectedValue(new Error('boom'));

      await expect(list.load(fetchPage)).rejects.toThrow('boom');
      expect(list.items.value).toEqual([]);
      expect(list.error.value).toBe(true);
      expect(list.loading.value).toBe(false);
    });
  });

  describe('loadMore', () => {
    it('дописывает вторую порцию в конец, не затирая первую', async () => {
      const list = useInfiniteList({ perPage: 2 });
      const fetchPage = vi.fn()
        .mockResolvedValueOnce({ items: [{ id: 1 }, { id: 2 }], total: 3 })
        .mockResolvedValueOnce({ items: [{ id: 3 }], total: 3 });

      await list.load(fetchPage);
      await list.loadMore(fetchPage);

      expect(fetchPage).toHaveBeenLastCalledWith(2, 2);
      expect(list.items.value).toEqual([{ id: 1 }, { id: 2 }, { id: 3 }]);
      expect(list.page.value).toBe(2);
      expect(list.hasMore.value).toBe(false);
    });

    it('дедуп по id при append: пересекающиеся страницы не создают дублей, hasMore корректен', async () => {
      // Offset-пагинация поверх живых данных: вставка новой заявки сдвигает границы
      // страниц, и последняя строка page1 (id:2) повторяется в page2. Без дедупа был бы
      // дубль (Vue key-warning на :key) и завышенный items.length -> врущий hasMore (#1158).
      const list = useInfiniteList({ perPage: 2 });
      const fetchPage = vi.fn()
        .mockResolvedValueOnce({ items: [{ id: 1 }, { id: 2 }], total: 3 })
        .mockResolvedValueOnce({ items: [{ id: 2 }, { id: 3 }], total: 3 });

      await list.load(fetchPage);
      await list.loadMore(fetchPage);

      // id:2 не задублировался, набор из 3 уникальных.
      expect(list.items.value).toEqual([{ id: 1 }, { id: 2 }, { id: 3 }]);
      // hasMore по числу УНИКАЛЬНЫХ (3) против total (3) - false, а не завышенный 4<3.
      expect(list.hasMore.value).toBe(false);
    });

    it('кастомный keyFn дедупит по своему ключу', async () => {
      const list = useInfiniteList({ perPage: 2, keyFn: (i) => i.uid });
      const fetchPage = vi.fn()
        .mockResolvedValueOnce({ items: [{ uid: 'a' }], total: 2 })
        .mockResolvedValueOnce({ items: [{ uid: 'a' }, { uid: 'b' }], total: 2 });

      await list.load(fetchPage);
      await list.loadMore(fetchPage);

      expect(list.items.value).toEqual([{ uid: 'a' }, { uid: 'b' }]);
    });

    it('не запускает новый запрос, если hasMore=false', async () => {
      const list = useInfiniteList({ perPage: 2 });
      const fetchPage = vi.fn().mockResolvedValue({ items: [{ id: 1 }], total: 1 });

      await list.load(fetchPage);
      expect(list.hasMore.value).toBe(false);

      await list.loadMore(fetchPage);
      expect(fetchPage).toHaveBeenCalledTimes(1);
      expect(list.page.value).toBe(1);
    });

    it('не запускает новый запрос, пока предыдущий ещё грузится', async () => {
      const list = useInfiniteList({ perPage: 2 });
      const first = deferred();
      const fetchPage = vi.fn().mockReturnValueOnce(first.promise);

      const loadPromise = list.load(fetchPage);
      expect(list.loading.value).toBe(true);

      // Список уже "loading" - loadMore не должен пустить второй параллельный запрос.
      const more = list.loadMore(fetchPage);
      expect(fetchPage).toHaveBeenCalledTimes(1);

      first.resolve({ items: [{ id: 1 }, { id: 2 }], total: 10 });
      await loadPromise;
      await more;
    });
  });

  describe('seq-guard устаревшего ответа', () => {
    it('игнорирует резолв устаревшего запроса, если следующий reset ушёл раньше', async () => {
      const list = useInfiniteList({ perPage: 2 });
      const stale = deferred();
      const fetchPage = vi.fn()
        .mockReturnValueOnce(stale.promise)
        .mockResolvedValueOnce({ items: [{ id: 99 }], total: 1 });

      const staleLoad = list.load(fetchPage); // первый запрос - зависает
      const freshLoad = list.load(fetchPage); // второй запрос уходит раньше, чем резолвится первый

      // Резолвим устаревший запрос ПОСЛЕ того, как свежий уже стартовал.
      stale.resolve({ items: [{ id: 1 }, { id: 2 }], total: 5 });

      await staleLoad;
      await freshLoad;

      // Итоговое состояние - от свежего запроса, устаревший ответ не применился.
      expect(list.items.value).toEqual([{ id: 99 }]);
      expect(list.total.value).toBe(1);
    });

    it('игнорирует устаревший error, если свежий запрос уже успешен', async () => {
      const list = useInfiniteList({ perPage: 2 });
      const stale = deferred();
      const fetchPage = vi.fn()
        .mockReturnValueOnce(stale.promise)
        .mockResolvedValueOnce({ items: [{ id: 1 }], total: 1 });

      const staleLoad = list.load(fetchPage).catch(() => {});
      const freshLoad = list.load(fetchPage);

      stale.reject(new Error('устаревшая ошибка'));

      await staleLoad;
      await freshLoad;

      expect(list.error.value).toBe(false);
      expect(list.items.value).toEqual([{ id: 1 }]);
    });
  });

  describe('observeSentinel', () => {
    let originalIO;

    function stubIntersectionObserver() {
      const instances = [];
      class FakeIntersectionObserver {
        constructor(callback, options) {
          this.callback = callback;
          this.options = options;
          this.observed = null;
          this.disconnected = false;
          instances.push(this);
        }

        observe(el) {
          this.observed = el;
        }

        disconnect() {
          this.disconnected = true;
        }

        trigger(isIntersecting = true) {
          this.callback([{ isIntersecting, target: this.observed }]);
        }
      }
      global.IntersectionObserver = FakeIntersectionObserver;
      return instances;
    }

    it('вызывает loadMore при пересечении sentinel', async () => {
      originalIO = global.IntersectionObserver;
      const instances = stubIntersectionObserver();
      try {
        const list = useInfiniteList({ perPage: 2 });
        const fetchPage = vi.fn()
          .mockResolvedValueOnce({ items: [{ id: 1 }, { id: 2 }], total: 4 })
          .mockResolvedValueOnce({ items: [{ id: 3 }, { id: 4 }], total: 4 });

        await list.load(fetchPage);

        const el = {};
        list.observeSentinel(el, fetchPage);
        expect(instances).toHaveLength(1);
        expect(instances[0].observed).toBe(el);

        instances[0].trigger(true);
        await Promise.resolve();
        await Promise.resolve();

        expect(fetchPage).toHaveBeenLastCalledWith(2, 2);
        expect(list.items.value).toHaveLength(4);
      } finally {
        global.IntersectionObserver = originalIO;
      }
    });

    it('переподключение на новый el отключает предыдущий observer', () => {
      originalIO = global.IntersectionObserver;
      const instances = stubIntersectionObserver();
      try {
        const list = useInfiniteList({ perPage: 2 });
        const fetchPage = vi.fn();

        const elA = {};
        const elB = {};
        list.observeSentinel(elA, fetchPage);
        list.observeSentinel(elB, fetchPage);

        expect(instances[0].disconnected).toBe(true);
        expect(instances[1].observed).toBe(elB);
      } finally {
        global.IntersectionObserver = originalIO;
      }
    });

    it('el=null отключает observer без создания нового', () => {
      originalIO = global.IntersectionObserver;
      const instances = stubIntersectionObserver();
      try {
        const list = useInfiniteList({ perPage: 2 });
        const fetchPage = vi.fn();

        list.observeSentinel({}, fetchPage);
        expect(instances).toHaveLength(1);

        list.disconnectObserver();
        expect(instances[0].disconnected).toBe(true);
      } finally {
        global.IntersectionObserver = originalIO;
      }
    });

    it('circuit-breaker (#1173): повторное пересечение зависшего sentinel после ошибки не шлёт новый запрос', async () => {
      // Воспроизводит наблюдавшийся баг: sentinel остаётся видим после упавшего
      // fetchPage (5xx/сеть), и IntersectionObserver продолжает пересекаться -
      // без circuit-breaker'а loadMore звался бы на каждое пересечение.
      originalIO = global.IntersectionObserver;
      const instances = stubIntersectionObserver();
      try {
        const list = useInfiniteList({ perPage: 2 });
        const fetchPage = vi.fn()
          .mockResolvedValueOnce({ items: [{ id: 1 }, { id: 2 }], total: 6 })
          .mockRejectedValueOnce(new Error('502'));

        await list.load(fetchPage);
        const el = {};
        list.observeSentinel(el, fetchPage);

        instances[0].trigger(true);
        await Promise.resolve();
        await Promise.resolve();
        await Promise.resolve();

        expect(list.error.value).toBe(true);
        expect(fetchPage).toHaveBeenCalledTimes(2);

        // Повторные пересечения (типичная картина зависшего скролла) не добавляют
        // третий вызов, пока ошибка активна.
        instances[0].trigger(true);
        instances[0].trigger(true);
        await Promise.resolve();
        await Promise.resolve();

        expect(fetchPage).toHaveBeenCalledTimes(2);
        expect(list.page.value).toBe(1);
      } finally {
        global.IntersectionObserver = originalIO;
      }
    });
  });

  describe('устойчивость к ошибкам бэка (#1173)', () => {
    it('ошибка loadMore не наращивает page, ставит error и включает circuit-breaker', async () => {
      const list = useInfiniteList({ perPage: 2 });
      const fetchPage = vi.fn()
        .mockResolvedValueOnce({ items: [{ id: 1 }, { id: 2 }], total: 6 })
        .mockRejectedValueOnce(new Error('502'));

      await list.load(fetchPage);
      expect(list.page.value).toBe(1);

      await expect(list.loadMore(fetchPage)).rejects.toThrow('502');

      // page НЕ выросла до 2, несмотря на попытку - неудачная страница не коммитится.
      expect(list.page.value).toBe(1);
      expect(list.items.value).toEqual([{ id: 1 }, { id: 2 }]); // прежние данные не тронуты
      expect(list.error.value).toBe(true);
      expect(list.loading.value).toBe(false);
      // hasMore по-прежнему true (сервер отдавал total=6) - сентинел остаётся видим,
      // но canLoadMore (circuit-breaker) гасит автодогрузку.
      expect(list.hasMore.value).toBe(true);
      expect(list.canLoadMore.value).toBe(false);
    });

    it('повторный автовызов loadMore после ошибки не шлёт новый запрос', async () => {
      const list = useInfiniteList({ perPage: 2 });
      const fetchPage = vi.fn()
        .mockResolvedValueOnce({ items: [{ id: 1 }, { id: 2 }], total: 6 })
        .mockRejectedValueOnce(new Error('502'));

      await list.load(fetchPage);
      await list.loadMore(fetchPage).catch(() => {});
      expect(fetchPage).toHaveBeenCalledTimes(2);

      await list.loadMore(fetchPage);
      await list.loadMore(fetchPage);

      expect(fetchPage).toHaveBeenCalledTimes(2);
      expect(list.page.value).toBe(1);
    });

    it('при ошибке на reset очищает items И total, page возвращается на 1', async () => {
      // total тоже обязан сброситься - иначе hasMore врёт "есть ещё" по СТАРОМУ
      // значению, sentinel остаётся видим на пустом списке, и автодогрузка лавиной
      // долбит бэк (корень наблюдавшегося "page 1->36", #1173).
      const list = useInfiniteList({ perPage: 2 });
      const fetchPage = vi.fn()
        .mockResolvedValueOnce({ items: [{ id: 1 }, { id: 2 }], total: 4 })
        .mockResolvedValueOnce({ items: [{ id: 3 }, { id: 4 }], total: 4 })
        .mockRejectedValueOnce(new Error('network'));

      await list.load(fetchPage);
      await list.loadMore(fetchPage);
      expect(list.page.value).toBe(2);

      await expect(list.reset(fetchPage)).rejects.toThrow('network');

      expect(list.items.value).toEqual([]);
      expect(list.total.value).toBe(0);
      expect(list.page.value).toBe(1);
      expect(list.error.value).toBe(true);
      expect(list.hasMore.value).toBe(false); // 0 < 0 - не "протухший" total=4
      expect(list.canLoadMore.value).toBe(false);
    });

    it('retry() сбрасывает error и повторяет ИМЕННО упавшую страницу, накопление продолжается', async () => {
      const list = useInfiniteList({ perPage: 2 });
      const fetchPage = vi.fn()
        .mockResolvedValueOnce({ items: [{ id: 1 }, { id: 2 }], total: 6 })
        .mockRejectedValueOnce(new Error('502'))
        .mockResolvedValueOnce({ items: [{ id: 3 }, { id: 4 }], total: 6 });

      await list.load(fetchPage);
      await list.loadMore(fetchPage).catch(() => {});
      expect(list.error.value).toBe(true);

      await list.retry();

      expect(fetchPage).toHaveBeenLastCalledWith(2, 2); // та же страница, что упала
      expect(list.error.value).toBe(false);
      expect(list.items.value).toEqual([{ id: 1 }, { id: 2 }, { id: 3 }, { id: 4 }]);
      expect(list.page.value).toBe(2);
      expect(list.canLoadMore.value).toBe(true); // автодогрузка возобновилась
    });

    it('retry() после ошибки первичной загрузки повторяет reset этой же страницы', async () => {
      const list = useInfiniteList({ perPage: 2 });
      const fetchPage = vi.fn()
        .mockRejectedValueOnce(new Error('network'))
        .mockResolvedValueOnce({ items: [{ id: 1 }], total: 1 });

      await expect(list.load(fetchPage)).rejects.toThrow('network');
      expect(list.error.value).toBe(true);
      expect(list.items.value).toEqual([]);

      await list.retry();

      expect(fetchPage).toHaveBeenLastCalledWith(1, 2);
      expect(list.error.value).toBe(false);
      expect(list.items.value).toEqual([{ id: 1 }]);
      expect(list.page.value).toBe(1);
    });

    it('seq-guard: устаревший error не выставляется, если свежий запрос уже успешен (регресс на #1173)', async () => {
      const list = useInfiniteList({ perPage: 2 });
      const stale = deferred();
      const fetchPage = vi.fn()
        .mockReturnValueOnce(stale.promise)
        .mockResolvedValueOnce({ items: [{ id: 1 }], total: 1 });

      const staleLoad = list.load(fetchPage).catch(() => {});
      const freshLoad = list.load(fetchPage);

      stale.reject(new Error('устаревшая ошибка'));

      await staleLoad;
      await freshLoad;

      expect(list.error.value).toBe(false);
      expect(list.canLoadMore.value).toBe(false); // hasMore=false (1<1), не из-за error
      expect(list.items.value).toEqual([{ id: 1 }]);
    });
  });
});
