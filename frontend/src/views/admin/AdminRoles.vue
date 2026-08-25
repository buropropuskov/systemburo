<template>
  <AdminPageShell>
    <div
      class="roles-container dashboard-card"
      data-testid="ob-admin-roles"
    >
      <div class="management-header rt-header-inline">
        <h3 class="management-title">
          Роли пользователей
        </h3>
        <div class="header-controls">
          <SearchComponent
            v-model="searchQuery"
            :title="'Поиск ролей...'"
          />
          <button
            class="add-header-button rt-btn-compact"
            data-testid="role-add-btn"
            aria-label="Создать роль"
            @click="openCreate"
          >
            <span
              class="rt-btn-icon"
              aria-hidden="true"
            >+</span>
            <span class="rt-btn-label">Создать роль</span>
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
          :class="{ 'with-details': selectedRole }"
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
              <div class="header-col groups-col">
                <p>Группы</p>
              </div>
            </div>

            <div class="table-body">
              <div
                v-for="role in filteredRoles"
                :key="role.id"
                class="table-row rt-row"
                data-testid="role-row"
                :class="{ selected: selectedRole && selectedRole.id === role.id }"
                @click="selectRole(role)"
              >
                <div
                  class="table-col id-col"
                  data-label="ID"
                >
                  <span class="cell-content id-value">{{ role.id }}</span>
                </div>
                <div
                  class="table-col name-col"
                  data-label="Наименование"
                >
                  <span
                    class="truncate-text"
                    :title="role.name"
                  >
                    {{ role.name }}
                    <code class="role-code">{{ role.code }}</code>
                  </span>
                </div>
                <div
                  class="table-col groups-col"
                  data-label="Группы"
                >
                  <span class="groups-count">{{ (role.default_groups || []).length }}</span>
                </div>
              </div>

              <div
                v-if="!filteredRoles.length && !isLoading"
                class="no-results"
              >
                {{ emptyText }}
              </div>
              <div
                v-if="isLoading && !roles.length"
                class="roles-loading"
              >
                <LoaderSpinner label="Загрузка ролей..." />
              </div>
            </div>

            <div class="table-footer">
              <span class="items-count">
                Всего: {{ filteredRoles.length }}
              </span>
            </div>
          </div>
        </div>

        <div
          v-if="selectedRole"
          class="details-section"
          data-testid="role-details"
        >
          <div class="tab-content">
            <div class="details-header">
              <div class="details-title-wrapper">
                <h3 class="details-title">
                  {{ original.name }}
                </h3>
                <code class="role-code details-code">{{ selectedRole.code }}</code>
              </div>
              <div class="details-header-actions">
                <button
                  class="action-btn copy-btn"
                  data-testid="role-copy"
                  @click="openCopy(selectedRole)"
                >
                  Создать копию
                </button>
                <button
                  v-permission-scope="'permission.audit.manage'"
                  class="action-btn delete-btn"
                  data-testid="role-delete"
                  @click="deleteConfirm = { id: selectedRole.id, name: original.name }"
                >
                  Удалить
                </button>
              </div>
            </div>

            <div class="details-body">
              <label class="field-label">Наименование</label>
              <input
                v-model.trim="selectedRole.name"
                type="text"
                class="lk-input"
                maxlength="100"
                placeholder="Название роли"
                :disabled="isSaving"
                data-testid="role-detail-name"
                @keyup.enter="saveSelected"
              >

              <label class="field-label">Код</label>
              <input
                :value="selectedRole.code"
                type="text"
                class="lk-input"
                disabled
                data-testid="role-detail-code"
              >
              <span class="field-hint">Код задаётся при создании и не меняется.</span>

              <label class="field-label">Описание</label>
              <textarea
                v-model.trim="selectedRole.description"
                class="lk-textarea"
                rows="2"
                maxlength="500"
                placeholder="Для чего эта роль"
                :disabled="isSaving"
                data-testid="role-detail-description"
              />

              <div class="perms-block">
                <label class="field-label">Права роли</label>
                <span class="field-hint">
                  Собственные точечные права роли и дефолтные группы. Права из групп добавляются к точечным.
                </span>
                <button
                  type="button"
                  class="manage-perms-btn"
                  data-testid="role-perms-btn"
                  @click="openPermsModal"
                >
                  Настроить права
                  <span class="manage-perms-btn__meta">
                    {{ currentDirectKeys.length }} точечных · {{ currentGroupIds.length }} групп
                  </span>
                </button>
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
                  data-testid="role-save"
                  @click="saveSelected"
                >
                  {{ isSaving ? 'Сохранение...' : 'Сохранить' }}
                </button>
              </div>

              <div class="details-meta">
                <span>ID: {{ selectedRole.id }}</span>
              </div>
            </div>
          </div>
        </div>
        <div
          v-else
          class="no-selection-message"
        >
          <p>Выберите роль для просмотра и редактирования</p>
        </div>
      </div>

      <!-- Модалка создания / копирования роли -->
      <Teleport to="body">
        <transition name="modal-fade">
          <div
            v-if="showMetaModal"
            class="modal-overlay"
            data-testid="role-modal"
            @mousedown="onOverlayMousedown"
            @mouseup="onOverlayMouseup"
          >
            <div
              class="role-modal"
              @mousedown.stop
            >
              <div class="modal-header">
                <h3>{{ modalMode === 'copy' ? `Копия роли «${copySource?.name}»` : 'Новая роль' }}</h3>
                <button
                  class="modal-close"
                  aria-label="Закрыть"
                  data-testid="role-modal-close"
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
                    placeholder="Например, Арендатор"
                    maxlength="100"
                    class="lk-input"
                    data-testid="role-input-name"
                    @keyup.enter="submitMeta"
                  >
                </div>

                <div class="form-group">
                  <label class="form-label">Код (латиницей, неизменный)</label>
                  <input
                    v-model.trim="metaForm.code"
                    type="text"
                    placeholder="tenant"
                    maxlength="50"
                    class="lk-input"
                    data-testid="role-input-code"
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
                    placeholder="Для чего эта роль"
                    data-testid="role-input-description"
                  />
                </div>

                <p
                  v-if="modalMode === 'copy' && copySourceGroupIds.length"
                  class="copy-hint"
                >
                  Будут скопированы дефолтные группы: {{ copySourceGroupIds.length }}.
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
                  data-testid="role-modal-cancel"
                  @click="requestCloseMeta"
                >
                  Отмена
                </button>
                <button
                  class="lk-button lk-button--primary"
                  :disabled="!metaForm.name || !metaForm.code || isSubmitting"
                  data-testid="role-modal-save"
                  @click="submitMeta"
                >
                  {{ modalMode === 'copy' ? 'Создать копию' : 'Создать' }}
                </button>
              </div>
            </div>
          </div>
        </transition>
      </Teleport>

      <RolePermissionsModal
        :show="permsModal.show"
        :role-name="original.name"
        :catalog="catalog"
        :groups="allGroups"
        :initial-direct-keys="currentDirectKeys"
        :initial-group-ids="currentGroupIds"
        :saving="permsSaving"
        @close="permsModal.show = false"
        @save="handleSavePerms"
      />

      <ConfirmationModal
        :show="!!deleteConfirm"
        title="Удаление роли"
        :message="deleteConfirm ? `Удалить роль «${deleteConfirm.name}»? Если у неё есть пользователи, удаление будет отклонено.` : ''"
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
import RolePermissionsModal from '@/components/admin/RolePermissionsModal.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { registerDirtyTracker, confirmIfAnyDirty } from '@/utils/dirtyTracker';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import {
  listRoles,
  createRole,
  updateRole,
  deleteRole,
  setRoleDefaultGroups,
  setRolePermissions,
  listPermissionGroups,
  getPermissionCatalog,
} from '@/api/permissions';
import AppIcon from '@/components/icons/AppIcon.vue';

