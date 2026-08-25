<template>
  <BaseModal
    :show="show"
    width="min(1600px, 95vw)"
    closable
    content-class="te-modal-rounded"
    :close-on-overlay="!rebindingMode"
    @close="onClose"
  >
    <template #header>
      <div class="te-header">
        <h3 class="te-header__title">
          Настройка генерации Excel-бланка
        </h3>
        <div class="te-header__toggles">
          <ToggleSwitch
            :model-value="enabled"
            @update:model-value="onToggleEnabled"
          >
            Генерация бланка
          </ToggleSwitch>
          <ToggleSwitch
            v-if="enabled && template && template.file_path"
            v-model="showPaths"
          >
            Отображать пути
          </ToggleSwitch>
        </div>
      </div>
    </template>
    <div
      ref="modalBody"
      class="te-modal-body"
    >
      <!-- Баннер привязки/перепривязки -->
      <transition name="te-banner-fade">
        <div
          v-if="rebindingMode"
          class="te-action-banner te-action-banner--warning"
        >
          <span>
            Перепривязка: <strong>{{ getFieldLabel(rebindMapping.field_path) }}</strong>
            ({{ rebindMapping.old_cell_ref }}) - кликните на новую ячейку
          </span>
          <button
            class="lk-button lk-button--ghost te-btn-sm"
            @click="cancelRebind"
          >
            Отмена
          </button>
        </div>
        <div
          v-else-if="pendingFieldPath"
          class="te-action-banner"
          :class="pendingIsForeignList ? 'te-action-banner--warning' : 'te-action-banner--info'"
        >
          <span>
            Привязка: <strong>{{ pendingFieldLabel }}</strong> - кликните на ячейку в документе
            <template v-if="pendingIsForeignList">
              . Это поле другого типа вложения, в бланке оно останется пустым
            </template>
            <template v-else-if="pendingIsListField">
              . Поле списка: строки берутся из диапазона списка, важна только колонка ячейки
            </template>
          </span>
          <button
            class="lk-button lk-button--ghost te-btn-sm"
            @click="pendingFieldPath = ''; pendingFieldLabel = ''"
          >
            Отмена
          </button>
        </div>
      </transition>

      <!-- SVG линии поверх всего -->
      <svg
        v-if="showPaths || pathsAnimatingOut"
        ref="pathSvg"
        class="te-path-overlay"
        :class="{ 'te-paths-leaving': pathsAnimatingOut }"
        :viewBox="`0 0 ${svgWidth} ${svgHeight}`"
      >
        <path
          v-for="line in pathLines"
          :key="line.id + '-hit'"
          :d="line.d"
          class="te-path-hitarea"
          @mouseenter="onPathHover(line)"
          @mouseleave="onPathLeave"
          @click="onPathClick(line, $event)"
        />
        <path
          v-for="(line, idx) in pathLines"
          :key="line.id"
          :d="line.d"
          class="te-path-line"
          :class="pathLineClasses(line, idx)"
          :style="{ stroke: line.color, opacity: line.opacity }"
        />
        <circle
          v-for="(line, idx) in pathLines"
          :key="line.id + '-dot-l'"
          :cx="line.x2"
          :cy="line.y2"
          r="3"
          :fill="line.color"
          class="te-path-dot"
          :class="pathDotClasses(line, idx)"
          :style="{ opacity: line.opacity }"
        />
        <circle
          v-for="(line, idx) in pathLines"
          :key="line.id + '-dot-r'"
          :cx="line.x1"
          :cy="line.y1"
          r="3"
          :fill="line.color"
          class="te-path-dot"
          :class="pathDotClasses(line, idx)"
          :style="{ opacity: line.opacity }"
        />
        <!-- Кнопка удаления на пути -->
        <transition name="te-popup-fade">
          <foreignObject
            v-if="pathPopup"
            :key="pathPopup.fieldPath + pathPopup.cellRef"
            :x="pathPopup.x - 36"
            :y="pathPopup.y - 14"
            width="72"
            height="28"
            style="pointer-events: auto;"
            :class="{ 'te-popup-faded': hoveredFieldPath && hoveredFieldPath !== pathPopup.fieldPath }"
          >
            <button
              class="te-path-delete-btn"
              @click="confirmPathDelete"
            >
              Удалить
            </button>
          </foreignObject>
        </transition>
      </svg>

      <!-- Левая панель: превью документа -->
      <div
        ref="previewPanel"
        class="te-preview-panel"
        @click="pathPopup = null; removePopupField = ''"
      >
        <transition
          name="te-content-fade"
          mode="out-in"
        >
          <div
            v-if="loadingTemplate"
            key="loading"
            class="te-preview-empty"
          >
            <span class="te-spinner" />
          </div>
          <XlsxViewer
            v-else-if="enabled && templateFileBuffer"
            key="viewer"
            ref="xlsxViewer"
            :file-buffer="templateFileBuffer"
            :mappings="enrichedMappings"
            :selected-cell="activeCellRef"
            :cell-colors="cellColorMap"
            :concat-separator="concatSeparator"
            @cell-click="onCellClick"
            @cell-hover="onCellHover"
          />
          <div
            v-else
            key="empty"
            class="te-preview-empty"
          >
            <span>Загрузите шаблон .xlsx для настройки генерации бланка</span>
          </div>
        </transition>
      </div>

      <!-- Правая панель: настройки -->
      <div
        ref="settingsPanel"
        class="te-settings-panel"
        @click="onSettingsClick"
      >
        <!-- Поля для привязки (в самом верху) -->
        <div
          v-if="enabled && template && template.file_path"
          class="te-field-picker"
        >
          <div class="te-picker-header">
            <h4>Привязка полей системы</h4>
            <div class="te-search-wrap">
              <AppIcon
                name="search"
                class="te-search-icon"
              />
              <input
                v-model="searchQuery"
                type="text"
                class="lk-input te-search-input"
                placeholder="Поиск..."
              >
            </div>
          </div>
          <div
            ref="fieldPickerScroll"
            class="te-field-picker-scroll"
          >
            <transition
              name="te-content-fade"
              mode="out-in"
            >
              <div
                v-if="loadingFields"
                key="fields-loading"
                class="te-fields-loading"
              >
                <span class="te-spinner" />
              </div>
              <div
                v-else
                key="fields-list"
              >
                <div
                  v-for="g in filteredFieldGroups"
                  :key="g.group"
                  class="te-field-group"
                >
                  <span class="te-field-group-label">{{ g.label }}</span>
                  <div class="te-field-chips">
                    <button
                      v-for="f in g.fields"
                      :key="f.path"
                      :data-field-path="f.path"
                      class="te-field-chip"
                      :class="{
                        active: pendingFieldPath === f.path,
                        used: fieldPathUsed(f.path),
                      }"
                      :style="chipStyle(f.path)"
                      @click="selectField(f)"
                      @mouseenter="onChipHover(f.path)"
                      @mouseleave="onChipHover('')"
                    >
                      <span class="te-chip-label">{{ f.label }}</span>
                      <span
                        v-if="fieldPathUsed(f.path)"
                        class="te-chip-ref"
                      >
                        {{ fieldCellRefs(f.path) }}
                      </span>
                      <span
                        v-if="fieldPathUsed(f.path)"
                        class="te-chip-remove"
                        @click.stop="onChipRemoveClick(f.path)"
                      >
                        &times;
                      </span>
                      <div
                        v-if="removePopupField === f.path"
                        class="te-remove-popup"
                        @click.stop
                      >
                        <div
                          v-for="rm in fieldMappingEntries(f.path)"
                          :key="rm.idx"
                          class="te-remove-popup__item"
                          @click="removeMapping(rm.idx); removePopupField = ''"
                        >
                          {{ rm.cell_ref }} <span class="te-remove-popup__x">&times;</span>
                        </div>
                        <div
                          class="te-remove-popup__all"
                          @click="removeMappingsByPath(f.path); removePopupField = ''"
                        >
                          Удалить все
                        </div>
                      </div>
                    </button>
                  </div>
                </div>
                <div
                  v-if="filteredFieldGroups.length === 0"
                  class="te-no-results"
                >
                  Поля не найдены
                </div>
              </div>
            </transition>
          </div>
        </div>

        <!-- Файл шаблона -->
        <div
          v-if="enabled"
          class="te-section"
        >
          <!-- Шаблоны: dropdown + действия -->
          <div
            v-if="allTemplates.length > 0 && !showUpload"
            class="te-templates-block"
          >
            <div class="te-template-dropdown-wrap">
              <button
                class="te-template-dropdown-trigger"
                @click="templateDropdownOpen = !templateDropdownOpen"
              >
                <span class="te-dropdown-filename">{{ template?.original_file_name || 'template.xlsx' }}</span>
                <span
                  class="te-dropdown-arrow"
                  :class="{ open: templateDropdownOpen }"
                >&#9662;</span>
              </button>
              <transition name="te-dropdown-fade">
                <div
                  v-if="templateDropdownOpen"
                  class="te-template-dropdown"
                >
                  <button
                    v-for="tmpl in allTemplates"
                    :key="tmpl.id"
                    class="te-dropdown-item"
                    :class="{ active: tmpl.id === template?.id }"
                    @click="onDropdownSelectTemplate(tmpl)"
                  >
                    {{ tmpl.original_file_name || 'template.xlsx' }}
                  </button>
                  <button
                    class="te-dropdown-item te-dropdown-add"
                    @click="showUpload = true; templateDropdownOpen = false"
                  >
                    Добавить файл..
                  </button>
                </div>
              </transition>
            </div>
            <div class="te-file-actions">
              <button
                class="lk-button lk-button--ghost te-btn-sm"
                @click="downloadCurrentTemplate"
              >
                Скачать
              </button>
              <button
                class="lk-button lk-button--ghost te-btn-sm"
                @click="showUpload = true"
              >
                Редактировать
              </button>
              <button
                class="lk-button lk-button--danger te-btn-sm"
                @click="onDeleteTemplate"
              >
                Удалить
              </button>
            </div>
          </div>

          <!-- Границы строк списка: правятся без перезагрузки файла -->
          <div
            v-if="template && template.file_path && !showUpload"
            class="te-params-block"
          >
            <div class="te-params-fields">
              <div class="te-form-field">
                <label>Список с</label>
                <input
                  v-model.number="form.listStartRow"
                  type="number"
                  min="1"
                  class="lk-input te-compact-input"
                  data-testid="template-list-start"
                >
              </div>
              <div class="te-form-field">
                <label>Список по</label>
                <input
                  v-model.number="form.listEndRow"
                  type="number"
                  min="1"
                  class="lk-input te-compact-input"
                  data-testid="template-list-end"
                >
              </div>
              <div class="te-form-field">
                <label>Макс. строк</label>
                <input
                  v-model.number="form.maxListRows"
                  type="number"
                  min="0"
                  class="lk-input te-compact-input"
                  placeholder="авто"
                  data-testid="template-list-max"
                >
              </div>
              <div
                v-if="showItemsRange"
                class="te-form-field"
              >
                <label>Строк ТМЦ</label>
                <input
                  v-model.number="form.itemsMaxListRows"
                  type="number"
                  min="0"
                  class="lk-input te-compact-input"
                  placeholder="нет"
                  data-testid="template-items-rows"
                >
              </div>
            </div>
            <button
              class="lk-button lk-button--ghost te-btn-sm"
              :disabled="savingParams || !listRangeChanged"
              data-testid="template-params-save"
              @click="saveParams"
            >
              {{ savingParams ? 'Сохранение...' : 'Сохранить' }}
            </button>
          </div>

          <!-- Настройка разделителя (для совмещения полей) -->
          <div
            v-if="template && template.file_path && !showUpload && hasCombinedCells"
            class="te-separator-block"
          >
            <label class="te-separator-label">Разделитель совмещенных полей:</label>
            <input
              v-model="concatSeparator"
              type="text"
              class="lk-input te-separator-input"
              placeholder=", "
            >
          </div>

          <!-- Drag & Drop загрузка -->
          <div
            v-if="!template || !template.file_path || showUpload"
            class="te-upload-area"
          >
            <div
              class="te-dropzone"
              :class="{ 'te-dropzone--active': isDragging, 'te-dropzone--has-file': form.file }"
              @dragenter.prevent="isDragging = true"
              @dragover.prevent
              @dragleave.prevent="isDragging = false"
              @drop.prevent="onDrop"
            >
              <div
                v-if="form.file"
                class="te-dropzone__file"
              >
                <span class="te-dropzone__filename">{{ form.file.name }}</span>
                <button
                  class="te-dropzone__clear"
                  @click.stop="form.file = null"
                >
                  &times;
                </button>
              </div>
              <div
                v-else
                class="te-dropzone__placeholder"
              >
                <span class="te-dropzone__hint">Перетащите .xlsx файл сюда</span>
                <span class="te-dropzone__or">или</span>
                <label class="lk-button lk-button--ghost te-btn-sm te-dropzone__browse">
                  Выберите файл
                  <input
                    type="file"
                    accept=".xlsx"
                    hidden
                    @change="onFileChange"
                  >
                </label>
              </div>
            </div>
            <div class="te-upload-fields">
              <div class="te-form-field">
                <label>Список с</label>
                <input
                  v-model.number="form.listStartRow"
                  type="number"
                  min="1"
                  class="lk-input te-compact-input"
                  required
                >
              </div>
              <div class="te-form-field">
                <label>Список по</label>
                <input
                  v-model.number="form.listEndRow"
                  type="number"
                  min="1"
                  class="lk-input te-compact-input"
                  required
                >
              </div>
              <div class="te-form-field">
                <label>Макс. строк</label>
                <input
                  v-model.number="form.maxListRows"
                  type="number"
                  min="0"
                  class="lk-input te-compact-input"
                  placeholder="авто"
                >
              </div>
            </div>
            <div class="te-upload-actions">
              <button
                v-if="template && template.file_path"
                type="button"
                class="lk-button lk-button--ghost te-btn-sm"
                @click="showUpload = false"
              >
                Отмена
              </button>
              <button
                class="lk-button lk-button--primary te-btn-sm"
                :disabled="!form.file || uploading"
                @click="onUpload"
              >
                {{ uploading ? 'Загрузка...' : 'Загрузить' }}
              </button>
            </div>
          </div>
        </div>

        <!-- Привязки (collapsible) -->
        <div
          v-if="enabled && template && template.file_path"
          class="te-mappings-section"
        >
          <div
            v-if="foreignListMappings.length"
            class="te-foreign-warning"
          >
            В списке есть {{ foreignListMappings.length }} привязк{{ foreignListMappings.length === 1 ? 'а' : 'и' }}
            из другой группы полей ({{ foreignListMappings.map(m => getFieldLabel(m.field_path)).join(', ') }}).
            У этого типа вложения такие данные не заполняются - ячейки останутся пустыми.
          </div>
          <div
            v-if="itemsMappingsWithoutRange.length"
            class="te-foreign-warning"
            data-testid="template-items-range-hint"
          >
            Привязки к ТМЦ есть ({{ itemsMappingsWithoutRange.map(m => getFieldLabel(m.field_path)).join(', ') }}),
            но число строк под таблицу не задано - ввозимый товар в бланк не попадёт.
            Укажите «Строк под таблицу ТМЦ» выше и нажмите «Сохранить».
          </div>
          <button
            class="te-section-toggle"
            @click="showMappings = !showMappings"
          >
            <h4>Привязки</h4>
            <span class="te-mappings-count">{{ mappings.length }}</span>
            <span
              class="te-chevron"
              :class="{ open: showMappings }"
            >&#9656;</span>
          </button>
          <div
            v-show="showMappings"
            class="te-mappings-body"
          >
            <div
              v-if="!mappings.length"
              class="te-empty-state"
            >
              Нет привязок
            </div>
            <div
              v-for="(m, idx) in enrichedMappings"
              :key="idx"
              class="te-mapping-row"
              :class="{ highlight: activeCellRef === m.cell_ref }"
            >
              <span class="te-mapping-cell">{{ m.cell_ref }}</span>
              <span
                v-if="m.cellTotal > 1"
                class="te-mapping-order"
                :data-hint="`Порядок склейки в ${m.cell_ref}: ${m.cellIndex} из ${m.cellTotal}`"
              >{{ m.cellIndex }}/{{ m.cellTotal }}</span>
              <span class="te-mapping-field">{{ m.fieldLabel || m.field_path }}</span>
              <span
                v-if="m.cellTotal > 1"
                class="te-mapping-move"
              >
                <button
                  class="te-mapping-move-btn"
                  :disabled="m.cellIndex === 1"
                  title="Раньше в склейке"
                  data-testid="mapping-move-up"
                  @click="moveMappingInCell(idx, -1)"
                >&#9650;</button>
                <button
                  class="te-mapping-move-btn"
                  :disabled="m.cellIndex === m.cellTotal"
                  title="Позже в склейке"
                  data-testid="mapping-move-down"
                  @click="moveMappingInCell(idx, 1)"
                >&#9660;</button>
              </span>
              <span
                v-if="m.is_list_field"
                class="te-list-badge"
              >список</span>
              <span
                v-else-if="m.repeatsInList"
                class="te-list-badge te-list-badge--repeat"
              >в каждой строке</span>
              <button
                class="te-mapping-remove"
                title="Удалить привязку"
                @click="removeMapping(idx)"
              >
                &times;
              </button>
            </div>
          </div>
        </div>

        <!-- Сохранить -->
        <div
          v-if="enabled && template && template.file_path"
          class="te-save-area"
        >
          <button
            class="lk-button lk-button--ghost"
            data-testid="template-copy-open"
            @click="showCopyModal = true"
          >
            Скопировать привязки
          </button>
          <button
            class="lk-button lk-button--primary"
            :disabled="savingMappings"
            @click="saveMappings"
          >
            {{ savingMappings ? 'Сохранение...' : 'Сохранить привязки' }}
          </button>
        </div>

        <AttachmentMappingCopyModal
          v-if="template && template.file_path"
          :show="showCopyModal"
          :unique-attachment-id="uniqueAttachmentId"
          :attachment-type="attachmentType"
          :current-mappings-count="mappings.length"
          :current-template-id="template.id"
          :target-file-name="template.original_file_name"
          :unsaved-changes="hasUnsavedMappings"
          @close="showCopyModal = false"
          @copied="onMappingsCopied"
        />
      </div>
    </div>
  </BaseModal>
