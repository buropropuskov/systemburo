import { describe, it, expect, vi, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { useNarrowScreen } from '@/composables/useNarrowScreen';

// Управляемый мок MediaQueryList: помним слушателей, чтобы эмитить change вручную.
// addEventListener ИЛИ addListener (старый Safari) - тестируем обе ветки.
function makeMql(matches, { legacy = false } = {}) {
  const listeners = [];
  const mql = {
    matches,
    emitChange: (m) => listeners.forEach((cb) => cb({ matches: m })),
  };
  if (legacy) {
    mql.addListener = (cb) => listeners.push(cb);
    mql.removeListener = (cb) => {
      const i = listeners.indexOf(cb);
      if (i >= 0) listeners.splice(i, 1);
    };
  } else {
    mql.addEventListener = (_ev, cb) => listeners.push(cb);
    mql.removeEventListener = (_ev, cb) => {
      const i = listeners.indexOf(cb);
      if (i >= 0) listeners.splice(i, 1);
    };
  }
  return mql;
}

const Host = {
  template: '<div />',
  setup() {
    return useNarrowScreen();
  },
};

const origMatchMedia = window.matchMedia;
afterEach(() => {
  window.matchMedia = origMatchMedia;
});

describe('useNarrowScreen', () => {
  it('isNarrow = matches на момент монтирования', () => {
    window.matchMedia = vi.fn(() => makeMql(true));
    const w = mount(Host);
    expect(w.vm.isNarrow).toBe(true);
  });

  it('десктоп: matches=false -> isNarrow=false', () => {
    window.matchMedia = vi.fn(() => makeMql(false));
    const w = mount(Host);
    expect(w.vm.isNarrow).toBe(false);
  });

  it('реагирует на change медиазапроса', async () => {
    const mql = makeMql(false);
    window.matchMedia = vi.fn(() => mql);
    const w = mount(Host);
    expect(w.vm.isNarrow).toBe(false);
    mql.emitChange(true);
    await w.vm.$nextTick();
    expect(w.vm.isNarrow).toBe(true);
  });

  it('фолбэк addListener (старый Safari): реагирует и снимается', () => {
    const mql = makeMql(true, { legacy: true });
    const removeSpy = vi.spyOn(mql, 'removeListener');
    window.matchMedia = vi.fn(() => mql);
    const w = mount(Host);
    expect(w.vm.isNarrow).toBe(true);
    w.unmount();
    expect(removeSpy).toHaveBeenCalled();
  });
});
