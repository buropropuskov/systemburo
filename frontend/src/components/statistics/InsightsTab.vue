<template>
  <div class="insights">

    <!-- Ошибка загрузки -->
    <div
      v-if="error"
      class="insights__state insights__state--error"
    >
      <p>{{ error }}</p>
      <button
        class="lk-button lk-button--ghost"
        @click="load"
      >
        Повторить
      </button>
    </div>

    <!-- Загрузка -->
    <div
      v-else-if="loading"
      class="insights__skeletons"
    >
      <div
        v-for="n in 4"
        :key="n"
        class="insights__skeleton"
      />
    </div>

    <!-- Пусто -->
    <div
      v-else-if="isEmpty"
      class="insights__state"
    >
      <p>За выбранный период нет данных для инсайтов.</p>
    </div>

    <template v-else>
      <!-- ===== СРАВНЕНИЕ С ПРОШЛЫМ ПЕРИОДОМ ===== -->
      <section
        v-if="data.comparisons.length"
        class="insights__section"
      >
        <div class="insights__section-head">
          <h2 class="insights__section-title">Сравнение с прошлым периодом</h2>
          <span class="insights__section-chip">текущий период против предыдущего равной длины</span>
          <span class="insights__section-rule" />
        </div>

        <div class="insights__cards">
          <div
            v-for="c in data.comparisons"
            :key="c.metric"
            class="insights__cmp"
          >
            <div class="insights__cmp-label">{{ c.label }}</div>
            <div class="insights__cmp-current">{{ fmt(c.current) }}</div>
            <div class="insights__cmp-foot">
              <span class="insights__cmp-prev">было {{ fmt(c.previous) }}</span>
              <span
                class="insights__delta"
                :class="`insights__delta--${c.direction}`"
              >
                <DirIcon :direction="c.direction" />
                {{ deltaText(c.delta_pct) }}
              </span>
            </div>
          </div>
        </div>
      </section>

      <!-- ===== ПИК НАГРУЗКИ ПО ЧАСАМ ===== -->
      <section
        v-if="data.peak_hours.length"
        class="insights__section"
      >
        <div class="insights__section-head">
          <h2 class="insights__section-title">Пик нагрузки по часам</h2>
          <span class="insights__section-chip">распределение по часам суток</span>
          <span class="insights__section-rule" />
        </div>

        <div class="insights__peaks">
          <div
            v-for="p in data.peak_hours"
            :key="p.metric"
            class="insights__peak"
          >
            <div class="insights__peak-head">
              <h3 class="insights__peak-title">{{ p.label }}</h3>
              <span class="insights__peak-badge">
                Пик в {{ hourLabel(p.peak_hour) }} — {{ fmt(p.peak_value) }}<template v-if="p.unit"> {{ p.unit }}</template>
              </span>
            </div>
            <div class="insights__peak-chart">
              <ReportChart
                :rows="hourlyRows(p)"
                type="bar"
                :label="p.label"
                :unit="p.unit || ''"
              />
            </div>
          </div>
        </div>
      </section>

      <!-- ===== ТОП МЕСТ И ОРГАНИЗАЦИЙ ===== -->
      <section
        v-if="data.top_places.length || data.top_orgs.length"
        class="insights__section"
      >
        <div class="insights__section-head">
          <h2 class="insights__section-title">Топ за период</h2>
          <span class="insights__section-chip">лидеры по нагрузке</span>
          <span class="insights__section-rule" />
        </div>

        <div class="insights__tops">
          <InsightTopList
            title="Места разгрузки"
            subtitle="по въездам машин"
            :items="data.top_places"
          />
          <InsightTopList
            title="Организации"
            subtitle="по числу заявок"
            :items="data.top_orgs"
          />
        </div>
      </section>

      <!-- ===== ТРЕНДЫ ===== -->
      <section
        v-if="data.trends.length"
        class="insights__section"
      >
        <div class="insights__section-head">
          <h2 class="insights__section-title">Тренды по дням</h2>
          <span class="insights__section-chip">направление за период</span>
          <span class="insights__section-rule" />
        </div>

        <div class="insights__trends">
          <div
            v-for="t in data.trends"
            :key="t.metric"
            class="insights__trend"
          >
            <div class="insights__trend-label">{{ t.label }}</div>
            <Sparkline
              class="insights__trend-spark"
              :series="t.series"
              :direction="t.direction"
            />
            <span
              class="insights__delta insights__trend-dir"
              :class="`insights__delta--${t.direction}`"
            >
              <DirIcon :direction="t.direction" />
              {{ directionText(t.direction) }}
            </span>
          </div>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted, h } from 'vue';
