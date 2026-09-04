<template>
  <BaseModal
    :show="show"
    :title="reportTitle"
    width="720px"
    radius="30px"
    content-class="pass-report-modal"
    content-testid="ob-pass-report"
    @close="$emit('close')"
  >
    <div
      class="pr-section"
      data-testid="pass-report-live"
    >
      <div class="pr-today">
        <div class="pr-today__text">
          <div class="pr-today__title">
            {{ liveTitle }}
          </div>
          <div class="pr-today__hint">
            отчётные сутки: с 21:30 вчера до 21:30 сегодня
          </div>
        </div>
        <RefreshButton
          :loading="loadingLive"
          @refresh="loadLive"
        />
      </div>

      <div
        v-if="loadingLive && !live"
        class="pr-empty"
      >
        Загрузка...
      </div>
      <template v-else-if="live">
        <div class="pr-cards">
          <div
            v-for="s in sectionDefs"
            :key="s.key"
            class="pr-card"
          >
            <div class="pr-card__title">
              <AppIcon
                :name="s.icon"
                class="pr-card__icon"
              />
              {{ s.title }}
            </div>
            <div class="pr-card__stat pr-card__stat--in">
              <span class="pr-card__label">{{ s.inLabel }}</span>
              <span
                class="pr-card__num"
                :data-testid="`pr-total-${s.key}-in`"
              >{{ liveCount(s.inField) }}</span>
            </div>
            <div class="pr-card__stat pr-card__stat--out">
              <span class="pr-card__label">{{ s.outLabel }}</span>
              <span
                class="pr-card__num"
                :data-testid="`pr-total-${s.key}-out`"
              >{{ liveCount(s.outField) }}</span>
            </div>
          </div>
        </div>

        <!-- Кейс «открыл в 21:31»: в 21:30 начались новые отчётные сутки, текущие -
             пустые. Уходящий охранник тупанул бы на нулях; подсказываем, что его
             отметки уже в прошлых днях. Показываем всегда, когда текущие отчётные
             сутки пусты. -->
        <div
          v-if="liveIsEmpty"
          class="pr-hint"
          data-testid="pass-report-empty-hint"
        >
          <b>За текущие отчётные сутки отметок пока нет.</b>
          Отчётные сутки меняются каждый день в 21:30. Если ищете отчёт за прошедший день -
          нажмите «Показать прошлые дни» ниже.
        </div>

        <!-- Разбивка по охранникам видна, только когда за отчётные сутки отмечал не
             один человек (у деда на посту одна строка - лишний шум не показываем). -->
        <div
          v-if="live.rows && live.rows.length > 1"
          class="pr-breakdown"
          data-testid="pass-report-breakdown"
        >
          <div class="pr-breakdown__title">
            Кто сколько отметил
          </div>
          <div
            v-for="row in live.rows"
            :key="row.user_id"
            class="pr-breakdown__row"
          >
            <span class="pr-breakdown__name">{{ userLabel(row) }}</span>
            <span class="pr-breakdown__nums">{{ rowSummary(row) }}</span>
          </div>
        </div>
      </template>
      <div
        v-else
        class="pr-empty"
      >
        Не удалось загрузить отчёт
      </div>
    </div>

    <div class="pr-section pr-section--history">
      <button
        class="pr-history-toggle"
        data-testid="pass-report-history-toggle"
        :aria-expanded="historyOpen ? 'true' : 'false'"
        @click="toggleHistory"
      >
        <span>{{ historyOpen ? 'Скрыть прошлые дни' : 'Показать прошлые дни' }}</span>
        <span
          class="pr-history-toggle__arrow"
          :class="{ open: historyOpen }"
          aria-hidden="true"
        >▾</span>
      </button>

      <!-- Плавное раскрытие/скрытие через grid-template-rows 0fr<->1fr (эталон
           проекта): содержимое остаётся в DOM, поэтому анимируется и закрытие. -->
      <div
        class="pr-history-wrap"
        :class="{ open: historyOpen }"
      >
        <div class="pr-history-inner">
          <div
            class="pr-history"
            data-testid="pass-report-history"
          >
            <div class="pr-history__head">
              <span class="pr-history__title">Прошлые дни</span>
              <div class="pr-history__filter">
                <span class="pr-history__filter-label">Период:</span>
                <DateFilter
                  mode="range"
                  :date-range-start="dateRangeStart"
                  :date-range-end="dateRangeEnd"
                  @update:date-range-start="dateRangeStart = $event"
                  @update:date-range-end="dateRangeEnd = $event"
                  @apply="loadDays"
                  @clear="clearDates"
                />
              </div>
            </div>

            <div
              v-if="loadingDays"
              class="pr-empty"
            >
              Загрузка...
            </div>
            <div
              v-else-if="!days.length"
              class="pr-empty"
              data-testid="pass-report-days-empty"
            >
              Пока нет данных за прошлые дни
            </div>
            <template v-else>
              <div class="pr-days">
                <div
                  v-for="day in days"
                  :key="day.report_date"
                  class="pr-day"
                  data-testid="pass-report-day"
                >
                  <div class="pr-day__date">
                    {{ formatDayHuman(day.report_date) }}
                  </div>
                  <div class="pr-day__total">
                    {{ rowSummary(day.totals) }}
                  </div>
                  <div
                    v-if="day.rows && day.rows.length > 1"
                    class="pr-day__people"
                  >
                    <div
                      v-for="row in day.rows"
                      :key="`${day.report_date}-${row.user_id}`"
                      class="pr-day__person"
                    >
                      <span class="pr-day__person-name">{{ userLabel(row) }}</span>
                      <span class="pr-day__person-nums">{{ rowSummary(row) }}</span>
                    </div>
                  </div>
                </div>
              </div>

              <button
                class="pr-export"
                :disabled="isExporting || !days.length"
                data-testid="pass-report-export"
                @click="exportToExcel"
              >
                <AppIcon
                  v-if="!isExporting"
                  name="export"
                  class="pr-export__icon"
                />
                <span>{{ isExporting ? 'Формируем файл...' : 'Скачать в Excel' }}</span>
              </button>
            </template>
          </div>
        </div>
      </div>
    </div>
  </BaseModal>
