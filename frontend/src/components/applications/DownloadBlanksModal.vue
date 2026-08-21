<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="dbm-overlay"
        data-testid="download-blanks-modal"
        @mousedown="onOverlayMousedown"
        @mouseup="onOverlayMouseup"
      >
        <div class="dbm-modal">
          <div class="dbm-header">
            <h3 class="dbm-title">
              Скачивание бланков
            </h3>
            <button
              class="dbm-close"
              @click="$emit('close')"
            >
              &times;
            </button>
          </div>

          <!-- Наполнение бланка - тумблер, а не вкладки: включено или нет, третьего
               состояния нет, и подпись читается сразу. Блок отделён от списка фоном и
               рамкой, а тумблер стоит справа - у строк списка он слева, и без этого
               настройка читалась как ещё одно вложение. Видно только тем, кому документы
               участников положены; остальным идёт строка о том, почему в файле прочерки. -->
          <div
            v-if="showDocumentsChoice"
            class="dbm-option"
          >
            <span class="dbm-option-text">
              <span class="dbm-option-title">Паспортные данные</span>
              <span class="dbm-option-hint">{{ withDocuments ? 'Попадут в бланк' : 'В бланке будет прочерк' }}</span>
            </span>
            <ToggleSwitch
              v-model="withDocuments"
              data-testid="blank-documents-toggle"
            />
          </div>
          <p
            v-if="!isLoading && !error && eligibleAttachments.length && !canChooseDocuments"
            class="dbm-docs-note"
            data-testid="blank-documents-note"
          >
            Паспортные данные, патент и иное разрешение в бланке заменены прочерком: нет права на их выгрузку.
          </p>

          <div
            v-if="isLoading"
            class="dbm-state"
          >
            <span class="dbm-spinner" />
          </div>
          <div
            v-else-if="error"
            class="dbm-state dbm-state--error"
          >
            {{ error }}
          </div>
          <div
            v-else-if="!eligibleAttachments.length"
            class="dbm-state"
          >
            У заявки нет вложений с настроенным шаблоном бланка.
          </div>
          <template v-else>
            <span class="dbm-list-title">Вложения</span>
            <div class="dbm-list">
              <div
                v-for="att in eligibleAttachments"
                :key="att.id"
                class="dbm-item"
                :class="{ selected: selectedIds.includes(att.id) }"
              >
                <ToggleSwitch
                  :model-value="selectedIds.includes(att.id)"
                  :data-testid="`blank-select-${att.id}`"
                  @update:model-value="toggleSelected(att.id, $event)"
                />
                <div class="dbm-item-info">
                  <span class="dbm-item-name">{{ att.attachment_display_name || att.unique_attachment_display_name || att.attachment_name }}</span>
                  <span
                    v-if="attachmentTypeLabel(att.attachment_type)"
                    class="dbm-item-type"
                  >{{ attachmentTypeLabel(att.attachment_type) }}</span>
                </div>
                <button
                  class="dbm-item-download"
                  :disabled="downloadingId === att.id"
                  @click.prevent="downloadOne(att)"
                >
                  {{ downloadingId === att.id ? '...' : 'Скачать' }}
                </button>
              </div>
            </div>
          </template>

          <footer
            v-if="eligibleAttachments.length"
            class="dbm-footer"
          >
            <button
              class="dbm-btn dbm-btn--ghost"
              @click="$emit('close')"
            >
              Закрыть
            </button>
            <button
              class="dbm-btn dbm-btn--ghost"
              :disabled="downloadingAll"
              @click="downloadAll"
            >
              Скачать все
            </button>
            <button
              class="dbm-btn dbm-btn--primary"
              :disabled="!selectedIds.length || downloadingAll"
              @click="downloadSelected"
            >
              {{ downloadingAll ? 'Скачивание...' : `Скачать (${selectedIds.length})` }}
            </button>
          </footer>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import JSZip from 'jszip';
import { apiRequest } from '@/api/client';
import { useDeletionsStore } from '@/stores/deletions';
import { downloadBlank, saveBlobAs } from '@/api/attachment-templates';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { usePermissionsStore } from '@/stores/permissions';
import { useAuthStore } from '@/stores/auth';
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue';

const TYPE_LABELS = {
  cars: 'Автомобили',
  people: 'Сотрудники',
  items: 'Имущество',
};

