<template>
  <BaseModal
    :show="show"
    title="Excel-бланк"
    width="95vw"
    closable
    :close-on-overlay="false"
    @close="$emit('close')"
  >
    <div class="te-modal-body">
      <!-- Верхняя панель: управление -->
      <div class="te-toolbar">
        <div class="te-toolbar-left">
          <label class="te-toggle">
            <input
              type="checkbox"
              :checked="enabled"
              @change="onToggle"
            >
            <span>Генерация бланка</span>
          </label>
        </div>
        <div
          v-if="enabled && template && template.file_path"
          class="te-toolbar-right"
        >
          <div class="te-file-info">
            <span class="te-file-name">{{ template.original_file_name || 'template.xlsx' }}</span>
            <span class="te-file-meta">
              строки {{ template.list_start_row }}-{{ template.list_end_row }}
            </span>
          </div>
          <button
            class="lk-button lk-button--ghost te-btn-sm"
            @click="showUpload = true"
          >
            Заменить
          </button>
          <button
            class="lk-button lk-button--danger te-btn-sm"
            @click="onDeleteTemplate"
          >
            Удалить
          </button>
        </div>
      </div>

      <!-- Форма загрузки шаблона -->
      <div
        v-if="enabled && (!template || !template.file_path || showUpload)"
        class="te-upload-section"
      >
        <form
          class="te-upload-form"
          @submit.prevent="onUpload"
        >
          <div class="te-upload-file">
            <input
              type="file"
              accept=".xlsx"
              required
              @change="onFileChange"
            >
          </div>
          <div class="te-upload-fields">
            <div class="te-form-field">
              <label>Начальная строка списка</label>
              <input
                v-model.number="form.listStartRow"
                type="number"
                min="1"
                class="lk-input"
                required
              >
            </div>
            <div class="te-form-field">
              <label>Конечная строка списка</label>
              <input
                v-model.number="form.listEndRow"
                type="number"
                min="1"
                class="lk-input"
                required
              >
            </div>
            <div class="te-form-field">
              <label>Макс. записей</label>
              <input
                v-model.number="form.maxListRows"
                type="number"
                min="0"
                class="lk-input"
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
              type="submit"
              class="lk-button lk-button--primary te-btn-sm"
              :disabled="!form.file || uploading"
            >
              {{ uploading ? 'Загрузка...' : 'Загрузить' }}
            </button>
          </div>
        </form>
      </div>

      <!-- Визуальный редактор -->
      <div
        v-if="enabled && template && template.file_path"
        class="te-editor"
      >
        <!-- Панель поиска и фильтров -->
        <div class="te-filters">
          <div class="te-search-wrap">
            <input
              v-model="searchQuery"
              type="text"
              class="lk-input te-search-input"
              placeholder="Поиск полей..."
            >
          </div>
          <div class="te-category-filter">
            <button
              class="te-cat-btn"
              :class="{ active: activeCategory === '' }"
              @click="activeCategory = ''"
            >
              Все
            </button>
            <button
              v-for="g in fieldGroups"
              :key="g.group"
              class="te-cat-btn"
              :class="{ active: activeCategory === g.group }"
              @click="activeCategory = g.group"
            >
              {{ g.label }}
            </button>
          </div>
          <label class="te-paths-toggle">
            <input
              v-model="showPaths"
              type="checkbox"
            >
            <span>Показать пути</span>
          </label>
        </div>

        <!-- Split panel: xlsx + поля -->
        <div
          ref="splitPanel"
          class="te-split-panel"
        >
          <!-- SVG линии -->
          <svg
            v-if="showPaths && pathLines.length"
            class="te-path-overlay"
            :viewBox="`0 0 ${svgWidth} ${svgHeight}`"
          >
            <path
              v-for="line in pathLines"
              :key="line.id"
              :d="line.d"
              class="te-path-line"
              :style="{ stroke: line.color }"
            />
            <circle
              v-for="line in pathLines"
              :key="line.id + '-dot-l'"
              :cx="line.x2"
              :cy="line.y2"
              r="3"
              :fill="line.color"
            />
            <circle
              v-for="line in pathLines"
              :key="line.id + '-dot-r'"
              :cx="line.x1"
              :cy="line.y1"
              r="3"
              :fill="line.color"
            />
          </svg>

          <!-- Левая панель: xlsx -->
          <div
            ref="panelLeft"
            class="te-panel-left"
          >
            <XlsxViewer
              ref="xlsxViewer"
              :file-buffer="templateFileBuffer"
              :mappings="enrichedMappings"
              :selected-cell="pendingCellRef"
              @cell-click="onCellClick"
            />
          </div>

          <!-- Правая панель: поля + привязки -->
          <div
            ref="panelRight"
            class="te-panel-right"
          >
            <!-- Поля -->
            <div class="te-field-picker">
              <div class="te-picker-header">
                <h4>Поля для привязки</h4>
                <span
                  v-if="pendingFieldPath"
                  class="te-pick-hint"
                >
                  Кликните на ячейку слева
                </span>
              </div>
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
                    @click="selectField(f)"
                  >
                    <span class="te-chip-label">{{ f.label }}</span>
                    <span
                      v-if="fieldPathUsed(f.path)"
                      class="te-chip-ref"
                    >
                      {{ fieldCellRef(f.path) }}
                    </span>
                    <span
                      v-if="fieldPathUsed(f.path)"
                      class="te-chip-remove"
                      @click.stop="removeMappingByPath(f.path)"
                    >
                      &times;
                    </span>
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

            <!-- Привязки -->
            <div class="te-mappings-section">
              <div class="te-section-header">
                <h4>Привязки</h4>
                <span class="te-mappings-count">{{ mappings.length }}</span>
              </div>
              <div
                v-if="!mappings.length"
                class="te-empty-state"
              >
                Нет привязок. Выберите поле и кликните на ячейку.
              </div>
              <div class="te-mappings-list">
                <div
                  v-for="(m, idx) in enrichedMappings"
                  :key="idx"
                  class="te-mapping-row"
                  :class="{ highlight: pendingCellRef === m.cell_ref }"
                >
                  <span class="te-mapping-cell">{{ m.cell_ref }}</span>
                  <span class="te-mapping-field">{{ m.fieldLabel || m.field_path }}</span>
                  <span
                    v-if="m.is_list_field"
                    class="te-list-badge"
                  >список</span>
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
          </div>
        </div>
      </div>

      <!-- Кастомные поля -->
      <div class="te-custom-section">
        <div class="te-section-header">
          <h4>Дополнительные поля вложения</h4>
        </div>
        <table class="te-custom-table">
          <thead>
            <tr>
              <th>Заголовок</th>
              <th>Плейсхолдер</th>
              <th class="th-order">
                N
              </th>
              <th class="th-actions" />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="cf in customFields"
              :key="cf.id"
            >
              <td>
                <input
                  v-model="cf.label"
                  class="lk-input te-table-input"
                >
              </td>
              <td>
                <input
                  v-model="cf.placeholder"
                  class="lk-input te-table-input"
                >
              </td>
              <td>
                <input
                  v-model.number="cf.sort_order"
                  type="number"
                  class="lk-input te-table-input te-order-input"
                >
              </td>
              <td class="te-row-actions">
                <button
                  class="lk-button lk-button--ghost te-btn-xs"
                  @click="updateCF(cf)"
                >
                  Сохранить
                </button>
                <button
                  class="lk-button lk-button--danger te-btn-xs"
                  @click="deleteCF(cf)"
                >
                  Удалить
                </button>
              </td>
            </tr>
            <tr v-if="!customFields.length">
              <td
                colspan="4"
                class="te-empty-cell"
              >
                Кастомных полей нет
              </td>
            </tr>
          </tbody>
        </table>
        <div class="te-custom-add">
          <input
            v-model="newCF.label"
            placeholder="Заголовок"
            class="lk-input te-add-input"
          >
          <input
            v-model="newCF.placeholder"
            placeholder="Плейсхолдер"
            class="lk-input te-add-input"
          >
          <button
            class="lk-button lk-button--primary te-btn-sm"
            :disabled="!newCF.label"
            @click="addCF"
          >
            + Добавить поле
          </button>
        </div>
      </div>
    </div>

    <template #actions>
      <button
        v-if="enabled && template && template.file_path"
        class="lk-button lk-button--primary"
        :disabled="savingMappings"
        @click="saveMappings"
      >
        {{ savingMappings ? 'Сохранение...' : 'Сохранить привязки' }}
      </button>
      <button
        class="lk-button lk-button--ghost"
        @click="$emit('close')"
      >
        Закрыть
      </button>
    </template>
  </BaseModal>
