<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="dbm-overlay"
        data-testid="download-blanks-modal"
        @click.self="$emit('close')"
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

          <!-- Переключатель источника - общий FilterTabs, а не свои кнопки: он уже
               держит вид вкладок в восьми разделах, и вторая реализация разъедется
               с ними на первой же правке оформления. -->
          <FilterTabs
            v-if="!isLoading && !error && eligibleAttachments.length"
            v-model="source"
            class="dbm-source"
            :tabs="sourceTabs"
          />

          <!-- Выбор наполнения бланка виден только тем, кому документы участников
               положены; остальным вместо переключателя идёт строка о том, почему в
               скачанном файле прочерки. -->
          <FilterTabs
            v-if="showDocumentsChoice"
            v-model="documentsMode"
            class="dbm-source dbm-documents"
            data-testid="blank-documents-tabs"
            :tabs="documentsTabs"
          />
          <p
            v-if="!isLoading && !error && eligibleAttachments.length && !canExportDocuments"
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
          <div
            v-else
            class="dbm-list"
          >
            <label
              v-for="att in eligibleAttachments"
              :key="att.id"
              class="dbm-item"
              :class="{ selected: selectedIds.includes(att.id) }"
            >
              <input
                v-model="selectedIds"
                type="checkbox"
                :value="att.id"
                class="dbm-checkbox"
              >
              <div class="dbm-item-info">
                <span class="dbm-item-name">{{ att.attachment_display_name || att.unique_attachment_display_name || att.attachment_name }}</span>
                <span
                  v-if="attachmentTypeLabel(att.attachment_type)"
                  class="dbm-item-type"
                >{{ attachmentTypeLabel(att.attachment_type) }}</span>
              </div>
              <StatusBadge
                v-if="archiveStatusLabel(att.archive_status)"
                :status="archiveStatusLabel(att.archive_status)"
                class="dbm-archive-badge"
              />
              <button
                class="dbm-item-download"
                :disabled="downloadingId === att.id || unavailableInArchive[att.id]"
                :title="unavailableInArchive[att.id] ? 'Сохранённого файла пока нет - выберите «Сформировать заново»' : ''"
                @click.prevent="downloadOne(att)"
              >
                {{ downloadingId === att.id ? '...' : 'Скачать' }}
              </button>
            </label>
          </div>

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
import { downloadBlank, downloadApplicationArchive, saveBlobAs } from '@/api/attachment-templates';
import { usePermissionsStore } from '@/stores/permissions';
import StatusBadge from '@/components/ui/StatusBadge.vue';
import FilterTabs from '@/components/ui/FilterTabs.vue';

const TYPE_LABELS = {
  cars: 'Автомобили',
  people: 'Сотрудники',
  items: 'Имущество',
};

// Статусы строки реестра файлового архива (internal/models/blank_export.go),
// сведённые к трём бейджам модалки (#1615, C6): skipped/no_template/orphan и
// отсутствие строки не показываются вовсе - это не про ожидание, а про то, что
// архивная копия для вложения не предполагается.
const ARCHIVE_BADGE_LABELS = {
  ok: 'В архиве',
  pending: 'В очереди',
  // Остановка по нехватке места - не обычное ожидание: очередь стоит, пока
  // администратор не освободит место, и слово об этом должно отличаться.
  blocked: 'Нет места',
  failed: 'Ошибка',
};

