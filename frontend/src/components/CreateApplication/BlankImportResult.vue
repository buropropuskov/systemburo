<template>
  <div
    class="bim"
    data-testid="blank-import-result"
  >
    <div class="bim__counters">
      <div
        v-if="hasResult"
        class="bim__counter"
      >
        <span class="bim__counter-value">{{ summary.read || 0 }}</span>
        <span class="bim__counter-label">прочитано строк</span>
      </div>
      <!-- Счётчик берём из списка, а не из разбора: удалённая справа строка обязана
           уйти и отсюда, иначе кнопка обещает добавить то, чего уже нет. -->
      <div class="bim__counter bim__counter--ok">
        <span
          class="bim__counter-value"
          data-testid="bim-pending-count"
        >{{ pendingCount }}</span>
        <span class="bim__counter-label">готово к добавлению</span>
      </div>
      <div
        v-if="hasResult && summary.rejected"
        class="bim__counter bim__counter--error"
      >
        <span class="bim__counter-value">{{ summary.rejected || 0 }}</span>
        <span class="bim__counter-label">с ошибками</span>
      </div>
    </div>

    <p
      v-if="!hasResult"
      class="bim__pending-hint"
    >
      Строки прошлого бланка ждут в списке серыми. Выберите места и нажмите «Добавить» -
      или удалите их прямо в списке.
    </p>

    <!-- Мест прохода/разгрузки/проезда в файле нет и не будет (решение владельца,
         blank-import) - применяются ко ВСЕМ импортируемым строкам целиком здесь. -->
    <div
      v-if="showTargetTables"
      class="bim__places"
      data-testid="bim-target-tables"
    >
      <label class="input__label">Места прохода <span
        v-if="targetTablesRequired"
        class="required"
      >*</span></label>
      <div
        v-if="!targetTablesOptions.length"
        class="bim__places-empty"
      >
        Нет доступных мест прохода
      </div>
      <TargetTablesGrid
        v-else
        v-model="selectedTargetTables"
        :tables="targetTablesOptions"
      />
    </div>

    <div
      v-if="showUnloadPlaces"
      class="bim__places"
      data-testid="bim-unload-places"
    >
      <label class="input__label">Места разгрузки <span
        v-if="unloadPlacesRequired"
        class="required"
      >*</span></label>
      <div
        v-if="!unloadPlacesOptions.length"
        class="bim__places-empty"
      >
        Нет доступных мест разгрузки
      </div>
      <TargetTablesGrid
        v-else
        v-model="selectedUnloadPlaces"
        :tables="unloadPlacesOptions"
      />
    </div>

    <div
      v-if="showPassageTables"
      class="bim__places"
      data-testid="bim-passage-tables"
    >
      <label class="input__label">Проезд <span
        v-if="passageTablesRequired"
        class="required"
      >*</span></label>
      <div
        v-if="!passageTablesOptions.length"
        class="bim__places-empty"
      >
        Нет доступных мест проезда
      </div>
      <TargetTablesGrid
        v-else
        v-model="selectedPassageTables"
        :tables="passageTablesOptions"
      />
    </div>

    <!-- Строки, которые система поправила сама (раскладка в номере, омоглифы в ФИО,
         дополнение номера нулями). Принимаются без вмешательства, но человек должен
         видеть, что именно изменилось: молчаливая правка данных - худший исход. -->
    <div
      v-if="warningRows.length"
      class="bim__warnings"
    >
      <h4 class="bim__problems-title">
        Система поправила
      </h4>
      <ul class="bim__warnings-list">
        <li
          v-for="row in warningRows"
          :key="`warn-${row.row_number}`"
          class="bim__warnings-item"
        >
          <span class="bim__warnings-row">Стр. {{ row.row_number }}</span>
          <span>{{ row.warnings.join('; ') }}</span>
        </li>
      </ul>
    </div>

    <div
      v-if="problemRows.length"
      class="bim__problems"
    >
      <h4 class="bim__problems-title">
        Строки с ошибками
      </h4>
      <p class="bim__problems-hint">
        Правьте поля прямо здесь и отмечайте строку галочкой, чтобы добавить её вместе с
        остальными. Галочка доступна только для исправимых здесь причин (пустые поля,
        гражданство). Чёрный список, дубли внутри файла и данные, которых в этой таблице
        нет (паспорт, патент), — строку нужно добавить через обычную форму.
      </p>
      <div class="bim__problems-table-wrap">
        <table class="bim__problems-table">
          <thead>
            <tr>
              <th>Стр.</th>
              <th v-if="isPeople">
                Фамилия
              </th>
              <th v-if="isPeople">
                Имя
              </th>
              <th v-if="isPeople">
                Отчество
              </th>
              <th v-if="isPeople">
                Гражданство
              </th>
              <th v-if="!isPeople">
                Номер Т/С
              </th>
              <th v-if="!isPeople">
                Марка
              </th>
              <th>Причина</th>
              <th>Добавить</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in problemRows"
              :key="row.rowNumber"
              :data-testid="`bim-problem-row-${row.rowNumber}`"
            >
              <td>{{ row.rowNumber }}</td>
              <template v-if="isPeople">
                <td>
                  <input
                    v-model="row.fields.lastName"
                    type="text"
                    class="lk-input bim__cell-input"
                  >
                </td>
                <td>
                  <input
                    v-model="row.fields.firstName"
                    type="text"
                    class="lk-input bim__cell-input"
                  >
                </td>
                <td>
                  <input
                    v-model="row.fields.middleName"
                    type="text"
                    class="lk-input bim__cell-input"
                  >
                </td>
                <td>
                  <select
                    v-model="row.fields.citizenshipId"
                    class="lk-select bim__cell-input"
                  >
                    <option :value="null">
                      Не выбрано
                    </option>
                    <option
                      v-for="c in citizenships"
                      :key="c.id"
                      :value="c.id"
                    >
                      {{ c.name }}
                    </option>
                  </select>
                </td>
              </template>
              <template v-else>
                <td>
                  <input
                    v-model="row.fields.plateNumber"
                    type="text"
                    class="lk-input bim__cell-input"
                  >
                </td>
                <td>
                  <input
                    v-model="row.fields.mark"
                    type="text"
                    class="lk-input bim__cell-input"
                  >
                </td>
              </template>
              <td class="bim__reason-cell">
                {{ row.errors.join('; ') }}
              </td>
              <td class="bim__include-cell">
                <input
                  v-model="row.included"
                  type="checkbox"
                  :disabled="!canIncludeRow(row)"
                  :data-testid="`bim-include-${row.rowNumber}`"
                >
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="bim__actions">
      <button
        v-if="problemRows.length"
        type="button"
        class="lk-button lk-button--ghost"
        data-testid="bim-download-errors"
        @click="downloadErrors"
      >
        Скачать список ошибок
      </button>
      <button
        type="button"
        class="lk-button lk-button--ghost"
        data-testid="bim-reset"
        @click="$emit('reset')"
      >
        Загрузить другой файл
      </button>
      <button
        type="button"
        class="lk-button lk-button--primary"
        data-testid="bim-submit"
        :disabled="!canSubmit"
        @click="onSubmit"
      >
        Добавить в заявку ({{ addableCount }})
      </button>
    </div>
  </div>
