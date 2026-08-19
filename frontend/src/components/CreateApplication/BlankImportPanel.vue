<template>
  <section
    class="bip"
    data-testid="blank-import-panel"
  >
    <div class="bip__head">
      <h4 class="bip__title">
        Импорт из бланка
      </h4>
      <button
        type="button"
        class="bip__close"
        aria-label="Выйти из режима импорта"
        data-testid="blank-import-close"
        @click="$emit('close')"
      >
        &times;
      </button>
    </div>

    <BlankImportResult
      v-if="showSummary"
      :attachment-type="attachmentType"
      :has-result="!!result"
      :summary="result ? (result.summary || {}) : {}"
      :rows="result ? (result.rows || []) : []"
      :pending-count="pendingCount"
      :all-passage-tables="allPassageTables"
      :all-unloading-places="allUnloadingPlaces"
      :field-config="fieldConfig"
      @stage="$emit('stage', $event)"
      @import="$emit('import', $event)"
      @reset="onResetResult"
    />

    <template v-else>
      <div
        class="bip__dropzone"
        :class="{ 'bip__dropzone--active': isDragging, 'bip__dropzone--busy': uploading }"
        data-testid="import-dropzone"
        @dragenter.prevent="isDragging = true"
        @dragover.prevent
        @dragleave.prevent="isDragging = false"
        @drop.prevent="onDrop"
      >
        <AppIcon
          name="xlsx"
          :size="44"
          class="bip__dropzone-icon"
        />
        <p class="bip__dropzone-title">
          Перетащите заполненный бланк сюда
        </p>
        <p class="bip__dropzone-hint">
          Файл .xlsx по скачанному бланку, до {{ MAX_IMPORT_ROWS }} строк
        </p>
        <label
          class="lk-button lk-button--primary bip__browse"
          :class="{ 'bip__browse--busy': uploading }"
        >
          {{ uploading ? 'Загружаем...' : 'Выберите файл' }}
          <input
            type="file"
            accept=".xlsx"
            hidden
            :disabled="uploading"
            data-testid="import-file-input"
            @change="onFileChange"
          >
        </label>
      </div>

      <!-- Скачивание пустого бланка - вспомогательное действие рядом с основным
           (загрузкой), поэтому ghost-кнопка, а не вторая заметная кнопка. -->
      <button
        type="button"
        class="lk-button lk-button--ghost lk-button--sm bip__download"
        data-testid="download-blank-template-btn"
        :disabled="downloading"
        @click="$emit('download-blank')"
      >
        <AppIcon
          name="download"
          :size="14"
          class="bip__download-icon"
        />
        {{ downloading ? 'Скачиваем...' : 'Скачать пустой бланк' }}
      </button>

      <!-- Предварительные строки уже в списке: без этой двери «Загрузить другой файл»
           оставлял бы их без сводки, то есть без кнопки, которая делает их обычными. -->
      <button
        v-if="pendingCount > 0"
        type="button"
        class="lk-button lk-button--ghost lk-button--sm bip__download"
        data-testid="import-back-to-summary"
        @click="showUploader = false"
      >
        К сводке ({{ pendingCount }})
      </button>
    </template>
  </section>
</template>

<script>
import BlankImportResult from './BlankImportResult.vue';
import { useDeletionsStore } from '@/stores/deletions';
import AppIcon from '@/components/icons/AppIcon.vue';

// Потолок строк одного файла - internal/services/attachment_import_service.go
// (maxImportListRows). Держим числом рядом с текстом, чтобы подпись не разъезжалась
// с сообщением бэка "максимум 2000".
const MAX_IMPORT_ROWS = 2000;

/**
 * Режим импорта левой колонки подачи заявки (эпик blank-import-ux, срез U4): вместо
 * формы ручного ввода - область загрузки заполненного бланка, а после разбора файла на
 * её месте сводка (BlankImportResult.vue). Сеть здесь не трогается: панель отдаёт
 * выбранный файл наверх, запросы и их состояния держит CreateApplication.vue.
 */