export default {
  name: 'AdminRoles',
  components: { AdminPageShell, SearchComponent, RefreshButton, ConfirmationModal, LoaderSpinner, RolePermissionsModal, AppIcon },
  setup() {
    // Колбэк закрытия модалки присваивается в created - нужен доступ к this с проверкой dirty.
    const overlay = { close: () => {} };
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => overlay.close());
    return { onOverlayMousedown, onOverlayMouseup, overlay };
  },
  data() {
    return {
      roles: [],
      allGroups: [],
      catalog: [],
      searchQuery: '',
      sortField: null,
      sortDirection: 'asc',
      isLoading: false,
      selectedRole: null,
      original: { name: '', description: '' },
      currentGroupIds: [],
      currentDirectKeys: [],
      permsModal: { show: false },
      permsSaving: false,
      detailError: '',
      isSaving: false,
      showMetaModal: false,
      modalMode: 'create',
      metaForm: { name: '', code: '', description: '' },
      metaError: '',
      isSubmitting: false,
      copySource: null,
      copySourceGroupIds: [],
      deleteConfirm: null,
    };
  },
  computed: {
    filteredRoles() {
      const variants = buildSearchVariants(this.searchQuery);
      let list = this.roles;
      if (variants.length) {
        list = list.filter(r => matchesSearch(`${r.name} ${r.code || ''} ${r.id}`, variants));
      }
      return this.sortList(list);
    },
    emptyText() {
      return this.searchQuery.trim() ? 'Ничего не найдено по запросу' : 'Ролей пока нет';
    },
    isMetaModalDirty() {
      if (!this.showMetaModal) return false;
      return this.metaForm.name.trim() !== ''
        || this.metaForm.code.trim() !== ''
        || this.metaForm.description.trim() !== '';
    },
    isDetailsDirty() {
      const s = this.selectedRole;
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
    this._stopGuard = registerDirtyTracker({
      isDirty: () => this.isDirty,
      getChanges: () => {
        if (this.isMetaModalDirty) {
          return [`${this.modalMode === 'copy' ? 'Копия роли' : 'Новая роль'}: "${this.metaForm.name.trim()}"`];
        }
        if (this.isDetailsDirty) {
          const s = this.selectedRole;
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
    sortList(list) {
      const arr = [...list];
      if (!this.sortField) {
        return arr.sort((a, b) => a.name.localeCompare(b.name));
      }
      return arr.sort((a, b) => {
        if (this.sortField === 'id') {
          return this.sortDirection === 'asc' ? a.id - b.id : b.id - a.id;
        }
        const r = a.name.localeCompare(b.name);
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
      this.selectedRole = { id: fresh.id, code: fresh.code, name: fresh.name, description: fresh.description || '' };
      this.original = { name: fresh.name, description: fresh.description || '' };
      this.currentGroupIds = (fresh.default_groups || []).map(g => g.id);
      this.currentDirectKeys = [...(fresh.direct_grants || [])];
    },
    async refresh() {
      this.isLoading = true;
      try {
        const [roles, groups, catalog] = await Promise.all([
          listRoles(),
          listPermissionGroups(),
          getPermissionCatalog(),
        ]);
        this.roles = Array.isArray(roles) ? roles : [];
        this.allGroups = Array.isArray(groups) ? groups : [];
        this.catalog = Array.isArray(catalog) ? catalog : [];
        if (this.selectedRole) {
          const fresh = this.roles.find(r => r.id === this.selectedRole.id);
          if (fresh && !this.isDetailsDirty) {
            this.syncSelectedFrom(fresh);
          } else if (!fresh) {
            this.selectedRole = null;
          }
        }
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'роли', type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },
    async selectRole(role) {
      if (this.selectedRole && this.selectedRole.id === role.id) return;
      if (this.isDetailsDirty && !(await confirmIfAnyDirty())) return;
      this.syncSelectedFrom(role);
      this.detailError = '';
    },
    async saveSelected() {
      if (!this.isDetailsDirty || this.isSaving) return;
      const s = this.selectedRole;
      const name = s.name.trim();
      if (!name) {
        this.detailError = 'Введите название роли';
        return;
      }
      this.isSaving = true;
      this.detailError = '';
      try {
        const res = await updateRole(s.id, { name, description: (s.description || '').trim() || null });
        if (res && res.message && res.updated === undefined) throw new Error(res.message);
        useDeletionsStore().notify({ prefix: 'Изменения сохранены в ', bold: name });
        await this.refresh();
      } catch (e) {
        this.detailError = e?.message || 'Не удалось сохранить';
      } finally {
        this.isSaving = false;
      }
    },
    openPermsModal() {
      if (!this.selectedRole) return;
      this.permsModal.show = true;
    },
    sameKeys(a, b) {
      if (a.length !== b.length) return false;
      const setB = new Set(b);
      return a.every(k => setB.has(k));
    },
    async handleSavePerms({ directKeys, groupIds }) {
      const s = this.selectedRole;
      if (!s || this.permsSaving) return;
      const groupsChanged = !this.sameKeys(groupIds, this.currentGroupIds);
      const grantsChanged = !this.sameKeys(directKeys, this.currentDirectKeys);
      if (!groupsChanged && !grantsChanged) {
        this.permsModal.show = false;
        return;
      }
      this.permsSaving = true;
      try {
        if (groupsChanged) {
          const res = await setRoleDefaultGroups(s.id, groupIds);
          if (res && res.message && res.updated === undefined) throw new Error(res.message);
        }
        if (grantsChanged) {
          const res = await setRolePermissions(s.id, directKeys);
          if (res && res.message && res.updated === undefined) throw new Error(res.message);
        }
        useDeletionsStore().notify({ prefix: 'Права роли ', bold: this.original.name, suffix: ' обновлены' });
        this.permsModal.show = false;
        await this.refresh();
      } catch (e) {
        useDeletionsStore().notify({
          prefix: 'Не удалось сохранить права: ',
          bold: e?.message || 'ошибка',
          type: 'error',
        });
        // Первый вызов (группы) мог пройти, второй (гранты) упасть -- перечитываем,
        // чтобы currentGroupIds/currentDirectKeys и счётчики не остались устаревшими.
        await this.refresh();
      } finally {
        this.permsSaving = false;
      }
    },
    openCreate() {
      this.modalMode = 'create';
      this.metaForm = { name: '', code: '', description: '' };
      this.copySource = null;
      this.copySourceGroupIds = [];
      this.metaError = '';
      this.showMetaModal = true;
    },
    openCopy(role) {
      this.modalMode = 'copy';
      this.copySource = { id: role.id, name: this.original.name || role.name };
      this.copySourceGroupIds = [...this.currentGroupIds];
      this.metaForm = {
        name: `Копия: ${this.original.name || role.name}`,
        code: `${role.code}_copy`,
        description: (this.selectedRole?.description || '').trim(),
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
      this.metaForm = { name: '', code: '', description: '' };
      this.copySource = null;
      this.copySourceGroupIds = [];
      this.metaError = '';
    },
    async submitMeta() {
      const name = this.metaForm.name.trim();
      const code = this.metaForm.code.trim();
      if (!name || !code || this.isSubmitting) return;
      this.isSubmitting = true;
      this.metaError = '';
      try {
        const created = await createRole({
          name,
          code,
          description: this.metaForm.description.trim() || null,
        });
        if (!created || !created.id) {
          throw new Error(created?.message || 'Не удалось создать роль');
        }
        if (this.modalMode === 'copy' && this.copySourceGroupIds.length) {
          const res = await setRoleDefaultGroups(created.id, this.copySourceGroupIds);
          if (res && res.message && res.updated === undefined) throw new Error(res.message);
        }
        useDeletionsStore().notify({
          prefix: 'Роль ',
          bold: name,
          suffix: this.modalMode === 'copy' ? ' создана как копия' : ' создана',
        });
        const newId = created.id;
        this.forceCloseMeta();
        await this.refresh();
        const fresh = this.roles.find(r => r.id === newId);
        if (fresh) this.syncSelectedFrom(fresh);
      } catch (e) {
        this.metaError = e?.message || 'Не удалось создать роль';
      } finally {
        this.isSubmitting = false;
      }
    },
    async performDelete() {
      const target = this.deleteConfirm;
      this.deleteConfirm = null;
      if (!target) return;
      try {
        const res = await deleteRole(target.id);
        if (res && res.message && res.deleted === undefined) throw new Error(res.message);
        useDeletionsStore().notify({ prefix: 'Роль ', bold: target.name, suffix: ' удалена' });
        if (this.selectedRole && this.selectedRole.id === target.id) this.selectedRole = null;
        await this.refresh();
      } catch (e) {
        useDeletionsStore().notify({
          prefix: 'Не удалось удалить роль: ',
          bold: e?.message || 'возможно, есть привязанные пользователи',
          type: 'error',
        });
      }
    },
  },
};
</script>

<style scoped>
.roles-container {
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

.groups-col {
  width: 20%;
  min-width: 70px;
  cursor: default;
  justify-content: flex-start;
}

.groups-col:hover {
  color: var(--text-muted);
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

.role-code {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.75em;
  color: var(--text-muted);
  background: var(--accent-tint);
  padding: 1px 6px;
  border-radius: 6px;
  margin-left: 6px;
  vertical-align: middle;
}

.groups-count {
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

.roles-loading {
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

.details-code {
  margin-left: 0;
  font-size: 0.8em;
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

.manage-perms-btn {
  align-self: flex-start;
  margin-top: 4px;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 9px 18px;
  background: var(--surface);
  color: var(--accent-text);
  border: 1px solid var(--accent);
  border-radius: 30px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  transition: background 0.2s ease;
}

.manage-perms-btn:hover {
  background: var(--accent-tint);
}

.manage-perms-btn__meta {
  font-size: 11px;
  color: var(--text-muted);
  font-weight: 400;
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

.role-modal {
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

.modal-fade-enter-active .role-modal,
.modal-fade-leave-active .role-modal {
  transition: all 0.25s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  background: transparent;
}

.modal-fade-enter-from .role-modal,
.modal-fade-leave-to .role-modal {
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