</template>

<script>
import TargetTablesGrid from '@/components/CreateApplication/TargetTablesGrid.vue';
import { listCitizenships } from '@/api/citizenships';
import { useDeletionsStore } from '@/stores/deletions';
import { useFieldConfig } from '@/composables/useFieldConfig';
import { employeeLabel, vehicleLabel } from '@/utils/applicationDuplicates';

// Метки полей реестра (attachment_fields_registry.go), которые эта таблица умеет
// править - ФИКСИРОВАННЫЕ строки (Label не переопределяется оверрайдами шаблона,
// см. MergeFieldConfig), поэтому сверка префиксом "Поле «<Label>»" надёжна. Любая
// причина вне этого списка (паспорт, патент, должность, чёрный список, дубль
// внутри файла) остаётся блокирующей - полей для них здесь нет и не должно быть
// (152-ФЗ), а чёрный список/дубли - решение, которое клиент не переигрывает.
const PEOPLE_FIXABLE_FIELD_LABELS = ['Фамилия', 'Имя', 'Отчество', 'Гражданство'];
const CAR_FIXABLE_FIELD_LABELS = ['Номер ТС', 'Марка ТС'];

/**
 * Сводка разбора заполненного бланка (эпик blank-import, срез D1D2; срез U4 перенёс
 * её из модалки в панель режима импорта - BlankImportPanel.vue). Принятые бэком
 * строки (без errors) уходят в заявку тем же путём, что ручное добавление -
 * родитель вызывает handleEmployeesAdded/handleVehiclesAdded с собранным здесь
 * массивом, без своей логики создания строк списка (см. CreateApplication.vue).
 *
 * Места прохода/разгрузки/проезда бэк не читает из файла (решение владельца:
 * задаются на сайте) - выбор здесь обязателен и применяется ко ВСЕМ строкам разом.
 *
 * Строки с ошибками показывают только ФИО/номер и причину - паспорт и патент не
 * выводятся и не редактируются здесь (152-ФЗ, доб. поля правятся в обычной форме).
 * "Добавить" по исправленной строке не перепроверяет причину отказа повторно
 * (чёрный список, дубли) - финальная подача делает это как для любой ручной строки.
 */
