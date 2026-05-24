<template>
  <section class="acf-section">
    <header class="acf-header">
      <h4>Дополнительные поля вложения</h4>
    </header>
    <table class="acf-table">
      <thead>
        <tr>
          <th>Заголовок</th>
          <th>Плейсхолдер</th>
          <th class="acf-th-order">
            N
          </th>
          <th class="acf-th-actions" />
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
              class="lk-input acf-input"
            >
          </td>
          <td>
            <input
              v-model="cf.placeholder"
              class="lk-input acf-input"
            >
          </td>
          <td>
            <input
              v-model.number="cf.sort_order"
              type="number"
              class="lk-input acf-input acf-input-order"
            >
          </td>
          <td class="acf-row-actions">
            <button
              class="lk-button lk-button--ghost acf-btn"
              @click="updateCF(cf)"
            >
              Сохранить
            </button>
            <button
              class="lk-button lk-button--danger acf-btn"
              @click="deleteCF(cf)"
            >
              Удалить
            </button>
          </td>
        </tr>
        <tr v-if="!customFields.length">
          <td
            colspan="4"
            class="acf-empty"
          >
            Кастомных полей нет
          </td>
        </tr>
      </tbody>
    </table>
    <div class="acf-add-row">
      <input
        v-model="newCF.label"
        placeholder="Заголовок"
        class="lk-input acf-add-input"
      >
      <input
        v-model="newCF.placeholder"
        placeholder="Плейсхолдер"
        class="lk-input acf-add-input"
      >
      <button
        class="lk-button lk-button--primary acf-btn"
        :disabled="!newCF.label"
        @click="addCF"
      >
        + Добавить поле
      </button>
    </div>
  </section>
</template>

<script>
import { useUiStore } from '@/stores/ui';
import {
  listCustomFields, createCustomField, updateCustomField, deleteCustomField,
} from '@/api/attachment-templates';

export default {
  name: 'AttachmentCustomFields',
  props: {
    uniqueAttachmentId: { type: Number, required: true },
  },
  data() {
    return {
      customFields: [],
      newCF: { label: '', placeholder: '' },
    };
  },
  watch: {
    uniqueAttachmentId: {
      immediate: true,
      handler(v) {
        if (v) this.load();
      },
    },
  },
  methods: {
    async load() {
      try {
        const data = await listCustomFields(this.uniqueAttachmentId);
        this.customFields = Array.isArray(data) ? data : [];
      } catch {
        this.customFields = [];
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
        await this.load();
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
        await this.load();
        useUiStore().success('Поле удалено');
      } catch {
        useUiStore().error('Не удалось удалить');
      }
    },
  },
};
</script>

<style scoped>
.acf-section {
  margin-top: 16px;
  border-top: 1px solid var(--color-border);
  padding-top: 16px;
}

.acf-header h4 {
  margin: 0 0 12px;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
}

.acf-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  margin-bottom: 12px;
}

.acf-table th {
  padding: 8px 10px;
  text-align: left;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-muted);
  border-bottom: 2px solid var(--color-border);
}

.acf-table td {
  padding: 6px 8px;
  border-bottom: 1px solid var(--color-border);
  vertical-align: middle;
}

.acf-th-order { width: 60px; }
.acf-th-actions { width: 180px; }

.acf-input {
  padding: 5px 8px !important;
  font-size: 13px !important;
}

.acf-input-order { width: 60px; }

.acf-row-actions {
  display: flex;
  gap: 6px;
}

.acf-btn {
  padding: 4px 10px !important;
  font-size: 11px !important;
}

.acf-empty {
  text-align: center;
  color: var(--color-text-muted);
  padding: 16px 0 !important;
}

.acf-add-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.acf-add-input {
  flex: 1;
  padding: 6px 10px !important;
  font-size: 13px !important;
}
</style>
