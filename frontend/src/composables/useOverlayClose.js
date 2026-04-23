import { ref } from 'vue'

/**
 * Composable для корректного закрытия модалки по клику на оверлей,
 * с защитой от drag-out: если пользователь начал выделять текст внутри
 * модалки и отпустил курсор на оверлее — модалка НЕ закрывается.
 *
 * Применяется к оверлею (не к самому модальному окну):
 *
 *   <div class="modal-overlay"
 *        @mousedown="onOverlayMousedown"
 *        @mouseup="onOverlayMouseup">
 *     <div class="modal-content"
 *          @mousedown.stop
 *          @click.stop>
 *       ...
 *     </div>
 *   </div>
 *
 * @param {() => void} onClose - колбэк закрытия модалки
 * @returns {{ onOverlayMousedown, onOverlayMouseup }}
 */
export function useOverlayClose(onClose) {
  const startedOnOverlay = ref(false)

  function onOverlayMousedown(e) {
    startedOnOverlay.value = e.target === e.currentTarget
  }

  function onOverlayMouseup(e) {
    const wasOnOverlay = startedOnOverlay.value
    startedOnOverlay.value = false
    if (!wasOnOverlay) return
    if (e.target !== e.currentTarget) return
    onClose()
  }

  return { onOverlayMousedown, onOverlayMouseup }
}
