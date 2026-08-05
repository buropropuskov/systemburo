<template>
  <section class="archive-breakdown">
    <h4 class="archive-breakdown__title">
      Разбивка по периодам
    </h4>
    <p
      v-if="!yearGroups.length"
      class="archive-breakdown__empty"
    >
      Архив пока пуст
    </p>
    <div
      v-else
      class="archive-breakdown__table rt-table"
    >
      <div class="archive-breakdown__row archive-breakdown__row--head rt-head-row">
        <span class="archive-breakdown__cell archive-breakdown__cell--period">Период</span>
        <span class="archive-breakdown__cell archive-breakdown__cell--bytes">Место</span>
        <span class="archive-breakdown__cell archive-breakdown__cell--files">Файлов</span>
      </div>

      <template
        v-for="(group, index) in yearGroups"
        :key="group.year"
      >
        <button
          type="button"
          class="archive-breakdown__row archive-breakdown__row--year rt-row"
          :class="{ 'archive-breakdown__row--year-open': isExpanded(group.year, index) }"
          :aria-expanded="isExpanded(group.year, index)"
          @click="toggle(group.year, index)"
        >
          <span
            class="archive-breakdown__cell archive-breakdown__cell--period"
            data-label="Период"
          >
            <!-- Стрелка и год - одной группой: в карточке (мобилка) ячейка
                 раскладывается space-between, и отдельными детьми стрелка
                 уезжала бы в середину строки между подписью и годом. -->
            <span class="archive-breakdown__year">
              <svg
                class="archive-breakdown__caret"
                :class="{ 'archive-breakdown__caret--open': isExpanded(group.year, index) }"
                viewBox="0 0 10 6"
                fill="none"
                aria-hidden="true"
              >
                <path
                  d="M1 1L5 5L9 1"
                  stroke="currentColor"
                  stroke-width="1.5"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
              {{ group.year }}
            </span>
          </span>
          <span
            class="archive-breakdown__cell archive-breakdown__cell--bytes"
            data-label="Место"
          >{{ formatBytes(group.bytes) }}</span>
          <span
            class="archive-breakdown__cell archive-breakdown__cell--files"
            data-label="Файлов"
          >{{ group.fileCount }}</span>
        </button>

        <div
          class="archive-breakdown__months"
          :class="{ 'archive-breakdown__months--open': isExpanded(group.year, index) }"
        >
          <div class="archive-breakdown__months-inner">
            <div
              v-for="month in group.months"
              :key="month.month"
              class="archive-breakdown__row archive-breakdown__row--month rt-row"
            >
              <span
                class="archive-breakdown__cell archive-breakdown__cell--period"
                data-label="Период"
              >{{ formatMonthRu(month.month) }}</span>
              <span
                class="archive-breakdown__cell archive-breakdown__cell--bytes"
                data-label="Место"
              >{{ formatBytes(month.bytes) }}</span>
              <span
                class="archive-breakdown__cell archive-breakdown__cell--files"
                data-label="Файлов"
              >{{ month.file_count }}</span>
            </div>
          </div>
        </div>
      </template>
    </div>
  </section>
</template>

<script setup>
/**
 * Нижняя полоса вкладки «Обзор» (#1615, срез C2): разбивка файлового архива
 * по периодам в виде аккордеона год -> месяцы. Периоды приходят из
 * GET /file-archive/stats уже отсортированными по убыванию месяца
 * (внутри года порядок сохраняется), группировка по году идёт на фронте.
 */
import { computed, ref } from 'vue';
import { formatBytes } from '@/utils/download';
import { formatMonthRu } from '@/utils/datetime';

const props = defineProps({
  /** Помесячная разбивка архива: [{ month: 'YYYY-MM', bytes, file_count }, ...]. */
  periods: {
    type: Array,
    default: () => [],
  },
});

const yearGroups = computed(() => {
  const byYear = new Map();
  for (const period of props.periods) {
    const year = String(period.month || '').slice(0, 4);
    if (!year) continue;
    if (!byYear.has(year)) byYear.set(year, { year, bytes: 0, fileCount: 0, months: [] });
    const group = byYear.get(year);
    group.bytes += Number(period.bytes) || 0;
    group.fileCount += Number(period.file_count) || 0;
    group.months.push(period);
  }
  // periods уже отсортированы по убыванию — Map сохраняет порядок первого появления года.
  return Array.from(byYear.values());
});

// Явные тогглы пользователя хранятся отдельно от дефолта, чтобы обновление
// данных (RefreshButton) не схлопывало уже раскрытый год обратно.
const expandedOverrides = ref({});

function isExpanded(year, index) {
  if (year in expandedOverrides.value) return expandedOverrides.value[year];
  return index === 0; // самый свежий год открыт по умолчанию
}