export default {
  name: 'BlankImportPanel',
  components: { AppIcon, BlankImportResult },
  props: {
    attachmentType: {
      type: String,
      default: 'people',
      validator: (v) => ['people', 'cars'].includes(v),
    },
    // Ответ разбора {rows, summary}; null - файл ещё не загружен, показываем дропзон.
    result: { type: Object, default: null },
    uploading: { type: Boolean, default: false },
    downloading: { type: Boolean, default: false },
    allPassageTables: { type: Array, default: () => [] },
    allUnloadingPlaces: { type: Array, default: () => [] },
    fieldConfig: { type: Object, default: () => ({}) },
    // Сколько строк текущего вложения ещё предварительные (U5). Сводку держим открытой
    // и по ним одним: разбор файла перезагрузку страницы не переживает, а серые строки
    // переживают - иначе перевести их в обычные было бы нечем.
    pendingCount: { type: Number, default: 0 },
  },
  emits: ['file', 'download-blank', 'stage', 'import', 'reset', 'close'],
  data() {
    return {
      isDragging: false,
      // Человек сам попросил область загрузки при живой сводке («Загрузить другой файл»).
      showUploader: false,
      MAX_IMPORT_ROWS,
    };
  },
  computed: {
    showSummary() {
      return !this.showUploader && (!!this.result || this.pendingCount > 0);
    },
  },
  watch: {
    result(value) {
      if (value) this.showUploader = false;
    },
  },
  methods: {
    onResetResult() {
      this.showUploader = true;
      this.$emit('reset');
    },

    onFileChange(e) {
      const file = e.target.files[0];
      // Сброс значения - иначе повторный выбор ТОГО ЖЕ файла не даёт change.
      e.target.value = '';
      if (file) this.$emit('file', file);
    },
    onDrop(e) {
      this.isDragging = false;
      if (this.uploading) return;
      const file = e.dataTransfer.files[0];
      if (!file) return;
      if (!file.name.toLowerCase().endsWith('.xlsx')) {
        useDeletionsStore().notify({
          prefix: 'Нужен файл .xlsx: ',
          bold: file.name,
          suffix: ' не подходит',
          type: 'error',
        });
        return;
      }
      this.$emit('file', file);
    },
  },
};
</script>

<style scoped>
/* Панель занимает место формы ручного ввода в том же flex-ряду - геометрия зеркалит
   .data__completion (VehicleForm/EmployeeForm), иначе список рядом переедет. */
.bip {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 450px;
  max-width: 100%;
  min-width: 0;
  padding: 15px;
  border-right: 1px solid var(--border);
  animation: bip-appear 0.2s ease-out;
}

@keyframes bip-appear {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .bip {
    animation: none;
  }
}

.bip__head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.bip__title {
  margin: 0;
  flex: 1;
  min-width: 0;
}

.bip__close {
  flex: 0 0 auto;
  width: 32px;
  height: 32px;
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-muted);
  font-size: 22px;
  line-height: 1;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.bip__close:hover {
  background: var(--surface-2);
  color: var(--text);
}

/* Основная область режима: крупная, чтобы читаться как цель перетаскивания.
   Разметка и состояния - те же, что у единственного другого xlsx-дропзона проекта
   (AttachmentTemplateEditor.vue, .te-dropzone). */
.bip__dropzone {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 220px;
  padding: 24px 16px;
  text-align: center;
  border: 2px dashed var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--surface);
  transition: border-color 0.15s ease, background 0.15s ease;
}

.bip__dropzone--active {
  border-color: var(--accent);
  background: var(--accent-tint);
}

.bip__dropzone--busy {
  opacity: 0.7;
}

.bip__download-icon {
  /* 14px: общая обводка 1.7 даёт 0.99px - тоньше подписи кнопки рядом. */
  stroke-width: 2.2;
}

.bip__dropzone-icon {
  opacity: 0.8;
  /* Значок книги Excel был зелёным цветом формата - остаётся им. Берём
     --success-text, а не --success: последний в проекте живёт рамкой и фоном,
     а на поверхности карточки даёт 2.1 при норме 3.0 для графики. */
  color: var(--success-text);
}

.bip__dropzone-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text);
}

.bip__dropzone-hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}

.bip__browse {
  margin-top: 4px;
  cursor: pointer;
}

.bip__browse--busy {
  opacity: 0.5;
  cursor: progress;
}

.bip__download {
  align-self: center;
}

/* Тот же брейкпоинт, на котором form__data стекается в колонку. */
@media (max-width: 1024px) {
  .bip {
    width: 100%;
    border-right: none;
    border-bottom: 1px solid var(--border);
  }
}

@media (max-width: 768px) {
  .bip__dropzone {
    min-height: 180px;
  }

  /* Тач-таргеты по WCAG 2.5.5: --sm и label-кнопка сами по себе ниже 44px. */
  .bip__close,
  .bip__browse,
  .bip__download {
    min-height: 44px;
  }

  .bip__close {
    width: 44px;
  }
}
</style>
