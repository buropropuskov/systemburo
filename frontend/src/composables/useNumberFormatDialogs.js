import { ref, watch, onBeforeUnmount } from 'vue';
import { useOverlayClose } from './useOverlayClose';
import { useEscapeClose } from './useEscapeClose';
import { createDialogFocus } from '@/utils/dialogFocus';
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';

function dialog(close, titleId) {
  const visible = ref(false), panel = ref(null), owner = {};
  const focus = createDialogFocus(() => panel.value, () => isTop());
  // Возвращаем фокус до удаления владельца из общей стопки окон.
  onBeforeUnmount(() => { focus.close(); releaseBodyScrollLock(owner); });
  const isTop = useEscapeClose(close, visible, 1000);
  watch(visible, value => {
    setBodyScrollLock(owner, value);
    if (!value) focus.close();
  }, { flush: 'sync' });
  watch(visible, value => { if (value) focus.open(); }, { flush: 'post' });
  const attrs = {
    ref: panel, role: 'dialog', 'aria-modal': 'true',
    'aria-labelledby': titleId, tabindex: -1,
    style: { outline: 'none' },
    onKeydown: event => focus.handleTab(event),
    onMousedown: event => event.stopPropagation(),
  };
  return { visible, attrs, overlay: useOverlayClose(close) };
}

export function useNumberFormatDialogs() {
  // Компонент назначает закрытие с существующей проверкой несохранённых полей.
  const addOverlay = { close: () => {} }, cellOverlay = { close: () => {} };
  const add = dialog(() => addOverlay.close(), 'nf-add-title');
  const cell = dialog(() => cellOverlay.close(), 'nf-cell-title');
  return {
    addOverlay, cellOverlay,
    showAddModal: add.visible, showCellEditModal: cell.visible,
    addDialogAttrs: add.attrs, cellDialogAttrs: cell.attrs,
    onAddOverlayMousedown: add.overlay.onOverlayMousedown,
    onAddOverlayMouseup: add.overlay.onOverlayMouseup,
    onCellOverlayMousedown: cell.overlay.onOverlayMousedown,
    onCellOverlayMouseup: cell.overlay.onOverlayMouseup,
  };
}
