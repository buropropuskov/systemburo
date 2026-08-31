<template>
  <div class="attachments-management-container dashboard-card">
    <div class="management-header rt-header-inline">
      <h3 class="management-title">
        Вложения заявок
      </h3>
      <div class="header-controls">
        <BaseDropdown
          class="archive-dropdown"
          :model-value="showArchive ? 'archive' : 'active'"
          :options="archiveOptions"
          label-key="label"
          value-key="value"
          @update:model-value="onArchiveModeChange"
        />
        <SearchComponent
          v-model="searchQuery"
          :title="'Поиск вложений...'"
        />
        <button
          class="add-header-button rt-btn-compact"
          aria-label="Создать вложение"
          data-testid="attachment-add-btn"
          @click="openAddModal"
        >
          <span
            class="rt-btn-icon"
            aria-hidden="true"
          >+</span>
          <span class="rt-btn-label">Создать вложение</span>
        </button>
        <RefreshButton
          :loading="isLoading"
          @refresh="refresh"
        />
      </div>
    </div>

    <div class="content-container">
      <div
        class="table-section"
        :class="{ 'with-details': selectedAttachment }"
      >
        <div class="table-container rt-table">
          <div class="table-header rt-head-row">
            <div
              class="header-col id-col"
              @click="sortBy('id')"
            >
              <p :class="{ 'active-sort': sortField === 'id' }">
                ID
              </p>
              <AppIcon
                name="sort"
                class="sort-icon"
                :class="{ sorted: sortField === 'id', desc: sortField === 'id' && sortDirection === 'desc' }"
              />
            </div>
            <div
              class="header-col name-col"
              @click="sortBy('display_name')"
            >
              <p :class="{ 'active-sort': sortField === 'display_name' }">
                Наименование
              </p>
              <AppIcon
                name="sort"
                class="sort-icon"
                :class="{ sorted: sortField === 'display_name', desc: sortField === 'display_name' && sortDirection === 'desc' }"
              />
            </div>
          </div>

          <div class="table-body">
            <div
              v-for="a in filteredAttachments"
              :key="a.id"
              class="table-row rt-row"
              data-testid="attachment-row"
              :class="{
                selected: selectedAttachment && selectedAttachment.id === a.id,
                inactive: !a.is_active,
              }"
              @click="selectAttachment(a)"
            >
              <div
                class="table-col id-col"
                data-label="ID"
              >
                <span class="cell-content id-value">{{ a.id }}</span>
              </div>
              <div
                class="table-col name-col"
                data-label="Наименование"
              >
                <div class="name-with-badges">
                  <span
                    class="truncate-text"
                    :title="a.display_name"
                  >{{ a.display_name }}</span>
                  <span
                    class="type-badge"
                    :class="a.attachment_type"
                  >{{ typeLabel(a.attachment_type) }}</span>
                  <span
                    v-if="!a.is_active"
                    class="inactive-badge"
                  >(архив)</span>
                </div>
              </div>
            </div>

            <div
              v-if="!filteredAttachments.length && !isLoading"
              class="no-results"
            >
              {{ emptyText }}
            </div>
            <div
              v-if="isLoading && !items.length"
              class="attachments-loading"
            >
              <LoaderSpinner label="Загрузка вложений..." />
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">
              {{ showArchive ? 'В архиве' : 'Всего вложений' }}: {{ filteredAttachments.length }}
            </span>
          </div>
        </div>
      </div>

      <div
        v-if="selectedAttachment"
        class="details-section"
        data-testid="attachment-details"
      >
        <div class="tab-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <input
                v-if="editingName"
                ref="nameEditInput"
                v-model="editingNameValue"
                class="lk-input name-edit-input"
                maxlength="255"
                @keyup.enter="saveNameEdit"
                @keyup.escape="cancelNameEdit"
                @blur="saveNameEdit"
              >
              <template v-else>
                <h3 class="details-title">
                  {{ original.display_name }}
                </h3>
                <button
                  v-if="selectedAttachment.is_active"
                  class="name-edit-btn"
                  title="Переименовать"
                  @click="startNameEdit"
                >
                  <AppIcon
                    name="edit"
                    class="name-edit-icon"
                  />
                </button>
              </template>
              <span
                class="type-badge details-type-badge"
                :class="selectedAttachment.attachment_type"
              >{{ typeLabel(selectedAttachment.attachment_type) }}</span>
            </div>
            <div class="details-header-actions">
              <span
                v-if="!selectedAttachment.is_active"
                class="archive-badge"
              >В архиве</span>
              <button
                class="action-btn history-btn"
                data-testid="attachment-history"
                @click="openHistory(selectedAttachment)"
              >
                История
              </button>
              <button
                v-if="selectedAttachment.is_active"
                class="action-btn template-btn"
                data-testid="attachment-template-btn"
                @click="showTemplateEditor = true"
              >
                Excel-бланк
              </button>
              <button
                v-if="selectedAttachment.is_active"
                class="action-btn template-btn"
                data-testid="attachment-fields-btn"
                @click="openFieldsConfig(selectedAttachment)"
              >
                Настройка полей
              </button>
              <button
                v-if="selectedAttachment.is_active"
                class="action-btn archive-action-btn"
                data-testid="attachment-archive"
                @click="onArchiveClick(selectedAttachment)"
              >
                В архив
              </button>
              <button
                v-else
                class="action-btn restore-btn"
                data-testid="attachment-restore"
                @click="onRestore(selectedAttachment)"
              >
                Восстановить
              </button>
            </div>
          </div>

          <div class="details-body">
            <div class="form-row">
              <div class="form-group">
                <label class="field-label">Наименование вложения</label>
                <input
                  v-model="form.display_name"
                  type="text"
                  class="lk-input"
                  maxlength="255"
                  placeholder="Название вложения"
                  :disabled="!selectedAttachment.is_active || isSaving"
                  data-testid="attachment-detail-name"
                  @keyup.enter="saveSelected"
                >
              </div>

              <div class="form-group">
                <label class="field-label">Системное имя</label>
                <input
                  :value="form.name"
                  type="text"
                  class="lk-input"
                  disabled
                  title="Системное имя задаётся при создании и не меняется"
                  data-testid="attachment-detail-system-name"
                >
                <span class="field-hint">Системное имя задаётся при создании и не изменяется</span>
              </div>
            </div>

            <div class="form-row">
              <div class="form-group">
                <label class="field-label">Заголовок</label>
                <input
                  v-model="form.title"
                  type="text"
                  class="lk-input"
                  maxlength="255"
                  placeholder="АВТОЗАЯВКИ"
                  :disabled="!selectedAttachment.is_active || isSaving"
                  data-testid="attachment-detail-title"
                  @input="form.title = form.title.toUpperCase()"
                  @keyup.enter="saveSelected"
                >
                <span class="field-hint">Отображается в заголовке категории (всегда в верхнем регистре)</span>
              </div>

              <div class="form-group">
                <label class="field-label">Тип вложения</label>
                <BaseDropdown
                  class="type-dropdown"
                  :model-value="form.attachment_type"
                  :options="typeOptions"
                  label-key="label"
                  value-key="value"
                  :disabled="!selectedAttachment.is_active || isSaving"
                  @update:model-value="form.attachment_type = $event"
                />
              </div>
            </div>

            <div class="form-row">
              <div class="form-group">
                <label class="field-label">Автосохранение в файловый архив</label>
                <span
                  class="hint-anchor"
                  :data-hint="autoExportHint"
                  data-testid="attachment-auto-export-hint"
                >
                  <ToggleSwitch
                    v-model="form.auto_export"
                    :disabled="!selectedAttachment.is_active || isSaving || autoExportDisabled"
                    data-testid="attachment-auto-export"
                  >
                    {{ form.auto_export ? 'Включено' : 'Выключено' }}
                  </ToggleSwitch>
                </span>
                <span class="field-hint">Писать ли бланки этого типа в файловый архив бюро</span>
              </div>
            </div>

            <div class="form-group">
              <label class="field-label">Инструкция к вложению</label>
              <TextConstructor
                v-model="form.instruction"
                :disabled="!selectedAttachment.is_active"
                placeholder="Введите инструкцию для вложения..."
                rows="6"
              />
            </div>

            <div
              v-if="detailError"
              class="form-error"
            >
              {{ detailError }}
            </div>

            <div
              v-if="selectedAttachment.is_active"
              class="details-actions"
            >
              <button
                class="lk-button lk-button--primary"
                :disabled="!isDetailsDirty || isSaving"
                data-testid="attachment-save"
                @click="saveSelected"
              >
                Сохранить
              </button>
            </div>

            <template v-if="selectedAttachment.is_active">
              <AttachmentTemplateEditor
                :key="`te-${selectedAttachment.id}`"
                :show="showTemplateEditor"
                :unique-attachment-id="selectedAttachment.id"
                :attachment-type="selectedAttachment.attachment_type"
                @close="onTemplateEditorClose"
              />
            </template>

            <div class="details-meta">
              <span>ID: {{ selectedAttachment.id }}</span>
              <span v-if="selectedAttachment.created_at">Создано: {{ formatDate(selectedAttachment.created_at) }}</span>
            </div>
          </div>
        </div>
      </div>
      <div
        v-else
        class="no-selection-message"
      >
        <p>{{ showArchive ? 'Выберите архивное вложение' : 'Выберите вложение для просмотра и редактирования' }}</p>
      </div>
    </div>

    <!-- Модалка создания -->
    <BaseModal
      :show="showAddModal"
      title="Новое вложение"
      width="480px"
      radius="30px"
      content-testid="attachment-modal"
      @close="requestCloseAdd"
    >
      <div class="attachment-modal-body">
        <div class="form-group">
          <label class="form-label">Тип вложения</label>
          <BaseDropdown
            teleport
            :menu-z-index="1100"
            :model-value="addForm.attachment_type"
            :options="typeOptions"
            label-key="label"
            value-key="value"
            @update:model-value="addForm.attachment_type = $event"
          />
        </div>

        <div class="form-group">
          <label class="form-label">Наименование вложения</label>
          <input
            v-model="addForm.display_name"
            type="text"
            class="lk-input"
            maxlength="255"
            placeholder="Автозаявка"
            data-testid="attachment-input-display-name"
            @keyup.enter="submitAdd"
          >
        </div>

        <div class="form-group">
          <label class="form-label">Системное имя</label>
          <input
            v-model="addForm.name"
            type="text"
            class="lk-input"
            :class="{ 'input-error': !!nameError }"
            maxlength="255"
            placeholder="avtozayavka"
            data-testid="attachment-input-name"
            @input="onNameInput"
          >
          <span class="field-hint">Латинские буквы, цифры и подчёркивания</span>
          <span
            v-if="nameError"
            class="form-error"
          >{{ nameError }}</span>
        </div>

        <div class="form-group">
          <label class="form-label">Заголовок</label>
          <input
            v-model="addForm.title"
            type="text"
            class="lk-input"
            maxlength="255"
            placeholder="АВТОЗАЯВКИ"
            data-testid="attachment-input-title"
            @input="addForm.title = addForm.title.toUpperCase()"
            @keyup.enter="submitAdd"
          >
          <span class="field-hint">Отображается в заголовке категории (в верхнем регистре)</span>
        </div>

        <div
          v-if="archivedDuplicate"
          class="duplicate-hint"
          data-testid="attachment-duplicate-hint"
        >
          <span>В архиве уже есть вложение <b>{{ archivedDuplicate.display_name }}</b> с такими данными.</span>
          <button
            class="lk-button lk-button--secondary"
            :disabled="isAdding"
            @click="restoreDuplicate(archivedDuplicate)"
          >
            Восстановить из архива
          </button>
        </div>

        <div
          v-if="addError"
          class="form-error"
        >
          {{ addError }}
        </div>
      </div>

      <template #actions>
        <button
          class="lk-button lk-button--ghost"
          data-testid="attachment-modal-cancel"
          @click="requestCloseAdd"
        >
          Отмена
        </button>
        <button
          class="lk-button lk-button--primary"
          :disabled="!addValid || isAdding"
          data-testid="attachment-modal-save"
          @click="submitAdd"
        >
          Создать
        </button>
      </template>
    </BaseModal>

    <ConfirmationModal
      :show="!!archiveConfirm"
      title="Архивация вложения"
      :message="archiveConfirm ? `Переместить вложение «${archiveConfirm.display_name}» в архив? Его можно будет восстановить.` : ''"
      confirm-text="В архив"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
      @confirm="performArchive"
      @cancel="archiveConfirm = null"
    />

    <UniqueAttachmentHistoryModal
      v-if="historyForAttachment"
      :attachment="historyForAttachment"
      :current-user-name="currentUserName"
      @close="historyForAttachment = null"
    />

    <AttachmentFieldsModal
      v-if="fieldsForAttachment"
      :unique-attachment-id="fieldsForAttachment.id"
      :attachment-name="fieldsForAttachment.name"
      @close="fieldsForAttachment = null"
    />
  </div>