</template>

<script>
import { useUiStore } from '@/stores/ui';
import {
  getTemplate, uploadTemplate, updateMappings, deleteTemplate,
  getTemplateFields, getTemplateFile,
  listCustomFields, createCustomField, updateCustomField, deleteCustomField,
} from '@/api/attachment-templates';
import BaseModal from '@/components/ui/BaseModal.vue';
import XlsxViewer from './XlsxViewer.vue';

const PATH_COLORS = [
  '#4F5BDF', '#e85d75', '#2e9e5a', '#e8a317', '#8e44ad',
  '#16a085', '#c0392b', '#2980b9', '#d35400', '#7f8c8d',
];

export default {
  name: 'AttachmentTemplateEditor',
  components: { BaseModal, XlsxViewer },
  props: {
    show: { type: Boolean, required: true },
    uniqueAttachmentId: { type: Number, required: true },
  },
  emits: ['close'],
  data() {
    return {
      template: null,
      mappings: [],
      fieldGroups: [],
      customFields: [],
      newCF: { label: '', placeholder: '' },
      enabled: false,
      showUpload: false,
      form: { file: null, listStartRow: 1, listEndRow: 1, maxListRows: 0 },
      uploading: false,
      savingMappings: false,
      templateFileBuffer: null,
      pendingFieldPath: '',
      pendingFieldLabel: '',
      pendingCellRef: '',
      searchQuery: '',
      activeCategory: '',
      showPaths: false,
      pathLines: [],
      svgWidth: 0,
      svgHeight: 0,
      rafId: null,
    };
  },
  computed: {
    enrichedMappings() {
      return this.mappings.map(m => ({
        ...m,
        fieldLabel: this.getFieldLabel(m.field_path),
      }));
    },
    filteredFieldGroups() {
      let groups = this.fieldGroups;
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
  },
  watch: {
    show(val) {
      if (val) {
        this.loadAll();
      } else {
        this.cleanupPathListeners();
      }
    },
    showPaths(val) {
      if (val) {
        this.$nextTick(() => this.setupPathListeners());
      } else {
        this.cleanupPathListeners();
        this.pathLines = [];
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
    async loadAll() {
      await Promise.all([this.loadTemplate(), this.loadCustomFields(), this.loadFields()]);
    },
    async loadTemplate() {
      try {
        const data = await getTemplate(this.uniqueAttachmentId);
        this.template = data;
        this.mappings = (data && data.mappings) || [];
        this.enabled = !!(data && data.file_path);
        this.form.listStartRow = data && data.list_start_row || 1;
        this.form.listEndRow = data && data.list_end_row || 1;
        this.form.maxListRows = data && data.max_list_rows || 0;
        if (this.enabled) this.loadTemplateFile();
      } catch {
        this.template = null;
        this.mappings = [];
        this.enabled = false;
        this.templateFileBuffer = null;
      }
    },
    async loadTemplateFile() {
      try {
        this.templateFileBuffer = await getTemplateFile(this.uniqueAttachmentId);
      } catch {
        this.templateFileBuffer = null;
      }
    },
    async loadCustomFields() {
      try {
        const data = await listCustomFields(this.uniqueAttachmentId);
        this.customFields = Array.isArray(data) ? data : [];
      } catch {
        this.customFields = [];
      }
    },
    async loadFields() {
      try {
        const data = await getTemplateFields(this.uniqueAttachmentId);
        this.fieldGroups = Array.isArray(data) ? data : [];
      } catch {
        this.fieldGroups = [];
      }
    },
    onToggle(e) {
      this.enabled = e.target.checked;
      if (!this.enabled && this.template && this.template.file_path) {
        if (!confirm('Отключить генерацию бланка? Текущий шаблон будет удален.')) {
          this.enabled = true;
          e.target.checked = true;
          return;
        }
        this.onDeleteTemplate();
      }
    },
    onFileChange(e) {
      this.form.file = e.target.files[0] || null;
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
        useUiStore().success('Шаблон загружен');
        this.showUpload = false;
        await this.loadTemplate();
      } catch (err) {
        useUiStore().error(err.message || 'Не удалось загрузить шаблон');
      } finally {
        this.uploading = false;
      }
    },
    async onDeleteTemplate() {
      try {
        await deleteTemplate(this.uniqueAttachmentId);
        useUiStore().success('Шаблон удален');
        this.templateFileBuffer = null;
        await this.loadTemplate();
      } catch {
        useUiStore().error('Не удалось удалить шаблон');
      }
    },
    selectField(field) {
      if (this.pendingFieldPath === field.path) {
        this.pendingFieldPath = '';
        this.pendingFieldLabel = '';
      } else {
        this.pendingFieldPath = field.path;
        this.pendingFieldLabel = field.label;
      }
    },
    onCellClick(cellRef) {
      if (this.pendingFieldPath) {
        const existingIdx = this.mappings.findIndex(m => m.cell_ref === cellRef);
        if (existingIdx >= 0) {
          this.mappings[existingIdx].field_path = this.pendingFieldPath;
          this.mappings[existingIdx].is_list_field = this.isListField(this.pendingFieldPath);
        } else {
          this.mappings.push({
            cell_ref: cellRef,
            field_path: this.pendingFieldPath,
            is_list_field: this.isListField(this.pendingFieldPath),
          });
        }
        this.pendingFieldPath = '';
        this.pendingFieldLabel = '';
        this.pendingCellRef = '';
      } else {
        const mappedIdx = this.mappings.findIndex(m => m.cell_ref === cellRef);
        if (mappedIdx >= 0) {
          this.pendingCellRef = cellRef;
        } else {
          this.pendingCellRef = cellRef;
        }
      }
    },
    removeMapping(idx) {
      this.mappings.splice(idx, 1);
    },
    removeMappingByPath(fieldPath) {
      const idx = this.mappings.findIndex(m => m.field_path === fieldPath);
      if (idx >= 0) this.mappings.splice(idx, 1);
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
    fieldCellRef(path) {
      const m = this.mappings.find(x => x.field_path === path);
      return m ? m.cell_ref : '';
    },
    getFieldLabel(fieldPath) {
      for (const g of this.fieldGroups) {
        const f = g.fields.find(x => x.path === fieldPath);
        if (f) return f.label;
      }
      return fieldPath;
    },
    async saveMappings() {
      this.savingMappings = true;
      try {
        const payload = this.mappings.filter(m => m.cell_ref && m.field_path);
        await updateMappings(this.uniqueAttachmentId, payload);
        useUiStore().success('Привязки сохранены');
        await this.loadTemplate();
      } catch (err) {
        useUiStore().error(err.message || 'Не удалось сохранить');
      } finally {
        this.savingMappings = false;
      }
    },
    async addCF() {
      if (!this.newCF.label) return;
      try {
        await createCustomField(this.uniqueAttachmentId, {
          label: this.newCF.label,
          placeholder: this.newCF.placeholder,
          sortOrder: this.customFields.length,
        });
        this.newCF = { label: '', placeholder: '' };
        await this.loadCustomFields();
        await this.loadFields();
        useUiStore().success('Поле добавлено');
      } catch {
        useUiStore().error('Не удалось добавить поле');
      }
    },
    async updateCF(cf) {
      try {
        await updateCustomField(cf.id, {
          label: cf.label,
          placeholder: cf.placeholder || '',
          sortOrder: cf.sort_order || 0,
        });
        useUiStore().success('Поле обновлено');
      } catch {
        useUiStore().error('Не удалось обновить');
      }
    },
    async deleteCF(cf) {
      if (!confirm(`Удалить поле "${cf.label}"?`)) return;
      try {
        await deleteCustomField(cf.id);
        await this.loadCustomFields();
        await this.loadFields();
        useUiStore().success('Поле удалено');
      } catch {
        useUiStore().error('Не удалось удалить');
      }
    },

    setupPathListeners() {
      this.cleanupPathListeners();
      const panel = this.$refs.splitPanel;
      if (!panel) return;

      const scrollables = panel.querySelectorAll('.te-panel-left, .te-panel-right, .xv-table-wrap');
      this._scrollHandler = () => {
        if (this.rafId) cancelAnimationFrame(this.rafId);
        this.rafId = requestAnimationFrame(() => this.updatePaths());
      };
      scrollables.forEach(el => el.addEventListener('scroll', this._scrollHandler, { passive: true }));

      this._resizeObserver = new ResizeObserver(this._scrollHandler);
      this._resizeObserver.observe(panel);

      this.updatePaths();
    },
    cleanupPathListeners() {
      if (this._scrollHandler) {
        const panel = this.$refs.splitPanel;
        if (panel) {
          const scrollables = panel.querySelectorAll('.te-panel-left, .te-panel-right, .xv-table-wrap');
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
      const panel = this.$refs.splitPanel;
      const leftPanel = this.$refs.panelLeft;
      const rightPanel = this.$refs.panelRight;
      if (!panel || !leftPanel || !rightPanel) {
        this.pathLines = [];
        return;
      }

      const panelRect = panel.getBoundingClientRect();
      const leftRect = leftPanel.getBoundingClientRect();
      const rightRect = rightPanel.getBoundingClientRect();

      this.svgWidth = panelRect.width;
      this.svgHeight = panelRect.height;

      const lines = [];
      this.enrichedMappings.forEach((m, i) => {
        const cellEl = leftPanel.querySelector(`[data-cell-ref="${m.cell_ref}"]`);
        const chipEl = rightPanel.querySelector(`[data-field-path="${m.field_path}"]`);
        if (!cellEl || !chipEl) return;

        const cellRect = cellEl.getBoundingClientRect();
        const chipRect = chipEl.getBoundingClientRect();

        const cellVisible = cellRect.top >= leftRect.top - 2 && cellRect.bottom <= leftRect.bottom + 2;
        const chipVisible = chipRect.top >= rightRect.top - 2 && chipRect.bottom <= rightRect.bottom + 2;
        if (!cellVisible || !chipVisible) return;

        const x1 = chipRect.left - panelRect.left;
        const y1 = chipRect.top + chipRect.height / 2 - panelRect.top;
        const x2 = cellRect.right - panelRect.left;
        const y2 = cellRect.top + cellRect.height / 2 - panelRect.top;

        const dx = Math.abs(x1 - x2) * 0.4;
        const d = `M${x1},${y1} C${x1 - dx},${y1} ${x2 + dx},${y2} ${x2},${y2}`;

        lines.push({
          id: m.field_path + '-' + m.cell_ref,
          d,
          x1, y1, x2, y2,
          color: PATH_COLORS[i % PATH_COLORS.length],
        });
      });

      this.pathLines = lines;
    },
  },
};
</script>

<style scoped>
/* Modal body override */
.te-modal-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 0;
}

/* Toolbar */
.te-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  padding: 12px 16px;
  background: var(--color-bg-secondary);
  border-radius: var(--radius-sm);
}

.te-toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.te-toolbar-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.te-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  color: var(--color-text);
}

.te-toggle input[type="checkbox"] {
  width: 16px;
  height: 16px;
  accent-color: var(--color-primary);
}

.te-file-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.te-file-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text);
}

