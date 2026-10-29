<template>
  <div class="app-files">
    <div class="app-files__controls">
      <input
        ref="fileInput"
        type="file"
        multiple
        class="app-files__input"
        :accept="accept"
        @change="onSelect"
      >
      <button
        type="button"
        class="app-files__add"
        :disabled="uploading || files.length >= maxCount"
        data-testid="app-files-add"
        :title="`Прикрепить файл (${files.length} из ${maxCount})`"
        @click="$refs.fileInput.click()"
      >
        <svg
          width="15"
          height="15"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        ><path d="M19.5 11l-7.8 7.8a4 4 0 0 1-5.7-5.7l8-8a2.5 2.5 0 0 1 3.6 3.6l-7.6 7.6a1 1 0 0 1-1.5-1.4l6.8-6.8" /></svg>
        <span>{{ uploading ? 'Загрузка...' : 'Прикрепить' }}</span>
      </button>
      <button v-if="canManageDrafts" type="button" class="app-files__add" @click="openDrafts">Файлы-черновики</button>
      <span v-if="quota" class="app-files__quota">{{ quota.usage.saved_count + quota.usage.reserved_count }} из {{ quota.limits.max_count }} · {{ formatBytes(quota.usage.saved_bytes + quota.usage.reserved_bytes) }} из {{ formatBytes(quota.limits.max_bytes) }}</span>
    </div>

    <TransitionGroup
      name="app-files-list"
      tag="div"
      class="app-files__list"
    >
      <div
        v-for="file in files"
        :key="file.id"
        class="app-files__item"
        data-testid="app-files-item"
        :title="`${file.file_name} — ${formatBytes(file.file_size)}`"
      >
        <span
          class="app-files__ext"
          :class="`app-files__ext--${kind(file)}`"
        >{{ ext(file) }}</span>
        <span class="app-files__meta">
          <span class="app-files__name">{{ file.file_name }}</span>
          <span class="app-files__size">{{ formatBytes(file.file_size) }}</span>
        </span>
        <button
          type="button"
          class="app-files__remove"
          aria-label="Убрать файл"
          :disabled="uploading"
          @click="remove(file)"
        >
          ×
        </button>
      </div>
    </TransitionGroup>
    <ApplicationDraftFilesModal v-if="draftsOpened && canManageDrafts" :show="showDrafts" @close="showDrafts = false" @deleted="onDraftDeleted" @usage="onUsage" />
  </div>
</template>

<script setup>
/**
 * Прикрепление файлов к заявке (#1721).
 *
 * Файлы уходят на сервер сразу при выборе и живут черновиками до подачи: сама
 * подача привязывает их по списку id. Поэтому компонент отдаёт наверх именно
 * id, а не объекты File - к моменту отправки формы они уже на диске сервера.
 */
import { ref, computed, watch, onMounted, onBeforeUnmount, defineAsyncComponent } from 'vue';
import { uploadApplicationFiles, deleteApplicationDraftFile, fetchApplicationDraftFiles } from '@/api/applicationFiles';
import { formatBytes } from '@/utils/download';
import { useDeletionsStore } from '@/stores/deletions';
import { useAuthStore } from '@/stores/auth';

const ApplicationDraftFilesModal = defineAsyncComponent(() => import('./ApplicationDraftFilesModal.vue'));

const props = defineProps({
  maxCount: { type: Number, default: 10 },
  accept: { type: String, default: 'image/jpeg,image/png,image/webp,application/pdf,.docx,.xlsx' },
});

const model = defineModel({ type: Array, default: () => [] });

const files = ref([]);
const uploading = ref(false);
const fileInput = ref(null);
const notifications = useDeletionsStore();
const auth = useAuthStore();
const canManageDrafts = computed(() => !auth.isImpersonating);
const showDrafts = ref(false);
const draftsOpened = ref(false);
function openDrafts() { draftsOpened.value = true; showDrafts.value = true; }
watch(canManageDrafts, allowed => { if (!allowed) { showDrafts.value = false; draftsOpened.value = false; } });
const quota = ref(null);
let active = true;
let usageSequence = 0;
async function refreshUsage() {
  if (!canManageDrafts.value) return;
  const sequence = ++usageSequence;
  try { const result = await fetchApplicationDraftFiles(1, 1); if (active && sequence === usageSequence) quota.value = result; } catch { /* The list provides retry and a visible error. */ }
}
function onFocus() { if (!uploading.value) refreshUsage(); }
function onDraftDeleted(id) { ++usageSequence; files.value = files.value.filter(file => file.id !== id); syncModel(); }
function onUsage(data) { ++usageSequence; quota.value = data; }
onMounted(() => { refreshUsage(); window.addEventListener('focus', onFocus); });
onBeforeUnmount(() => { active = false; window.removeEventListener('focus', onFocus); });