function toggle(year, index) {
  expandedOverrides.value = { ...expandedOverrides.value, [year]: !isExpanded(year, index) };
}
</script>

<style scoped>
.archive-breakdown {
  margin-top: 28px;
}

.archive-breakdown__title {
  margin: 0 0 12px;
  font-size: 15px;
  font-weight: 600;
  color: var(--text);
}

.archive-breakdown__empty {
  margin: 0;
  padding: 20px;
  text-align: center;
  color: var(--text-muted);
  font-size: 14px;
  border: 1px dashed var(--border);
  border-radius: var(--radius-md);
}

.archive-breakdown__row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  box-sizing: border-box;
}

.archive-breakdown__row--head {
  padding: 0 16px 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.archive-breakdown__row--year {
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  padding: 12px 16px;
  font: inherit;
  font-weight: 600;
  color: var(--text);
  cursor: pointer;
  text-align: left;
  transition: border-color 0.18s ease;
}

.archive-breakdown__row--year:hover {
  border-color: var(--accent);
}

/* Раскрытый год и список его месяцев - одна карточка, а не две. Пока у года
   оставались нижние скругления, его рамка обрывалась на закруглениях, а рамка
   списка начиналась прямыми углами - между ними получались хвосты боковых
   линий, из-за которых список читался как оторванный от заголовка. Скругления
   снимаем без перехода: анимируются только transform и opacity. */
.archive-breakdown__row--year-open {
  border-bottom-left-radius: 0;
  border-bottom-right-radius: 0;
}

/* Рамку года наследует список: на стыке иначе стояли бы две линии подряд. */
.archive-breakdown__row--year-open:hover + .archive-breakdown__months .archive-breakdown__months-inner {
  border-color: var(--accent);
}

.archive-breakdown__row--month {
  padding: 10px 16px 10px 40px;
  color: var(--text-muted);
  font-size: 14px;
}

/* Сплошная тонкая линия вместо пунктирной: пунктир в списке из четырёх строк
   читался как обрывки, а не как разделители. */
.archive-breakdown__row--month + .archive-breakdown__row--month {
  border-top: 1px solid var(--border);
}

.archive-breakdown__cell--period {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.archive-breakdown__cell--bytes {
  flex: 0 0 120px;
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.archive-breakdown__cell--files {
  flex: 0 0 90px;
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.archive-breakdown__year {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.archive-breakdown__caret {
  flex-shrink: 0;
  width: 10px;
  height: 6px;
  color: var(--text-muted);
  transform: rotate(-90deg);
  transition: transform 0.2s ease;
}

.archive-breakdown__caret--open {
  transform: rotate(0deg);
}

/* Аккордеон месяцев: только transform/opacity анимировать нельзя (высота
   раскрывающегося блока), поэтому используется grid-template-rows 0fr -> 1fr
   (см. mobile-adaptive-etalon.md 3.3) - display не анимируется вовсе. */
.archive-breakdown__months {
  display: grid;
  grid-template-rows: 0fr;
  transition: grid-template-rows 0.25s ease;
}

.archive-breakdown__months--open {
  grid-template-rows: 1fr;
}

/* Рамка появляется только у раскрытого списка: схлопнутый блок иначе оставляет
   под карточкой года полоску в один пиксель - собственные границы, которым
   нечего обрамлять. */
.archive-breakdown__months-inner {
  overflow: hidden;
  min-height: 0;
  border: 0 solid var(--border);
  border-radius: 0 0 var(--radius-md) var(--radius-md);
}

.archive-breakdown__months--open .archive-breakdown__months-inner {
  border-width: 0 1px 1px;
}

/* Соседние группы «год»: между строкой-годом одной группы и строкой-годом
   следующей всегда стоит .archive-breakdown__months, поэтому глобальный
   .rt-row + .rt-row (responsive-tables.css) до неё не достаёт - зазор задан
   вручную здесь и в мобильном блоке ниже. */
.archive-breakdown__row--year {
  margin-top: 8px;
}

.archive-breakdown__row--year:first-of-type {
  margin-top: 0;
}

@media (max-width: 767.98px) {
  .archive-breakdown__row--head {
    display: none;
  }

  /* В карточке строка становится колонкой, и flex-basis ячеек (120px и 90px)
     превращается в ВЫСОТУ: карточка года раздувалась до 286px при вьюпорте 390
     вместо 106. Ширину ячейки в карточке задаёт responsive-tables.css. */
  .archive-breakdown__cell--period,
  .archive-breakdown__cell--bytes,
  .archive-breakdown__cell--files {
    flex: 0 0 auto;
  }

  .archive-breakdown__row--month {
    padding: 10px 14px;
  }

  .archive-breakdown__row--month + .archive-breakdown__row--month {
    border-top: none;
  }

  .archive-breakdown__months-inner {
    border: none;
    margin: 8px 0 0;
  }
}
</style>
