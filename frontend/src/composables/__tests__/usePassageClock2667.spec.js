import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { effectScope, nextTick, ref } from 'vue';
import { usePassageClock } from '../usePassageClock';

const scopes = [];
function create(server, options) {
  const scope = effectScope(); scopes.push(scope);
  return scope.run(() => usePassageClock(server, options));
}
describe('server passage clock #2667', () => {
  beforeEach(() => vi.useFakeTimers({ toFake: ['Date', 'performance', 'setTimeout', 'clearTimeout', 'setInterval', 'clearInterval'] }));
  afterEach(() => { scopes.splice(0).forEach(scope => scope.stop()); vi.useRealTimers(); });
  it('uses a server anchor plus monotonic elapsed time across client clock changes', () => {
    const server = ref('2035-01-01T12:00:00Z');
    const clock = create(server);
    vi.advanceTimersByTime(2000);
    vi.setSystemTime(new Date('2045-01-01T00:00:00Z'));
    vi.advanceTimersByTime(1000);
    expect(clock.now.value).toBe(Date.parse(server.value) + 3000);
    expect(clock.elapsedSince('2035-01-01T11:59:59Z')).toBe(4000);
  });
  it('fires grace at exactly five minutes and attention strictly after 48 hours', async () => {
    const server = ref('2035-01-03T12:00:00Z'), onBoundary = vi.fn();
    const anchor = Date.parse(server.value);
    const entry = Date.parse('2035-01-01T12:05:00Z');
    const deadlines = ref([anchor + 300000, entry + 48 * 3600000 + 1]);
    create(server, { deadlines, onBoundary });
    vi.advanceTimersByTime(299999);
    expect(onBoundary).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(onBoundary).toHaveBeenCalledExactlyOnceWith(anchor + 300000);
    vi.advanceTimersByTime(1);
    expect(onBoundary).toHaveBeenNthCalledWith(2, anchor + 300001);
    deadlines.value = [...deadlines.value]; await nextTick();
    vi.advanceTimersByTime(1000);
    expect(onBoundary).toHaveBeenCalledTimes(2);
  });
  it('fails closed without a valid server instant and releases timers on disposal', async () => {
    const server = ref(null), active = ref(true), onBoundary = vi.fn();
    const clock = create(server, { active, deadlines: ref(['2035-01-01T12:00:01Z']), onBoundary });
    expect(clock.now.value).toBeNull();
    expect(clock.elapsedSince('2035-01-01T12:00:00Z')).toBeNull();
    server.value = '2035-01-01T12:00:00Z'; await nextTick();
    scopes[0].stop(); vi.advanceTimersByTime(10000);
    expect(onBoundary).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
  });
  it('reanchors when a fresh server read arrives and ignores malformed entry instants', async () => {
    const server = ref('2035-01-01T12:00:00Z');
    const clock = create(server);
    server.value = '2035-01-01T13:00:00Z'; await nextTick();
    expect(clock.now.value).toBe(Date.parse(server.value));
    expect(clock.elapsedSince('invalid')).toBeNull();
  });
});
