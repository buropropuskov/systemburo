<template>
  <Teleport to="body">
    <transition
      name="supp-fade"
      @after-leave="resetState"
    >
      <div
        v-if="show"
        class="modal-overlay supp-overlay"
        data-testid="supplement-modal"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div
          class="modal-content supp-modal"
          :class="{ 'is-dragging': sheetDragging }"
          :style="sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : null"
          role="dialog"
          aria-modal="true"
          aria-label="Дополнение заявки"
          @mousedown.stop
          @click.stop
          @touchstart="onSheetTouchStart"
          @touchmove="onSheetTouchMove"
          @touchend="onSheetTouchEnd"
        >
          <div
            class="sheet-handle"
            aria-hidden="true"
          />

          <div class="supp-modal__header">
            <h3 class="supp-modal__title">
              Дополнить заявку {{ application.application_number }}
            </h3>
            <button
              type="button"
              class="supp-modal__close"
              aria-label="Закрыть"
              data-testid="supplement-button-close"
              @click="close"
            >
              &times;
            </button>
          </div>

          <div
            ref="body"
            class="supp-modal__body"
          >
            <p class="supp-modal__hint">
              Выберите вложение заявки - в него попадут новые строки. Организация, компания и
              срок действия принадлежат вложению и не меняются. Если заявка уже в работе,
              добавка уйдёт на отдельный круг согласования, а выданные пропуска продолжат
              действовать.
            </p>

            <div class="supp-modal__field supp-modal__field--target">
              <label class="supp-modal__label">
                Вложение заявки <span class="supp-modal__req">*</span>
              </label>
              <BaseDropdown
                :model-value="selectedAttachmentId"
                :options="attachmentOptions"
                label-key="label"
                value-key="id"
                :searchable="attachmentOptions.length > 5"
                :teleport="true"
                :menu-z-index="MENU_Z_INDEX"
                placeholder="Выберите вложение"
                data-testid="supplement-attachment"
                @update:model-value="onAttachmentChange"
              />
              <p
                v-if="!attachmentOptions.length"
                class="supp-modal__note"
                data-testid="supplement-no-attachments"
              >
                У заявки нет действующих вложений - дополнять нечего.
              </p>
            </div>

            <div
              v-if="selectedAttachment"
              class="supp-modal__readonly"
            >
              <div class="supp-modal__field">
                <label
                  class="supp-modal__label"
                  for="supp-organization"
                >Организация</label>
                <input
                  id="supp-organization"
                  class="lk-input supp-modal__static"
                  type="text"
                  readonly
                  :value="application.organization_name || 'Не указана'"
                >
              </div>
              <div class="supp-modal__field">
                <label
                  class="supp-modal__label"
                  for="supp-company"
                >Компания</label>
                <input
                  id="supp-company"
                  class="lk-input supp-modal__static"
                  type="text"
                  readonly
                  :value="application.company_name || 'Без компании'"
                >
              </div>
              <div class="supp-modal__field">
                <label
                  class="supp-modal__label"
                  for="supp-period"
                >Срок действия вложения</label>
                <input
                  id="supp-period"
                  class="lk-input supp-modal__static"
                  type="text"
                  readonly
                  data-testid="supplement-period"
                  :value="periodLabel"
                >
              </div>
            </div>

            <div
              v-if="selectedAttachment"
              class="supp-modal__form"
              :class="{ 'supp-modal__form--busy': loadingFieldConfig }"
            >
              <template v-if="attachmentType === 'cars'">
                <VehicleForm
                  ref="vehicleForm"
                  :field-config="currentFieldConfig"
                  :user-organization="application.organization_name || ''"
                  :user-organization-id="application.organization_id || null"
                  :user-company="application.company_name || ''"
                  :user-company-id="application.company_id || null"
                  :existing-vehicles="currentRows"
                  :entry-period="entryPeriod"
                  :existing-modal-z-index="EXISTING_MODAL_Z_INDEX"
                  @notices-change="placeNotices = $event"
                  @vehicle-added="addRow"
                  @vehicles-added="addRows"
                  @vehicle-updated="updateRow"
                />
                <VehiclesList
                  :vehicles="sortedRows"
                  :sort-field="sortField"
                  :sort-direction="sortDirection"
                  :all-unloading-places="allUnloadingPlaces"
                  :license-plate-formats="licensePlateFormats"
                  :detail-info="detailInfo"
                  @sort="sortBy"
                  @edit-vehicle="editRow"
                  @delete-vehicle="deleteRow"
                />
              </template>

              <template v-else-if="attachmentType === 'people'">
                <EmployeeForm
                  ref="employeeForm"
                  :field-config="currentFieldConfig"
                  :user-organization="application.organization_name || ''"
                  :user-organization-id="application.organization_id || null"
                  :user-company="application.company_name || ''"
                  :user-company-id="application.company_id || null"
                  :existing-employees="currentRows"
                  :entry-period="entryPeriod"
                  :existing-modal-z-index="EXISTING_MODAL_Z_INDEX"
                  @notices-change="placeNotices = $event"
                  @employee-added="addRow"
                  @employees-added="addRows"
                  @employee-updated="updateRow"
                />
                <EmployeesList
                  :employees="sortedRows"
                  :sort-field="sortField"
                  :sort-direction="sortDirection"
                  :all-tables="allTables"
                  :detail-info="detailInfo"
                  @sort="sortBy"
                  @edit-employee="editRow"
                  @delete-employee="deleteRow"
                />
              </template>

              <template v-else-if="attachmentType === 'items'">
                <ItemsForm
                  ref="itemsForm"
                  :field-config="currentFieldConfig"
                  :existing-items="currentRows"
                  @item-added="addRow"
                  @items-added="addRows"
                  @item-updated="updateRow"
                />
                <ItemsList
                  :items="sortedRows"
                  :sort-field="sortField"
                  :sort-direction="sortDirection"
                  @sort="sortBy"
                  @edit-item="editRow"
                  @delete-item="deleteRow"
                />
              </template>
            </div>

            <p
              v-if="otherAttachmentsSummary"
              class="supp-modal__other"
              data-testid="supplement-other-attachments"
            >
              В других вложениях уже подготовлено: {{ otherAttachmentsSummary }}. Они уйдут
              вместе с этим дополнением.
            </p>
          </div>

          <div class="supp-modal__footer">
            <div class="supp-modal__field">
              <label
                class="supp-modal__label"
                for="supp-comment"
              >Комментарий - зачем понадобилась добавка</label>
              <textarea
                id="supp-comment"
                v-model="comment"
                class="lk-textarea supp-modal__comment"
                rows="2"
                maxlength="1000"
                placeholder="Например: подрядчик прислал двух монтажников сверх списка"
                data-testid="supplement-comment"
              />
            </div>
            <div class="supp-modal__actions">
              <button
                type="button"
                class="lk-button lk-button--ghost"
                @click="close"
              >
                Отмена
              </button>
              <span
                class="hint-anchor hint-anchor--right"
                :data-hint="submitHint"
              >
                <button
                  type="button"
                  class="lk-button lk-button--primary"
                  :disabled="!canSubmit"
                  data-testid="supplement-submit"
                  @click="submit"
                >
                  {{ submitting ? 'Отправка...' : 'Отправить дополнение' }}
                </button>
              </span>
            </div>
          </div>
        </div>
      </div>

    </transition>

    <!-- Предупреждения по выбранным местам и постам (#1183): формы считают их и здесь,
         панель выводится поверх окна - под ним её просто не было бы видно. Вне
         transition: тот допускает единственный корневой элемент. -->
    <SchedulePlaceWarningPanel v-if="show" :groups="placeNotices" :z-index="NOTICES_Z_INDEX" />
  </Teleport>
</template>

<script>
import { ref } from 'vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import VehicleForm from '@/components/CreateApplication/VehicleForm.vue';
import VehiclesList from '@/components/CreateApplication/VehiclesList.vue';
import EmployeeForm from '@/components/CreateApplication/EmployeeForm.vue';
import EmployeesList from '@/components/CreateApplication/EmployeesList.vue';
import ItemsForm from '@/components/CreateApplication/ItemsForm.vue';
import ItemsList from '@/components/CreateApplication/ItemsList.vue';
import { createSupplement } from '@/api/applications';
import SchedulePlaceWarningPanel from './SchedulePlaceWarningPanel.vue';
import { toAttachmentContent } from '@/utils/applicationEntityPayload';
import { SUPPLEMENT_MERGED } from '@/utils/supplementStatuses';
import { useDeletionsStore } from '@/stores/deletions';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { useSwipeDismiss } from '@/composables/useSwipeDismiss';
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';

// Окно открывается ПОВЕРХ детали заявки (10002) - см. лестницу z-index в
// .claude/ui-etalon. Дочерние слои (поиск существующих, меню дропдауна) обязаны
// лечь выше него, иначе уедут за собственного родителя.
// Дополнять можно только вложения тех типов, под которые есть форма ввода.
const SUPPORTED_ATTACHMENT_TYPES = ['cars', 'people', 'items'];

const EXISTING_MODAL_Z_INDEX = 10012;
const MENU_Z_INDEX = 10014;
// Панель предупреждений выше самого окна, но ниже его выпадающих меню.
const NOTICES_Z_INDEX = 10013;


// Ключи сортировки, которые реально эмитят списки (`$emit('sort', key)`). Числовые
// поля сортируем как числа, остальные - регистронезависимо строкой.
const SORT_ACCESSORS = {
    cars: {
        number: { numeric: true, get: (v) => v.id },
        plate: { get: (v) => v.plateNumber },
        mark: { get: (v) => v.mark },
    },
    people: {
        number: { numeric: true, get: (e) => e.id },
        lastName: { get: (e) => e.lastName },
        firstName: { get: (e) => e.firstName },
        middleName: { get: (e) => e.middleName },
    },
    items: {
        number: { numeric: true, get: (i) => i.id },
        name: { get: (i) => i.itemName },
        quantity: { numeric: true, get: (i) => i.quantity },
    },
};

const TYPE_NOUNS = {
    cars: ['машина', 'машины', 'машин'],
    people: ['сотрудник', 'сотрудника', 'сотрудников'],
    items: ['позиция ТМЦ', 'позиции ТМЦ', 'позиций ТМЦ'],
};

function plural(count, forms) {
    const mod10 = count % 10;
    const mod100 = count % 100;
    if (mod10 === 1 && mod100 !== 11) return forms[0];
    if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return forms[1];
    return forms[2];
}

/** Дата вложения приходит либо `YYYY-MM-DD`, либо ISO с временем - нужен только день. */
function apiDate(value) {
    return value ? String(value).slice(0, 10) : null;
}

/** Время вложения хранится с секундами (`08:00:00`); формам и показу нужны часы-минуты. */
function shortTime(value) {
    return value ? String(value).slice(0, 5) : '';
}

function humanDate(value) {
    const iso = apiDate(value);
    if (!iso) return '';
    const [year, month, day] = iso.split('-');
    return `${day}.${month}.${year}`;
}

export default {
    name: 'SupplementModal',
    components: {
        SchedulePlaceWarningPanel,
        BaseDropdown,
        VehicleForm,
        VehiclesList,
        EmployeeForm,
        EmployeesList,
        ItemsForm,
        ItemsList,
    },
    props: {
        show: {
            type: Boolean,
            default: false,
        },
        application: {
            type: Object,
            required: true,
        },
        // Вложения заявки - те же, что уже загрузила карточка (GET /applications/:id/attachments).
        attachments: {
            type: Array,
            default: () => [],
        },
        // Справочники для показа добавленных строк. Карточка заявки грузит их сама
        // (loadCommonData) - второй раз за ними не ходим.
        allUnloadingPlaces: {
            type: Array,
            default: () => [],
        },
        licensePlateFormats: {
            type: Array,
            default: () => [],
        },
        allTables: {
            type: Array,
            default: () => [],
        },
    },
    emits: ['close', 'submitted'],
    setup() {
        const closeRef = { fn: null };
        const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => closeRef.fn && closeRef.fn());
        const body = ref(null);
        const swipe = useSwipeDismiss(() => closeRef.fn && closeRef.fn(), {
            handleSelector: '.sheet-handle, .supp-modal__header',
            getScrollTop: () => body.value?.scrollTop ?? 0,
        });
        return {
            closeRef,
            body,
            onOverlayMousedown,
            onOverlayMouseup,
            sheetOffset: swipe.offset,
            sheetDragging: swipe.isDragging,
            onSheetTouchStart: swipe.onTouchStart,
            onSheetTouchMove: swipe.onTouchMove,
            onSheetTouchEnd: swipe.onTouchEnd,
            EXISTING_MODAL_Z_INDEX,
            NOTICES_Z_INDEX,
            MENU_Z_INDEX,
        };
    },
    data() {
        return {
            selectedAttachmentId: null,
            // Строки копим ПО ВЛОЖЕНИЯМ: смена вложения в дропдауне не должна терять
            // уже набранное, а контракт additions[] и так принимает несколько целей.
            rowsByAttachment: {},
            rowIdCounter: 1,
            comment: '',
            submitting: false,
            fieldConfigByAttachment: {},
            loadingFieldConfig: false,
            fieldConfigSeq: 0,
            placeNotices: [],
            sortField: null,
            sortDirection: null,
        };
    },
    computed: {
        // Вложение с истёкшим сроком бэк отклоняет (строки не доживут до проходной) -
        // не предлагаем его вовсе, иначе выбор заканчивается 400 без объяснимой причины.
        activeAttachments() {
            const today = new Date();
            const localToday = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`;
            return this.attachments.filter((a) => {
                if (!SUPPORTED_ATTACHMENT_TYPES.includes(a.attachment_type)) return false;
                const to = apiDate(a.entry_date_to);
                return !to || to >= localToday;
            });
        },
        attachmentOptions() {
            return this.activeAttachments.map((a) => ({
                id: a.id,
                label: this.attachmentLabel(a),
            }));
        },
        selectedAttachment() {
            return this.activeAttachments.find((a) => a.id === this.selectedAttachmentId) || null;
        },
        attachmentType() {
            return this.selectedAttachment ? this.selectedAttachment.attachment_type : null;
        },
        currentFieldConfig() {
            const uaId = this.selectedAttachment?.unique_attachment_id;
            return (uaId && this.fieldConfigByAttachment[uaId]) || {};
        },
        currentRows() {
            return this.rowsByAttachment[this.selectedAttachmentId] || [];
        },
        sortedRows() {
            const accessors = SORT_ACCESSORS[this.attachmentType] || {};
            const accessor = accessors[this.sortField];
            if (!accessor) return this.currentRows;
            const dir = this.sortDirection === 'desc' ? -1 : 1;
            return [...this.currentRows].sort((a, b) => {
                if (accessor.numeric) return (Number(accessor.get(a)) - Number(accessor.get(b))) * dir;
                const valueA = String(accessor.get(a) ?? '').toLowerCase();
                const valueB = String(accessor.get(b) ?? '').toLowerCase();
                if (valueA < valueB) return -dir;
                if (valueA > valueB) return dir;
                return 0;
            });
        },
        totalRows() {
            return Object.values(this.rowsByAttachment).reduce((sum, rows) => sum + rows.length, 0);
        },
        // Подсказка «в других вложениях уже подготовлено» - иначе набранное в
        // предыдущем вложении не видно после переключения дропдауна и выглядит потерянным.
        otherAttachmentsSummary() {
            const parts = [];
            for (const attachment of this.activeAttachments) {
                if (attachment.id === this.selectedAttachmentId) continue;
                const count = (this.rowsByAttachment[attachment.id] || []).length;
                if (!count) continue;
                parts.push(`${count} ${plural(count, TYPE_NOUNS[attachment.attachment_type])}`);
            }
            return parts.join(', ');
        },
        entryPeriod() {
            const a = this.selectedAttachment;
            if (!a) return null;
            return {
                date_from: apiDate(a.entry_date_from),
                date_to: apiDate(a.entry_date_to),
                time_from: shortTime(a.entry_time_from) || null,
                time_to: shortTime(a.entry_time_to) || null,
            };
        },
        detailInfo() {
            const a = this.selectedAttachment;
            return {
                organization: this.application.organization_name || '',
                company: this.application.company_name || '',
                entryDateTo: a ? apiDate(a.entry_date_to) : null,
                timeFrom: a ? shortTime(a.entry_time_from) : '',
                timeTo: a ? shortTime(a.entry_time_to) : '',
            };
        },
        periodLabel() {
            const a = this.selectedAttachment;
            if (!a) return 'Выберите вложение';
            const from = humanDate(a.entry_date_from);
            const to = humanDate(a.entry_date_to);
            const time = shortTime(a.entry_time_from) && shortTime(a.entry_time_to)
                ? `, ${shortTime(a.entry_time_from)} - ${shortTime(a.entry_time_to)}`
                : '';
            if (from && to) return `${from === to ? from : `${from} - ${to}`}${time}`;
            return from || to || 'Срок не задан';
        },
        canSubmit() {
            return !this.submitting && this.totalRows > 0;
        },
        submitHint() {
            if (this.canSubmit || this.submitting) return '';
            if (!this.attachmentOptions.length) return 'У заявки нет действующих вложений';
            if (!this.selectedAttachmentId) return 'Выберите вложение заявки';
            return 'Добавьте хотя бы одну строку';
        },
    },
    watch: {
        show(open) {
            setBodyScrollLock(this, open);
            if (open) this.initState();
        },
    },
    mounted() {
        this.closeRef.fn = this.close;
        document.addEventListener('keydown', this.onKeydown);
        if (this.show) {
            setBodyScrollLock(this, true);
            this.initState();
        }
    },
    beforeUnmount() {
        document.removeEventListener('keydown', this.onKeydown);
        releaseBodyScrollLock(this);
    },
    methods: {
        onKeydown(e) {
            if (e.key === 'Escape' && this.show) this.close();
        },
        initState() {
            this.resetState();
            const first = this.activeAttachments[0];
            if (first) this.onAttachmentChange(first.id);
        },
        attachmentLabel(attachment) {
            const name = attachment.unique_attachment_display_name
                || attachment.attachment_display_name
                || attachment.unique_attachment_title
                || attachment.attachment_name
                || `Вложение №${attachment.id}`;
            const from = humanDate(attachment.entry_date_from);
            const to = humanDate(attachment.entry_date_to);
            if (from && to) return `${name} (${from === to ? from : `${from} - ${to}`})`;
            return name;
        },
        onAttachmentChange(id) {
            this.selectedAttachmentId = id;
            this.sortField = null;
            this.sortDirection = null;
            // Гасим предупреждения прошлого вложения: новая форма пришлёт свои, а до
            // пересчёта иначе мелькнут чужие - тот же приём, что и в форме подачи.
            this.placeNotices = [];
            const uaId = this.selectedAttachment?.unique_attachment_id;
            if (uaId) this.loadFieldConfig(uaId);
        },
        /**
         * Обязательность и видимость полей у дополнения обязаны совпасть с подачей -
         * иначе строка пройдёт форму и упрётся в серверную проверку шаблона. Источник
         * тот же, что у CreateApplication: GET /attachments/{uaId}/field-config.
         */
        async loadFieldConfig(uniqueAttachmentId) {
            if (this.fieldConfigByAttachment[uniqueAttachmentId]) return;
            // Быстрое переключение вложений пускает несколько загрузок; пишем только
            // ответ последнего выбора, иначе форма покажет чужую настройку.
            const seq = ++this.fieldConfigSeq;
            this.loadingFieldConfig = true;
            try {
                const { getFieldConfig } = await import('@/api/attachment-templates');
                const data = await getFieldConfig(uniqueAttachmentId);
                const base = Array.isArray(data?.base) ? data.base : [];
                const map = {};
                base.forEach((f) => {
                    map[f.key] = {
                        visible: f.visible,
                        required: f.required,
                        locked: f.locked,
                        requirable: f.requirable,
                    };
                });
                this.fieldConfigByAttachment[uniqueAttachmentId] = map;
            } catch {
                // Конфиг недоступен - деградируем к дефолту (все поля видимы), как в подаче.
                // Пустой объект falsy для проверки выше -> следующий выбор повторит запрос.
                this.fieldConfigByAttachment[uniqueAttachmentId] = {};
            } finally {
                if (seq === this.fieldConfigSeq) this.loadingFieldConfig = false;
            }
        },
        rowsBucket() {
            if (!this.rowsByAttachment[this.selectedAttachmentId]) {
                this.rowsByAttachment[this.selectedAttachmentId] = [];
            }
            return this.rowsByAttachment[this.selectedAttachmentId];
        },
        addRow(row) {
            if (!this.selectedAttachmentId) return;
            this.rowsBucket().push({ ...row, id: this.rowIdCounter++, isExisting: false });
        },
        addRows(rows) {
            (rows || []).forEach((row) => this.addRow(row));
        },
        updateRow(updated) {
            const rows = this.rowsByAttachment[this.selectedAttachmentId];
            if (!rows) return;
            const index = rows.findIndex((r) => r.id === updated.id);
            if (index !== -1) rows.splice(index, 1, updated);
        },
        editRow(row) {
            const form = this.$refs.vehicleForm || this.$refs.employeeForm || this.$refs.itemsForm;
            if (!form) return;
            if (form.editVehicle) form.editVehicle(row);
            else if (form.editEmployee) form.editEmployee(row);
            else if (form.editItem) form.editItem(row);
        },
        deleteRow(row) {
            const rows = this.rowsByAttachment[this.selectedAttachmentId];
            if (!rows) return;
            const id = typeof row === 'object' ? row.id : row;
            const index = rows.findIndex((r) => r.id === id);
            if (index !== -1) rows.splice(index, 1);
        },
        sortBy(field) {
            if (this.sortField === field) {
                this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
                return;
            }
            this.sortField = field;
            this.sortDirection = 'asc';
        },
        buildAdditions() {
            const additions = [];
            for (const attachment of this.activeAttachments) {
                const rows = this.rowsByAttachment[attachment.id] || [];
                if (!rows.length) continue;
                additions.push({
                    attachment_id: attachment.id,
                    ...toAttachmentContent(attachment.attachment_type, rows),
                });
            }
            return additions;
        },
        async submit() {
            if (!this.canSubmit) return;
            const additions = this.buildAdditions();
            if (!additions.length) return;

            this.submitting = true;
            const deletions = useDeletionsStore();
            try {
                const trimmed = this.comment.trim();
                const result = await createSupplement(this.application.id, {
                    comment: trimmed || null,
                    additions,
                });
                deletions.notify({
                    prefix: result.status === SUPPLEMENT_MERGED
                        ? 'Строки добавлены в заявку '
                        : 'Дополнение отправлено на согласование по заявке ',
                    bold: this.application.application_number,
                    type: 'success',
                });
                this.$emit('submitted', result);
                this.$emit('close');
            } catch (e) {
                deletions.notify({
                    bold: e.status === 409
                        // 409 приходит и на «уже есть незакрытый раунд», и на закрытый статус
                        // заявки - текст бэка уже разводит эти два случая, показываем его.
                        ? e.message
                        : (e.message || 'Не удалось отправить дополнение'),
                    type: 'error',
                });
            } finally {
                this.submitting = false;
            }
        },
        resetState() {
            this.selectedAttachmentId = null;
            this.rowsByAttachment = {};
            this.rowIdCounter = 1;
            this.comment = '';
            this.submitting = false;
            this.loadingFieldConfig = false;
            this.sortField = null;
            this.sortDirection = null;
        },
        // Сброс формы делаем ПОСЛЕ анимации закрытия (after-leave), а не в close() до
        // эмита - иначе окно пустеет за кадр до угасания оверлея.
        close() {
            this.$emit('close');
        },
    },
};
</script>

<style scoped>
.supp-overlay {
    position: fixed;
    inset: 0;
    background: var(--overlay);
    display: flex;
    align-items: center;
    justify-content: center;
    /* Выше панели детали заявки (10002), из которой окно открывается. */
    z-index: 10010;
    padding: 20px;
}

.supp-modal {
    background: var(--surface);
    border-radius: 30px;
    width: 940px;
    max-width: 95%;
    max-height: calc(var(--app-vh, 1vh) * 90);
    display: flex;
    flex-direction: column;
    box-shadow: 0 10px 30px var(--shadow-drop);
    overflow: hidden;
}

.sheet-handle {
    display: none;
    width: 40px;
    height: 4px;
    border-radius: 2px;
    background: var(--border);
    margin: 6px auto 0;
    flex-shrink: 0;
}

.supp-modal__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 24px 28px 16px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
}

.supp-modal__title {
    margin: 0;
    font-size: 20px;
    font-weight: 700;
    color: var(--text);
}

.supp-modal__close {
    border: none;
    background: transparent;
    font-size: 28px;
    line-height: 1;
    cursor: pointer;
    color: var(--text-muted);
    padding: 0 4px;
}

.supp-modal__close:hover {
    color: var(--text);
}

.supp-modal__body {
    padding: 20px 28px;
    overflow-y: auto;
    overscroll-behavior: contain;
    flex: 1 1 auto;
    min-height: 0;
}

.supp-modal__hint {
    margin: 0 0 18px;
    font-size: 13px;
    color: var(--text-muted);
}

.supp-modal__readonly {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 16px;
    margin-bottom: 18px;
}

.supp-modal__field--target {
    margin-bottom: 24px;
}

.supp-modal__label {
    display: block;
    margin-bottom: 6px;
    font-size: 13px;
    font-weight: 500;
    color: var(--text);
}

.supp-modal__req {
    color: var(--accent-text);
}

/* Поля вложения показываем, но менять нельзя: они принадлежат вложению, а не добавке. */
.supp-modal__static {
    background: var(--surface-2);
    color: var(--text-muted);
    cursor: default;
}

.supp-modal__note {
    margin: 6px 0 0;
    font-size: 12px;
    color: var(--danger-text);
}

.supp-modal__other {
    margin: 16px 0 0;
    padding: 10px 14px;
    background: var(--accent-tint);
    border-radius: var(--radius-md);
    font-size: 13px;
    color: var(--text);
}

/* Пока едет настройка полей, форма приглушена: иначе на кадр показываются поля,
   скрытые шаблоном, и человек начинает заполнять то, чего в бланке нет. */
.supp-modal__form--busy {
    opacity: 0.6;
    pointer-events: none;
}

/* Формы заявки - левая колонка двухколоночной раскладки (width 450px + border-right
   под список справа). В окне список идёт под формой, поэтому растягиваем на всю
   ширину и оформляем блок формы отдельной карточкой (как в ManualAddModal). */
.supp-modal :deep(.data__completion) {
    width: 100%;
    box-sizing: border-box;
    border-right: none;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 16px 18px;
}

/* Гриды чипов заточены под 425px-колонку заявки - снимаем кап, чтобы места
   разгрузки и таблицы проезда занимали всю ширину формы. */
.supp-modal :deep(.unloading__grid),
.supp-modal :deep(.passage__grid) {
    max-width: 100%;
}

.supp-modal :deep(.completion__fields) {
    gap: 12px;
}

.supp-modal :deep(.completion__number) {
    flex: 0 0 202px;
    max-width: 202px;
}

.supp-modal :deep(.completion__mark) {
    flex: 0 1 320px;
    max-width: 320px;
}

/* Меню марки в заявке открывается справа от поля (там узкая колонка формы). В окне
   поле широкое и его правый край у края окна - открываем меню вниз, по ширине поля. */
.supp-modal :deep(.mark__dropdown-menu) {
    top: calc(100% + 6px);
    left: 0;
    right: auto;
    margin-left: 0;
    width: 100%;
}

.supp-modal__footer {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px 28px 24px;
    border-top: 1px solid var(--border);
    flex-shrink: 0;
}

.supp-modal__comment {
    min-height: 56px;
}

.supp-modal__actions {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
}

.supp-fade-enter-active {
    transition: opacity 0.2s ease-out;
}

.supp-fade-leave-active {
    transition: opacity 0.2s ease;
}

.supp-fade-enter-from,
.supp-fade-leave-to {
    opacity: 0;
}

.supp-fade-enter-active .supp-modal {
    transition: transform 0.2s ease-out;
}

.supp-fade-enter-from .supp-modal {
    transform: translateY(20px);
}

/* Лист идёт за пальцем 1:1 - на время жеста transition снимаем. */
.supp-modal.is-dragging {
    transition: none;
}

@media (max-width: 768px) {
    .supp-modal {
        /* Лист фиксированной высоты: набор строк меняет высоту содержимого, и на
           «по контенту» лист подпрыгивал бы после каждого добавления. Глобальное
           правило App.vue уже прижимает его к низу и запускает выезд снизу - своя
           enter-анимация спорила бы с ним, оставляем одну, глобальную. */
        height: 90dvh;
    }

    .sheet-handle {
        display: block;
    }

    .supp-modal__header {
        padding: 6px 16px 10px;
    }

    .supp-modal__title {
        font-size: 16px;
    }

    .supp-modal__body {
        padding: 14px 16px;
    }

    .supp-modal__footer {
        padding: 12px 16px 16px;
    }

    .supp-modal__readonly {
        grid-template-columns: 1fr;
    }

    .supp-fade-enter-active .supp-modal {
        transition: none;
    }

    .supp-fade-enter-from .supp-modal {
        transform: none;
    }

    /* Уход листа вниз: inline-transform свайпа перебивает переход, второго слайда нет. */
    .supp-fade-leave-active .supp-modal {
        animation: none;
        transition: transform 0.24s cubic-bezier(0.32, 0.72, 0, 1);
    }

    .supp-fade-leave-to .supp-modal {
        transform: translateY(100%);
    }

    .supp-modal :deep(.completion__number),
    .supp-modal :deep(.completion__mark) {
        flex: 1 1 100%;
        max-width: 100%;
    }
}
</style>