</template>

<script>
import { useDeletionsStore } from '@/stores/deletions';
import {
  getTemplate, uploadTemplate, updateMappings, updateTemplateParams, deleteTemplate,
  getTemplateFields, getTemplateFile, saveBlobAs,
  listTemplates, setActiveTemplate, deleteTemplateByID, getTemplateFileByID,
  deactivateAllTemplates,
} from '@/api/attachment-templates';
import BaseModal from '@/components/ui/BaseModal.vue';
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue';
import AttachmentMappingCopyModal from './AttachmentMappingCopyModal.vue';
import XlsxViewer from './XlsxViewer.vue';
import AppIcon from '@/components/icons/AppIcon.vue';

const PATH_COLORS = [
  '#4F5BDF', '#e85d75', '#2e9e5a', '#e8a317', '#8e44ad',
  '#16a085', '#c0392b', '#2980b9', '#d35400', '#e06090',
  '#1abc9c', '#6c5ce7', '#00b894', '#e17055', '#0984e3',
  '#b8255f', '#299438', '#6accbc', '#af38eb', '#ff9933',
];

function getPathColor(index) {
  if (index < PATH_COLORS.length) return PATH_COLORS[index];
  const hue = (index * 137.508) % 360;
  if (hue > 180 && hue < 240) return `hsl(${(hue + 80) % 360}, 65%, 45%)`;
  return `hsl(${hue}, 65%, 45%)`;
}

