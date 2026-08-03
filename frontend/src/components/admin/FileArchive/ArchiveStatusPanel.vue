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
        >
          <div class="archive-status__tile-label">
            {{ tile.label }}
          </div>
          <div class="archive-status__tile-val">
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

        <div
          class="archive-status__disk-bar"
          :class="diskBarClass"
        >
          <span
            v-for="seg in barSegments"
            :key="seg.key"
            class="archive-status__disk-seg"
            :style="{ width: seg.percent + '%', background: seg.color }"
            :title="`${seg.label}: ${formatBytes(seg.bytes)}`"
          />
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
import { getArchiveStats } from '@/api/fileArchive';
import { formatBytes } from '@/utils/download';
import { formatMonthRu } from '@/utils/datetime';
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

const latestPeriodLabel = computed(() => {
  const first = stats.value?.periods?.[0];
  return first ? formatMonthRu(first.month) : '—';
});

const tiles = computed(() => {
  const s = stats.value;
  return [
    { key: 'used', label: 'Занято', display: formatBytes(s?.used_bytes ?? 0) },
    { key: 'files', label: 'Файлов', value: s?.file_count ?? 0, animated: true },
    { key: 'free', label: 'Свободно', display: formatBytes(s?.free_bytes ?? 0) },
    { key: 'last', label: 'Последняя запись', display: latestPeriodLabel.value },
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
      { key: 'archive', label: 'Архив', bytes: disk.archive_bytes || 0, color: 'var(--accent)' },
      { key: 'uploads', label: 'Загрузки', bytes: disk.uploads_bytes || 0, color: '#7c8cf5' },
      { key: 'database', label: 'База', bytes: disk.database_bytes || 0, color: 'var(--warning)' },
      { key: 'logs', label: 'Логи', bytes: disk.logs_bytes || 0, color: '#9333ea' },
      { key: 'other', label: 'Прочее', bytes: disk.other_bytes || 0, color: 'var(--text-muted)' },
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

.archive-status__disk-bar {
  display: flex;
  height: 14px;
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