export default {
  name: 'BlankImportResult',
  components: { TargetTablesGrid },
  props: {
    attachmentType: {
      type: String,
      default: 'people',
      validator: (v) => ['people', 'cars'].includes(v),
    },
    summary: {
      type: Object,
      default: () => ({ read: 0, accepted: 0, rejected: 0 }),
    },
    rows: { type: Array, default: () => [] },
    // Есть ли свежий разбор файла. Сводка открывается и по одним предварительным
    // строкам (после перезагрузки страницы), и тогда счётчики разбора врали бы нулями.
    hasResult: { type: Boolean, default: true },
    // Сколько строк этого вложения сейчас лежит в списке предварительными.
    pendingCount: { type: Number, default: 0 },
    // Сырые списки CreateApplication (/system-tables, /unload-places) - те же, что
    // питают EmployeesList/VehicleForm, переформатируются в {table:{...}} для
    // TargetTablesGrid по образцу TableBulkTargetModal.availableTables.
    allPassageTables: { type: Array, default: () => [] },
    allUnloadingPlaces: { type: Array, default: () => [] },
    fieldConfig: { type: Object, default: () => ({}) },
  },
  emits: ['reset', 'import', 'stage'],
  data() {
    return {
      citizenships: [],
      selectedTargetTables: [],
      selectedUnloadPlaces: [],
      selectedPassageTables: [],
      problemRows: [],
    };
  },
  computed: {
    isPeople() {
      return this.attachmentType === 'people';
    },
    acceptedRows() {
      return this.rows.filter((r) => !(r.errors && r.errors.length));
    },
    // Строки с предупреждениями показываем отдельно: у отклонённых причина и так видна
    // в таблице ошибок, а вот принятая строка с исправленным значением иначе уходит
    // в заявку молча.
    warningRows() {
      return this.rows.filter((r) => r.warnings && r.warnings.length
        && !(r.errors && r.errors.length));
    },
    showTargetTables() {
      return this.isPeople && this.fieldVisible('target_tables');
    },
    showUnloadPlaces() {
      return !this.isPeople && this.fieldVisible('unloading_places');
    },
    showPassageTables() {
      return !this.isPeople && this.fieldVisible('passage_tables');
    },
    // Обязательность выбора - ЗЕРКАЛО ручных форм (EmployeeForm.vue:491-492,
    // VehicleForm.vue:573-574): "видимо И required", а не одно только "видимо".
    // Видимое необязательное поле грид всё равно рисует (можно выбрать по желанию),
    // но submit им не блокируется.
    targetTablesRequired() {
      return this.showTargetTables && this.fieldRequired('target_tables');
    },
    unloadPlacesRequired() {
      return this.showUnloadPlaces && this.fieldRequired('unloading_places');
    },
    passageTablesRequired() {
      return this.showPassageTables && this.fieldRequired('passage_tables');
    },
    targetTablesOptions() {
      return this.reshapeTables(this.allPassageTables, 'people');
    },
    passageTablesOptions() {
      return this.reshapeTables(this.allPassageTables, 'cars');
    },
    unloadPlacesOptions() {
      return (this.allUnloadingPlaces || []).map((p) => ({
        table: {
          id: p.id,
          display_name: p.name,
          status: p.status || 'active',
          status_comment: p.status_comment,
        },
      }));
    },
    placesReady() {
      if (this.isPeople) {
        return !this.targetTablesRequired || this.selectedTargetTables.length > 0;
      }
      const unloadOk = !this.unloadPlacesRequired || this.selectedUnloadPlaces.length > 0;
      const passageOk = !this.passageTablesRequired || this.selectedPassageTables.length > 0;
      return unloadOk && passageOk;
    },
    includedFixedRows() {
      return this.problemRows.filter((r) => r.included && this.canIncludeRow(r));
    },
    // Принятые строки уже стоят в списке предварительными, поэтому считаем их оттуда:
    // сколько человек оставил, столько и добавится.
    addableCount() {
      return this.pendingCount + this.includedFixedRows.length;
    },
    canSubmit() {
      return this.placesReady && this.addableCount > 0;
    },
    // Места в файле не приходят и раскатываются на всю пачку разом - патч полей строки
    // собираем здесь, применяет его родитель ко всем предварительным строкам.
    placesPatch() {
      if (this.isPeople) {
        return {
          targetTables: [...this.selectedTargetTables],
          passageTables: this.formatSelectedNames(this.selectedTargetTables, this.targetTablesOptions),
        };
      }
      return {
        unloadPlaces: [...this.selectedUnloadPlaces],
        unloadingPlace: this.formatSelectedNames(this.selectedUnloadPlaces, this.unloadPlacesOptions),
        passage_tables: [...this.selectedPassageTables],
      };
    },
  },
  watch: {
    rows: {
      // Панель живёт ровно столько, сколько есть разобранный файл, и монтируется уже
      // с данными - immediate обязателен. Новая загрузка отдаёт новый массив строк:
      // выбор мест и правки прошлого файла на неё не переносятся.
      immediate: true,
      handler() {
        this.resetState();
      },
    },
  },
  methods: {
    fieldVisible(key) {
      return useFieldConfig(() => this.fieldConfig).fieldVisible(key);
    },
    fieldRequired(key) {
      return useFieldConfig(() => this.fieldConfig).fieldRequired(key);
    },
    async resetState() {
      this.selectedTargetTables = [];
      this.selectedUnloadPlaces = [];
      this.selectedPassageTables = [];
      this.problemRows = this.rows
        .filter((r) => r.errors && r.errors.length)
        .map((r) => ({
          rowNumber: r.row_number,
          errors: r.errors || [],
          included: false,
          original: r,
          fields: this.isPeople
            ? {
              lastName: (r.employee && r.employee.last_name) || '',
              firstName: (r.employee && r.employee.first_name) || '',
              middleName: (r.employee && r.employee.middle_name) || '',
              citizenshipId: (r.employee && r.employee.citizenship_id) || null,
            }
            : {
              plateNumber: (r.vehicle && r.vehicle.car_number) || '',
              mark: (r.vehicle && r.vehicle.car_brand) || '',
            },
        }));
      // Справочник ждём ДО передачи строк наверх: citizenshipName собирается по нему,
      // и без него в списке и в карточке строки гражданство осталось бы пустым навсегда.
      if (this.isPeople && !this.citizenships.length) {
        await this.loadCitizenships();
      }
      this.stageAcceptedRows();
    },

    // Принятые бэком строки уходят в список сразу после разбора - предварительными.
    // Места к ним приезжают позже, на «Добавить» (в файле их нет и не будет).
    stageAcceptedRows() {
      if (!this.acceptedRows.length) return;
      const build = this.isPeople
        ? (row) => this.buildEmployeeFromRow(row, false)
        : (row) => this.buildVehicleFromRow(row, false);
      this.$emit('stage', {
        attachmentType: this.attachmentType,
        rows: this.acceptedRows.map((row) => build(row)),
      });
    },
    async loadCitizenships() {
      try {
        this.citizenships = await listCitizenships();
      } catch (error) {
        // Список гражданств нужен только для правки проблемных строк - пустая
        // заявка на импорт не должна падать из-за недоступности справочника.
        console.error('Ошибка при загрузке гражданств:', error);
        this.citizenships = [];
      }
    },
    reshapeTables(list, tableType) {
      return (list || [])
        .map((t) => t.table || t)
        .filter((tbl) => tbl.table_type === tableType)
        .map((tbl) => ({
          table: {
            id: tbl.id,
            display_name: tbl.display_name || tbl.name,
            status: tbl.status || 'active',
            status_comment: tbl.status_comment,
          },
        }));
    },
    citizenshipName(id) {
      const c = this.citizenships.find((x) => x.id === id);
      return c ? c.name : '';
    },
    formatSelectedNames(ids, options) {
      const names = ids
        .map((id) => {
          const found = options.find((o) => o.table.id === id);
          return found ? found.table.display_name : '';
        })
        .filter(Boolean);
      if (names.length > 1) return `${names[0]} и др.`;
      return names[0] || '';
    },
    // Причина ошибки "исправима здесь", только если это пустое/слишком длинное
    // значение поля, которое эта таблица реально редактирует, либо неизвестное
    // гражданство. Всё остальное (чёрный список, дубль внутри файла, паспорт,
    // патент, должность) - НЕ матчится и остаётся блокирующим: подстрока проверяет
    // ФИКСИРОВАННЫЙ префикс "Поле «<Label>»", где Label берётся из Go-реестра
    // (attachment_fields_registry.go) и не переопределяется оверрайдами шаблона.
    errorIsFixable(text) {
      const labels = this.isPeople ? PEOPLE_FIXABLE_FIELD_LABELS : CAR_FIXABLE_FIELD_LABELS;
      if (labels.some((label) => text.startsWith(`Поле «${label}»`))) return true;
      if (this.isPeople && text.includes('не найдено в справочнике')) return true;
      return false;
    },
    // Строка становится добавляемой, только когда ВСЕ её причины исправимы здесь
    // И минимальные поля реально заполнены - блокирующие причины (ЧС, дубль,
    // паспорт/патент - полей для них тут нет по 152-ФЗ) чекбокс не разблокируют
    // никакой правкой ФИО/номера.
    canIncludeRow(row) {
      if (!row.errors.every((e) => this.errorIsFixable(e))) return false;
      if (this.isPeople) {
        if (!row.fields.lastName.trim() || !row.fields.firstName.trim()) return false;
        // Гражданство признано исправимым здесь, значит и требовать его надо так же,
        // как форма: иначе строку с неопознанным гражданством можно включить, ничего
        // не выбрав, и она отобьётся уже на подаче.
        if (this.fieldVisible('citizenship') && this.fieldRequired('citizenship')) {
          return !!row.fields.citizenshipId;
        }
        return true;
      }
      return !!row.fields.plateNumber.trim();
    },
    buildEmployeeFromRow(row, isFixed) {
      const emp = row.original ? row.original.employee : row.employee;
      const fields = isFixed
        ? row.fields
        : {
          lastName: emp.last_name,
          firstName: emp.first_name,
          middleName: emp.middle_name || '',
          citizenshipId: emp.citizenship_id,
        };
      return {
        lastName: (fields.lastName || '').trim(),
        firstName: (fields.firstName || '').trim(),
        middleName: (fields.middleName || '').trim(),
        position: emp.position || '',
        citizenshipId: fields.citizenshipId,
        citizenshipName: this.citizenshipName(fields.citizenshipId),
        passportSeriesNumber: emp.passport_series_number || '',
        patentNumber: emp.patent_number || null,
        otherPermission: emp.other_permission || null,
        passageTables: this.formatSelectedNames(this.selectedTargetTables, this.targetTablesOptions),
        targetTables: [...this.selectedTargetTables],
        isExisting: false,
      };
    },
    buildVehicleFromRow(row, isFixed) {
      const veh = row.original ? row.original.vehicle : row.vehicle;
      const fields = isFixed
        ? row.fields
        : { plateNumber: veh.car_number, mark: veh.car_brand };
      const mark = (fields.mark || '').trim() || null;
      return {
        plateNumber: (fields.plateNumber || '').trim(),
        mark,
        markId: null,
        markName: mark,
        unloadingPlace: this.formatSelectedNames(this.selectedUnloadPlaces, this.unloadPlacesOptions),
        unloadPlaces: [...this.selectedUnloadPlaces],
        passage_tables: [...this.selectedPassageTables],
        formatId: null,
        isExisting: false,
      };
    },
    // «Добавить»: принятые строки уже в списке (их родитель только раскатывает местами и
    // делает обычными), а вручную исправленные проблемные строки заводятся этим событием.
    onSubmit() {
      if (!this.canSubmit) return;
      const build = this.isPeople
        ? (row) => this.buildEmployeeFromRow(row, true)
        : (row) => this.buildVehicleFromRow(row, true);
      this.$emit('import', {
        attachmentType: this.attachmentType,
        places: this.placesPatch,
        rows: this.includedFixedRows.map((row) => build(row)),
      });
    },
    async downloadErrors() {
      try {
        const ExcelJS = (await import('exceljs')).default;
        const workbook = new ExcelJS.Workbook();
        const worksheet = workbook.addWorksheet('Ошибки импорта');
        const headers = ['Строка', this.isPeople ? 'ФИО' : 'Номер Т/С', 'Причина'];
        const headerRow = worksheet.addRow(headers);
        headerRow.eachCell((cell) => {
          cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FF4F5BDF' } };
          cell.font = { bold: true, color: { argb: 'FFFFFFFF' } };
        });
        this.rows
          .filter((r) => r.errors && r.errors.length)
          .forEach((r) => {
            const label = this.isPeople
              ? employeeLabel({
                lastName: r.employee && r.employee.last_name,
                firstName: r.employee && r.employee.first_name,
                middleName: r.employee && r.employee.middle_name,
              })
              : vehicleLabel({
                plateNumber: r.vehicle && r.vehicle.car_number,
                mark: r.vehicle && r.vehicle.car_brand,
              });
            worksheet.addRow([r.row_number, label, r.errors.join('; ')]);
          });
        worksheet.columns = [{ width: 10 }, { width: 30 }, { width: 70 }];
        const buffer = await workbook.xlsx.writeBuffer();
        const blob = new Blob([buffer], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
        const { saveBlobAs } = await import('@/api/attachment-templates');
        saveBlobAs(blob, 'oshibki_importa.xlsx');
      } catch (error) {
        console.error('Ошибка при выгрузке списка ошибок:', error);
        useDeletionsStore().notify({ bold: 'Не удалось сформировать список ошибок', type: 'error' });
      }
    },
  },
};
</script>

<style scoped>
.bim {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.bim__actions {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 10px;
}

.bim__counters {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;
}

.bim__counter {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.bim__counter-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1;
}

.bim__counter--ok .bim__counter-value {
  color: var(--color-success, #15803d);
}

.bim__counter--error .bim__counter-value {
  color: var(--danger-text);
}

.bim__counter-label {
  font-size: 12px;
  color: var(--text-muted);
}

.bim__places-empty {
  font-size: 13px;
  color: var(--text-muted);
}

.bim__pending-hint {
  margin: 0;
  font-size: 13px;
  color: var(--text-muted);
}

.input__label {
  font-size: 13px;
  color: var(--text-muted);
  display: block;
  margin-bottom: 4px;
}

.required {
  color: var(--danger-text);
}

.bim__problems-title {
  margin: 0 0 4px;
  font-size: 15px;
}

.bim__problems-hint {
  margin: 0 0 12px;
  font-size: 12px;
  color: var(--text-muted);
}

.bim__warnings {
  margin-bottom: 16px;
}

.bim__warnings-list {
  margin: 0;
  padding: 0;
  list-style: none;
  max-height: 160px;
  overflow: auto;
  font-size: 12px;
  color: var(--text-muted);
}

.bim__warnings-item {
  display: flex;
  gap: 8px;
  padding: 4px 0;
}

.bim__warnings-row {
  flex: 0 0 auto;
  color: var(--text-secondary);
}

.bim__problems-table-wrap {
  max-height: 320px;
  overflow: auto;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
}

.bim__problems-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.bim__problems-table th {
  position: sticky;
  top: 0;
  background: var(--surface-2);
  text-align: left;
  padding: 8px;
  border-bottom: 1px solid var(--color-border);
  white-space: nowrap;
}

.bim__problems-table td {
  padding: 6px 8px;
  border-bottom: 1px solid var(--color-border);
  vertical-align: middle;
}

.bim__cell-input {
  padding: 6px 8px;
  font-size: 12px;
  min-width: 90px;
}

.bim__reason-cell {
  color: var(--danger-text);
  max-width: 260px;
}

.bim__include-cell {
  text-align: center;
}

@media (max-width: 768px) {
  .bim__cell-input {
    min-width: 70px;
  }

  /* Тач-таргет 44px (WCAG 2.5.5): --sm-модификатора у этих кнопок нет, но базовые
     8px padding дают 30px высоты. */
  .bim__actions .lk-button {
    min-height: 44px;
    flex: 1 1 auto;
  }
}
</style>
