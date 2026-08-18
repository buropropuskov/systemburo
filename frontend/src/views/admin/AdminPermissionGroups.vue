<template>
  <AdminPageShell>
    <div
      class="groups-container dashboard-card"
      data-testid="ob-admin-permission-groups"
    >
      <div class="management-header rt-header-inline">
        <h3 class="management-title">
          Группы прав доступа
        </h3>
        <div class="header-controls">
          <SearchComponent
            v-model="searchQuery"
            :title="'Поиск групп...'"
          />
          <button
            class="add-header-button rt-btn-compact"
            data-testid="group-add-btn"
            aria-label="Создать группу"
            @click="openCreate"
          >
            <span
              class="rt-btn-icon"
              aria-hidden="true"
            >+</span>
            <span class="rt-btn-label">Создать группу</span>
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
          :class="{ 'with-details': selectedGroup }"
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
                @click="sortBy('name')"
              >
                <p :class="{ 'active-sort': sortField === 'name' }">
                  Наименование
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{ sorted: sortField === 'name', desc: sortField === 'name' && sortDirection === 'desc' }"
                />
              </div>
              <div
                class="header-col keys-col"
                @click="sortBy('keys')"
              >
                <p :class="{ 'active-sort': sortField === 'keys' }">
                  Прав
                </p>
                <AppIcon
                  name="sort"
                  class="sort-icon"
                  :class="{ sorted: sortField === 'keys', desc: sortField === 'keys' && sortDirection === 'desc' }"
                />
              </div>
            </div>

            <div class="table-body">
              <div
                v-for="group in filteredGroups"
                :key="group.id"
                class="table-row rt-row"
                data-testid="group-row"
                :class="{ selected: selectedGroup && selectedGroup.id === group.id }"
                @click="selectGroup(group)"
              >
                <div
                  class="table-col id-col"
                  data-label="ID"
                >
                  <span class="cell-content id-value">{{ group.id }}</span>
                </div>
                <div
                  class="table-col name-col"
                  data-label="Наименование"
                >
                  <span
                    class="truncate-text"
                    :title="group.name"
                  >
                    {{ group.name }}
                  </span>
                </div>
                <div
                  class="table-col keys-col"
                  data-label="Прав"
                >
                  <span class="keys-count">{{ (group.keys || []).length }}</span>
                </div>
              </div>

              <div
                v-if="!filteredGroups.length && !isLoading"
                class="no-results"
              >
                {{ emptyText }}
              </div>
              <div
                v-if="isLoading && !groups.length"
                class="groups-loading"
              >
                <LoaderSpinner label="Загрузка групп..." />
              </div>
            </div>

            <div class="table-footer">
              <span class="items-count">
                Всего: {{ filteredGroups.length }}
              </span>
            </div>
          </div>
        </div>

        <div
          v-if="selectedGroup"
          class="details-section"
          data-testid="group-details"
        >
          <div class="tab-content">
            <div class="details-header">
              <div class="details-title-wrapper">
                <h3 class="details-title">
                  {{ original.name }}
                </h3>
              </div>
              <div class="details-header-actions">
                <button
                  class="action-btn copy-btn"
                  data-testid="group-copy"
                  @click="openCopy(selectedGroup)"
                >
                  Создать копию
                </button>
                <button
                  v-permission-scope="'permission.audit.manage'"
                  class="action-btn delete-btn"
                  data-testid="group-delete"
                  @click="deleteConfirm = { id: selectedGroup.id, name: original.name }"
                >
                  Удалить
                </button>
              </div>
            </div>

            <div class="details-body">
              <label class="field-label">Наименование</label>
              <input
                v-model.trim="selectedGroup.name"
                type="text"
                class="lk-input"
                maxlength="100"
                placeholder="Например, Доступ ко всем таблицам"
                :disabled="isSaving"
                data-testid="group-detail-name"
                @keyup.enter="saveSelected"
              >

              <label class="field-label">Описание</label>
              <textarea
                v-model.trim="selectedGroup.description"
                class="lk-textarea"
                rows="2"
                maxlength="500"
                placeholder="Для чего эта группа"
                :disabled="isSaving"
                data-testid="group-detail-description"
              />

              <div class="perms-block">
                <label class="field-label">Права группы</label>
                <span class="field-hint">
                  Точечные права, которые получают пользователи и роли с этой группой.
                </span>
                <div class="perms-summary">
                  <span
                    class="perms-count"
                    data-testid="group-keys-count"
                  >
                    Выбрано прав: {{ selectedKeys.length }}
                  </span>
                  <button
                    class="lk-button lk-button--ghost perms-edit-btn"
                    data-testid="group-edit-perms"
                    @click="openPermissions"
                  >
                    Редактировать права
                  </button>
                </div>
              </div>

              <div
                v-if="detailError"
                class="form-error"
              >
                {{ detailError }}
              </div>

              <div class="details-actions">
                <button
                  class="lk-button lk-button--primary"
                  :disabled="!isDetailsDirty || isSaving"
                  data-testid="group-save"
                  @click="saveSelected"
                >
                  {{ isSaving ? 'Сохранение...' : 'Сохранить' }}
                </button>
              </div>

              <div class="details-meta">
                <span>ID: {{ selectedGroup.id }}</span>
              </div>
            </div>
          </div>
        </div>
        <div
          v-else
          class="no-selection-message"
        >
          <p>Выберите группу для просмотра и редактирования</p>
        </div>
      </div>

      <!-- Модалка создания / копирования группы -->
      <Teleport to="body">
        <transition name="modal-fade">
          <div
            v-if="showMetaModal"
            class="modal-overlay"
            data-testid="group-modal"
            @mousedown="onOverlayMousedown"
            @mouseup="onOverlayMouseup"
          >
            <div
              class="group-modal"
              @mousedown.stop
            >
              <div class="modal-header">
                <h3>{{ modalMode === 'copy' ? `Копия группы «${copySource?.name}»` : 'Новая группа прав' }}</h3>
                <button
                  class="modal-close"
                  aria-label="Закрыть"
                  data-testid="group-modal-close"
                  @click="requestCloseMeta"
                >
                  ×
                </button>
              </div>

              <div class="modal-body">
                <div class="form-group">
                  <label class="form-label">Название</label>
                  <input
                    v-model.trim="metaForm.name"
                    type="text"
                    placeholder="Например, Доступ ко всем таблицам"
                    maxlength="100"
                    class="lk-input"
                    data-testid="group-input-name"
                    @keyup.enter="submitMeta"
                  >
                </div>

                <div class="form-group">
                  <label class="form-label">Описание</label>
                  <textarea
                    v-model.trim="metaForm.description"
                    class="lk-textarea"
                    rows="2"
                    maxlength="500"
                    placeholder="Для чего эта группа"
                    data-testid="group-input-description"
                  />
                </div>

                <p
                  v-if="modalMode === 'copy'"
                  class="copy-hint"
                >
                  Будут скопированы точечные права: {{ copySourceKeys.length }}.
                </p>

                <div
                  v-if="metaError"
                  class="form-error"
                >
                  {{ metaError }}
                </div>
              </div>

              <div class="modal-footer">
                <button
                  class="lk-button lk-button--ghost"
                  data-testid="group-modal-cancel"
                  @click="requestCloseMeta"
                >
                  Отмена
                </button>
                <button
                  class="lk-button lk-button--primary"
                  :disabled="!metaForm.name || isSubmitting"
                  data-testid="group-modal-save"
                  @click="submitMeta"
                >
                  {{ modalMode === 'copy' ? 'Создать копию' : 'Создать' }}
                </button>
              </div>
            </div>
          </div>
        </transition>
      </Teleport>

      <GroupPermissionsModal
        :show="permsModal.show"
        :title="permsModal.title"
        :initial-keys="permsModal.initialKeys"
        :catalog="catalog"
        :saving="isSavingPerms"
        @close="closePermissions"
        @save="handleSavePermissions"
      />

      <ConfirmationModal
        :show="!!deleteConfirm"
        title="Удаление группы прав"
        :message="deleteConfirm ? `Удалить группу «${deleteConfirm.name}»? Это действие нельзя отменить.` : ''"
        confirm-text="Удалить"
        cancel-text="Отмена"
        :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
        @confirm="performDelete"
        @cancel="deleteConfirm = null"
      />
    </div>
  </AdminPageShell>
