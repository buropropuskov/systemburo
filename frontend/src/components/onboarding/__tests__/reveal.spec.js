import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createPinia, setActivePinia } from 'pinia';
import bus from '@/eventBus';
import {
  resolveReveal,
  applyReveal,
  restoreReveal,
  isNavDrawerOpen,
  OPEN_TARGETS,
} from '../reveal';
import { securityOnboardingSteps } from '../securityOnboardingSteps';
import { onboardingSteps } from '../onboardingSteps';
import { useOnboardingStore } from '@/stores/onboarding';

/**
 * Раскрытие свёрнутых целей тура. Две оси:
 * `mobile` - бургер-drawer на <=768px (#1097 S11): рельс уезжает transform'ом за
 * экран, оставаясь в DOM "видимым" для waitForElement, поэтому drawer открываем
 * ПЕРЕД подсветкой. DOM/$bus мокаем поведением NavMenu (burger -> класс drawer).
 * `open` - узел, свёрнутый на любой ширине; резолвер поднимает сигнал в сторе, а
 * владелец узла реагирует сам, поэтому здесь проверяем именно сигнал.
 */
describe('reveal', () => {
  const originalWidth = window.innerWidth;
  let navHandler;

  function setViewport(w) {
    window.innerWidth = w;
  }

  beforeEach(() => {
    setActivePinia(createPinia());
    document.body.innerHTML = `
      <nav data-testid="ob-nav-rail" class="nav-menu"></nav>
    `;
    const nav = document.querySelector('[data-testid="ob-nav-rail"]');
    // Эмуляция NavMenu: $bus mobile-nav-toggle -> тоггл класса drawer.
    navHandler = () => nav.classList.toggle('nav-menu--mobile-open');
    bus.on('mobile-nav-toggle', navHandler);
  });

  afterEach(() => {
    bus.off('mobile-nav-toggle', navHandler);
    document.body.innerHTML = '';
    window.innerWidth = originalWidth;
    vi.useRealTimers();
  });

  const idxOf = (steps, id) => steps.findIndex((s) => s.id === id);

  /**
 * Порядок «сначала переход, потом сворачивание».
 *
 * Подготовка следующего шага раскрывает то, что ему нужно, но НЕ сворачивает узел
 * текущего: между нажатием стрелки и сменой шага проходит около 400 мс, и всё это
 * время человек смотрит на прежнее окно шага. Прежде список уведомлений гас сразу
 * по нажатию - на экране оставались подсветка пустого места и подпись «Список
 * уведомлений» (замечание владельца 21.08, снято из его же лога:
 * `10.293 reveal=notifications шаг 3` -> `10.489 reveal=null шаг 3` ->
 * `10.688 панель=false шаг 3` -> `10.889 шаг 4`).
 */
describe('applyReveal - раскрытие без преждевременного сворачивания', () => {
  const steps = [
    { id: 'bell', route: '/news' },
    { id: 'panel', route: '/news', reveal: { open: 'notifications' } },
    { id: 'search', route: '/news' },
    { id: 'search-panel', route: '/news', reveal: { open: 'search-panel' } },
  ];

  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it('подготовка шага без раскрытия не гасит узел предыдущего', async () => {
    const store = useOnboardingStore();
    store.setRevealOpen('notifications');

    await applyReveal(steps, 2, { closeOthers: false });

    expect(store.revealOpen).toBe('notifications');
  });

  it('подсветка того же шага гасит его, когда переход состоялся', async () => {
    const store = useOnboardingStore();
    store.setRevealOpen('notifications');

    await applyReveal(steps, 2);

    expect(store.revealOpen).toBe(null);
  });

  it('подготовка шага в хосте зовёт мягкий режим - иначе узел гаснет до перехода', () => {
    const host = readFileSync(
      resolve(dirname(fileURLToPath(import.meta.url)), '../OnboardingTour.vue'), 'utf8');
    expect(host).toMatch(/const revealed = await applyReveal\(store\.steps, globalIndex, \{ closeOthers: false \}\)/);
  });

  it('подготовка шага с раскрытием ставит свой узел и в мягком режиме', async () => {
    const store = useOnboardingStore();
    store.setRevealOpen('notifications');

    await applyReveal(steps, 3, { closeOthers: false });

    expect(store.revealOpen).toBe('search-panel');
  });
});

describe('resolveReveal (чистая логика)', () => {
    it('reveal текущего шага имеет приоритет: рельс -> mobile nav', () => {
      const iR = idxOf(securityOnboardingSteps, 'sec-nav-rail');
      expect(resolveReveal(securityOnboardingSteps, iR)).toEqual({ mobile: 'nav', open: null });
    });

    it('шаг колокольчика reveal НЕ несёт (вынесен в шапку, #1097 W3.2)', () => {
      const iApp = idxOf(onboardingSteps, 'header-notifications');
      const iSec = idxOf(securityOnboardingSteps, 'sec-header-notifications');
      expect(onboardingSteps[iApp].reveal).toBeUndefined();
      expect(securityOnboardingSteps[iSec].reveal).toBeUndefined();
      expect(resolveReveal(onboardingSteps, iApp)).toEqual({ mobile: null, open: null });
      expect(resolveReveal(securityOnboardingSteps, iSec)).toEqual({ mobile: null, open: null });
    });

    it('шаг без reveal между РАЗНЫМИ соседями ничего не наследует', () => {
      // header-submit стоит между header-notifications (без reveal) и nav-rail (nav):
      // соседи разного типа -> drawer закрыт, чужой reveal не наследуется.
      const i = idxOf(onboardingSteps, 'header-submit');
      expect(onboardingSteps[i].reveal).toBeUndefined();
      expect(resolveReveal(onboardingSteps, i).mobile).toBe(null);
    });

    it('шаг без reveal ВНУТРИ группы одинаковых удерживает раскрытие', () => {
      const steps = [
        { id: 'a', route: '/x', reveal: { open: 'admin-column' } },
        { id: 'b', route: '/x' },
        { id: 'c', route: '/x', reveal: { open: 'admin-column' } },
      ];
      expect(resolveReveal(steps, 1).open).toBe('admin-column');
    });

    it('соседи с РАЗНЫМИ open не удерживают раскрытие', () => {
      const steps = [
        { id: 'a', route: '/x', reveal: { open: 'admin-column' } },
        { id: 'b', route: '/x' },
        { id: 'c', route: '/x', reveal: { open: 'search-panel' } },
      ];
      expect(resolveReveal(steps, 1).open).toBe(null);
    });

    it('оси независимы: шаг может просить и drawer, и раскрытие узла', () => {
      const steps = [{ id: 'a', route: '/x', reveal: { mobile: 'nav', open: 'search-panel' } }];
      expect(resolveReveal(steps, 0)).toEqual({ mobile: 'nav', open: 'search-panel' });
    });

    it('center-модал старта без reveal', () => {
      const i = idxOf(securityOnboardingSteps, 'sec-start');
      expect(resolveReveal(securityOnboardingSteps, i)).toEqual({ mobile: null, open: null });
    });

    it('несуществующий индекс не роняет резолвер', () => {
      expect(resolveReveal(onboardingSteps, 999)).toEqual({ mobile: null, open: null });
      expect(resolveReveal(undefined, 0)).toEqual({ mobile: null, open: null });
    });
  });

  describe('applyReveal - drawer на мобилке (<=768)', () => {
    it('sec-nav-rail: drawer открыт', async () => {
      setViewport(390);
      const i = idxOf(securityOnboardingSteps, 'sec-nav-rail');
      vi.useFakeTimers();
      const p = applyReveal(securityOnboardingSteps, i);
      await vi.advanceTimersByTimeAsync(300);
      await p;
      expect(isNavDrawerOpen()).toBe(true);
    });

    it('sec-header-notifications: drawer ЗАКРЫТ - шаг колокольчика', async () => {
      setViewport(390);
      // предусловие: drawer открыт (как после предыдущего nav-шага)
      document.querySelector('[data-testid="ob-nav-rail"]').classList.add('nav-menu--mobile-open');
      const i = idxOf(securityOnboardingSteps, 'sec-header-notifications');
      await applyReveal(securityOnboardingSteps, i);
      expect(isNavDrawerOpen()).toBe(false);
    });

    it('переход nav -> без reveal закрывает drawer', async () => {
      setViewport(390);
      const iR = idxOf(securityOnboardingSteps, 'sec-nav-rail');
      const iN = idxOf(securityOnboardingSteps, 'sec-header-notifications');

      vi.useFakeTimers();
      const p = applyReveal(securityOnboardingSteps, iR);
      await vi.advanceTimersByTimeAsync(300);
      await p;
      expect(isNavDrawerOpen()).toBe(true);

      await applyReveal(securityOnboardingSteps, iN);
      expect(isNavDrawerOpen()).toBe(false);
    });

    it('на 768 (граница CSS max-width:768px) reveal активен', async () => {
      setViewport(768);
      const i = idxOf(securityOnboardingSteps, 'sec-nav-rail');
      vi.useFakeTimers();
      const p = applyReveal(securityOnboardingSteps, i);
      await vi.advanceTimersByTimeAsync(300);
      await p;
      expect(isNavDrawerOpen()).toBe(true);
    });
  });

  describe('applyReveal - десктоп (>=769) не трогает drawer', () => {
    it('на 1024 drawer не открывается', async () => {
      setViewport(1024);
      const i = idxOf(securityOnboardingSteps, 'sec-nav-rail');
      await applyReveal(securityOnboardingSteps, i);
      expect(isNavDrawerOpen()).toBe(false);
    });

    it('но ось open на десктопе работает - узел свёрнут на любой ширине', async () => {
      setViewport(1024);
      const steps = [{ id: 'a', route: '/x', reveal: { open: 'admin-column' } }];
      vi.useFakeTimers();
      const p = applyReveal(steps, 0);
      await vi.advanceTimersByTimeAsync(300);
      await p;
      expect(useOnboardingStore().revealOpen).toBe('admin-column');
    });
  });

  describe('applyReveal - сигнал раскрытия узла (ось open)', () => {
    it('поднимает сигнал нужной цели', async () => {
      setViewport(1024);
      const store = useOnboardingStore();
      const steps = [{ id: 'a', route: '/x', reveal: { open: 'search-panel' } }];
      vi.useFakeTimers();
      const p = applyReveal(steps, 0);
      await vi.advanceTimersByTimeAsync(300);
      await p;
      expect(store.revealOpen).toBe('search-panel');
    });

    it('переход на шаг без open гасит сигнал - узел закрывается', async () => {
      setViewport(1024);
      const store = useOnboardingStore();
      const steps = [
        { id: 'a', route: '/x', reveal: { open: 'first-application' } },
        { id: 'b', route: '/x' },
      ];
      vi.useFakeTimers();
      const p = applyReveal(steps, 0);
      await vi.advanceTimersByTimeAsync(300);
      await p;
      expect(store.revealOpen).toBe('first-application');

      await applyReveal(steps, 1);
      expect(store.revealOpen).toBe(null);
    });

    it('смена цели переключает сигнал (эксклюзивность узлов)', async () => {
      setViewport(1024);
      const store = useOnboardingStore();
      const steps = [
        { id: 'a', route: '/x', reveal: { open: 'admin-column' } },
        { id: 'b', route: '/x', reveal: { open: 'search-panel' } },
      ];
      vi.useFakeTimers();
      const p1 = applyReveal(steps, 0);
      await vi.advanceTimersByTimeAsync(300);
      await p1;
      const p2 = applyReveal(steps, 1);
      await vi.advanceTimersByTimeAsync(300);
      await p2;
      expect(store.revealOpen).toBe('search-panel');
    });

    it('повторный вызов на том же шаге не ждёт анимацию заново', async () => {
      setViewport(1024);
      const steps = [{ id: 'a', route: '/x', reveal: { open: 'admin-column' } }];
      vi.useFakeTimers();
      const p = applyReveal(steps, 0);
      await vi.advanceTimersByTimeAsync(300);
      await p;
      // Второй вызов резолвится без прокрутки таймеров и сообщает «ничего не
      // раскрывал» - по этому признаку хост ждёт цель коротко, а не 4 секунды.
      await expect(applyReveal(steps, 0)).resolves.toBe(false);
    });
  });

  describe('restoreReveal - граница сегмента и teardown', () => {
    it('открытый drawer гасится', () => {
      setViewport(390);
      document.querySelector('[data-testid="ob-nav-rail"]').classList.add('nav-menu--mobile-open');
      restoreReveal();
      expect(isNavDrawerOpen()).toBe(false);
    });

    it('сигнал раскрытия узла гасится - владелец закроет за собой', async () => {
      setViewport(1024);
      const store = useOnboardingStore();
      const steps = [{ id: 'a', route: '/x', reveal: { open: 'admin-column' } }];
      vi.useFakeTimers();
      const p = applyReveal(steps, 0);
      await vi.advanceTimersByTimeAsync(300);
      await p;

      restoreReveal();
      expect(store.revealOpen).toBe(null);
    });
  });

  describe('контракт значений', () => {
    it('все open в конфигурациях туров - из списка поддерживаемых резолвером', () => {
      const configs = [onboardingSteps, securityOnboardingSteps];
      configs.flat().forEach((s) => {
        if (s.reveal?.open) expect(OPEN_TARGETS).toContain(s.reveal.open);
      });
    });

    it('единственное значение mobile - nav', () => {
      const configs = [onboardingSteps, securityOnboardingSteps];
      configs.flat().forEach((s) => {
        if (s.reveal?.mobile) expect(s.reveal.mobile).toBe('nav');
      });
    });
  });
});
