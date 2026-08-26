<template>
  <div class="documents-container dashboard-card">
    <!-- Шапка 50px -->
    <div class="management-header rt-header-inline">
      <h3 class="management-title">Документы</h3>
      <div class="header-controls">
        <BaseDropdown
          class="group-filter-dropdown"
          :model-value="filterGroupId"
          :options="groupFilterOptions"
          label-key="label"
          value-key="value"
          @update:model-value="onGroupFilterChange"
        />
        <button
          class="lk-button lk-button--ghost"
          @click="openGroupsModal"
        >
          Управление группами
        </button>
        <button
          class="lk-button lk-button--primary rt-btn-compact"
          aria-label="Загрузить документы"
          @click="openUploadModal"
        >
          <span
            class="rt-btn-icon"
            aria-hidden="true"
          >+</span>
          <span class="rt-btn-label">Загрузить документы</span>
        </button>
        <RefreshButton
          :loading="isLoading"
          @refresh="refresh"
        />
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть: список документов -->
      <div
        class="table-section"
        :class="{ 'with-details': selectedDoc }"
      >
        <div class="table-container">
          <div class="table-body">
            <!-- Загрузка -->
            <div
              v-if="isLoading && !documents.length"
              class="docs-loading"
            >
              <LoaderSpinner label="Загрузка документов..." />
            </div>

            <!-- Список с drag -->
            <div
              v-for="(doc, idx) in filteredDocuments"
              :key="doc.id"
              class="table-row docs-row"
              :class="{ selected: selectedDoc && selectedDoc.id === doc.id }"
              draggable="true"
              @dragstart="onDragStart(idx)"
              @dragover.prevent="onDragOver(idx)"
              @dragend="onDragEnd"
              @click="selectDoc(doc)"
            >
              <span class="docs-drag-handle" title="Перетащить">&#8942;&#8942;</span>
              <FileTypeIcon
                :ext="doc.file_ext || 'file'"
                :size="28"
              />
              <div class="docs-row-main">
                <div class="docs-row-name">
                  {{ doc.title }}
                  <span
                    v-if="!doc.is_visible"
                    class="docs-badge docs-badge--hidden"
                  >скрыт</span>
                  <span
                    v-if="doc.group_name"
                    class="docs-badge docs-badge--group"
                  >{{ doc.group_name }}</span>
                </div>
                <div class="docs-row-meta">
                  {{ doc.group_name || 'Прочее' }} &middot; {{ (doc.file_ext || '').toUpperCase() }}
                </div>
              </div>
            </div>

            <div
              v-if="!isLoading && !filteredDocuments.length"
              class="no-results"
            >
              Нет документов
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">Всего: {{ filteredDocuments.length }}</span>
          </div>
        </div>
      </div>

      <!-- Правая часть: панель деталей -->
      <div
        v-if="selectedDoc"
        class="details-section"
      >
        <div class="doc-detail-preview">
          <FileTypeIcon
            :ext="selectedDoc.file_ext || 'file'"
            :size="48"
          />
          <div>
            <div class="doc-detail-filename">{{ selectedDoc.file_name }}</div>
            <div class="doc-detail-meta">
              {{ formatBytes(selectedDoc.file_size) }} &middot;
              загружен {{ formatDate(selectedDoc.created_at) }}
            </div>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Наименование (видно на сайте)</label>
          <input
            v-model="editForm.title"
            type="text"
            maxlength="255"
            class="lk-input"
          >
        </div>

        <div class="form-group">
          <label class="form-label">Описание (серый текст)</label>
          <textarea
            v-model="editForm.description"
            rows="3"
            class="lk-input lk-textarea"
          />
        </div>

        <div class="form-group">
          <label class="form-label">Группа</label>
          <select
            v-model="editForm.group_id"
            class="lk-select"
          >
            <option :value="null">— без группы (Прочее) —</option>
            <option
              v-for="g in groups"
              :key="g.id"
              :value="g.id"
            >
              {{ g.name }}
            </option>
          </select>
        </div>

        <div class="form-group">
          <label class="form-label">Дата публикации</label>
          <input
            v-model="editForm.published_at"
            type="date"
            class="lk-input"
            style="max-width: 200px"
          >
        </div>

        <div class="switch-row">
          <div>
            <div class="switch-label">Показывать на «Обзор и новости»</div>
            <div class="switch-desc">Скрытый документ остаётся в админке, но не виден пользователям</div>
          </div>
          <button
            class="toggle-switch"
            :class="{ 'toggle-switch--on': editForm.is_visible }"
            :aria-pressed="editForm.is_visible"
            @click="editForm.is_visible = !editForm.is_visible"
          />
        </div>

        <div
          v-if="editError"
          class="form-error"
        >
          {{ editError }}
        </div>

        <div class="detail-actions">
          <button
            class="lk-button lk-button--ghost"
            @click="downloadSelected"
          >
            Скачать
          </button>
          <label class="lk-button lk-button--ghost" style="cursor: pointer;">
            Заменить файл
            <input
              ref="replaceFileInput"
              type="file"
              accept=".doc,.docx,.pdf,.xlsx,.pptx"
              style="display: none"
              @change="onReplaceFile"
            >
          </label>
          <button
            class="lk-button lk-button--primary"
            :disabled="isSaving"
            @click="saveDoc"
          >
            Сохранить
          </button>
          <button
            class="lk-button lk-button--danger"
            :disabled="isDeleting"
            @click="confirmDelete"
          >
            Удалить
          </button>
        </div>
      </div>

      <div
        v-else
        class="no-selection-message"
      >
        <p>Выберите документ для просмотра и редактирования</p>
      </div>
    </div>

    <!-- Модалка: Загрузить документы -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showUploadModal"
          class="modal-overlay"
          @mousedown="onUploadOverlayMousedown"
          @mouseup="onUploadOverlayMouseup"
        >
          <div
            class="docs-modal"
            @mousedown.stop
          >
            <div class="modal-header">
              <h3>Загрузить документы</h3>
              <button
                class="modal-close"
                aria-label="Закрыть"
                @click="closeUploadModal"
              >
                ×
              </button>
            </div>
            <div class="modal-body">
              <!-- Dropzone -->
              <div
                class="dropzone"
                :class="{ 'dropzone--over': isDragOver }"
                @click="$refs.uploadInput.click()"
                @dragover.prevent="isDragOver = true"
                @dragleave="isDragOver = false"
                @drop.prevent="onDropFiles"
              >
                <div class="dropzone-text">
                  Перетащите файлы сюда или <strong>выберите на устройстве</strong>
                </div>
                <div class="dropzone-hint">
                  doc, docx, pdf, xlsx, pptx &middot; до {{ maxFileSizeMb }} МБ &middot; можно несколько
                </div>
                <input
                  ref="uploadInput"
                  type="file"
                  multiple
                  accept=".doc,.docx,.pdf,.xlsx,.pptx"
                  style="display: none"
                  @change="onSelectFiles"
                >
              </div>

              <!-- Общая группа / по своим -->
              <div
                v-if="uploadQueue.length"
                class="upload-queue-header"
              >
                <label class="form-label" style="margin: 0">
                  Очередь загрузки ({{ uploadQueue.length }}) — порядок задаёт позицию в группе
                </label>
                <select
                  v-model="uploadCommonGroupId"
                  class="lk-select"
                  style="max-width: 220px"
                >
                  <option value="__each__">у каждого своя группа</option>
                  <option :value="null">— без группы (Прочее) —</option>
                  <option
                    v-for="g in groups"
                    :key="g.id"
                    :value="g.id"
                  >
                    {{ g.name }}
                  </option>
                </select>
              </div>

              <!-- Элементы очереди -->
              <div
                v-for="(item, idx) in uploadQueue"
                :key="item._key"
                class="uq-row"
                draggable="true"
                @dragstart="onQueueDragStart(idx)"
                @dragover.prevent="onQueueDragOver(idx)"
                @dragend="onQueueDragEnd"
              >
                <span class="uq-drag">&#8942;&#8942;</span>
                <span class="uq-ord">{{ idx + 1 }}</span>
                <FileTypeIcon
                  :ext="item.ext"
                  :size="28"
                />
                <div class="uq-fields">
                  <div class="uq-filename">
                    {{ item.file.name }}
                    <span class="uq-size">&middot; {{ formatBytes(item.file.size) }}</span>
                  </div>
                  <input
                    v-model="item.title"
                    type="text"
                    class="lk-input"
                    placeholder="Наименование на сайте"
                  >
                  <input
                    v-model="item.description"
                    type="text"
                    class="lk-input"
                    placeholder="Описание (необязательно)"
                  >
                  <select
                    v-if="uploadCommonGroupId === '__each__'"
                    v-model="item.group_id"
                    class="lk-select"
                  >
                    <option :value="null">— без группы (Прочее) —</option>
                    <option
                      v-for="g in groups"
                      :key="g.id"
                      :value="g.id"
                    >
                      {{ g.name }}
                    </option>
                  </select>
                </div>
                <button
                  class="uq-remove"
                  title="Убрать из очереди"
                  @click="removeFromQueue(idx)"
                >
                  ×
                </button>
              </div>

              <div
                v-if="uploadError"
                class="form-error"
              >
                {{ uploadError }}
              </div>
            </div>
            <div class="modal-footer">
              <button
                class="lk-button lk-button--ghost"
                @click="closeUploadModal"
              >
                Отмена
              </button>
              <button
                class="lk-button lk-button--primary"
                :disabled="!uploadQueue.length || isUploading"
                @click="submitUpload"
              >
                {{ isUploading ? 'Загрузка...' : `Загрузить ${uploadQueue.length > 0 ? uploadQueue.length + ' ' + fileWord(uploadQueue.length) : ''}` }}
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <!-- Модалка: Управление группами -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showGroupsModal"
          class="modal-overlay"
          @mousedown="onGroupsOverlayMousedown"
          @mouseup="onGroupsOverlayMouseup"
        >
          <div
            class="docs-modal"
            @mousedown.stop
          >
            <div class="modal-header">
              <h3>Группы документов</h3>
              <button
                class="modal-close"
                aria-label="Закрыть"
                @click="closeGroupsModal"
              >
                ×
              </button>
            </div>
            <div class="modal-body">
              <div
                v-for="(grp, idx) in editableGroups"
                :key="grp.id"
                class="grp-row"
                draggable="true"
                @dragstart="onGroupDragStart(idx)"
                @dragover.prevent="onGroupDragOver(idx)"
                @dragend="onGroupDragEnd"
              >
                <span class="grp-drag">&#8942;&#8942;</span>
                <div class="grp-row-info">
                  <div
                    v-if="grp._editMode"
                    class="grp-edit-row"
                  >
                    <input
                      v-model="grp._editName"
                      type="text"
                      maxlength="255"
                      class="lk-input"
                      @keyup.enter="saveGroupRename(grp)"
                    >
                    <button
                      class="lk-button lk-button--primary"
                      style="padding: 0 12px; height: 34px"
                      @click="saveGroupRename(grp)"
                    >
                      OK
                    </button>
                    <button
                      class="lk-button lk-button--ghost"
                      style="padding: 0 12px; height: 34px"
                      @click="grp._editMode = false"
                    >
                      Отмена
                    </button>
                  </div>
                  <template v-else>
                    <div class="grp-name">{{ grp.name }}</div>
                    <div class="grp-cnt">{{ grp.count ?? 0 }} документов</div>
                  </template>
                </div>
                <div
                  v-if="!grp._editMode"
                  class="grp-actions"
                >
                  <button
                    class="lk-button lk-button--ghost"
                    style="padding: 0 10px; height: 28px; font-size: 12px"
                    @click="startGroupRename(grp)"
                  >
                    Переименовать
                  </button>
                  <button
                    class="lk-button lk-button--danger"
                    style="padding: 0 10px; height: 28px; font-size: 12px"
                    @click="deleteGroup(grp)"
                  >
                    Удалить
                  </button>
                </div>
              </div>

              <!-- Виртуальная группа «Прочее» — не редактируется -->
              <div class="grp-row grp-row--misc">
                <div class="grp-row-info">
                  <div class="grp-name" style="color: #8a8a9a">Прочее <em style="font-style: normal; font-size: 11px">(виртуальная)</em></div>
                  <div class="grp-cnt">документы без группы</div>
                </div>
              </div>

              <div class="form-group" style="margin-top: 16px">
                <label class="form-label">Новая группа</label>
                <div class="new-group-row">
                  <input
                    v-model="newGroupName"
                    type="text"
                    maxlength="255"
                    class="lk-input"
                    placeholder="Название группы"
                    @keyup.enter="createGroup"
                  >
                  <button
                    class="lk-button lk-button--primary"
                    :disabled="!newGroupName.trim() || isCreatingGroup"
                    @click="createGroup"
                  >
                    Создать
                  </button>
                </div>
              </div>
            </div>
            <div class="modal-footer">
              <button
                class="lk-button lk-button--ghost"
                @click="closeGroupsModal"
              >
                Закрыть
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <!-- Confirm-диалог удаления -->
    <ConfirmationModal
      :show="!!deleteConfirmDoc"
      title="Удаление документа"
      :message="deleteConfirmDoc ? `Удалить документ «${deleteConfirmDoc.title}»? Файл будет удалён с сервера безвозвратно.` : ''"
      confirm-text="Удалить"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
      @confirm="performDelete"
      @cancel="deleteConfirmDoc = null"
    />
  </div>
