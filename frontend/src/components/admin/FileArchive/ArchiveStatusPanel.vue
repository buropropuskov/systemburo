<template>
  <div class="archive-status">
    <p
      v-if="error"
      class="archive-status__error"
    >
      {{ error }}
    </p>
    <template v-else>
      <div
        v-if="loading && !stats"
        class="archive-status__tiles"
      >
        <div
          v-for="n in 6"
          :key="n"
          class="archive-status__tile archive-status__tile--skeleton"
        />
      </div>
      <div
        v-else
        class="archive-status__tiles"
      >
        <div
          v-for="tile in tiles"
          :key="tile.key"
          class="archive-status__tile"
          :title="tile.hint || null"
        >
          <div class="archive-status__tile-label">
            {{ tile.label }}
          </div>
          <div
            class="archive-status__tile-val"
            :class="{ 'archive-status__tile-val--wide': tile.wide }"
          >
            <AnimatedCounter
              v-if="tile.animated"
              :value="tile.value"
            />
            <template v-else>
              {{ tile.display }}
            </template>
          </div>
        </div>
      </div>

      <p
        v-if="snapshotLabel"
        class="archive-status__snapshot"
      >
        Данные на {{ snapshotLabel }}, пересчитываются не чаще раза в пять минут.
      </p>

      <section
        v-if="stats"
        class="archive-status__disk"
      >
        <div class="archive-status__disk-head">
          <h4 class="archive-status__disk-title">
            Занятость раздела диска
          </h4>
          <BaseDropdown
            v-if="partitionOptions.length > 1"
            v-model="selectedPartitionIndex"
            class="archive-status__disk-select"
            :options="partitionOptions"
            label-key="label"
            value-key="index"
          />
        </div>

        <div class="archive-status__disk-barwrap">
          <div
            class="archive-status__disk-bar"
            :class="diskBarClass"
          >
            <span
              v-for="seg in barSegments"
              :key="seg.key"
              class="archive-status__disk-seg"
              :class="{ 'archive-status__disk-seg--active': hoveredSegment?.key === seg.key }"
              :style="{ width: seg.percent + '%', background: seg.color }"
              :title="`${seg.label}: ${formatBytes(seg.bytes)}`"
              tabindex="0"
              @mouseenter="hoveredSegment = seg"
              @mouseleave="hoveredSegment = null"
              @focus="hoveredSegment = seg"
              @blur="hoveredSegment = null"
            />
          </div>

          <div
            v-if="hoveredSegment"
            class="archive-status__disk-tip"
            :style="tipStyle"
          >
            <span
              class="archive-status__disk-dot"
              :style="{ background: hoveredSegment.color }"
            />
            <span>
              {{ hoveredSegment.label }}: {{ formatBytes(hoveredSegment.bytes) }}
              <span class="archive-status__disk-tip-share">{{ formatShare(hoveredSegment.percent) }}</span>
              <span
                v-if="hoveredSegment.note"
                class="archive-status__disk-tip-note"
              >{{ hoveredSegment.note }}</span>
            </span>
          </div>
        </div>

        <ul class="archive-status__disk-legend">
          <li
            v-for="seg in diskSegments"
            :key="seg.key"
          >
            <span
              class="archive-status__disk-dot"
              :style="{ background: seg.color }"
            />
            {{ seg.label }}: {{ formatBytes(seg.bytes) }}
          </li>
        </ul>

        <p
          v-if="selectedPartitionLabels.length"
          class="archive-status__disk-caption"
        >
          На этом разделе также: {{ selectedPartitionLabels.join(', ') }}. Их состав отдельно
          не измерялся - раздел показан целиком как «Занято» / «Свободно».
        </p>
      </section>

      <ArchiveSizeBreakdown :periods="stats?.periods || []" />
      <ArchiveTypeBreakdown :types="stats?.attachment_types || []" />
    </template>
  </div>
</template>

<script setup>
/**
 * Вкладка «Обзор» файлового архива (#1615, срез C2): плитки статуса, полоса
 * занятости диска (с выбором раздела) и разбивка по периодам. Единственный
 * источник данных - GET /file-archive/stats (срез B2), опрашивается один раз
 * при монтировании и вручную по RefreshButton родителя (см. defineExpose).
 */
import { computed, onMounted, ref } from 'vue';
import AnimatedCounter from '@/components/ui/AnimatedCounter.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import ArchiveSizeBreakdown from './ArchiveSizeBreakdown.vue';
import ArchiveTypeBreakdown from './ArchiveTypeBreakdown.vue';
import { getArchiveStats } from '@/api/fileArchive';
import { formatBytes } from '@/utils/download';
import { formatDateTime, formatMonthRu } from '@/utils/datetime';
import { useDeletionsStore } from '@/stores/deletions';

