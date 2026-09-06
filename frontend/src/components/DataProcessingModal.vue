<template>
  <BaseModal
    :show="show"
    title="Обработка персональных данных"
    width="720px"
    content-class="dp-modal"
    @close="$emit('close')"
  >
    <div class="dp-modal__body">
      <p
        v-if="loading"
        class="dp-modal__state"
      >
        Загрузка документа...
      </p>

      <div
        v-else-if="error"
        class="dp-modal__state dp-modal__state--error"
      >
        <p>{{ error }}</p>
        <button
          type="button"
          class="dp-modal__btn dp-modal__btn--ghost"
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
        class="dp-modal__text"
        v-html="safeHtml"
      />

      <div
        v-else-if="!meta"
        class="dp-modal__state"
      >
        <p class="dp-modal__title">
          Документ ещё не загружен
        </p>
        <p class="dp-modal__hint">
          Документ о порядке обработки персональных данных пока не размещён.
        </p>
      </div>

      <PdfDocumentViewer
        v-else-if="isPdf"
        :blob="blob"
        class="dp-modal__viewer"
      />

      <div
        v-else
        class="dp-modal__state"
      >
        <p class="dp-modal__title">
          {{ meta.file_name }}
        </p>
        <p class="dp-modal__hint">
          Этот формат нельзя открыть прямо здесь. Скачайте документ, чтобы прочитать его.
        </p>
      </div>
    </div>

    <template #actions>
      <button
        v-if="meta"
        type="button"
        class="dp-modal__btn dp-modal__btn--primary"
        :disabled="downloading"
        @click="download"
      >
        {{ downloading ? 'Скачивание...' : 'Скачать' }}
      </button>
    </template>
  </BaseModal>
</template>

<script setup>
import { computed, ref, watch } from 'vue';
import BaseModal from '@/components/ui/BaseModal.vue';
import PdfDocumentViewer from '@/components/ui/PdfDocumentViewer.vue';
import {
  getDataProcessingMeta,
  fetchDataProcessingBlob,
  downloadDataProcessingDoc,
} from '@/api/dataProcessing';
import { usePDConsentStore } from '@/stores/pdConsent';
import { sanitizeHtml, stripHtml } from '@/utils/sanitize';

/**
 * Модалка согласия на обработку ПД для мобильного показа: bottom-sheet (BaseModal)
 * с содержимым документа прямо внутри. На десктопе документ по-прежнему открывается
 * страницей /data-processing (нативный <embed>), эта модалка туда не лезет.
 */
const props = defineProps({
  show: {
    type: Boolean,
    default: false,
  },
});
defineEmits(['close']);

const consent = usePDConsentStore();
const meta = ref(null);
const blob = ref(null);
const loading = ref(false);
const error = ref(null);
const downloading = ref(false);
// Грузим документ один раз на сеанс: повторное открытие модалки не дёргает бэк заново.
const loaded = ref(false);
// Seq-токен: быстрый закрыть/открыть до резолва пускает два load() в общие meta/blob;
// пишет только последний, устаревший ответ отбрасываем (last-resolve-wins иначе).
let loadSeq = 0;

const isPdf = computed(
  () => meta.value && (meta.value.mime_type === 'application/pdf' || meta.value.ext === '.pdf'),
);
// Редактор на очищенном документе отдаёт "<p></p>": голый Boolean счёл бы это
// текстом и показал пустой лист вместо файла. Считаем по видимому тексту -
// той же меркой, что и серверный гейт (hasVisibleText).
const hasText = computed(() => stripHtml(consent.html).length > 0);
const safeHtml = computed(() => sanitizeHtml(consent.html));

async function load() {
  const seq = ++loadSeq;
  loading.value = true;
  error.value = null;
  // Сетевую ошибку стор глушит сам: текста просто не будет, и мы уйдём на файл.
  await consent.refresh();
  if (seq !== loadSeq) return;
  try {
    const nextMeta = await getDataProcessingMeta();
    if (seq !== loadSeq) return;
    meta.value = nextMeta;
    // Когда текст задан, файл не читаем вовсе - показывать будем текст, а блоб
    // весит мегабайты и грузился бы впустую.
    if (!hasText.value && isPdf.value) {
      const nextBlob = await fetchDataProcessingBlob();
      if (seq !== loadSeq) return;
      blob.value = nextBlob;
    }
    loaded.value = true;
  } catch {
    if (seq !== loadSeq) return;
    // Текст есть - окно остаётся полезным: показываем его, а не экран ошибки.
    // meta не обнуляем: если упало чтение файла, имя документа уже известно и
    // кнопка скачивания рабочая. loaded НЕ поднимаем - иначе «грузим раз на
    // сеанс» навсегда запомнил бы неудачу и следующее открытие не повторило бы.
    if (!hasText.value) {
      error.value = 'Не удалось загрузить документ. Попробуйте ещё раз.';
    }
    loaded.value = false;
  } finally {
    if (seq === loadSeq) loading.value = false;
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

watch(
  () => props.show,
  (val) => {
    if (val && !loaded.value) load();
  },
);
</script>

<style scoped>
.dp-modal__body {
  display: flex;
  flex-direction: column;
  min-height: 0;
  /* Внутри BaseModal тело окна имеет padding:0; на десктопе высоту читалки задаём тут,
     на мобилке лист сам растягивается на 90dvh (flex-column из BaseModal). */
  height: min(70vh, 640px);
}

.dp-modal__viewer {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.dp-modal__text {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 16px 20px;
  color: var(--color-text);
  font-size: 14px;
  line-height: 1.65;
  overflow-wrap: anywhere;
}

.dp-modal__text :deep(p) {
  margin: 0 0 10px;
}

.dp-modal__text :deep(h1),
.dp-modal__text :deep(h2),
.dp-modal__text :deep(h3) {
  margin: 18px 0 10px;
  line-height: 1.35;
}

.dp-modal__text :deep(ul),
.dp-modal__text :deep(ol) {
  margin: 0 0 10px;
  padding-left: 22px;
}

.dp-modal__text :deep(img) {
  max-width: 100%;
  height: auto;
}

.dp-modal__state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  text-align: center;
  padding: 40px 24px;
  color: var(--color-text-muted);
}

.dp-modal__state--error {
  color: var(--danger-text);
}

.dp-modal__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text);
}

.dp-modal__hint {
  margin: 0;
  max-width: 420px;
  font-size: 14px;
  color: var(--color-text-muted);
}

.dp-modal__btn {
  padding: 10px 24px;
  border: 1px solid transparent;
  border-radius: var(--radius-pill);
  cursor: pointer;
  font-family: 'Montserrat', sans-serif;
  font-size: 14px;
  font-weight: 600;
}

.dp-modal__btn--primary {
  background: var(--color-primary);
  color: var(--accent-contrast);
}

.dp-modal__btn--primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.dp-modal__btn--ghost {
  background: transparent;
  color: var(--accent-text);
  border-color: var(--accent);
}

@media (max-width: 768px) {
  /* На мобилке лист - bottom-sheet во всю высоту тела (BaseModal), фикс-высоту снимаем. */
  .dp-modal__body {
    height: auto;
    flex: 1;
  }
}
</style>
