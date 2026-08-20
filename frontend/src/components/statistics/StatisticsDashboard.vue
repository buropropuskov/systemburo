<template>
  <div class="dashboard">

    <!-- ===== ГРУППА: ДАННЫЕ ===== -->
    <div class="dashboard__group">
      <div class="dashboard__group-head">
        <h2 class="dashboard__group-title">Данные</h2>
        <span class="dashboard__group-chip">бизнес-показатели за период</span>
        <span class="dashboard__group-rule" />
      </div>

      <div
        v-if="summaryLoading && !summaryReady"
        class="dashboard__tiles"
      >
        <div
          v-for="n in 6"
          :key="n"
          class="dashboard__tile dashboard__tile--skeleton"
        />
      </div>

      <div
        v-else
        class="dashboard__tiles"
      >
        <div
          v-for="tile in dataTiles"
          :key="tile.label"
          class="dashboard__tile"
          :class="{
            'dashboard__tile--clickable': tile.expandable,
            'dashboard__tile--active': tile.expandable && expandedMetric === tile.metric,
          }"
          :role="tile.expandable ? 'button' : null"
          :tabindex="tile.expandable ? 0 : null"
          :aria-expanded="tile.expandable ? (expandedMetric === tile.metric) : null"
          @click="tile.expandable && toggleExpand(tile.metric)"
          @keydown.enter="tile.expandable && toggleExpand(tile.metric)"
          @keydown.space.prevent="tile.expandable && toggleExpand(tile.metric)"
        >
          <div class="dashboard__tile-label">
            {{ tile.label }}
            <span
              v-if="tile.expandable"
              class="dashboard__tile-caret"
              aria-hidden="true"
            />
          </div>
          <div class="dashboard__tile-val">
            <AnimatedNumber :value="tile.value" />
          </div>
          <div
            v-if="tile.comparison || tile.trend"
            class="dashboard__tile-insight"
          >
            <TrendSparkline
              v-if="tile.trend"
              class="dashboard__tile-spark"
              :series="tile.trend.series"
              :direction="tile.trend.direction"
            />
            <span
              v-if="tile.comparison"
              class="dashboard__delta"
              :class="`dashboard__delta--${tile.comparison.direction}`"
            >
              <DirIcon :direction="tile.comparison.direction" />
              {{ deltaText(tile.comparison.delta_pct) }}
            </span>
          </div>
        </div>
      </div>

      <!-- Детальный разворот карточки: тренд по дням (area) + пик по часам (bar).
           Высота анимируется через grid-rows -> соседние группы съезжают плавно. -->
      <Transition name="dashboard-detail">
        <div
          v-if="expandedDetail"
          class="dashboard__detail-collapse"
        >
          <div class="dashboard__detail-collapse-inner">
            <div class="dashboard__detail">
              <div class="dashboard__detail-head">
                <h3 class="dashboard__detail-title">{{ expandedDetail.label }} — детально</h3>
                <button
                  type="button"
                  class="dashboard__detail-close"
                  aria-label="Свернуть"
                  @click="expandedMetric = null"
                >
                  ×
                </button>
              </div>
              <div class="dashboard__detail-charts">
                <div class="dashboard__detail-chart">
                  <div class="dashboard__detail-chart-title">Тренд по дням</div>
                  <AnalyticsAreaChart
                    :data="expandedDetail.trendData"
                    :height="220"
                    color="#4F5BDF"
                    :series-name="expandedDetail.label"
                    :unit-forms="expandedDetail.unitForms"
                  />
                </div>
                <div
                  v-if="expandedDetail.peak"
                  class="dashboard__detail-chart"
                >
                  <div class="dashboard__detail-chart-title">
                    Пик по часам
                    <span class="dashboard__detail-chart-note">
                      пик в {{ expandedDetail.peakHourLabel }}
                    </span>
                  </div>
                  <AnalyticsBarChart
                    :data="expandedDetail.peakData"
                    :height="220"
                    color="#4F5BDF"
                    :series-name="expandedDetail.label"
                    :unit-forms="expandedDetail.unitForms"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </div>

    <!-- ===== ВЛОЖЕНИЯ И PUSH: два блока в одну строку ===== -->
    <div class="an-pair">
      <div class="dashboard__group">
        <div class="dashboard__group-head">
          <h2 class="dashboard__group-title">Вложения</h2>
          <span class="dashboard__group-chip">по типам за период</span>
          <span class="dashboard__group-rule" />
        </div>

        <div
          v-if="summaryLoading && !summaryReady"
          class="dashboard__tiles"
        >
          <div
            v-for="n in 6"
            :key="n"
            class="dashboard__tile dashboard__tile--skeleton"
          />
        </div>

        <div
          v-else-if="attachmentBreakdown.length === 0"
          class="dashboard__feed-empty"
        >
          В системе нет настроенных типов вложений
        </div>

        <div
          v-else
          class="an-panel"
        >
          <div class="an-panel__chart">
            <AnalyticsDonutChart
              :data="attachmentDonutData"
              :height="300"
              total-label="Вложений"
              :unit-forms="['вложение', 'вложения', 'вложений']"
              empty-ring
            />
          </div>
          <div class="an-panel__tiles">
            <div
              v-for="item in attachmentBreakdown"
              :key="item.label"
              class="dashboard__tile"
            >
              <div class="dashboard__tile-label">{{ item.label }}</div>
              <div class="dashboard__tile-val">
                <AnimatedNumber :value="item.count" />
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- ===== PUSH-УВЕДОМЛЕНИЯ (#974) =====
           Снимок на сейчас, не завязан на выбранный период (from/to) - в отличие
           от групп выше, поэтому не в watch([props.from, props.to]) и не в
           summarySeq/summaryLoading, а сам себе загружает и обновляет данные. -->
      <PushAdoptionSummary ref="pushAdoptionRef" />
    </div>

    <!-- ===== ГРУППА: СИСТЕМА ===== -->
    <div class="dashboard__group">
      <div class="dashboard__group-head">
        <h2 class="dashboard__group-title">Система</h2>
        <span class="dashboard__group-chip">технические показатели</span>
        <span class="dashboard__group-rule" />
      </div>

      <div
        v-if="summaryLoading && !summaryReady"
        class="dashboard__tiles"
      >
        <div
          v-for="n in 4"
          :key="n"
          class="dashboard__tile dashboard__tile--skeleton"
        />
      </div>

      <div
        v-else
        class="dashboard__tiles"
      >
        <div
          class="dashboard__tile dashboard__tile--clickable"
          role="button"
          tabindex="0"
          aria-haspopup="dialog"
          @click="openOnlineUsers"
          @keydown.enter="openOnlineUsers"
          @keydown.space.prevent="openOnlineUsers"
        >
          <div class="dashboard__tile-label">Пользователей онлайн</div>
          <div class="dashboard__tile-val">
            <AnimatedNumber :value="summary.users_online" />
          </div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Пользователи</div>
          <div class="dashboard__tile-val">
            <AnimatedNumber :value="summary.active_users" />
          </div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Заблокировано</div>
          <div class="dashboard__tile-val">
            <AnimatedNumber :value="summary.banned_users" />
          </div>
        </div>
        <div class="dashboard__tile">
          <div class="dashboard__tile-label">Открытых обращений</div>
          <div class="dashboard__tile-val">
            <AnimatedNumber :value="summary.open_feedback" />
          </div>
        </div>
      </div>
    </div>

    <!-- ===== ГРАФИК ===== -->
    <div class="dashboard__chart-card">
      <div class="dashboard__chart-head">
        <div>
          <h3 class="dashboard__chart-title">{{ chartTitle }}</h3>
          <div class="dashboard__chart-sub">{{ chartSubtitle }}</div>
        </div>
        <div class="dashboard__chart-controls">
          <div class="dashboard__seg">
            <button
              v-for="m in metricOptions"
              :key="m.value"
              class="dashboard__seg-btn"
              :class="{ 'dashboard__seg-btn--active': activeMetric === m.value }"
              @click="activeMetric = m.value"
            >
              {{ m.label }}
            </button>
          </div>
          <div class="dashboard__seg">
            <button
              v-for="g in granularityOptions"
              :key="g.value"
              class="dashboard__seg-btn"
              :class="{ 'dashboard__seg-btn--active': activeGranularity === g.value }"
              @click="activeGranularity = g.value"
            >
              {{ g.label }}
            </button>
          </div>
        </div>
      </div>

      <div
        v-if="timelineLoading && !timelineReady"
        class="dashboard__chart-skeleton"
      />
      <AnalyticsAreaChart
        v-else
        :data="chartData"
        :height="300"
        color="#4F5BDF"
        :series-name="chartTitle"
        :unit-forms="chartUnit"
      />
    </div>

    <!-- ===== ДИНАМИКА ОНЛАЙНА ===== -->
    <div class="dashboard__chart-card">
      <div class="dashboard__chart-head">
        <div>
          <h3 class="dashboard__chart-title">Динамика онлайна</h3>
          <div class="dashboard__chart-sub">пик пользователей онлайн по дням</div>
        </div>
      </div>

      <div
        v-if="onlinePeaksLoading && !onlinePeaksReady"
        class="dashboard__chart-skeleton"
      />
      <AnalyticsAreaChart
        v-else
        :data="onlinePeaksData"
        :height="240"
        color="#4F5BDF"
        series-name="Пик онлайна"
        :unit-forms="['пользователь', 'пользователя', 'пользователей']"
      />
    </div>

    <!-- ===== ТОП ЗА ПЕРИОД ===== -->
    <div class="dashboard__group">
      <div class="dashboard__group-head">
        <h2 class="dashboard__group-title">Топ за период</h2>
        <span class="dashboard__group-chip">лидеры по нагрузке</span>
        <span class="dashboard__group-rule" />
      </div>

      <div
        v-if="insightsLoading && !insightsReady"
        class="dashboard__tops"
      >
        <div
          v-for="n in 2"
          :key="n"
          class="dashboard__tile dashboard__tile--skeleton dashboard__top-skeleton"
        />
      </div>

      <div
        v-else
        class="dashboard__tops"
      >
        <TopList
          title="Места разгрузки"
          subtitle="по въездам машин"
          :items="insights.top_places"
        />
        <TopList
          title="Организации"
          subtitle="по числу заявок"
          :items="insights.top_orgs"
        />
      </div>
    </div>

    <!-- ===== ГРУППА: МОНИТОРИНГ (реальное время, не зависит от периода) ===== -->
    <div class="dashboard__group dashboard__group--monitoring">
      <div class="dashboard__group-head">
        <h2 class="dashboard__group-title">Мониторинг</h2>
        <span class="dashboard__live-dot" />
        <span class="dashboard__group-chip">в реальном времени · сейчас</span>
        <span class="dashboard__group-rule" />
      </div>

      <!-- Снимок территории: сколько внутри прямо сейчас -->
      <div
        v-if="summaryLoading && !summaryReady"
        class="dashboard__tiles"
      >
        <div
          v-for="n in 2"
          :key="n"
          class="dashboard__tile dashboard__tile--skeleton"
        />
      </div>
      <div
        v-else
        class="dashboard__tiles"
      >
        <div
          v-for="tile in occupancyTiles"
          :key="tile.label"
          class="dashboard__tile"
        >
          <div class="dashboard__tile-label">{{ tile.label }}</div>
          <div class="dashboard__tile-val">
            <AnimatedNumber :value="tile.value" />
          </div>
        </div>
      </div>

      <!-- Живые ленты проходов/проездов -->
      <div class="dashboard__feeds">

        <!-- Лента людей -->
        <div class="dashboard__feed">
          <div class="dashboard__feed-head">
            <h3 class="dashboard__feed-title">
              <span class="dashboard__live-dot" />
              Проход людей
            </h3>
            <RefreshButton
              :loading="feedRefreshing"
              @refresh="refreshFeeds"
            />
          </div>

          <!-- Своя вертикальная прокрутка (волна 9): та же область, что на
               десктопе (max-height 360px), помечена data-scroll-own - гейт
               мобильных инвариантов считает её законной, а не воровкой жеста
               у страницы. -->
          <div
            class="dashboard__feed-list"
            data-scroll-own
          >
            <template v-if="feedLoading">
              <div
                v-for="n in 5"
                :key="n"
                class="dashboard__feed-row dashboard__feed-row--skeleton"
              />
            </template>
            <template v-else-if="peopleFeed.length === 0">
              <div class="dashboard__feed-empty">Нет записей</div>
            </template>
            <template v-else>
              <div
                v-for="row in peopleFeed"
                :key="feedRowKey(row)"
                class="dashboard__feed-row"
              >
                <div class="dashboard__feed-main">
                  <div class="dashboard__feed-name">{{ row.subject }}</div>
                  <div class="dashboard__feed-sub">{{ row.organization }}</div>
                  <div
                    class="dashboard__feed-post"
                    :class="{ 'dashboard__feed-post--empty': !hasPlace(row) }"
                  >
                    Место: {{ placeLabel(row) }}
                  </div>
                </div>
                <div class="dashboard__feed-right">
                  <div class="dashboard__feed-date">{{ formatDate(row.created_at) }}</div>
                  <div class="dashboard__feed-time">{{ formatTime(row.created_at) }}</div>
                  <span
                    class="dashboard__dir-badge"
                    :class="row.action_type === 'entry' ? 'dashboard__dir-badge--in' : 'dashboard__dir-badge--out'"
                  >
                    {{ row.action_type === 'entry' ? 'Вход' : 'Выход' }}
                  </span>
                </div>
              </div>
            </template>
          </div>
        </div>

        <!-- Лента машин -->
        <div class="dashboard__feed">
          <div class="dashboard__feed-head">
            <h3 class="dashboard__feed-title">
              <span class="dashboard__live-dot" />
              Проезд машин
            </h3>
            <RefreshButton
              :loading="feedRefreshing"
              @refresh="refreshFeeds"
            />
          </div>

          <div
            class="dashboard__feed-list"
            data-scroll-own
          >
            <template v-if="feedLoading">
              <div
                v-for="n in 5"
                :key="n"
                class="dashboard__feed-row dashboard__feed-row--skeleton"
              />
            </template>
            <template v-else-if="carsFeed.length === 0">
              <div class="dashboard__feed-empty">Нет записей</div>
            </template>
            <template v-else>
              <div
                v-for="row in carsFeed"
                :key="feedRowKey(row)"
                class="dashboard__feed-row"
              >
                <div class="dashboard__plate">{{ row.subject }}</div>
                <div class="dashboard__feed-main">
                  <div class="dashboard__feed-name">
                    {{ row.mark || '' }}
                  </div>
                  <div class="dashboard__feed-sub">{{ row.organization }}</div>
                  <div
                    class="dashboard__feed-post"
                    :class="{ 'dashboard__feed-post--empty': !hasPlace(row) }"
                  >
                    Место: {{ placeLabel(row) }}
                  </div>
                </div>
                <div class="dashboard__feed-right">
                  <div class="dashboard__feed-date">{{ formatDate(row.created_at) }}</div>
                  <div class="dashboard__feed-time">{{ formatTime(row.created_at) }}</div>
                  <span
                    class="dashboard__dir-badge"
                    :class="row.action_type === 'entry' ? 'dashboard__dir-badge--in' : 'dashboard__dir-badge--out'"
                  >
                    {{ row.action_type === 'entry' ? 'Въезд' : 'Выезд' }}
                  </span>
                </div>
              </div>
            </template>
          </div>
        </div>
      </div>
    </div>

    <OnlineUsersModal
      :show="onlineModalOpen"
      :users="onlineUsers"
      :loading="onlineUsersLoading"
      :error="onlineUsersError"
      @close="onlineModalOpen = false"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted, onUnmounted } from 'vue';
