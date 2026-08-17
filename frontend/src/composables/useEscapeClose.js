import { onMounted, onBeforeUnmount, unref, watchEffect } from 'vue';

import { setModalOpen, releaseModal, isTopModal, isEscapeHandled, markEscapeHandled } from '@/utils/modalStack';

/**
 * Закрытие модалки по Escape. Вешает один keydown-листенер на document,
 * снимает при размонтировании.
 *
 * Окно встаёт в общую стопку окон и на Escape отвечает только когда оно верхнее, а
 * ответив - помечает нажатие обработанным. Без этого один Escape закрывал сразу два
 * слоя: карточка машины закрывалась своим слушателем, а панель заявки под ней считала
 * себя верхней (о карточке стопка не знала) и закрывалась тем же нажатием.
 *
 * @param {(e: KeyboardEvent) => void} onEscape - что вызвать на Escape (close/cancel)
 * @param {undefined | (() => boolean) | import('vue').Ref<boolean>} isActive -
 *   необязательный guard для всегда-смонтированных модалок (store-driven, visible-prop):
 *   Escape сработает только когда isActive истинно. Для модалок, монтируемых по v-if
 *   (присутствуют в DOM только когда открыты), guard не нужен - не передавай.
 * @param {number} [zIndex] слой окна для стопки: при равных слоях верхним считается
 *   открытое последним, поэтому значение обязательно только там, где слои неравные.
 */
export function useEscapeClose(onEscape, isActive, zIndex = 0) {
    // Ключ владельца в стопке: свой объект, а не инстанс компонента - композабл вызывают
    // и из setup, где инстанса под рукой нет.
    const owner = {};

    const isOpen = () => {
        if (isActive === undefined) return true;
        return typeof isActive === 'function' ? !!isActive() : !!unref(isActive);
    };

    const handler = (e) => {
        if (e.key !== 'Escape') return;
        if (!isOpen()) return;
        // Нажатие уже разобрал слой выше - молчим, иначе закроемся вместе с ним.
        if (isEscapeHandled(e)) return;
        if (!isTopModal(owner)) return;
        markEscapeHandled(e);
        onEscape(e);
    };

    onMounted(() => {
        document.addEventListener('keydown', handler);
        // Окна по v-if смонтированы только открытыми, поэтому регистрируем сразу;
        // окна с guard'ом встают в стопку через watchEffect ниже.
        if (isActive === undefined) setModalOpen(owner, true, zIndex);
    });

    if (isActive !== undefined) {
        watchEffect(() => setModalOpen(owner, isOpen(), zIndex));
    }

    onBeforeUnmount(() => {
        document.removeEventListener('keydown', handler);
        releaseModal(owner);
    });
}
