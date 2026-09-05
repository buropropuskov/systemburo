<template>
  <div
    class="bim"
    data-testid="blank-import-result"
  >
    <!-- Сводка - отдельный блок, а не цифры на голом фоне панели. -->
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
      <!-- Тоже из живого состояния, а не из разбора: исправленная строка уходит из
           карточек в список, и снимок summary.rejected начинал спорить с соседним
           счётчиком - 15 прочитано при 14 готовых и 2 с ошибками. Заголовок блока
           ниже всегда считал карточки, поэтому расхождение было видно на одном экране. -->
      <div
        v-if="hasResult && problemRows.length"
        class="bim__counter bim__counter--error"
      >
        <span
          class="bim__counter-value"
          data-testid="bim-error-count"
        >{{ problemRows.length }}</span>
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

    <!-- Согласия субъекта в файле нет и не будет: колонки под него в бланке не заводили,
         поэтому отметка ставится здесь один раз на весь список - тем же порядком, что и
         места. Уходит патчем ко всем строкам пачки (placesPatch): у разобранных строк
         своей галочки нет, они уже лежат в списке предварительными. -->
    <div
      v-if="showPDConsent"
      class="bim__consent"
      data-testid="bim-pd-consent"
    >
      <label class="consent-option">
        <input
          v-model="pdConsent"
          type="checkbox"
          data-testid="bim-pd-consent-checkbox"
        >
        <span>
          {{ consentSubjectsLabel }} уведомлены об <a
            href="/data-processing"
            target="_blank"
            rel="noopener"
            class="blue"
            @click.stop
          >обработке персональных данных</a><span
            v-if="pdConsentRequired"
            class="required"
          >*</span>
        </span>
      </label>
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

    <!-- Строки с ошибками - не таблица, а карточки: причина отказа это фраза целиком,
         в узкой колонке она рвалась на три строки, а поля правки жались вплотную. -->
    <div
      v-if="problemRows.length"
      class="bim__problems"
    >
      <h4 class="bim__problems-title">
        Строки с ошибками
        <span class="bim__problems-count">{{ problemRows.length }}</span>
      </h4>
      <p class="bim__problems-hint">
        Поправьте поля прямо здесь и нажмите «Добавить» - строка уйдёт в список.
        Причину с пометкой «Только вручную» здесь не снять (чёрный список, дубль внутри
        файла, паспорт, патент, должность) - такую строку заводят обычной формой.
      </p>

      <ul class="bim__problems-list">
        <li
          v-for="row in problemRows"
          :key="row.rowNumber"
          class="bim__problem"
          :class="{ 'bim__problem--blocked': !rowFixable(row) }"
          :data-testid="`bim-problem-row-${row.rowNumber}`"
        >
          <div class="bim__problem-head">
            <span class="bim__problem-num">Строка {{ row.rowNumber }}</span>
            <Badge
              size="sm"
              :variant="rowFixable(row) ? 'warning' : 'danger'"
              :label="rowFixable(row) ? 'Можно исправить' : 'Только вручную'"
            />
            <button
              type="button"
              class="lk-button lk-button--secondary lk-button--sm bim__row-add"
              :disabled="!canIncludeRow(row)"
              :data-testid="`bim-include-${row.rowNumber}`"
              @click="addProblemRow(row)"
            >
              Добавить
            </button>
          </div>

          <ul class="bim__reasons">
            <li
              v-for="(error, index) in row.errors"
              :key="index"
              class="bim__reason"
              :class="{ 'bim__reason--blocking': !error.fixable }"
            >
              {{ error.text }}
            </li>
          </ul>

          <div class="bim__fields">
            <template v-if="isPeople">
              <label class="bim__field">
                <span class="bim__field-label">Фамилия</span>
                <input
                  v-model="row.fields.lastName"
                  type="text"
                  class="lk-input bim__cell-input"
                >
              </label>
              <label class="bim__field">
                <span class="bim__field-label">Имя</span>
                <input
                  v-model="row.fields.firstName"
                  type="text"
                  class="lk-input bim__cell-input"
                >
              </label>
              <label class="bim__field">
                <span class="bim__field-label">Отчество</span>
                <input
                  v-model="row.fields.middleName"
                  type="text"
                  class="lk-input bim__cell-input"
                >
              </label>
              <label class="bim__field">
                <span class="bim__field-label">Гражданство</span>
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
              </label>
            </template>
            <template v-else>
              <!-- Номер по ячейкам - тот же принцип, что в VehicleForm (посегментный ввод
                   с валидацией/доформатированием по cell), только компактнее: короткая
                   пилюля вместо отдельной строки поля, свой тумблер "по факту" рядом
                   с подписью. -->
              <div class="bim__field bim__field--plate">
                <div class="bim__plate-head">
                  <span class="bim__field-label">Номер Т/С</span>
                  <label class="bim__fact-toggle">
                    <input
                      v-model="row.fields.isByFact"
                      type="checkbox"
                      class="bim__fact-checkbox"
                      @change="handleRowFactChange(row)"
                    >
                    <span
                      class="bim__fact-switch"
                      aria-hidden="true"
                    />
                    <span class="bim__fact-text">по факту</span>
                  </label>
                </div>
                <div
                  v-if="row.fields.isByFact"
                  class="bim__plate-field bim__plate-field--fact"
                >
                  <input
                    class="bim__plate-fact-input"
                    value="По факту"
                    readonly
                  >
                </div>
                <div
                  v-else-if="rowPlateFormat(row)"
                  class="bim__plate-field"
                >
                  <input
                    v-for="(cell, index) in rowPlateFormat(row).cells"
                    :key="index"
                    :ref="(el) => setPlateCellRef(row.rowNumber, index, el)"
                    v-model="row.fields.numberParts[index]"
                    class="bim__plate-cell"
                    :placeholder="getPlaceholder(cell)"
                    :maxlength="cell.max_length"
                    :style="{ width: getInputWidth(cell) }"
                    @input="validatePlatePart(row, index, $event, cell)"
                    @blur="formatPlatePart(row, index, cell)"
                  >
                </div>
                <div
                  v-else
                  class="bim__plate-empty"
                >
                  Выберите формат номера
                </div>
              </div>
              <!-- Формат номера - явный выбор рядом с полем номера (доводка владельца).
                   Пункта "определить автоматически" здесь нет: ячейки ввода нельзя
                   нарисовать "вообще", они всегда принадлежат конкретному формату, и
                   подпись обязана называть тот же формат, по которому нарисованы ячейки.
                   Дропдаун проекта, а не нативный select: телепорт нужен ещё и по делу -
                   список карточек прокручиваемый (.bim__problems-list), меню внутри него
                   обрезалось бы по краю. Для людей выбора формата нет и не должно быть. -->
              <div class="bim__field">
                <span class="bim__field-label">Формат номера</span>
                <BaseDropdown
                  class="bim__format"
                  :model-value="row.fields.formatId"
                  :options="formatOptions"
                  :menu-min-width="220"
                  placeholder="Выберите формат"
                  teleport
                  :data-testid="`bim-format-${row.rowNumber}`"
                  @update:model-value="(id) => selectFormat(row, id)"
                />
              </div>
              <label class="bim__field">
                <span class="bim__field-label">Марка</span>
                <input
                  v-model="row.fields.mark"
                  type="text"
                  class="lk-input bim__cell-input"
                >
              </label>
            </template>
          </div>

          <!-- Место под подсказку зарезервировано ВСЕГДА для исправимой строки (высота
               не скачет по мере ввода - от этого дёргался вертикальный скролл списка и
               вместе с ним горизонтальный соседней колонки, см. .bim__problem-note ниже);
               видимость переключается классом, а не добавлением/удалением из разметки. -->
          <p
            v-if="rowFixable(row)"
            class="bim__problem-note"
            :class="{ 'bim__problem-note--hidden': canIncludeRow(row) }"
          >
            {{ rowNote(row) }}
          </p>
        </li>
      </ul>
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
      <!-- Подсказка висит на ОБЁРТКЕ: заблокированная кнопка событий мыши не получает,
           :hover на ней не наступает никогда. Причина нужна именно здесь - рядом стоят
           карточки с ошибками, и серую кнопку без объяснения человек связывает с ними,
           хотя ошибочные строки её не блокируют (мест в бланке нет, их выбирают тут). -->
      <span
        class="hint-anchor bim__submit-wrap"
        :data-hint="submitHint"
        data-testid="bim-submit-hint"
      >
        <button
          type="button"
          class="lk-button lk-button--primary bim__submit"
          data-testid="bim-submit"
          :disabled="!canSubmit"
          @click="onSubmit"
        >
          Добавить в заявку ({{ addableCount }})
        </button>
      </span>
    </div>
  </div>
