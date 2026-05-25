<template>
  <div
    class="dbm-overlay"
    data-testid="download-blanks-modal"
    @click.self="$emit('close')"
  >
    <div class="dbm-modal">
      <div class="dbm-header">
        <h3 class="dbm-title">
          Скачивание бланков
        </h3>
        <button
          class="dbm-close"
          @click="$emit('close')"
        >
          &times;
        </button>
      </div>

      <div
        v-if="isLoading"
        class="dbm-state"
      >
        <span class="dbm-spinner" />
      </div>
      <div
        v-else-if="error"
        class="dbm-state dbm-state--error"
      >
        {{ error }}
      </div>
      <div
        v-else-if="!eligibleAttachments.length"
        class="dbm-state"
      >
        У заявки нет вложений с настроенным шаблоном бланка.
      </div>
      <div
        v-else
        class="dbm-list"
      >
        <label
          v-for="att in eligibleAttachments"
          :key="att.id"
          class="dbm-item"
          :class="{ selected: selectedIds.includes(att.id) }"
        >
          <input
            v-model="selectedIds"
            type="checkbox"
            :value="att.id"
            class="dbm-checkbox"
          >
          <div class="dbm-item-info">
            <span class="dbm-item-name">{{ att.attachment_display_name || att.unique_attachment_display_name || att.attachment_name }}</span>
            <span
              v-if="attachmentTypeLabel(att.attachment_type)"
              class="dbm-item-type"
            >{{ attachmentTypeLabel(att.attachment_type) }}</span>
          </div>
          <button
            class="dbm-item-download"
            :disabled="downloadingId === att.id"
            @click.prevent="downloadOne(att)"
          >
            {{ downloadingId === att.id ? '...' : 'Скачать' }}
          </button>
        </label>
      </div>

      <footer
        v-if="eligibleAttachments.length"
        class="dbm-footer"
      >
        <button
          class="dbm-btn dbm-btn--ghost"
          @click="$emit('close')"
        >
          Закрыть
        </button>
        <button
          class="dbm-btn dbm-btn--ghost"
          :disabled="downloadingAll"
          @click="downloadAll"
        >
          Скачать все
        </button>
        <button
          class="dbm-btn dbm-btn--primary"
          :disabled="!selectedIds.length || downloadingAll"
          @click="downloadSelected"
        >
          {{ downloadingAll ? 'Скачивание...' : `Скачать (${selectedIds.length})` }}
        </button>
      </footer>
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client';
import { useUiStore } from '@/stores/ui';
import { downloadBlank, saveBlobAs } from '@/api/attachment-templates';

const TYPE_LABELS = {
  cars: 'Автомобили',
  people: 'Сотрудники',
  items: 'Имущество',
};

export default {
  name: 'DownloadBlanksModal',
  props: {
    applicationId: { type: Number, required: true },
  },
  emits: ['close'],
  data() {
    return {
      attachments: [],
      selectedIds: [],
      isLoading: false,
      downloadingId: null,
      downloadingAll: false,
      error: '',
    };
  },
  computed: {
    eligibleAttachments() {
      return this.attachments.filter(att => att.has_template);
    },
  },
  mounted() {
    this.load();
  },
  methods: {
    async load() {
      this.isLoading = true;
      try {
        const res = await apiRequest(`/applications/${this.applicationId}/attachments`);
        const data = await res.json();
        this.attachments = Array.isArray(data) ? data : [];
      } catch {
        this.error = 'Не удалось загрузить вложения';
      } finally {
        this.isLoading = false;
      }
    },
    attachmentTypeLabel(t) {
      return TYPE_LABELS[t] || t || '';
    },
    async downloadOne(att) {
      this.downloadingId = att.id;
      try {
        const { blob, filename } = await downloadBlank(this.applicationId, att.id);
        saveBlobAs(blob, filename);
      } catch (err) {
        useUiStore().error(err.message || 'Не удалось скачать');
      } finally {
        this.downloadingId = null;
      }
    },
    async downloadSelected() {
      if (!this.selectedIds.length) return;
      this.downloadingAll = true;
      try {
        for (const id of this.selectedIds) {
          await this.downloadOne({ id });
        }
        useUiStore().success(`Скачано: ${this.selectedIds.length}`);
      } finally {
        this.downloadingAll = false;
      }
    },
    async downloadAll() {
      this.selectedIds = this.eligibleAttachments.map(a => a.id);
      await this.downloadSelected();
    },
  },
};
</script>

<style scoped>
.dbm-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.dbm-modal {
  background: #fff;
  border-radius: 30px;
  padding: 0;
  width: 480px;
  max-width: 92vw;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.12);
}

.dbm-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px 0;
}

.dbm-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text);
}

.dbm-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  background: var(--color-bg-secondary);
  border-radius: 50%;
  font-size: 18px;
  color: var(--color-text-muted);
  cursor: pointer;
  transition: all 0.15s;
  line-height: 1;
}

.dbm-close:hover {
  background: var(--color-border);
  color: var(--color-text);
}

.dbm-state {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 24px;
  color: var(--color-text-muted);
  font-size: 13px;
}

.dbm-state--error {
  color: var(--color-danger);
}

.dbm-spinner {
  display: block;
  width: 28px;
  height: 28px;
  border: 3px solid var(--color-border);
  border-top-color: var(--color-primary);
  border-radius: 50%;
  animation: dbm-spin 0.7s linear infinite;
}

@keyframes dbm-spin {
  to { transform: rotate(360deg); }
}

.dbm-list {
  padding: 16px 24px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.dbm-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  cursor: pointer;
  transition: all 0.15s;
  background: #fff;
}

.dbm-item:hover {
  border-color: var(--color-primary);
  background: #f8faff;
}

.dbm-item.selected {
  border-color: var(--color-primary);
  background: #eef4ff;
}

.dbm-checkbox {
  accent-color: var(--color-primary);
  width: 16px;
  height: 16px;
  flex-shrink: 0;
}

.dbm-item-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.dbm-item-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dbm-item-type {
  font-size: 11px;
  color: var(--color-text-muted);
}

.dbm-item-download {
  flex-shrink: 0;
  background: none;
  border: none;
  color: var(--color-primary);
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  padding: 4px 8px;
  border-radius: var(--radius-pill);
  transition: all 0.15s;
}

.dbm-item-download:hover {
  background: #eef4ff;
}

.dbm-item-download:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.dbm-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 0 24px 20px;
}

.dbm-btn {
  padding: 8px 18px;
  border-radius: var(--radius-pill);
  border: none;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}

.dbm-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.dbm-btn--primary {
  background: var(--color-primary);
  color: #fff;
}

.dbm-btn--primary:hover:not(:disabled) {
  filter: brightness(0.92);
}

.dbm-btn--ghost {
  background: transparent;
  border: 1px solid var(--color-border);
  color: var(--color-text);
}

.dbm-btn--ghost:hover:not(:disabled) {
  border-color: var(--color-primary);
  color: var(--color-primary);
}
</style>
