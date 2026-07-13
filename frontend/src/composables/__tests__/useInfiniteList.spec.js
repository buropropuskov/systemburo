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
  });
});