const remaining = computed(() => props.maxCount - files.value.length);

async function onSelect(event) {
  const picked = Array.from(event.target.files || []);
  // Сбрасываем сразу: без этого повторный выбор того же файла не даёт события.
  event.target.value = '';
  if (picked.length === 0) return;

  if (picked.length > remaining.value) {
    notifications.notify({
      prefix: 'К заявке можно приложить не больше ',
      bold: `${props.maxCount} файлов`,
      type: 'error',
    });
    return;
  }

  uploading.value = true;
  try {
    const saved = await uploadApplicationFiles(picked);
    files.value = [...files.value, ...saved];
    syncModel();
  } catch (error) {
    notifications.notify({ bold: error.message, type: 'error' });
  } finally {
    uploading.value = false;
    await refreshUsage();
  }
}

async function remove(file) {
  try {
    await deleteApplicationDraftFile(file.id);
    files.value = files.value.filter((f) => f.id !== file.id);
    syncModel();
  } catch (error) {
    notifications.notify({ bold: error.message, type: 'error' });
  } finally {
    await refreshUsage();
  }
}

/** Расширение для плашки: без точки и не длиннее четырёх букв, иначе плитка распухает. */
function ext(file) {
  const raw = (file.file_name.split('.').pop() || '').toLowerCase();
  return raw.length > 4 || raw === file.file_name.toLowerCase() ? 'файл' : raw;
}

/** Семейство формата по расширению имени: офисные типы по сигнатуре неразличимы. */
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
  return 'other';
}

function syncModel() {
  model.value = files.value.map((f) => f.id);
}

/** Очистка после успешной подачи: файлы уже привязаны к заявке. */
function reset() {
  files.value = [];
  model.value = [];
  refreshUsage();
}

defineExpose({ reset });
</script>

<style scoped>
.app-files {
    display: flex;
    align-items: stretch;
    flex-direction: column;
    width: 100%;
    flex-wrap: wrap;
    gap: 6px;
}



.app-files__controls {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
}

.app-files__quota { font-size: 10px; color: var(--text-muted); }

/* Кнопка компактная: она лишь открывает выбор файла и не должна спорить по весу
   с «Отправить заявку» в той же колонке. */
.app-files__add {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 3px 8px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: transparent;
    color: var(--text-muted);
    font-size: 11px;
    line-height: 16px;
    cursor: pointer;
}

.app-files__add:hover:not(:disabled) {
    color: var(--text);
    border-color: var(--text-muted);
}

.app-files__add:disabled {
    opacity: 0.5;
    cursor: default;
}

.app-files__input {
    display: none;
}


.app-files__list {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 160px), 1fr));
    gap: 6px;
    width: 100%;
}

.app-files__item {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    min-width: 0;
    padding: 3px 8px 3px 4px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface);
}

.app-files__ext {
    flex-shrink: 0;
    width: 24px;
    height: 24px;
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

.app-files__ext--pdf { background: #c0392b; }
.app-files__ext--image { background: #2e86c1; }
.app-files__ext--sheet { background: #1e8449; }
.app-files__ext--doc { background: #2874a6; }
.app-files__ext--slides { background: #d35400; }

.app-files__meta {
    display: flex;
    flex-direction: column;
    min-width: 0;
    flex: 1;
}

.app-files__name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 12px;
}

.app-files__size {
    font-size: 11px;
    color: var(--text-muted);
    white-space: nowrap;
}

.app-files__remove {
    flex-shrink: 0;
    border: none;
    background: none;
    cursor: pointer;
    font-size: 18px;
    line-height: 1;
    color: var(--text-muted);
    padding: 0 2px;
}

.app-files__remove:hover:not(:disabled) {
    color: var(--danger-text);
}

/* Только transform и opacity: список живёт внутри формы, дёргать её высоту нельзя. */
.app-files-list-enter-active,
.app-files-list-leave-active {
    transition: opacity 200ms ease, transform 200ms ease;
}

.app-files-list-enter-from,
.app-files-list-leave-to {
    opacity: 0;
    transform: translateY(-4px);
}
</style>
