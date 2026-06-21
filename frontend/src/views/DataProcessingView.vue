<template>
  <section class="data-processing">
    <header class="dp-header">
      <h1 class="dp-title">
        Согласие на обработку персональных данных
      </h1>
      <button
        v-if="meta"
        class="dp-button dp-button--primary"
        :disabled="downloading"
        @click="download"
      >
        {{ downloading ? 'Скачивание...' : 'Скачать' }}
      </button>
    </header>

    <div class="dp-body">
      <p
        v-if="loading"
        class="dp-state"
      >
        Загрузка документа...
      </p>

      <div
        v-else-if="error"
        class="dp-state dp-state--error"
      >
        <p>{{ error }}</p>
        <button
          class="dp-button dp-button--ghost"
          @click="load"
        >
          Повторить
        </button>
      </div>

      <div
        v-else-if="!meta"
        class="dp-state"
      >
        <p class="dp-empty-title">
          Документ ещё не загружен
        </p>
        <p class="dp-empty-hint">
          Администратор пока не разместил документ о порядке обработки персональных данных.
        </p>
      </div>

      <embed
        v-else-if="isPdf && pdfUrl"
        :src="pdfUrl"
        type="application/pdf"
        class="dp-pdf"
      >

      <div
        v-else
        class="dp-state"
      >
        <p class="dp-empty-title">
          {{ meta.file_name }}
        </p>
        <p class="dp-empty-hint">
          Этот формат нельзя открыть прямо на странице. Скачайте документ, чтобы прочитать его.
        </p>
        <button
          class="dp-button dp-button--primary"
          :disabled="downloading"
          @click="download"
        >
          {{ downloading ? 'Скачивание...' : 'Скачать документ' }}
        </button>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import {
  getDataProcessingMeta,
  fetchDataProcessingBlob,
  downloadDataProcessingDoc,
} from '@/api/dataProcessing';

const meta = ref(null);
const loading = ref(true);
const error = ref(null);
const pdfUrl = ref(null);
const downloading = ref(false);

const isPdf = computed(
  () => meta.value && (meta.value.mime_type === 'application/pdf' || meta.value.ext === '.pdf'),
);

function revokePdf() {
  if (pdfUrl.value) {
    URL.revokeObjectURL(pdfUrl.value);
    pdfUrl.value = null;
  }
}

async function load() {
  loading.value = true;
  error.value = null;
  revokePdf();
  try {
    meta.value = await getDataProcessingMeta();
    if (isPdf.value) {
      const blob = await fetchDataProcessingBlob();
      pdfUrl.value = URL.createObjectURL(blob);
    }
  } catch {
    error.value = 'Не удалось загрузить документ. Попробуйте обновить страницу.';
  } finally {
    loading.value = false;
  }
}

async function download() {
  if (!meta.value) return;
  downloading.value = true;
  try {
    await downloadDataProcessingDoc(meta.value.file_name);
  } catch {
    error.value = 'Не удалось скачать документ.';
  } finally {
    downloading.value = false;
  }
}

onMounted(load);
onBeforeUnmount(revokePdf);
</script>

<style scoped>
.data-processing {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 24px;
  min-height: calc(100vh - 48px);
  font-family: 'Montserrat', sans-serif;
}

.dp-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.dp-title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  color: var(--color-text);
}

.dp-body {
  flex: 1;
  display: flex;
  min-height: 0;
}

.dp-pdf {
  width: 100%;
  height: 100%;
  min-height: 70vh;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: #fff;
}

.dp-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  text-align: center;
  padding: 48px 24px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: #fff;
  color: var(--color-text-muted);
}

.dp-state--error {
  color: var(--color-danger);
}

.dp-empty-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text);
}

.dp-empty-hint {
  margin: 0;
  max-width: 460px;
  font-size: 14px;
  color: var(--color-text-muted);
}

.dp-button {
  padding: 10px 24px;
  border: 1px solid transparent;
  border-radius: var(--radius-pill);
  cursor: pointer;
  font-family: 'Montserrat', sans-serif;
  font-size: 14px;
  font-weight: 600;
  transition: background 0.2s ease, color 0.2s ease, border-color 0.2s ease;
}

.dp-button--primary {
  background: var(--color-primary);
  color: #fff;
}

.dp-button--primary:hover:not(:disabled) {
  background: var(--color-primary-hover);
}

.dp-button--ghost {
  background: transparent;
  color: var(--color-primary);
  border-color: var(--color-primary);
}

.dp-button--ghost:hover:not(:disabled) {
  background: var(--color-bg);
}

.dp-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
