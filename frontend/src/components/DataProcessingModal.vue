<template>
  <BaseModal
    :show="show"
    title="Согласие на обработку персональных данных"
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

      <div
        v-else-if="!meta"
        class="dp-modal__state"
      >
        <p class="dp-modal__title">
          Документ ещё не загружен
        </p>
        <p class="dp-modal__hint">
          Администратор пока не разместил документ о порядке обработки персональных данных.
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

const meta = ref(null);
const blob = ref(null);
const loading = ref(false);
const error = ref(null);
const downloading = ref(false);
// Грузим документ один раз на сеанс: повторное открытие модалки не дёргает бэк заново.
const loaded = ref(false);

const isPdf = computed(
  () => meta.value && (meta.value.mime_type === 'application/pdf' || meta.value.ext === '.pdf'),
);

async function load() {
  loading.value = true;
  error.value = null;
  try {
    meta.value = await getDataProcessingMeta();
    if (isPdf.value) {
      blob.value = await fetchDataProcessingBlob();
    }
    loaded.value = true;
  } catch {
    error.value = 'Не удалось загрузить документ. Попробуйте ещё раз.';
    loaded.value = false;
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
  color: var(--color-danger);
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
  color: #fff;
}

.dp-modal__btn--primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.dp-modal__btn--ghost {
  background: transparent;
  color: var(--color-primary);
  border-color: var(--color-primary);
}

@media (max-width: 768px) {
  /* На мобилке лист - bottom-sheet во всю высоту тела (BaseModal), фикс-высоту снимаем. */
  .dp-modal__body {
    height: auto;
    flex: 1;
  }
}
</style>