</template>

<script>
import SearchComponent from './SearchComponent.vue';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import RefreshButton from './RefreshButton.vue';
import ConfirmationModal from './ConfirmationModal.vue';
import TextConstructor from './TextConstructor.vue';
import BaseDropdown from './ui/BaseDropdown.vue';
import BaseModal from './ui/BaseModal.vue';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import ToggleSwitch from './ui/ToggleSwitch.vue';
import AttachmentFieldsModal from './admin/AttachmentFieldsModal.vue';
import AttachmentTemplateEditor from './admin/AttachmentTemplateEditor.vue';
import UniqueAttachmentHistoryModal from './UniqueAttachmentHistoryModal.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { registerDirtyTracker, confirmIfAnyDirty } from '@/utils/dirtyTracker';
import { apiRequest } from '@/api/client';
import {
  listAllAttachments,
  createAttachment,
  updateAttachment,
  archiveAttachment,
  restoreAttachment,
} from '@/api/attachments';
import { getTemplate } from '@/api/attachment-templates';
import { getArchiveSettings } from '@/api/fileArchive';
import AppIcon from '@/components/icons/AppIcon.vue';

const SYSTEM_NAME_RE = /^[a-z0-9_]*$/;

function emptyForm() {
  return {
    display_name: '', name: '', title: '', attachment_type: 'cars', instruction: '', auto_export: true,
  };
}

