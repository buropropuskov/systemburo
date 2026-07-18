<template>
  <div class="proc">
    <!-- Ошибка загрузки бандла -->
    <div
      v-if="errorMsg"
      class="proc__state proc__state--error"
    >
      {{ errorMsg }}
    </div>

    <!-- За период не подавали заявок: доли и длительности считать не по чему -->
    <div
      v-else-if="ready && isEmpty"
      class="proc__state"
    >
      За выбранный период заявок не подавали.
    </div>

    <template v-else>
      <!-- ===== ЭТАПЫ ОБРАБОТКИ ===== -->
      <section class="proc__group">
        <div class="proc__group-head">
          <h2 class="proc__group-title">Этапы обработки</h2>
          <span
            v-if="ready"
            class="proc__group-chip"
          >{{ totalChip }}</span>
          <span class="proc__group-rule" />
        </div>

        <div class="proc__tiles">
          <template v-if="showSkeleton">
            <div
              v-for="n in 4"
              :key="n"
              class="proc__tile proc__tile--skeleton"
            />
          </template>
          <template v-else>
            <div
              v-for="s in stages"
              :key="s.key"
              class="proc__tile"
            >
              <div class="proc__tile-label">
                <span class="proc__tile-name">{{ s.label }}</span>
                <span
                  v-if="stageMeta(s.key).hint"
                  class="proc__hint"
                  :data-hint="stageMeta(s.key).hint"
                  tabindex="0"
                  role="note"
                  :aria-label="stageMeta(s.key).hint"
                >i</span>
              </div>
              <div class="proc__tile-val">{{ fmtDur(s.avg) }}</div>
              <div class="proc__tile-meta">
                <span class="proc__tile-sub">p90: {{ fmtDur(s.p90) }}</span>
                <span
                  v-if="s.trend"
                  class="proc__delta"
                  :class="`proc__delta--${s.trend.sentiment}`"
                >
                  <DirIcon :direction="s.trend.direction" />
                  {{ deltaText(s.trend.delta_pct) }}
                </span>
              </div>
              <div class="proc__tile-foot">
                <span>{{ samplesText(s.samples) }}</span>
                <span
                  class="proc__basis"
                  :class="`proc__basis--${stageMeta(s.key).basis}`"
                >{{ stageMeta(s.key).basisLabel }}</span>
              </div>
            </div>
          </template>
        </div>
      </section>

      <!-- ===== УЗКИЕ МЕСТА ===== -->
      <section class="proc__group">
        <div class="proc__group-head">
          <h2 class="proc__group-title">Узкие места</h2>
          <span class="proc__group-chip">среднее время этапа</span>
          <span class="proc__group-rule" />
        </div>
        <div
          v-if="showSkeleton"
          class="proc__skeleton proc__skeleton--chart"
        />
        <div
          v-else
          class="proc__card"
        >
          <AnalyticsBarChart
            :data="bottleneckData"
            value-type="duration"
            series-name="Среднее время"
            :height="260"
          />
        </div>
      </section>

      <!-- ===== КАЧЕСТВО ОБРАБОТКИ ===== -->
      <section class="proc__group">
        <div class="proc__group-head">
          <h2 class="proc__group-title">Качество обработки</h2>
          <span class="proc__group-rule" />
        </div>
        <div class="proc__tiles">
          <template v-if="showSkeleton">
            <div
              v-for="n in 2"
              :key="n"
              class="proc__tile proc__tile--skeleton"
            />
          </template>
          <template v-else>
            <div
              v-for="q in quality"
              :key="q.key"
              class="proc__tile"
            >
              <div class="proc__tile-label">
                <span class="proc__tile-name">{{ q.label }}</span>
                <span
                  v-if="qualityHint(q.key)"
                  class="proc__hint"
                  :data-hint="qualityHint(q.key)"
                  tabindex="0"
                  role="note"
                  :aria-label="qualityHint(q.key)"
                >i</span>
              </div>
              <div class="proc__tile-val">{{ fmtQuality(q) }}</div>
              <div class="proc__tile-meta">
                <span
                  v-if="q.unit && q.unit !== '%'"
                  class="proc__tile-sub"
                >{{ q.unit }}</span>
                <span
                  v-else
                  class="proc__tile-sub"
                />
                <span
                  v-if="q.trend"
                  class="proc__delta"
                  :class="`proc__delta--${q.trend.sentiment}`"
                >
                  <DirIcon :direction="q.trend.direction" />
                  {{ deltaText(q.trend.delta_pct) }}
                </span>
              </div>
            </div>
          </template>
        </div>
      </section>

      <!-- ===== СОГЛАСУЮЩИЕ И ОРГАНИЗАЦИИ ===== -->
      <div class="proc__cols">
        <!-- Медленные согласующие -->
        <section class="proc__group">
          <div class="proc__group-head">
            <h2 class="proc__group-title">Согласующие</h2>
            <span class="proc__group-chip">топ по времени реакции</span>
            <span class="proc__group-rule" />
          </div>
          <div
            v-if="showSkeleton"
            class="proc__skeleton proc__skeleton--table"
          />
          <div
            v-else
            class="proc__card proc__card--table"
          >
            <table class="proc__table">
              <thead>
                <tr>
                  <th>Согласующий</th>
                  <th class="proc__num">Время реакции</th>
                  <th class="proc__num">Нагрузка</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(a, i) in slowApprovers"
                  :key="i"
                >
                  <td
                    class="proc__ellipsis"
                    :title="a.name"
                  >{{ a.name }}</td>
                  <td class="proc__num">{{ fmtDur(a.avg_response_time) }}</td>
                  <td class="proc__num">{{ fmtCount(a.votes_count) }}</td>
                </tr>
                <tr v-if="slowApprovers.length === 0">
                  <td
                    colspan="3"
                    class="proc__table-empty"
                  >Нет данных</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <!-- Разбивка по организациям -->
        <section class="proc__group">
          <div class="proc__group-head">
            <h2 class="proc__group-title">По организациям</h2>
            <span class="proc__group-chip">время обработки</span>
            <span class="proc__group-rule" />
          </div>
          <div
            v-if="showSkeleton"
            class="proc__skeleton proc__skeleton--table"
          />
          <div
            v-else
            class="proc__card proc__card--table"
          >
            <table class="proc__table">
              <thead>
                <tr>
                  <th>Организация</th>
                  <th class="proc__num">Время обработки</th>
                  <th class="proc__num">Заявок</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(o, i) in byOrganization"
                  :key="i"
                >
                  <td
                    class="proc__ellipsis"
                    :title="o.label"
                  >{{ o.label }}</td>
                  <td class="proc__num">{{ fmtDur(o.avg_processing_time) }}</td>
                  <td class="proc__num">{{ fmtCount(o.applications_count) }}</td>
                </tr>
                <tr v-if="byOrganization.length === 0">
                  <td
                    colspan="3"
                    class="proc__table-empty"
                  >Нет данных</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue';