const GROUP_ORDER = [
  'application', 'attachment', 'app_items', 'employee', 'car', 'item', 'custom',
];

export default {
  name: 'AttachmentTemplateEditor',
  components: {
    AppIcon,
    BaseModal, ToggleSwitch, XlsxViewer, AttachmentMappingCopyModal,
  },
  props: {
    show: { type: Boolean, required: true },
    uniqueAttachmentId: { type: Number, required: true },
    // Тип вложения задаёт, какая группа полей попадает в строки списка: у бланка
    // имущества привязка car.* останется пустой (#1454).
    attachmentType: { type: String, default: '' },
  },
  emits: ['close'],
  data() {
    return {
      template: null,
      allTemplates: [],
      mappings: [],
      fieldGroups: [],
      enabled: false,
      showUpload: false,
      isDragging: false,
      form: {
        file: null, listStartRow: 1, listEndRow: 1, maxListRows: 0,
        itemsMaxListRows: 0,
      },
      uploading: false,
      savingMappings: false,
      savingParams: false,
      concatSeparator: ', ',
      loadingTemplate: false,
      loadingFields: false,
      templateFileBuffer: null,
      pendingFieldPath: '',
      pendingFieldLabel: '',
      pendingCellRef: '',
      searchQuery: '',
      activeCategory: '',
      showPaths: false,
      showMappings: false,
      hoveredFieldPath: '',
      pathPopup: null,
      removePopupField: '',
      rebindingMode: false,
      rebindMapping: null,
      pathLines: [],
      pathsAnimatingOut: false,
      hoveredPathIndex: null,
      hoveredCellRef: '',
      templateDropdownOpen: false,
      showCopyModal: false,
      savedMappingsKey: '',
      svgWidth: 0,
      svgHeight: 0,
      rafId: null,
    };
  },
  computed: {
    enrichedMappings() {
      const seen = {};
      const total = {};
      for (const m of this.mappings) total[m.cell_ref] = (total[m.cell_ref] || 0) + 1;
      return this.mappings.map(m => {
        seen[m.cell_ref] = (seen[m.cell_ref] || 0) + 1;
        return {
          ...m,
          fieldLabel: this.getFieldLabel(m.field_path),
          repeatsInList: this.repeatsInList(m),
          // Позиция в ячейке = порядок склейки: бланк соединяет поля в этом порядке.
          cellIndex: seen[m.cell_ref],
          cellTotal: total[m.cell_ref],
        };
      });
    },
    filteredFieldGroups() {
      let groups = this.fieldGroups.map(g => ({
        ...g,
        fields: [...g.fields].sort((a, b) => a.label.localeCompare(b.label, 'ru')),
      }));
      groups.sort((a, b) => {
        const ai = GROUP_ORDER.indexOf(a.group);
        const bi = GROUP_ORDER.indexOf(b.group);
        return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi);
      });
      if (this.activeCategory) {
        groups = groups.filter(g => g.group === this.activeCategory);
      }
      if (this.searchQuery) {
        const q = this.searchQuery.toLowerCase();
        groups = groups.map(g => ({
          ...g,
          fields: g.fields.filter(f =>
            f.label.toLowerCase().includes(q) ||
            f.path.toLowerCase().includes(q)
          ),
        })).filter(g => g.fields.length > 0);
      }
      return groups;
    },
    activeCellRef() {
      if (this.rebindingMode && this.rebindMapping) return this.rebindMapping.old_cell_ref;
      return this.pendingCellRef;
    },
    hasCombinedCells() {
      const cellCounts = {};
      for (const m of this.mappings) {
        cellCounts[m.cell_ref] = (cellCounts[m.cell_ref] || 0) + 1;
      }
      return Object.values(cellCounts).some(c => c > 1);
    },
    // Группа полей, которая реально попадает в строки списка этого бланка.
    listGroupForType() {
      return { cars: 'car', people: 'employee', items: 'item' }[this.attachmentType] || '';
    },
    // Верхняя строка с привязками ТМЦ - с неё начинается таблица ввозимого товара.
    // Отдельного поля под неё нет: привязка и есть указание места.
    itemsSectionStart() {
      const rows = this.mappings
        .filter(m => m.field_path.startsWith('item.'))
        .map(m => Number((/^[A-Za-z]+(\d+)$/.exec(String(m.cell_ref || '').trim()) || [])[1]))
        .filter(n => n > 0);
      return rows.length ? Math.min(...rows) : 0;
    },
    // Настройку таблицы ТМЦ показываем, только когда она нужна: поля имущества
    // привязаны, а бланк не про сам ввоз (там ТМЦ заполняют собственный список).
    showItemsRange() {
      return this.attachmentType !== 'items' && this.itemsSectionStart > 0;
    },
    // Число строк задано - привязки item.* в этом бланке рабочие.
    itemsRangeSet() {
      return this.form.itemsMaxListRows > 0;
    },
    // Привязка поля-списка из чужой группы: значений у неё не будет, потому что
    // источник (машины/сотрудники/ТМЦ) принадлежит другому типу вложения. Исключение -
    // item.* при заданной таблице ТМЦ: она заполняется «Заявками на ввоз» заявки.
    foreignListMappings() {
      if (!this.listGroupForType) return [];
      return this.mappings.filter(
        m => m.is_list_field
          && !m.field_path.startsWith(`${this.listGroupForType}.`)
          && !(this.itemsRangeSet && m.field_path.startsWith('item.'))
      );
    },
    // Привязки к ТМЦ есть, а строки таблицы не заданы - в бланке останется пустое место.
    itemsMappingsWithoutRange() {
      if (!this.showItemsRange || this.itemsRangeSet) return [];
      return this.mappings.filter(m => m.is_list_field && m.field_path.startsWith('item.'));
    },
    // Локальные правки привязок, ещё не отправленные на сервер: перенос с другого
    // шаблона идёт по серверному состоянию, поэтому о них надо предупредить.
    hasUnsavedMappings() {
      return this.mappingsKey(this.mappings) !== this.savedMappingsKey;
    },
    pendingIsForeignList() {
      if (!this.pendingFieldPath || !this.listGroupForType) return false;
      if (this.itemsRangeSet && this.pendingFieldPath.startsWith('item.')) return false;
      return this.isListField(this.pendingFieldPath)
        && !this.pendingFieldPath.startsWith(`${this.listGroupForType}.`);
    },
    listRangeChanged() {
      if (!this.template) return false;
      return this.form.listStartRow !== this.template.list_start_row
        || this.form.listEndRow !== this.template.list_end_row
        || this.form.maxListRows !== (this.template.max_list_rows || 0)
        || this.form.itemsMaxListRows !== (this.template.items_max_list_rows || 0);
    },
    pendingIsListField() {
      return !!this.pendingFieldPath && this.isListField(this.pendingFieldPath);
    },
    cellColorMap() {
      if (!this.showPaths) return new Map();
      const map = new Map();
      this.mappings.forEach((m, i) => {
        const color = getPathColor(i);
        map.set(m.cell_ref.toUpperCase(), color);
      });
      return map;
    },
  },
  watch: {
    show(val) {
      if (val) {
        this.templateFileBuffer = null;
        this.template = null;
        this.allTemplates = [];
        this.mappings = [];
        this.loadAll();
      } else {
        this.cleanupPathListeners();
        this.resetState();
        this.templateFileBuffer = null;
      }
    },
    showPaths(val) {
      if (val) {
        this.pathsAnimatingOut = false;
        this.$nextTick(() => this.setupPathListeners());
      } else {
        this.pathsAnimatingOut = true;
        this.pathPopup = null;
        setTimeout(() => {
          this.pathsAnimatingOut = false;
          this.pathLines = [];
          this.cleanupPathListeners();
        }, 800);
      }
    },
    mappings: {
      deep: true,
      handler() {
        if (this.showPaths) this.$nextTick(() => this.updatePaths());
      },
    },
  },
  beforeUnmount() {
    this.cleanupPathListeners();
  },
  methods: {
    onSettingsClick() {
      this.removePopupField = '';
      this.pathPopup = null;
    },
    resetState() {
      this.pendingFieldPath = '';
      this.pendingFieldLabel = '';
      this.pendingCellRef = '';
      this.rebindingMode = false;
      this.rebindMapping = null;
      this.pathPopup = null;
      this.removePopupField = '';
      this.hoveredFieldPath = '';
      this.hoveredPathIndex = null;
      this.hoveredCellRef = '';
      this.templateDropdownOpen = false;
      this.pathLines = [];
      this.pathsAnimatingOut = false;
      this.showPaths = false;
    },
    onClose() {
      if (this.rebindingMode) {
        this.cancelRebind();
        return;
      }
      this.$emit('close');
    },
    async loadAll() {
      await Promise.all([this.loadTemplate(), this.loadFields()]);
    },
    async loadTemplate() {
      this.loadingTemplate = true;
      this.templateFileBuffer = null;
      try {
        const data = await getTemplate(this.uniqueAttachmentId);
        this.template = data;
        // Копия, а не сам массив ответа: правки привязок иначе мутируют
        // template.mappings, и сохранённое состояние становится неотличимо от текущего.
        this.mappings = ((data && data.mappings) || []).map(m => ({ ...m }));
        this.savedMappingsKey = this.mappingsKey(this.mappings);
        this.enabled = !!(data && data.file_path);
        this.form.listStartRow = data && data.list_start_row || 1;
        this.form.listEndRow = data && data.list_end_row || 1;
        this.form.maxListRows = data && data.max_list_rows || 0;
        this.form.itemsMaxListRows = data && data.items_max_list_rows || 0;
        this.concatSeparator = data && data.concat_separator || ', ';
        if (this.enabled) await this.loadTemplateFile();
      } catch {
        this.template = null;
        this.mappings = [];
        this.savedMappingsKey = '';
        this.enabled = false;
        this.templateFileBuffer = null;
      } finally {
        this.loadingTemplate = false;
      }
      try {
        const all = await listTemplates(this.uniqueAttachmentId);
        this.allTemplates = Array.isArray(all) ? all : [];
      } catch {
        this.allTemplates = [];
      }
    },
    async loadTemplateFile() {
      try {
        if (this.template && this.template.id) {
          this.templateFileBuffer = await getTemplateFileByID(this.uniqueAttachmentId, this.template.id);
        } else {
          this.templateFileBuffer = await getTemplateFile(this.uniqueAttachmentId);
        }
      } catch {
        this.templateFileBuffer = null;
      }
    },
    async loadFields() {
      this.loadingFields = true;
      try {
        const data = await getTemplateFields(this.uniqueAttachmentId);
        this.fieldGroups = Array.isArray(data) ? data : [];
      } catch {
        this.fieldGroups = [];
      } finally {
        this.loadingFields = false;
      }
    },
    async onToggleEnabled(val) {
      this.enabled = val;
      try {
        if (!val) {
          await deactivateAllTemplates(this.uniqueAttachmentId);
        } else if (this.allTemplates.length > 0) {
          const latest = this.allTemplates[0];
          await setActiveTemplate(this.uniqueAttachmentId, latest.id);
          await this.loadTemplate();
        }
      } catch {
        this.enabled = !val;
        useDeletionsStore().notify({ bold: 'Не удалось переключить генерацию бланка', type: 'error' });
      }
    },
    onFileChange(e) {
      this.form.file = e.target.files[0] || null;
    },
    onDrop(e) {
      this.isDragging = false;
      const files = e.dataTransfer.files;
      if (files.length > 0 && files[0].name.endsWith('.xlsx')) {
        this.form.file = files[0];
      }
    },
    async onUpload() {
      if (!this.form.file) return;
      this.uploading = true;
      try {
        await uploadTemplate(this.uniqueAttachmentId, this.form.file, {
          listStartRow: this.form.listStartRow,
          listEndRow: this.form.listEndRow,
          maxListRows: this.form.maxListRows,
        });
        useDeletionsStore().notify({ bold: 'Шаблон загружен' });
        this.showUpload = false;
        this.form.file = null;
        await this.loadTemplate();
      } catch (err) {
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить шаблон: ', bold: err.message || 'ошибка сервера', type: 'error' });
      } finally {
        this.uploading = false;
      }
    },
    async onDropdownSelectTemplate(tmpl) {
      this.templateDropdownOpen = false;
      if (tmpl.id === this.template?.id) return;
      await this.switchTemplate(tmpl);
    },
    async switchTemplate(tmpl) {
      if (tmpl.id === this.template?.id) return;
      this.loadingTemplate = true;
      this.loadingFields = true;
      this.templateFileBuffer = null;
      try {
        await setActiveTemplate(this.uniqueAttachmentId, tmpl.id);
        useDeletionsStore().notify({ bold: 'Шаблон активирован' });
        this.resetState();
        await Promise.all([this.loadTemplate(), this.loadFields()]);
      } catch {
        useDeletionsStore().notify({ bold: 'Не удалось переключить шаблон', type: 'error' });
        this.loadingTemplate = false;
        this.loadingFields = false;
      }
    },
    async deleteSpecificTemplate(tmpl) {
      try {
        await deleteTemplateByID(this.uniqueAttachmentId, tmpl.id);
        useDeletionsStore().notify({ bold: 'Шаблон удалён' });
        if (tmpl.id === this.template?.id) {
          this.templateFileBuffer = null;
        }
        await this.loadTemplate();
      } catch {
        useDeletionsStore().notify({ bold: 'Не удалось удалить шаблон', type: 'error' });
      }
    },
    async onDeleteTemplate() {
      try {
        await deleteTemplate(this.uniqueAttachmentId);
        useDeletionsStore().notify({ bold: 'Шаблон удалён' });
        this.templateFileBuffer = null;
        await this.loadTemplate();
      } catch {
        useDeletionsStore().notify({ bold: 'Не удалось удалить шаблон', type: 'error' });
      }
    },
    async downloadCurrentTemplate() {
      try {
        const buf = this.templateFileBuffer || await getTemplateFile(this.uniqueAttachmentId);
        const blob = new Blob([buf], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
        saveBlobAs(blob, this.template.original_file_name || 'template.xlsx');
      } catch {
        useDeletionsStore().notify({ bold: 'Не удалось скачать шаблон', type: 'error' });
      }
    },

    selectField(field) {
      if (this.rebindingMode) return;
      if (this.pendingFieldPath === field.path) {
        this.pendingFieldPath = '';
        this.pendingFieldLabel = '';
      } else {
        this.pendingFieldPath = field.path;
        this.pendingFieldLabel = field.label;
      }
    },
    onCellClick(cellRef) {
      this.pathPopup = null;

      if (this.rebindingMode) {
        this.finishRebind(cellRef);
        return;
      }

      if (this.pendingFieldPath) {
        const duplicate = this.mappings.find(
          m => m.cell_ref === cellRef && m.field_path === this.pendingFieldPath
        );
        if (duplicate) {
          this.pendingFieldPath = '';
          return;
        }
        this.mappings.push({
          cell_ref: cellRef,
          field_path: this.pendingFieldPath,
          is_list_field: this.isListField(this.pendingFieldPath),
        });
        this.pendingFieldPath = '';
        this.pendingFieldLabel = '';
        return;
      }

      const mapped = this.mappings.find(m => m.cell_ref === cellRef);
      if (mapped) {
        this.startRebind(mapped, cellRef);
        return;
      }

      this.pendingCellRef = cellRef;
    },

    startRebind(mapping, cellRef) {
      const idx = this.mappings.findIndex(
        m => m.cell_ref === cellRef && m.field_path === mapping.field_path
      );
      this.rebindingMode = true;
      this.rebindMapping = {
        index: idx,
        field_path: mapping.field_path,
        old_cell_ref: cellRef,
      };
    },
    finishRebind(newCellRef) {
      if (!this.rebindMapping) return;
      if (this.rebindMapping.index >= 0) {
        this.mappings[this.rebindMapping.index].cell_ref = newCellRef;
      }
      this.rebindingMode = false;
      this.rebindMapping = null;
    },
    cancelRebind() {
      this.rebindingMode = false;
      this.rebindMapping = null;
    },

    // Меняет порядок склейки внутри ячейки: переставляем привязку с соседней по той же
    // ячейке, остальные привязки не двигаем.
    moveMappingInCell(idx, dir) {
      const cell = this.mappings[idx] && this.mappings[idx].cell_ref;
      if (!cell) return;
      const step = dir > 0 ? 1 : -1;
      for (let i = idx + step; i >= 0 && i < this.mappings.length; i += step) {
        if (this.mappings[i].cell_ref !== cell) continue;
        const next = [...this.mappings];
        [next[idx], next[i]] = [next[i], next[idx]];
        this.mappings = next;
        return;
      }
    },
    removeMapping(idx) {
      this.mappings.splice(idx, 1);
    },
    onChipRemoveClick(fieldPath) {
      const entries = this.fieldMappingEntries(fieldPath);
      if (entries.length <= 1) {
        this.removeMappingsByPath(fieldPath);
      } else {
        this.removePopupField = this.removePopupField === fieldPath ? '' : fieldPath;
      }
    },
    fieldMappingEntries(fieldPath) {
      return this.mappings
        .map((m, idx) => ({ ...m, idx }))
        .filter(m => m.field_path === fieldPath);
    },
    removeMappingsByPath(fieldPath) {
      this.mappings = this.mappings.filter(m => m.field_path !== fieldPath);
    },
    // Границы берём из сохранённого шаблона, а не из полей ввода: генерация бланка
    // считает по сохранённым, пока админ не нажал «Сохранить».
    repeatsInList(m) {
      if (!m || m.is_list_field || !this.template) return false;
      const start = this.template.list_start_row;
      const end = this.template.list_end_row;
      if (!start || !end || end < start) return false;
      const parsed = /^[A-Za-z]+(\d+)$/.exec(String(m.cell_ref || '').trim());
      if (!parsed) return false;
      const row = Number(parsed[1]);
      return row >= start && row <= end;
    },
    isListField(fieldPath) {
      for (const g of this.fieldGroups) {
        const f = g.fields.find(x => x.path === fieldPath);
        if (f) return !!f.is_list;
      }
      return false;
    },
    fieldPathUsed(path) {
      return this.mappings.some(m => m.field_path === path);
    },
    fieldCellRefs(path) {
      return this.mappings
        .filter(m => m.field_path === path)
        .map(m => m.cell_ref)
        .join(', ');
    },
    getFieldLabel(fieldPath) {
      for (const g of this.fieldGroups) {
        const f = g.fields.find(x => x.path === fieldPath);
        if (f) return f.label;
      }
      return fieldPath;
    },
    getFieldColor(fieldPath) {
      const idx = this.mappings.findIndex(m => m.field_path === fieldPath);
      return idx >= 0 ? getPathColor(idx) : '';
    },
    chipStyle(fieldPath) {
      if (!this.showPaths || !this.fieldPathUsed(fieldPath)) return {};
      return { borderColor: this.getFieldColor(fieldPath) };
    },
    async saveParams() {
      if (this.savingParams) return;
      this.savingParams = true;
      try {
        await updateTemplateParams(this.uniqueAttachmentId, {
          listStartRow: this.form.listStartRow,
          listEndRow: this.form.listEndRow,
          maxListRows: this.form.maxListRows,
          itemsMaxListRows: this.form.itemsMaxListRows,
        });
        useDeletionsStore().notify({ bold: 'Границы списка сохранены' });
        await this.loadTemplate();
      } catch (err) {
        useDeletionsStore().notify({
          prefix: 'Не удалось сохранить границы: ',
          bold: err.message || 'ошибка сервера',
          type: 'error',
        });
      } finally {
        this.savingParams = false;
      }
    },
    mappingsKey(list) {
      return (list || [])
        .map(m => `${m.cell_ref}|${m.field_path}`)
        .sort()
        .join(',');
    },
    async onMappingsCopied() {
      await this.loadTemplate();
    },
    async saveMappings() {
      this.savingMappings = true;
      try {
        const payload = this.mappings.filter(m => m.cell_ref && m.field_path);
        await updateMappings(this.uniqueAttachmentId, payload, this.concatSeparator);
        useDeletionsStore().notify({ bold: 'Привязки сохранены' });
        await this.loadTemplate();
      } catch (err) {
        useDeletionsStore().notify({ prefix: 'Не удалось сохранить: ', bold: err.message || 'ошибка сервера', type: 'error' });
      } finally {
        this.savingMappings = false;
      }
    },

    onChipHover(fieldPath) {
      if (fieldPath && !this.fieldPathUsed(fieldPath)) return;
      this.hoveredFieldPath = fieldPath;
    },
    onCellHover(cellRef) {
      if (!this.showPaths) return;
      if (cellRef && this.mappings.some(m => m.cell_ref === cellRef)) {
        this.hoveredCellRef = cellRef;
      } else {
        this.hoveredCellRef = '';
      }
    },
    onPathHover(line) {
      const idx = this.pathLines.findIndex(l => l.id === line.id);
      this.hoveredPathIndex = idx >= 0 ? idx : null;
    },
    onPathLeave() {
      this.hoveredPathIndex = null;
    },
    isPathHighlighted(line, idx) {
      if (this.hoveredPathIndex === idx) return true;
      if (this.hoveredFieldPath && line.fieldPath === this.hoveredFieldPath) return true;
      if (this.hoveredCellRef && line.cellRef === this.hoveredCellRef) return true;
      return false;
    },
    isPathHidden(line, idx) {
      if (this.hoveredPathIndex !== null) {
        return idx !== this.hoveredPathIndex;
      }
      if (this.hoveredFieldPath) {
        return line.fieldPath !== this.hoveredFieldPath;
      }
      if (this.hoveredCellRef) {
        return line.cellRef !== this.hoveredCellRef;
      }
      if (this.pathPopup) {
        return false;
      }
      return false;
    },
    isPathDimmed(line) {
      if (this.pathPopup && this.hoveredPathIndex === null) {
        const isPopupPath = line.fieldPath === this.pathPopup.fieldPath
          && line.cellRef === this.pathPopup.cellRef;
        return !isPopupPath;
      }
      return false;
    },
    pathLineClasses(line, idx) {
      const highlighted = this.isPathHighlighted(line, idx);
      const hidden = this.isPathHidden(line, idx);
      const dimmed = this.isPathDimmed(line, idx);
      return {
        'te-path-hover-active': highlighted,
        'te-path-hover-hidden': hidden && !highlighted,
        'te-path-dimmed': dimmed && !highlighted && !hidden,
      };
    },
    pathDotClasses(line, idx) {
      const highlighted = this.isPathHighlighted(line, idx);
      const hidden = this.isPathHidden(line, idx);
      const dimmed = this.isPathDimmed(line, idx);
      return {
        'te-path-hover-hidden': (hidden || dimmed) && !highlighted,
      };
    },
    onPathClick(line, e) {
      e.stopPropagation();
      this.hoveredFieldPath = line.fieldPath;
      this.pathPopup = {
        x: (line.x1 + line.x2) / 2,
        y: (line.y1 + line.y2) / 2,
        fieldPath: line.fieldPath,
        cellRef: line.cellRef,
      };
    },
    confirmPathDelete() {
      if (!this.pathPopup) return;
      const idx = this.mappings.findIndex(
        m => m.field_path === this.pathPopup.fieldPath && m.cell_ref === this.pathPopup.cellRef
      );
      if (idx >= 0) this.mappings.splice(idx, 1);
      this.pathPopup = null;
    },

    setupPathListeners() {
      this.cleanupPathListeners();
      const body = this.$refs.modalBody;
      if (!body) return;

      const scrollables = body.querySelectorAll(
        '.te-preview-panel, .xv-table-wrap, .te-field-picker-scroll, .te-settings-panel'
      );
      this._scrollHandler = () => this.updatePaths();
      scrollables.forEach(el =>
        el.addEventListener('scroll', this._scrollHandler, { passive: true })
      );

      this._resizeObserver = new ResizeObserver(this._scrollHandler);
      this._resizeObserver.observe(body);

      this.updatePaths();
    },
    cleanupPathListeners() {
      if (this._scrollHandler) {
        const body = this.$refs.modalBody;
        if (body) {
          const scrollables = body.querySelectorAll(
            '.te-preview-panel, .xv-table-wrap, .te-field-picker-scroll, .te-settings-panel'
          );
          scrollables.forEach(el => el.removeEventListener('scroll', this._scrollHandler));
        }
      }
      if (this._resizeObserver) {
        this._resizeObserver.disconnect();
        this._resizeObserver = null;
      }
      if (this.rafId) {
        cancelAnimationFrame(this.rafId);
        this.rafId = null;
      }
    },
    updatePaths() {
      const body = this.$refs.modalBody;
      const previewPanel = this.$refs.previewPanel;
      const settingsPanel = this.$refs.settingsPanel;
      if (!body || !previewPanel || !settingsPanel) {
        this.pathLines = [];
        return;
      }

      const bodyRect = body.getBoundingClientRect();
      const previewRect = previewPanel.getBoundingClientRect();

      const pickerScroll = this.$refs.fieldPickerScroll;
      const pickerRect = pickerScroll ? pickerScroll.getBoundingClientRect() : null;

      this.svgWidth = bodyRect.width;
      this.svgHeight = bodyRect.height;

      const fadeZone = 30;
      const lines = [];

      this.enrichedMappings.forEach((m, i) => {
        const cellEl = previewPanel.querySelector(`[data-cell-ref="${m.cell_ref}"]`);
        const chipEl = settingsPanel.querySelector(`[data-field-path="${m.field_path}"]`);
        if (!cellEl || !chipEl) return;

        const cellRect = cellEl.getBoundingClientRect();
        const chipRect = chipEl.getBoundingClientRect();

        const cellDistTop = cellRect.top - previewRect.top;
        const cellDistBot = previewRect.bottom - cellRect.bottom;
        const chipDistTop = pickerRect ? chipRect.top - pickerRect.top : 100;
        const chipDistBot = pickerRect ? pickerRect.bottom - chipRect.bottom : 100;

        const minDist = Math.min(cellDistTop, cellDistBot, chipDistTop, chipDistBot);
        if (minDist < -5) return;

        let opacity = 0.6;
        if (minDist < fadeZone) opacity = Math.max(0.05, 0.6 * (minDist / fadeZone));

        const x1 = chipRect.left - bodyRect.left;
        const y1 = chipRect.top + chipRect.height / 2 - bodyRect.top;
        const x2 = cellRect.right - bodyRect.left;
        const y2 = cellRect.top + cellRect.height / 2 - bodyRect.top;

        const dx = Math.abs(x1 - x2) * 0.4;
        const d = `M${x1},${y1} C${x1 - dx},${y1} ${x2 + dx},${y2} ${x2},${y2}`;

        lines.push({
          id: m.field_path + '-' + m.cell_ref,
          fieldPath: m.field_path,
          cellRef: m.cell_ref,
          d,
          x1, y1, x2, y2,
          opacity,
          color: getPathColor(i),
        });
      });

      this.pathLines = lines;

      if (this.pathPopup) {
        const match = lines.find(
          l => l.fieldPath === this.pathPopup.fieldPath && l.cellRef === this.pathPopup.cellRef
        );
        if (match) {
          this.pathPopup.x = (match.x1 + match.x2) / 2;
          this.pathPopup.y = (match.y1 + match.y2) / 2;
        } else {
          this.pathPopup = null;
        }
      }
    },
  },
};
</script>