</template>

<script>
import ExcelJS from 'exceljs';
import BaseModal from '@/components/ui/BaseModal.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import DateFilter from '@/components/DateFilter.vue';
import { getPassReportLive, listPassReports } from '@/api/pass-reports';
import { useDeletionsStore } from '@/stores/deletions';
import AppIcon from '@/components/icons/AppIcon.vue';
import { moscowParts, serverNow } from '@/utils/serverTime';
import { formatDateTime } from '@/utils/datetime';

const MONTHS = ['января', 'февраля', 'марта', 'апреля', 'мая', 'июня', 'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря'];

// Дата -> YYYY-MM-DD по ЛОКАЛЬНЫМ частям: toISOString увёз бы выбранный день на
// предыдущий у пользователей восточнее UTC.
function ymd(d) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

export default {
  name: 'PassReportModal',
  components: { AppIcon, BaseModal, RefreshButton, DateFilter },
  props: {
    show: { type: Boolean, required: true },
    tableId: { type: Number, default: null },
    tableType: { type: String, default: '' },
    tableDisplayName: { type: String, default: '' },
    currentUserName: { type: String, default: '' },
  },
  emits: ['close'],
  data() {
    return {
      loadingLive: false,
      live: null,
      loadingDays: false,
      days: [],
      dateRangeStart: null,
      dateRangeEnd: null,
      isExporting: false,
      historyOpen: false,
    };
  },
  computed: {
    // Секции по типу таблицы; «чужая» пара показывается, только если в данных
    // реально есть события (не фабрикуем нули там, где их не бывает). Простые
    // слова направления - «заехало/выехало» для машин, «зашло/вышло» для людей.
    sectionDefs() {
      const defs = [];
      if (this.tableType === 'cars' || this.anyCount('car_entries') || this.anyCount('car_exits')) {
        defs.push({ key: 'cars', icon: 'car', title: 'Машины', inLabel: 'Заехало', outLabel: 'Выехало', inField: 'car_entries', outField: 'car_exits' });
      }
      if (this.tableType === 'people' || this.anyCount('people_entries') || this.anyCount('people_exits')) {
        defs.push({ key: 'people', icon: 'employees', title: 'Люди', inLabel: 'Зашло', outLabel: 'Вышло', inField: 'people_entries', outField: 'people_exits' });
      }
      return defs;
    },
    /**
     * Машины ездят, люди ходят - заголовок берёт слово по типу таблицы. Прежде
     * окно всегда звалось «Отчёт по проходам», в том числе на таблице машин, хотя
     * внутри оно и так считает «Заехало/Выехало» для одних и «Зашло/Вышло» для
     * других, а в заявке система говорит «Посты проезда» и «Места прохода».
     */
    reportTitle() {
      const kind = this.tableType === 'cars'
        ? 'проездам'
        : this.tableType === 'people' ? 'проходам' : 'проездам и проходам';
      return `Отчёт по ${kind} — ${this.tableDisplayName}`;
    },
    /**
     * Дата берётся московская, а не по часам машины (#2298): отчётные сутки
     * закрываются в 21:30 МСК и на машине восточнее «сегодня» окна уже назвалось бы
     * завтрашним числом - охранник читал бы свою смену как чужую.
     */
    liveTitle() {
      if (!this.live) return 'Сегодня';
      const d = new Date(this.live.period_end);
      if (Number.isNaN(d.getTime())) return 'Сегодня';
      const m = moscowParts(d);
      return `Сегодня, ${m.day} ${MONTHS[m.month - 1]}`;
    },
    // Текущие отчётные сутки пусты по всем активным секциям - основание показать
    // подсказку про границу в 21:30 (кейс «открыл сразу после 21:30»).
    liveIsEmpty() {
      const t = this.live?.totals;
      if (!t) return false;
      return this.sectionDefs.every((s) => !(t[s.inField] || 0) && !(t[s.outField] || 0));
    },
  },
  watch: {
    show(value) {
      if (value) this.reload();
    },
    // Кнопка отчёта гейтится роутом и кликабельна ДО прихода tableData - модалка
    // может открыться с tableId=null (loadLive/loadDays уходят в ранний return).
    // Дозагружаем, когда id приехал; в один тик с show он не меняется (tableData
    // ставится асинхронным fetch задолго до/после клика), двойного вызова нет.
    tableId(value) {
      if (value && this.show) this.reload();
    },
  },
  methods: {
    anyCount(field) {
      const inTotals = (this.live?.totals?.[field] || 0) > 0;
      return inTotals || this.days.some((d) => (d.totals?.[field] || 0) > 0);
    },
    liveCount(field) {
      return this.live?.totals?.[field] ?? 0;
    },
    userLabel(row) {
      return row.user_name || (row.user_id ? `Пользователь #${row.user_id}` : 'Без автора');
    },
    // Человекочитаемая сводка счётчиков: «Машины: заехало 5, выехало 4».
    rowSummary(counts) {
      return this.sectionDefs
        .map((s) => `${s.title}: ${s.inLabel.toLowerCase()} ${counts?.[s.inField] || 0}, ${s.outLabel.toLowerCase()} ${counts?.[s.outField] || 0}`)
        .join(' · ');
    },
    formatDayHuman(dateStr) {
      const [y, m, d] = dateStr.split('-').map(Number);
      return `${d} ${MONTHS[m - 1]} ${y}`;
    },
    // Для имени файла экспорта (ASCII, без месяцев словами).
    formatDay(dateStr) {
      const [y, m, d] = dateStr.split('-');
      return `${d}.${m}.${y}`;
    },
    toggleHistory() {
      this.historyOpen = !this.historyOpen;
    },
    reload() {
      this.loadLive();
      this.loadDays();
    },
    async loadLive() {
      if (!this.tableId) return;
      this.loadingLive = true;
      try {
        this.live = await getPassReportLive(this.tableId);
      } catch (e) {
        this.live = null;
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить отчёт: ', bold: e.message || 'ошибка сервера', type: 'error' });
      } finally {
        this.loadingLive = false;
      }
    },
    async loadDays() {
      if (!this.tableId) return;
      this.loadingDays = true;
      try {
        const params = {};
        if (this.dateRangeStart) params.from = ymd(this.dateRangeStart);
        if (this.dateRangeEnd) params.to = ymd(this.dateRangeEnd);
        const data = await listPassReports(this.tableId, params);
        this.days = data.days || [];
      } catch (e) {
        this.days = [];
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить историю: ', bold: e.message || 'ошибка сервера', type: 'error' });
      } finally {
        this.loadingDays = false;
      }
    },
    clearDates() {
      this.dateRangeStart = null;
      this.dateRangeEnd = null;
      this.loadDays();
    },
    async exportToExcel() {
      if (!this.days.length || this.isExporting) return;
      this.isExporting = true;
      try {
        const workbook = new ExcelJS.Workbook();
        const worksheet = workbook.addWorksheet('Otchet_po_prohodam');

        const headers = ['Дата', 'Охранник', 'Машины: заехало', 'Машины: выехало', 'Люди: зашло', 'Люди: вышло'];
        const headerRow = worksheet.addRow(headers);
        headerRow.height = 25;
        headerRow.eachCell((cell) => {
          cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF4F5BDF' } };
          cell.font = { name: 'Verdana', size: 11, bold: true, color: { argb: 'FFFFFFFF' } };
          cell.alignment = { vertical: 'middle', horizontal: 'center' };
        });

        let stripe = 0;
        const addDataRow = (values, bold = false) => {
          const row = worksheet.addRow(values);
          row.height = 20;
          const fillColor = stripe % 2 === 0 ? 'FFF0F5FF' : 'FFE0E9FF';
          stripe += 1;
          row.eachCell((cell) => {
            cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: fillColor } };
            cell.font = { name: 'Verdana', size: 9, bold, color: { argb: 'FF333333' } };
            cell.alignment = { vertical: 'middle' };
          });
        };

        this.days.forEach((day) => {
          (day.rows || []).forEach((row) => {
            addDataRow([this.formatDay(day.report_date), this.userLabel(row), row.car_entries, row.car_exits, row.people_entries, row.people_exits]);
          });
          addDataRow([this.formatDay(day.report_date), 'Итого по посту', day.totals.car_entries, day.totals.car_exits, day.totals.people_entries, day.totals.people_exits], true);
        });

        worksheet.addRow([]);
        // Штамп выгрузки - московский и по серверным часам: файл уходит из бюро
        // наружу, и время в нём должно совпадать с временем отметок внутри отчёта.
        const stamp = formatDateTime(serverNow());
        const infoRow1 = worksheet.addRow(['Отчёт сформировал:', (this.currentUserName || '').trim() || 'Пользователь']);
        const infoRow2 = worksheet.addRow(['Дата формирования:', stamp]);
        [infoRow1, infoRow2].forEach((row) => {
          row.eachCell((cell) => {
            cell.font = { name: 'Verdana', size: 10, color: { argb: 'FF333333' } };
          });
        });
        worksheet.columns = [{ width: 14 }, { width: 34 }, { width: 16 }, { width: 16 }, { width: 14 }, { width: 14 }];

        const buffer = await workbook.xlsx.writeBuffer();
        const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.download = `Otchet_po_prohodam_${stamp.replace(/[.:\s]/g, '-')}.xlsx`;
        a.href = url;
        a.click();
        window.URL.revokeObjectURL(url);
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось выгрузить отчёт: ', bold: e.message || 'ошибка', type: 'error' });
      } finally {
        this.isExporting = false;
      }
    },
  },
};
</script>