// Пороги свободного места на полосе диска (руководство по развёртыванию,
// раздел 12.4) - раздельны от archive.warn_percent из настроек: тот считает
// долю, которую занимает САМ архив в своей квоте, этот - долю свободного
// места физического раздела в целом.
const FREE_WARN_PERCENT = 20;
const FREE_CRITICAL_PERCENT = 10;

const loading = ref(false);
const error = ref('');
const stats = ref(null);
const selectedPartitionIndex = ref(0);

async function load() {
  loading.value = true;
  error.value = '';
  try {
    stats.value = await getArchiveStats();
    const count = stats.value?.disk?.partitions?.length || 0;
    if (selectedPartitionIndex.value >= count) selectedPartitionIndex.value = 0;
  } catch (e) {
    error.value = e?.message || 'Не удалось загрузить сводку файлового архива';
    useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'сводку файлового архива', type: 'error' });
  } finally {
    loading.value = false;
  }
}

onMounted(load);
defineExpose({ refresh: load });

// Момент последней записи показываем полностью - до минуты. Месяц («Август 2026»)
// на этот вопрос не отвечает: администратор смотрит сюда, когда выясняет, пишется
// ли архив прямо сейчас, а не в каком месяце лежат файлы.
const lastWrittenLabel = computed(() => {
  const written = stats.value?.last_written_at;
  // Год у записи этого года не показываем: полный момент «04.08.2026 18:49» не
  // помещается в плитку и переносом растягивал весь ряд. Для прошлых лет год
  // важнее минут - там показываем дату целиком, без времени.
  if (written) {
    const full = formatDateTime(written);
    const [date, time] = full.split(' ');
    const year = date?.slice(-4);
    return year === String(new Date().getFullYear()) ? `${date.slice(0, 5)}, ${time}` : date;
  }
  // Пустой архив и архив, у которого записи есть, но момент неизвестен, - разные
  // вещи: во втором случае месяц из разбивки всё же лучше прочерка.
  const first = stats.value?.periods?.[0];
  return first ? formatMonthRu(first.month) : '—';
});

// Сводка кэшируется на сервере 5 минут, поэтому свежая запись появляется здесь не
// мгновенно. Без этой подписи задержка читается как «архив встал».
const snapshotLabel = computed(() => {
  const at = stats.value?.generated_at;
  return at ? formatDateTime(at) : '';
});

const tiles = computed(() => {
  const s = stats.value;
  const composition = s?.composition || {};
  return [
    // Занято и свободно - пара: одно без другого ничего не говорит, поэтому
    // стоят рядом, а не по разным концам ряда.
    { key: 'used', label: 'Занято архивом', display: formatBytes(s?.used_bytes ?? 0) },
    { key: 'free', label: 'Свободно на диске', display: formatBytes(s?.free_bytes ?? 0) },
    { key: 'applications', label: 'Заявок', value: composition.applications ?? 0, animated: true },
    { key: 'blanks', label: 'Бланков', value: composition.blanks ?? 0, animated: true },
    {
      key: 'snapshots',
      label: 'Описаний заявок',
      value: composition.snapshots ?? 0,
      animated: true,
      hint: 'Машиночитаемое описание заявки рядом с бланками - по одному на заявку',
    },
    {
      key: 'last',
      label: 'Последняя запись',
      display: lastWrittenLabel.value,
      // Полный момент со всеми цифрами остаётся в подсказке: на экране он не
      // помещается, но при разборе «когда именно» он и нужен.
      hint: stats.value?.last_written_at ? formatDateTime(stats.value.last_written_at) : null,
      wide: true,
    },
    { key: 'errors', label: 'Ошибок', value: s?.statuses?.failed ?? 0, animated: true },
    { key: 'no_template', label: 'Без шаблона', value: s?.statuses?.no_template ?? 0, animated: true },
  ];
});

const partitionOptions = computed(() => {
  const partitions = stats.value?.disk?.partitions || [];
  return partitions.map((p, index) => ({
    index,
    label: (p.labels && p.labels.length) ? p.labels.join(' + ') : `Раздел ${index + 1}`,
  }));
});

const selectedPartition = computed(
  () => stats.value?.disk?.partitions?.[selectedPartitionIndex.value] || null,
);

// Полный состав (архив/загрузки/база/логи/прочее) посчитан бэком только для
// раздела, на котором физически лежит корень архива (см. B2,
// BlankExportQuotaService.diskUsage) - у остальных разделов из выпадашки есть
// только total/free, дальше без выдумывания состав не рисуем.
const isPrimaryPartition = computed(() => {
  const p = selectedPartition.value;
  return !p || (p.labels || []).includes('Архив');
});

const selectedPartitionLabels = computed(() => {
  if (isPrimaryPartition.value) return [];
  return (selectedPartition.value?.labels || []).filter((l) => l !== 'Архив');
});