import AnalyticsAreaChart from '@/components/statistics/AnalyticsAreaChart.vue';
import AnalyticsBarChart from '@/components/statistics/AnalyticsBarChart.vue';
import AnalyticsDonutChart from '@/components/statistics/AnalyticsDonutChart.vue';
import DirIcon from '@/components/statistics/DirIcon.vue';
import TrendSparkline from '@/components/statistics/TrendSparkline.vue';
import TopList from '@/components/statistics/TopList.vue';
import AnimatedNumber from '@/components/statistics/AnimatedNumber.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import OnlineUsersModal from '@/components/statistics/OnlineUsersModal.vue';
import PushAdoptionSummary from '@/components/statistics/PushAdoptionSummary.vue';
import { getSummary, getTimeline, getRecentPassages, getInsights, getOnlinePeaks, getOnlineUsers } from '@/api/statistics.js';
import { mergeFeed, feedRowKey } from './feedMerge.js';

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

// ---- состояние данных ----
// *Ready-флаги: скелетон показываем только до первой загрузки источника. При смене
// периода контент остаётся на месте (числа count-up, графики морфят серию),
// а не мигает скелетоном - убирает "дёрганье" при перезагрузке (фидбэк #632).
const summary = ref({});
const summaryLoading = ref(false);
const summaryReady = ref(false);

