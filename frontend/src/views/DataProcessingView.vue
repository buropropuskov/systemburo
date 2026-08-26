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

      <!-- Текст согласия важнее файла: он и есть та редакция, которую подтверждает
           пользователь при входе (#1567). Рендер только через sanitizeHtml -
           в system_settings HTML лежит сырым. -->
      <article
        v-else-if="hasText"
        class="dp-text"
        v-html="safeHtml"
      />

      <div
        v-else-if="!meta"
        class="dp-state"
      >
        <p class="dp-empty-title">
          Документ ещё не загружен
        </p>
        <p class="dp-empty-hint">
          Документ о порядке обработки персональных данных пока не размещён.
        </p>
      </div>

      <!-- На телефоне <embed> PDF не рендерит - показываем страницами через pdf.js. -->
      <PdfDocumentViewer
        v-else-if="isPdf && isMobile && pdfBlob"
        :blob="pdfBlob"
        class="dp-pdf-mobile"
      />

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
import PdfDocumentViewer from '@/components/ui/PdfDocumentViewer.vue';
import { usePDConsentStore } from '@/stores/pdConsent';
import { sanitizeHtml, stripHtml } from '@/utils/sanitize';

const consent = usePDConsentStore();
const meta = ref(null);
const loading = ref(true);
const error = ref(null);
const pdfUrl = ref(null);
const pdfBlob = ref(null);
const downloading = ref(false);
// Порог 768px: телефон рендерит PDF через pdf.js (нативный embed не работает),
// десктоп/планшет оставляет <embed>. Реактивно - вью полноэкранная, ловим поворот/ресайз.
const isMobile = ref(false);
let mql = null;

const isPdf = computed(
  () => meta.value && (meta.value.mime_type === 'application/pdf' || meta.value.ext === '.pdf'),
);
// Редактор на очищенном документе отдаёт "<p></p>": голый Boolean счёл бы это
// текстом и показал пустой лист вместо файла. Считаем по видимому тексту -
// той же меркой, что и серверный гейт (hasVisibleText).
const hasText = computed(() => stripHtml(consent.html).length > 0);
const safeHtml = computed(() => sanitizeHtml(consent.html));

function revokePdf() {
  if (pdfUrl.value) {
    URL.revokeObjectURL(pdfUrl.value);
    pdfUrl.value = null;
  }
  pdfBlob.value = null;
}

async function load() {
  loading.value = true;
  error.value = null;
  revokePdf();
  // Сетевую ошибку стор глушит сам: текста просто не будет, и мы уйдём на файл.
  await consent.refresh();
  try {
    meta.value = await getDataProcessingMeta();
    // Когда текст задан, файл не читаем вовсе - показывать будем текст, а блоб
    // весит мегабайты и грузился бы впустую.
    if (!hasText.value && isPdf.value) {
      const blob = await fetchDataProcessingBlob();
      pdfBlob.value = blob;
      pdfUrl.value = URL.createObjectURL(blob);
    }
  } catch {
    // Текст есть - страница остаётся полезной, показываем его вместо экрана ошибки.
    // meta не обнуляем: если упало чтение файла, имя документа уже известно и
    // кнопка скачивания рабочая.
    if (!hasText.value) {
      error.value = 'Не удалось загрузить документ. Попробуйте обновить страницу.';
    }
  } finally {
    loading.value = false;
  }
}

function applyMobile(e) {
  isMobile.value = e.matches;
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

onMounted(() => {
  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    mql = window.matchMedia('(max-width: 768px)');
    isMobile.value = mql.matches;
    mql.addEventListener('change', applyMobile);
  }
  load();
});
onBeforeUnmount(() => {
  if (mql) mql.removeEventListener('change', applyMobile);
  revokePdf();
});
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
  /* Под документом всегда белый лист: PDF рисует нативный просмотрщик, тема его
     не перекрашивает - тёмная плашка просвечивала бы в проёмах страницы. */
  background: #fff;
}

.dp-pdf-mobile {
  flex: 1;
  width: 100%;
  min-height: 70vh;
  overflow-y: auto;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
}

.dp-text {
  flex: 1;
  min-width: 0;
  padding: 24px 28px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--surface);
  color: var(--color-text);
  font-size: 14px;
  line-height: 1.65;
  overflow-wrap: anywhere;
}

.dp-text :deep(p) {
  margin: 0 0 10px;
}

.dp-text :deep(h1),
.dp-text :deep(h2),
.dp-text :deep(h3) {
  margin: 18px 0 10px;
  line-height: 1.35;
}

.dp-text :deep(ul),
.dp-text :deep(ol) {
  margin: 0 0 10px;
  padding-left: 22px;
}

.dp-text :deep(img) {
  max-width: 100%;
  height: auto;
}

.dp-text :deep(table) {
  width: 100%;
  border-collapse: collapse;
}

.dp-text :deep(td),
.dp-text :deep(th) {
  border: 1px solid var(--color-border);
  padding: 6px 8px;
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
  background: var(--surface);
  color: var(--color-text-muted);
}

.dp-state--error {
  color: var(--danger-text);
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
  color: var(--accent-contrast);
}

.dp-button--primary:hover:not(:disabled) {
  background: var(--color-primary-hover);
}

.dp-button--ghost {
  background: transparent;
  color: var(--accent-text);
  border-color: var(--accent);
}

.dp-button--ghost:hover:not(:disabled) {
  background: var(--color-bg);
}

.dp-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
