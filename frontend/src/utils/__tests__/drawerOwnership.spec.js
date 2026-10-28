import { describe, expect, it, vi } from 'vitest';
import { createDrawerOwnership } from '../drawerOwnership';

let token = 0;
function owner(initial = false, effect = () => {}) {
  let value = initial;
  const write = vi.fn(open => { value = open; effect(open); });
  const api = createDrawerOwnership({ read: () => value, write });
  return { ...api, read: () => value, write };
}
describe('drawer state ownership', () => {
  it('restores an initially closed drawer opened by the tour', () => {
    const drawer = owner(), id = ++token;
    expect(drawer.claim(true, id)).toBe(true); expect(drawer.read()).toBe(true);
    expect(drawer.restore(id)).toBe(true); expect(drawer.read()).toBe(false);
    expect(drawer.restore(id)).toBe(false);
  });
  it('does not acquire an unchanged preopened drawer', () => {
    const drawer = owner(true), id = ++token;
    expect(drawer.claim(true, id)).toBe(false); expect(drawer.restore(id)).toBe(false);
    expect(drawer.read()).toBe(true); expect(drawer.write).not.toHaveBeenCalled();
  });
  it('restores preopened state after a temporary tour close', () => {
    const drawer = owner(true), id = ++token;
    drawer.claim(false, id); drawer.claim(false, id); drawer.restore(id);
    expect(drawer.read()).toBe(true); expect(drawer.write).toHaveBeenCalledTimes(2);
  });
  it('retains the original snapshot across multiple requests with the same token', () => {
    const drawer = owner(), id = ++token;
    drawer.claim(true, id); drawer.claim(false, id); drawer.claim(true, id); drawer.restore(id);
    expect(drawer.read()).toBe(false);
  });
  it.each(['close-reopen', 'idempotent', 'route', 'breakpoint'])('manual/system %s invalidates ownership', action => {
    const drawer = owner(), id = ++token; drawer.claim(true, id);
    if (action === 'close-reopen') { drawer.set(false); drawer.set(true); }
    else drawer.set(action === 'idempotent');
    const state = drawer.read(); expect(drawer.restore(id)).toBe(false);
    expect(drawer.claim(!state, id)).toBe(false); expect(drawer.read()).toBe(state);
  });
  it('rejects invalid and older token claims', () => {
    const drawer = owner(), old = ++token, newer = ++token;
    drawer.claim(true, newer);
    for (const id of [old, 0, -1, 1.5, NaN, Infinity, '3']) expect(drawer.claim(false, id)).toBe(false);
    expect(drawer.read()).toBe(true);
  });
  it('keeps token history local to the factory; DOM lifetime is enforced by its caller', () => {
    const old = owner(), id = ++token; old.claim(true, id); old.dispose();
    const next = owner(); expect(next.claim(true, id)).toBe(true);
    expect(next.restore(id)).toBe(true);
  });
  it('preserves the original snapshot through explicit previousToken succession', () => {
    const drawer = owner(), first = ++token, second = ++token, third = ++token;
    drawer.claim(true, first); drawer.claim(true, second, first); drawer.claim(false, third, second);
    expect(drawer.restore(second)).toBe(false); drawer.restore(third); expect(drawer.read()).toBe(false);
  });
  it('a fresh token after a manual action uses the new user state as baseline', () => {
    const drawer = owner(), first = ++token; drawer.claim(true, first); drawer.set(false); drawer.set(true);
    const next = ++token; drawer.claim(false, next, first); drawer.restore(next); expect(drawer.read()).toBe(true);
  });
  it('clears restoration ownership before a reentrant write claims a newer lease', () => {
    let armed = false, drawer; const first = ++token, newer = ++token;
    drawer = owner(false, () => { if (armed) { armed = false; drawer.claim(true, newer); } });
    drawer.claim(true, first); armed = true; drawer.restore(first);
    expect(drawer.read()).toBe(true); expect(drawer.restore(newer)).toBe(true); expect(drawer.read()).toBe(false);
  });
  it('will not restore after a state change made outside its writer', () => {
    let value = false; const drawer = createDrawerOwnership({ read: () => value, write: open => { value = open; } });
    const id = ++token; drawer.claim(true, id); value = false;
    expect(drawer.restore(id)).toBe(false);
  });
  it('dispose permanently disables every write path', () => {
    const drawer = owner(), id = ++token; drawer.claim(true, id); drawer.dispose();
    expect(drawer.set(false)).toBe(false); expect(drawer.claim(false, ++token)).toBe(false);
    expect(drawer.restore(id)).toBe(false); expect(drawer.read()).toBe(true);
  });
});