import ReportChart from '@/components/statistics/ReportChart.vue';
import { getInsights } from '@/api/statistics.js';

const props = defineProps({
  from: {
    type: String,
    default: '',
  },
  to: {
    type: String,
    default: '',
  },
});

const data = reactive({
  peak_hours: [],
  comparisons: [],
  top_places: [],
  top_orgs: [],
  trends: [],
});
const loading = ref(false);
const error = ref('');

const isEmpty = computed(() =>
  !data.peak_hours.length
  && !data.comparisons.length
  && !data.top_places.length
  && !data.top_orgs.length
  && !data.trends.length
);

// Токен последовательности: быстрая смена периода пускает несколько запросов;
// результат пишет только последний, иначе медленный предыдущий ответ затирает
// актуальный (last-resolve-wins).
let loadSeq = 0;

async function load() {
  const seq = ++loadSeq;
  loading.value = true;
  error.value = '';
  try {
    const res = await getInsights(props.from, props.to);
    if (seq !== loadSeq) return;
    data.peak_hours = Array.isArray(res?.peak_hours) ? res.peak_hours : [];
    data.comparisons = Array.isArray(res?.comparisons) ? res.comparisons : [];
    data.top_places = Array.isArray(res?.top_places) ? res.top_places : [];
    data.top_orgs = Array.isArray(res?.top_orgs) ? res.top_orgs : [];
    data.trends = Array.isArray(res?.trends) ? res.trends : [];
  } catch (e) {
    if (seq !== loadSeq) return;
    error.value = e?.message || 'Не удалось загрузить инсайты';
  } finally {
    if (seq === loadSeq) loading.value = false;
  }
}

function refresh() {
  return load();
}

defineExpose({ refresh });

watch([() => props.from, () => props.to], load);
onMounted(load);

// ---- форматирование ----
function fmt(val) {
  if (val == null) return '—';
  return Number(val).toLocaleString('ru-RU');
}

function hourLabel(hour) {
  return `${String(hour).padStart(2, '0')}:00`;
}

// Почасовое распределение -> полный профиль суток 0..23 (пустые часы = 0),
// чтобы график читался как профиль дня, а не разреженный набор столбцов.
function hourlyRows(peak) {
  const byHour = new Map((peak.hourly || []).map((b) => [b.hour, b.value]));
  return Array.from({ length: 24 }, (_, h) => ({
    label: hourLabel(h),
    value: byHour.get(h) || 0,
  }));
}

function deltaText(pct) {
  // Бэкенд округляет до 1 знака, но JSON->Number может дать хвост (16.700000003);
  // повторно округляем, чтобы не показать артефакт.
  const n = Math.round((Number(pct) || 0) * 10) / 10;
  const sign = n > 0 ? '+' : '';
  return `${sign}${n}%`;
}

function directionText(dir) {
  if (dir === 'up') return 'рост';
  if (dir === 'down') return 'снижение';
  return 'без изменений';
}

// ---- иконка направления ----
const DirIcon = (p) => {
  const paths = {
    up: 'M3 14l5-6 5 6',
    down: 'M3 6l5 6 5-6',
    flat: 'M3 8h10',
  };
  return h('svg', {
    width: 14,
    height: 14,
    viewBox: '0 0 16 16',
    fill: 'none',
    stroke: 'currentColor',
    'stroke-width': 2,
    'stroke-linecap': 'round',
    'stroke-linejoin': 'round',
    'aria-hidden': 'true',
  }, [h('path', { d: paths[p.direction] || paths.flat })]);
};
DirIcon.props = ['direction'];

