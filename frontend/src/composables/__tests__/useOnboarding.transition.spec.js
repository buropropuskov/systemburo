import { beforeEach, describe, expect, it, vi } from 'vitest';

const state = vi.hoisted(() => ({ config: null, index: 0, moves: [], store: null }));
vi.mock('driver.js', () => ({
  driver: (config) => {
    state.config = config;
    return {
      getConfig: () => state.config,
      setConfig: (next) => { state.config = next; },
      getActiveIndex: () => state.index,
      moveNext: () => { state.index += 1; state.moves.push(state.index); },
      movePrevious: () => { state.index -= 1; state.moves.push(state.index); },
      moveTo: (index) => { state.index = index; state.moves.push(index); },
      destroy: () => {},
    };
  },
}));
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => state.store }));
import { useOnboarding } from '../useOnboarding';
import { createStepTransition } from '@/components/onboarding/stepTransition';
import { createKeyboardNav } from '@/components/onboarding/keyboardNav';

function deferred() {
  let resolve;
  const promise = new Promise((done) => { resolve = done; });
  return { promise, resolve };
}

describe('переход тура принадлежит одному действующему инстансу', () => {
  let steps;
  beforeEach(() => {
    steps = [{ id: 'a' }, { id: 'b' }, { id: 'c', reveal: { open: 'card' } }];
    state.index = 0;
    state.moves = [];
    state.store = { steps, skippedIndexes: [], markSkipped: vi.fn() };
  });

  it('повторная кнопка во время подготовки не готовит и не пропускает шаг дважды', async () => {
    const ready = deferred();
    const prepare = vi.fn(() => ready.promise);
    const driver = useOnboarding().createDriver(steps, { onBeforeStep: prepare });
    const first = driver.obNext();
    const second = driver.obNext();
    expect(prepare).toHaveBeenCalledTimes(1);
    ready.resolve(true);
    await Promise.all([first, second]);
    expect(state.moves).toEqual([1]);
  });

  it('назад пересекает границу, если первый шаг сегмента тоже был пропущен', async () => {
    const prior = { id: 'prior' };
    state.store.steps = [prior, ...steps];
    state.store.skippedIndexes = [1, 2];
    state.index = 2;
    const boundary = vi.fn();
    const prepare = vi.fn();
    const driver = useOnboarding().createDriver(steps, { startIndex: 1, onBoundaryPrev: boundary, onBeforeStep: prepare });
    await driver.obPrev();
    expect(boundary).toHaveBeenCalledWith(1);
    expect(prepare).not.toHaveBeenCalled();
    expect(state.moves).toEqual([]);
  });

  it('уничтожение во время Далее запрещает и перемещение, и запись пропуска', async () => {
    const ready = deferred();
    const driver = useOnboarding().createDriver(steps, { onBeforeStep: () => ready.promise });
    const moving = driver.obNext();
    driver.destroy();
    ready.resolve(false);
    await moving;
    expect(state.moves).toEqual([]);
    expect(state.store.markSkipped).not.toHaveBeenCalled();
  });

  it('уничтожение во время подготовки Назад не выполняет поздний movePrevious', async () => {
    state.index = 2;
    steps[1].reveal = { open: 'search' };
    const ready = deferred();
    const driver = useOnboarding().createDriver(steps, { onBeforeStep: () => ready.promise });
    const moving = driver.obPrev();
    driver.destroy();
    ready.resolve(true);
    await moving;
    await Promise.resolve();
    expect(state.moves).toEqual([]);
  });

  it('ошибка подготовки освобождает переход для следующей попытки', async () => {
    const prepare = vi.fn().mockRejectedValueOnce(new Error('prepare')).mockResolvedValue(true);
    const driver = useOnboarding().createDriver(steps, { onBeforeStep: prepare });
    await expect(driver.obNext()).rejects.toThrow('prepare');
    await driver.obNext();
    expect(state.moves).toEqual([1]);
  });

  it('замена тура во время подготовки запрещает commit старого шага', async () => {
    state.store.activeTourKey = 'user';
    const ready = deferred();
    const driver = useOnboarding().createDriver(steps, { onBeforeStep: () => ready.promise });
    const moving = driver.obNext();
    state.store.activeTourKey = 'admin';
    ready.resolve(true);
    await moving;
    expect(state.moves).toEqual([]);
  });

  it('изменение состава шагов не записывает пропуск по индексу другого шага', async () => {
    const ready = deferred();
    const driver = useOnboarding().createDriver(steps, { onBeforeStep: () => ready.promise });
    const moving = driver.obNext();
    state.store.steps = [steps[0], steps[2]];
    ready.resolve(false);
    await moving;
    expect(state.moves).toEqual([]);
    expect(state.store.markSkipped).not.toHaveBeenCalled();
  });

  it('одна отложенная стрелка вычисляет следующий шаг после commit первого', async () => {
    const ready = deferred();
    let driver;
    const keys = createKeyboardNav({ isActive: () => true,
      next: () => driver.obNext(), prev: () => driver.obPrev(), close: () => {} });
    const transitions = createStepTransition({ onBusyChange: keys.setBusy });
    const prepare = vi.fn().mockImplementationOnce(() => ready.promise).mockResolvedValue(true);
    driver = useOnboarding().createDriver(steps, {
      onBeforeStep: prepare, runTransition: transitions.run,
    });
    keys.attach();
    try {
      const first = driver.obNext();
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
      expect(prepare).toHaveBeenCalledTimes(1);
      ready.resolve(true);
      await first;
      await Promise.resolve();
      expect(state.moves).toEqual([1, 2]);
      expect(prepare.mock.calls.map(([index]) => index)).toEqual([1, 2]);
    } finally {
      keys.detach();
      transitions.cancel();
      driver.destroy();
    }
  });
});