<style scoped>
/* ---- Header ---- */
.te-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  gap: 16px;
}

.te-header__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text);
}

.te-header__toggles {
  display: flex;
  align-items: center;
  gap: 16px;
}

/* ---- Layout ---- */
/* min-width:0 - иначе окно (flex-элемент оверлея) распирается минимальной шириной
   отрисованного листа и вылезает за свой max-width, а раскладка прыгает. */
.te-modal-body {
  display: flex;
  position: relative;
  min-width: 0;
  height: calc(var(--app-vh, 1vh) * 92 - 100px);
  min-height: 400px;
  overflow: hidden;
}

/* Место под предпросмотр резервируем долей окна: раньше панель тянулась по ширине
   отрисованного листа, поэтому при загрузке и смене шаблона раскладка прыгала.
   Лист шире доли - прокручивается внутри панели. */
.te-preview-panel {
  flex: 0 0 62%;
  max-width: 62%;
  min-width: 0;
  overflow: auto;
  border-right: 1px solid var(--color-border);
}

.te-preview-panel :deep(.xlsx-viewer) {
  border: none;
  border-radius: 0;
  min-height: 100%;
}

.te-preview-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--color-text-muted);
  font-size: 14px;
  padding: 20px 40px;
  text-align: center;
}

.te-settings-panel {
  flex: 1 1 0;
  min-width: 280px;
  overflow-y: auto;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* ---- Action banner ---- */
.te-action-banner {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 25;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 16px;
  font-size: 13px;
}

.te-action-banner--warning {
  background: var(--warning-bg);
  border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
  border-bottom: 1px solid var(--warning);
  color: var(--warning-text);
}

/* Баннер лежит поверх листа бланка, поэтому фон обязан быть непрозрачным: --accent-tint
   полупрозрачен (в тёмной теме 0.22), и сквозь него просвечивала таблица. */
.te-action-banner--info {
  background: color-mix(in srgb, var(--color-primary) 12%, var(--surface));
  border-bottom: 1px solid var(--color-primary);
  color: var(--accent-text);
}

.te-banner-fade-enter-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.te-banner-fade-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.te-banner-fade-enter-from {
  opacity: 0;
  transform: translateY(-100%);
}

.te-banner-fade-leave-to {
  opacity: 0;
  transform: translateY(-100%);
}

/* ---- SVG paths ---- */
.te-path-overlay {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  z-index: 15;
  overflow: hidden;
}

.te-path-line {
  fill: none;
  stroke-width: 1.5;
  pointer-events: stroke;
  cursor: pointer;
  transition: opacity 0.25s ease, stroke-width 0.25s ease, filter 0.25s ease;
  stroke-dasharray: 2000;
  animation: te-path-draw 1s ease forwards;
}

.te-paths-leaving .te-path-line {
  animation: te-path-retract 0.8s ease forwards;
}

@keyframes te-path-draw {
  from { stroke-dashoffset: 2000; }
  to { stroke-dashoffset: 0; }
}

@keyframes te-path-retract {
  from { stroke-dashoffset: 0; }
  to { stroke-dashoffset: 2000; }
}

.te-paths-leaving .te-path-dot {
  animation: te-dot-fade-out 0.6s ease forwards;
}

@keyframes te-dot-fade-out {
  to { opacity: 0; }
}

.te-path-hitarea {
  fill: none;
  stroke: transparent;
  stroke-width: 14;
  pointer-events: stroke;
  cursor: pointer;
}

.te-path-line.te-path-highlighted {
  filter: brightness(0.85);
}

.te-path-line.te-path-dimmed {
  opacity: 0.15 !important;
}

.te-path-line.te-path-hover-hidden {
  opacity: 0 !important;
  transition: opacity 0.25s ease;
}

.te-path-line.te-path-hover-active {
  stroke-width: 2.5 !important;
  filter: brightness(0.85);
  transition: stroke-width 0.25s ease, filter 0.25s ease, opacity 0.25s ease;
}

.te-path-dot {
  transition: opacity 0.25s ease;
}

.te-path-dot.te-path-dimmed {
  opacity: 0.15 !important;
}

.te-path-dot.te-path-hover-hidden {
  opacity: 0 !important;
}

.te-path-delete-btn {
  display: block;
  width: 100%;
  padding: 4px 8px;
  background: var(--color-danger);
  color: var(--fill-text);
  border: none;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 500;
  cursor: pointer;
  text-align: center;
}

.te-path-delete-btn:hover {
  background: var(--danger);
}

.te-popup-faded {
  opacity: 0.3;
  transition: opacity 0.2s ease;
}

.te-popup-fade-enter-active {
  transition: opacity 0.2s ease;
}

.te-popup-fade-leave-active {
  transition: opacity 0.15s ease;
}

.te-popup-fade-enter-from,
.te-popup-fade-leave-to {
  opacity: 0;
}

/* ---- Sections ---- */
.te-section {
  border: 1px solid var(--color-border);
  border-radius: 30px;
  padding: 16px 20px;
  background: var(--surface);
}

.te-section--compact {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* ---- Templates block ---- */
.te-templates-block {
  padding: 0 0 4px;
  background: var(--surface);
  display: flex;
  flex-direction: column;
  gap: 12px;
}


/* ---- Separator ---- */
.te-params-block {
  display: flex;
  align-items: flex-end;
  gap: 14px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}

/* Подписи полей разной высоты (часть переносится на две строки), поэтому равняем по
   нижнему краю - иначе поля с короткой подписью висят выше соседних. */
.te-params-fields {
  display: flex;
  align-items: flex-end;
  gap: 14px;
  flex: 1;
  min-width: 0;
}

/* Цвета те же, что у te-action-banner--warning выше: непрозрачная светлая плашка
   с тёмным текстом читается в обеих темах, полупрозрачный фон в тёмной сливался. */
.te-foreign-warning {
  margin: 0 0 8px;
  padding: 8px 10px;
  border-radius: var(--radius-md, 15px);
  background: #fff3cd;
  border: 1px solid #ffc107;
  color: #856404;
  font-size: 12px;
  line-height: 1.4;
}

.te-repeat-note {
  margin: 0 0 8px;
  padding: 8px 10px;
  border-radius: var(--radius-md, 15px);
  background: var(--info-bg);
  border: 1px solid var(--info);
  color: var(--info-text);
  font-size: 12px;
  line-height: 1.4;
}

.te-separator-block {
  display: flex;
  align-items: center;
  gap: 12px;
}

.te-separator-label {
  font-size: 13px;
  color: var(--color-text-muted);
  white-space: nowrap;
}

.te-separator-input {
  width: 72px;
  padding: 8px 12px !important;
  font-size: 14px !important;
  text-align: center;
}

/* ---- File block ---- */
.te-file-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.te-file-info {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.te-file-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text);
}

.te-file-meta {
  font-size: 11px;
  color: var(--color-text-muted);
  background: var(--color-bg-secondary);
  padding: 2px 6px;
  border-radius: 4px;
}

.te-file-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

/* ---- Drag & Drop ---- */
.te-upload-area {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.te-dropzone {
  border: 2px dashed var(--color-border);
  border-radius: var(--radius-sm);
  padding: 16px;
  text-align: center;
  transition: all 0.2s ease;
  background: var(--surface);
}

.te-dropzone--active {
  border-color: var(--accent);
  background: var(--accent-tint);
}

.te-dropzone--has-file {
  border-style: solid;
  border-color: var(--color-success);
  background: var(--success-bg);
}

.te-dropzone__file {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.te-dropzone__filename {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text);
}

.te-dropzone__clear {
  background: none;
  border: none;
  color: var(--danger-text);
  font-size: 18px;
  cursor: pointer;
  line-height: 1;
}

.te-dropzone__placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
}

.te-dropzone__hint {
  font-size: 12px;
  color: var(--color-text-muted);
}

.te-dropzone__or {
  font-size: 11px;
  color: var(--text-muted);
}

.te-dropzone__browse {
  cursor: pointer;
}

.te-upload-fields {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 6px;
}

.te-form-field {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.te-form-field label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-muted);
}