// ---- топ-список (leaderboard) ----
const InsightTopList = {
  props: {
    title: { type: String, required: true },
    subtitle: { type: String, default: '' },
    items: { type: Array, default: () => [] },
  },
  setup(p) {
    const max = computed(() =>
      p.items.reduce((m, it) => Math.max(m, Number(it.value) || 0), 0)
    );
    const barWidth = (v) => {
      const m = max.value;
      if (!m) return 0;
      return Math.max(4, Math.round((Number(v) || 0) / m * 100));
    };
    return () => h('div', { class: 'top' }, [
      h('div', { class: 'top__head' }, [
        h('h3', { class: 'top__title' }, p.title),
        p.subtitle ? h('span', { class: 'top__sub' }, p.subtitle) : null,
      ]),
      p.items.length === 0
        ? h('div', { class: 'top__empty' }, 'Нет данных')
        : h('ol', { class: 'top__list' }, p.items.map((it, i) =>
          h('li', { key: i, class: 'top__row' }, [
            h('span', { class: 'top__rank' }, String(i + 1)),
            h('div', { class: 'top__body' }, [
              h('div', { class: 'top__line' }, [
                h('span', { class: 'top__name', title: it.label }, it.label),
                h('span', { class: 'top__val' }, fmt(it.value)),
              ]),
              h('div', { class: 'top__bar' }, [
                h('span', { class: 'top__bar-fill', style: { width: `${barWidth(it.value)}%` } }),
              ]),
            ]),
          ]))),
    ]);
  },
};

// ---- спарклайн ----
const Sparkline = {
  props: {
    series: { type: Array, default: () => [] },
    direction: { type: String, default: 'flat' },
  },
  setup(p) {
    const W = 120;
    const H = 32;
    const PAD = 3;
    const colorByDir = { up: '#28a745', down: '#dc3545', flat: '#8a90a6' };
    const points = computed(() => {
      const s = p.series.map((v) => Number(v) || 0);
      if (s.length === 0) return '';
      if (s.length === 1) {
        const y = H / 2;
        return `${PAD},${y} ${W - PAD},${y}`;
      }
      const min = Math.min(...s);
      const max = Math.max(...s);
      const span = max - min || 1;
      const stepX = (W - PAD * 2) / (s.length - 1);
      return s.map((v, i) => {
        const x = PAD + i * stepX;
        const y = H - PAD - ((v - min) / span) * (H - PAD * 2);
        return `${x.toFixed(1)},${y.toFixed(1)}`;
      }).join(' ');
    });
    return () => h('svg', {
      class: 'spark',
      viewBox: `0 0 ${W} ${H}`,
      width: W,
      height: H,
      preserveAspectRatio: 'none',
      'aria-hidden': 'true',
    }, [
      h('polyline', {
        points: points.value,
        fill: 'none',
        stroke: colorByDir[p.direction] || colorByDir.flat,
        'stroke-width': 2,
        'stroke-linecap': 'round',
        'stroke-linejoin': 'round',
      }),
    ]);
  },
};
</script>

<style scoped>
.insights {
  display: flex;
  flex-direction: column;
  gap: 30px;
}

/* ===== СОСТОЯНИЯ ===== */
.insights__state {
  padding: 48px 20px;
  text-align: center;
  color: var(--color-text-muted);
  font-size: 14px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
}

.insights__state--error {
  color: var(--color-danger);
}

.insights__skeletons {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.insights__skeleton {
  height: 120px;
  border-radius: var(--radius-lg);
  background: var(--color-skeleton);
  animation: skeleton-pulse 1.4s ease-in-out infinite;
}

@keyframes skeleton-pulse {
  0%, 100% { opacity: 1; }
  50%       { opacity: 0.55; }
}

/* ===== СЕКЦИИ ===== */
.insights__section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.insights__section-head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.insights__section-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text);
  margin: 0;
  white-space: nowrap;
}