// Инсайты обогащают карточки группы «Данные»: сравнение с прошлым периодом
// (дельта/направление), тренд по дням (спарклайн) и профиль пика по часам.
// Метрики инсайтов покрывают заявки/проезды/проходы — остальные карточки без
// инсайтов. peak_hours есть только у проездов/проходов (у заявок его нет).
// top_places/top_orgs питают секцию «Топ за период» (лидерборды).
const insights = reactive({ comparisons: [], trends: [], peak_hours: [], top_places: [], top_orgs: [] });
const insightsLoading = ref(false);
const insightsReady = ref(false);

// Метрика развёрнутой карточки (ключ инсайта) либо null — раскрывает детальные
// графики под сеткой «Данные». Клик по той же карточке сворачивает.
const expandedMetric = ref(null);

const timeline = ref([]);
// Датированный тренд развёрнутой карточки — отдельно от серии инсайта (та без дат).
const detailTimeline = ref([]);
const timelineLoading = ref(false);
const timelineReady = ref(false);

// Дневные пики онлайна за период (area-график под основным графиком).
const onlinePeaks = ref([]);
const onlinePeaksLoading = ref(false);
const onlinePeaksReady = ref(false);

// Модалка «кто онлайн» по клику на плитку «Пользователей онлайн».
const onlineModalOpen = ref(false);
const onlineUsers = ref([]);
const onlineUsersLoading = ref(false);
const onlineUsersError = ref('');
// Токен последовательности: при быстром повторном открытии пишет только последний
// запрос, медленный предыдущий ответ не затирает актуальный список (урок #632).
let onlineUsersSeq = 0;

