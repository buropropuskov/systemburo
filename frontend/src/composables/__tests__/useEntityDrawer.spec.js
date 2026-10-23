import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { defineComponent, h, ref } from 'vue';
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { useEntityDrawer, useEntityDrawerDialog, confirmDrawerDiscard } from '../useEntityDrawer';
import { useUiStore } from '@/stores/ui';
import { releaseModal, resetModalStack, setModalOpen } from '@/utils/modalStack';

const mounted = [];
const Child = defineComponent({
  props: { layer: { type: Number, default: 10005 } },
  setup(props, { expose }) {
    const root = ref(null);
    const drawer = useEntityDrawerDialog(root, () => true, () => props.layer, true, true);
    expose(drawer);
    return () => h('section', { ref: root, tabindex: -1, 'data-layer': drawer.modalLayer.value }, [h('button', 'Child')]);
  },
});

function harness(phone = false, rootLayer = 10001) {
  const narrow = ref(phone), show = ref(true), child = ref(false);
  const Host = defineComponent({
    setup() {
      const drawer = useEntityDrawer(narrow, () => show.value, () => rootLayer);
      return () => show.value ? h('div', [
        h('section', { ref: drawer.drawerRoot, tabindex: drawer.drawerDesktop.value ? -1 : null }, [
          h('button', { 'data-first': '' }, 'First'), h('button', { 'data-last': '' }, 'Last'),
        ]),
        child.value ? h(Child) : null,
      ]) : null;
    },
  });
  const wrapper = mount(Host, { attachTo: document.body });
  mounted.push(wrapper);
  return { wrapper, narrow, show, child };
}

beforeEach(() => {
  setActivePinia(createPinia());
  resetModalStack();
  vi.spyOn(HTMLElement.prototype, 'getClientRects').mockImplementation(function () {
    return this.isConnected ? [{ width: 100, height: 40 }] : [];
  });
});
afterEach(() => {
  mounted.splice(0).forEach(wrapper => wrapper.unmount());
  document.body.replaceChildren();
  vi.restoreAllMocks();
});

describe('entity drawer desktop scope', () => {
  it('focuses the desktop card, cycles Tab, and restores the opener', async () => {
    const opener = document.createElement('button');
    document.body.append(opener); opener.focus();
    const { wrapper, show } = harness();
    await flushPromises();
    expect(document.activeElement).toBe(wrapper.find('section').element);
    wrapper.find('[data-last]').element.focus();
    const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true });
    document.activeElement.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(wrapper.find('[data-first]').element);
    show.value = false; await flushPromises();
    expect(document.activeElement).toBe(opener);
  });

  it('opens child forms above a card from history and returns focus without scrolling', async () => {
    const { wrapper, child } = harness(false, 14000);
    await flushPromises();
    const opener = wrapper.find('[data-last]').element;
    opener.focus(); child.value = true; await flushPromises();
    const dialog = wrapper.find('[data-layer]');
    expect(dialog.attributes('data-layer')).toBe('14001');
    expect(document.activeElement).toBe(dialog.element);
    child.value = false; await flushPromises();
    expect(document.activeElement).toBe(opener);
  });

  it('does not steal focus from a separate dialog outside its own subtree', async () => {
    harness(); await flushPromises();
    const external = document.createElement('button');
    document.body.append(external); external.focus();
    const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true });
    external.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(false);
  });

  it('lets a higher form or global confirmation handle Escape before the place window', async () => {
    const { wrapper, child } = harness();
    child.value = true; await flushPromises();
    const place = wrapper.getComponent(Child).vm;
    expect(place.isDrawerTop()).toBe(true);
    const confirmation = {};
    setModalOpen(confirmation, true, 22000);
    expect(place.isDrawerTop()).toBe(false);
    releaseModal(confirmation);
    expect(place.isDrawerTop()).toBe(true);
  });

  it('restores the external opener when the card unmounts with a child still open', async () => {
    const opener = document.createElement('button');
    document.body.append(opener); opener.focus();
    const { wrapper, child } = harness();
    await flushPromises();
    wrapper.find('[data-last]').element.focus();
    child.value = true; await flushPromises();
    wrapper.unmount(); await flushPromises();
    expect(document.activeElement).toBe(opener);
  });

  it('keeps phone focus, declared child layers, and closing behavior unchanged', async () => {
    const opener = document.createElement('button');
    document.body.append(opener); opener.focus();
    const { wrapper, child } = harness(true, 14000);
    await flushPromises();
    expect(wrapper.find('section').attributes('tabindex')).toBeUndefined();
    expect(document.activeElement).toBe(opener);
    child.value = true; await flushPromises();
    expect(wrapper.find('[data-layer]').attributes('data-layer')).toBe('10005');
    expect(document.activeElement).toBe(opener);
    expect(await confirmDrawerDiscard({ desktop: ref(false) }, true)).toBe(true);
    expect(useUiStore().confirmState).toBeNull();
  });

  it('switches scope on viewport changes and removes keyboard listeners on unmount', async () => {
    const { wrapper, narrow } = harness(true);
    await flushPromises();
    narrow.value = false; await flushPromises();
    expect(wrapper.find('section').attributes('tabindex')).toBe('-1');
    narrow.value = true; await flushPromises();
    expect(wrapper.find('section').attributes('tabindex')).toBeUndefined();
    const remove = vi.spyOn(document, 'removeEventListener');
    wrapper.unmount();
    expect(remove).toHaveBeenCalledWith('keydown', expect.any(Function), true);
  });
});

describe('discard confirmation', () => {
  it('does not ask for unchanged data and keeps edits after cancellation', async () => {
    const context = { desktop: ref(true) };
    expect(await confirmDrawerDiscard(context, false)).toBe(true);
    const pending = confirmDrawerDiscard(context, true);
    expect(useUiStore().confirmState.cancelText).toBe('Продолжить изменение');
    expect(await confirmDrawerDiscard(context, true)).toBe(false);
    useUiStore().resolveConfirm(false);
    expect(await pending).toBe(false);
  });

  it('returns the explicit discard decision', async () => {
    const pending = confirmDrawerDiscard({ desktop: ref(true) }, true);
    useUiStore().resolveConfirm(true);
    expect(await pending).toBe(true);
  });
});