</template>

<script>
import RefreshButton from './RefreshButton.vue';
import ConfirmationModal from './ConfirmationModal.vue';
import BaseDropdown from './ui/BaseDropdown.vue';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import FileTypeIcon from './ui/FileTypeIcon.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { formatBytes } from '@/utils/download';
import {
  listDocumentGroups,
  createDocumentGroup,
  renameDocumentGroup,
  deleteDocumentGroup,
  reorderDocumentGroups,
  listDocuments,
  uploadDocument,
  updateDocument,
  replaceDocumentFile,
  deleteDocument,
  reorderDocuments,
  downloadDocument,
} from '@/api/documents';

function extFromName(name) {
  const m = name.match(/\.([^.]+)$/);
  return m ? m[1].toLowerCase() : 'file';
}

function makeKey() {
  return Math.random().toString(36).slice(2);
}

export default {
  name: 'DocumentsManagement',
  components: { RefreshButton, ConfirmationModal, BaseDropdown, LoaderSpinner, FileTypeIcon },
  setup() {
    // Три оверлея — три пары хендлеров
    const uploadOverlay = { close: () => {} };
    const { onOverlayMousedown: onUploadOverlayMousedown, onOverlayMouseup: onUploadOverlayMouseup } =
      useOverlayClose(() => uploadOverlay.close());

    const groupsOverlay = { close: () => {} };
    const { onOverlayMousedown: onGroupsOverlayMousedown, onOverlayMouseup: onGroupsOverlayMouseup } =
      useOverlayClose(() => groupsOverlay.close());

    return {
      onUploadOverlayMousedown,
      onUploadOverlayMouseup,
      uploadOverlay,
      onGroupsOverlayMousedown,
      onGroupsOverlayMouseup,
      groupsOverlay,
    };
  },
  data() {
    return {
      documents: [],
      groups: [],
      isLoading: false,
      isSaving: false,
      isDeleting: false,
      isUploading: false,
      isCreatingGroup: false,

      filterGroupId: '__all__',

      selectedDoc: null,
      editForm: {
        title: '',
        description: '',
        group_id: null,
        published_at: '',
        is_visible: true,
      },
      editError: null,

      showUploadModal: false,
      isDragOver: false,
      uploadQueue: [],
      uploadCommonGroupId: '__each__',
      uploadError: null,
      maxFileSizeMb: 10,

      showGroupsModal: false,
      editableGroups: [],
      newGroupName: '',

      deleteConfirmDoc: null,

      // drag state для списка документов
      dragDocIdx: null,
      // drag state для очереди
      dragQueueIdx: null,
      // drag state для групп
      dragGroupIdx: null,
    };
  },
  computed: {
    groupFilterOptions() {
      const opts = [{ label: 'Все группы', value: '__all__' }];
      this.groups.forEach((g) => opts.push({ label: g.name, value: g.id }));
      opts.push({ label: 'Прочее', value: '__misc__' });
      return opts;
    },
    filteredDocuments() {
      if (this.filterGroupId === '__all__') return this.documents;
      if (this.filterGroupId === '__misc__') return this.documents.filter((d) => !d.group_id);
      return this.documents.filter((d) => d.group_id === this.filterGroupId);
    },
  },
  created() {
    this.uploadOverlay.close = this.closeUploadModal;
    this.groupsOverlay.close = this.closeGroupsModal;
    this.loadAll();
  },
  methods: {
    async loadAll() {
      this.isLoading = true;
      try {
        const [docs, groups] = await Promise.all([
          listDocuments({ includeHidden: true }),
          listDocumentGroups(),
        ]);
        // Бэкенд может вернуть массив или {documents:[...]}  — страхуемся
        this.documents = Array.isArray(docs) ? docs : (docs.documents || []);
        this.groups = Array.isArray(groups) ? groups : (groups.groups || []);
        // Подмешиваем group_name в документы для отображения
        const groupMap = {};
        this.groups.forEach((g) => { groupMap[g.id] = g.name; });
        this.documents.forEach((d) => { d.group_name = d.group_id ? groupMap[d.group_id] : null; });
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Ошибка загрузки документов: ', bold: e?.message || 'сбой', type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },
    async refresh() {
      await this.loadAll();
    },
    selectDoc(doc) {
      this.selectedDoc = doc;
      this.editError = null;
      this.editForm = {
        title: doc.title || '',
        description: doc.description || '',
        group_id: doc.group_id ?? null,
        published_at: doc.published_at ? doc.published_at.slice(0, 10) : '',
        is_visible: !!doc.is_visible,
      };
    },
    onGroupFilterChange(val) {
      this.filterGroupId = val;
      this.selectedDoc = null;
    },

    // --- Сохранение деталей ---
    async saveDoc() {
      if (!this.selectedDoc) return;
      this.isSaving = true;
      this.editError = null;
      try {
        const payload = {
          title: this.editForm.title.trim(),
          description: this.editForm.description.trim() || null,
          group_id: this.editForm.group_id ?? null,
          published_at: this.editForm.published_at || null,
          is_visible: this.editForm.is_visible,
        };
        const result = await updateDocument(this.selectedDoc.id, payload);
        if (result?.message) {
          this.editError = result.message;
          return;
        }
        const title = this.editForm.title;
        // Обновляем в локальном массиве
        const idx = this.documents.findIndex((d) => d.id === this.selectedDoc.id);
        if (idx !== -1) {
          const groupMap = {};
          this.groups.forEach((g) => { groupMap[g.id] = g.name; });
          const merged = { ...this.documents[idx], ...payload, group_name: payload.group_id ? groupMap[payload.group_id] : null };
          this.documents.splice(idx, 1, merged);
          this.selectedDoc = merged;
        }
        useDeletionsStore().notify({ prefix: 'Документ ', bold: title, suffix: ' сохранён' });
      } catch (e) {
        this.editError = e?.message || 'Ошибка сохранения';
      } finally {
        this.isSaving = false;
      }
    },

    // --- Скачивание ---
    async downloadSelected() {
      if (!this.selectedDoc) return;
      try {
        await downloadDocument(this.selectedDoc.id, this.selectedDoc.file_name);
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Ошибка скачивания: ', bold: e?.message || 'сбой', type: 'error' });
      }
    },

    // --- Замена файла ---
    async onReplaceFile(e) {
      const file = e.target.files[0];
      if (!file || !this.selectedDoc) return;
      try {
        const result = await replaceDocumentFile(this.selectedDoc.id, file);
        const idx = this.documents.findIndex((d) => d.id === this.selectedDoc.id);
        if (idx !== -1) {
          const merged = { ...this.documents[idx], ...result };
          this.documents.splice(idx, 1, merged);
          this.selectedDoc = merged;
        }
        useDeletionsStore().notify({ prefix: 'Файл документа ', bold: this.selectedDoc.title, suffix: ' заменён' });
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Ошибка замены файла: ', bold: e?.message || 'сбой', type: 'error' });
      }
      e.target.value = '';
    },

    // --- Удаление ---
    confirmDelete() {
      this.deleteConfirmDoc = this.selectedDoc;
    },
    async performDelete() {
      const doc = this.deleteConfirmDoc;
      this.deleteConfirmDoc = null;
      this.isDeleting = true;
      try {
        await deleteDocument(doc.id);
        const idx = this.documents.findIndex((d) => d.id === doc.id);
        if (idx !== -1) this.documents.splice(idx, 1);
        if (this.selectedDoc && this.selectedDoc.id === doc.id) this.selectedDoc = null;
        useDeletionsStore().notify({ prefix: 'Документ ', bold: doc.title, suffix: ' удалён' });
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Ошибка удаления: ', bold: e?.message || 'сбой', type: 'error' });
      } finally {
        this.isDeleting = false;
      }
    },

    // --- Drag-сортировка документов ---
    onDragStart(idx) {
      this.dragDocIdx = idx;
    },
    onDragOver(idx) {
      if (this.dragDocIdx === null || this.dragDocIdx === idx) return;
      const visible = this.filteredDocuments;
      const movedDoc = visible[this.dragDocIdx];
      const targetDoc = visible[idx];
      const from = this.documents.indexOf(movedDoc);
      if (from === -1) return;
      this.documents.splice(from, 1);
      const to = this.documents.indexOf(targetDoc);
      this.documents.splice(to === -1 ? this.documents.length : to, 0, movedDoc);
      this.dragDocIdx = idx;
    },
    async onDragEnd() {
      this.dragDocIdx = null;
      // Сохраняем порядок на бэкенд
      const groupId = this.filterGroupId === '__all__' || this.filterGroupId === '__misc__'
        ? null
        : this.filterGroupId;
      const ids = this.filteredDocuments.map((d) => d.id);
      try {
        await reorderDocuments(groupId, ids);
      } catch {
        // порядок документов не критичен для пользователя — ошибку не показываем
      }
    },

    // --- Модалка загрузки ---
    openUploadModal() {
      this.uploadQueue = [];
      this.uploadCommonGroupId = '__each__';
      this.uploadError = null;
      this.showUploadModal = true;
    },
    closeUploadModal() {
      if (this.isUploading) return;
      this.showUploadModal = false;
    },
    onDropFiles(e) {
      this.isDragOver = false;
      const files = Array.from(e.dataTransfer.files);
      this.addFilesToQueue(files);
    },
    onSelectFiles(e) {
      const files = Array.from(e.target.files);
      this.addFilesToQueue(files);
      e.target.value = '';
    },
    addFilesToQueue(files) {
      const ALLOWED = ['doc', 'docx', 'pdf', 'xlsx', 'pptx'];
      files.forEach((f) => {
        const ext = extFromName(f.name);
        if (!ALLOWED.includes(ext)) return;
        this.uploadQueue.push({
          _key: makeKey(),
          file: f,
          ext,
          title: f.name.replace(/\.[^.]+$/, '').replace(/_/g, ' '),
          description: '',
          group_id: null,
        });
      });
    },
    removeFromQueue(idx) {
      this.uploadQueue.splice(idx, 1);
    },
    async submitUpload() {
      if (!this.uploadQueue.length) return;
      this.isUploading = true;
      this.uploadError = null;
      let uploaded = 0;
      try {
        for (let i = 0; i < this.uploadQueue.length; i++) {
          const item = this.uploadQueue[i];
          const gid = this.uploadCommonGroupId === '__each__'
            ? item.group_id
            : this.uploadCommonGroupId === null ? null : this.uploadCommonGroupId;
          await uploadDocument(item.file, {
            title: item.title || item.file.name,
            description: item.description || null,
            group_id: gid,
            sort_order: i,
          });
          uploaded++;
        }
        useDeletionsStore().notify({
          prefix: `Загружено документов: `,
          bold: String(uploaded),
        });
        this.showUploadModal = false;
        await this.loadAll();
      } catch (e) {
        this.uploadError = e?.message || 'Ошибка при загрузке';
      } finally {
        this.isUploading = false;
      }
    },
    // drag в очереди
    onQueueDragStart(idx) { this.dragQueueIdx = idx; },
    onQueueDragOver(idx) {
      if (this.dragQueueIdx === null || this.dragQueueIdx === idx) return;
      const arr = this.uploadQueue;
      const moved = arr[this.dragQueueIdx];
      arr.splice(this.dragQueueIdx, 1);
      arr.splice(idx, 0, moved);
      this.dragQueueIdx = idx;
    },
    onQueueDragEnd() { this.dragQueueIdx = null; },

    // --- Модалка групп ---
    openGroupsModal() {
      this.editableGroups = this.groups.map((g) => ({
        ...g,
        _editMode: false,
        _editName: g.name,
      }));
      this.newGroupName = '';
      this.showGroupsModal = true;
    },
    closeGroupsModal() {
      this.showGroupsModal = false;
    },
    startGroupRename(grp) {
      grp._editName = grp.name;
      grp._editMode = true;
    },
    async saveGroupRename(grp) {
      const name = grp._editName.trim();
      if (!name) return;
      try {
        await renameDocumentGroup(grp.id, { name });
        grp.name = name;
        grp._editMode = false;
        // Обновляем в основном массиве
        const g = this.groups.find((x) => x.id === grp.id);
        if (g) g.name = name;
        // Обновляем group_name в документах
        this.documents.forEach((d) => {
          if (d.group_id === grp.id) d.group_name = name;
        });
        useDeletionsStore().notify({ prefix: 'Группа переименована в ', bold: name });
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Ошибка: ', bold: e?.message || 'сбой', type: 'error' });
      }
    },
    async deleteGroup(grp) {
      try {
        await deleteDocumentGroup(grp.id);
        const idx = this.editableGroups.findIndex((g) => g.id === grp.id);
        if (idx !== -1) this.editableGroups.splice(idx, 1);
        const gi = this.groups.findIndex((g) => g.id === grp.id);
        if (gi !== -1) this.groups.splice(gi, 1);
        // Документы группы переходят в «Прочее»
        this.documents.forEach((d) => {
          if (d.group_id === grp.id) { d.group_id = null; d.group_name = null; }
        });
        useDeletionsStore().notify({ prefix: 'Группа ', bold: grp.name, suffix: ' удалена' });
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Ошибка: ', bold: e?.message || 'сбой', type: 'error' });
      }
    },
    async createGroup() {
      const name = this.newGroupName.trim();
      if (!name) return;
      this.isCreatingGroup = true;
      try {
        const g = await createDocumentGroup({ name });
        const newGroup = { ...g, count: 0 };
        this.groups.push(newGroup);
        this.editableGroups.push({ ...newGroup, _editMode: false, _editName: name });
        this.newGroupName = '';
        useDeletionsStore().notify({ prefix: 'Группа ', bold: name, suffix: ' создана' });
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Ошибка: ', bold: e?.message || 'сбой', type: 'error' });
      } finally {
        this.isCreatingGroup = false;
      }
    },
    // drag в группах
    onGroupDragStart(idx) { this.dragGroupIdx = idx; },
    onGroupDragOver(idx) {
      if (this.dragGroupIdx === null || this.dragGroupIdx === idx) return;
      const arr = this.editableGroups;
      const moved = arr[this.dragGroupIdx];
      arr.splice(this.dragGroupIdx, 1);
      arr.splice(idx, 0, moved);
      this.dragGroupIdx = idx;
    },
    async onGroupDragEnd() {
      this.dragGroupIdx = null;
      const ids = this.editableGroups.map((g) => g.id);
      try {
        await reorderDocumentGroups(ids);
        // Синхронизируем this.groups
        this.groups = [...this.editableGroups.map((g) => ({
          id: g.id, name: g.name, count: g.count, sort_order: g.sort_order,
        }))];
      } catch {
        // порядок групп не критичен для пользователя — ошибку не показываем
      }
    },

    // --- Утилиты ---
    formatDate(dt) {
      if (!dt) return '';
      return new Date(dt).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
    },
    formatBytes,
    fileWord(n) {
      if (n % 10 === 1 && n % 100 !== 11) return 'файл';
      if ([2, 3, 4].includes(n % 10) && ![12, 13, 14].includes(n % 100)) return 'файла';
      return 'файлов';
    },
  },
};
</script>

<style scoped>
.documents-container {
  display: flex;
  flex-direction: column;
  background: var(--surface);
  border-radius: 35px;
  border: 1px solid var(--border);
  overflow: hidden;
}

/* --- Шапка --- */
.management-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 20px;
  height: 50px;
  border-bottom: 1px solid var(--color-border);
  flex-shrink: 0;
}

.management-title {
  margin: 0;
  font-size: 1.2em;
  font-weight: 600;
  color: var(--text);
  flex-shrink: 0;
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

/* Фиксированная ширина фильтра групп - не прыгает от длины выбранной группы
   (длинные имена обрезаются по ellipsis внутри BaseDropdown). */
.group-filter-dropdown {
  width: 190px;
  flex-shrink: 0;
}

/* Шрифт фильтра выравниваем под соседние .lk-button (13px), а не дефолтные 14px BaseDropdown. */
.group-filter-dropdown :deep(.base-dropdown__text) {
  font-size: 13px;
}

/* --- Master-detail --- */
.content-container {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

.table-section {
  width: 46%;
  border-right: 1px solid var(--color-border);
  display: flex;
  flex-direction: column;
  transition: width 0.2s ease;
}

.table-section.with-details {
  width: 44%;
}

.table-container {
  display: flex;
  flex-direction: column;
  flex: 1;
  overflow: hidden;
}

.table-body {
  flex: 1;
  overflow-y: auto;
}

.table-footer {
  padding: 10px 16px;
  border-top: 1px solid var(--color-border);
  font-size: 12px;
  color: var(--text-muted);
  background: var(--accent-tint);
  flex-shrink: 0;
}

.items-count {
  font-weight: 500;
}

/* --- Строка документа --- */
.docs-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  cursor: pointer;
  border-bottom: 1px solid var(--border);
  transition: background 0.15s ease;
}

.docs-row:last-child {
  border-bottom: none;
}

.docs-row:hover {
  background: var(--accent-tint);
}

.docs-row.selected {
  background: var(--accent-tint);
}

.docs-drag-handle {
  color: var(--accent-text);
  cursor: grab;
  font-size: 14px;
  user-select: none;
  flex-shrink: 0;
}

.docs-row-main {
  flex: 1;
  min-width: 0;
}

.docs-row-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.docs-row-meta {
  font-size: 10.5px;
  color: var(--text-muted);
  margin-top: 3px;
}

.docs-badge {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 12px;
}

.docs-badge--hidden {
  background: var(--accent-tint);
  color: var(--text-muted);
}

.docs-badge--group {
  background: var(--accent-tint);
  color: var(--accent-text);
}

/* --- Панель деталей --- */
.details-section {
  flex: 1;
  padding: 22px 24px;
  overflow-y: auto;
}

.doc-detail-preview {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 18px;
}

.doc-detail-filename {
  font-size: 15px;
  font-weight: 700;
  color: var(--text);
}

.doc-detail-meta {
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 3px;
}

.form-group {
  margin-bottom: 14px;
}

.form-label {
  display: block;
  font-size: 12px;
  font-weight: 600;
  color: var(--text);
  margin-bottom: 6px;
}

.form-error {
  color: var(--danger-text);
  font-size: 12px;
  margin-top: 6px;
}

.lk-textarea {
  resize: vertical;
  min-height: 60px;
  width: 100%;
}

/* --- Toggle switch --- */
.switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 11px 0;
  border-top: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  margin-bottom: 4px;
}

.switch-label {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--text);
}