export default {
  name: 'AttachmentsManagement',
  components: {
    SearchComponent,
    RefreshButton,
    ConfirmationModal,
    TextConstructor,
    BaseDropdown, BaseModal,
    LoaderSpinner,
    ToggleSwitch,
    AttachmentFieldsModal,
    AttachmentTemplateEditor,
    UniqueAttachmentHistoryModal,
    AppIcon,
  },
  setup() {
    return { deletions: useDeletionsStore() };
  },
  data() {
    return {
      items: [],
      searchQuery: '',
      showArchive: false,
      sortField: null,
      sortDirection: 'asc',
      isLoading: false,
      selectedAttachment: null,
      form: emptyForm(),
      original: emptyForm(),
      detailError: '',
      isSaving: false,
      showTemplateEditor: false,
      showAddModal: false,
      addForm: { display_name: '', name: '', title: '', attachment_type: 'cars' },
      addError: '',
      nameError: '',
      isAdding: false,
      archiveConfirm: null,
      historyForAttachment: null,
      fieldsForAttachment: null,
      editingName: false,
      editingNameValue: '',
      currentUserName: '',
      // Тумблер архива (#1615): глобальный рубильник грузится один раз, а признак
      // активного бланка - на каждый выбор строки (у своего вложения он свой).
      archiveEnabled: false,
      hasActiveTemplate: false,
      archiveOptions: [
        { label: 'Активные', value: 'active' },
        { label: 'Архив', value: 'archive' },
      ],
      typeOptions: [
        { label: 'Машины', value: 'cars' },
        { label: 'Люди', value: 'people' },
        { label: 'ТМЦ', value: 'items' },
      ],
    };
  },
  computed: {
    filteredAttachments() {
      const variants = buildSearchVariants(this.searchQuery);
      let list = this.items.filter(a => (this.showArchive ? !a.is_active : a.is_active));
      if (variants.length) {
        list = list.filter(a => matchesSearch(
          `${a.display_name || ''} ${a.name || ''} ${a.title || ''} ${a.id}`,
          variants,
        ));
      }
      return this.sortList(list);
    },
    emptyText() {
      if (this.searchQuery.trim()) return 'Ничего не найдено по запросу';
      return this.showArchive ? 'В архиве пусто' : 'Вложений пока нет';
    },
    // Архивный дубль по любому из уникальных полей - предлагаем восстановить
    // вместо создания нового (иначе у активного и архивного совпадут имена).
    archivedDuplicate() {
      const dn = this.addForm.display_name.trim().toLowerCase();
      const nm = this.addForm.name.trim().toLowerCase();
      const tt = this.addForm.title.trim().toUpperCase();
      if (!dn && !nm && !tt) return null;
      return this.items.find(a => !a.is_active && (
        (dn && (a.display_name || '').toLowerCase() === dn)
        || (nm && (a.name || '').toLowerCase() === nm)
        || (tt && (a.title || '').toUpperCase() === tt)
      )) || null;
    },
    addValid() {
      const f = this.addForm;
      return f.display_name.trim() !== ''
        && f.name.trim() !== ''
        && f.title.trim() !== ''
        && !this.nameError;
    },
    isAddDirty() {
      if (!this.showAddModal) return false;
      const f = this.addForm;
      return f.display_name.trim() !== '' || f.name.trim() !== '' || f.title.trim() !== '';
    },
    isDetailsDirty() {
      const s = this.selectedAttachment;
      if (!s || !s.is_active) return false;
      const f = this.form;
      const o = this.original;
      return f.display_name.trim() !== o.display_name
        || f.title.trim() !== o.title
        || f.attachment_type !== o.attachment_type
        || (f.instruction || '') !== (o.instruction || '')
        || !!f.auto_export !== !!o.auto_export;
    },
    isDirty() {
      return this.isAddDirty || this.isDetailsDirty;
    },
    // Тумблер архива заблокирован в двух случаях (#1615): глобальный рубильник
    // выключен на сервере, либо у ЭТОГО типа вложения нет активного
    // Excel-бланка - без него генерировать нечего.
    autoExportDisabled() {
      return !this.archiveEnabled || !this.hasActiveTemplate;
    },
    autoExportHint() {
      // Раздел «Файловый архив» рубильник только показывает: включает его команда
      // server archive on на сервере, и подсказка не должна звать туда, где кнопки нет.
      if (!this.archiveEnabled) return 'Файловый архив выключен - включается на сервере командой server archive on';
      if (!this.hasActiveTemplate) return 'У вложения нет активного Excel-бланка - настройте его во вкладке «Excel-бланк»';
      return '';
    },
  },
  created() {
    this._templateStatusSeq = 0;
  },
  mounted() {
    this.refresh();
    this.fetchCurrentUser();
    this.loadArchiveEnabled();
    this._stopGuard = registerDirtyTracker({
      isDirty: () => this.isDirty,
      getChanges: () => {
        if (this.isAddDirty) return [`Новое вложение: "${this.addForm.display_name.trim() || this.addForm.name.trim()}"`];
        if (this.isDetailsDirty) {
          const f = this.form;
          const o = this.original;
          const ch = [];
          if (f.display_name.trim() !== o.display_name) {
            ch.push({ label: 'Наименование', from: o.display_name, to: f.display_name.trim() });
          }
          if (f.title.trim() !== o.title) {
            ch.push({ label: 'Заголовок', from: o.title, to: f.title.trim() });
          }
          if (f.attachment_type !== o.attachment_type) {
            ch.push({ label: 'Тип', from: this.typeLabel(o.attachment_type), to: this.typeLabel(f.attachment_type) });
          }
          if ((f.instruction || '') !== (o.instruction || '')) {
            ch.push({ label: 'Инструкция', from: '', to: 'изменена' });
          }
          if (!!f.auto_export !== !!o.auto_export) {
            ch.push({ label: 'Автосохранение в архив', from: o.auto_export ? 'включено' : 'выключено', to: f.auto_export ? 'включено' : 'выключено' });
          }
          return ch;
        }
        return [];
      },
      save: async () => {
        if (this.isAddDirty) await this.submitAdd();
        if (this.isDetailsDirty) await this.saveSelected();
      },
    });
  },
  beforeUnmount() {
    this._stopGuard?.();
  },
  methods: {
    typeLabel(type) {
      const o = this.typeOptions.find(t => t.value === type);
      return o ? o.label : type;
    },
    sortList(list) {
      const arr = [...list];
      if (!this.sortField) {
        return arr.sort((a, b) => (a.display_name || '').localeCompare(b.display_name || ''));
      }
      return arr.sort((a, b) => {
        if (this.sortField === 'id') {
          return this.sortDirection === 'asc' ? a.id - b.id : b.id - a.id;
        }
        const r = (a.display_name || '').localeCompare(b.display_name || '');
        return this.sortDirection === 'asc' ? r : -r;
      });
    },
    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'asc';
      }
    },
    formatDate(s) {
      if (!s) return '';
      return new Date(s).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
    },
    syncSelectedFrom(fresh) {
      this.selectedAttachment = { ...fresh };
      const vals = {
        display_name: fresh.display_name ?? '',
        name: fresh.name ?? '',
        title: fresh.title ?? '',
        attachment_type: fresh.attachment_type ?? 'cars',
        instruction: fresh.instruction ?? '',
        auto_export: !!fresh.auto_export,
      };
      this.form = { ...vals };
      this.original = { ...vals };
      this.showTemplateEditor = false;
      this.loadTemplateStatus(fresh.id);
    },
    // Признак «есть активный Excel-бланк» нужен только для гейта тумблера архива -
    // 404 «Шаблон не настроен» здесь штатный случай, а не сетевая ошибка (getTemplate
    // не проверяет res.ok, так же читает его AttachmentTemplateEditor.loadTemplate).
    async loadTemplateStatus(id) {
      const seq = (this._templateStatusSeq += 1);
      this.hasActiveTemplate = false;
      try {
        const data = await getTemplate(id);
        if (seq !== this._templateStatusSeq) return;
        this.hasActiveTemplate = !!(data && data.file_path);
      } catch {
        if (seq !== this._templateStatusSeq) return;
        this.hasActiveTemplate = false;
      }
    },
    // Настройка бланка могла появиться/исчезнуть в редакторе - перепроверяем гейт
    // тумблера архива при закрытии, иначе он останется disabled ещё один цикл.
    onTemplateEditorClose() {
      this.showTemplateEditor = false;
      if (this.selectedAttachment) this.loadTemplateStatus(this.selectedAttachment.id);
    },
    async loadArchiveEnabled() {
      try {
        const settings = await getArchiveSettings();
        this.archiveEnabled = !!(settings && settings.enabled);
      } catch {
        // Раздел архива недоступен/не настроен - тумблер остаётся заблокированным,
        // это безопасное значение по умолчанию, а не повод падать формой вложений.
        this.archiveEnabled = false;
      }
    },
    async refresh() {
      this.isLoading = true;
      try {
        const data = await listAllAttachments();
        this.items = Array.isArray(data) ? data : [];
        if (this.selectedAttachment) {
          const fresh = this.items.find(a => a.id === this.selectedAttachment.id);
          const visible = fresh && (this.showArchive ? !fresh.is_active : fresh.is_active);
          if (fresh && visible && !this.isDetailsDirty) {
            this.syncSelectedFrom(fresh);
          } else if (!visible) {
            this.selectedAttachment = null;
          }
        }
      } catch {
        this.deletions.notify({ prefix: 'Не удалось загрузить ', bold: 'вложения', type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },
    async onArchiveModeChange(value) {
      if (this.isDetailsDirty && !(await confirmIfAnyDirty())) return;
      this.showArchive = value === 'archive';
      this.selectedAttachment = null;
      this.detailError = '';
    },
    async selectAttachment(a) {
      if (this.selectedAttachment && this.selectedAttachment.id === a.id) return;
      if (this.isDetailsDirty && !(await confirmIfAnyDirty())) return;
      this.syncSelectedFrom(a);
      this.detailError = '';
    },
    async saveSelected() {
      if (!this.isDetailsDirty || this.isSaving) return;
      const displayName = this.form.display_name.trim();
      const title = this.form.title.trim().toUpperCase();
      if (!displayName || !title) {
        this.detailError = 'Заполните наименование и заголовок';
        return;
      }
      this.isSaving = true;
      this.detailError = '';
      try {
        await updateAttachment(this.selectedAttachment.id, {
          attachmentType: this.form.attachment_type,
          name: this.form.name,
          displayName,
          title,
          instruction: this.form.instruction || null,
          autoExport: this.form.auto_export,
        });
        this.deletions.notify({ prefix: 'Изменения сохранены в ', bold: displayName });
        this.form.display_name = displayName;
        this.form.title = title;
        this.original = { ...this.form };
        await this.refresh();
      } catch (e) {
        this.detailError = e?.message || 'Не удалось сохранить';
      } finally {
        this.isSaving = false;
      }
    },
    openAddModal() {
      this.showAddModal = true;
      this.addForm = { display_name: '', name: '', title: '', attachment_type: 'cars' };
      this.addError = '';
      this.nameError = '';
    },
    async requestCloseAdd() {
      if (this.isAddDirty && !(await confirmIfAnyDirty())) return;
      this.forceCloseAdd();
    },
    forceCloseAdd() {
      this.showAddModal = false;
      this.addForm = { display_name: '', name: '', title: '', attachment_type: 'cars' };
      this.addError = '';
      this.nameError = '';
    },
    onNameInput() {
      this.addForm.name = this.addForm.name.toLowerCase();
      this.nameError = SYSTEM_NAME_RE.test(this.addForm.name)
        ? ''
        : 'Только латинские буквы, цифры и подчёркивания';
    },
    async submitAdd() {
      const displayName = this.addForm.display_name.trim();
      const name = this.addForm.name.trim();
      const title = this.addForm.title.trim().toUpperCase();
      if (this.isAdding) return;
      // Молча выходить нельзя: кроме кнопки (она заблокирована на неполной форме)
      // сюда приходит «Сохранить все изменения» из диалога несохранённого, и
      // тихий выход там читается как «нажал, и ничего не произошло».
      if (!this.addValid) {
        this.addError = 'Заполните тип, наименование, системное имя и заголовок - без них вложение не создать.';
        // Ошибку в форме не видно, когда сохранение пришло из диалога
        // несохранённого: он лежит выше окна и перекрывает её. Тост -
        // единственный слой поверх диалога.
        this.deletions.notify({ prefix: this.addError, type: 'error' });
        return;
      }
      this.isAdding = true;
      this.addError = '';
      try {
        await createAttachment({
          attachmentType: this.addForm.attachment_type,
          name,
          displayName,
          title,
          instruction: null,
        });
        this.deletions.notify({ prefix: 'Вложение ', bold: displayName, suffix: ' создано' });
        this.forceCloseAdd();
        await this.refresh();
      } catch (e) {
        this.addError = e?.message || 'Не удалось создать вложение';
      } finally {
        this.isAdding = false;
      }
    },
    async restoreDuplicate(a) {
      if (this.isAdding) return;
      this.isAdding = true;
      try {
        await restoreAttachment(a.id);
        this.deletions.notify({ prefix: 'Вложение ', bold: a.display_name, suffix: ' восстановлено из архива' });
        this.forceCloseAdd();
        this.showArchive = false;
        await this.refresh();
      } catch (e) {
        this.addError = e?.message || 'Не удалось восстановить вложение';
      } finally {
        this.isAdding = false;
      }
    },
    onArchiveClick(a) {
      this.archiveConfirm = a;
    },
    async performArchive() {
      const a = this.archiveConfirm;
      this.archiveConfirm = null;
      if (!a) return;
      try {
        await archiveAttachment(a.id);
        this.deletions.notify({ prefix: 'Вложение ', bold: a.display_name, suffix: ' архивировано' });
        if (this.selectedAttachment && this.selectedAttachment.id === a.id && !this.showArchive) {
          this.selectedAttachment = null;
        }
        await this.refresh();
      } catch (e) {
        this.deletions.notify({ prefix: 'Не удалось архивировать: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
    async onRestore(a) {
      try {
        await restoreAttachment(a.id);
        this.deletions.notify({ prefix: 'Вложение ', bold: a.display_name, suffix: ' восстановлено из архива' });
        if (this.selectedAttachment && this.selectedAttachment.id === a.id && this.showArchive) {
          this.selectedAttachment = null;
        }
        await this.refresh();
      } catch (e) {
        this.deletions.notify({ prefix: 'Не удалось восстановить: ', bold: e?.message || 'ошибка', type: 'error' });
      }
    },
    startNameEdit() {
      this.editingNameValue = this.original.display_name;
      this.editingName = true;
      this.$nextTick(() => {
        this.$refs.nameEditInput?.focus();
        this.$refs.nameEditInput?.select();
      });
    },
    saveNameEdit() {
      if (!this.editingName) return;
      const trimmed = this.editingNameValue.trim();
      this.editingName = false;
      if (!trimmed || trimmed === this.original.display_name) return;
      this.form.display_name = trimmed;
      this.saveSelected();
    },
    cancelNameEdit() {
      this.editingName = false;
    },
    openHistory(a) {
      // Подпись в заголовке - актуальное наименование (из формы, если правилось).
      this.historyForAttachment = { id: a.id, name: this.original.display_name || a.display_name };
    },
    openFieldsConfig(a) {
      this.fieldsForAttachment = { id: a.id, name: this.original.display_name || a.display_name };
    },
    async fetchCurrentUser() {
      // Имя нужно для футера Excel-экспорта истории ("Отчёт сформировал").
      try {
        const res = await apiRequest('/users/me');
        if (!res.ok) return;
        const u = await res.json();
        const parts = [u.last_name, u.first_name, u.middle_name].filter(Boolean);
        this.currentUserName = parts.join(' ') || u.username || '';
      } catch {
        // Имя - необязательная деталь экспорта, молчим (footer покажет дефолт).
      }
    },
  },
};
</script>

<style scoped>
@import '@/assets/directory-management.css';

.attachments-management-container {
  background: var(--surface);
  border-radius: 16px;
  border: 1px solid var(--border);
  overflow: hidden;
}

.management-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  height: 50px;
  gap: 12px;
}

.header-controls {
  display: flex;
  gap: 10px;
  align-items: center;
}

/* Master-detail layout (эталон TableConstructor) */
.content-container {
  display: flex;
  height: 540px;
  width: 100%;
  overflow: hidden;
}

.table-section {
  width: 40%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--border);
  background: var(--surface);
}

.header-col {
  padding: 0 8px;
  font-size: 14px;
  color: var(--text-muted);
  font-weight: 600;
  text-align: left;
  display: flex;
  align-items: center;
  gap: 5px;
  transition: 0.2s;
  cursor: pointer;
  user-select: none;
}

.header-col p {
  margin: 0;
}

.sort-icon {
  color: var(--text-muted);
  width: 12px;
  height: 12px;
  transition: 0.2s;
}

.id-col {
  width: 25%;
  min-width: 60px;
}

.name-col {
  width: 75%;
  min-width: 160px;
}

.table-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.table-row.inactive .id-value {
  color: var(--text-muted);
}

/* Бейдж типа вложения (tonal pill) */
/* Имя и бейджи стоят в одной строке: обрезается только имя, бейджи целиком */
.name-with-badges {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.name-with-badges .truncate-text {
  flex: 0 1 auto;
  min-width: 0;
}

.name-with-badges .type-badge,
.name-with-badges .inactive-badge {
  flex: 0 0 auto;
  margin-left: 0;
}

.type-badge {
  display: inline-flex;
  align-items: center;
  font-size: 0.7em;
  line-height: 1;
  padding: 4px 8px;
  border-radius: 999px;
  margin-left: 6px;
  font-weight: 500;
  white-space: nowrap;
}

.type-badge.cars {
  background: var(--accent-tint);
  color: var(--accent-text);
}

.type-badge.people {
  background: var(--success-bg);
  color: var(--success-text);
}

/* Все три бейджа одной формы: заливка плюс цвет текста, без рамки. Рамка была только
   у ТМЦ и делала его на два пиксела выше остальных в той же строке. */
.type-badge.items {
  background: var(--warning-bg);
  color: var(--warning-text);
}

.no-results {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-muted);
  width: 100%;
}

.attachments-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 0;
}

.table-footer {
  padding: 6px 20px;
  border-top: 1px solid var(--border);
  text-align: right;
  background: var(--accent-tint);
}

/* Details */
.details-section {
  width: 60%;
  display: flex;
  flex-direction: column;
  background: var(--surface);
  overflow: hidden;
}

.tab-content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background: var(--surface);
  line-height: 1.5;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
  gap: 12px;
}

.details-title-wrapper {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  min-width: 0;
}

.details-title {
  margin: 0;
  color: var(--text);
  font-size: 1.2em;
  font-weight: 600;
  word-break: break-word;
}

.details-type-badge {
  margin-left: 0;
  font-size: 0.75em;
}

.details-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.action-btn {
  padding: 8px 16px;
  border: none;
  border-radius: 30px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  transition: background 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
  white-space: nowrap;
}

.action-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.history-btn {
  background: var(--surface);
  color: var(--accent-text);
  border: 1px solid var(--accent);
}

.history-btn:hover {
  background: var(--accent-tint);
}

.archive-action-btn {
  background: var(--surface);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.archive-action-btn:hover:not(:disabled) {
  background: var(--danger-bg);
  border-color: var(--danger);
}

.restore-btn {
  background: var(--success);
  color: var(--fill-text);
}

.restore-btn:hover {
  background: color-mix(in srgb, var(--success) 85%, var(--text));
}

.template-btn {
  background: var(--surface);
  color: var(--text);
  border: 1px solid var(--border);
}

.template-btn:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--accent-text);
  background: var(--accent-tint);
}