</template>

<script>
import AdminPageShell from './AdminPageShell.vue';
import SearchComponent from '@/components/SearchComponent.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import ConfirmationModal from '@/components/ConfirmationModal.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import GroupPermissionsModal from '@/components/admin/GroupPermissionsModal.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { registerDirtyTracker, confirmIfAnyDirty } from '@/utils/dirtyTracker';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import {
  listPermissionGroups,
  getPermissionGroup,
  createPermissionGroup,
  updatePermissionGroup,
  deletePermissionGroup,
  getPermissionCatalog,
} from '@/api/permissions';
import AppIcon from '@/components/icons/AppIcon.vue';

export default {
  name: 'AdminPermissionGroups',
  components: { AdminPageShell, SearchComponent, RefreshButton, ConfirmationModal, LoaderSpinner, GroupPermissionsModal, AppIcon },
  setup() {
    // Колбэк закрытия модалки присваивается в created - нужен доступ к this с проверкой dirty.
    const overlay = { close: () => {} };
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => overlay.close());
    return { onOverlayMousedown, onOverlayMouseup, overlay };
  },
  data() {
    return {
      groups: [],
      catalog: [],
      searchQuery: '',
      sortField: null,
      sortDirection: 'asc',
      isLoading: false,
      selectedGroup: null,
      original: { name: '', description: '' },
      selectedKeys: [],
      detailError: '',
      isSaving: false,
      showMetaModal: false,
      modalMode: 'create',
      metaForm: { name: '', description: '' },
      metaError: '',
      isSubmitting: false,
      copySource: null,
      copySourceKeys: [],
      permsModal: { show: false, title: '', initialKeys: [] },
      isSavingPerms: false,
      deleteConfirm: null,
    };
  },
  computed: {
    filteredGroups() {
      const variants = buildSearchVariants(this.searchQuery);
      let list = this.groups;
      if (variants.length) {
        list = list.filter(g => matchesSearch(`${g.name} ${g.description || ''} ${g.id}`, variants));
      }
      return this.sortList(list);
    },
    emptyText() {
      return this.searchQuery.trim() ? 'Ничего не найдено по запросу' : 'Групп прав пока нет';
    },
    isMetaModalDirty() {
      if (!this.showMetaModal) return false;
      return this.metaForm.name.trim() !== '' || this.metaForm.description.trim() !== '';
    },
    isDetailsDirty() {
      const s = this.selectedGroup;
      if (!s) return false;
      return s.name.trim() !== this.original.name
        || (s.description || '').trim() !== this.original.description;
    },
    isDirty() {
      return this.isMetaModalDirty || this.isDetailsDirty;
    },
  },
  created() {
    this.overlay.close = () => { this.requestCloseMeta(); };
  },
  mounted() {
    this.refresh();
    this.loadCatalog();
    this._stopGuard = registerDirtyTracker({
      isDirty: () => this.isDirty,
      getChanges: () => {
        if (this.isMetaModalDirty) {
          return [`${this.modalMode === 'copy' ? 'Копия группы' : 'Новая группа'}: "${this.metaForm.name.trim()}"`];
        }
        if (this.isDetailsDirty) {
          const s = this.selectedGroup;
          const ch = [];
          if (s.name.trim() !== this.original.name) {
            ch.push({ label: 'Наименование', from: this.original.name, to: s.name.trim() });
          }
          if ((s.description || '').trim() !== this.original.description) {
            ch.push({ label: 'Описание', from: this.original.description || '—', to: (s.description || '').trim() || '—' });
          }
          return ch;
        }
        return [];
      },
      save: async () => {
        if (this.isMetaModalDirty) await this.submitMeta();
        if (this.isDetailsDirty) await this.saveSelected();
      },
    });
    document.addEventListener('keydown', this.onKeydown);
  },
  beforeUnmount() {
    this._stopGuard?.();
    document.removeEventListener('keydown', this.onKeydown);
  },
  methods: {
    onKeydown(e) {
      if (e.key === 'Escape' && this.showMetaModal) this.requestCloseMeta();
    },
    async loadCatalog() {
      try {
        const json = await getPermissionCatalog();
        this.catalog = Array.isArray(json) ? json : [];
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'каталог прав', type: 'error' });
      }
    },
    sortList(list) {
      const arr = [...list];
      if (!this.sortField) {
        return arr.sort((a, b) => a.name.localeCompare(b.name));
      }
      return arr.sort((a, b) => {
        let r;
        if (this.sortField === 'id') r = a.id - b.id;
        else if (this.sortField === 'keys') r = (a.keys || []).length - (b.keys || []).length;
        else r = a.name.localeCompare(b.name);
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
    syncSelectedFrom(fresh) {
      this.selectedGroup = { id: fresh.id, name: fresh.name, description: fresh.description || '' };
      this.original = { name: fresh.name, description: fresh.description || '' };
      this.selectedKeys = [...(fresh.keys || [])];
    },
    async refresh() {
      this.isLoading = true;
      try {
        const json = await listPermissionGroups();
        this.groups = Array.isArray(json) ? json : [];
        if (this.selectedGroup) {
          const fresh = this.groups.find(g => g.id === this.selectedGroup.id);
          if (fresh && !this.isDetailsDirty) {
            this.syncSelectedFrom(fresh);
          } else if (!fresh) {
            this.selectedGroup = null;
          }
        }
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'группы прав', type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },
    async selectGroup(group) {
      if (this.selectedGroup && this.selectedGroup.id === group.id) return;
      if (this.isDetailsDirty && !(await confirmIfAnyDirty())) return;
      this.syncSelectedFrom(group);
      this.detailError = '';
    },
    /** Полная замена группы (name/description/keys) через единый PUT. */
    async persist(keys) {
      const s = this.selectedGroup;
      const res = await updatePermissionGroup(s.id, {
        name: s.name.trim(),
        description: (s.description || '').trim() || null,
        keys,
      });
      if (res && res.message && res.updated === undefined) throw new Error(res.message);
    },
    async saveSelected() {
      if (!this.isDetailsDirty || this.isSaving) return;
      const s = this.selectedGroup;
      const name = s.name.trim();
      if (!name) {
        this.detailError = 'Введите название группы';
        return;
      }
      this.isSaving = true;
      this.detailError = '';
      try {
        await this.persist(this.selectedKeys);
        this.original = { name, description: (s.description || '').trim() };
        useDeletionsStore().notify({ prefix: 'Изменения сохранены в ', bold: name });
        await this.refresh();
      } catch (e) {
        this.detailError = e?.message || 'Не удалось сохранить';
      } finally {
        this.isSaving = false;
      }
    },
    openPermissions() {
      this.permsModal = {
        show: true,
        title: `Права группы «${this.selectedGroup.name.trim() || this.original.name}»`,
        initialKeys: [...this.selectedKeys],
      };
    },
    closePermissions() {
      this.permsModal = { show: false, title: '', initialKeys: [] };
    },
    async handleSavePermissions(keys) {
      if (this.isSavingPerms) return;
      this.isSavingPerms = true;
      try {
        await this.persist(keys);
        this.selectedKeys = keys;
        this.original = { name: this.selectedGroup.name.trim(), description: (this.selectedGroup.description || '').trim() };
        this.closePermissions();
        useDeletionsStore().notify({ prefix: 'Права группы ', bold: this.original.name, suffix: ' сохранены' });
        await this.refresh();
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось сохранить права ', bold: this.original.name, type: 'error' });
        this.detailError = e?.message || 'Не удалось сохранить права';
      } finally {
        this.isSavingPerms = false;
      }
    },
    openCreate() {
      this.modalMode = 'create';
      this.metaForm = { name: '', description: '' };
      this.copySource = null;
      this.copySourceKeys = [];
      this.metaError = '';
      this.showMetaModal = true;
    },
    openCopy(group) {
      this.modalMode = 'copy';
      this.copySource = { id: group.id, name: this.original.name || group.name };
      this.copySourceKeys = [...this.selectedKeys];
      this.metaForm = {
        name: `Копия: ${this.original.name || group.name}`,
        description: (this.selectedGroup?.description || '').trim(),
      };
      this.metaError = '';
      this.showMetaModal = true;
    },
    async requestCloseMeta() {
      if (this.isMetaModalDirty && !(await confirmIfAnyDirty())) return;
      this.forceCloseMeta();
    },
    forceCloseMeta() {
      this.showMetaModal = false;
      this.metaForm = { name: '', description: '' };
      this.copySource = null;
      this.copySourceKeys = [];
      this.metaError = '';
    },
    async submitMeta() {
      const name = this.metaForm.name.trim();
      if (!name || this.isSubmitting) return;
      this.isSubmitting = true;
      this.metaError = '';
      try {
        // Ключи копии берём свежими из источника (List мог устареть).
        let keys = [];
        if (this.modalMode === 'copy' && this.copySource) {
          const src = await getPermissionGroup(this.copySource.id);
          keys = Array.isArray(src?.keys) ? src.keys : this.copySourceKeys;
        }
        const created = await createPermissionGroup({
          name,
          description: this.metaForm.description.trim() || null,
          keys,
        });
        if (!created || !created.id) {
          throw new Error(created?.message || 'Не удалось создать группу');
        }
        useDeletionsStore().notify({
          prefix: 'Группа ',
          bold: name,
          suffix: this.modalMode === 'copy' ? ' создана как копия' : ' создана',
        });
        const newId = created.id;
        const wasCreate = this.modalMode === 'create';
        this.forceCloseMeta();
        await this.refresh();
        const fresh = this.groups.find(g => g.id === newId);
        if (fresh) {
          this.syncSelectedFrom(fresh);
          // Новую пустую группу сразу открываем на настройку прав.
          if (wasCreate) this.openPermissions();
        }
      } catch (e) {
        this.metaError = e?.message || 'Не удалось создать группу';
      } finally {
        this.isSubmitting = false;
      }
    },
    async performDelete() {
      const target = this.deleteConfirm;
      this.deleteConfirm = null;
      if (!target) return;
      try {
        const res = await deletePermissionGroup(target.id);
        if (res && res.message && res.deleted === undefined) throw new Error(res.message);
        useDeletionsStore().notify({ prefix: 'Группа ', bold: target.name, suffix: ' удалена' });
        if (this.selectedGroup && this.selectedGroup.id === target.id) this.selectedGroup = null;
        await this.refresh();
      } catch (e) {
        useDeletionsStore().notify({
          prefix: 'Не удалось удалить группу: ',
          bold: e?.message || 'попробуйте позже',
          type: 'error',
        });
      }
    },
  },
};
</script>

<style scoped>
.groups-container {
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

.management-title {
  margin: 0;
  font-size: 1.2em;
  font-weight: 600;
  color: var(--text);
}

.header-controls {
  display: flex;
  gap: 10px;
  align-items: center;
}

.add-header-button {
  padding: 8px 16px;
  background: var(--accent);
  color: var(--accent-contrast);
  border: none;
  border-radius: 50px;
  cursor: pointer;
  font-size: 0.9em;
  transition: background-color 0.2s ease;
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.add-header-button:hover {
  background: var(--accent-hover);
}

/* Master-detail layout (эталон TableConstructor) */
.content-container {
  display: flex;
  height: 500px;
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

.table-container {
  background: var(--surface);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.table-header {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  height: 43px;
  align-items: center;
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

.header-col:hover {
  color: var(--text);
}

.header-col:hover .sort-icon {
  color: var(--text);
}

.sort-icon {
  color: var(--text-muted);
  width: 12px;
  height: 12px;
  transition: 0.2s;
}

.sort-icon.sorted {
  color: var(--text);
}

.sort-icon.desc {
  transform: rotate(180deg);
}

.active-sort {
  color: var(--text) !important;
  font-weight: 600 !important;
}

.id-col {
  width: 18%;
  min-width: 50px;
}

.name-col {
  width: 62%;
  min-width: 160px;
}

.keys-col {
  width: 20%;
  min-width: 70px;
  justify-content: flex-start;
}

.table-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.table-row {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  align-items: center;
  transition: background-color 0.2s ease;
  cursor: pointer;
  height: 42px;
  font-size: 14px;
}

.table-row:hover {
  background-color: var(--surface-2);
}

.table-row.selected {
  background-color: var(--accent-tint);
}

.table-row:last-child {
  border-bottom: none;
}

.table-col {
  padding: 0 8px;
}

.cell-content {
  display: block;
  padding: 4px 0;
}

.id-value {
  font-weight: 600;
  color: var(--text);
}

.truncate-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
  display: block;
}

.keys-count {
  font-size: 13px;
  color: var(--text-muted);
  font-weight: 500;
}

.no-results {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-muted);
  width: 100%;
}

.groups-loading {
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

.items-count {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
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

.copy-btn {
  background: var(--surface);
  color: var(--accent-text);
  border: 1px solid var(--accent);
}

.copy-btn:hover {
  background: var(--accent-tint);
}

.delete-btn {
  background: var(--surface);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.delete-btn:hover {
  background: var(--danger-bg);
  border-color: var(--danger);
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
  margin-top: 4px;
}

.field-hint {
  font-size: 0.8em;
  color: var(--text-muted);
  line-height: 1.4;
}

.details-body .lk-input {
  max-width: 360px;
}

.details-body .lk-input:disabled {
  background: var(--accent-tint);
  color: var(--text-muted);
}

.details-body .lk-textarea {
  max-width: 360px;
  resize: vertical;
}

.perms-block {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 8px;
}

.perms-summary {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 14px;
  background: var(--accent-tint);
  border-radius: 15px;
  border: 1px solid var(--border);
}

.perms-count {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
}

.perms-edit-btn {
  margin-left: auto;
}

.details-actions {
  display: flex;
  gap: 10px;
  margin-top: 8px;
}

.details-meta {
  display: flex;
  gap: 16px;
  margin-top: 12px;
  font-size: 12px;
  color: var(--text-muted);
}

.form-error {
  color: var(--danger-text);
  font-size: 0.85em;
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

/* Модалка создания / копирования */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

.group-modal {
  width: 100%;
  max-width: 460px;
  background: var(--surface);
  border-radius: 30px;
  box-shadow: 0 10px 30px var(--shadow-drop);
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 18px 24px;
  border-bottom: 1px solid var(--border);
}

.modal-header h3 {
  margin: 0;
  font-size: 1.1em;
  font-weight: 600;
  color: var(--text);
}

.modal-close {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  line-height: 1;
  color: var(--text-muted);
  background: none;
  border: none;
  cursor: pointer;
  border-radius: 50%;
  transition: all 0.2s;
}

.modal-close:hover {
  color: var(--text);
  background: var(--surface-2);
}

.modal-body {
  padding: 22px 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 0.85em;
  color: var(--text-muted);
  font-weight: 500;
}

.copy-hint {
  margin: 0;
  font-size: 0.8em;
  color: var(--accent-text);
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 24px;
  border-top: 1px solid var(--border);
}

/* Анимация открытия/закрытия */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: all 0.25s ease;
}

.modal-fade-enter-active .group-modal,
.modal-fade-leave-active .group-modal {
  transition: all 0.25s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  background: transparent;
}

.modal-fade-enter-from .group-modal,
.modal-fade-leave-to .group-modal {
  opacity: 0;
  transform: translateY(20px);
}

@media (max-width: 767.98px) {
  /* Направление/высоту шапки берёт на себя глобальный .rt-header-inline
     (responsive-tables.css, !important - перебивает scoped-специфичность). */
  .management-header {
    padding: 10px var(--gutter, 16px);
  }
  .header-controls {
    flex-wrap: wrap;
    row-gap: 8px;
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
  .details-body .lk-input,
  .details-body .lk-textarea {
    max-width: 100%;
  }
}
</style>