async function openOnlineUsers() {
  onlineModalOpen.value = true;
  onlineUsersLoading.value = true;
  onlineUsersError.value = '';
  const seq = ++onlineUsersSeq;
  try {
    const list = await getOnlineUsers();
    if (seq !== onlineUsersSeq) return;
    onlineUsers.value = Array.isArray(list) ? list : [];
  } catch {
    if (seq !== onlineUsersSeq) return;
    onlineUsers.value = [];
    onlineUsersError.value = 'Не удалось загрузить список';
  } finally {
    if (seq === onlineUsersSeq) onlineUsersLoading.value = false;
  }
}

const peopleFeed = ref([]);
const carsFeed = ref([]);
const feedLoading = ref(false);
const feedRefreshing = ref(false);

// Волна 5 держала на телефоне лишь 5 записей и без кнопки «Показать ещё» -
// у ленты не было своей прокрутки, и полные 15 записей растягивали блок на
// 2000+px. Волна 9 отдаёт ленте свою прокрутку (see .dashboard__feed-list в
// шаблоне, data-scroll-own), поэтому лимит теперь один на оба брейкпоинта.
const FEED_LIMIT = 15;

// ---- переключатели графика ----
const metricOptions = [
  { label: 'Заявки', value: 'applications' },
  { label: 'Проходы людей', value: 'people_entries' },
  { label: 'Проезды машин', value: 'car_entries' },
];
const granularityOptions = [
  { label: 'День', value: 'day' },
  { label: 'Неделя', value: 'week' },
  { label: 'Месяц', value: 'month' },
];
const activeMetric = ref('applications');
const activeGranularity = ref('day');

const chartTitles = {
  applications: 'Динамика заявок',
  people_entries: 'Динамика проходов людей',
  car_entries: 'Динамика проездов машин',
};

const chartTitle = computed(() => chartTitles[activeMetric.value] ?? '');

// Формы склонения единицы тултипа по метрике [одна, две-четыре, пять+].
const chartUnitForms = {
  applications: ['заявка', 'заявки', 'заявок'],
  people_entries: ['проход', 'прохода', 'проходов'],
  car_entries: ['проезд', 'проезда', 'проездов'],
};
const chartUnit = computed(() => chartUnitForms[activeMetric.value] ?? ['ед.', 'ед.', 'ед.']);
const chartSubtitle = computed(() => {
  const labels = { day: 'по дням', week: 'по неделям', month: 'по месяцам' };
  const period = [props.from, props.to].filter(Boolean).join(' — ');
  return [period, labels[activeGranularity.value]].filter(Boolean).join(' · ');
});

// AnalyticsAreaChart ожидает [{timestamp, count}], бэк отдаёт [{date, count}]
const chartData = computed(() =>
  (timeline.value || []).map((d) => ({ timestamp: d.date, count: d.count }))
);

// Пики онлайна: бэк отдаёт [{date, peak}] -> форма AnalyticsAreaChart.
const onlinePeaksData = computed(() =>
  (onlinePeaks.value || []).map((d) => ({ timestamp: d.date, count: d.peak }))
);

// Формы склонения единицы для тултипов детального разворота, по ключу инсайта.
const detailUnitForms = {
  applications_count: ['заявка', 'заявки', 'заявок'],
  car_entries_count: ['проезд', 'проезда', 'проездов'],
  people_entries_count: ['проход', 'прохода', 'проходов'],
};

// ---- вычисляемые из summary ----
const attachmentBreakdown = computed(() => {
  const list = summary.value.by_attachment_type;
  if (!Array.isArray(list)) return [];
  return list.map((item) => ({ label: item.name, count: item.count }));
});

// Donut распределения по типам вложений: только ненулевые доли (пустые типы
// засоряют легенду и не несут смысла для распределения).
const attachmentDonutData = computed(() =>
  attachmentBreakdown.value
    .filter((item) => Number(item.count) > 0)
    .map((item) => ({ label: item.label, value: item.count })),
);

// Карточки группы «Данные». metric связывает карточку с инсайтом того же ключа;
// карточки без metric (Обработано, На территории) инсайтом не покрыты.
const dataTiles = computed(() => {
  const s = summary.value;
  const defs = [
    { label: 'Получено заявок', value: s.total_applications, metric: 'applications_count' },
    { label: 'Обработано', value: s.processed },
    { label: 'Машин заехало', value: s.cars_entered, metric: 'car_entries_count' },
    { label: 'Людей прошло', value: s.people_entered, metric: 'people_entries_count' },
  ];
  return defs.map((t) => {
    const comparison = t.metric ? insights.comparisons.find((c) => c.metric === t.metric) || null : null;
    const trend = t.metric ? insights.trends.find((tr) => tr.metric === t.metric) || null : null;
    const peak = t.metric ? insights.peak_hours.find((p) => p.metric === t.metric) || null : null;
    return { ...t, comparison, trend, peak, expandable: Boolean(comparison || trend || peak) };
  });
});

