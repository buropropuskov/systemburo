<template>
  <BaseModal
    :show="show"
    :title="`Отчёт по проходам — ${tableDisplayName}`"
    width="760px"
    radius="30px"
    content-class="pass-report-modal"
    @close="$emit('close')"
  >
    <div
      class="pr-section"
      data-testid="pass-report-live"
    >
      <div class="pr-section__head">
        <span class="pr-period">{{ livePeriodLabel }}</span>
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
        <div class="pr-counters">
          <div
            v-if="showCarsSection"
            class="pr-counter"
          >
            <span class="pr-counter__title">Машины</span>
            <span class="pr-counter__pair">
              пропущено <b data-testid="pr-total-car-entries">{{ live.totals?.car_entries ?? 0 }}</b>
              · выпущено <b data-testid="pr-total-car-exits">{{ live.totals?.car_exits ?? 0 }}</b>
            </span>
          </div>
          <div
            v-if="showPeopleSection"
            class="pr-counter"
          >
            <span class="pr-counter__title">Люди</span>
            <span class="pr-counter__pair">
              пропущено <b data-testid="pr-total-people-entries">{{ live.totals?.people_entries ?? 0 }}</b>
              · выпущено <b data-testid="pr-total-people-exits">{{ live.totals?.people_exits ?? 0 }}</b>
            </span>
          </div>
        </div>

        <table
          v-if="live.rows && live.rows.length"
          class="pr-rows"
          data-testid="pass-report-rows"
        >
          <thead>
            <tr>
              <th>Охранник</th>
              <template v-if="showCarsSection">
                <th>Машины: въезд</th>
                <th>Машины: выезд</th>
              </template>
              <template v-if="showPeopleSection">
                <th>Люди: вход</th>
                <th>Люди: выход</th>
              </template>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in live.rows"
              :key="row.user_id"
            >
              <td>{{ userLabel(row) }}</td>
              <template v-if="showCarsSection">
                <td>{{ row.car_entries }}</td>
                <td>{{ row.car_exits }}</td>
              </template>
              <template v-if="showPeopleSection">
                <td>{{ row.people_entries }}</td>
                <td>{{ row.people_exits }}</td>
              </template>
            </tr>
          </tbody>
        </table>
        <div
          v-else
          class="pr-empty"
        >
          В текущем периоде отметок ещё нет
        </div>
      </template>
      <div
        v-else
        class="pr-empty"
      >
        Не удалось загрузить отчёт
      </div>
    </div>

    <div
      class="pr-section"
      data-testid="pass-report-history"
    >
      <div class="pr-section__head">
        <h4 class="pr-section__title">
          История по дням
        </h4>
        <div class="pr-section__tools">
          <DateFilter
            mode="range"
            :date-range-start="dateRangeStart"
            :date-range-end="dateRangeEnd"
            @update:date-range-start="dateRangeStart = $event"
            @update:date-range-end="dateRangeEnd = $event"
            @apply="loadDays"
            @clear="clearDates"
          />
          <button
            class="pr-export"
            :disabled="isExporting || !days.length"
            data-testid="pass-report-export"
            @click="exportToExcel"
          >
            <img
              v-if="!isExporting"
              src="@/assets/icons/export.png"
              class="pr-export__icon"
              alt=""
            >
            <span>{{ isExporting ? 'Формируем...' : 'Экспорт' }}</span>
          </button>
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
        Нет сохранённых отчётов за период
      </div>
      <div
        v-else
        class="pr-days"
      >
        <div
          v-for="day in days"
          :key="day.report_date"
          class="pr-day"
          data-testid="pass-report-day"
        >
          <div class="pr-day__head">
            <span class="pr-day__date">{{ formatDay(day.report_date) }}</span>
            <span class="pr-day__totals">
              <template v-if="showCarsSection">машины {{ day.totals.car_entries }} / {{ day.totals.car_exits }}</template>
              <template v-if="showCarsSection && showPeopleSection"> · </template>
              <template v-if="showPeopleSection">люди {{ day.totals.people_entries }} / {{ day.totals.people_exits }}</template>
            </span>
          </div>
          <div
            v-for="row in day.rows"
            :key="`${day.report_date}-${row.user_id}`"
            class="pr-day__row"
          >
            <span class="pr-day__user">{{ userLabel(row) }}</span>
            <span class="pr-day__counts">
              <template v-if="showCarsSection">машины {{ row.car_entries }} / {{ row.car_exits }}</template>
              <template v-if="showCarsSection && showPeopleSection"> · </template>
              <template v-if="showPeopleSection">люди {{ row.people_entries }} / {{ row.people_exits }}</template>
            </span>
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

