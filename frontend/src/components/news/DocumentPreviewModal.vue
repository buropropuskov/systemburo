<template>
  <BaseModal
    :show="show"
    :title="doc?.title || 'Документ'"
    width="520px"
    @close="$emit('close')"
  >
    <div
      v-if="doc"
      class="doc-view"
      data-testid="document-preview"
    >
      <div class="doc-view__head">
        <FileTypeIcon
          :ext="doc.file_ext || 'file'"
          :size="40"
        />
        <div class="doc-view__file">
          <div class="doc-view__name">
            {{ doc.file_name }}
          </div>
          <div class="doc-view__meta">
            {{ formatSize(doc.file_size) }} &middot; опубликован {{ formatDate(doc.published_at || doc.created_at) }}
          </div>
        </div>
      </div>

      <div
        v-if="doc.description"
        class="doc-view__row"
      >
        <span class="doc-view__label">Описание</span>
        <p class="doc-view__text">
          {{ doc.description }}
        </p>
      </div>

      <div class="doc-view__row">
        <span class="doc-view__label">Пояснение бюро</span>
        <p
          v-if="doc.comment"
          class="doc-view__text"
          data-testid="document-comment"
        >
          {{ doc.comment }}
        </p>
        <p
          v-else
          class="doc-view__empty"
          data-testid="document-comment-empty"
        >
          Бюро пока ничего не пояснило к этому документу.
        </p>
      </div>
    </div>

    <template #actions>
      <button
        type="button"
        class="lk-button lk-button--ghost"
        @click="$emit('close')"
      >
        Закрыть
      </button>
      <button
        type="button"
        class="lk-button lk-button--primary"
        data-testid="document-download"
        :disabled="downloading"
        @click="$emit('download', doc)"
      >
        {{ downloading ? 'Скачивание...' : 'Скачать' }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import FileTypeIcon from '@/components/ui/FileTypeIcon.vue';
import { formatMomentDate } from '@/utils/datetime';

/**
 * Окно документа с обзора: что за файл, описание, пояснение бюро и скачивание.
 *
 * Сам файл здесь не показывается - решение владельца: половина документов это
 * бланки xlsx, которые всё равно заполняют у себя, а не читают с экрана.
 */
export default {
  name: 'DocumentPreviewModal',
  components: { BaseModal, FileTypeIcon },
  props: {
    show: { type: Boolean, required: true },
    doc: { type: Object, default: null },
    downloading: { type: Boolean, default: false },
  },
  emits: ['close', 'download'],
  methods: {
    formatDate(dt) {
      return dt ? formatMomentDate(new Date(dt)) : '';
    },
    /** Размер строкой: килобайты до мегабайта, дальше мегабайты с одним знаком. */
    formatSize(bytes) {
      if (!bytes) return 'размер неизвестен';
      if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} КБ`;
      return `${(bytes / 1024 / 1024).toFixed(1)} МБ`;
    },
  },
};
</script>

<style scoped>
.doc-view {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.doc-view__head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--border);
}

.doc-view__file {
  min-width: 0;
}

.doc-view__name {
  font-size: 14px;
  color: var(--text);
  overflow-wrap: anywhere;
}

.doc-view__meta {
  margin-top: 2px;
  font-size: 12px;
  color: var(--text-muted);
}

.doc-view__row {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.doc-view__label {
  font-size: 13px;
  color: var(--text-muted);
}

.doc-view__text {
  margin: 0;
  font-size: 14px;
  line-height: 1.5;
  color: var(--text);
  white-space: pre-line;
  overflow-wrap: anywhere;
}

.doc-view__empty {
  margin: 0;
  font-size: 14px;
  color: var(--text-muted);
}
</style>
