import { describe, expect, it } from 'vitest';
import { effectScope, nextTick, ref } from 'vue';
import { useTourDisclosure } from '../tourDisclosure';

function fixture(initial = false) {
  const requested = ref(false), enabled = ref(true), open = ref(initial);
  const scope = effectScope();
  scope.run(() => useTourDisclosure({ requested, enabled, open }));
  return { requested, enabled, open, scope };
}

describe('tour disclosure ownership', () => {
  it('opens and releases only the requested owner panel', async () => {
    const state = fixture(); state.requested.value = true; await nextTick();
    expect(state.open.value).toBe(true);
    state.requested.value = false; await nextTick(); expect(state.open.value).toBe(false);
    state.scope.stop();
  });
  it('preserves a panel already opened by the user', async () => {
    const state = fixture(true); state.requested.value = true; await nextTick();
    state.requested.value = false; await nextTick(); state.scope.stop();
    expect(state.open.value).toBe(true);
  });
  it('releases its mobile panel on rotation and opens it again when applicable', async () => {
    const state = fixture(); state.requested.value = true; await nextTick();
    state.enabled.value = false; await nextTick(); expect(state.open.value).toBe(false);
    state.enabled.value = true; await nextTick(); expect(state.open.value).toBe(true);
    state.scope.stop(); expect(state.open.value).toBe(false);
  });
  it('does not immediately reopen a panel dismissed by the user', async () => {
    const state = fixture(); state.requested.value = true; await nextTick();
    state.open.value = false; await nextTick(); expect(state.open.value).toBe(false);
    state.scope.stop();
  });
  it('relinquishes ownership after dismissal and preserves a later manual opening', async () => {
    const state = fixture(); state.requested.value = true; await nextTick();
    state.open.value = false; await nextTick();
    state.open.value = true; await nextTick();
    state.requested.value = false; await nextTick();
    expect(state.open.value).toBe(true);
    state.scope.stop(); expect(state.open.value).toBe(true);
  });
});