.te-file-meta {
  font-size: 12px;
  color: var(--color-text-muted);
  background: #fff;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
}

/* Upload section */
.te-upload-section {
  border: 2px dashed var(--color-border);
  border-radius: var(--radius-sm);
  padding: 16px;
  background: #fff;
}

.te-upload-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.te-upload-file input[type="file"] {
  font-size: 13px;
}

.te-upload-fields {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.te-form-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.te-form-field label {
  font-size: 12px;
  font-weight: 500;
  color: #666;
}

.te-upload-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

/* Filters bar */
.te-filters {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.te-search-wrap {
  flex: 0 0 220px;
}

.te-search-input {
  padding: 6px 12px !important;
  font-size: 13px !important;
}

.te-category-filter {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  flex: 1;
}

.te-cat-btn {
  padding: 4px 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  background: #fff;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s;
  color: var(--color-text);
  white-space: nowrap;
}

.te-cat-btn:hover {
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.te-cat-btn.active {
  background: var(--color-primary);
  color: #fff;
  border-color: var(--color-primary);
}

.te-paths-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  cursor: pointer;
  white-space: nowrap;
  margin-left: auto;
  color: var(--color-text);
}

.te-paths-toggle input[type="checkbox"] {
  accent-color: var(--color-primary);
}

/* Split panel */
.te-split-panel {
  display: grid;
  grid-template-columns: 1fr 340px;
  gap: 16px;
  position: relative;
  flex: 1;
  min-height: 0;
  height: calc(60vh - 100px);
}

.te-path-overlay {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  z-index: 10;
  overflow: visible;
}

.te-path-line {
  fill: none;
  stroke-width: 1.5;
  opacity: 0.6;
}

.te-panel-left {
  min-width: 0;
  overflow: auto;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
}

.te-panel-left :deep(.xlsx-viewer) {
  border: none;
  border-radius: 0;
}

.te-panel-right {
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow-y: auto;
  min-height: 0;
}

/* Field picker */
.te-field-picker {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  padding: 12px;
  background: #fff;
}

.te-picker-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.te-picker-header h4 {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
}

.te-pick-hint {
  font-size: 11px;
  color: var(--color-primary);
  font-weight: 500;
  animation: te-pulse 1.5s ease-in-out infinite;
}

@keyframes te-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.te-field-group {
  margin-bottom: 10px;
}

.te-field-group:last-child {
  margin-bottom: 0;
}

.te-field-group-label {
  display: block;
  font-size: 10px;
  color: var(--color-text-muted);
  margin-bottom: 4px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  font-weight: 600;
}

.te-field-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.te-field-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  font-size: 12px;
  background: #fff;
  cursor: pointer;
  transition: all 0.15s;
  color: var(--color-text);
}

.te-field-chip:hover {
  border-color: var(--color-primary);
  background: #f0f4ff;
}

.te-field-chip.active {
  background: var(--color-primary);
  color: #fff;
  border-color: var(--color-primary);
}

.te-field-chip.used {
  background: #e8f8e8;
  border-color: var(--color-success);
  color: #1a6e2e;
}

.te-field-chip.used:hover {
  background: #d4f0d4;
}

.te-chip-label {
  white-space: nowrap;
}

.te-chip-ref {
  font-size: 10px;
  opacity: 0.7;
  font-weight: 600;
}

.te-chip-remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  font-size: 14px;
  line-height: 1;
  color: var(--color-danger);
  background: rgba(220, 53, 69, 0.1);
  margin-left: 2px;
  transition: all 0.15s;
}

