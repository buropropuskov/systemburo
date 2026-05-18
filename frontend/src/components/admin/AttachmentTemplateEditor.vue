<template>
  <section class="template-editor">
    <header class="te-header">
      <h3>Excel-бланк (#183)</h3>
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
          <button class="te-link-btn danger" @click="onDeleteTemplate">Удалить шаблон</button>
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

      <!-- Маппинг ячеек -->
      <div v-if="template && template.file_path" class="te-block">
        <h4>Связь ячейки → поля заявки</h4>
        <table class="te-mapping-table">
          <thead>
            <tr>
              <th class="th-cell">Ячейка</th>
              <th>Поле</th>
              <th>Список</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(m, idx) in mappings" :key="idx">
              <td>
                <input
                  v-model="m.cell_ref"
                  placeholder="A1"
                  maxlength="10"
                  class="te-input cell-input"
                >
              </td>
              <td>
                <select v-model="m.field_path" class="te-select" @change="onFieldChange(m)">
                  <option value="" disabled>Выберите поле</option>
                  <optgroup v-for="g in fieldGroups" :key="g.group" :label="g.label">
                    <option v-for="f in g.fields" :key="f.path" :value="f.path">{{ f.label }}</option>
                  </optgroup>
                </select>
              </td>
              <td>
                <input type="checkbox" :checked="m.is_list_field" disabled>
              </td>
              <td>
                <button class="te-link-btn danger" @click="mappings.splice(idx, 1)">Удалить</button>
              </td>
            </tr>
            <tr v-if="!mappings.length">
              <td colspan="4" class="te-empty">Нет маппингов. Добавьте первый.</td>
            </tr>
          </tbody>
        </table>
        <div class="te-mapping-actions">
          <button class="te-link-btn" @click="addMapping">+ Добавить маппинг</button>
          <button class="te-btn" :disabled="savingMappings" @click="saveMappings">
            {{ savingMappings ? 'Сохранение...' : 'Сохранить маппинги' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Кастомные поля - всегда доступны (не зависят от toggle бланка) -->
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
  getTemplateFields,
  listCustomFields, createCustomField, updateCustomField, deleteCustomField,
} from '@/api/attachment-templates';

export default {
  name: 'AttachmentTemplateEditor',
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
    };
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
      } catch {
        this.template = null;
        this.mappings = [];
        this.enabled = false;
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
        await this.loadTemplate();
      } catch {
        useUiStore().error('Не удалось удалить шаблон');
      }
    },
    addMapping() {
      this.mappings.push({ cell_ref: '', field_path: '', is_list_field: false });
    },
    onFieldChange(m) {
      // Авто-выставление is_list_field на основе field_path.
      for (const g of this.fieldGroups) {
        const f = g.fields.find(x => x.path === m.field_path);
        if (f) {
          m.is_list_field = !!f.is_list;
          return;
        }
      }
    },
    async saveMappings() {
      this.savingMappings = true;
      try {
        const payload = this.mappings.filter(m => m.cell_ref && m.field_path);
        await updateMappings(this.uniqueAttachmentId, payload);
        useUiStore().success('Маппинги сохранены');
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
      if (!confirm(`Удалить поле «${cf.label}»?`)) return;
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

.te-mapping-table,
.te-custom-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  margin-bottom: 10px;
}

.te-mapping-table th,
.te-mapping-table td,
.te-custom-table th,
.te-custom-table td {
  padding: 6px 8px;
  text-align: left;
  border-bottom: 1px solid var(--color-border);
}

.th-cell { width: 90px; }
.th-order { width: 60px; }

.te-input {
  width: 100%;
  padding: 5px 8px;
  border: 1px solid var(--color-border);
  border-radius: 4px;
  font-size: 13px;
}

.cell-input { width: 70px; }
.order-input { width: 60px; }

.te-select {
  width: 100%;
  padding: 5px 8px;
  border: 1px solid var(--color-border);
  border-radius: 4px;
  font-size: 13px;
}

.te-empty {
  text-align: center;
  color: #999;
  padding: 16px 0;
}

.te-mapping-actions,
.te-custom-add {
  display: flex;
  gap: 10px;
  align-items: center;
}

.te-custom-add {
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
</style>