import { getProcessingSummary } from '@/api/statistics.js';
import { formatDuration } from '@/utils/datetime';
import AnalyticsBarChart from './AnalyticsBarChart.vue';
import DirIcon from './DirIcon.vue';

const props = defineProps({
  from: { type: String, default: '' },
  to: { type: String, default: '' },
});

const summary = ref(null);
const loading = ref(false);
const ready = ref(false);
const errorMsg = ref('');

// Бандл считается несколькими планами и приходит одним запросом; при быстрой смене
// периода медленный предыдущий ответ не должен затирать актуальный (seq-guard,
// как у loadSummary дашборда, урок #632).
let summarySeq = 0;
async function loadSummary() {
  const seq = ++summarySeq;
  loading.value = true;
  errorMsg.value = '';
  try {
    const data = await getProcessingSummary(props.from, props.to);
    if (seq !== summarySeq) return;
    summary.value = data || null;
  } catch (e) {
    if (seq !== summarySeq) return;
    summary.value = null;
    errorMsg.value = e?.message || 'Не удалось загрузить сводку обработки заявок';
  } finally {
    if (seq === summarySeq) {
      loading.value = false;
      ready.value = true;
    }
  }
}

const stages = computed(() => summary.value?.stages ?? []);
const quality = computed(() => summary.value?.quality ?? []);
const slowApprovers = computed(() => summary.value?.slow_approvers ?? []);
const byOrganization = computed(() => summary.value?.by_organization ?? []);
const totalApplications = computed(() => summary.value?.total_applications ?? 0);

// Пустой период — заявок не подавали: доли и длительности считать не по чему.
const isEmpty = computed(() => !summary.value || totalApplications.value === 0);

// Первая загрузка: показываем скелетоны во ВСЕХ секциях. Без этого график и
// таблицы (в отличие от плиток) на холодном заходе флэшнули бы «Нет данных» до
// прихода ответа — «пусто» и «ещё грузится» смешались бы. На рефреше (ready уже
// true) скелетон не мигает: до нового ответа держим прежние данные.
const showSkeleton = computed(() => loading.value && !ready.value);

// Столбцы узких мест = среднее время этапа. null (этап никто не прошёл) едет как
// null, график рисует разрыв, а не нулевой столбец (valueType=duration, F1b).
const bottleneckData = computed(() =>
  stages.value.map((s) => ({ label: s.label, value: s.avg })),
);

