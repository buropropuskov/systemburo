<template>
  <div
    v-if="files.length > 0 || loading"
    class="app-files-view"
    data-testid="application-files"
  >
    <div class="app-files-view__header">
      <h4>Файлы к заявке</h4>
    </div>

    <div
      v-if="loading"
      class="app-files-view__empty"
    >
      Загрузка...
    </div>

    <div
      v-for="file in files"
      v-else
      :key="file.id"
      class="app-files-view__item"
      data-testid="application-file-item"
    >
      <button
        type="button"
        class="app-files-view__name"
        :disabled="downloadingId === file.id"
        @click="download(file)"
      >
        {{ file.file_name }}
      </button>
      <span class="app-files-view__size">{{ formatBytes(file.file_size) }}</span>
      <button
        v-if="canRemove"
        type="button"
        class="app-files-view__remove"
        aria-label="Убрать файл"
        @click="remove(file)"
      >
        ×
      </button>
    </div>
  </div>
</template>

<script setup>
/**
 * Файлы, приложенные к заявке при подаче (#1721).
 *
 * Скачивание идёт через Blob с Bearer-токеном: файлы раздаются только под
 * доступом к заявке, а обычная ссылка заголовок авторизации не несёт.
 */
import { ref, watch, onMounted } from 'vue';
import {
  fetchApplicationFiles,
  downloadApplicationFile,
  deleteApplicationFile,
} from '@/api/applicationFiles';
import { formatBytes } from '@/utils/download';
import { useDeletionsStore } from '@/stores/deletions';

const props = defineProps({
  applicationId: { type: Number, required: true },
  /** Показывать крестик: убрать файл может подавший заявку до её закрытия. */
  canRemove: { type: Boolean, default: false },
});

const files = ref([]);
const loading = ref(false);
const downloadingId = ref(null);
const notifications = useDeletionsStore();

async function load() {
  if (!props.applicationId) return;
  loading.value = true;
  try {
    files.value = await fetchApplicationFiles(props.applicationId);
  } catch (error) {
    notifications.notify({ bold: error.message, type: 'error' });
    files.value = [];
  } finally {
    loading.value = false;
  }
}

async function download(file) {
  downloadingId.value = file.id;
  try {
    await downloadApplicationFile(props.applicationId, file.id, file.file_name);
  } catch (error) {
    notifications.notify({ bold: error.message, type: 'error' });
  } finally {
    downloadingId.value = null;
  }
}

async function remove(file) {
  try {
    await deleteApplicationFile(props.applicationId, file.id);
    files.value = files.value.filter((f) => f.id !== file.id);
    notifications.notify({ prefix: 'Файл ', bold: file.file_name, suffix: ' убран', type: 'success' });
  } catch (error) {
    notifications.notify({ bold: error.message, type: 'error' });
  }
}

onMounted(load);
watch(() => props.applicationId, load);
</script>

<style scoped>
.app-files-view {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.app-files-view__header h4 {
    margin: 0;
    font-size: 14px;
}

.app-files-view__empty {
    font-size: 13px;
    color: var(--text-muted);
}

.app-files-view__item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface);
}

.app-files-view__name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: left;
    border: none;
    background: none;
    cursor: pointer;
    font-size: 13px;
    color: var(--primary);
    padding: 0;
}

.app-files-view__name:hover:not(:disabled) {
    text-decoration: underline;
}

.app-files-view__size {
    font-size: 12px;
    color: var(--text-muted);
    white-space: nowrap;
}

.app-files-view__remove {
    border: none;
    background: none;
    cursor: pointer;
    font-size: 18px;
    line-height: 1;
    color: var(--text-muted);
    padding: 0 2px;
}

.app-files-view__remove:hover {
    color: var(--danger-text);
}
</style>
