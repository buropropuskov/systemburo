<template>
  <div
    v-if="attachment"
    class="execution-mark"
  >
    <button
      type="button"
      class="lk-button lk-button--primary execution-mark__button"
      :disabled="!canMark"
      data-testid="aa-mark-executed"
      @click="mark"
    >
      {{ label }}
    </button>
    <p
      v-if="todayCount > 0"
      class="execution-mark__summary"
      data-testid="aa-mark-summary"
    >
      Сегодня отмечено {{ todayCount }} {{ timesLabel }}<template v-if="listTrimmed">, показаны последние {{ marksSummary.recent.length }}</template>
    </p>
    <ul
      v-if="todayCount > 0"
      class="execution-mark__list"
      data-testid="aa-mark-list"
    >
      <li
        v-for="(entry, i) in marksSummary.recent"
        :key="i"
        class="execution-mark__item"
      >
        <span class="execution-mark__time">{{ formatMarkTime(entry.created_at) }}</span>
        <span
          v-if="entry.actor_name"
          class="execution-mark__actor"
        >{{ entry.actor_name }}</span>
      </li>
    </ul>
  </div>
</template>

<script setup>
/**
 * Кнопка "Отметить как исполненное" во вкладке "Доступные мне" (#2446) плюс след
 * отметок под ней: сколько раз сегодня и кем (доп. запрос владельца после первого
 * PR). Вынесена отдельным компонентом - AccessibleAttachmentsView.vue уже за
 * порогом размера template/style, а такой объём разметки в него бы не поместился.
 *
 * Окно повтора (5 минут) и сводка за сегодня считает бэк - здесь только отображение
 * готовых значений и тиканье локального "сейчас", чтобы кнопка сама разблокировалась
 * без повторного открытия вложения.
 */
import { ref, computed, watch, onBeforeUnmount } from 'vue';
import { markAccessibleAttachmentExecuted } from '@/api/applications';
import { useDeletionsStore } from '@/stores/deletions';
import { pluralRu } from '@/utils/entityCount';
import { formatMoscow } from '@/utils/serverTime';

const props = defineProps({
  /** Заголовок вложения (detail.attachment) - attachment_id, execution_marked_until, execution_marks. */
  attachment: { type: Object, default: null },
});

// marked (#2446 доп.) - сигнал наверх после успешной отметки: сама сводка живёт в
// detail.attachment, а его перечитывает родитель (свежий execution_marks одним
// запросом вместе с остальной деталью, отдельного эндпоинта под сводку не заводили).
const emit = defineEmits(['marked']);

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

const marksSummary = computed(() => props.attachment?.execution_marks ?? null);
const todayCount = computed(() => marksSummary.value?.today_count ?? 0);
const timesLabel = computed(() => pluralRu(todayCount.value, ['раз', 'раза', 'раз']));
// Список ограничен сверху, а счётчик точный: без оговорки «показаны последние N» их
// расхождение читается как потерянные отметки.
const listTrimmed = computed(() => todayCount.value > (marksSummary.value?.recent?.length ?? 0));

/** Время отметки без даты - весь список и так за сегодня (см. GetAttachmentExecutionMarksSummary). */
function formatMarkTime(iso) {
  return formatMoscow(new Date(iso), { hour: '2-digit', minute: '2-digit' });
}

async function mark() {
  if (!canMark.value || !props.attachment) return;
  submitting.value = true;
  try {
    const data = await markAccessibleAttachmentExecuted(props.attachment.attachment_id);
    localUntil.value = data.execution_marked_until;
    now.value = Date.now();
    deletions.notify({ prefix: 'Вложение отмечено исполненным', type: 'success' });
    emit('marked');
  } catch (e) {
    deletions.notify({ prefix: e.message || 'Не удалось отметить вложение', type: 'error' });
  } finally {
    submitting.value = false;
  }
}
</script>

<style scoped>
/* Компонент стоит СНАРУЖИ .detail-actions отдельным блоком (не в общем флекс-ряду с
   "Посмотреть файл"): со сводкой и списком под кнопкой он не помещается в строку
   без искажения высоты соседней кнопки, а трогать style .detail-actions нельзя -
   он в AccessibleAttachmentsView.vue уже за порогом размера. Свой отступ несёт сам. */
.execution-mark {
  margin-top: 8px;
}
.execution-mark__summary {
  margin: 8px 0 4px;
  color: var(--text-muted);
  font-size: 13px;
}
.execution-mark__list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.execution-mark__item {
  display: flex;
  gap: 8px;
  font-size: 13px;
  color: var(--text-muted);
}
.execution-mark__time {
  font-variant-numeric: tabular-nums;
  color: var(--text);
}
</style>