/* Ряд границ списка: подписи в одну строку, поля одинаковой ширины - иначе колонки
   разной высоты и ряд выглядит рваным. Панель узкая, поэтому поля переносятся, а не
   наезжают на кнопку сохранения. */
.te-params-fields {
  flex-wrap: wrap;
  row-gap: 10px;
}

.te-params-fields .te-form-field {
  flex: 0 0 92px;
  min-width: 0;
}

.te-params-fields .te-form-field label {
  white-space: nowrap;
}

.te-params-fields .te-compact-input {
  width: 100%;
}

.te-compact-input {
  padding: 8px 12px !important;
  font-size: 14px !important;
}

.te-upload-actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
}

/* ---- Filters ---- */
.te-category-filter {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.te-cat-btn {
  padding: 3px 10px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  background: var(--surface);
  font-size: 11px;
  cursor: pointer;
  transition: all 0.15s;
  color: var(--color-text);
  white-space: nowrap;
}

.te-cat-btn:hover {
  border-color: var(--accent);
  color: var(--accent-text);
}

.te-cat-btn.active {
  background: var(--color-primary);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

/* ---- Field picker ---- */
.te-field-picker {
  border: 1px solid var(--color-border);
  border-radius: 30px;
  background: var(--surface);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.te-picker-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  border-bottom: 1px solid var(--color-border);
  flex-shrink: 0;
}

.te-picker-header h4 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
}

.te-search-wrap {
  position: relative;
  display: flex;
  align-items: center;
}

.te-search-icon {
  position: absolute;
  left: 8px;
  width: 12px;
  height: 12px;
  opacity: 0.45;
  pointer-events: none;
  /* 12px - самый мелкий значок среза: 1.7 вырождается в 0.85px. */
  stroke-width: 2.4;
  color: var(--text);
}

.te-search-input {
  width: 140px;
  padding: 4px 8px 4px 24px !important;
  font-size: 11px !important;
}

.te-pick-hint {
  font-size: 11px;
  color: var(--accent-text);
  font-weight: 500;
  animation: te-pulse 1.5s ease-in-out infinite;
}

@keyframes te-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.te-field-picker-scroll {
  overflow-y: auto;
  padding: 0;
  height: 450px;
}

.te-field-group {
  padding: 8px 10px 10px;
  border-bottom: 1px solid var(--color-border);
}

.te-field-group:last-child {
  border-bottom: none;
}

.te-field-group-label {
  display: block;
  font-size: 10px;
  color: var(--color-text-muted);
  margin-bottom: 3px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  font-weight: 600;
}

.te-field-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 3px;
}