const totalChip = computed(() => `${totalApplications.value} ${plural(totalApplications.value, APP_FORMS)} за период`);

const APP_FORMS = ['заявка', 'заявки', 'заявок'];

function plural(n, forms) {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return forms[0];
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return forms[1];
  return forms[2];
}

// Пояснение «что считается» для каждого этапа + основа времени (#1251 S5): три этапа
// работы людей бюро считаются по рабочему времени (ночь и выходные вычитаются, S2),
// «время до завершения» — календарный срок пропуска. Тултипы снимают путаницу между
// похожими «принятием» и «обработкой», а пометка объясняет, почему сроки меньше
// календарных.
const STAGE_META = {
  approval_time: {
    hint: 'От подачи заявки до согласования. Считается по рабочему времени бюро: ночь и выходные не учитываются.',
    basis: 'work',
  },
  acceptance_time: {
    hint: 'От согласования до принятия заявки в работу принимающим. По рабочему времени бюро.',
    basis: 'work',
  },
  processing_time: {
    hint: 'Полное время от подачи до принятия в работу (согласование и принятие вместе). По рабочему времени бюро.',
    basis: 'work',
  },
  completion_time: {
    hint: 'Срок действия пропуска: от подачи до завершения по истечении срока вложений. Календарное время — рабочие часы бюро тут не вычитаются.',
    basis: 'calendar',
  },
};

const BASIS_LABEL = { work: 'раб. время бюро', calendar: 'календарное' };

function stageMeta(key) {
  const m = STAGE_META[key] || { hint: '', basis: 'work' };
  return { ...m, basisLabel: BASIS_LABEL[m.basis] };
}

// Пояснение «что считается» для метрик качества.
const QUALITY_HINTS = {
  refusal_rate: 'Доля заявок, по которым отказано (статус «Отказано» или несогласование), от всех поданных за период.',
  avg_forwards: 'Среднее число пересылок заявки другим сотрудникам за период.',
};

function qualityHint(key) {
  return QUALITY_HINTS[key] || '';
}

// Длительность: null = «нет данных» (этап не прошёл никто) -> прочерк, НЕ «0 мин».
function fmtDur(seconds) {
  return seconds == null ? '—' : formatDuration(seconds);
}

// Доля/среднее качества: значение уже в единицах метрики (refusal_rate — проценты
// 0..100, avg_forwards — число), бэк округлил до одного знака. Проценты пишем
// слитно, прочие единицы уходят подписью рядом.
function fmtQuality(q) {
  if (q.value == null) return '—';
  const num = Number(q.value).toLocaleString('ru-RU', { maximumFractionDigits: 1 });
  return q.unit === '%' ? `${num}%` : num;
}

function fmtCount(n) {
  return Number(n || 0).toLocaleString('ru-RU');
}

function samplesText(n) {
  return `${fmtCount(n)} ${plural(Number(n) || 0, APP_FORMS)}`;
}

// Знак изменения; значение к прошлому периоду уже округлено бэком (roundOne).
function deltaText(pct) {
  const r = Math.round((Number(pct) || 0) * 10) / 10;
  return `${r > 0 ? '+' : ''}${r}%`;
}

watch(
  () => [props.from, props.to],
  () => loadSummary(),
);
onMounted(loadSummary);

defineExpose({ refresh: loadSummary });
</script>

<style scoped>
.proc {
  display: flex;
  flex-direction: column;
  gap: 30px;
}

/* ===== СОСТОЯНИЯ ===== */
.proc__state {
  padding: 40px 0;
  text-align: center;
  font-size: 14px;
  color: var(--color-text-muted);
}

.proc__state--error {
  color: var(--color-danger);
}

/* ===== ГРУППА ===== */
.proc__group {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.proc__group-head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.proc__group-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text);
  margin: 0;
  white-space: nowrap;
}

.proc__group-chip {
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-muted);
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  padding: 3px 10px;
  border-radius: var(--radius-pill);
  white-space: nowrap;
}

.proc__group-rule {
  flex: 1;
  height: 1px;
  background: var(--color-border);
}

/* ===== ПЛИТКИ ===== */
.proc__tiles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
  gap: 12px;
}

.proc__tile {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 12px 14px;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
  animation: proc-tile-in 0.35s ease both;
}