.switch-desc {
  font-size: 10.5px;
  color: var(--text-muted);
  margin-top: 2px;
}

.toggle-switch {
  width: 42px;
  height: 24px;
  border-radius: 50px;
  background: var(--accent);
  position: relative;
  flex-shrink: 0;
  cursor: pointer;
  border: none;
  outline: none;
  transition: background 0.2s ease;
}

.toggle-switch--on {
  background: var(--color-primary);
}

.toggle-switch::after {
  content: '';
  position: absolute;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: var(--surface);
  top: 3px;
  left: 3px;
  transition: transform 0.18s ease;
}

.toggle-switch--on::after {
  transform: translateX(18px);
}

/* --- Действия с деталью --- */
.detail-actions {
  display: flex;
  gap: 8px;
  margin-top: 18px;
  padding-top: 16px;
  border-top: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  flex-wrap: wrap;
}

/* --- No selection --- */
.no-selection-message {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-size: 13px;
}

.no-results {
  padding: 30px;
  text-align: center;
  color: var(--text-muted);
  font-size: 12px;
}

.docs-loading {
  padding: 20px;
}

/* --- Модалка --- */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1100;
}

.docs-modal {
  width: 560px;
  max-width: 95vw;
  max-height: calc(var(--app-vh, 1vh) * 90);
  background: var(--surface);
  border-radius: 30px;
  box-shadow: 0 20px 60px rgba(20, 22, 60, 0.18);
  border: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 24px 0;
  flex-shrink: 0;
}

