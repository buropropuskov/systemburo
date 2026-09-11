<template>
  <button
    v-if="attachment"
    type="button"
    class="lk-button lk-button--primary execution-mark"
    :disabled="!canMark"
    data-testid="aa-mark-executed"
    @click="mark"
  >
    {{ label }}
  </button>
</template>

<script setup>
/**
 * Кнопка "Отметить как исполненное" во вкладке "Доступные мне" (#2446): охранник
 * подтверждает, что по заявке приехали/пришли. Вынесена отдельным компонентом -
 * AccessibleAttachmentsView.vue уже за порогом размера template/style, а логика
 * отметки в него бы не поместилась.
 *
 * Окно повтора (5 минут) считает бэк - здесь только отображение готового значения
 * (execution_marked_until из детали вложения либо из ответа самой отметки) и тиканье
 * локального "сейчас", чтобы кнопка сама разблокировалась без повторного открытия
 * вложения.
 */
import { ref, computed, watch, onBeforeUnmount } from 'vue';
import { markAccessibleAttachmentExecuted } from '@/api/applications';
import { useDeletionsStore } from '@/stores/deletions';

const props = defineProps({
  /** Заголовок вложения (detail.attachment) - attachment_id и execution_marked_until. */
  attachment: { type: Object, default: null },
});

const deletions = useDeletionsStore();

// localUntil - свежий срок после успешного клика. До первого клика источник - поле с
// бэка: деталь вложения уже несёт текущее состояние окна при открытии.
const localUntil = ref(null);
const submitting = ref(false);
const now = ref(Date.now());
let ticker = null;

function stopTicker() {
  if (ticker) {
    clearInterval(ticker);
    ticker = null;
  }
}

const markedUntilMs = computed(() => {
  const raw = localUntil.value ?? props.attachment?.execution_marked_until ?? null;
  const ms = raw ? new Date(raw).getTime() : NaN;
  return Number.isNaN(ms) ? null : ms;
});

// Тикер живёт, только пока есть что отсчитывать - на пустом вложении/до отметки
// компонент не занимает таймер впустую.
watch(
  markedUntilMs,
  (until) => {
    stopTicker();
    if (until) ticker = setInterval(() => { now.value = Date.now(); }, 1000);
  },
  { immediate: true },
);
onBeforeUnmount(stopTicker);

const canMark = computed(() => !submitting.value && (!markedUntilMs.value || now.value >= markedUntilMs.value));

const label = computed(() => {
  if (submitting.value) return 'Отмечаю...';
  if (!canMark.value) {
    const left = Math.max(0, Math.ceil((markedUntilMs.value - now.value) / 1000));
    const m = Math.floor(left / 60);
    const s = String(left % 60).padStart(2, '0');
    return `Отмечено, повтор через ${m}:${s}`;
  }
  return 'Отметить как исполненное';
});

async function mark() {
  if (!canMark.value || !props.attachment) return;
  submitting.value = true;
  try {
    const data = await markAccessibleAttachmentExecuted(props.attachment.attachment_id);
    localUntil.value = data.execution_marked_until;
    now.value = Date.now();
    deletions.notify({ prefix: 'Вложение отмечено исполненным', type: 'success' });
  } catch (e) {
    deletions.notify({ prefix: e.message || 'Не удалось отметить вложение', type: 'error' });
  } finally {
    submitting.value = false;
  }
}
</script>

<style scoped>
/* margin, не gap родителя: .detail-actions (AccessibleAttachmentsView) уже за порогом
   размера style-блока и не может прирасти ни на строку - отступ между кнопками несёт
   на себе тот, кто добавился вторым. */
.execution-mark {
  margin-left: 12px;
  white-space: nowrap;
}
.execution-mark:first-child {
  margin-left: 0;
}
</style>