const diskSegments = computed(() => {
  const disk = stats.value?.disk;
  if (!disk) return [];
  const partition = selectedPartition.value;
  const total = partition ? partition.total_bytes : disk.total_bytes;
  const free = partition ? partition.free_bytes : disk.free_bytes;
  if (!total) return [];

  if (isPrimaryPartition.value) {
    const raw = [
      { key: 'archive', label: 'Архив бланков', bytes: disk.archive_bytes || 0, color: 'var(--accent)' },
      {
        key: 'uploads',
        label: 'Файлы из заявок',
        bytes: disk.uploads_bytes || 0,
        color: '#7c8cf5',
        note: 'документы и фотографии, приложенные к заявкам',
      },
      {
        key: 'database',
        label: 'База данных',
        bytes: disk.database_bytes || 0,
        color: 'var(--warning)',
        note: 'сами заявки, справочники, журналы',
      },
      { key: 'logs', label: 'Журналы работы', bytes: disk.logs_bytes || 0, color: '#9333ea' },
      {
        key: 'other',
        label: 'Прочее',
        bytes: disk.other_bytes || 0,
        color: 'var(--text-muted)',
        note: 'всё, что на диске не от системы: образы контейнеров, сама операционная система',
      },
      { key: 'free', label: 'Свободно', bytes: free || 0, color: 'var(--border)' },
    ];
    return raw.map((s) => ({ ...s, percent: (s.bytes / total) * 100 }));
  }

  const used = Math.max(0, total - free);
  return [
    { key: 'used', label: 'Занято', bytes: used, color: 'var(--text-muted)', percent: (used / total) * 100 },
    { key: 'free', label: 'Свободно', bytes: free, color: 'var(--border)', percent: (free / total) * 100 },
  ];
});

// В полосе рисуем только непустые доли: сегменту задан минимум ширины (иначе
// доли в сотые доли процента схлопываются в невидимую нитку на узком экране), и
// без этого фильтра нулевая доля получила бы ту же нитку и читалась как занятое
// место. Подписи со значениями идут легендой под полосой - там пустые остаются.
const barSegments = computed(() => diskSegments.value.filter((s) => s.bytes > 0));

const freePercent = computed(() => {
  const disk = stats.value?.disk;
  if (!disk) return null;
  const partition = selectedPartition.value;
  const total = partition ? partition.total_bytes : disk.total_bytes;
  const free = partition ? partition.free_bytes : disk.free_bytes;
  return total ? (free / total) * 100 : null;
});

const diskBarClass = computed(() => {
  const p = freePercent.value;
  if (p == null) return '';
  if (p < FREE_CRITICAL_PERCENT) return 'archive-status__disk-bar--critical';
  if (p < FREE_WARN_PERCENT) return 'archive-status__disk-bar--warning';
  return '';
});

// Сегмент, на который сейчас наведено (или который получил фокус с клавиатуры).
// Легенда под полосой отвечает на вопрос «что тут вообще есть», подсказка - на
// вопрос «а вот эта конкретная полоска чья».
const hoveredSegment = ref(null);

// Положение подсказки считаем из накопленных долей, а не из геометрии узла:
// проценты и так есть, а замер через getBoundingClientRect на этом проекте
// требует поправки на масштаб корня и врёт во время анимаций.
const tipStyle = computed(() => {
  const seg = hoveredSegment.value;
  if (!seg) return {};
  let offset = 0;
  for (const s of barSegments.value) {
    if (s.key === seg.key) break;
    offset += s.percent;
  }
  const center = offset + seg.percent / 2;
  // У краёв подсказку прижимаем к соответствующей стороне: центрированная
  // уехала бы за пределы карточки и обрезалась.
  if (center < 12) return { left: '0%', transform: 'none' };
  if (center > 88) return { left: '100%', transform: 'translateX(-100%)' };
  return { left: `${center}%`, transform: 'translateX(-50%)' };
});

// Доля раздела: меньше десятой процента показываем как «<0.1 %», иначе на
// мелких сегментах подсказка сообщала бы «0 %» рядом с ненулевым размером.
function formatShare(percent) {
  const value = Number(percent) || 0;
  if (value > 0 && value < 0.1) return '<0.1 %';
  return `${value.toFixed(1)} %`;
}
</script>

<style scoped>
.archive-status__error {
  color: var(--danger-text);
  font-size: 14px;
  margin: 0;
}

/* ===== ПЛИТКИ ===== */
.archive-status__tiles {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 12px;
}

.archive-status__tile {
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  padding: 14px 16px;
}

.archive-status__tile--skeleton {
  height: 66px;
  background: linear-gradient(90deg, var(--surface-2) 25%, var(--surface-sunken) 37%, var(--surface-2) 63%);
  background-size: 400% 100%;
  animation: archive-status-shimmer 1.4s ease infinite;
  border-color: transparent;
}