.modal-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--text);
}

.modal-close {
  background: none;
  border: none;
  font-size: 20px;
  color: var(--text-muted);
  cursor: pointer;
  padding: 0 4px;
  line-height: 1;
}

.modal-body {
  padding: 18px 24px;
  overflow-y: auto;
  flex: 1;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 14px 24px 22px;
  flex-shrink: 0;
}

/* --- Dropzone --- */
.dropzone {
  border: 2px dashed color-mix(in srgb, var(--accent) 25%, var(--surface));
  border-radius: 16px;
  background: var(--accent-tint);
  padding: 26px;
  text-align: center;
  color: var(--accent-text);
  font-size: 12.5px;
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;
}

.dropzone--over {
  border-color: var(--accent);
  background: var(--accent-tint);
}

.dropzone-text {
  margin-bottom: 4px;
}

.dropzone-hint {
  font-size: 11px;
  color: var(--accent-text);
  margin-top: 6px;
}

/* --- Очередь загрузки --- */
.upload-queue-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 16px 0 8px;
  flex-wrap: wrap;
  gap: 8px;
}

.uq-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px;
  border: 1px solid var(--color-border);
  border-radius: 14px;
  margin-bottom: 8px;
  background: var(--surface);
}

.uq-drag {
  color: var(--accent-text);
  cursor: grab;
  font-size: 13px;
  margin-top: 3px;
  user-select: none;
  flex-shrink: 0;
}

