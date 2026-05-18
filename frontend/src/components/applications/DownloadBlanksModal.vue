<template>
  <div class="modal-overlay" data-testid="download-blanks-modal" @click.self="$emit('close')">
    <div class="modal-content">
      <h3 class="modal-title">Скачивание бланков заявки</h3>

      <div v-if="isLoading" class="loader">Загрузка вложений...</div>
      <div v-else-if="error" class="error">{{ error }}</div>
      <div v-else-if="!eligibleAttachments.length" class="empty">
        У заявки нет вложений с настроенным шаблоном бланка.
      </div>
      <div v-else class="attachments">
        <label v-for="att in eligibleAttachments" :key="att.id" class="attachment-row">
          <input
            type="checkbox"
            :value="att.id"
            v-model="selectedIds"
          >
          <span class="att-name">{{ att.display_name || att.name }}</span>
          <span class="att-type">{{ attachmentTypeLabel(att.attachment_type) }}</span>
          <button
            class="link-btn"
            :disabled="downloadingId === att.id"
            @click.prevent="downloadOne(att)"
          >
            {{ downloadingId === att.id ? '...' : 'Скачать' }}
          </button>
        </label>
      </div>

      <footer v-if="eligibleAttachments.length" class="modal-actions">
        <button class="lk-btn lk-btn--ghost" @click="$emit('close')">Закрыть</button>
        <button
          class="lk-btn"
          :disabled="!selectedIds.length || downloadingAll"
          @click="downloadSelected"
        >
          {{ downloadingAll ? 'Скачивание...' : `Скачать выбранное (${selectedIds.length})` }}
        </button>
        <button
          class="lk-btn lk-btn--ghost"
          :disabled="downloadingAll"
          @click="downloadAll"
        >
          Скачать все
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
      // На бэке нет флага has_template в Attachment - показываем все, при
      // скачивании 404/400 если шаблона нет (ошибка в toast).
      return this.attachments;
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
      // ZIP-выгрузка пока не реализована на бэке - скачиваем по одному.
      // TODO: backend endpoint /applications/:id/blanks?attachment_ids= → отдельная задача.
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
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal-content {
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  width: 520px;
  max-width: 92vw;
  max-height: 80vh;
  overflow-y: auto;
}

.modal-title {
  margin: 0 0 16px;
  font-size: 18px;
}

.loader, .empty, .error {
  text-align: center;
  color: #888;
  padding: 24px 0;
}

.error {
  color: #d73a3a;
}

.attachment-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 8px;
  border-bottom: 1px solid var(--color-border);
  cursor: pointer;
}

.attachment-row:hover {
  background: var(--color-bg-secondary);
}

.att-name {
  flex: 1;
  font-size: 14px;
}

.att-type {
  font-size: 12px;
  color: #888;
}

.link-btn {
  background: none;
  border: 0;
  color: var(--color-primary);
  cursor: pointer;
  font-size: 13px;
  padding: 4px 8px;
}

.link-btn:disabled {
  opacity: 0.5;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 16px;
}

.lk-btn {
  padding: 8px 16px;
  border-radius: 8px;
  border: 0;
  background: var(--color-primary);
  color: #fff;
  cursor: pointer;
}

.lk-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.lk-btn--ghost {
  background: transparent;
  border: 1px solid var(--color-border);
  color: #333;
}
</style>