// Дата -> YYYY-MM-DD по ЛОКАЛЬНЫМ частям: toISOString увёз бы выбранный день на
// предыдущий у пользователей восточнее UTC.
function ymd(d) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

export default {
  name: 'PassReportModal',
  components: { BaseModal, RefreshButton, DateFilter },
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
    };
  },
  computed: {
    // Секции по типу таблицы; «чужая» пара показывается, только если в данных
    // реально есть события (не фабрикуем нули там, где их не бывает).
    showCarsSection() {
      if (this.tableType === 'cars') return true;
      return this.anyCount('car_entries') || this.anyCount('car_exits');
    },
    showPeopleSection() {
      if (this.tableType === 'people') return true;
      return this.anyCount('people_entries') || this.anyCount('people_exits');
    },
    livePeriodLabel() {
      if (!this.live) return 'Текущий период';
      return `Текущий период: с ${this.formatMoment(this.live.period_start)} до ${this.formatMoment(this.live.period_end)}`;
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
    userLabel(row) {
      return row.user_name || (row.user_id ? `Пользователь #${row.user_id}` : 'Без автора');
    },
    formatMoment(iso) {
      const d = new Date(iso);
      if (Number.isNaN(d.getTime())) return '';
      const dd = `${String(d.getDate()).padStart(2, '0')}.${String(d.getMonth() + 1).padStart(2, '0')}`;
      const hm = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
      return `${dd} ${hm}`;
    },
    formatDay(dateStr) {
      const [y, m, d] = dateStr.split('-');
      return `${d}.${m}.${y}`;
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

        const headers = ['Дата', 'Охранник', 'Машины: въезд', 'Машины: выезд', 'Люди: вход', 'Люди: выход'];
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
          addDataRow([this.formatDay(day.report_date), 'Итого по таблице', day.totals.car_entries, day.totals.car_exits, day.totals.people_entries, day.totals.people_exits], true);
        });

        worksheet.addRow([]);
        const stamp = new Date().toLocaleString('ru-RU', {
          day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit',
        }).replace(',', '');
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
/* base-modal__body имеет padding:0 - горизонтальный отступ несёт секция,
   20px в тон паддингу шапки/футера BaseModal. */
.pr-section {
  padding: 6px 20px 14px;
}

.pr-section + .pr-section {
  border-top: 1px solid #e6e6e6;
  padding-top: 14px;
}

.pr-section__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 10px;
}

.pr-section__title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}

.pr-section__tools {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.pr-period {
  font-size: 13px;
  color: #555;
}

.pr-counters {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.pr-counter {
  flex: 1 1 220px;
  min-width: 0;
  border: 1px solid #e6e6e6;
  border-radius: var(--radius-md, 15px);
  padding: 10px 14px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.pr-counter__title {
  font-size: 12px;
  color: #777;
}

.pr-counter__pair {
  font-size: 14px;
}

.pr-counter__pair b {
  font-size: 18px;
}

.pr-rows {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.pr-rows th,
.pr-rows td {
  text-align: left;
  padding: 6px 8px;
  border-bottom: 1px solid #eee;
}

.pr-rows th {
  font-weight: 600;
  color: #777;
  font-size: 12px;
}

.pr-rows tr:last-child td {
  border-bottom: none;
}

.pr-empty {
  font-size: 13px;
  color: #888;
  padding: 8px 0;
}

.pr-days {
  display: flex;
  flex-direction: column;
}

.pr-day {
  padding: 8px 0;
}

.pr-day:not(:last-child) {
  border-bottom: 1px solid #eee;
}

.pr-day__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.pr-day__date {
  font-weight: 600;
  font-size: 13px;
}

.pr-day__totals {
  font-size: 13px;
  color: #333;
}

.pr-day__row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
  padding: 2px 0 2px 14px;
  font-size: 12px;
  color: #666;
}

.pr-export {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: 1px solid #e6e6e6;
  background: #fff;
  border-radius: 999px;
  padding: 7px 14px;
  font-size: 13px;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.pr-export:hover:not(:disabled) {
  background: #f5f6ff;
}

.pr-export:disabled {
  opacity: 0.5;
  cursor: default;
}

.pr-export__icon {
  width: 14px;
  height: 14px;
}
</style>