.insights__section-chip {
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-muted);
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  padding: 3px 10px;
  border-radius: var(--radius-pill);
  white-space: nowrap;
}

.insights__section-rule {
  flex: 1;
  height: 1px;
  background: var(--color-border);
}

/* ===== СРАВНЕНИЕ ===== */
.insights__cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 14px;
}

.insights__cmp {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 16px 18px;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
}

.insights__cmp:hover {
  border-color: var(--color-primary);
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

.insights__cmp-label {
  font-size: 12px;
  color: var(--color-text-muted);
  font-weight: 500;
}

.insights__cmp-current {
  font-size: 30px;
  font-weight: 700;
  color: var(--color-text);
  line-height: 1;
  margin: 8px 0 10px;
}

.insights__cmp-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.insights__cmp-prev {
  font-size: 12px;
  color: var(--color-text-muted);
}

/* ===== БЕЙДЖ ДЕЛЬТЫ / НАПРАВЛЕНИЯ ===== */
.insights__delta {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 700;
  padding: 3px 9px;
  border-radius: var(--radius-pill);
  white-space: nowrap;
}

.insights__delta--up {
  background: rgba(40, 167, 69, 0.12);
  color: var(--color-success);
}

.insights__delta--down {
  background: rgba(220, 53, 69, 0.12);
  color: var(--color-danger);
}

.insights__delta--flat {
  background: var(--color-bg);
  color: var(--color-text-muted);
  border: 1px solid var(--color-border);
}

/* ===== ПИК ПО ЧАСАМ ===== */
.insights__peaks {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 18px;
}

.insights__peak {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 18px 20px;
  background: #fff;
}

.insights__peak-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.insights__peak-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text);
  margin: 0;
}

.insights__peak-badge {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-primary);
  background: rgba(79, 91, 223, 0.1);
  border-radius: var(--radius-pill);
  padding: 4px 11px;
  white-space: nowrap;
}

.insights__peak-chart {
  height: 220px;
}

/* ===== ТОПЫ ===== */
.insights__tops {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

@media (max-width: 900px) {
  .insights__tops {
    grid-template-columns: 1fr;
  }
}

:deep(.top) {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 18px 20px;
  background: #fff;
}

:deep(.top__head) {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 14px;
}

:deep(.top__title) {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text);
  margin: 0;
}

:deep(.top__sub) {
  font-size: 11px;
  color: var(--color-text-muted);
}

:deep(.top__empty) {
  font-size: 13px;
  color: var(--color-text-muted);
  padding: 16px 0;
  text-align: center;
}

:deep(.top__list) {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

:deep(.top__row) {
  display: flex;
  align-items: center;
  gap: 12px;
}

:deep(.top__rank) {
  flex-shrink: 0;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  font-size: 11px;
  font-weight: 700;
  color: var(--color-text-muted);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

:deep(.top__body) {
  flex: 1;
  min-width: 0;
}

:deep(.top__line) {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 5px;
}

:deep(.top__name) {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

:deep(.top__val) {
  font-size: 13px;
  font-weight: 700;
  color: var(--color-text);
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}

:deep(.top__bar) {
  height: 7px;
  border-radius: var(--radius-pill);
  background: var(--color-bg);
  overflow: hidden;
}

:deep(.top__bar-fill) {
  display: block;
  height: 100%;
  border-radius: var(--radius-pill);
  background: var(--color-primary);
  transition: width 0.4s ease;
}

/* ===== ТРЕНДЫ ===== */
.insights__trends {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.insights__trend {
  display: flex;
  align-items: center;
  gap: 16px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 12px 18px;
  background: #fff;
}

.insights__trend-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  flex: 1;
  min-width: 0;
}

.insights__trend-spark {
  flex-shrink: 0;
  width: 120px;
  height: 32px;
}

.insights__trend-dir {
  flex-shrink: 0;
  min-width: 124px;
  justify-content: center;
}
</style>
