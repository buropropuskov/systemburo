<template>
  <div
    ref="rootEl"
    class="pdf-viewer"
  >
    <p
      v-if="status === 'loading'"
      class="pdf-viewer__state"
    >
      Загрузка документа...
    </p>

    <div
      v-else-if="status === 'error'"
      class="pdf-viewer__state pdf-viewer__state--error"
    >
      <p>Не удалось показать документ.</p>
      <button
        type="button"
        class="pdf-viewer__retry"
        @click="render"
      >
        Повторить
      </button>
    </div>

    <!-- Контейнер держим в DOM всегда (ref для canvas), прячем через v-show до готовности.
         data-theme="light" - светлый островок: внутри лист документа, он белый в
         любой теме, а тема сюда пришла бы от корня (см. assets/tokens.css). -->
    <div
      v-show="status === 'ready'"
      ref="pagesEl"
      class="pdf-viewer__pages"
      data-theme="light"
    />
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
// ?worker (а не ?url): Vite бандлит воркер в отдельный .js-чанк и отдаёт конструктор Worker.
// Через ?url воркер оставался .mjs-ассетом, а боевой nginx отдаёт .mjs как
// application/octet-stream -> браузер отвергает module-скрипт (strict MIME) и воркер не
// поднимается. .js nginx отдаёт корректно. Основной бандл не толстеет (воркер - свой чанк),
// а тело pdf.js по-прежнему грузится лениво (динамический import ниже).
import PdfWorker from 'pdfjs-dist/build/pdf.worker.min.mjs?worker';

/**
 * Рендер PDF-документа страницами в canvas через pdf.js.
 * Нужен там, где нативный <embed type="application/pdf"> не работает (мобильные браузеры).
 * Данные PDF передаём в воркер как ArrayBuffer, поэтому воркеру не нужен сетевой доступ
 * (CSP connect-src), а сам воркер - same-origin модуль (покрыт script-src 'self').
 */
const props = defineProps({
  // Blob документа (application/pdf). null - пусто, рендерить нечего. Object - чтобы
  // принимать и настоящий Blob, и blob-подобные объекты (arrayBuffer()) в тестах.
  blob: {
    type: [Blob, Object],
    default: null,
    validator: (v) => v === null || typeof v === 'object',
  },
});
const emit = defineEmits(['loaded', 'error']);

const rootEl = ref(null);
const pagesEl = ref(null);
const status = ref('idle'); // 'idle' | 'loading' | 'ready' | 'error'

// Горизонтальный padding .pdf-viewer__pages (12px x2): ширину canvas берём от видимого
// корня минус этот отступ, т.к. сам контейнер страниц скрыт v-show и его clientWidth === 0.
const PAGES_PADDING_X = 24;

let pdfjsLib = null;
let pdfDoc = null;
// Токен последовательности: при быстрой смене blob / размонтировании старый рендер
// не должен дописывать canvas'ы и переключать статус поверх актуального.
let renderToken = 0;

async function ensurePdfjs() {
  if (pdfjsLib) return pdfjsLib;
  // Ленивая загрузка: тело pdf.js в отдельном чанке, не тянется в основной бандл.
  pdfjsLib = await import('pdfjs-dist');
  // workerPort вместо workerSrc: собранный ?worker-конструктор поднимает воркер из .js-чанка
  // (обходит .mjs-MIME боевого nginx). Один общий воркер на все документы - нам хватает.
  if (!pdfjsLib.GlobalWorkerOptions.workerPort) {
    pdfjsLib.GlobalWorkerOptions.workerPort = new PdfWorker();
  }
  return pdfjsLib;
}

function destroyDoc() {
  if (!pdfDoc) return;
  const doc = pdfDoc;
  pdfDoc = null;
  // best-effort: destroy отклоняет промис при незавершённом рендере - для очистки не важно.
  Promise.resolve(doc.destroy?.()).catch(() => {});
}

function clearPages() {
  if (pagesEl.value) pagesEl.value.replaceChildren();
}

async function render() {
  const token = ++renderToken;
  destroyDoc();
  clearPages();

  const blob = props.blob;
  if (!blob) {
    status.value = 'idle';
    return;
  }

  status.value = 'loading';
  try {
    const lib = await ensurePdfjs();
    const data = new Uint8Array(await blob.arrayBuffer());
    if (token !== renderToken) return;

    pdfDoc = await lib.getDocument({ data }).promise;
    if (token !== renderToken) {
      destroyDoc();
      return;
    }

    const host = pagesEl.value;
    if (!host) return;
    // Меряем ВИДИМЫЙ корень (pagesEl скрыт v-show до 'ready' -> его clientWidth === 0,
    // и canvas отрендерился бы по фолбэку 800px, раздувая битмап на телефоне).
    const rootWidth = rootEl.value?.clientWidth || 800;
    const cssWidth = Math.max(rootWidth - PAGES_PADDING_X, 200);
    // Кап DPR на 2: на телефонах с DPR 3 canvas раздувался бы втрое без видимой пользы.
    const dpr = Math.min(window.devicePixelRatio || 1, 2);

    for (let n = 1; n <= pdfDoc.numPages; n += 1) {
      if (token !== renderToken) return;
      const page = await pdfDoc.getPage(n);
      const base = page.getViewport({ scale: 1 });
      const viewport = page.getViewport({ scale: (cssWidth / base.width) * dpr });

      const canvas = document.createElement('canvas');
      canvas.className = 'pdf-viewer__page';
      canvas.width = Math.floor(viewport.width);
      canvas.height = Math.floor(viewport.height);
      // CSS-ширина по контейнеру: при смене ориентации canvas просто масштабируется,
      // перерендер не нужен (крупный DPR-битмап держит резкость при растяжении).
      canvas.style.width = '100%';
      canvas.style.height = 'auto';
      host.appendChild(canvas);

      const ctx = canvas.getContext('2d');
      await page.render({ canvasContext: ctx, viewport }).promise;
    }

    if (token !== renderToken) return;
    status.value = 'ready';
    emit('loaded', pdfDoc.numPages);
  } catch (e) {
    if (token !== renderToken) return;
    destroyDoc();
    clearPages();
    status.value = 'error';
    emit('error', e);
  }
}

watch(() => props.blob, render);
onMounted(render);
onBeforeUnmount(() => {
  renderToken += 1;
  destroyDoc();
});

defineExpose({ render });
</script>

<style scoped>
.pdf-viewer {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.pdf-viewer__state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  text-align: center;
  padding: 48px 24px;
  color: var(--color-text-muted);
}

.pdf-viewer__state--error {
  color: var(--danger-text);
}

.pdf-viewer__retry {
  padding: 8px 20px;
  border: 1px solid var(--color-primary);
  border-radius: var(--radius-pill);
  background: transparent;
  color: var(--accent-text);
  font-family: 'Montserrat', sans-serif;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
}

.pdf-viewer__pages {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 12px;
  /* Подложка под листом - светлая палитра островка, не литерал. */
  background: var(--surface-2);
}

.pdf-viewer__pages :deep(.pdf-viewer__page) {
  display: block;
  width: 100%;
  height: auto;
  background: var(--surface);
  box-shadow: 0 1px 6px rgba(0, 0, 0, 0.12);
  border-radius: 4px;
}
</style>