<style scoped>
/* base-modal__body имеет padding:0 - горизонтальный отступ несёт секция. */
.pr-section {
  padding: 10px 22px 18px;
}

.pr-section--history {
  border-top: 1px solid var(--border);
  padding-top: 14px;
}

/* Заголовок «Сегодня» - крупный, для пожилого пользователя. */
.pr-today {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.pr-today__title {
  font-size: 22px;
  font-weight: 700;
  color: var(--text);
}

.pr-today__hint {
  font-size: 14px;
  color: var(--text-muted);
  margin-top: 2px;
}

.pr-cards {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

.pr-card {
  flex: 1 1 260px;
  min-width: 0;
  border: 1px solid var(--border);
  border-radius: 20px;
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.pr-card__title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 19px;
  font-weight: 700;
  color: var(--text);
}

.pr-card__icon {
  width: 26px;
  height: 26px;
  color: var(--text);
}

.pr-card__stat {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-radius: 14px;
}

/* Гамма сайта (primary #4f5bdf): «заехало» - мягкий синий с акцентным числом,
   «выехало» - нейтральный серо-синий. Без зелёного. */
.pr-card__stat--in {
  background: var(--accent-tint);
}

.pr-card__stat--out {
  background: var(--surface-2);
}

.pr-card__label {
  font-size: 18px;
  color: var(--text);
}

.pr-card__num {
  font-size: 44px;
  font-weight: 800;
  line-height: 1;
  color: var(--text);
}

.pr-card__stat--in .pr-card__num {
  color: var(--accent-text);
}

.pr-hint {
  margin-top: 16px;
  padding: 12px 16px;
  background: var(--accent-tint);
  border: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  border-left: 4px solid var(--accent);
  border-radius: 14px;
  font-size: 15px;
  line-height: 1.5;
  color: var(--text);
}

.pr-breakdown {
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px dashed var(--border);
}

.pr-breakdown__title {
  font-size: 14px;
  color: var(--text-muted);
  margin-bottom: 8px;
}

.pr-breakdown__row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 4px 0;
  font-size: 15px;
}

.pr-breakdown__name {
  font-weight: 600;
  color: var(--text);
}

.pr-breakdown__nums {
  color: var(--text);
}

.pr-empty {
  font-size: 16px;
  color: var(--text-muted);
  padding: 12px 0;
  text-align: center;
}

/* Кнопка-аккордеон истории - крупная, во всю ширину, как «ссылка-действие». */
.pr-history-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  width: 100%;
  background: var(--accent-tint);
  border: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  border-radius: 999px;
  padding: 12px 18px;
  font-size: 16px;
  font-weight: 600;
  color: var(--accent-text);
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.pr-history-toggle:hover {
  background: color-mix(in srgb, var(--accent) 18%, var(--surface));
}

.pr-history-toggle__arrow {
  font-size: 14px;
  transition: transform 0.2s ease;
}

.pr-history-toggle__arrow.open {
  transform: rotate(180deg);
}

/* Аккордеон: высота анимируется через grid-template-rows 0fr<->1fr, внутренний
   контейнер с min-height:0 + overflow:hidden обрезает содержимое при свёртке. */
.pr-history-wrap {
  display: grid;
  grid-template-rows: 0fr;
  transition: grid-template-rows 0.28s ease;
}

.pr-history-wrap.open {
  grid-template-rows: 1fr;
}

.pr-history-inner {
  min-height: 0;
  overflow: hidden;
}

/* «Прошлые дни» - отдельный обведённый блок под кнопкой. */
.pr-history {
  margin-top: 12px;
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 16px;
  background: var(--accent-tint);
}

.pr-history__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.pr-history__title {
  font-size: 16px;
  font-weight: 700;
  color: var(--text);
}

.pr-history__filter {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.pr-history__filter-label {
  font-size: 15px;
  color: var(--text);
}

@media (prefers-reduced-motion: reduce) {
  .pr-history-wrap {
    transition: none;
  }

  .pr-history-toggle__arrow {
    transition: none;
  }
}

.pr-days {
  display: flex;
  flex-direction: column;
}

.pr-day {
  padding: 12px 0;
}

.pr-day:not(:last-child) {
  border-bottom: 1px solid var(--border);
}

.pr-day__date {
  font-size: 16px;
  font-weight: 700;
  color: var(--text);
  margin-bottom: 4px;
}

.pr-day__total {
  font-size: 15px;
  color: var(--text);
}

.pr-day__people {
  margin-top: 6px;
  padding-left: 14px;
}

.pr-day__person {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
  padding: 2px 0;
  font-size: 14px;
  color: var(--text-muted);
}

.pr-day__person-name {
  font-weight: 600;
}

.pr-export {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-top: 14px;
  border: 1px solid var(--border);
  background: var(--surface);
  border-radius: 999px;
  padding: 10px 18px;
  font-size: 15px;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.pr-export:hover:not(:disabled) {
  background: var(--accent-tint);
}

.pr-export:disabled {
  opacity: 0.5;
  cursor: default;
}

.pr-export__icon {
  width: 16px;
  height: 16px;
}

@media (max-width: 560px) {
  .pr-card__num {
    font-size: 38px;
  }

  .pr-cards {
    gap: 12px;
  }
}
</style>