.te-chip-remove:hover {
  background: var(--color-danger);
  color: #fff;
}

.te-no-results {
  font-size: 12px;
  color: var(--color-text-muted);
  text-align: center;
  padding: 12px;
}

/* Mappings */
.te-mappings-section {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  padding: 12px;
  background: #fff;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.te-section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}

.te-section-header h4 {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
}

.te-mappings-count {
  font-size: 11px;
  background: var(--color-primary);
  color: #fff;
  padding: 1px 7px;
  border-radius: var(--radius-pill);
  font-weight: 600;
}

.te-empty-state {
  font-size: 12px;
  color: var(--color-text-muted);
  text-align: center;
  padding: 16px 0;
}

.te-mappings-list {
  flex: 1;
  overflow-y: auto;
}

.te-mapping-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 6px;
  font-size: 12px;
  transition: background 0.1s;
}

.te-mapping-row:hover {
  background: var(--color-bg-secondary);
}

.te-mapping-row.highlight {
  background: #e8f4fd;
}

.te-mapping-cell {
  font-weight: 600;
  min-width: 36px;
  color: var(--color-primary);
}

.te-mapping-field {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-text);
}

.te-list-badge {
  font-size: 10px;
  background: #e3f2fd;
  color: #1565c0;
  padding: 2px 6px;
  border-radius: var(--radius-pill);
  font-weight: 500;
}