.te-field-chip {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 3px 8px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  font-size: 11px;
  background: var(--surface);
  cursor: pointer;
  transition: all 0.15s;
  color: var(--color-text);
}

.te-field-chip:hover {
  border-color: var(--accent);
  background: var(--accent-tint);
}

.te-field-chip.active {
  background: var(--color-primary);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

.te-field-chip.used {
  background: var(--success-bg);
  border-color: var(--color-success);
  color: var(--success-text);
}

.te-field-chip.used:hover {
  background: color-mix(in srgb, var(--success) 22%, var(--surface));
}

.te-chip-label {
  white-space: nowrap;
}

.te-chip-ref {
  font-size: 9px;
  opacity: 0.7;
  font-weight: 600;
}

.te-chip-remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  font-size: 13px;
  line-height: 1;
  color: var(--danger-text);
  background: color-mix(in srgb, var(--danger) 10%, var(--surface));
  transition: all 0.15s;
}

.te-chip-remove:hover {
  background: var(--color-danger);
  color: var(--fill-text);
}

/* Remove popup */
.te-field-chip {
  position: relative;
}

.te-remove-popup {
  position: absolute;
  top: calc(100% + 4px);
  right: 0;
  background: var(--surface);
  border: 1px solid var(--color-border);
  border-radius: 6px;
  box-shadow: var(--shadow-md);
  z-index: 20;
  min-width: 100px;
  overflow: hidden;
}