export default {
  name: 'DownloadBlanksModal',
  components: { ToggleSwitch },
  props: {
    show: { type: Boolean, default: false },
    applicationId: { type: Number, default: 0 },
    applicationInfo: { type: Object, default: null },
  },
  emits: ['close'],
  setup(props, { emit }) {
    // Закрытие по подложке - через общий composable: он отличает клик по фону от
    // протяжки, начатой внутри окна (выделение текста мышью не должно закрывать).
    return useOverlayClose(() => emit('close'));
  },
  data() {
    return {
      attachments: [],
      selectedIds: [],
      isLoading: false,
      downloadingId: null,
      downloadingAll: false,
      error: '',
      // Наполнение бланка. Выключено - паспорт, патент и иное разрешение заменены
      // прочерком. Умолчание намеренно закрытое: вынос персональных данных из
      // системы должен быть отдельным решением, а не тем, что случилось само.
      documentsRequested: false,
    };
  },
  computed: {
    eligibleAttachments() {
      return this.attachments.filter(att => att.has_template);
    },
    // Пара прав, а не одно: detail.documents открывает документы на экране карточки,
    // detail.documents.export - их вынос файлом. Отзыв первого гасит и второе.
    canExportDocuments() {
      const perms = usePermissionsStore();
      return perms.hasPermission('detail.documents') && perms.hasPermission('detail.documents.export');
    },
    // Инициатор заявки сам набирал паспорта участников в форме подачи - из своей же
    // заявки они и уходят. Тот же вывод делает сервер (canExportBlankDocuments),
    // здесь это лишь про то, показывать ли переключатель.
    isInitiator() {
      const senderID = this.applicationInfo?.sender_user_id;
      const userID = useAuthStore().userId;
      return Boolean(senderID) && Boolean(userID) && senderID === userID;
    },
    canChooseDocuments() {
      return this.canExportDocuments || this.isInitiator;
    },
    showDocumentsChoice() {
      return !this.isLoading && !this.error && this.eligibleAttachments.length > 0 && this.canChooseDocuments;
    },
    // Тумблер наполнения: право проверяется здесь же, поэтому включённое состояние
    // без права невозможно в принципе - даже если оно осталось от прошлого открытия.
    withDocuments: {
      get() {
        return this.canChooseDocuments && this.documentsRequested;
      },
      set(value) {
        this.documentsRequested = value;
      },
    },
  },
  watch: {
    // Модалка всегда смонтирована (для leave-анимации): грузим вложения при
    // открытии, а не на mount (иначе fetch с пустым applicationId на старте).
    // Закрытие модалки снимает обработчик Escape: он висит на документе, и без
    // снятия каждое открытие добавляло бы ещё один.
    show(visible) {
      if (visible && this.applicationId) {
        this.selectedIds = [];
        this.documentsRequested = false;
        this.load();
      }
      if (visible) document.addEventListener('keydown', this.onKeydown);
      else document.removeEventListener('keydown', this.onKeydown);
    },
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKeydown);
  },
  methods: {
    onKeydown(e) {
      if (e.key === 'Escape') this.$emit('close');
    },
    async load() {
      this.isLoading = true;
      try {
        const res = await apiRequest(`/applications/${this.applicationId}/attachments`);
        const data = await res.json();
        this.attachments = Array.isArray(data) ? data : [];
      } catch {
        this.error = 'Не удалось загрузить вложения';
      } finally {
        this.isLoading = false;
      }
    },
    attachmentTypeLabel(t) {
      return TYPE_LABELS[t] || t || '';
    },
    // Тумблер строки списка: выбор для «Скачать (N)». Массив, а не Set - его же
    // читает разметка, а Vue не отслеживает изменения Set.
    toggleSelected(id, on) {
      this.selectedIds = on
        ? [...this.selectedIds, id]
        : this.selectedIds.filter((selected) => selected !== id);
    },
    async downloadOne(att) {
      this.downloadingId = att.id;
      try {
        const { blob, filename } = await downloadBlank(this.applicationId, att.id,
          { withDocuments: this.withDocuments });
        saveBlobAs(blob, filename);
      } catch (err) {
        useDeletionsStore().notify({ prefix: 'Не удалось скачать: ', bold: err.message || 'ошибка сервера', type: 'error' });
      } finally {
        this.downloadingId = null;
      }
    },
    async downloadSelected() {
      if (!this.selectedIds.length) return;
      this.downloadingAll = true;
      try {
        if (this.selectedIds.length === 1) {
          await this.downloadOne({ id: this.selectedIds[0] });
        } else {
          await this.downloadAsZip(this.selectedIds);
        }
      } finally {
        this.downloadingAll = false;
      }
    },
    async downloadAsZip(ids) {
      const zip = new JSZip();
      for (const id of ids) {
        try {
          const { blob, filename } = await downloadBlank(this.applicationId, id,
            { withDocuments: this.withDocuments });
          zip.file(filename, blob);
        } catch (err) {
          useDeletionsStore().notify({ prefix: 'Не удалось скачать файл: ', bold: err.message || 'ошибка сервера', type: 'error' });
        }
      }
      const content = await zip.generateAsync({ type: 'blob' });
      const info = this.applicationInfo;
      const num = info?.application_number || this.applicationId;
      const date = info?.sending_datetime ? new Date(info.sending_datetime).toLocaleDateString('ru-RU') : '';
      const org = info?.organization_name || '';
      const parts = [num, date, org].filter(Boolean).join('_').replace(/[/\\:*?"<>|]/g, '_');
      saveBlobAs(content, `${parts}.zip`);
      useDeletionsStore().notify({ prefix: 'Скачано: ', bold: `${ids.length} файлов в ZIP` });
    },
    async downloadAll() {
      this.selectedIds = this.eligibleAttachments.map(a => a.id);
      await this.downloadSelected();
    },
  },
};
</script>

<style scoped>
.dbm-overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  /* Открывается по кнопке "Скачать" ИЗ детали заявки (overlay 10002), поэтому должна
     быть выше неё - иначе список бланков уходит ЗА деталь. В один ряд с историей
     (12000), которая тоже вызывается из детали (#1097 R3-2, лестница z-index детали). */
  z-index: 12000;
}