export default {
  name: 'DownloadBlanksModal',
  components: { StatusBadge, FilterTabs },
  props: {
    show: { type: Boolean, default: false },
    applicationId: { type: Number, default: 0 },
    applicationInfo: { type: Object, default: null },
  },
  emits: ['close'],
  data() {
    return {
      attachments: [],
      selectedIds: [],
      isLoading: false,
      downloadingId: null,
      downloadingAll: false,
      error: '',
      // Источник скачивания: archive - сохранённый на диске файл файлового
      // архива, live - генерация бланка заново из текущих данных заявки.
      source: 'live',
      // Наполнение бланка: without - паспорт, патент и иное разрешение заменены
      // прочерком. Умолчание намеренно закрытое: вынос персональных данных из
      // системы должен быть отдельным решением, а не тем, что случилось само.
      documentsMode: 'without',
    };
  },
  computed: {
    eligibleAttachments() {
      return this.attachments.filter(att => att.has_template);
    },
    sourceTabs() {
      return [
        // Сохранённый файл собран с документами, и вырезать их из готового .xlsx
        // нечем - поэтому вкладка живёт только в режиме «с паспортными данными»
        // (сервер на этот случай отвечает 403, см. attachment_blank.go).
        { key: 'archive', label: 'Сохранённый файл', visible: this.withDocuments },
        { key: 'live', label: 'Сформировать заново' },
      ];
    },
    // Пара прав, а не одно: detail.documents открывает документы на экране карточки,
    // detail.documents.export - их вынос файлом. Отзыв первого гасит и второе.
    canExportDocuments() {
      const perms = usePermissionsStore();
      return perms.hasPermission('detail.documents') && perms.hasPermission('detail.documents.export');
    },
    showDocumentsChoice() {
      return !this.isLoading && !this.error && this.eligibleAttachments.length > 0 && this.canExportDocuments;
    },
    withDocuments() {
      return this.canExportDocuments && this.documentsMode === 'with';
    },
    documentsTabs() {
      return [
        { key: 'without', label: 'Без паспортных данных' },
        { key: 'with', label: 'С паспортными данными' },
      ];
    },
    // Сохранённый файл есть не у каждого вложения: у вложения в очереди, с ошибкой
    // или вовсе без строки реестра скачивать с диска нечего. Кнопку в этом случае
    // гасим - иначе выбор источника превращает редкую ошибку сервера в частый
    // отказ с невнятным текстом.
    unavailableInArchive() {
      const ids = {};
      for (const att of this.eligibleAttachments) {
        ids[att.id] = this.source === 'archive' && att.archive_status !== 'ok';
      }
      return ids;
    },
  },
  watch: {
    // Модалка всегда смонтирована (для leave-анимации): грузим вложения при
    // открытии, а не на mount (иначе fetch с пустым applicationId на старте).
    show(visible) {
      if (visible && this.applicationId) {
        this.selectedIds = [];
        this.documentsMode = 'without';
        this.load();
      }
    },
    // Режим документов управляет и источником: в закрытом вкладка «Сохранённый файл»
    // исчезает, и оставшийся в source archive молча получал бы 403. В открытом она
    // возвращается вместе с прежним умолчанием - сохранённый файл, если он есть.
    withDocuments(enabled) {
      if (!enabled) {
        this.source = 'live';
        return;
      }
      if (this.eligibleAttachments.some(a => a.archive_status === 'ok')) this.source = 'archive';
    },
  },
  methods: {
    async load() {
      this.isLoading = true;
      try {
        const res = await apiRequest(`/applications/${this.applicationId}/attachments`);
        const data = await res.json();
        this.attachments = Array.isArray(data) ? data : [];
        // Дефолт "сохранённый файл", если хоть одно вложение уже реально
        // записано в архив - иначе живая генерация (архив либо выключен,
        // либо ещё не успел выгрузить ни одного бланка этой заявки).
        const hasSaved = this.eligibleAttachments.some(a => a.archive_status === 'ok');
        this.source = hasSaved && this.withDocuments ? 'archive' : 'live';
      } catch {
        this.error = 'Не удалось загрузить вложения';
      } finally {
        this.isLoading = false;
      }
    },
    attachmentTypeLabel(t) {
      return TYPE_LABELS[t] || t || '';
    },
    archiveStatusLabel(status) {
      return ARCHIVE_BADGE_LABELS[status] || '';
    },
    async downloadOne(att) {
      this.downloadingId = att.id;
      try {
        const { blob, filename } = await downloadBlank(this.applicationId, att.id,
          { source: this.source, withDocuments: this.withDocuments });
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
            { source: this.source, withDocuments: this.withDocuments });
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
      // source=archive - серверный ZIP заявки целиком (#1615, C6): бэк уже
      // собирает его из файлов реестра, тянуть их по одному через JSZip
      // избыточно и не даёт скачать вложения без активного бланка, у которых
      // тем не менее есть сохранённый файл в архиве.
      if (this.source === 'archive') {
        this.downloadingAll = true;
        try {
          await this.downloadServerZip();
        } finally {
          this.downloadingAll = false;
        }
        return;
      }
      this.selectedIds = this.eligibleAttachments.map(a => a.id);
      await this.downloadSelected();
    },
    async downloadServerZip() {
      try {
        const { blob, filename } = await downloadApplicationArchive(this.applicationId);
        saveBlobAs(blob, filename);
        useDeletionsStore().notify({ prefix: 'Скачано: ', bold: 'архив заявки' });
      } catch (err) {
        useDeletionsStore().notify({ prefix: 'Не удалось скачать архив: ', bold: err.message || 'ошибка сервера', type: 'error' });
      }
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

.dbm-source {
  display: flex;
  gap: 6px;
  padding: 14px 24px 0;
}

/* Вторая группа вкладок идёт вплотную к первой: это две грани одного выбора
   «что скачиваем», а не отдельный блок настроек. */
.dbm-documents {
  padding-top: 8px;
}

.dbm-docs-note {
  margin: 0;
  padding: 12px 24px 0;
  font-size: 13px;
  line-height: 1.4;
  color: var(--color-text-muted);
}

.dbm-archive-badge {
  flex-shrink: 0;
}

.dbm-archive-badge :deep(.status-badge) {
  min-width: auto;
  padding: 3px 8px;
  font-size: 10px;
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
  cursor: pointer;
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

.dbm-checkbox {
  accent-color: var(--accent-text);
  width: 16px;
  height: 16px;
  flex-shrink: 0;
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
