<template>
  <section class="archive-types">
    <h4 class="archive-types__title">
      По типам вложений
    </h4>
    <p
      v-if="!rows.length"
      class="archive-types__empty"
    >
      Бланков в архиве пока нет
    </p>
    <div
      v-else
      class="archive-types__table rt-table"
    >
      <div class="archive-types__row archive-types__row--head rt-head-row">
        <span class="archive-types__cell archive-types__cell--name">Тип вложения</span>
        <span class="archive-types__cell archive-types__cell--bytes">Место</span>
        <span class="archive-types__cell archive-types__cell--files">Файлов</span>
      </div>

      <div
        v-for="row in rows"
        :key="row.name"
        class="archive-types__row rt-row"
      >
        <span
          class="archive-types__cell archive-types__cell--name"
          data-label="Тип вложения"
        >
          <span
            class="archive-types__share"
            :style="{ width: row.share + '%' }"
            aria-hidden="true"
          />
          <span class="archive-types__name">{{ row.name }}</span>
        </span>
        <span
          class="archive-types__cell archive-types__cell--bytes"
          data-label="Место"
        >{{ formatBytes(row.bytes) }}</span>
        <span
          class="archive-types__cell archive-types__cell--files"
          data-label="Файлов"
        >{{ row.file_count }}</span>
      </div>
    </div>
  </section>
</template>

<script setup>
/**
 * Разбивка архива по типам вложений (#1615 followup, срез S2). Отвечает на
 * вопрос «чего в архиве больше всего»: одна плитка «Бланков: 191» его не
 * закрывает. Данные приходят из GET /file-archive/stats уже отсортированными по
 * убыванию объёма - порядок бэка сохраняем, на фронте считается только доля
 * полосы, и та от самого тяжёлого типа, а не от всего архива: доли от целого на
 * длинном хвосте типов вырождаются в невидимые полоски.
 */
import { computed } from 'vue';
import { formatBytes } from '@/utils/download';

const props = defineProps({
  /** [{ name, bytes, file_count }, ...], тяжёлые сверху. */
  types: {
    type: Array,
    default: () => [],
  },
});

const rows = computed(() => {
  const list = props.types || [];
  const max = list.reduce((acc, t) => Math.max(acc, Number(t.bytes) || 0), 0);
  return list.map((t) => ({
    name: t.name || 'Без наименования',
    bytes: Number(t.bytes) || 0,
    file_count: Number(t.file_count) || 0,
    share: max ? Math.max(2, Math.round(((Number(t.bytes) || 0) / max) * 100)) : 0,
  }));
});
</script>

<style scoped>
.archive-types {
  margin-top: 28px;
}

.archive-types__title {
  margin: 0 0 12px;
  font-size: 15px;
  font-weight: 600;
  color: var(--text);
}

.archive-types__empty {
  margin: 0;
  padding: 20px;
  text-align: center;
  color: var(--text-muted);
  font-size: 14px;
  border: 1px dashed var(--border);
  border-radius: var(--radius-md);
}

.archive-types__row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  box-sizing: border-box;
}

.archive-types__row--head {
  padding: 0 16px 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.archive-types__row:not(.archive-types__row--head) {
  padding: 10px 16px;
  border-bottom: 1px solid var(--border);
  font-size: 14px;
  color: var(--text);
}

.archive-types__row:last-child {
  border-bottom: none;
}

.archive-types__cell--name {
  flex: 1 1 auto;
  min-width: 0;
  position: relative;
  display: flex;
  align-items: center;
}

/* Полоса доли лежит подложкой под названием: отдельная колонка под неё съела бы
   место у самого названия, а оно и есть главное в строке. Полоса и текст
   отступают от края одинаково - иначе буквы упираются в кромку заливки. */
.archive-types__share {
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  height: 26px;
  border-radius: var(--radius-sm, 8px);
  background: color-mix(in srgb, var(--accent) 18%, transparent);
  pointer-events: none;
}

.archive-types__name {
  position: relative;
  padding-left: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.archive-types__cell--bytes {
  flex: 0 0 110px;
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.archive-types__cell--files {
  flex: 0 0 80px;
  text-align: right;
  font-variant-numeric: tabular-nums;
}

@media (max-width: 767.98px) {
  .archive-types__cell--bytes,
  .archive-types__cell--files {
    flex: 0 0 auto;
    text-align: left;
  }
}
</style>