@keyframes archive-status-shimmer {
  0% { background-position: 100% 50%; }
  100% { background-position: 0 50%; }
}

.archive-status__tile-label {
  font-size: 12px;
  color: var(--text-muted);
  margin-bottom: 6px;
}

.archive-status__tile-val {
  font-size: 20px;
  font-weight: 600;
  color: var(--text);
  font-variant-numeric: tabular-nums;
}

/* Значение из даты и времени длиннее числа в разы. Перенос на вторую строку
   растягивал по высоте ВЕСЬ ряд - плитки в сетке равной высоты, - поэтому такому
   значению даём свой кегль и запрещаем перенос. */
.archive-status__tile-val--wide {
  font-size: 17px;
  white-space: nowrap;
}

/* ===== ПОЛОСА ДИСКА ===== */
.archive-status__disk {
  margin-top: 24px;
}

.archive-status__disk-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.archive-status__disk-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text);
}

.archive-status__disk-select {
  width: 220px;
  flex-shrink: 0;
}

/* Подсказка всплывает поверх соседей, как ей и положено: резервировать под неё
   пустое место в потоке было ошибкой - ряд получал сорокапиксельную дыру,
   которую видно всегда, ради того, что появляется на наведение. */
.archive-status__disk-barwrap {
  position: relative;
  margin-top: 12px;
}

.archive-status__disk-bar {
  display: flex;
  height: 22px;
  border-radius: var(--radius-pill);
  overflow: hidden;
  background: var(--surface-2);
  border: 1px solid var(--border);
}

.archive-status__disk-bar--warning {
  border-color: var(--warning);
}

.archive-status__disk-bar--critical {
  border-color: var(--danger);
}

.archive-status__disk-seg {
  height: 100%;
  min-width: 3px;
  cursor: default;
  /* Только opacity - раскладка полосы от наведения не должна дрожать. */
  transition: opacity 150ms ease;
}

.archive-status__disk-seg:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}

/* Активной остаётся выделенная доля, остальные приглушаются: подсветить саму
   долю нечем - у сегментов свои цвета, и осветление ломало бы их узнавание. */
.archive-status__disk-bar:hover .archive-status__disk-seg,
.archive-status__disk-bar:focus-within .archive-status__disk-seg {
  opacity: 0.45;
}

/* Специфичность обязана совпадать с правилом приглушения выше (:hover считается
   за класс): при 0,2,0 против 0,3,0 активная доля гасла вместе с остальными. */
.archive-status__disk-bar:hover .archive-status__disk-seg--active,
.archive-status__disk-bar:focus-within .archive-status__disk-seg--active {
  opacity: 1;
}

.archive-status__disk-tip {
  position: absolute;
  bottom: calc(100% + 8px);
  /* Выше заголовка блока: подсказка на секунду закрывает его собой - это
     нормально для всплывающей подписи и дешевле, чем держать под неё пустоту. */
  z-index: 5;
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
  padding: 6px 10px;
  border-radius: var(--radius-md);
  background: var(--surface);
  border: 1px solid var(--border);
  box-shadow: 0 6px 18px rgb(0 0 0 / 18%);
  font-size: 12px;
  color: var(--text);
  pointer-events: none;
}

.archive-status__disk-tip-share {
  color: var(--text-muted);
}

/* Пояснение под названием доли: «Файлы из заявок» и «База данных» опознаются по
   имени, а вот что именно туда входит - нет. */
.archive-status__disk-tip-note {
  display: block;
  margin-top: 2px;
  color: var(--text-muted);
}

.archive-status__disk-legend {
  list-style: none;
  margin: 10px 0 0;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 8px 18px;
  font-size: 13px;
  color: var(--text-muted);
}

.archive-status__disk-legend li {
  display: flex;
  align-items: center;
  gap: 6px;
}

.archive-status__disk-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.archive-status__disk-caption {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--text-muted);
}

.archive-status__snapshot {
  margin: 10px 0 0;
  font-size: 12px;
  color: var(--text-muted);
}

@media (max-width: 767.98px) {
  .archive-status__tiles {
    grid-template-columns: repeat(2, 1fr);
  }

  .archive-status__disk-head {
    flex-direction: column;
    align-items: stretch;
  }

  .archive-status__disk-select {
    width: 100%;
  }

  /* Тач-таргет даём самой кнопке дропдауна - у неё свой min-height 30px,
     min-height обёртки её не растягивает (образец - AccessDenialsLog). */
  .archive-status__disk-select :deep(.base-dropdown__button) {
    min-height: 44px;
  }

  /* Легенда - единственная читаемая подпись долей на узком экране (в самой
     полосе мелкие доли занимают 3px), поэтому она не жмётся: перенос по
     строкам, подпись целиком, без сокращений. */
  .archive-status__disk-legend {
    gap: 6px 14px;
  }
}
</style>