.te-remove-popup__item {
  padding: 5px 10px;
  font-size: 11px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.te-remove-popup__item:hover {
  background: var(--danger-bg);
}

.te-remove-popup__x {
  color: var(--danger-text);
  font-size: 14px;
}

.te-remove-popup__all {
  padding: 5px 10px;
  font-size: 11px;
  cursor: pointer;
  border-top: 1px solid var(--color-border);
  color: var(--danger-text);
  font-weight: 500;
}

.te-remove-popup__all:hover {
  background: var(--danger-bg);
}

.te-no-results {
  font-size: 12px;
  color: var(--color-text-muted);
  text-align: center;
  padding: 12px;
}

/* ---- Mappings (collapsible) ---- */
.te-mappings-section {
  border: 1px solid var(--color-border);
  border-radius: 30px;
  background: var(--surface);
  flex-shrink: 0;
  overflow: hidden;
}

.te-section-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  background: none;
  border: none;
  cursor: pointer;
  padding: 8px 12px;
}

.te-section-toggle h4 {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text);
}

.te-mappings-count {
  font-size: 10px;
  background: var(--color-primary);
  color: var(--accent-contrast);
  padding: 1px 6px;
  border-radius: var(--radius-pill);
  font-weight: 600;
}

.te-chevron {
  margin-left: auto;
  transition: transform 0.2s;
  font-size: 11px;
  color: var(--color-text-muted);
}

