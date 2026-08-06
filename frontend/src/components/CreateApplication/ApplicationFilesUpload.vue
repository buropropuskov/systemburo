<template>
  <div class="app-files">
    <div class="app-files__head">
      <span class="app-files__label">Файлы к заявке</span>
      <span class="app-files__hint">
        Не прикрепляйте копии паспортов: номер уже указан в карточке человека
      </span>
    </div>

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
        class="lk-button lk-button--secondary app-files__add"
        :disabled="uploading || files.length >= maxCount"
        data-testid="app-files-add"
        @click="$refs.fileInput.click()"
      >
        {{ uploading ? 'Загрузка...' : 'Прикрепить файл' }}
      </button>
      <span class="app-files__counter">{{ files.length }} из {{ maxCount }}</span>
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
      >
        <span class="app-files__name">{{ file.file_name }}</span>
        <span class="app-files__size">{{ formatBytes(file.file_size) }}</span>
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
import { ref, computed } from 'vue';
import { uploadApplicationFiles, deleteApplicationDraftFile } from '@/api/applicationFiles';
import { formatBytes } from '@/utils/download';
import { useDeletionsStore } from '@/stores/deletions';

const props = defineProps({
  maxCount: { type: Number, default: 10 },
  accept: { type: String, default: 'image/jpeg,image/png,image/webp,application/pdf,.docx,.xlsx' },
});

const model = defineModel({ type: Array, default: () => [] });

const files = ref([]);
const uploading = ref(false);
const fileInput = ref(null);
const notifications = useDeletionsStore();

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
  }
}

async function remove(file) {
  try {
    await deleteApplicationDraftFile(file.id);
    files.value = files.value.filter((f) => f.id !== file.id);
    syncModel();
  } catch (error) {
    notifications.notify({ bold: error.message, type: 'error' });
  }
}

function syncModel() {
  model.value = files.value.map((f) => f.id);
}

/** Очистка после успешной подачи: файлы уже привязаны к заявке. */
function reset() {
  files.value = [];
  model.value = [];
}

defineExpose({ reset });
</script>

<style scoped>
.app-files {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.app-files__head {
    display: flex;
    flex-direction: column;
    gap: 2px;
}

.app-files__label {
    font-size: 13px;
    color: var(--text-muted);
}

.app-files__hint {
    font-size: 12px;
    color: var(--text-muted);
    opacity: 0.8;
}

.app-files__controls {
    display: flex;
    align-items: center;
    gap: 10px;
}

.app-files__input {
    display: none;
}

.app-files__counter {
    font-size: 12px;
    color: var(--text-muted);
}

.app-files__list {
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.app-files__item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--surface);
}

.app-files__name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 13px;
}

.app-files__size {
    font-size: 12px;
    color: var(--text-muted);
    white-space: nowrap;
}

.app-files__remove {
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