.details-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.field-label {
  font-size: 0.85em;
  color: var(--text-muted);
  font-weight: 500;
  margin-top: 6px;
}

.field-hint {
  font-size: 0.78em;
  color: var(--text-muted);
  line-height: 1.4;
}

/* Деталь в 2 колонки (как до 416-2), но с .lk-input 15px вместо легаси .form-input-sm */
.form-row {
  display: flex;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
  min-width: 0;
}

.details-body .lk-input,
.type-dropdown {
  width: 100%;
  max-width: 100%;
}

.details-actions {
  display: flex;
  gap: 10px;
  margin-top: 6px;
}

.details-meta {
  display: flex;
  gap: 16px;
  margin-top: 16px;
  font-size: 12px;
  color: var(--text-muted);
}

.form-error {
  color: var(--danger-text);
  font-size: 0.85em;
}

.input-error {
  border-color: var(--danger) !important;
}

.no-selection-message {
  width: 60%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-weight: 400;
  font-size: 14px;
}

/* Модалка создания */
/* Окно, затемнение, анимация и закрытие живут в BaseModal. Здесь остаются только
   отступы содержимого: base-modal__body идёт без padding, их несёт содержимое. */
.attachment-modal-body {
  padding: 22px 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

@media (max-width: 767.98px) {
  .management-header {
    height: auto;
    padding: 16px;
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
    border-bottom: 1px solid var(--border);
  }
  .table-body {
    max-height: 300px;
  }
  .form-row {
    flex-direction: column;
    gap: 8px;
  }
  .details-body .lk-input,
  .type-dropdown {
    max-width: 100%;
  }

  /* Список -> карточки (rt-table): дропдаун архива и поиск ужимаем, иначе
     строка контролов (дропдаун+поиск+компактные Создать/Обновить) не
     помещается на 375-390px (см. TableConstructor.vue - тот же паттерн). */
  .archive-dropdown {
    min-width: 92px;
  }

  :deep(.search) {
    width: 120px;
  }

  /* В карточке больше горизонтального места, чем в узкой табличной колонке -
     наименование не обрезаем многоточием. */
  .rt-row .truncate-text {
    white-space: normal;
    overflow: visible;
    text-overflow: clip;
  }
}

/* Inline-редактирование имени в заголовке детали */
.name-edit-btn {
  background: none;
  border: none;
  padding: 4px;
  cursor: pointer;
  opacity: 0.35;
  transition: opacity 0.15s, background 0.15s;
  display: flex;
  align-items: center;
  border-radius: 6px;
  flex-shrink: 0;
}

.name-edit-btn:hover {
  opacity: 1;
  background: var(--border);
}

.name-edit-icon {
  color: var(--text);
  width: 16px;
  height: 16px;
  display: block;
}

.name-edit-input {
  font-size: 1.2em;
  font-weight: 600;
  padding: 2px 8px;
  min-width: 0;
  flex: 1;
}
</style>
