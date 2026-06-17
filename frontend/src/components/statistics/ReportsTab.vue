<template>
  <div class="reports">
    <!-- Загрузка каталога -->
    <div
      v-if="catalogLoading"
      class="reports__state"
    >
      <LoaderSpinner />
      <span>Загружаем конструктор отчётов…</span>
    </div>

    <div
      v-else-if="catalogError"
      class="reports__state reports__state--error"
    >
      {{ catalogError }}
    </div>

    <template v-else-if="catalog">
      <section class="reports__block">
        <h2 class="reports__heading">
          Сформировать отчёт
        </h2>
        <p class="reports__lead">
          Выберите показатель и разрез или выгрузку строк — результат появится ниже. Период берётся из фильтра вверху.
        </p>
        <ReportBuilder
          :catalog="catalog"
          :period="period"
          :loading="running"
          @run="onRun"
        />
      </section>

      <section class="reports__block reports__block--result">
        <h2 class="reports__heading">
          Результат
        </h2>
        <ReportResult
          :result="result"
          :loading="running"
          :error="runError"
        />
      </section>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import ReportBuilder from './ReportBuilder.vue';
import ReportResult from './ReportResult.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import { getReportCatalog, runReport } from '@/api/statistics';

const props = defineProps({
  from: { type: String, default: '' },
  to: { type: String, default: '' },
});

const period = computed(() => ({ from: props.from, to: props.to }));

const catalog = ref(null);
const catalogLoading = ref(true);
const catalogError = ref('');

const result = ref(null);
const running = ref(false);
const runError = ref('');

onMounted(async () => {
  try {
    catalog.value = await getReportCatalog();
  } catch (e) {
    catalogError.value = e?.message || 'Не удалось загрузить каталог отчётов';
  } finally {
    catalogLoading.value = false;
  }
});

async function onRun(request) {
  running.value = true;
  runError.value = '';
  try {
    result.value = await runReport(request);
  } catch (e) {
    result.value = null;
    runError.value = e?.message || 'Не удалось построить отчёт. Проверьте параметры.';
  } finally {
    running.value = false;
  }
}
</script>

<style scoped>
.reports {
  display: flex;
  flex-direction: column;
  gap: 28px;
}

.reports__state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 240px;
  color: var(--color-text-muted);
}

.reports__state--error {
  color: #c0392b;
}

.reports__block {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.reports__block--result {
  border-top: 1px solid var(--color-border);
  padding-top: 24px;
}

.reports__heading {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--color-text);
}

.reports__lead {
  margin: 0 0 12px;
  font-size: 13px;
  color: var(--color-text-muted);
}
</style>
