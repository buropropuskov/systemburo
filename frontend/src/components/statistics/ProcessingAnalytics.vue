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
                <HintTooltip
                  v-if="stageMeta(s.key).hint"
                  :text="stageMeta(s.key).hint"
                />
              </div>
              <div class="proc__tile-val">
                {{ fmtDur(s.avg) }}
                <span class="proc__tile-agg">среднее</span>
              </div>
              <div class="proc__tile-meta">
                <span class="proc__tile-sub">9 из 10 — до {{ fmtDur(s.p90) }}</span>
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

      <!-- ===== ДИНАМИКА ЭТАПОВ ===== -->
      <section class="proc__group">
        <div class="proc__group-head">
          <h2 class="proc__group-title">Динамика</h2>
          <span class="proc__group-chip">среднее время этапа {{ trendGranularity.label }}</span>
          <span class="proc__group-rule" />
        </div>
        <div class="proc__trend-head">
          <FilterTabs
            v-model="trendStage"
            :tabs="TREND_TABS"
          />
          <HintTooltip :text="TREND_HINT" />
        </div>
        <div
          v-if="trendError"
          class="proc__state proc__state--error"
        >
          {{ trendError }}
        </div>
        <div
          v-else-if="!trendReady"
          class="proc__skeleton proc__skeleton--chart"
        />
        <div
          v-else
          class="proc__card"
        >
          <AnalyticsAreaChart
            :data="trendData"
            value-type="duration"
            :series-name="trendSeriesName"
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
                <HintTooltip
                  v-if="qualityHint(q.key)"
                  :text="qualityHint(q.key)"
                />
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

      <!-- ===== РЕЙТИНГИ: СОГЛАСУЮЩИЕ И ПРИНИМАЮЩИЕ ===== -->
      <div class="proc__cols">
        <!-- Согласующие: полный рейтинг по скорости реакции (быстрые сверху) -->
        <section class="proc__group">
          <div class="proc__group-head">
            <h2 class="proc__group-title">Согласующие</h2>
            <span class="proc__group-chip">рейтинг по скорости реакции</span>
            <span class="proc__group-rule" />
          </div>
          <div
            v-if="showSkeleton"
            class="proc__skeleton proc__skeleton--table"
          />
          <div
            v-else
            class="proc__card proc__card--table proc__card--scroll"
          >
            <table class="proc__table proc__table--rating rt-table">
              <thead class="rt-head-row">
                <tr>
                  <th class="proc__rank-h">#</th>
                  <th>Согласующий</th>
                  <th class="proc__num">Время реакции</th>
                  <th class="proc__num">Нагрузка</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(a, i) in approvers"
                  :key="i"
                  class="rt-row"
                >
                  <td
                    class="proc__rank"
                    data-label="Место"
                  >{{ i + 1 }}</td>
                  <td
                    class="proc__ellipsis"
                    data-label="Согласующий"
                    :title="a.name"
                  >{{ a.name }}</td>
                  <td
                    class="proc__num"
                    data-label="Время реакции"
                  >{{ fmtDur(a.avg_response_time) }}</td>
                  <td
                    class="proc__num"
                    data-label="Нагрузка"
                  >{{ fmtCount(a.votes_count) }}</td>
                </tr>
                <tr v-if="approvers.length === 0">
                  <td
                    colspan="4"
                    class="proc__table-empty"
                  >Нет данных</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <!-- Принимающие: полный рейтинг по скорости принятия в работу -->
        <section class="proc__group">
          <div class="proc__group-head">
            <h2 class="proc__group-title">Принимающие</h2>
            <span class="proc__group-chip">рейтинг по скорости принятия</span>
            <span class="proc__group-rule" />
          </div>
          <div
            v-if="showSkeleton"
            class="proc__skeleton proc__skeleton--table"
          />
          <div
            v-else
            class="proc__card proc__card--table proc__card--scroll"
          >
            <table class="proc__table proc__table--rating rt-table">
              <thead class="rt-head-row">
                <tr>
                  <th class="proc__rank-h">#</th>
                  <th>Принимающий</th>
                  <th class="proc__num">Время принятия</th>
                  <th class="proc__num">Принято</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(a, i) in acceptors"
                  :key="i"
                  class="rt-row"
                >
                  <td
                    class="proc__rank"
                    data-label="Место"
                  >{{ i + 1 }}</td>
                  <td
                    class="proc__ellipsis"
                    data-label="Принимающий"
                    :title="a.name"
                  >{{ a.name }}</td>
                  <td
                    class="proc__num"
                    data-label="Время принятия"
                  >{{ fmtDur(a.avg_acceptance_time) }}</td>
                  <td
                    class="proc__num"
                    data-label="Принято"
                  >{{ fmtCount(a.accepts_count) }}</td>
                </tr>
                <tr v-if="acceptors.length === 0">
                  <td
                    colspan="4"
                    class="proc__table-empty"
                  >Нет данных</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <!-- ===== РАЗБИВКА: ОРГАНИЗАЦИИ И КОМПАНИИ ===== -->
      <section class="proc__group">
        <div class="proc__group-head">
          <h2 class="proc__group-title">Разбивка</h2>
          <span class="proc__group-chip">среднее время этапов, дольше всего сверху</span>
          <span class="proc__group-rule" />
        </div>
        <div class="proc__trend-head">
          <FilterTabs
            v-model="breakdownDim"
            :tabs="BREAKDOWN_TABS"
          />
          <HintTooltip :text="BREAKDOWN_HINT" />
        </div>
        <div
          v-if="showSkeleton"
          class="proc__skeleton proc__skeleton--table"
        />
        <div
          v-else
          class="proc__card proc__card--table proc__card--scroll"
        >
          <table class="proc__table proc__table--breakdown rt-table">
            <thead class="rt-head-row">
              <tr>
                <th>{{ breakdownNameHeader }}</th>
                <th class="proc__num">Согласование</th>
                <th class="proc__num">Принятие</th>
                <th class="proc__num">Обработка</th>
                <th class="proc__num">Заявок</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(o, i) in breakdownRows"
                :key="i"
                class="rt-row"
              >
                <td
                  class="proc__ellipsis"
                  :data-label="breakdownNameHeader"
                  :title="o.label"
                >{{ o.label }}</td>
                <td
                  class="proc__num"
                  data-label="Согласование"
                >{{ fmtDur(o.avg_approval_time) }}</td>
                <td
                  class="proc__num"
                  data-label="Принятие"
                >{{ fmtDur(o.avg_acceptance_time) }}</td>
                <td
                  class="proc__num"
                  data-label="Обработка"
                >{{ fmtDur(o.avg_processing_time) }}</td>
                <td
                  class="proc__num"
                  data-label="Заявок"
                >{{ fmtCount(o.applications_count) }}</td>
              </tr>
              <tr v-if="breakdownRows.length === 0">
                <td
                  colspan="5"
                  class="proc__table-empty"
                >Нет данных</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>

    <!-- ===== ЗАВИСШИЕ СОГЛАСОВАНИЯ =====
         ВНЕ isEmpty-гейта, как журнал: снимок ТЕКУЩИХ зависших заявок не зависит
         от дат вкладки (эндпоинт их не принимает) — заявка может ждать согласования
         дольше выбранного периода. Своя загрузка/ошибка/пусто. Рядом с рейтингом
         согласующих смыслово: та же ось «кто тормозит согласование». -->
    <section
      v-if="!errorMsg"
      class="proc__group"
    >
      <div class="proc__group-head">
        <h2 class="proc__group-title">Зависшие согласования</h2>
        <span class="proc__group-chip">ждут решения дольше порога напоминаний</span>
        <span class="proc__group-rule" />
        <RefreshButton
          :loading="stuckLoading"
          title="Обновить список зависших согласований"
          @refresh="loadStuck"
        />
      </div>
      <div
        v-if="stuckError"
        class="proc__state proc__state--error"
      >
        {{ stuckError }}
      </div>
      <div
        v-else-if="!stuckReady"
        class="proc__skeleton proc__skeleton--table"
      />
      <div
        v-else
        class="proc__card proc__card--table proc__card--scroll"
      >
        <table class="proc__table proc__table--stuck rt-table">
          <thead class="rt-head-row">
            <tr>
              <th class="proc__col-app">Заявка</th>
              <th>Согласующий</th>
              <th class="proc__num">Ждёт</th>
              <th class="proc__num">Напоминаний</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(a, i) in stuck"
              :key="`${a.application_id}-${i}`"
              class="rt-row"
            >
              <td data-label="Заявка">
                <button
                  type="button"
                  class="proc__stuck-app proc__journal-app--copy"
                  title="Скопировать номер заявки"
                  @click="copyApplicationNumber(a.application_number)"
                >{{ a.application_number }}</button>
              </td>
              <td
                class="proc__ellipsis"
                data-label="Согласующий"
                :title="a.approver_name"
              >{{ a.approver_name }}</td>
              <td
                class="proc__num"
                data-label="Ждёт"
              >{{ waitingText(a.waiting_days) }}</td>
              <td
                class="proc__num"
                data-label="Напоминаний"
              >{{ fmtCount(a.reminder_count) }}</td>
            </tr>
            <tr v-if="stuck.length === 0">
              <td
                colspan="4"
                class="proc__table-empty"
              >Зависших согласований нет</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <!-- ===== ЖУРНАЛ: ЛЕНТА РЕШЕНИЙ ПО ЗАЯВКАМ =====
         ВНЕ isEmpty-гейта намеренно: окно журнала бьёт по времени СОБЫТИЯ,
         а не по дате подачи, поэтому события могут быть и
         когда за период не подали ни одной новой заявки. Своя загрузка/ошибка/пусто. -->
    <section
      v-if="!errorMsg"
      class="proc__group"
    >
      <div class="proc__group-head">
        <h2 class="proc__group-title">Журнал</h2>
        <span class="proc__group-chip">
          <span class="proc__group-chip-full">решения по заявкам: согласования, принятия, отказы, отзывы</span>
          <span class="proc__group-chip-short">решения по заявкам</span>
        </span>
        <span class="proc__group-rule" />
        <RefreshButton
          :loading="journalLoading"
          title="Обновить только журнал (кнопка вверху страницы обновляет всю вкладку)"
          @refresh="loadJournal"
        />
      </div>
      <!-- Фильтры ленты (#1251 P5c). Отбор идёт на бэке: страница и «Всего»
           считаются по одному предикату, иначе фильтрация в пределах текущей
           страницы врала бы о числе событий.
           На телефоне шесть табов роли не помещаются - заменяются BaseDropdown
           (десктоп не трогаем, FilterTabs остаются), а дата и поиск встают в
           одну строку через .proc__journal-daterow (поиск раскрывается поверх
           неё иконкой - зеркало Центра/кабинета). -->
      <div class="proc__journal-filters">
        <FilterTabs
          v-if="!isNarrow"
          :model-value="journalRole"
          :tabs="JOURNAL_ROLE_TABS"
          @update:model-value="onJournalRoleChange"
        />
        <BaseDropdown
          v-else
          class="proc__journal-role-dropdown"
          :model-value="journalRole"
          :options="JOURNAL_ROLE_TABS"
          value-key="key"
          label-key="label"
          @update:model-value="onJournalRoleChange"
        />
        <SearchComponent
          v-if="!isNarrow"
          :model-value="journalSearch"
          class="proc__journal-search"
          title="Номер заявки или ФИО"
          @update:model-value="onJournalSearchInput"
        />
        <div class="proc__journal-daterow">
          <DateFilter
            mode="range"
            :selected-date="journalSelectedDate"
            :date-range-start="journalRangeStart"
            :date-range-end="journalRangeEnd"
            @update:selected-date="journalSelectedDate = $event"
            @update:date-range-start="journalRangeStart = $event"
            @update:date-range-end="journalRangeEnd = $event"
            @apply="applyJournalFilters"
            @clear="clearJournalRange"
          />
          <button
            v-if="isNarrow"
            type="button"
            class="proc__journal-search-icon"
            :class="{ 'proc__journal-search-icon--active': journalSearchOpen || !!journalSearch.trim() }"
            aria-label="Поиск по журналу"
            @click="toggleJournalSearch"
          >
            <AppIcon
              name="search"
              class="proc__journal-search-icon__img"
            />
          </button>
          <Transition name="proc-journal-search">
            <div
              v-if="isNarrow && journalSearchOpen"
              class="proc__journal-search-overlay"
            >
              <div class="field search">
                <input
                  ref="journalSearchInputRef"
                  :value="journalSearch"
                  placeholder="Номер заявки или ФИО"
                  type="text"
                  class="proc__journal-search-input"
                  @input="onJournalSearchInput($event.target.value)"
                >
                <button
                  v-if="journalSearch.trim()"
                  type="button"
                  class="proc__journal-search-clear"
                  aria-label="Очистить поиск"
                  @click="clearJournalSearch"
                >
                  &times;
                </button>
              </div>
            </div>
          </Transition>
        </div>
        <button
          type="button"
          class="lk-button lk-button--ghost proc__journal-reset"
          :disabled="!journalHasFilters"
          @click="resetJournalFilters"
        >
          Сбросить
        </button>
      </div>
      <div
        v-if="journalError"
        class="proc__state proc__state--error"
      >
        {{ journalError }}
      </div>
      <div
        v-else-if="!journalReady"
        class="proc__skeleton proc__skeleton--table"
      />
      <div
        v-else
        class="proc__card proc__card--scroll proc__journal rt-table"
      >
        <!-- Шапка ленты: те же классы ячеек, что у строк, поэтому колонки совпадают
             по ширине без отдельной таблицы разметки. На телефоне строка события
             становится карточкой, и шапку прячет rt-head-row. -->
        <div class="proc__journal-line proc__journal-head rt-head-row">
          <span class="proc__journal-role-h">Событие</span>
          <span class="proc__journal-actor">Кто</span>
          <span class="proc__journal-app">Заявка</span>
          <span class="proc__journal-dur">Рабочее время</span>
          <span class="proc__journal-when">Когда</span>
        </div>
        <div
          v-for="e in journal"
          :key="`${e.application_id}-${e.role}-${e.occurred_at}`"
          class="proc__journal-row rt-row"
        >
          <!-- Бейдж роли без data-label намеренно: в карточке он читается как
               заголовок события, подпись «Событие: Согласование» была бы маслом масляным. -->
          <span
            class="proc__journal-role"
            :class="`proc__journal-role--${e.role}`"
          >{{ roleLabel(e.role) }}</span>
          <span
            class="proc__journal-actor"
            data-label="Кто"
            :title="e.actor_name"
          >{{ e.actor_name }}</span>
          <button
            v-if="e.application_number"
            type="button"
            class="proc__journal-app proc__journal-app--copy"
            data-label="Заявка"
            title="Скопировать номер заявки"
            @click="copyApplicationNumber(e.application_number)"
          >{{ e.application_number }}</button>
          <span
            v-else
            class="proc__journal-app"
            data-label="Заявка"
          >—</span>
          <span
            class="proc__journal-dur"
            data-label="Рабочее время"
          >{{ e.working_seconds == null ? '' : fmtDur(e.working_seconds) }}</span>
          <span
            class="proc__journal-when"
            data-label="Когда"
            :title="formatTimeAgo(e.occurred_at)"
          >{{ formatDateTime(e.occurred_at) }}</span>
        </div>
        <div
          v-if="journal.length === 0"
          class="proc__table-empty"
        >{{ journalHasFilters ? 'По фильтрам ничего не нашлось' : 'Событий за период нет' }}</div>
      </div>
      <Pager
        v-if="!journalError && journalReady && journalTotal > 0"
        class="proc__journal-pager"
        :page="journalPage"
        :total-pages="journalTotalPages"
        :total="fmtCount(journalTotal)"
        :loading="journalLoading"
        @update:page="goToJournalPage"
      />
    </section>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue';
import { getProcessingSummary, getProcessingJournal, getStuckApprovals, runReport } from '@/api/statistics.js';
import { formatDuration, formatDateTime, formatTimeAgo } from '@/utils/datetime';
import { MAX_REPORT_LIMIT } from '@/composables/useReportRequest';
import { useNarrowScreen } from '@/composables/useNarrowScreen.js';
import eventStream from '@/services/eventStream';
import { useDeletionsStore } from '@/stores/deletions';
import { copyText } from '@/utils/clipboard';
import HintTooltip from '@/components/ui/HintTooltip.vue';
import FilterTabs from '@/components/ui/FilterTabs.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import Pager from '@/components/ui/Pager.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import SearchComponent from '@/components/SearchComponent.vue';
import DateFilter from '@/components/DateFilter.vue';
import AnalyticsAreaChart from './AnalyticsAreaChart.vue';
import DirIcon from './DirIcon.vue';
import AppIcon from '@/components/icons/AppIcon.vue';

// Табы роли журнала не помещаются на телефоне (шесть кнопок) - на узком экране
// заменяются одной выпадающей кнопкой (BaseDropdown), а поиск сворачивается в
// иконку (тот же приём, что в Центре/кабинете - grep search-icon-btn).
const { isNarrow } = useNarrowScreen();

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

// Зависшие согласования (#1315 S4): снимок ТЕКУЩИХ заявок, ждущих решения
// согласующего дольше порога молчания. Как журнал - от периода вкладки не зависит
// (эндпоинт дат не принимает) и живой: перечитываем по тому же SSE-сигналу
// applications-center (согласование убирает заявку из списка) + фолбэк-опрос. Свой
// seq-guard и stuckReady, чтобы гонка устаревшего ответа не затирала актуальный и на
// рефреше не мигал скелетон.
const stuck = ref([]);
const stuckError = ref('');
const stuckReady = ref(false);
const stuckLoading = ref(false);
let stuckSeq = 0;
async function loadStuck() {
  const seq = ++stuckSeq;
  stuckError.value = '';
  stuckLoading.value = true;
  try {
    const data = await getStuckApprovals();
    if (seq !== stuckSeq) return;
    stuck.value = Array.isArray(data) ? data : [];
  } catch (e) {
    if (seq !== stuckSeq) return;
    stuckError.value = e?.message || 'Не удалось загрузить зависшие согласования';
  } finally {
    if (seq === stuckSeq) {
      stuckReady.value = true;
      stuckLoading.value = false;
    }
  }
}

const DAY_FORMS = ['день', 'дня', 'дней'];
function waitingText(days) {
  const n = Number(days) || 0;
  return `${n} ${plural(n, DAY_FORMS)}`;
}

// Журнал (#1251 S7, роли расширены в P7): лента решений по заявкам - голоса
// согласующих, принятия, отказы принимающего и отзывы инициатором - отдельным
// запросом, без кэша (реальное время). Обновляется по SSE-сигналу
// applications-center (тот же, что двигает Центр заявок) + рефетч. Свой seq-guard и journalReady, чтобы гонка
// устаревшего ответа не затирала актуальный и на рефреше не мигал скелетон.
const journal = ref([]);
const journalError = ref('');
const journalReady = ref(false);
const journalLoading = ref(false);
const JOURNAL_LIMIT = 50;
// Страница ленты (#1251 P5b): бэк отдаёт срез limit/offset и общее число событий
// периода. Автообновление (SSE/опрос) перечитывает ТЕКУЩУЮ страницу, а не сбрасывает
// на первую - иначе лента уезжала бы из-под читающего.
const journalPage = ref(1);
const journalTotal = ref(0);
// Размер страницы берём из ответа: limit бэк клампит своими правилами, и считать
// страницы по своей константе значило бы верить, что она совпала.
const journalPerPage = ref(JOURNAL_LIMIT);
const journalTotalPages = computed(() => Math.max(1, Math.ceil(journalTotal.value / journalPerPage.value)));

// Фильтры ленты (#1251 P5c): роль события, свой диапазон дат внутри периода вкладки
// и поиск по номеру заявки или ФИО актора. Отбор серверный - иначе «Всего» и
// страницы считались бы по нефильтрованной выборке.
// Роли перечислены в порядке пути заявки и совпадают с models.ProcessingJournalRoles
// на бэке: чужое значение эндпоинт отбивает 400, поэтому ключи вкладок - не свободный
// текст.
const JOURNAL_ROLE_TABS = [
  { key: '', label: 'Все' },
  { key: 'approval', label: 'Согласования' },
  { key: 'not_approved', label: 'Несогласования' },
  { key: 'acceptance', label: 'Принятия' },
  { key: 'rejection', label: 'Отказы' },
  { key: 'withdrawal', label: 'Отзывы' },
];
const JOURNAL_SEARCH_DEBOUNCE_MS = 300;

const journalRole = ref('');
const journalSearch = ref('');
const journalSelectedDate = ref(null);
const journalRangeStart = ref(null);
const journalRangeEnd = ref(null);

// Дата в 'YYYY-MM-DD' по ЛОКАЛЬНЫМ частям: toISOString увёл бы выбранный день на
// предыдущий восточнее UTC (бэк трактует границы периода в МСК).
function toYMD(d) {
  if (!(d instanceof Date)) return '';
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${d.getFullYear()}-${m}-${day}`;
}

// Свой диапазон журнала сужает период вкладки; не задан - берём период вкладки.
const journalFrom = computed(() => toYMD(journalSelectedDate.value || journalRangeStart.value) || props.from);
const journalTo = computed(() => toYMD(journalSelectedDate.value || journalRangeEnd.value) || props.to);

const journalHasFilters = computed(() => Boolean(
  journalRole.value
  || journalSearch.value.trim()
  || journalSelectedDate.value
  || journalRangeStart.value
  || journalRangeEnd.value,
));

let journalSeq = 0;
async function loadJournal() {
  const seq = ++journalSeq;
  const page = journalPage.value;
  journalError.value = '';
  journalLoading.value = true;
  try {
    const { items, meta } = await getProcessingJournal(
      journalFrom.value, journalTo.value, JOURNAL_LIMIT, (page - 1) * journalPerPage.value,
      { role: journalRole.value, q: journalSearch.value.trim() },
    );
    if (seq !== journalSeq) return;
    journal.value = Array.isArray(items) ? items : [];
    journalTotal.value = Number(meta?.total) || 0;
    journalPerPage.value = Number(meta?.per_page) || JOURNAL_LIMIT;
    // События могли уйти из окна (смена периода, удаление) - страница за хвостом
    // осталась бы пустой без единого способа вернуться, кроме кнопки «Назад».
    if (page > journalTotalPages.value) {
      journalPage.value = journalTotalPages.value;
      await loadJournal();
      return;
    }
  } catch (e) {
    if (seq !== journalSeq) return;
    journalError.value = e?.message || 'Не удалось загрузить журнал обработки';
  } finally {
    if (seq === journalSeq) {
      journalReady.value = true;
      journalLoading.value = false;
    }
  }
}

// Печать в поиске не должна бить по бэку на каждый символ. Дебаунс висит на вводе, а
// не на watch(journalSearch): иначе программная очистка поля в «Сбросить» ставила бы
// отложенный запрос поверх немедленного (два обращения на одно действие).
let journalSearchTimer = null;
function cancelJournalSearchDebounce() {
  if (journalSearchTimer) clearTimeout(journalSearchTimer);
  journalSearchTimer = null;
}

// Смена фильтра всегда возвращает на первую страницу: номер страницы относится к
// прежней выборке, оставлять его - показать пустоту за хвостом новой.
function applyJournalFilters() {
  cancelJournalSearchDebounce();
  journalPage.value = 1;
  loadJournal();
}

function onJournalRoleChange(role) {
  journalRole.value = role;
  applyJournalFilters();
}

function onJournalSearchInput(value) {
  journalSearch.value = value;
  cancelJournalSearchDebounce();
  journalSearchTimer = setTimeout(applyJournalFilters, JOURNAL_SEARCH_DEBOUNCE_MS);
}

// Раскрывающийся поиск на телефоне (зеркало Центра/кабинета): иконка-тоггл
// поверх ряда даты, автофокус на раскрытии - это не searchable-поле внутри
// BaseDropdown (там автофокус убран в #1303 - выбрасывал клавиатуру поверх
// списка опций), а отдельная страница поиска по журналу, тот же случай, что
// уже решён в ApplicationsCenter/UserApplications.
const journalSearchOpen = ref(false);
const journalSearchInputRef = ref(null);

// Возврат на десктоп - гасим мобильное раскрытие (зеркало Центра/кабинета).
watch(isNarrow, (narrow) => {
  if (!narrow) journalSearchOpen.value = false;
});

function toggleJournalSearch() {
  journalSearchOpen.value = !journalSearchOpen.value;
  if (journalSearchOpen.value) {
    nextTick(() => { journalSearchInputRef.value?.focus(); });
  }
}

function clearJournalSearch() {
  cancelJournalSearchDebounce();
  journalSearch.value = '';
  applyJournalFilters();
  journalSearchOpen.value = false;
}

function clearJournalRange() {
  journalSelectedDate.value = null;
  journalRangeStart.value = null;
  journalRangeEnd.value = null;
  applyJournalFilters();
}

function resetJournalFilters() {
  journalRole.value = '';
  journalSearch.value = '';
  clearJournalRange();
}

function goToJournalPage(next) {
  if (next < 1 || next > journalTotalPages.value || journalLoading.value) return;
  journalPage.value = next;
  loadJournal();
}

async function copyApplicationNumber(number) {
  if (!number) return;
  const value = String(number);
  const copied = await copyText(value);
  useDeletionsStore().notify(copied
    ? { prefix: 'Скопирован номер ', bold: value, type: 'success' }
    : { prefix: 'Не удалось ', bold: 'скопировать номер', type: 'error' });
}

const JOURNAL_ROLES = {
  approval: 'Согласование',
  not_approved: 'Несогласование',
  acceptance: 'Принятие',
  rejection: 'Отказ',
  withdrawal: 'Отзыв',
};
function roleLabel(role) {
  return JOURNAL_ROLES[role] || role;
}

/*
 * Динамика по дням (#1251 polish, п.7). Прежний столбчатый график «Узкие места»
 * повторял те же три числа, что и плитки над ним - отсюда «непонятно зачем».
 * Здесь показываем то, чего в плитках нет: как этап менялся ПО ДНЯМ периода, с
 * выбором этапа (раньше вид был один и без выбора). Отдельный запрос к движку
 * отчётов - разрез period он умеет, новый эндпоинт не нужен.
 */
const TREND_TABS = [
  { key: 'approval_time', label: 'Согласование' },
  { key: 'acceptance_time', label: 'Принятие' },
  { key: 'processing_time', label: 'Обработка' },
];
const TREND_HINT = 'Как менялось среднее время выбранного этапа внутри периода. Если в этот день заявки были, но этап не прошла ни одна, линия рвётся, а не падает в ноль.';

// Лимит строк ответа. Без явного значения движок берёт дефолт 100, а строки
// разреза period он сортирует хронологически ДО обрезки - на годовом периоде
// хвост молча пропал бы (осталась бы только первая сотня дней).
const TREND_LIMIT = MAX_REPORT_LIMIT;

// Гранулярность под длину окна: 365 точек по дням ещё читаются, но многолетний
// произвольный диапазон упёрся бы в лимит. Подпись рядом с графиком всегда
// говорит, в чём шаг, чтобы «по дням» не превращалось во враньё.
const TREND_GRANULARITY = [
  { maxDays: 92, key: 'day', label: 'по дням' },
  { maxDays: 730, key: 'week', label: 'по неделям' },
];
const TREND_GRANULARITY_FALLBACK = { key: 'month', label: 'по месяцам' };

const trendGranularity = computed(() => {
  const from = Date.parse(props.from);
  const to = Date.parse(props.to);
  if (!Number.isFinite(from) || !Number.isFinite(to)) return TREND_GRANULARITY[0];
  const days = Math.round((to - from) / 86400000) + 1;
  return TREND_GRANULARITY.find((g) => days <= g.maxDays) ?? TREND_GRANULARITY_FALLBACK;
});

const trendStage = ref('processing_time');
const trendRows = ref([]);
const trendError = ref('');
const trendReady = ref(false);
let trendSeq = 0;

async function loadTrend() {
  const seq = ++trendSeq;
  const metric = `avg_${trendStage.value}`;
  trendError.value = '';
  try {
    const data = await runReport({
      mode: 'aggregate',
      // applications_count тянем спутником НЕ для показа: движок анкерит бины
      // объединением строк всех запрошенных метрик. Без него день, когда заявки
      // были, но этап не прошла ни одна, вообще не пришёл бы в ответе - линия
      // склеилась бы через него. Со спутником такой день приходит без ключа
      // длительности -> ниже превращается в null -> график рисует разрыв.
      metrics: [metric, 'applications_count'],
      dimension: 'period',
      granularity: trendGranularity.value.key,
      limit: TREND_LIMIT,
      filters: [{ key: 'date_range', from: props.from, to: props.to }],
    });
    if (seq !== trendSeq) return;
    trendRows.value = data?.metric_rows ?? [];
  } catch (e) {
    if (seq !== trendSeq) return;
    trendRows.value = [];
    trendError.value = e?.message || 'Не удалось построить динамику';
  } finally {
    if (seq === trendSeq) trendReady.value = true;
  }
}

// Ключ метрики может отсутствовать в values (движок не дорисовывает ноль
// производным метрикам) - отдаём null, график рисует разрыв, а не падение в ноль.
const trendData = computed(() => {
  const metric = `avg_${trendStage.value}`;
  return trendRows.value.map((r) => ({
    timestamp: r.label,
    count: r.values?.[metric] ?? null,
  }));
});

const trendSeriesName = computed(
  () => TREND_TABS.find((t) => t.key === trendStage.value)?.label ?? 'Среднее время',
);

// «Время до завершения» с вкладки убрано (#1251 polish, п.6): это срок действия
// пропуска, а не работа бюро, и оно в тысячи раз больше остальных этапов (59 суток
// против секунд) - на графике три полезных столбца схлопывались в ноль. Метрика
// осталась в конструкторе отчётов, кому нужна.
const HIDDEN_STAGES = ['completion_time'];
const stages = computed(() =>
  (summary.value?.stages ?? []).filter((s) => !HIDDEN_STAGES.includes(s.key)),
);
const quality = computed(() => summary.value?.quality ?? []);
// Полные рейтинги по скорости (#1251 S6): согласующие по времени реакции и
// принимающие по времени принятия, оба уже отсортированы бэком (быстрые сверху).
const approvers = computed(() => summary.value?.approvers ?? []);
const acceptors = computed(() => summary.value?.acceptors ?? []);
// Разбивка (#1251 polish, п.10): один набор колонок в двух разрезах, переключаемых
// на месте, вместо отдельной таблицы «по организациям» с единственной длительностью.
const BREAKDOWN_TABS = [
  { key: 'organization', label: 'Организации' },
  { key: 'company', label: 'Компании' },
];
const BREAKDOWN_HINT = 'Сколько в среднем занимают этапы у заявок каждой организации или компании. Сверху те, у кого полная обработка идёт дольше всего. Прочерк — этап не прошла ни одна их заявка за период.';

const breakdownDim = ref('organization');
const byOrganization = computed(() => summary.value?.by_organization ?? []);
const byCompany = computed(() => summary.value?.by_company ?? []);
const breakdownRows = computed(() =>
  (breakdownDim.value === 'company' ? byCompany.value : byOrganization.value),
);
const breakdownNameHeader = computed(() =>
  (breakdownDim.value === 'company' ? 'Компания' : 'Организация'),
);
const totalApplications = computed(() => summary.value?.total_applications ?? 0);

// Пустой период — заявок не подавали: доли и длительности считать не по чему.
const isEmpty = computed(() => !summary.value || totalApplications.value === 0);

// Первая загрузка: показываем скелетоны во ВСЕХ секциях. Без этого график и
// таблицы (в отличие от плиток) на холодном заходе флэшнули бы «Нет данных» до
// прихода ответа — «пусто» и «ещё грузится» смешались бы. На рефреше (ready уже
// true) скелетон не мигает: до нового ответа держим прежние данные.
const showSkeleton = computed(() => loading.value && !ready.value);

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
    hint: 'От подачи заявки до согласования. Считается по рабочему времени бюро: ночь и выходные не учитываются. Если событие не попало в рабочие часы графика, показывается календарное время.',
    basis: 'work',
  },
  acceptance_time: {
    hint: 'От согласования до принятия заявки в работу принимающим. По рабочему времени бюро; если событие вне рабочих часов графика - календарное время.',
    basis: 'work',
  },
  processing_time: {
    hint: 'Полное время от подачи до принятия в работу (согласование и принятие вместе). По рабочему времени бюро; если событие вне рабочих часов графика - календарное время.',
    basis: 'work',
  },
  completion_time: {
    hint: 'Срок действия пропуска: от подачи до завершения по истечении срока вложений. Календарное время — рабочие часы бюро тут не вычитаются.',
    basis: 'calendar',
  },
};

const BASIS_LABEL = { work: 'раб. время бюро', calendar: 'календарное' };

// Общий хвост подсказки: объясняет, ЧТО за числа на плитке (среднее vs 9 из 10) —
// раньше было непонятно, среднее это, максимум или минимум (#1251 polish, пп.2,5).
const STAGE_NUMBERS_HINT = 'Крупное число — среднее по заявкам периода. Строкой ниже — время, в которое укладываются 9 заявок из 10: редкие зависшие заявки его не задирают.';

function stageMeta(key) {
  const m = STAGE_META[key] || { hint: '', basis: 'work' };
  return {
    ...m,
    hint: m.hint ? `${m.hint} ${STAGE_NUMBERS_HINT}` : '',
    basisLabel: BASIS_LABEL[m.basis],
  };
}

// Пояснение «что считается» для метрик качества. Ветки отказа разведены (#1251
// polish, п.8): по объединённой доле не было понятно, кто завернул заявку.
const QUALITY_HINTS = {
  rejected_rate: 'Доля заявок, которым отказал принимающий (статус «Отказано»), от всех поданных за период. Одна заявка может попасть и сюда, и в несогласованные — доли не складываются.',
  not_approved_rate: 'Доля заявок, которые не согласовали согласующие (отметка «Не согласовано»), от всех поданных за период. Одна заявка может попасть и сюда, и в отказы принимающего — доли не складываются.',
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

function reload() {
  loadSummary();
  loadJournal();
  loadTrend();
  loadStuck();
}

// Новый период - новая лента: страница сбрасывается на первую (её события к
// прошлому окну отношения не имеют). Ручное «Обновить» страницу сохраняет.
// Свой диапазон журнала - сужение внутри прежнего периода, к новому он отношения не
// имеет и тоже сбрасывается; роль и поиск остаются, они от дат не зависят.
watch(
  () => [props.from, props.to],
  () => {
    journalSelectedDate.value = null;
    journalRangeStart.value = null;
    journalRangeEnd.value = null;
    cancelJournalSearchDebounce();
    journalPage.value = 1;
    reload();
  },
);

// Смена этапа перестраивает только график - бандл и лента от неё не зависят.
watch(trendStage, loadTrend);

// Real-time: тот же SSE-сигнал, что двигает Центр заявок (подача/согласование/
// принятие/пересылка). На каждый сигнал перечитываем журнал И зависшие согласования
// за текущий момент (согласование убирает заявку из зависших); бандл-метрики не
// трогаем — они тяжёлые и кэшируются, лента и снимок зависших лёгкие и живые.
let journalStreamOff = null;
let journalPoll = null;
const JOURNAL_POLL_MS = 60000;
function refreshLive() {
  loadJournal();
  loadStuck();
}
onMounted(() => {
  reload();
  // connect держит SSE-соединение через refcount (как у прочих потребителей
  // eventStream); без него subscribe молча полагался бы на чужой connect.
  eventStream.connect();
  journalStreamOff = eventStream.subscribe('applications-center', refreshLive);
  // Фолбэк-опрос: если SSE ушёл в fallback, лента и зависшие всё равно не протухнут.
  journalPoll = setInterval(refreshLive, JOURNAL_POLL_MS);
});
onBeforeUnmount(() => {
  if (journalStreamOff) journalStreamOff();
  journalStreamOff = null;
  eventStream.disconnect();
  if (journalPoll) clearInterval(journalPoll);
  journalPoll = null;
  cancelJournalSearchDebounce();
});

defineExpose({ refresh: reload });
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
  color: var(--danger-text);
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

/* Короткая подпись чипа «Журнал» - только на мобилке (эталон §12: подмена
   текста парой классов, не обрезка). На десктопе чип один - полный текст. */
.proc__group-chip-short {
  display: none;
}

.proc__group-rule {
  flex: 1;
  height: 1px;
  background: var(--color-border);
}

/* Строка над графиком динамики: выбор этапа + подсказка что показано. */
.proc__trend-head {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

/* ===== ПЛИТКИ ===== */
.proc__tiles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
  gap: 12px;
}

.proc__tile {
  background: var(--surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 12px 14px;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
  animation: proc-tile-in 0.35s ease both;
}

.proc__tile:hover {
  border-color: var(--accent);
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

.proc__tile-val {
  font-size: 24px;
  font-weight: 700;
  color: var(--color-text);
  margin-top: 6px;
  line-height: 1.1;
}

/* «среднее» под крупным числом: без этой подписи было непонятно, среднее это,
   максимум или минимум (#1251 polish, п.2). Отдельной строкой, а не в строку с
   числом: иначе на узкой плитке подпись то влезает, то переносится — вид скачет. */
.proc__tile-agg {
  display: block;
  margin-top: 2px;
  font-size: 11px;
  font-weight: 500;
  line-height: 1.2;
  color: var(--color-text-muted);
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
  background: color-mix(in srgb, var(--success) 10%, var(--surface));
  color: var(--success-text);
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
  background: color-mix(in srgb, var(--success) 12%, var(--surface));
  color: var(--success-text);
}

.proc__delta--bad {
  background: color-mix(in srgb, var(--danger) 12%, var(--surface));
  color: var(--danger-text);
}

.proc__delta--neutral {
  background: var(--color-bg);
  color: var(--color-text-muted);
  border: 1px solid var(--color-border);
}

/* ===== КАРТОЧКИ ===== */
.proc__card {
  background: var(--surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 18px 20px;
}

.proc__card--table {
  padding: 6px 6px;
  overflow-x: auto;
}

/* Рейтинги согласующих/принимающих — полный список, поэтому карточка скроллится, а
   шапка липнет к верху (видно колонки при прокрутке длинного рейтинга). */
.proc__card--scroll {
  max-height: 320px;
  overflow-y: auto;
  /* Верхний padding убран намеренно: sticky-шапка липнет к границе padding-box, и в
     этой щели над ней проезжали строки данных (было видно текст сквозь заголовки). */
  padding-top: 0;
  /* Место под скроллбар резервируем всегда, иначе его появление сужает контент и
     колонки дёргаются (тот же приём, что в UserLoginHistory). */
  scrollbar-gutter: stable;
}

.proc__card--scroll .proc__table thead th {
  position: sticky;
  top: 0;
  background: var(--surface);
  z-index: 1;
}

/* Колонка ранга: узкая, номер по центру. */
.proc__rank,
.proc__rank-h {
  width: 40px;
  text-align: center;
  color: var(--color-text-muted);
  font-variant-numeric: tabular-nums;
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

/* ===== ДВЕ КОЛОНКИ =====
   minmax(0, 1fr), а не 1fr: у трека 1fr минимум - min-content содержимого, и
   `min-width: 460px` таблицы внутри тянул колонку до 474px независимо от того,
   что карточка вокруг неё скроллится (`overflow-x: auto` зануляет автоминимум
   только самому скроллеру, но не деду-гриду). На 390 обе карточки рейтинга
   вставали шириной 474 в контейнер 368, распирали вкладку вправо, и вбок ехала
   вся панель целиком вместо таблицы внутри блока. */
.proc__cols {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 20px;
}

/* Тот же ноль-минимум нужен и самим секциям: без него распирает уже не трек,
   а элемент грида (лечим родителя, а не блок внутри - #1685). */
.proc__cols > .proc__group {
  min-width: 0;
}

@media (max-width: 900px) {
  .proc__cols {
    grid-template-columns: minmax(0, 1fr);
  }
}

/* ===== ТАБЛИЦА ===== */
.proc__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  /* Ширины колонок заданы явно и не зависят от длины данных: при auto-раскладке
     колонки перескакивали при смене периода, разреза и страницы. Доли, а не
     пиксели: с фиксированными px колонка имени получала остаток и на узком
     контейнере схлопывалась до нуля вместо горизонтального скролла. */
  table-layout: fixed;
  /* Ниже этой ширины таблица не ужимается, а честно скроллится внутри карточки
     (.proc__card--table { overflow-x: auto }) - иначе имя/организация исчезают.
     Минимум свой у каждой раскладки: он подобран так, чтобы заголовки колонок
     («Время реакции», «Согласование») влезали целиком, а не резались многоточием. */
  min-width: 460px;
}

.proc__table th {
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
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

/* Подпись колонки обязана стоять там же, где её значения. Селектор `.proc__table th`
   (0,1,1) перебивает `.proc__num` и `.proc__rank-h` (0,1,0), поэтому заголовки
   числовых колонок и «#» прижимались влево, пока сами числа стояли справа и по
   центру - шапка не сходилась со столбцами. */
.proc__table th.proc__num {
  text-align: right;
}

.proc__table th.proc__rank-h {
  text-align: center;
}

/* Доли числовых колонок разные, потому что колонок разное число: в рейтингах их
   две, в разбивке четыре. Имени в обоих случаях остаётся около трети ширины. */
.proc__table--rating .proc__num {
  width: 30%;
}

.proc__table--breakdown {
  min-width: 680px;
}

.proc__table--breakdown .proc__num {
  width: 17%;
}

/* Зависшие согласования (#1315 S4): свои доли колонок, НЕ reuse --rating. У рейтинга
   .proc__num=30% на две колонки (60%), из-за чего номер заявки и имя делили по 20%,
   а 140px-кнопка номера (стиль ленты) вылезала на колонку имени. Здесь номер широкий
   (не режем - по нему копируют), имя ellipsis-им, счётчики узкие. */
.proc__table--stuck {
  min-width: 460px;
}

.proc__table--stuck .proc__col-app {
  width: 34%;
}

.proc__table--stuck .proc__num {
  width: 16%;
}

/* Номер заявки в ячейке: слева, во всю ширину колонки; лишнее - многоточием
   (копируется полное значение из данных, а не из отображаемого текста). Без width:140px
   и правого выравнивания из .proc__journal-app - те для флекс-ленты журнала. */
.proc__stuck-app {
  display: inline-block;
  max-width: 100%;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-text);
  font-variant-numeric: tabular-nums;
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

/* ===== ЖУРНАЛ (лента) ===== */
.proc__journal {
  /* Верх без отступа: иначе строки проезжают в щели над липкой шапкой ленты. */
  padding: 0 4px 2px;
  /* Узкий экран: лента едет вбок вместе с шапкой, а не схлопывает колонку с ФИО в
     ноль - ширины остальных ячеек фиксированные, сжиматься там нечему. */
  overflow-x: auto;
}

/* Строка фильтров ленты: роль, поиск, свой диапазон дат и сброс. */
.proc__journal-filters {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

/* Ряд даты на десктопе - те же дети в общем потоке фильтров, без обособления
   (обёртка нужна только на телефоне под оверлей поиска). Правило ОБЯЗАНО стоять
   раньше мобильного @media - иначе оно, будучи позже в файле при равной
   специфичности, перебивает display:flex из media независимо от ширины экрана
   (заметил по скриншоту стенда: ряд даты не группировался, волна 8). */
.proc__journal-daterow {
  display: contents;
}

.proc__journal-search {
  width: 240px;
}

.proc__journal-reset {
  height: 30px;
  padding: 0 16px;
  font-size: 13px;
}

/* Общая раскладка строки ленты - и для шапки, и для событий: одни ширины ячеек,
   поэтому колонки шапки и данных всегда совпадают. */
.proc__journal-line,
.proc__journal-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 8px;
  font-size: 13px;
  /* Сумма фиксированных колонок плюс читаемый минимум на ФИО (~150px): уже -
     горизонтальный скролл, а не сжатие имени в несколько букв. */
  min-width: 700px;
}

.proc__journal-row {
  border-bottom: 1px solid var(--color-bg);
}

.proc__journal-row:last-child {
  border-bottom: none;
}

/* Шапка ленты: липнет к верху карточки, как thead у таблиц. */
.proc__journal-head {
  position: sticky;
  top: 0;
  z-index: 1;
  background: var(--surface);
  border-bottom: 1px solid var(--color-border);
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-muted);
  text-transform: none;
}

/* Ширина колонки роли общая для шапки и строк: врозь они разъедутся при первой же
   правке одного из значений. Фиксированная, а не min-width - подписи разной длины
   («Принятие» vs «Согласование») раздвигали колонку по-разному в каждой строке. */
.proc__journal-role-h,
.proc__journal-role {
  flex-shrink: 0;
  width: 122px;
  /* Бейдж роли центрирован, поэтому и подпись колонки по центру - иначе шапка
     ленты уезжает влево от собственных значений. */
  text-align: center;
}

.proc__journal-role {
  text-align: center;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 9px;
  border-radius: var(--radius-pill);
  white-space: nowrap;
}

/* Цвет бейджа = исход: синий - шаг пути пройден (согласование), зелёный - заявка
   пошла в работу, оранжевый - её не согласовали, красный - отказал принимающий,
   серый - инициатор забрал сам. Пастель (тёмный текст на светлом фоне), как в
   остальных бейджах проекта. */
.proc__journal-role--approval {
  background: var(--color-primary-tint);
  color: var(--accent-text);
}

.proc__journal-role--acceptance {
  background: color-mix(in srgb, var(--success) 12%, var(--surface));
  color: var(--success-text);
}

.proc__journal-role--not_approved {
  background: color-mix(in srgb, var(--warning) 14%, var(--surface));
  color: var(--warning-text);
}

.proc__journal-role--rejection {
  background: color-mix(in srgb, var(--danger) 12%, var(--surface));
  color: var(--danger-text);
}

.proc__journal-role--withdrawal {
  background: rgba(120, 120, 120, 0.14);
  color: var(--text);
}

.proc__journal-actor {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-text);
  font-weight: 500;
}

/* Колонки ленты фиксированной ширины: раньше номер и время подстраивались под
   содержимое, и соседние строки не выстраивались в столбцы (#1251 polish, п.12). */
.proc__journal-app {
  flex-shrink: 0;
  width: 140px;
  text-align: right;
  color: var(--color-text-muted);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

/* Номер кликабелен - копируется в буфер. */
.proc__journal-app--copy {
  border: none;
  background: none;
  padding: 0;
  font: inherit;
  cursor: pointer;
  border-radius: var(--radius-sm);
}

.proc__journal-app--copy:hover,
.proc__journal-app--copy:focus-visible {
  color: var(--accent-text);
  text-decoration: underline;
  outline: none;
}

.proc__journal-dur {
  flex-shrink: 0;
  width: 110px;
  text-align: right;
  color: var(--color-text-muted);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.proc__journal-when {
  flex-shrink: 0;
  width: 124px;
  text-align: right;
  color: var(--color-text-muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

/* Пейджер ленты - общий ui/Pager: здесь только место в карточке и размер кнопок. */
.proc__journal-pager {
  justify-content: flex-end;
  margin-top: 10px;
  color: var(--color-text-muted);
}

/* «Всего» прижато к левому краю карточки, страницы - к правому. */
.proc__journal-pager :deep(.pager__total) {
  margin-right: auto;
}

.proc__journal-pager :deep(.pager__btn) {
  padding: 6px 14px;
}

/* ===== МОБИЛКА (<=768) ===== */
@media (max-width: 768px) {
  /* Шапка группы: чип не переносится и на телефоне уходит за правый край панели -
     «решения по заявкам: согласования, принятия, отказы, отзывы» шире экрана сам
     по себе, «ждут решения дольше порога напоминаний» не встаёт рядом с длинным
     заголовком. Разрешаем шапке вторую строку, а чипу - перенос текста; линейка
     добирает остаток строки. Тот же блок, что у шапок групп дашборда. */
  .proc__group-head {
    flex-wrap: wrap;
    gap: 8px;
  }

  .proc__group-chip {
    white-space: normal;
  }

  .proc__group-rule {
    min-width: 24px;
  }

  /* Журнал: короткая подпись держит заголовок и чип на одной строке даже на
     320px - полная версия («решения по заявкам: согласования, принятия,
     отказы, отзывы») перелетала на вторую строку (владелец, волна 8). */
  .proc__group-chip-full {
    display: none;
  }

  .proc__group-chip-short {
    display: inline;
  }

  /* Рейтинги на телефоне прокручиваются вместе со страницей (#1097 волна 5):
     своя область прокрутки внутри прокручиваемой страницы отбирает жест у окна.
     Sticky-шапка рейтинга при этом теряет смысл - липнуть не к чему, - поэтому
     снимается вместе с ограничением высоты. */
  .proc__card--scroll {
    max-height: none;
    overflow-y: visible;
  }

  .proc__card--scroll :deep(thead th),
  .proc__card--scroll th {
    position: static;
  }

  /* Фильтры ленты на телефоне - две строки (волна 8, было четыре с лишним):
     [дропдаун роли][Сбросить] на первой, [дата][поиск] на второй. Порядок
     задаём order, а не перестановкой в разметке (эталон §2.2) - в DOM ряд
     даты идёт раньше кнопки сброса (десктопный порядок сохраняем нетронутым:
     табы -> поиск -> дата -> сброс), а на телефоне сброс должен встать сразу
     за дропдауном, до ряда даты. */
  .proc__journal-role-dropdown {
    order: 1;
    width: 165px;
    flex: 0 0 auto;
  }

  .proc__journal-reset {
    order: 2;
    height: 35px;
    margin-left: auto;
  }

  /* Ряд даты: `display: contents` на десктопе оставляет DateFilter прямым
     ребёнком .proc__journal-filters (визуально ничего не меняется), на
     телефоне становится настоящим flex-рядом и переносится на новую строку
     через flex-basis: 100% - надёжнее, чем полагаться на то, что ряду 1 не
     хватит места (#1097 общий урок про "не полагаться на порядок"). */
  .proc__journal-daterow {
    display: flex;
    align-items: center;
    gap: 8px;
    position: relative;
    order: 3;
    flex: 1 1 100%;
  }

  /* Ширина поля периода зашита в DateFilter (215px, .date-filter И .date-field
     оба) - тем же приёмом, что в ArchiveBackfillPanel/ArchiveDownloadPanel,
     отдаём ей остаток ряда после иконки поиска. */
  .proc__journal-daterow :deep(.date-filter),
  .proc__journal-daterow :deep(.date-field) {
    width: 100%;
    min-width: 0;
  }

  .proc__journal-daterow :deep(.date-filter) {
    flex: 1;
  }

  /* Иконка-тоггл поиска (мобилка): раскрывает поле оверлеем поверх ряда даты -
     тот же приём и те же размеры, что в ApplicationsCenter/UserApplications. */
  .proc__journal-search-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 40px;
    height: 40px;
    border: 1px solid var(--color-border);
    border-radius: 50%;
    background: var(--surface);
    cursor: pointer;
    flex-shrink: 0;
    transition: background 0.15s ease, border-color 0.15s ease;
  }

  @media (hover: hover) {
    .proc__journal-search-icon:hover {
      background: var(--color-bg);
      border-color: var(--accent);
    }
  }

  .proc__journal-search-icon--active {
    background: var(--color-bg);
    border-color: var(--accent);
  }

  .proc__journal-search-icon__img {
    width: 16px;
    height: 16px;
    color: var(--text);
    stroke-width: 2.1;
  }

  /* Оверлей раскрытия: поверх ряда даты, оставляя иконку (40px) открытой справа. */
  .proc__journal-search-overlay {
    position: absolute;
    top: 0;
    bottom: 0;
    left: 0;
    right: 48px;
    z-index: 1;
    display: flex;
    align-items: center;
    background: var(--surface);
    border-radius: var(--radius-md);
  }

  .proc__journal-search-overlay .field.search {
    display: flex;
    align-items: center;
    width: 100%;
    height: 40px;
    border: 1px solid var(--color-border);
    border-radius: 15px;
    padding: 0 12px;
    box-sizing: border-box;
  }

  .proc__journal-search-input {
    flex: 1;
    min-width: 0;
    border: none;
    outline: none;
    background: transparent;
    font-size: 14px;
    color: var(--color-text);
  }

  .proc__journal-search-clear {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    padding: 0;
    border: none;
    background: transparent;
    color: var(--color-text-muted);
    font-size: 20px;
    line-height: 1;
    cursor: pointer;
    flex-shrink: 0;
  }

  @media (hover: hover) {
    .proc__journal-search-clear:hover {
      color: var(--accent-text);
    }
  }

  /* Раскрытие вправо-налево - clip-path (композитится, ряд не двигается). */
  .proc-journal-search-enter-active,
  .proc-journal-search-leave-active {
    transition: clip-path 0.25s ease;
  }

  .proc-journal-search-enter-from,
  .proc-journal-search-leave-to {
    clip-path: inset(0 0 0 100%);
  }

  .proc-journal-search-enter-to,
  .proc-journal-search-leave-from {
    clip-path: inset(0 0 0 0);
  }
}

/* ===== ТЕЛЕФОН: СТРОКА -> КАРТОЧКА (<=767.98) =====
   Порог именно 767.98, как у responsive-tables.css: на ровно 768 иначе собирается
   гибрид - подписи ячеек уже есть, а карточек ещё нет.

   Таблицы вкладки держат минимум ширины (460/680/700), чтобы на десктопе колонки
   не схлопывались, а узкий контейнер давал честный горизонтальный скролл внутри
   карточки (#1251). На телефоне этот минимум ни к чему: строка становится карточкой
   «подпись - значение», и скроллить вбок нечего. Сам минимум обязательно снять -
   `.rt-table { overflow-x: hidden }` иначе не уберёт переполнение, а молча обрежет
   карточку по правому краю. */
@media (max-width: 767.98px) {
  /* Двухклассовые селекторы: одноклассовые правила-источники объявлены выше по
     файлу с той же специфичностью, и при равенстве побеждает более позднее - но
     полагаться на порядок в SFC уже обжигало (#1550). */
  .proc__table.proc__table--rating,
  .proc__table.proc__table--breakdown,
  .proc__table.proc__table--stuck {
    min-width: 0;
  }

  /* Двойная рамка: `.proc__card` держит границу и padding вокруг всего блока
     (--color-border), а каждая строка внутри в карточном режиме получает свою
     точно такую же рамку (responsive-tables.css `.rt-table .rt-row`) - на
     телефоне это читается как рамка внутри рамки у Согласующих, Принимающих,
     Разбивки и Зависших согласований. У Журнала та же ловушка отдельным путём:
     rt-table навешен на тот же div, что и proc__card (не на вложенную таблицу).
     Внешний контейнер на телефоне становится прозрачным без своего отступа -
     границу несут уже сами карточки-строки.
     :has(.rt-row) обязателен: при нуле строк карточек нет вовсе, и `.proc__table-empty`
     своей рамки не несёт (только текст) - без гварда пустая выборка осталась бы
     голым текстом без единого признака таблицы (тот же дефект, что уже ловили на
     FactTable). Рамку снимаем только когда есть что ею заменить. */
  .proc__card--table:has(.rt-row),
  .proc__journal:has(.rt-row) {
    border: none;
    border-radius: 0;
    padding: 0;
    background: transparent;
  }

  .proc__journal .proc__journal-line,
  .proc__journal .proc__journal-row {
    min-width: 0;
  }

  /* Обрезка многоточием - приём узкой колонки. В карточке ФИО и название
     организации показываем целиком: `max-width: 0` иначе схлопнет ячейку в ноль
     (ширину от rt-* оно перебивает), а длинное слово без пробелов уедет за край. */
  .rt-row > .proc__ellipsis,
  .proc__journal-row > .proc__journal-actor {
    flex: 0 0 auto;
    max-width: none;
    overflow: visible;
    white-space: normal;
    overflow-wrap: anywhere;
    text-overflow: clip;
  }

  /* Бейдж события - заголовок карточки: по содержимому и слева, иначе ячейка
     карточки растянула бы пилюлю на всю ширину. */
  .proc__journal-row > .proc__journal-role {
    align-self: flex-start;
    width: auto;
    margin-bottom: 2px;
  }

  /* Номер заявки копируется по тапу: зону нажатия доводим до 44px невидимым
     псевдоэлементом, не раздувая саму строку карточки (эталон адаптива, §8).
     Именно ::after - ::before у ячейки карточки занят подписью поля (rt-*). */
  .proc__stuck-app,
  .proc__journal-app--copy {
    position: relative;
  }

  /* Обрезка номера многоточием нужна была узкой ячейке таблицы; в карточке номер
     стоит отдельной строкой целиком. Снять `overflow: hidden` здесь обязательно:
     он режет по границе кнопки и её собственную зону нажатия - тап мимо строки
     номера уходил в соседнюю ячейку. */
  .proc__stuck-app {
    max-width: none;
    overflow: visible;
    text-overflow: clip;
  }

  /* -15px по вертикали, а не -12: строка номера ростом 15px, и 12 дают 39 - до
     44 не дотягивает. Считать надо от реальной высоты элемента, а не брать
     число из соседнего компонента. */
  .proc__stuck-app::after,
  .proc__journal-app--copy::after {
    content: '';
    position: absolute;
    inset: -15px -10px;
  }
}
</style>
