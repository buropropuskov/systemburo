import { computed, getCurrentInstance, inject, nextTick, onBeforeUnmount, onMounted, provide, ref, watch } from 'vue';
import { useUiStore } from '@/stores/ui';
import { useOverlayClose } from './useOverlayClose';
import { isTopModal, releaseModal, setModalOpen } from '@/utils/modalStack';

export const entityDrawerKey = Symbol('entity-drawer');
const focusables = 'button, a[href], input, select, textarea, [tabindex]';
const visible = element => element?.isConnected && element.getClientRects().length > 0;

/** Only descendants of an open desktop entity card participate. */
export function useEntityDrawer(isNarrow, show, layer) {
  const instance = getCurrentInstance();
  const ready = ref(false);
  onMounted(() => { ready.value = true; });
  onBeforeUnmount(() => { ready.value = false; });
  const desktop = computed(() => ready.value && !isNarrow.value);
  const context = { desktop, layer: computed(layer), dialogs: new Set() };
  provide(entityDrawerKey, context);
  const drawerRoot = ref(null);
  useDrawerFocus(context, drawerRoot, () => desktop.value && show(), layer, true, true);
  const overlay = useOverlayClose(() => instance?.proxy?.closeDrawerPlace?.());
  return { drawerRoot, drawerDesktop: desktop, onPlaceOverlayMousedown: overlay.onOverlayMousedown, onPlaceOverlayMouseup: overlay.onOverlayMouseup };
}

/** Retain declared layers on phones and outside the entity drawer. */
export function useEntityDrawerDialog(element, show, layer, trap = true, trackStack = false) {
  const context = inject(entityDrawerKey, null);
  const desktop = computed(() => !!context?.desktop.value);
  const modalLayer = computed(() => Math.max(Number(layer()) || 0, desktop.value ? context.layer.value + 1 : 0));
  const owner = {};
  if (trackStack && context) {
    watch(() => [desktop.value && show(), modalLayer.value], ([open, value]) => setModalOpen(owner, open, value), { immediate: true });
    onBeforeUnmount(() => releaseModal(owner));
  }
  if (context) useDrawerFocus(context, element, () => desktop.value && show(), () => modalLayer.value, trap);
  return { drawerDesktop: desktop, modalLayer, drawerContext: context, isDrawerTop: () => !desktop.value || isTopModal(owner) };
}

function useDrawerFocus(context, element, active, layer, trap = true, owner = false) {
  const record = { element, layer, opener: null };
  const top = () => [...context.dialogs].sort((a, b) => a.layer() - b.layer()).at(-1);
  let version = 0;
  watch(active, async open => {
    const current = ++version;
    if (open) {
      record.opener = document.activeElement;
      context.dialogs.add(record);
      await nextTick();
      if (current !== version || !active() || top() !== record) return;
      const target = element.value;
      if (visible(target) && !target.contains(document.activeElement)) target.focus({ preventScroll: true });
    } else {
      const wasTop = top() === record;
      context.dialogs.delete(record);
      await nextTick();
      if (current === version && wasTop && visible(record.opener)) record.opener.focus({ preventScroll: true });
    }
  }, { immediate: true, flush: 'post' });

  const onTab = event => {
    if (!trap || event.key !== 'Tab' || !active() || top() !== record) return;
    const root = element.value;
    if (!visible(root) || !root.contains(document.activeElement)) return;
    const items = [...root.querySelectorAll(focusables)].filter(item => !item.disabled && item.tabIndex >= 0 && visible(item));
    const first = items[0], last = items.at(-1);
    if (!first) { event.preventDefault(); root.focus({ preventScroll: true }); return; }
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault(); (event.shiftKey ? last : first).focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault(); first.focus();
    }
  };
  onMounted(() => document.addEventListener('keydown', onTab, true));
  onBeforeUnmount(() => {
    version++;
    const wasTop = top() === record;
    context.dialogs.delete(record);
    document.removeEventListener('keydown', onTab, true);
    if (owner) context.dialogs.clear();
    if (owner || wasTop) nextTick(() => {
      if (visible(record.opener)) record.opener.focus({ preventScroll: true });
    });
  });
}

/** The existing global confirmation UI is above every entity/history layer. */
export async function confirmDrawerDiscard(context, dirty) {
  if (!context?.desktop.value || !dirty) return true;
  const ui = useUiStore();
  if (ui.confirmState) return false;
  return ui.confirm({
    title: 'Закрыть без сохранения?',
    message: 'Внесённые изменения не будут сохранены.',
    confirmText: 'Закрыть без сохранения',
    cancelText: 'Продолжить изменение',
  });
}