// Снимок «сейчас»: сколько машин/людей на территории прямо сейчас (territory_status=1).
// Живёт в Мониторинге, а не в «Данные» — значение не зависит от выбранного периода,
// поэтому смотреть его «за год» бессмысленно (фидбэк #632 п.6).
const occupancyTiles = computed(() => {
  const s = summary.value;
  return [
    { label: 'Машин на территории', value: s.cars_on_territory },
    { label: 'Людей на территории', value: s.people_on_territory },
  ];
});

// Развёрнутая карточка -> данные её детальных графиков. Тренд по дням берём из
// getTimeline (датированные точки), а не из серии инсайта без дат — иначе ось X
// получала порядковые номера вместо реальных дат. Пик — из почасового профиля.
const expandedDetail = computed(() => {
  if (!expandedMetric.value) return null;
  const tile = dataTiles.value.find((t) => t.metric === expandedMetric.value);
  if (!tile || !tile.expandable) return null;
  const peak = tile.peak;
  return {
    label: tile.label,
    unitForms: detailUnitForms[tile.metric] || ['ед.', 'ед.', 'ед.'],
    // timestamp -> AnalyticsAreaChart строит ось X как дд.мм, тултип дд.мм.гггг.
    trendData: (detailTimeline.value || []).map((d) => ({ timestamp: d.date, count: d.count })),
    peak,
    peakData: peak ? hourlyProfile(peak) : [],
    peakHourLabel: peak ? hourLabel(peak.peak_hour) : '',
  };
});

// ---- форматирование ----
function deltaText(pct) {
  // Бэкенд округляет до 1 знака, но JSON->Number может дать хвост (16.700000003) —
  // повторно округляем, чтобы не показать артефакт.
  const n = Math.round((Number(pct) || 0) * 10) / 10;
  const sign = n > 0 ? '+' : '';
  return `${sign}${n}%`;
}

function hourLabel(hour) {
  return `${String(hour).padStart(2, '0')}:00`;
}

// Почасовое распределение -> полный профиль суток 0..23 (пустые часы = 0), чтобы
// столбцы читались как профиль дня, а не разреженный набор.
function hourlyProfile(peak) {
  const byHour = new Map((peak.hourly || []).map((b) => [b.hour, b.value]));
  return Array.from({ length: 24 }, (_, h) => ({
    label: hourLabel(h),
    value: byHour.get(h) || 0,
  }));
}

function toggleExpand(metric) {
  expandedMetric.value = expandedMetric.value === metric ? null : metric;
}

function formatTime(iso) {
  if (!iso) return '—';
  const d = new Date(iso);
  // Смещение UTC+3
  const utc3 = new Date(d.getTime() + 3 * 60 * 60 * 1000);
  return utc3.toISOString().substring(11, 16);
}

function formatDate(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  // Смещение UTC+3: дату ленты считаем по МСК, иначе у ночных отметок съезжает день.
  const s = new Date(d.getTime() + 3 * 60 * 60 * 1000).toISOString();
  return `${s.substring(8, 10)}.${s.substring(5, 7)}.${s.substring(0, 4)}`;
}

// Место (place) прохода: бэк может вернуть пусто или плейсхолдер «—». Показываем
// явный лейбл у каждой строки, при отсутствии — заглушку, а не прячем (фидбэк #632 п.7).
function hasPlace(row) {
  const p = (row.place || '').trim();
  return Boolean(p) && p !== '—';
}

function placeLabel(row) {
  return hasPlace(row) ? row.place.trim() : 'не указан';
}

// ---- загрузка данных ----
// Токены последовательности: быстрое переключение периода/метрики/гранулярности
// пускает несколько запросов параллельно; результат пишет только последний, иначе
// медленный ответ предыдущего выбора затирает актуальный (last-resolve-wins).
let summarySeq = 0;
let timelineSeq = 0;
let insightsSeq = 0;
let onlinePeaksSeq = 0;
let detailTimelineSeq = 0;

async function loadSummary() {
  const seq = ++summarySeq;
  summaryLoading.value = true;
  try {
    const data = await getSummary(props.from, props.to);
    if (seq !== summarySeq) return;
    summary.value = data || {};
  } catch {
    if (seq !== summarySeq) return;
    summary.value = {};
  } finally {
    if (seq === summarySeq) {
      summaryLoading.value = false;
      summaryReady.value = true;
    }
  }
}

async function loadInsights() {
  const seq = ++insightsSeq;
  insightsLoading.value = true;
  try {
    const data = await getInsights(props.from, props.to);
    if (seq !== insightsSeq) return;
    insights.comparisons = Array.isArray(data?.comparisons) ? data.comparisons : [];
    insights.trends = Array.isArray(data?.trends) ? data.trends : [];
    insights.peak_hours = Array.isArray(data?.peak_hours) ? data.peak_hours : [];
    insights.top_places = Array.isArray(data?.top_places) ? data.top_places : [];
    insights.top_orgs = Array.isArray(data?.top_orgs) ? data.top_orgs : [];
  } catch {
    // Сбой инсайтов не должен ронять дашборд — карточки остаются без футера,
    // топы показывают пустое состояние.
    if (seq !== insightsSeq) return;
    insights.comparisons = [];
    insights.trends = [];
    insights.peak_hours = [];
    insights.top_places = [];
    insights.top_orgs = [];
  } finally {
    if (seq === insightsSeq) {
      insightsLoading.value = false;
      insightsReady.value = true;
    }
  }
}

async function loadOnlinePeaks() {
  const seq = ++onlinePeaksSeq;
  onlinePeaksLoading.value = true;
  try {
    const data = await getOnlinePeaks(props.from, props.to);
    if (seq !== onlinePeaksSeq) return;
    onlinePeaks.value = Array.isArray(data) ? data : [];
  } catch {
    // Сбой пиков онлайна не должен ронять дашборд — блок показывает заглушку.
    if (seq !== onlinePeaksSeq) return;
    onlinePeaks.value = [];
  } finally {
    if (seq === onlinePeaksSeq) {
      onlinePeaksLoading.value = false;
      onlinePeaksReady.value = true;
    }
  }
}

