<template>
  <section class="template-editor">
    <header class="te-header">
      <h3>Excel-бланк</h3>
      <label class="te-toggle">
        <input
          type="checkbox"
          :checked="enabled"
          @change="onToggle"
        >
        Генерация и скачивание бланка
      </label>
    </header>

    <div v-if="enabled" class="te-body">
      <!-- Загрузка шаблона -->
      <div class="te-block">
        <h4>Шаблон .xlsx</h4>
        <div v-if="template && template.file_path" class="te-existing">
          <span>Текущий: {{ template.original_file_name || 'template.xlsx' }}</span>
          <span class="te-meta">
            строки списка {{ template.list_start_row }}-{{ template.list_end_row }} (макс {{ template.max_list_rows }})
          </span>
          <button class="te-link-btn" @click="showUpload = true">Заменить</button>
          <button class="te-link-btn danger" @click="onDeleteTemplate">Удалить шаб��он</button>
        </div>
        <form v-if="!template || !template.file_path || showUpload" class="te-upload-form" @submit.prevent="onUpload">
          <input type="file" accept=".xlsx" @change="onFileChange" required>
          <label>
            Начальная строка списка
            <input v-model.number="form.listStartRow" type="number" min="1" required>
          </label>
          <label>
            Конечная строка списка
            <input v-model.number="form.listEndRow" type="number" min="1" required>
          </label>
          <label>
            Макс. записей (авто = end - start + 1)
            <input v-model.number="form.maxListRows" type="number" min="0">
          </label>
          <div class="te-form-actions">
            <button v-if="template && template.file_path" type="button" class="te-link-btn" @click="showUpload = false">
              Отмена
            </button>
            <button type="submit" class="te-btn" :disabled="!form.file || uploading">
              {{ uploading ? 'Загрузка...' : 'Загрузить' }}
            </button>
          </div>
        </form>
      </div>

      <!-- Визуальный редактор маппинга -->
      <div v-if="template && template.file_path" class="te-block te-visual-editor">
        <h4>Привязка полей к ячейкам</h4>
        <p class="te-hint">
          Выберите поле справа, затем кликните на ячейку в превью для привязки.
          Зеленые ячейки уже привязаны.
        </p>
        <div class="te-split-panel">
          <!-- Левая часть: превью xlsx -->
          <div class="te-panel-left">
            <XlsxViewer
              :file-buffer="templateFileBuffer"
              :mappings="enrichedMappings"
              :selected-cell="pendingCellRef"
              @cell-click="onCellClick"
            />
          </div>
          <!-- Правая часть: список полей + маппинги -->
          <div class="te-panel-right">
            <!-- Выбор поля для привязки -->
            <div class="te-field-picker">
              <h5>Поле для привязки</h5>
              <div
                v-for="g in fieldGroups"
                :key="g.group"
                class="te-field-group"
              >
                <span class="te-field-group-label">{{ g.label }}</span>
                <button
                  v-for="f in g.fields"
                  :key="f.path"
                  class="te-field-chip"
                  :class="{
                    active: pendingFieldPath === f.path,
                    used: fieldPathUsed(f.path),
                  }"
                  @click="selectField(f)"
                >
                  {{ f.label }}
                  <span v-if="fieldPathUsed(f.path)" class="te-chip-mapped">
                    ({{ fieldCellRef(f.path) }})
                  </span>
                </button>
              </div>
            </div>

            <!-- Текущие маппинги -->
            <div class="te-mappings-list">
              <h5>Привязки ({{ mappings.length }})</h5>
              <div v-if="!mappings.length" class="te-empty-mappings">
                Нет привязок. Выберите поле и кликните на ячейку.
              </div>
              <div
                v-for="(m, idx) in enrichedMappings"
                :key="idx"
                class="te-mapping-row"
                :class="{ highlight: pendingCellRef === m.cell_ref }"
              >
                <span class="te-mapping-cell">{{ m.cell_ref }}</span>
                <span class="te-mapping-field">{{ m.fieldLabel || m.field_path }}</span>
                <span v-if="m.is_list_field" class="te-list-badge">список</span>
                <button class="te-remove-btn" @click="removeMapping(idx)">×</button>
              </div>
            </div>

            <div class="te-save-actions">
              <button class="te-btn" :disabled="savingMappings" @click="saveMappings">
                {{ savingMappings ? 'Сохранение...' : 'Сохранить привязки' }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Кастомные поля - всегда доступны -->
    <div class="te-block">
      <h4>Дополнительные поля вложения</h4>
      <table class="te-custom-table">
        <thead>
          <tr>
            <th>Заголовок</th>
            <th>Плейсхолдер</th>
            <th class="th-order">№</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="cf in customFields" :key="cf.id">
            <td><input v-model="cf.label" class="te-input"></td>
            <td><input v-model="cf.placeholder" class="te-input"></td>
            <td><input v-model.number="cf.sort_order" type="number" class="te-input order-input"></td>
            <td>
              <button class="te-link-btn" @click="updateCF(cf)">Сохранить</button>
              <button class="te-link-btn danger" @click="deleteCF(cf)">Удалить</button>
            </td>
          </tr>
          <tr v-if="!customFields.length">
            <td colspan="4" class="te-empty">Кастомных полей нет</td>
          </tr>
        </tbody>
      </table>
      <div class="te-custom-add">
        <input v-model="newCF.label" placeholder="Заголовок нового поля" class="te-input">
        <input v-model="newCF.placeholder" placeholder="Плейсхолдер" class="te-input">
        <button class="te-btn" :disabled="!newCF.label" @click="addCF">+ Добавить поле</button>
      </div>
    </div>
  </section>
</template>

<script>
import { useUiStore } from '@/stores/ui';
import {
  getTemplate, uploadTemplate, updateMappings, deleteTemplate,
  getTemplateFields, getTemplateFile,
  listCustomFields, createCustomField, updateCustomField, deleteCustomField,
} from '@/api/attachment-templates';
import XlsxViewer from './XlsxViewer.vue';

export default {
  name: 'AttachmentTemplateEditor',
  components: { XlsxViewer },
  props: {
    uniqueAttachmentId: { type: Number, required: true },
  },
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
    };
  },
  computed: {
    enrichedMappings() {
      return this.mappings.map(m => ({
        ...m,
        fieldLabel: this.getFieldLabel(m.field_path),
      }));
    },
  },
  watch: {
    uniqueAttachmentId: {
      immediate: true,
      handler(v) {
        if (v) this.loadAll();
      },
    },
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
        if (!confirm('Отключить генерацию бланка? Текущий шаблон будет удалён.')) {
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
        useUiStore().success('Шаблон удалён');
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
        this.pendingCellRef = cellRef;
      }
    },
    removeMapping(idx) {
      this.mappings.splice(idx, 1);
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
  },
};
</script>