.uq-ord {
  width: 20px;
  height: 20px;
  border-radius: 6px;
  background: var(--accent-tint);
  color: var(--accent-text);
  font-size: 11px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  margin-top: 2px;
}

.uq-fields {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.uq-filename {
  font-size: 12px;
  font-weight: 600;
  color: var(--text);
}

.uq-size {
  color: var(--accent-text);
  font-weight: 400;
}

.uq-remove {
  background: none;
  border: none;
  color: var(--danger-text);
  cursor: pointer;
  font-size: 16px;
  margin-top: 2px;
  flex-shrink: 0;
  padding: 0;
  line-height: 1;
}

.uq-remove:hover {
  color: var(--danger-text);
}

/* --- Строки групп --- */
.grp-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 12px;
  border: 1px solid var(--color-border);
  border-radius: 14px;
  margin-bottom: 8px;
  cursor: grab;
}

.grp-row--misc {
  cursor: default;
  opacity: 0.7;
}

.grp-drag {
  color: var(--accent-text);
  cursor: grab;
  user-select: none;
  font-size: 14px;
}

.grp-row-info {
  flex: 1;
  min-width: 0;
}

.grp-name {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--text);
}

.grp-cnt {
  font-size: 10.5px;
  color: var(--text-muted);
  margin-top: 2px;
}