async function loadTimeline() {
  const seq = ++timelineSeq;
  timelineLoading.value = true;
  try {
    const data = await getTimeline({
      from: props.from,
      to: props.to,
      metric: activeMetric.value,
      granularity: activeGranularity.value,
    });
    if (seq !== timelineSeq) return;
    timeline.value = Array.isArray(data) ? data : [];
  } catch {
    if (seq !== timelineSeq) return;
    timeline.value = [];
  } finally {
    if (seq === timelineSeq) {
      timelineLoading.value = false;
      timelineReady.value = true;
    }
  }
}

// Ключ инсайта плитки -> metric для getTimeline (тот же набор, что в metricOptions).
const detailMetricMap = {
  applications_count: 'applications',
  car_entries_count: 'car_entries',
  people_entries_count: 'people_entries',
};

// Тренд развёрнутой карточки грузим по дням отдельным запросом: даёт даты по оси.
async function loadDetailTimeline(metricKey) {
  const tlMetric = detailMetricMap[metricKey];
  if (!tlMetric) {
    detailTimeline.value = [];
    return;
  }
  const seq = ++detailTimelineSeq;
  try {
    const data = await getTimeline({
      from: props.from,
      to: props.to,
      metric: tlMetric,
      granularity: 'day',
    });
    if (seq !== detailTimelineSeq) return;
    detailTimeline.value = Array.isArray(data) ? data : [];
  } catch {
    if (seq !== detailTimelineSeq) return;
    detailTimeline.value = [];
  }
}

// showSkeleton: только первичная загрузка показывает скелетоны. Фоновые тики и
// ручное обновление доливают новые записи сверху через mergeFeed — без мигания
// всего блока и без перерисовки уже показанных строк.
async function loadFeed({ showSkeleton = false } = {}) {
  if (showSkeleton) feedLoading.value = true;
  try {
    // Один запрос отдаёт обе ленты.
    const data = await getRecentPassages(FEED_LIMIT);
    const people = Array.isArray(data?.people) ? data.people : [];
    const cars = Array.isArray(data?.cars) ? data.cars : [];
    peopleFeed.value = mergeFeed(peopleFeed.value, people);
    carsFeed.value = mergeFeed(carsFeed.value, cars);
  } catch {
    // Фоновый сбой не должен очищать уже показанные ленты — чистим только при
    // первичной загрузке, где показать пустоту корректнее, чем скелетон навсегда.
    if (showSkeleton) {
      peopleFeed.value = [];
      carsFeed.value = [];
    }
  } finally {
    if (showSkeleton) feedLoading.value = false;
  }
}

// Ручное обновление лент — крутит RefreshButton, не показывает скелетон.
async function refreshFeeds() {
  feedRefreshing.value = true;
  try {
    await loadFeed();
  } finally {
    feedRefreshing.value = false;
  }
}

// Push-сводка (#974) - отдельный самодостаточный компонент со своей загрузкой
// (см. комментарий у <PushAdoptionSummary> в template), сюда попадает только
// ссылка для ручного обновления кнопкой «Обновить» в шапке аналитики.
const pushAdoptionRef = ref(null);

// ---- публичный метод для обновления из родителя ----
async function refresh() {
  await Promise.all([
    loadSummary(),
    loadTimeline(),
    loadInsights(),
    loadOnlinePeaks(),
    loadFeed(),
    pushAdoptionRef.value?.refresh(),
  ]);
}

defineExpose({ refresh });

// ---- реакция на смену периода ----
watch([() => props.from, () => props.to], () => {
  // Сворачиваем разворот: иначе под новым периодом мелькает деталь старого,
  // пока летит ответ инсайтов.
  expandedMetric.value = null;
  loadSummary();
  loadTimeline();
  loadInsights();
  loadOnlinePeaks();
});

// ---- реакция на смену настроек графика ----
watch([activeMetric, activeGranularity], () => {
  loadTimeline();
});

// ---- разворот карточки -> датированный тренд по дням ----
watch(expandedMetric, (metric) => {
  if (metric) loadDetailTimeline(metric);
  else detailTimeline.value = [];
});

// ---- polling живых лент (10 сек) ----
let feedInterval = null;

onMounted(() => {
  loadSummary();
  loadTimeline();
  loadInsights();
  loadOnlinePeaks();
  loadFeed({ showSkeleton: true });
  feedInterval = setInterval(() => loadFeed(), 10000);
});

onUnmounted(() => {
  if (feedInterval) clearInterval(feedInterval);
});
</script>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 30px;
}

/* ===== ГРУППЫ ===== */
.dashboard__group {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.dashboard__group-head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.dashboard__group-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text);
  margin: 0;
  white-space: nowrap;
}

.dashboard__group-chip {
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-muted);
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  padding: 3px 10px;
  border-radius: var(--radius-pill);
  white-space: nowrap;
}

.dashboard__group-rule {
  flex: 1;
  height: 1px;
  background: var(--color-border);
}

/* ===== ТОП ЗА ПЕРИОД ===== */
.dashboard__tops {
  display: grid;
  /* minmax(0,1fr), а не 1fr: трек не должен сайзиться по содержимому (длинные
     имена в TopList), иначе распирает грид за контейнер, не давая ellipsis. */
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 20px;
}

@media (max-width: 900px) {
  .dashboard__tops {
    grid-template-columns: minmax(0, 1fr);
  }
}

.dashboard__top-skeleton {
  min-height: 200px;
}

/* ===== ПЛИТКИ ===== */
.dashboard__tiles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(158px, 1fr));
  gap: 12px;
}

