<template>
  <span
    class="copyable-number"
    :data-tooltip="tooltip"
    role="button"
    tabindex="0"
    @click.stop="copy"
    @keydown.enter.prevent="copy"
  >{{ value }}</span>
</template>

<script>
import { copyText } from '@/utils/clipboard'
import { useDeletionsStore } from '@/stores/deletions'

/**
 * Номер, который копируют кликом: заявки диктуют по телефону и вставляют в переписку,
 * а выделять текст мышью внутри модалки неудобно.
 *
 * Отдельный элемент, а не кликабельный заголовок целиком: нажатие должно попадать в
 * номер, а не срабатывать на всей строке с датой и кнопками.
 */
export default {
    name: 'CopyableNumber',
    props: {
        value: {
            type: [String, Number],
            required: true
        },
        tooltip: {
            type: String,
            default: 'Копировать'
        }
    },
    methods: {
        async copy() {
            if (!this.value) return;
            const number = String(this.value);
            const copied = await copyText(number);
            useDeletionsStore().notify(copied
                ? { prefix: 'Номер ', bold: number, suffix: ' скопирован', type: 'success' }
                : { prefix: 'Не удалось ', bold: 'скопировать номер', type: 'error' });
        }
    }
}
</script>

<style scoped>
.copyable-number {
    position: relative;
    cursor: pointer;
    border-radius: 4px;
    outline: none;
    transition: color 0.15s;
}

.copyable-number:hover,
.copyable-number:focus-visible {
    color: var(--accent-text);
}

.copyable-number:focus-visible {
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 30%, transparent);
}

/* Подсказка появляется под номером: над ним в шапке заявки нет места. */
.copyable-number::after {
    content: attr(data-tooltip);
    position: absolute;
    top: calc(100% + 6px);
    left: 50%;
    transform: translateX(-50%);
    padding: 4px 8px;
    border-radius: 6px;
    background: var(--hint-bg);
    color: var(--hint-text);
    font-size: 11px;
    font-weight: 500;
    white-space: nowrap;
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.15s;
    box-shadow: 0 2px 8px var(--shadow-drop);
    z-index: 1;
}

.copyable-number:hover::after {
    opacity: 1;
}

/* Палец попадает мимо компактной надписи - расширяем зону нажатия, не раздувая
   визуально строку заголовка. */
@media (max-width: 767.98px) {
    .copyable-number::before {
        content: '';
        position: absolute;
        inset: -10px -8px;
    }
}
</style>