.proc__tile:hover {
  border-color: var(--color-primary);
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

@keyframes proc-tile-in {
  from { opacity: 0; transform: translateY(10px); }
  to   { opacity: 1; transform: translateY(0); }
}

.proc__tile--skeleton {
  background: var(--color-skeleton);
  min-height: 96px;
  border: none;
  animation: proc-skeleton-pulse 1.4s ease-in-out infinite;
}

@keyframes proc-skeleton-pulse {
  0%, 100% { opacity: 1; }
  50%      { opacity: 0.55; }
}

.proc__tile-label {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  color: var(--color-text-muted);
  font-weight: 500;
  line-height: 1.3;
}

.proc__tile-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Подсказка «что считается» — системный стиль #333 (data-hint), не native title. */
.proc__hint {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  color: var(--color-text-muted);
  font-size: 9px;
  font-weight: 700;
  font-style: normal;
  line-height: 1;
  cursor: help;
  position: relative;
  flex-shrink: 0;
  user-select: none;
}

.proc__hint::after {
  content: attr(data-hint);
  position: absolute;
  bottom: calc(100% + 8px);
  left: 50%;
  transform: translateX(-50%);
  width: max-content;
  max-width: 240px;
  padding: 8px 10px;
  background: #333;
  color: #fff;
  font-size: 11px;
  font-weight: 400;
  line-height: 1.4;
  text-align: left;
  white-space: normal;
  border-radius: 8px;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.15s ease;
  z-index: 10;
}

.proc__hint::before {
  content: '';
  position: absolute;
  bottom: calc(100% + 3px);
  left: 50%;
  transform: translateX(-50%);
  border: 5px solid transparent;
  border-top-color: #333;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.15s ease;
}

.proc__hint:hover::after,
.proc__hint:focus::after,
.proc__hint:hover::before,
.proc__hint:focus::before {
  opacity: 1;
}

.proc__tile-val {
  font-size: 24px;
  font-weight: 700;
  color: var(--color-text);
  margin-top: 6px;
  line-height: 1.1;
}

.proc__tile-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 10px;
}

.proc__tile-sub {
  font-size: 11px;
  color: var(--color-text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.proc__tile-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 4px 8px;
  margin-top: 6px;
  font-size: 11px;
  color: var(--color-text-muted);
}

/* Пометка основы расчёта времени этапа: рабочее время бюро vs календарное. */
.proc__basis {
  font-size: 10px;
  font-weight: 600;
  padding: 1px 7px;
  border-radius: var(--radius-pill);
  white-space: nowrap;
  flex-shrink: 0;
}

.proc__basis--work {
  background: rgba(40, 167, 69, 0.1);
  color: var(--color-success);
}

.proc__basis--calendar {
  background: var(--color-bg);
  color: var(--color-text-muted);
  border: 1px solid var(--color-border);
}

/* ===== ДЕЛЬТА (цвет по тональности, не по направлению) ===== */
.proc__delta {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: 11px;
  font-weight: 700;
  padding: 2px 7px;
  border-radius: var(--radius-pill);
  white-space: nowrap;
  flex-shrink: 0;
}

/* good = стало меньше (лучше), bad = стало больше (хуже), neutral = без изменения.
   Стрелка (DirIcon) показывает направление, цвет — смысл: время могло упасть
   (стрелка вниз) и это хорошо -> зелёный. */
.proc__delta--good {
  background: rgba(40, 167, 69, 0.12);
  color: var(--color-success);
}

.proc__delta--bad {
  background: rgba(220, 53, 69, 0.12);
  color: var(--color-danger);
}

.proc__delta--neutral {
  background: var(--color-bg);
  color: var(--color-text-muted);
  border: 1px solid var(--color-border);
}

/* ===== КАРТОЧКИ ===== */
.proc__card {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 18px 20px;
}

.proc__card--table {
  padding: 6px 6px;
  overflow-x: auto;
}

/* Скелетон секций-карточек на первой загрузке (график/таблицы), в тон плиткам. */
.proc__skeleton {
  background: var(--color-skeleton);
  border-radius: var(--radius-lg);
  animation: proc-skeleton-pulse 1.4s ease-in-out infinite;
}

.proc__skeleton--chart {
  min-height: 296px;
}

.proc__skeleton--table {
  min-height: 190px;
}

/* ===== ДВЕ КОЛОНКИ ===== */
.proc__cols {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

@media (max-width: 900px) {
  .proc__cols {
    grid-template-columns: 1fr;
  }
}

/* ===== ТАБЛИЦА ===== */
.proc__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.proc__table th {
  text-align: left;
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-muted);
  padding: 10px 12px;
  border-bottom: 1px solid var(--color-border);
  white-space: nowrap;
}

.proc__table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--color-bg);
  color: var(--color-text);
}

.proc__table tbody tr:last-child td {
  border-bottom: none;
}

.proc__num {
  text-align: right;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.proc__ellipsis {
  max-width: 0;
  width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.proc__table-empty {
  text-align: center;
  color: var(--color-text-muted);
  padding: 18px 0;
}
</style>