.dashboard__tile {
  background: var(--surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 12px 14px;
  cursor: default;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
  animation: tile-in 0.35s ease both;
}

.dashboard__tile:hover {
  border-color: var(--accent);
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

@keyframes tile-in {
  from { opacity: 0; transform: translateY(10px); }
  to   { opacity: 1; transform: translateY(0); }
}

.dashboard__tile--skeleton {
  background: var(--color-skeleton);
  min-height: 72px;
  border: none;
  cursor: default;
  animation: skeleton-pulse 1.4s ease-in-out infinite;
}

@keyframes skeleton-pulse {
  0%, 100% { opacity: 1; }
  50%       { opacity: 0.55; }
}

.dashboard__tile-label {
  font-size: 11px;
  color: var(--color-text-muted);
  font-weight: 500;
  line-height: 1.3;
}

.dashboard__tile-val {
  font-size: 26px;
  font-weight: 700;
  color: var(--color-text);
  margin-top: 6px;
  line-height: 1;
}

/* ===== ИНСАЙТ-ФУТЕР ПЛИТКИ ===== */
.dashboard__tile-insight {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 10px;
}

.dashboard__tile-spark {
  width: 100%;
  max-width: 72px;
  height: 24px;
  flex-shrink: 1;
}

.dashboard__delta {
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

.dashboard__delta--up {
  background: color-mix(in srgb, var(--success) 12%, var(--surface));
  color: var(--success-text);
}

.dashboard__delta--down {
  background: color-mix(in srgb, var(--danger) 12%, var(--surface));
  color: var(--danger-text);
}

.dashboard__delta--flat {
  background: var(--color-bg);
  color: var(--color-text-muted);
  border: 1px solid var(--color-border);
}

/* ===== РАЗВОРОТ КАРТОЧКИ ===== */
.dashboard__tile--clickable {
  cursor: pointer;
}

.dashboard__tile--active {
  border-color: var(--accent);
  box-shadow: var(--shadow-md);
}

.dashboard__tile-label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}

/* Каретка-подсказка «раскрывается»; поворачивается вверх у активной карточки. */
.dashboard__tile-caret {
  width: 7px;
  height: 7px;
  border-right: 1.5px solid var(--color-text-muted);
  border-bottom: 1.5px solid var(--color-text-muted);
  transform: rotate(45deg);
  transition: transform 0.2s ease, border-color 0.2s ease;
  flex-shrink: 0;
  margin-top: -3px;
}

.dashboard__tile--active .dashboard__tile-caret {
  transform: rotate(-135deg);
  margin-top: 2px;
  border-color: var(--accent);
}

.dashboard__detail {
  border: 1px solid var(--color-primary);
  border-radius: var(--radius-lg);
  padding: 18px 20px;
  background: var(--surface);
  margin-top: 4px;
}

.dashboard__detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.dashboard__detail-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text);
  margin: 0;
}

.dashboard__detail-close {
  border: none;
  background: transparent;
  font-size: 22px;
  line-height: 1;
  color: var(--color-text-muted);
  cursor: pointer;
  padding: 0 4px;
  border-radius: var(--radius-sm);
  transition: color 0.18s ease;
}

.dashboard__detail-close:hover {
  color: var(--color-text);
}

.dashboard__detail-charts {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 18px;
}

.dashboard__detail-chart-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  margin-bottom: 8px;
}

.dashboard__detail-chart-note {
  font-size: 11px;
  font-weight: 600;
  color: var(--accent-text);
  margin-left: 6px;
}

/* Плавный разворот высотой через grid-rows: соседние группы съезжают без рывка
   (телепортации), график внутри с фиксированной высотой 220 не дёргается. */
.dashboard__detail-collapse {
  display: grid;
  grid-template-rows: 1fr;
}

.dashboard__detail-collapse-inner {
  min-height: 0;
  overflow: hidden;
}

.dashboard-detail-enter-active,
.dashboard-detail-leave-active {
  transition: grid-template-rows 0.28s ease, opacity 0.28s ease;
}

.dashboard-detail-enter-from,
.dashboard-detail-leave-to {
  grid-template-rows: 0fr;
  opacity: 0;
}

/* ===== ГРАФИК ===== */
.dashboard__chart-card {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 20px 22px;
  background: var(--surface);
}

.dashboard__chart-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
  margin-bottom: 18px;
}

.dashboard__chart-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-text);
  margin: 0;
}

.dashboard__chart-sub {
  font-size: 12px;
  color: var(--color-text-muted);
  margin-top: 2px;
}

.dashboard__chart-controls {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.dashboard__seg {
  display: inline-flex;
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  padding: 3px;
  gap: 2px;
}

.dashboard__seg-btn {
  border: none;
  background: transparent;
  font-family: inherit;
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-muted);
  padding: 5px 12px;
  border-radius: var(--radius-pill);
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}

.dashboard__seg-btn:hover {
  color: var(--accent-text);
}

.dashboard__seg-btn--active {
  background: var(--color-primary);
  color: var(--accent-contrast);
}

/* На активной сегмент-кнопке hover не должен перекрашивать текст в синий. */
.dashboard__seg-btn--active:hover {
  color: var(--accent-contrast);
}

.dashboard__chart-skeleton {
  height: 300px;
  background: var(--color-skeleton);
  border-radius: var(--radius-sm);
  animation: skeleton-pulse 1.4s ease-in-out infinite;
}

/* ===== ЖИВЫЕ ЛЕНТЫ ===== */
.dashboard__feeds {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

@media (max-width: 1100px) {
  .dashboard__feeds {
    grid-template-columns: 1fr;
  }
}

.dashboard__feed {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  overflow: hidden;
  background: var(--surface);
  display: flex;
  flex-direction: column;
}

.dashboard__feed-head {
  padding: 14px 18px;
  border-bottom: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-shrink: 0;
}

.dashboard__feed-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text);
  margin: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.dashboard__live-dot {
  display: inline-block;
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--color-success);
  flex-shrink: 0;
  animation: live-pulse 1.8s ease-in-out infinite;
}