.te-mapping-remove {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  border: none;
  background: none;
  color: var(--color-text-muted);
  font-size: 16px;
  cursor: pointer;
  transition: all 0.15s;
  flex-shrink: 0;
}

.te-mapping-remove:hover {
  background: rgba(220, 53, 69, 0.1);
  color: var(--color-danger);
}

/* Custom fields */
.te-custom-section {
  border-top: 1px solid var(--color-border);
  padding-top: 16px;
}

.te-custom-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  margin-bottom: 12px;
}

.te-custom-table th {
  padding: 8px 10px;
  text-align: left;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.3px;
  border-bottom: 2px solid var(--color-border);
}

.te-custom-table td {
  padding: 6px 8px;
  border-bottom: 1px solid var(--color-border);
  vertical-align: middle;
}

.th-order { width: 60px; }
.th-actions { width: 180px; }

.te-table-input {
  padding: 5px 8px !important;
  font-size: 13px !important;
}

.te-order-input { width: 60px; }

.te-row-actions {
  display: flex;
  gap: 6px;
}

.te-empty-cell {
  text-align: center;
  color: var(--color-text-muted);
  padding: 16px 0 !important;
}

.te-custom-add {
  display: flex;
  gap: 8px;
  align-items: center;
}

.te-add-input {
  flex: 1;
  padding: 6px 10px !important;
  font-size: 13px !important;
}

/* Button sizes */
.te-btn-sm {
  padding: 6px 14px !important;
  font-size: 12px !important;
}

.te-btn-xs {
  padding: 4px 10px !important;
  font-size: 11px !important;
}

/* Responsive */
@media (max-width: 900px) {
  .te-split-panel {
    grid-template-columns: 1fr;
    height: auto;
  }

  .te-panel-left {
    max-height: 50vh;
  }

  .te-panel-right {
    max-height: none;
  }

  .te-upload-fields {
    grid-template-columns: 1fr;
  }

  .te-search-wrap {
    flex: 1 1 100%;
  }
}
</style>
