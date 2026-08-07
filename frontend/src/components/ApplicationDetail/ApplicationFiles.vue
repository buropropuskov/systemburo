<template>
  <div
    v-if="files.length > 0"
    class="app-files-strip"
    data-testid="application-files"
  >
    <button
      v-for="file in files"
      :key="file.id"
      type="button"
      class="app-files-strip__tile"
      :class="`app-files-strip__tile--${kind(file)}`"
      data-testid="application-file-item"
      :title="`${file.file_name} — ${formatBytes(file.file_size)}`"
      :disabled="downloadingId === file.id"
      @click="download(file)"
    >
      <span
        class="app-files-strip__ext"
        :class="`app-files-strip__ext--${kind(file)}`"
      >{{ ext(file) }}</span>
      <span class="app-files-strip__meta">
        <span class="app-files-strip__name">{{ file.file_name }}</span>
        <span class="app-files-strip__size">{{ formatBytes(file.file_size) }}</span>
      </span>
      <span
        v-if="canRemove"
        class="app-files-strip__remove"
        role="button"
        aria-label="Убрать файл"
        @click.stop="remove(file)"
      >×</span>
    </button>
  </div>
</template>

<script setup>
/**
 * Файлы, приложенные к заявке при подаче (#1721).
 *
 * Плитки над текстом письма, как вложения в почтовом клиенте: заявитель
 * прикладывает документы к сопроводительному письму, и читаются они вместе с ним.
 * Скачивание идёт через Blob с Bearer-токеном - файлы раздаются только под
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
  /** Крестик показывается администратору: состав заявки после подачи неизменен. */
  canRemove: { type: Boolean, default: false },
});

const files = ref([]);
const downloadingId = ref(null);
const notifications = useDeletionsStore();

/** Расширение для плашки: без точки и не длиннее четырёх букв, иначе плитка распухает. */
function ext(file) {
  const raw = (file.file_name.split('.').pop() || '').toLowerCase();
  return raw.length > 4 || raw === file.file_name.toLowerCase() ? 'файл' : raw;
}

/**
 * Семейство формата - для цвета плитки и плашки.
 *
 * Расширение имени старше типа: docx, xlsx и pptx неразличимы по сигнатуре, и у
 * файлов, загруженных до уточнения типа, в базе лежит docx независимо от того,
 * что это было. Имя же заявитель принёс своё, и оно не врёт.
 */
function kind(file) {
  const byName = {
    xlsx: 'sheet', xls: 'sheet', csv: 'sheet',
    docx: 'doc', doc: 'doc',
    pdf: 'pdf',
    png: 'image', jpg: 'image', jpeg: 'image', webp: 'image', gif: 'image',
    pptx: 'slides', ppt: 'slides',
  }[(file.file_name.split('.').pop() || '').toLowerCase()];
  if (byName) return byName;

  if ((file.mime_type || '').startsWith('image/')) return 'image';
  if (file.mime_type === 'application/pdf') return 'pdf';
  if (/sheet|excel/.test(file.mime_type || '')) return 'sheet';
  if (/word|document/.test(file.mime_type || '')) return 'doc';
  return 'other';
}

async function load() {
  if (!props.applicationId) return;
  try {
    files.value = await fetchApplicationFiles(props.applicationId);
  } catch (error) {
    notifications.notify({ bold: error.message, type: 'error' });
    files.value = [];
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
.app-files-strip {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-bottom: 12px;
}

.app-files-strip__tile {
    display: flex;
    align-items: center;
    gap: 8px;
    max-width: 240px;
    padding: 6px 10px 6px 6px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface);
    cursor: pointer;
    text-align: left;
    transition: border-color 150ms ease, box-shadow 150ms ease, opacity 150ms ease;
}

/* Подсветка вместо подъёма: плитки стоят в ряд над письмом, и сдвиг одной из них
   дёргает взгляд по всей строке. */
.app-files-strip__tile:hover:not(:disabled) {
    border-color: var(--primary);
    box-shadow: 0 0 0 2px rgba(var(--primary-rgb, 79, 124, 255), 0.15);
}

.app-files-strip__tile:disabled {
    opacity: 0.6;
    cursor: default;
}

.app-files-strip__ext {
    flex-shrink: 0;
    width: 34px;
    height: 34px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 8px;
    /* Четыре буквы (xlsx, docx, pptx) должны помещаться в плашку целиком:
       при 10px они не влезали и обрезались. */
    font-size: 8px;
    font-weight: 700;
    letter-spacing: -0.2px;
    text-transform: uppercase;
    color: #fff;
    background: var(--text-muted);
}

.app-files-strip__ext--pdf { background: #c0392b; }
.app-files-strip__ext--image { background: #2e86c1; }
.app-files-strip__ext--sheet { background: #1e8449; }
.app-files-strip__ext--doc { background: #2874a6; }
.app-files-strip__ext--slides { background: #d35400; }

/* Плитка окрашена в тон своего формата: прозрачная заливка поверх поверхности
   держит контраст и в светлой, и в тёмной теме, в отличие от сплошного цвета. */
.app-files-strip__tile--pdf {
    border-color: rgba(192, 57, 43, 0.45);
    background: rgba(192, 57, 43, 0.10);
}

.app-files-strip__tile--image {
    border-color: rgba(46, 134, 193, 0.45);
    background: rgba(46, 134, 193, 0.10);
}

.app-files-strip__tile--sheet {
    border-color: rgba(30, 132, 73, 0.45);
    background: rgba(30, 132, 73, 0.10);
}

.app-files-strip__tile--doc {
    border-color: rgba(40, 116, 166, 0.45);
    background: rgba(40, 116, 166, 0.10);
}

.app-files-strip__tile--slides {
    border-color: rgba(211, 84, 0, 0.45);
    background: rgba(211, 84, 0, 0.10);
}

.app-files-strip__meta {
    display: flex;
    flex-direction: column;
    min-width: 0;
}

.app-files-strip__name {
    font-size: 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.app-files-strip__size {
    font-size: 11px;
    color: var(--text-muted);
}

.app-files-strip__remove {
    flex-shrink: 0;
    font-size: 16px;
    line-height: 1;
    color: var(--text-muted);
    padding: 0 2px;
}

.app-files-strip__remove:hover {
    color: var(--danger-text);
}

@media (max-width: 767px) {
    .app-files-strip__tile {
        max-width: 100%;
        flex: 1 1 100%;
    }
}
</style>