@keyframes live-pulse {
  0%   { box-shadow: 0 0 0 0 color-mix(in srgb, var(--success) 55%, transparent); }
  70%  { box-shadow: 0 0 0 8px color-mix(in srgb, var(--success) 0%, transparent); }
  100% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--success) 0%, transparent); }
}

.dashboard__feed-list {
  max-height: 360px;
  overflow-y: auto;
}

.dashboard__feed-list::-webkit-scrollbar {
  width: 6px;
}

.dashboard__feed-list::-webkit-scrollbar-thumb {
  background: var(--accent-tint);
  border-radius: var(--radius-pill);
}

.dashboard__feed-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 11px 18px;
  border-bottom: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  animation: row-in 0.3s ease;
}

@keyframes row-in {
  from { opacity: 0; transform: translateY(-6px); }
  to   { opacity: 1; transform: translateY(0); }
}

.dashboard__feed-row--skeleton {
  min-height: 56px;
  background: var(--color-skeleton);
  border-bottom: 1px solid var(--surface);
  animation: skeleton-pulse 1.4s ease-in-out infinite;
}

.dashboard__feed-main {
  min-width: 0;
  flex: 1;
}

.dashboard__feed-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dashboard__feed-sub {
  font-size: 11px;
  color: var(--color-text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-top: 2px;
}

/* Место прохода — отдельной строкой, чтобы длинная организация не вытесняла его
   через ellipsis: место видно у каждой строки (фидбэк #632 п.7). */
.dashboard__feed-post {
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-top: 2px;
}

.dashboard__feed-post--empty {
  color: var(--color-text-muted);
  font-weight: 500;
  font-style: italic;
}

.dashboard__feed-right {
  text-align: right;
  flex-shrink: 0;
}

.dashboard__feed-date {
  font-size: 11px;
  color: var(--color-text-muted);
  font-variant-numeric: tabular-nums;
}

.dashboard__feed-time {
  font-size: 11px;
  color: var(--color-text-muted);
  font-variant-numeric: tabular-nums;
}

.dashboard__dir-badge {
  display: inline-flex;
  align-items: center;
  font-size: 10px;
  font-weight: 700;
  padding: 3px 8px;
  border-radius: var(--radius-pill);
  margin-top: 4px;
}

.dashboard__dir-badge--in {
  background: color-mix(in srgb, var(--success) 12%, var(--surface));
  /* Текстовый тон семьи, а не сплошной: сплошной на своей же бледной подложке
     давал 2.75 - так было и до перевода на переменные. */
  color: var(--success-text);
}

.dashboard__dir-badge--out {
  background: color-mix(in srgb, var(--danger) 12%, var(--surface));
  color: var(--danger-text);
}

/* Номерной знак машины */
.dashboard__plate {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 64px;
  padding: 3px 8px;
  border: 2px solid var(--color-text);
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  color: var(--color-text);
  letter-spacing: 0.04em;
  background: var(--surface);
  text-transform: uppercase;
}

.dashboard__feed-empty {
  padding: 24px 18px;
  font-size: 13px;
  color: var(--color-text-muted);
  text-align: center;
}

/* ===== МОБИЛКА (<=768) ===== */
@media (max-width: 768px) {
  .dashboard {
    gap: 22px;
  }

  /* Шапка группы: длинному чипу разрешаем перенос, линейка добирает остаток
     строки (на 320 «Мониторинг» + чип «в реальном времени · сейчас» в ряд не
     влезают и без wrap распирают контейнер). */
  .dashboard__group-head {
    flex-wrap: wrap;
    gap: 8px;
  }

  .dashboard__group-rule {
    min-width: 24px;
  }

  /* Плитки чуть плотнее, чтобы инсайт-футер (спарклайн + дельта) помещался в ряд
     без переполнения при двух колонках. */
  .dashboard__tile {
    padding: 12px;
  }

  .dashboard__tile-val {
    font-size: 24px;
  }

  /* График/динамика онлайна: компактнее рамка, контролы во всю ширину. */
  .dashboard__chart-card {
    padding: 14px;
  }

  .dashboard__chart-head {
    gap: 12px;
    margin-bottom: 14px;
  }

  .dashboard__chart-controls {
    width: 100%;
  }

  /* Две сегмент-группы по 3 кнопки не влезают в общий ряд на 390 -> каждая на
     всю ширину отдельной строкой. Кнопки тянутся flex-grow'ом, заполняя ряд; на
     узких телефонах садятся по содержимому (min-width:auto по умолчанию), чтобы
     длинные подписи "Проходы людей"/"Проезды машин" не обрезались - равные доли
     форсить нельзя без overflow-guard, иначе клип текста. */
  .dashboard__seg {
    width: 100%;
  }

  /* Высота под палец: 26px давали промах по соседнему сегменту. 36 - принятая в
     проекте норма компактного контрола (эталон §18). */
  .dashboard__seg-btn {
    flex: 1 1 0;
    text-align: center;
    padding: 6px 4px;
    min-height: 36px;
    font-size: 11px;
    white-space: nowrap;
  }

  /* Детальный разворот карточки: сетка графиков в один столбец (minmax(320px)
     распирал уже узкую панель на 320), паддинги компактнее. */
  .dashboard__detail {
    padding: 14px;
  }

  .dashboard__detail-charts {
    grid-template-columns: 1fr;
    gap: 14px;
  }

  /* Ленты мониторинга: компактнее отступы строк. Текст полей и так усечён
     ellipsis, feed-main держит min-width:0 -> строка не переполняется. */
  .dashboard__feed-head {
    padding: 12px 14px;
  }

  .dashboard__feed-row {
    padding: 10px 14px;
    gap: 10px;
  }

}

@media (max-width: 480px) {
  /* Номерной знак у́же, чтобы ленте машин хватало ширины под марку/место. */
  .dashboard__plate {
    min-width: 56px;
    padding: 3px 6px;
  }
}
</style>