.te-chevron.open {
  transform: rotate(90deg);
}

.te-mappings-body {
  padding: 0 10px 8px;
  max-height: 180px;
  overflow-y: auto;
}

.te-empty-state {
  font-size: 11px;
  color: var(--color-text-muted);
  text-align: center;
  padding: 8px 0;
}

.te-mapping-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 6px;
  border-radius: 4px;
  font-size: 11px;
  transition: background 0.1s;
}

.te-mapping-row:hover {
  background: var(--color-bg-secondary);
}

.te-mapping-row.highlight {
  background: var(--accent-tint);
}

.te-mapping-cell {
  font-weight: 600;
  min-width: 30px;
  color: var(--accent-text);
}

.te-mapping-field {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-text);
}

.te-list-badge {
  font-size: 9px;
  background: var(--info-bg);
  color: var(--info-text);
  padding: 1px 5px;
  border-radius: var(--radius-pill);
  font-weight: 500;
}

.te-mapping-order {
  font-size: 9px;
  font-weight: 700;
  padding: 1px 4px;
  border-radius: var(--radius-pill);
  background: var(--color-primary);
  color: var(--accent-contrast);
  position: relative;
}

.te-mapping-move {
  display: inline-flex;
  gap: 2px;
  margin-left: auto;
}

.te-mapping-move-btn {
  border: none;
  background: transparent;
  color: var(--text-secondary, #666);
  font-size: 9px;
  line-height: 1;
  padding: 2px 3px;
  cursor: pointer;
}

.te-mapping-move-btn:disabled {
  opacity: 0.3;
  cursor: default;
}

.te-list-badge--repeat {
  background: transparent;
  border: 1px solid var(--info);
}

.te-mapping-remove {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: none;
  background: none;
  color: var(--color-text-muted);
  font-size: 14px;
  cursor: pointer;
  transition: all 0.15s;
  flex-shrink: 0;
}

.te-mapping-remove:hover {
  background: color-mix(in srgb, var(--danger) 10%, var(--surface));
  color: var(--danger-text);
}

/* ---- Save ---- */
.te-save-area {
  flex-shrink: 0;
}

/* ---- Button sizes ---- */
.te-btn-sm {
  padding: 5px 12px !important;
  font-size: 11px !important;
}

/* ---- Content transitions ---- */
.te-content-fade-enter-active {
  transition: opacity 0.3s ease;
}

.te-content-fade-leave-active {
  transition: opacity 0.15s ease;
}

.te-content-fade-enter-from,
.te-content-fade-leave-to {
  opacity: 0;
}

/* ---- Spinner ---- */
.te-spinner {
  display: block;
  width: 28px;
  height: 28px;
  border: 3px solid var(--color-border);
  border-top-color: var(--accent-text);
  border-radius: 50%;
  animation: te-spin 0.7s linear infinite;
}

@keyframes te-spin {
  to { transform: rotate(360deg); }
}

.te-fields-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px 0;
}

/* ---- Responsive ---- */
@media (max-width: 900px) {
  .te-modal-body {
    flex-direction: column;
    height: auto;
  }

  .te-preview-panel {
    flex: 0 0 auto;
    max-width: 100%;
    height: 50vh;
    border-right: none;
    border-bottom: 1px solid var(--color-border);
  }

  .te-settings-panel {
    width: 100%;
  }
}
</style>

<style>
.te-modal-rounded.base-modal {
  border-radius: 40px;
  /* Ширина окна задаётся явно и не зависит от содержимого: иначе окно тянулось за
     шириной отрисованного листа, и раскладка прыгала между загрузкой и готовым
     предпросмотром. Лист шире - прокручивается внутри своей панели. */
  flex: 0 0 auto;
  width: 95vw;
  min-width: 0;
}

@media (max-width: 768px) {
  .te-modal-rounded.base-modal {
    border-radius: 16px 16px 0 0;
  }
}

.te-template-dropdown-wrap {
  position: relative;
}

.te-template-dropdown-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  padding: 6px 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  font-size: 12px;
  background: var(--surface);
  cursor: pointer;
  transition: border-color 0.15s;
  color: var(--color-text);
}

.te-template-dropdown-trigger:hover {
  border-color: var(--accent);
}

.te-dropdown-filename {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.te-dropdown-arrow {
  font-size: 10px;
  color: var(--color-text-muted);
  transition: transform 0.2s;
  flex-shrink: 0;
}

.te-dropdown-arrow.open {
  transform: rotate(180deg);
}

.te-template-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  background: var(--surface);
  border: 1px solid var(--color-border);
  border-radius: 20px;
  box-shadow: var(--shadow-md);
  z-index: 30;
  overflow: hidden;
}

.te-dropdown-item {
  display: block;
  width: 100%;
  padding: 7px 12px;
  border: none;
  background: none;
  font-size: 12px;
  text-align: left;
  cursor: pointer;
  color: var(--text);
  transition: background 0.1s;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.te-dropdown-item:hover {
  background: var(--color-bg-secondary);
}

.te-dropdown-item.active {
  background: var(--accent-tint);
  color: var(--text);
  font-weight: 500;
}

.te-dropdown-add {
  border-top: 1px solid var(--color-border);
  color: var(--text);
  font-weight: 500;
}

.te-dropdown-fade-enter-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.te-dropdown-fade-leave-active {
  transition: opacity 0.1s ease, transform 0.1s ease;
}

.te-dropdown-fade-enter-from,
.te-dropdown-fade-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