<style scoped>
.template-editor {
  margin-top: 24px;
  border-top: 1px solid var(--color-border);
  padding-top: 20px;
}

.te-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.te-header h3 {
  margin: 0;
  font-size: 16px;
}

.te-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  cursor: pointer;
}

.te-block {
  margin-bottom: 24px;
  background: var(--color-bg-secondary);
  padding: 14px;
  border-radius: 8px;
}

.te-block h4 {
  margin: 0 0 12px;
  font-size: 14px;
  color: #444;
}

.te-hint {
  margin: 0 0 12px;
  font-size: 12px;
  color: #888;
}

.te-existing {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  font-size: 13px;
}

.te-meta {
  color: #888;
}

.te-upload-form {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  font-size: 13px;
}

.te-upload-form input[type="file"] {
  grid-column: 1 / -1;
}

.te-form-actions {
  grid-column: 1 / -1;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

/* Split panel */
.te-split-panel {
  display: grid;
  grid-template-columns: 1fr 320px;
  gap: 16px;
  min-height: 400px;
}

.te-panel-left {
  min-width: 0;
  overflow: hidden;
}

.te-panel-right {
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow-y: auto;
  max-height: 560px;
}

.te-panel-right h5 {
  margin: 0 0 8px;
  font-size: 13px;
  color: #555;
}

/* Field picker */
.te-field-picker {
  border: 1px solid var(--color-border);
  border-radius: 6px;
  padding: 10px;
  background: #fff;
}

.te-field-group {
  margin-bottom: 8px;
}

.te-field-group-label {
  display: block;
  font-size: 11px;
  color: #888;
  margin-bottom: 4px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.te-field-chip {
  display: inline-block;
  padding: 3px 8px;
  margin: 2px 3px 2px 0;
  border: 1px solid var(--color-border);
  border-radius: 12px;
  font-size: 11px;
  background: #fff;
  cursor: pointer;
  transition: all 0.15s;
}

.te-field-chip:hover {
  border-color: var(--color-primary);
  background: #f0f7ff;
}

.te-field-chip.active {
  background: var(--color-primary);
  color: #fff;
  border-color: var(--color-primary);
}

.te-field-chip.used {
  background: #e8f8e8;
  border-color: #4caf50;
}

.te-chip-mapped {
  font-size: 10px;
  opacity: 0.7;
}

/* Mappings list */
.te-mappings-list {
  border: 1px solid var(--color-border);
  border-radius: 6px;
  padding: 10px;
  background: #fff;
  flex: 1;
  overflow-y: auto;
}

.te-empty-mappings {
  font-size: 12px;
  color: #999;
  text-align: center;
  padding: 12px 0;
}

.te-mapping-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 6px;
  border-radius: 4px;
  font-size: 12px;
  transition: background 0.1s;
}

.te-mapping-row:hover {
  background: #f5f5f5;
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
}

.te-list-badge {
  font-size: 10px;
  background: #e3f2fd;
  color: #1565c0;
  padding: 1px 5px;
  border-radius: 8px;
}

.te-remove-btn {
  background: none;
  border: none;
  color: #d73a3a;
  cursor: pointer;
  font-size: 16px;
  line-height: 1;
  padding: 0 4px;
}

.te-save-actions {
  display: flex;
  justify-content: flex-end;
}

/* Custom fields table */
.te-custom-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  margin-bottom: 10px;
}

.te-custom-table th,
.te-custom-table td {
  padding: 6px 8px;
  text-align: left;
  border-bottom: 1px solid var(--color-border);
}

.th-order { width: 60px; }

.te-input {
  width: 100%;
  padding: 5px 8px;
  border: 1px solid var(--color-border);
  border-radius: 4px;
  font-size: 13px;
}

.order-input { width: 60px; }

.te-empty {
  text-align: center;
  color: #999;
  padding: 16px 0;
}

.te-custom-add {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-top: 10px;
}

.te-btn {
  padding: 6px 14px;
  border-radius: 6px;
  border: 0;
  background: var(--color-primary);
  color: #fff;
  cursor: pointer;
  font-size: 13px;
}

.te-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.te-link-btn {
  background: none;
  border: 0;
  color: var(--color-primary);
  cursor: pointer;
  font-size: 13px;
  padding: 4px 8px;
}

.te-link-btn.danger {
  color: #d73a3a;
}

@media (max-width: 900px) {
  .te-split-panel {
    grid-template-columns: 1fr;
  }
  .te-panel-right {
    max-height: none;
  }
}
</style>