.dbm-modal {
  background: var(--surface);
  border-radius: 30px;
  padding: 0;
  width: 480px;
  max-width: 92vw;
  max-height: calc(var(--app-vh, 1vh) * 80);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 8px 32px var(--shadow-drop);
}

.dbm-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px 0;
}

.dbm-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text);
}

.dbm-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  background: var(--color-bg-secondary);
  border-radius: 50%;
  font-size: 18px;
  color: var(--color-text-muted);
  cursor: pointer;
  transition: all 0.15s;
  line-height: 1;
}

.dbm-close:hover {
  background: var(--color-border);
  color: var(--color-text);
}

/* Блок настройки бланка. Отделён от перечня вложений собственным фоном и рамкой, а
   тумблер стоит справа - в строках списка он слева. Без этого настройка читалась как
   ещё одно вложение: одинаковый тумблер, одинаковая строка. */
.dbm-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 16px 24px 0;
  padding: 12px 16px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-bg-secondary);
}

.dbm-option-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.dbm-option-title {
  font-size: 14px;
  color: var(--color-text);
}

.dbm-option-hint {
  font-size: 12px;
  color: var(--color-text-muted);
}

/* Подпись перечня: второй маркер границы между настройкой и списком. */
.dbm-list-title {
  display: block;
  padding: 18px 24px 0;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-text-muted);
}

.dbm-docs-note {
  margin: 0;
  padding: 12px 24px 0;
  font-size: 13px;
  line-height: 1.4;
  color: var(--color-text-muted);
}

.dbm-state {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 24px;
  color: var(--color-text-muted);
  font-size: 13px;
}

.dbm-state--error {
  color: var(--danger-text);
}

.dbm-spinner {
  display: block;
  width: 28px;
  height: 28px;
  border: 3px solid var(--color-border);
  border-top-color: var(--accent-text);
  border-radius: 50%;
  animation: dbm-spin 0.7s linear infinite;
}

@keyframes dbm-spin {
  to { transform: rotate(360deg); }
}

.dbm-list {
  padding: 16px 24px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.dbm-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  transition: all 0.15s;
  background: var(--surface);
}

.dbm-item:hover {
  border-color: var(--accent);
  background: var(--accent-tint);
}

.dbm-item.selected {
  border-color: var(--accent);
  background: var(--accent-tint);
}

.dbm-item-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.dbm-item-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dbm-item-type {
  font-size: 11px;
  color: var(--color-text-muted);
}

.dbm-item-download {
  flex-shrink: 0;
  background: none;
  border: none;
  color: var(--accent-text);
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  padding: 4px 8px;
  border-radius: var(--radius-pill);
  transition: all 0.15s;
}

.dbm-item-download:hover {
  background: var(--accent-tint);
}

.dbm-item-download:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.dbm-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 0 24px 20px;
}

.dbm-btn {
  padding: 8px 18px;
  border-radius: var(--radius-pill);
  border: none;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}

.dbm-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.dbm-btn--primary {
  background: var(--color-primary);
  color: var(--accent-contrast);
}

.dbm-btn--primary:hover:not(:disabled) {
  filter: brightness(0.92);
}

.dbm-btn--ghost {
  background: transparent;
  border: 1px solid var(--color-border);
  color: var(--color-text);
}

.dbm-btn--ghost:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--accent-text);
}

.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.25s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}
</style>