.grp-actions {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}

.grp-edit-row {
  display: flex;
  gap: 6px;
  align-items: center;
}

.new-group-row {
  display: flex;
  gap: 8px;
}

/* --- Анимация модалки --- */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .docs-modal,
.modal-fade-leave-active .docs-modal {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.modal-fade-enter-from .docs-modal,
.modal-fade-leave-to .docs-modal {
  opacity: 0;
  transform: translateY(-12px) scale(0.97);
}

/* --- BaseDropdown для фильтра групп --- */
.group-filter-dropdown {
  min-width: 140px;
}

/* Список уже card-like (иконка+название+бейджи+мета вместо колонок таблицы) -
   rt-table/data-label тут не подходят (нет head-row, нечего скрывать/подписывать),
   поэтому карточный вид на узком экране собираем локально: master-detail
   стекается (компонент раньше не имел ни одного @media - на мобилке колонки
   .table-section/.details-section просто сжимались бок о бок), .docs-row
   получает границу и радиус карточки, шапка ужимается тем же приёмом, что и
   в остальных 5 компонентах среза. */
@media (max-width: 767.98px) {
  .management-header {
    height: auto;
    padding: 16px;
  }

  .group-filter-dropdown {
    min-width: 110px;
    width: auto;
  }

  /* 4 контрола (дропдаун + 2 текстовых кнопки + Обновить) не помещаются в
     одну строку даже на всю ширину контейнера - "Управление группами" длинный
     текст, не компактится (не Add-кнопка). Переносим строкой ВНУТРИ
     header-controls, а не разваливаем на вертикальный стек по одной кнопке -
     каждый контрол остаётся пилюлей нормальной ширины. */
  .header-controls {
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .content-container {
    flex-direction: column;
    height: auto;
  }

  .table-section,
  .table-section.with-details,
  .details-section,
  .no-selection-message {
    width: 100%;
  }

  .table-section {
    border-right: none;
    border-bottom: 1px solid var(--color-border);
  }

  .table-body {
    max-height: 320px;
    padding: 8px;
  }

  .docs-row {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md, 15px);
    margin-bottom: 8px;
  }

  .docs-row:last-child {
    margin-bottom: 0;
  }
}
</style>
