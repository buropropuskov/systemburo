import { describe, expect, it, vi } from 'vitest';
import { createStepTransition } from '../stepTransition';

function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

describe('step transition ownership', () => {
  it('sets busy synchronously and ignores duplicate work until completion', async () => {
    const held = deferred(), changes = [];
    const transition = createStepTransition({ onBusyChange: value => changes.push(value) });
    const first = vi.fn(async () => {
      expect(transition.isBusy()).toBe(true);
      await held.promise;
      return 'committed';
    });
    const pending = transition.run(first);
    expect(transition.isBusy()).toBe(true);
    expect(changes).toEqual([true]);
    expect(first).toHaveBeenCalledTimes(1);
    const duplicate = vi.fn();
    expect(transition.run(duplicate)).toBeUndefined();
    await Promise.resolve();
    expect(first).toHaveBeenCalledTimes(1);
    expect(duplicate).not.toHaveBeenCalled();
    held.resolve();
    await expect(pending).resolves.toBe('committed');
    expect(transition.isBusy()).toBe(false);
    expect(changes).toEqual([true, false]);
  });

  it('replacement aborts the previous operation and makes its context stale', async () => {
    const held = deferred();
    const transition = createStepTransition();
    let oldContext, newContext;
    const first = transition.run(async context => { oldContext = context; await held.promise; });
    await Promise.resolve();
    const second = transition.run(context => { newContext = context; }, { replace: true });
    expect(oldContext.signal.aborted).toBe(true);
    expect(oldContext.isCurrent()).toBe(false);
    await second;
    expect(newContext.signal).not.toBe(oldContext.signal);
    held.resolve(); await first;
  });

  it.each(['resolve', 'reject'])('stale %s cannot unlock a pending replacement', async settlement => {
    const old = deferred(), fresh = deferred(), changes = [];
    const transition = createStepTransition({ onBusyChange: value => changes.push(value) });
    const first = transition.run(() => old.promise);
    const firstOutcome = first.catch(error => error);
    await Promise.resolve();
    const second = transition.run(() => fresh.promise, { replace: true });
    await Promise.resolve();
    old[settlement](settlement === 'resolve' ? 'old' : new Error('old failure'));
    await firstOutcome;
    expect(transition.isBusy()).toBe(true);
    expect(changes).toEqual([true, true]);
    fresh.resolve('new'); await second;
    expect(changes).toEqual([true, true, false]);
  });

  it('cancel invalidates an awaiting context and prevents a guarded commit', async () => {
    const held = deferred(), commit = vi.fn(), changes = [];
    const transition = createStepTransition({ onBusyChange: value => changes.push(value) });
    let context;
    const pending = transition.run(async value => {
      context = value; await held.promise;
      if (value.isCurrent()) commit();
    });
    await Promise.resolve(); transition.cancel();
    expect(context.signal.aborted).toBe(true);
    expect(context.isCurrent()).toBe(false);
    expect(transition.isBusy()).toBe(false);
    held.resolve(); await pending;
    expect(commit).not.toHaveBeenCalled();
    expect(changes).toEqual([true, false]);
  });

  it('a cancelled old finally cannot clear a new operation', async () => {
    const old = deferred(), fresh = deferred();
    const transition = createStepTransition();
    const first = transition.run(() => old.promise); await Promise.resolve();
    transition.cancel();
    const second = transition.run(() => fresh.promise); await Promise.resolve();
    old.resolve(); await first;
    expect(transition.isBusy()).toBe(true);
    fresh.resolve(); await second;
    expect(transition.isBusy()).toBe(false);
  });

  it.each(['sync', 'async'])('propagates %s failure and releases the lock', async kind => {
    const failure = new Error('failure'), held = deferred();
    const transition = createStepTransition();
    const pending = transition.run(() => {
      if (kind === 'sync') throw failure;
      return held.promise;
    });
    const assertion = expect(pending).rejects.toBe(failure);
    if (kind === 'async') { await Promise.resolve(); held.reject(failure); }
    await assertion;
    expect(transition.isBusy()).toBe(false);
    await expect(transition.run(() => 'next')).resolves.toBe('next');
  });

  it('does not start work while inactive', () => {
    const changes = vi.fn(), work = vi.fn();
    const transition = createStepTransition({ isActive: () => false, onBusyChange: changes });
    expect(transition.run(work)).toBeUndefined();
    expect(transition.isBusy()).toBe(false);
    expect(work).not.toHaveBeenCalled(); expect(changes).not.toHaveBeenCalled();
  });

  it('inactive state invalidates an already running context', async () => {
    let active = true, context;
    const held = deferred();
    const transition = createStepTransition({ isActive: () => active });
    const pending = transition.run(async value => { context = value; await held.promise; });
    await Promise.resolve(); active = false;
    expect(context.isCurrent()).toBe(false);
    held.resolve(); await pending;
    expect(transition.isBusy()).toBe(false);
  });

  it('a busy observer can cancel before work starts', async () => {
    let transition;
    transition = createStepTransition({ onBusyChange: busy => { if (busy) transition.cancel(); } });
    const work = vi.fn();
    const pending = transition.run(work);
    await pending;
    expect(work).not.toHaveBeenCalled();
    expect(transition.isBusy()).toBe(false);
  });
});