</template>

<script>
import TargetTablesGrid from '@/components/CreateApplication/TargetTablesGrid.vue';
import Badge from '@/components/ui/Badge.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import { listCitizenships } from '@/api/citizenships';
import { useDeletionsStore } from '@/stores/deletions';
import { useFieldConfig } from '@/composables/useFieldConfig';
import { employeeLabel, vehicleLabel } from '@/utils/applicationDuplicates';
import {
  matchNumberToFormat, validatePartValue, formatPartValue, initializeNumberParts,
} from '@/composables/useNumberFormat';
import { apiRequest } from '@/api/client';

// Спецзначение номера: конкретную машину не опознаёт, формату не подчиняется.
const VEHICLE_BY_FACT = 'По факту';

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
 * Исправима ли причина правкой на месте, решает сервер (ImportRowError.fixable,
 * internal/services/attachment_import_validate.go) - здесь текст причины только
 * показывается человеку и никак не разбирается.
 * "Добавить" по исправленной строке не перепроверяет причину отказа повторно
 * (чёрный список, дубли) - финальная подача делает это как для любой ручной строки.
 */
export default {
  name: 'BlankImportResult',
  components: { TargetTablesGrid, Badge, BaseDropdown },
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
      plateFormats: [],
      selectedTargetTables: [],
      selectedUnloadPlaces: [],
      selectedPassageTables: [],
      // Уведомление субъектов - одна отметка на всю пачку, см. блок bim__consent в разметке.
      pdConsent: false,
      problemRows: [],
      // Ссылки на инпуты ячеек номера по строке (rowNumber -> [input,...]) - нужны для
      // автопрыжка фокуса в следующую ячейку, как в VehicleForm.
      plateCellRefs: {},
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
    showPDConsent() {
      return this.fieldVisible('pd_consent');
    },
    // У бланка машин поле по умолчанию выключено, но администратор может его включить -
    // тогда подпись обязана говорить про владельцев машин, а не про работников.
    consentSubjectsLabel() {
      return this.isPeople ? 'Все работники списка' : 'Владельцы машин из списка';
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
    pdConsentRequired() {
      return this.showPDConsent && this.fieldRequired('pd_consent');
    },
    consentReady() {
      return !this.pdConsentRequired || this.pdConsent;
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
    // Пункты выпадающего списка форматов: справочник как есть, без синтетических
    // значений - выбран может быть только реально существующий формат.
    formatOptions() {
      return this.plateFormats.map((f) => ({ id: f.format.id, name: f.format.name }));
    },
    placesReady() {
      if (this.isPeople) {
        return !this.targetTablesRequired || this.selectedTargetTables.length > 0;
      }
      const unloadOk = !this.unloadPlacesRequired || this.selectedUnloadPlaces.length > 0;
      const passageOk = !this.passageTablesRequired || this.selectedPassageTables.length > 0;
      return unloadOk && passageOk;
    },
    // Исправленная строка уходит в список сразу по кнопке в своей карточке, поэтому к
    // моменту общего «Добавить» всё добавляемое уже стоит предварительным.
    addableCount() {
      return this.pendingCount;
    },
    canSubmit() {
      return this.placesReady && this.consentReady && this.addableCount > 0;
    },
    // Обязательные места, которых не хватает: подпись как на экране плюс признак,
    // есть ли вообще из чего выбирать - "выберите то, чего в справочнике нет" было бы
    // издевательством, а состояние это реальное (грид тогда пишет "Нет доступных...").
    missingPlaces() {
      const fields = this.isPeople
        ? [{
          label: 'места прохода',
          required: this.targetTablesRequired,
          chosen: this.selectedTargetTables,
          options: this.targetTablesOptions,
        }]
        : [
          {
            label: 'места разгрузки',
            required: this.unloadPlacesRequired,
            chosen: this.selectedUnloadPlaces,
            options: this.unloadPlacesOptions,
          },
          {
            label: 'проезд',
            required: this.passageTablesRequired,
            chosen: this.selectedPassageTables,
            options: this.passageTablesOptions,
          },
        ];
      return fields
        .filter((f) => f.required && !f.chosen.length)
        .map((f) => ({ label: f.label, available: f.options.length > 0 }));
    },
    /**
     * Почему «Добавить в заявку» заблокирована - причина словами, а не серая кнопка
     * молчком. Пустая строка гасит подсказку целиком (см. hints.css), поэтому на
     * рабочей кнопке ничего не всплывает.
     */
    submitHint() {
      if (this.canSubmit) return '';
      if (!this.addableCount) {
        return 'Готовых строк пока нет: поправьте строки с ошибками или загрузите другой файл.';
      }
      const absent = this.missingPlaces.filter((p) => !p.available);
      if (absent.length) {
        return `Не из чего выбрать: ${absent.map((p) => p.label).join(' и ')}. Обратитесь в бюро пропусков.`;
      }
      if (this.missingPlaces.length) {
        return `Выберите ${this.missingPlaces.map((p) => p.label).join(' и ')}, чтобы добавить строки в заявку.`;
      }
      if (!this.consentReady) {
        return 'Отметьте, что работники уведомлены об обработке персональных данных - без этого строки в заявку не уйдут.';
      }
      return '';
    },
    // Места в файле не приходят и раскатываются на всю пачку разом - патч полей строки
    // собираем здесь, применяет его родитель ко всем предварительным строкам.
    placesPatch() {
      // Отметка согласия едет вместе с местами: у строк из файла своей галочки нет, а
      // подача (applicationEntityPayload) читает pdConsent с каждой строки. Без этого
      // импортированный работник попадал в заявку без отметки - и запись реестра по нему
      // тоже не создавалась (гейт uniqueEmployeeService.Create).
      if (this.isPeople) {
        return {
          targetTables: [...this.selectedTargetTables],
          passageTables: this.formatSelectedNames(this.selectedTargetTables, this.targetTablesOptions),
          pdConsent: this.pdConsent,
        };
      }
      return {
        unloadPlaces: [...this.selectedUnloadPlaces],
        unloadingPlace: this.formatSelectedNames(this.selectedUnloadPlaces, this.unloadPlacesOptions),
        passage_tables: [...this.selectedPassageTables],
        pdConsent: this.pdConsent,
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
      // Новый файл - новые люди: отметку прошлой пачки не наследуем.
      this.pdConsent = false;
      this.plateCellRefs = {};
      // Справочник ждём ДО построения карточек ошибок: людям он нужен, чтобы
      // citizenshipName собрался при подаче, машинам - чтобы определить формат строки
      // и нарисовать его ячейки сразу, а не мигать пустым полем номера.
      if (this.isPeople && !this.citizenships.length) {
        await this.loadCitizenships();
      }
      if (!this.isPeople && !this.plateFormats.length) {
        await this.loadPlateFormats();
      }
      this.problemRows = this.rows
        .filter((r) => r.errors && r.errors.length)
        .map((r) => ({
          rowNumber: r.row_number,
          errors: r.errors || [],
          original: r,
          fields: this.isPeople ? this.buildPeopleFields(r) : this.buildCarFields(r),
        }));
      this.stageAcceptedRows();
    },

    buildPeopleFields(r) {
      return {
        lastName: (r.employee && r.employee.last_name) || '',
        firstName: (r.employee && r.employee.first_name) || '',
        middleName: (r.employee && r.employee.middle_name) || '',
        citizenshipId: (r.employee && r.employee.citizenship_id) || null,
      };
    },

    // Формат строки система определяет сама - тем же перебором, что и проверка номера
    // (matchNumberToFormat) - и он сразу стоит выбранным в списке: подпись формата и
    // ячейки под ней всегда об одном и том же. Номер из файла раскладывается по этим
    // ячейкам, чтобы годную часть не пришлось перепечатывать.
    // Не лёг ни в один формат - выбора нет, ячеек нет: человек называет формат сам,
    // как это делает форма подачи при непонятном номере (VehicleForm,
    // applyEditedVehicleNumber). Подставлять сюда дефолтный формат значило бы утверждать
    // на экране то, чего система не определяла.
    buildCarFields(r) {
      const raw = (r.vehicle && r.vehicle.car_number) || '';
      // matchNumberToFormat отдаёт ЗАПИСЬ справочника ({format, cells}), а не сам
      // format - id лежит на уровень глубже.
      const matched = matchNumberToFormat(raw, this.plateFormats);
      return {
        numberParts: matched ? [...matched.parts] : [],
        mark: (r.vehicle && r.vehicle.car_brand) || '',
        formatId: matched ? matched.format.format.id : null,
        isByFact: false,
      };
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
    async loadPlateFormats() {
      try {
        const res = await apiRequest('/license-plate-formats', { method: 'GET' });
        this.plateFormats = res.ok ? await res.json() : [];
      } catch (error) {
        // Без справочника форматов номер проверить нечем. Считаем, что проверка
        // недоступна, и не пускаем правку строки вслепую - см. plateAccepted.
        console.error('Ошибка при загрузке форматов номеров:', error);
        this.plateFormats = [];
      }
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
    // Все ли причины строки правятся прямо здесь. Признак приходит с сервера полем
    // fixable: разбирать текст причины фронтом нельзя - формулировки меняются, и
    // каждая новая (несовпадение формата номера) молча блокировала строку навсегда.
    rowFixable(row) {
      return row.errors.every((error) => !!error.fixable);
    },
    // Строка становится добавляемой, только когда ВСЕ её причины исправимы здесь
    // И минимальные поля реально заполнены - блокирующие причины (ЧС, дубль,
    // паспорт/патент - полей для них тут нет по 152-ФЗ) чекбокс не разблокируют
    // никакой правкой ФИО/номера.
    canIncludeRow(row) {
      if (!this.rowFixable(row)) return false;
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
      return this.plateAccepted(this.rowPlateValue(row), row.fields.formatId);
    },

    // Формат строки - ровно тот, что выбран в её списке: и ячейки ввода, и проверка
    // номера (plateAccepted) идут от него одного. Пусто - формат не выбран, ячеек нет.
    rowPlateFormat(row) {
      return this.selectedFormatEntry(row.fields.formatId);
    },

    // Собранный номер - те же части через пробел, что даёт VehicleForm (numberParts.join(' ')).
    rowPlateValue(row) {
      if (row.fields.isByFact) return VEHICLE_BY_FACT;
      return (row.fields.numberParts || []).join(' ');
    },

    /**
     * Номер годится, только когда он ложится в один из форматов справочника - тот же
     * разбор, которым пользуется форма ручного ввода (matchNumberToFormat). Проверять
     * одну непустоту нельзя: строка приезжает сюда именно потому, что номер не подошёл
     * формату, и непустым он был с самого начала - галочка разблокировалась бы без
     * единой правки, а мусор уехал бы в заявку.
     */
    /** Почему строку пока нельзя отметить - причина конкретная, а не общая просьба. */
    rowNote(row) {
      if (!this.isPeople) {
        if (!this.plateFormats.length) return 'Справочник форматов номеров недоступен, проверить номер нечем.';
        const chosen = this.selectedFormatEntry(row.fields.formatId);
        if (!chosen) return 'Выберите формат номера, чтобы отметить строку.';
        if (!this.rowPlateValue(row).trim()) return 'Введите номер Т/С, чтобы отметить строку.';
        return `Номер не подходит формату "${chosen.format.name}" - поправьте его или выберите другой формат.`;
      }
      if (!row.fields.lastName.trim() || !row.fields.firstName.trim()) {
        return 'Заполните фамилию и имя, чтобы отметить строку.';
      }
      return 'Выберите гражданство, чтобы отметить строку.';
    },

    selectedFormatEntry(formatId) {
      return this.plateFormats.find((f) => f.format.id === formatId) || null;
    },

    getPlaceholder(cell) {
      return cell.cell_type === 'numbers' ? '0'.repeat(cell.max_length) : 'A'.repeat(cell.max_length);
    },

    // Компактнее, чем в VehicleForm (25px/50px): карточка ошибки - не полноценная
    // строка формы, ячейкам достаточно места под символы без запаса.
    getInputWidth(cell) {
      const baseWidth = 20;
      const minWidth = 28;
      return `${Math.max(minWidth, cell.max_length * baseWidth)}px`;
    },

    setPlateCellRef(rowNumber, index, el) {
      if (!this.plateCellRefs[rowNumber]) this.plateCellRefs[rowNumber] = [];
      this.plateCellRefs[rowNumber][index] = el;
    },

    validatePlatePart(row, index, event, cell) {
      const value = validatePartValue(event.target.value, cell);
      row.fields.numberParts[index] = value;
      event.target.value = value;
      this.advancePlateFocus(row, index, value, cell);
    },

    // Клетка заполнена до предела - фокус сам прыгает в следующую (см. VehicleForm.advanceCellFocus).
    advancePlateFocus(row, index, value, cell) {
      if (!value || value.length < cell.max_length) return;
      const format = this.rowPlateFormat(row);
      if (!format || index >= format.cells.length - 1) return;
      this.$nextTick(() => {
        const next = (this.plateCellRefs[row.rowNumber] || [])[index + 1];
        if (next && !next.disabled) next.focus();
      });
    },

    formatPlatePart(row, index, cell) {
      if (row.fields.numberParts[index]) {
        const formatted = formatPartValue(row.fields.numberParts[index], cell);
        if (formatted !== row.fields.numberParts[index]) {
          row.fields.numberParts[index] = formatted;
        }
      }
    },

    // Смена формата очищает ячейки, как это делает форма ручного ввода (VehicleForm,
    // selectFormat). Подставлять сюда исходное значение из файла нельзя: человек мог
    // уже поправить номер руками, и переключение формата молча вернуло бы то, что
    // пришло в бланке. Разбор исходного номера остаётся только при первом показе строки.
    selectFormat(row, formatId) {
      row.fields.formatId = formatId;
      row.fields.numberParts = initializeNumberParts(this.rowPlateFormat(row));
    },

    handleRowFactChange(row) {
      row.fields.numberParts = row.fields.isByFact
        ? []
        : initializeNumberParts(this.rowPlateFormat(row));
    },

    /**
     * Номер проверяется ТОЛЬКО по выбранному формату - тому же, чьими ячейками он
     * набран. Формат не выбран (номер не лёг ни в один при разборе и человек ещё не
     * назвал свой) - проверять не по чему, строка не добавляется. "По факту" формату
     * не подчиняется и принимается при любом выборе.
     */
    plateAccepted(raw, formatId) {
      const value = (raw || '').trim();
      if (!value) return false;
      if (value.toLowerCase() === VEHICLE_BY_FACT.toLowerCase()) return true;
      const chosen = this.selectedFormatEntry(formatId);
      return !!chosen && !!matchNumberToFormat(value, [chosen]);
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
      // Собранный номер строки правки - части ячеек через пробел, тот же вид, что
      // отдаёт VehicleForm (numberParts.join(' ')) - см. rowPlateValue.
      const fields = isFixed
        ? { plateNumber: this.rowPlateValue(row), mark: row.fields.mark, formatId: row.fields.formatId }
        : { plateNumber: veh.car_number, mark: veh.car_brand, formatId: null };
      const mark = (fields.mark || '').trim() || null;
      return {
        plateNumber: (fields.plateNumber || '').trim(),
        mark,
        markId: null,
        markName: mark,
        unloadingPlace: this.formatSelectedNames(this.selectedUnloadPlaces, this.unloadPlacesOptions),
        unloadPlaces: [...this.selectedUnloadPlaces],
        passage_tables: [...this.selectedPassageTables],
        // Формат, выбранный в карточке ошибки, уезжает вместе со строкой в то же поле,
        // которым пользуется VehicleForm при ручном добавлении/правке (см.
        // applyEditedVehicleNumber) - второго поля под это заводить не нужно. У строки
        // "по факту" формата может не быть - там номер и не разбирается.
        formatId: fields.formatId || null,
        isExisting: false,
      };
    },
    /**
     * «Добавить» в самой карточке: исправленная строка уходит в список сразу, как
     * остальные разобранные, и исчезает из перечня ошибок. Раньше тут стояла галочка,
     * и строка ждала общей кнопки - было непонятно, случилось ли что-то от клика.
     */
    addProblemRow(row) {
      if (!this.canIncludeRow(row)) return;
      const built = this.isPeople
        ? this.buildEmployeeFromRow(row, true)
        : this.buildVehicleFromRow(row, true);
      this.$emit('stage', { attachmentType: this.attachmentType, rows: [built] });
      this.problemRows = this.problemRows.filter((r) => r.rowNumber !== row.rowNumber);
    },

    // «Добавить в заявку»: всё добавляемое уже стоит в списке предварительным, родителю
    // остаётся раскатать места и сделать строки обычными.
    onSubmit() {
      if (!this.canSubmit) return;
      this.$emit('import', {
        attachmentType: this.attachmentType,
        places: this.placesPatch,
        rows: [],
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
            worksheet.addRow([r.row_number, label, r.errors.map((e) => e.text).join('; ')]);
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

/* Основное действие панели - на всю её ширину, вспомогательные остаются по размеру
   содержимого строкой выше. Ширину держит обёртка-якорь подсказки, кнопка тянется
   по ней: flex-свойства на самой кнопке обёртка бы съела. */
.bim__submit-wrap {
  flex: 1 0 100%;
}

.bim__submit {
  width: 100%;
}

/* Счётчики разбора - блок карточкой, как остальные блоки формы: цифры на голом фоне
   читались случайным текстом. Одна строка ВСЕГДА (владелец: третий счётчик уезжал на
   вторую строку) - счётчики делят ширину РОВНЫМИ третями (flex-basis 0), поэтому при
   нехватке места переносится текст подписи внутри своей колонки, а не сам счётчик
   на новую строку. */
.bim__counters {
  display: flex;
  flex-wrap: nowrap;
  align-items: flex-start;
  gap: 12px 0;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface-2);
}

.bim__counter {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  flex: 1 1 0;
}

/* Разделители между счётчиками: смысл у трёх чисел разный, слитной строкой они
   читаются как одно. */
.bim__counter + .bim__counter {
  margin-left: 14px;
  padding-left: 14px;
  border-left: 1px solid var(--border);
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

/* Отметка согласия стоит последней перед кнопкой - мелким текстом, как в форме подачи
   (EmployeeForm .consent-option): это подтверждение, а не поле данных. */
.bim__consent {
  margin-top: 4px;
}

.bim__consent .consent-option {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 11px;
  line-height: 1.3;
  cursor: pointer;
}

.bim__consent .consent-option input[type="checkbox"] {
  margin-top: 2px;
  flex-shrink: 0;
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
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 4px;
  font-size: 15px;
}

.bim__problems-count {
  background: var(--danger-bg);
  color: var(--danger-text);
  border-radius: 999px;
  padding: 2px 8px;
  font-size: 12px;
  font-weight: 600;
}

.bim__problems-hint {
  margin: 0 0 12px;
  font-size: 12px;
  line-height: 1.45;
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

.bim__problems-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
  max-height: 420px;
  overflow-y: auto;
  scrollbar-width: thin;
  /* Место под скроллбар держим постоянно: без этого его появление/исчезновение при
     пересечении высотой max-height сужает/расширяет список и двигает соседнюю колонку
     (места прохода/разгрузки), даже когда высота карточек сама уже стабильна. */
  scrollbar-gutter: stable;
}

.bim__problem {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
}

/* Строку, которую здесь не спасти, отделяем цветом рамки, а не только текстом
   причины: в списке из десятка карточек это единственный быстрый признак. */
.bim__problem--blocked {
  border-color: color-mix(in srgb, var(--danger) 35%, var(--surface));
  background: color-mix(in srgb, var(--danger) 4%, var(--surface));
}

.bim__problem-head {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  min-width: 0;
}

.bim__problem-num {
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
}





.bim__reasons {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
}

/* Причина - готовая фраза целиком: своя строка на всю ширину карточки, без переноса
   в узкую колонку. Исправимая подсвечена как предупреждение, блокирующая - как отказ. */
.bim__reason {
  padding: 6px 10px;
  border-radius: var(--radius-sm);
  background: var(--warning-bg);
  color: var(--warning-text);
  font-size: 12.5px;
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.bim__reason--blocking {
  background: var(--danger-bg);
  color: var(--danger-text);
}

.bim__fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 8px;
}

.bim__field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.bim__field-label {
  font-size: 11px;
  color: var(--text-muted);
}

.bim__cell-input {
  padding: 7px 10px;
  font-size: 13px;
}

/* Выпадающий список форматов - общий BaseDropdown, ужатый до размеров карточки:
   штатные 30px пилюлей и шрифт 14px рассчитаны на строку формы, а здесь таких
   карточек десяток подряд. Метрики берём у соседнего поля марки (.bim__cell-input),
   чтобы в одном ряду сетки они стояли вровень, а скругление - карточное (radius-md),
   как у ячеек номера, а не пилюля формы. Меню телепортится в body, до него это
   правило не достаёт - и не должно: оно висит поверх, карточку не распирает. */
.bim__format :deep(.base-dropdown__button) {
  min-height: 34px;
  padding: 0 10px;
  gap: 6px;
  border-radius: var(--radius-md);
}

.bim__format :deep(.base-dropdown__text) {
  font-size: 13px;
  font-weight: 400;
}

/* Номер по ячейкам - компактный аналог VehicleForm.number__field: та же пилюля с
   внутренними разделителями, но ниже (34px против 40px) и без фиксированной ширины -
   сумма ширин ячеек формата, не растянутая колонка. Занимает всю ширину карточки
   (bim__field--plate), а не одну колонку grid-а: иначе узкие 150px-колонки бы
   растянулись под самое длинное поле и раздули формат/марку рядом. */
.bim__field--plate {
  grid-column: 1 / -1;
}

.bim__plate-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.bim__plate-field {
  display: inline-flex;
  max-width: 100%;
  height: 34px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  overflow: hidden;
  background: var(--surface);
}

.bim__plate-cell {
  border: none;
  outline: none;
  height: 100%;
  min-width: 0;
  flex: 1 1 auto;
  text-align: center;
  font-size: 13px;
  background: transparent;
  color: var(--color-text);
}

.bim__plate-cell:not(:last-child) {
  border-right: 1px solid var(--border);
}

.bim__plate-cell:focus {
  background: var(--surface-2);
}

.bim__plate-cell::placeholder {
  color: var(--text-muted);
  font-size: 11px;
}

.bim__plate-field--fact {
  width: 100%;
}

.bim__plate-fact-input {
  width: 100%;
  height: 100%;
  border: none;
  outline: none;
  background: transparent;
  padding: 0 10px;
  text-align: left;
  font-size: 13px;
  color: var(--text-muted);
}

.bim__plate-empty {
  font-size: 12px;
  color: var(--text-muted);
  padding: 7px 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface-2);
}

/* Тумблер "по факту" - тот же механизм, что VehicleForm.fact-toggle (нативный чекбокс
   спрятан, но остаётся в потоке для фокуса с клавиатуры), только меньше: здесь это
   вспомогательный переключатель внутри карточки ошибки, а не элемент полноценной формы. */
.bim__fact-toggle {
  display: flex;
  align-items: center;
  gap: 5px;
  cursor: pointer;
  user-select: none;
  font-size: 11px;
  color: var(--text-muted);
}

.bim__fact-checkbox {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
  pointer-events: none;
}

.bim__fact-switch {
  position: relative;
  display: inline-block;
  width: 26px;
  height: 15px;
  border-radius: var(--radius-pill);
  background: var(--border);
  transition: background-color 0.2s ease;
  flex-shrink: 0;
}

.bim__fact-switch::after {
  content: '';
  position: absolute;
  top: 2px;
  left: 2px;
  width: 11px;
  height: 11px;
  border-radius: 50%;
  background: var(--surface);
  transition: transform 0.2s ease;
}

.bim__fact-checkbox:checked + .bim__fact-switch {
  background: var(--color-primary);
}

.bim__fact-checkbox:checked + .bim__fact-switch::after {
  transform: translateX(11px);
}

.bim__fact-checkbox:focus-visible + .bim__fact-switch {
  box-shadow: var(--shadow-focus);
}

/* Высота зарезервирована на 2 строки текста (line-height того же блока подсказок
   .bim__problems-hint - 1.45) независимо от того, показана подсказка сейчас или нет:
   иначе карточка меняет высоту по мере ввода, у списка карточек то появляется, то
   пропадает вертикальный скролл, а из-за смены его ширины дёргается и соседняя
   колонка. Гашение - visibility, а не display/v-if, чтобы место оставалось. */
.bim__problem-note {
  margin: 0;
  font-size: 12px;
  line-height: 1.45;
  min-height: calc(1.45em * 2);
  color: var(--text-muted);
}

.bim__problem-note--hidden {
  visibility: hidden;
}

@media (max-width: 768px) {
  /* Одна строка счётчиков сохраняется (требование владельца), но на 320 колонка
     сжимается до 59px, и в неё не влезает ни четырёхзначное число (74px при 28px
     кегля), ни слово «добавлению» - текст вылезал поверх разделителя в соседний
     счётчик. Меньше кегль, поля и разделители - колонка становится 72px, и всё
     помещается. Переносить счётчик на вторую строку по-прежнему нельзя. */
  .bim__counters {
    padding: 10px;
  }

  .bim__counter + .bim__counter {
    margin-left: 6px;
    padding-left: 6px;
  }

  .bim__counter-value {
    font-size: 22px;
  }

  .bim__counter-label {
    font-size: 11px;
    line-height: 1.25;
  }

  /* На телефоне карточка идёт одним столбцом: поля во всю ширину, отметка -
     полноценная строка-цель, а не 16px квадрат в углу. */
  .bim__fields {
    grid-template-columns: 1fr;
  }

  .bim__cell-input {
    min-height: 44px;
    font-size: 16px;
  }

  /* Тач-таргет кнопки выбора формата: WCAG 2.5.5. */
  .bim__format :deep(.base-dropdown__button) {
    min-height: 44px;
  }

  /* Тач-таргет ячеек номера и тумблера "по факту": WCAG 2.5.5. */
  .bim__plate-field {
    height: 44px;
  }

  .bim__plate-cell,
  .bim__plate-fact-input {
    font-size: 16px;
  }

  .bim__fact-toggle {
    min-height: 44px;
  }

  /* Тач-таргет кнопки добавления строки: у --sm высота меньше нормы WCAG 2.5.5. */
  .bim__row-add {
    min-height: 44px;
  }

  .bim__problems-list {
    max-height: none;
    overflow-y: visible;
  }

  /* Тач-таргет 44px (WCAG 2.5.5): --sm-модификатора у этих кнопок нет, но базовые
     8px padding дают 30px высоты. */
  .bim__actions .lk-button {
    min-height: 44px;
    flex: 1 1 auto;
  }
}
</style>
