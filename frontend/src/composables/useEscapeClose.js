import { onMounted, onBeforeUnmount, unref } from 'vue';

/**
 * Закрытие модалки по Escape. Вешает один keydown-листенер на document,
 * снимает при размонтировании.
 *
 * @param {(e: KeyboardEvent) => void} onEscape - что вызвать на Escape (close/cancel)
 * @param {undefined | (() => boolean) | import('vue').Ref<boolean>} isActive -
 *   необязательный guard для всегда-смонтированных модалок (store-driven, visible-prop):
 *   Escape сработает только когда isActive истинно. Для модалок, монтируемых по v-if
 *   (присутствуют в DOM только когда открыты), guard не нужен - не передавай.
 */
export function useEscapeClose(onEscape, isActive) {
    const handler = (e) => {
        if (e.key !== 'Escape') return;
        if (isActive !== undefined) {
            const active = typeof isActive === 'function' ? isActive() : unref(isActive);
            if (!active) return;
        }
        onEscape(e);
    };
    onMounted(() => document.addEventListener('keydown', handler));
    onBeforeUnmount(() => document.removeEventListener('keydown', handler));
}
