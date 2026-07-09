<template>
  <div class="companies-management dashboard-card">
    <div class="management-header">
      <h3 class="management-title">
        Управление компаниями
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
        <BaseDropdown
          class="type-filter-dropdown"
          data-testid="companies-type-filter"
          :model-value="typeFilter"
          :options="typeFilterOptions"
          label-key="label"
          value-key="value"
          @update:model-value="typeFilter = $event"
        />
        <SearchComponent
          v-model="searchQuery"
          :title="'Поиск компаний...'"
        />
        <button
          class="add-header-button"
          data-testid="companies-add-btn"
          @click="openAddModal"
        >
          Добавить
        </button>
        <RefreshButton
          :loading="isLoading"
          @refresh="refreshData"
        />
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - таблица компаний -->
      <div class="table-section">
        <div class="table-container">
          <div class="table-header">
            <div
              class="header-col id-col"
              @click="sortBy('id')"
            >
              <p :class="{ 'active-sort': sortField === 'id' }">
                ID
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'id',
                  'desc': sortField === 'id' && sortDirection === 'desc'
                }"
              >
            </div>
            <div
              class="header-col name-col"
              @click="sortBy('name')"
            >
              <p :class="{ 'active-sort': sortField === 'name' }">
                Наименование
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'name',
                  'desc': sortField === 'name' && sortDirection === 'desc'
                }"
              >
            </div>
            <div
              class="header-col type-col"
              @click="sortBy('type')"
            >
              <p :class="{ 'active-sort': sortField === 'type' }">
                Тип
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'type',
                  'desc': sortField === 'type' && sortDirection === 'desc'
                }"
              >
            </div>
            <div
              class="header-col users-col"
              @click="sortBy('user_count')"
            >
              <p :class="{ 'active-sort': sortField === 'user_count' }">
                Пользователи
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'user_count',
                  'desc': sortField === 'user_count' && sortDirection === 'desc'
                }"
              >
            </div>
          </div>

          <div class="table-body">
            <div
              v-for="comp in sortedCompanies"
              :key="comp.id"
              class="table-row"
              data-testid="companies-row"
              :class="{
                'selected': selectedCompany && selectedCompany.id === comp.id,
                'inactive': !comp.is_active
              }"
              @click="selectCompany(comp)"
            >
              <div class="table-col id-col">
                <span class="cell-content id-value">{{ comp.id }}</span>
              </div>
              <div class="table-col name-col">
                <span
                  class="truncate-text"
                  :title="comp.name"
                >
                  {{ comp.name }}
                  <span
                    v-if="!comp.is_active"
                    class="inactive-badge"
                  >(архив)</span>
                </span>
              </div>
              <div class="table-col type-col">
                <span
                  class="truncate-text type-value"
                  :class="{ 'type-unspecified': !comp.type }"
                  :title="orgTypeLabel(comp.type)"
                >{{ orgTypeLabel(comp.type) }}</span>
              </div>
              <div class="table-col users-col">
                <span class="cell-content user-count">
                  <span class="count-value">{{ comp.user_count }}</span>
                </span>
              </div>
            </div>

            <div
              v-if="!sortedCompanies.length && !isLoading"
              class="no-results-inline"
            >
              {{ emptyText }}
            </div>
            <div
              v-if="isLoading && !companiesWithUsers.length"
              class="companies-loading"
            >
              <LoaderSpinner label="Загрузка компаний..." />
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">
              {{ showArchive ? 'В архиве' : 'Всего компаний' }}: {{ sortedCompanies.length }}
            </span>
          </div>
        </div>
      </div>

      <!-- Средняя часть - детали компании -->
      <div
        v-if="selectedCompany"
        class="details-section"
        data-testid="companies-details"
      >
        <div class="details-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">
                {{ originalSelectedName }}
              </h3>
            </div>
            <div class="details-header-actions">
              <span
                v-if="!selectedCompany.is_active"
                class="archive-badge"
              >В архиве</span>
              <button
                class="action-btn history-btn"
                data-testid="companies-history"
                @click="openHistory(selectedCompany)"
              >
                История
              </button>
              <button
                v-if="selectedCompany.is_active"
                class="action-btn archive-action-btn"
                data-testid="companies-archive"
                @click="onArchiveClick(selectedCompany)"
              >
                В архив
              </button>
              <button
                v-else
                class="action-btn restore-btn"
                data-testid="companies-restore"
                @click="onRestore(selectedCompany)"
              >
                Восстановить
              </button>
            </div>
          </div>

          <div class="details-body">
            <label class="field-label">Наименование</label>
            <div class="name-edit-row">
              <input
                v-model.trim="selectedCompany.name"
                type="text"
                class="lk-input"
                maxlength="100"
                placeholder="Введите название компании"
                autocomplete="off"
                :disabled="!selectedCompany.is_active || isSavingName"
                data-testid="companies-detail-name"
                @keyup.enter="saveSelectedName"
              >
              <button
                v-if="selectedCompany.is_active"
                class="lk-button lk-button--primary"
                :disabled="!isDetailsDirty || isSavingName"
                data-testid="companies-save-name"
                @click="saveSelectedName"
              >
                Сохранить
              </button>
            </div>
            <label class="field-label">Тип</label>
            <BaseDropdown
              data-testid="companies-detail-type"
              :model-value="selectedCompany.type"
              :options="typeDetailOptions"
              label-key="label"
              value-key="value"
              :placeholder="unspecifiedTypeLabel"
              :disabled="!selectedCompany.is_active || isSavingName"
              @update:model-value="onDetailTypeChange"
            />

            <div
              v-if="detailError"
              class="form-error"
            >
              {{ detailError }}
            </div>

            <template v-if="selectedCompany.is_active">
              <SelectUnloadPlaces
                :entity="selectedCompany"
                :entity-type="'company'"
                @places-updated="handlePlacesUpdated"
              />

              <SelectTables
                :entity="selectedCompany"
                :entity-type="'company'"
                @tables-updated="handleTablesUpdated"
              />
            </template>
            <p
              v-else
              class="archive-hint"
            >
              Восстановите компанию, чтобы редактировать места разгрузки, таблицы и ответственных.
            </p>

            <div
              class="members-section"
              data-testid="companies-members"
            >
              <div class="members-title">
                Пользователи, привязанные к компании «{{ originalSelectedName }}»: {{ members.length }}
              </div>
              <div
                v-if="membersLoading"
                class="members-loading"
              >
                <LoaderSpinner label="Загрузка пользователей..." />
              </div>
              <ul
                v-else-if="members.length"
                class="members-list"
              >
                <li
                  v-for="m in members"
                  :key="m.id"
                  class="members-item"
                >
                  <span class="members-name">{{ memberFullName(m) }}</span>
                  <span
                    v-if="m.position"
                    class="members-position"
                  >{{ m.position }}</span>
                </li>
              </ul>
              <p
                v-else
                class="members-empty"
              >
                Нет привязанных пользователей
              </p>
            </div>
          </div>
        </div>
      </div>

      <!-- Правая часть - ответственные лица -->
      <div
        class="responsible-section"
        :class="{'with-details': selectedCompany}"
      >
        <div
          v-if="selectedCompany && selectedCompany.is_active"
          class="responsible-content"
        >
          <ResponsibleUsersSection
            :entity="selectedCompany"
            :entity-type="'company'"
            @users-updated="handleUsersUpdated"
          />
        </div>
        <div
          v-else
          class="no-selection-message"
        >
          <p v-if="!selectedCompany">
            Выберите компанию для просмотра
          </p>
          <p v-else>
            Ответственные доступны после восстановления
          </p>
        </div>
      </div>
    </div>

    <!-- Модальное окно добавления -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showAddModal"
          class="modal-overlay"
          data-testid="companies-modal"
          @mousedown="onOverlayMousedown"
          @mouseup="onOverlayMouseup"
        >
          <div
            class="companies-modal"
            @mousedown.stop
          >
            <div class="modal-header">
              <h3>Новая компания</h3>
              <button
                class="modal-close"
                aria-label="Закрыть"
                data-testid="companies-modal-close"
                @click="requestCloseAdd"
              >
                ×
              </button>
            </div>

            <div class="modal-body">
              <div class="form-group">
                <label class="form-label">Название компании</label>
                <input
                  ref="nameInput"
                  v-model.trim="addForm.name"
                  type="text"
                  placeholder="Введите название компании"
                  maxlength="100"
                  class="lk-input"
                  data-testid="companies-input-name"
                  @keyup.enter="submitAdd"
                >
              </div>
              <div class="form-group">
                <label class="form-label">Тип</label>
                <BaseDropdown
                  data-testid="companies-input-type"
                  :model-value="addForm.type"
                  :options="typeCreateOptions"
                  label-key="label"
                  value-key="value"
                  placeholder="Выберите тип"
                  @update:model-value="addForm.type = $event"
                />
              </div>
              <div
                v-if="addError"
                class="form-error"
              >
                {{ addError }}
              </div>
            </div>

            <div class="modal-footer">
              <button
                class="lk-button lk-button--ghost"
                data-testid="companies-modal-cancel"
                @click="requestCloseAdd"
              >
                Отмена
              </button>
              <button
                class="lk-button lk-button--primary"
                :disabled="!addForm.name || !addForm.type || isAdding"
                data-testid="companies-modal-save"
                @click="submitAdd"
              >
                Создать
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <!-- Модальное окно подтверждения архивации -->
    <ConfirmationModal
      :show="!!archiveConfirmComp"
      title="Архивация компании"
      :message="archiveConfirmComp ? `Архивировать компанию «${archiveConfirmComp.name}»? Её можно будет восстановить из архива.` : ''"
      confirm-text="В архив"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
      @confirm="performArchive"
      @cancel="archiveConfirmComp = null"
    />

    <CompanyHistoryModal
      v-if="historyComp"
      :company="historyComp"
      :current-user-name="currentUserName"
      @close="historyComp = null"
    />
  </div>
</template>

<script>
import { mapState, mapActions } from 'pinia';
import { apiRequest } from '@/api/client';
import { getCompanyMembers } from '@/api/organizations';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import {
  ORG_TYPE_CREATE_OPTIONS,
  ORG_TYPE_DETAIL_OPTIONS,
  ORG_TYPE_FILTER_OPTIONS,
  ORG_TYPE_FILTER_ALL,
  ORG_TYPE_FILTER_UNSPECIFIED,
  ORG_TYPE_UNSPECIFIED_LABEL,
} from '@/constants/orgTypes';
import { useCompaniesStore } from '@/stores/companies';
import { useDeletionsStore } from '@/stores/deletions';
import { registerDirtyTracker, confirmIfAnyDirty } from '@/utils/dirtyTracker';
import { useOverlayClose } from '@/composables/useOverlayClose';
import RefreshButton from './RefreshButton.vue';
import SearchComponent from './SearchComponent.vue';
import ResponsibleUsersSection from './ResponsibleUsersSection.vue';
import SelectUnloadPlaces from './SelectUnloadPlaces.vue';
import SelectTables from './SelectTables.vue';
import ConfirmationModal from './ConfirmationModal.vue';
import BaseDropdown from './ui/BaseDropdown.vue';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import CompanyHistoryModal from './CompanyHistoryModal.vue';

export default {
  name: 'CompaniesManagement',
  components: {
    SearchComponent,
    RefreshButton,
    ResponsibleUsersSection,
    SelectUnloadPlaces,
    SelectTables,
    ConfirmationModal,
    BaseDropdown,
    LoaderSpinner,
    CompanyHistoryModal,
  },
  setup() {
    // Колбэк закрытия модалки присваивается в created - нужен доступ к this с проверкой dirty.
    const overlay = { close: () => {} };
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => overlay.close());
    return { onOverlayMousedown, onOverlayMouseup, overlay };
  },
  data() {
    return {
      searchQuery: '',
      showArchive: false,
      typeFilter: ORG_TYPE_FILTER_ALL,
      showAddModal: false,
      addForm: { name: '', type: null },
      addError: '',
      isAdding: false,
      selectedCompany: null,
      originalSelectedName: '',
      originalSelectedType: null,
      members: [],
      membersLoading: false,
      membersSeq: 0,
      detailError: '',
      isSavingName: false,
      archiveConfirmComp: null,
      historyComp: null,
      currentUserName: '',
      sortField: null,
      sortDirection: 'asc',
      archiveOptions: [
        { label: 'Активные', value: 'active' },
        { label: 'Архив', value: 'archive' },
      ],
      typeCreateOptions: ORG_TYPE_CREATE_OPTIONS,
      typeDetailOptions: ORG_TYPE_DETAIL_OPTIONS,
      typeFilterOptions: ORG_TYPE_FILTER_OPTIONS,
      unspecifiedTypeLabel: ORG_TYPE_UNSPECIFIED_LABEL,
    };
  },
  computed: {
    ...mapState(useCompaniesStore, {
      companiesWithUsers: 'itemsWithUsers',
      isLoading: 'isLoading',
    }),
    filteredCompanies() {
      let list = this.companiesWithUsers.filter(comp =>
        this.showArchive ? !comp.is_active : comp.is_active
      );
      if (this.typeFilter === ORG_TYPE_FILTER_UNSPECIFIED) {
        list = list.filter(comp => !comp.type);
      } else if (this.typeFilter !== ORG_TYPE_FILTER_ALL) {
        list = list.filter(comp => comp.type === this.typeFilter);
      }
      const variants = buildSearchVariants(this.searchQuery);
      if (!variants.length) return list;
      return list.filter(comp => matchesSearch(`${comp.name} ${comp.id}`, variants));
    },
    sortedCompanies() {
      const companies = [...this.filteredCompanies];

      if (!this.sortField) {
        return companies.sort((a, b) => a.name.localeCompare(b.name));
      }

      return companies.sort((a, b) => {
        let valueA, valueB;

        switch (this.sortField) {
          case 'id':
            valueA = a.id;
            valueB = b.id;
            break;
          case 'name':
            valueA = a.name;
            valueB = b.name;
            break;
          case 'type':
            // «не указан» (null) -> пустая строка: сортируется первой при asc.
            valueA = a.type || '';
            valueB = b.type || '';
            break;
          case 'user_count':
            valueA = a.user_count;
            valueB = b.user_count;
            break;
          default:
            return 0;
        }

        if (valueA < valueB) {
          return this.sortDirection === 'asc' ? -1 : 1;
        }
        if (valueA > valueB) {
          return this.sortDirection === 'asc' ? 1 : -1;
        }
        return 0;
      });
    },
    emptyText() {
      if (this.searchQuery.trim()) return 'Ничего не найдено по запросу';
      if (this.typeFilter !== ORG_TYPE_FILTER_ALL) return 'Нет компаний с таким типом';
      return this.showArchive ? 'В архиве пусто' : 'Компаний пока нет';
    },
    isAddDirty() {
      return this.showAddModal && (this.addForm.name.trim() !== '' || !!this.addForm.type);
    },
    isDetailsDirty() {
      if (!this.selectedCompany || !this.selectedCompany.is_active) return false;
      const nameChanged = this.selectedCompany.name.trim() !== this.originalSelectedName;
      const typeChanged = (this.selectedCompany.type ?? null) !== (this.originalSelectedType ?? null);
      return nameChanged || typeChanged;
    },
    isDirty() {
      return this.isAddDirty || this.isDetailsDirty;
    },
  },
  watch: {
    showAddModal(newVal) {
      if (newVal) {
        this.$nextTick(() => {
          this.$refs.nameInput?.focus();
        });
      }
    }
  },
  created() {
    this.overlay.close = () => { this.requestCloseAdd(); };
  },
  mounted() {
    this.refreshData();
    this.fetchCurrentUser();
    this._stopGuard = registerDirtyTracker({
      isDirty: () => this.isDirty,
      getChanges: () => {
        if (this.isAddDirty) return [`Новая компания: "${this.addForm.name.trim()}"`];
        if (this.isDetailsDirty) {
          const changes = [];
          if (this.selectedCompany.name.trim() !== this.originalSelectedName) {
            changes.push({ label: 'Наименование', from: this.originalSelectedName, to: this.selectedCompany.name.trim() });
          }
          if ((this.selectedCompany.type ?? null) !== (this.originalSelectedType ?? null)) {
            changes.push({ label: 'Тип', from: this.orgTypeLabel(this.originalSelectedType), to: this.orgTypeLabel(this.selectedCompany.type) });
          }
          return changes;
        }
        return [];
      },
      save: async () => {
        if (this.isAddDirty) await this.submitAdd();
        if (this.isDetailsDirty) await this.saveSelectedName();
      },
    });
    document.addEventListener('keydown', this.onKeydown);
  },
  beforeUnmount() {
    this._stopGuard?.();
    document.removeEventListener('keydown', this.onKeydown);
  },
  methods: {
    ...mapActions(useCompaniesStore, [
      'refresh',
      'createCompany',
      'updateCompany',
      'deleteCompany',
      'restoreCompany',
      'fetchCompaniesWithUsers',
    ]),

    onKeydown(e) {
      if (e.key === 'Escape' && this.showAddModal) this.requestCloseAdd();
    },

    async refreshData() {
      // Тянем и архивные тоже - переключение режима фильтрует на клиенте без рефетча.
      await this.refresh(true);
      this.syncSelected();
    },

    syncSelected() {
      if (!this.selectedCompany) return;
      const fresh = this.companiesWithUsers.find(c => c.id === this.selectedCompany.id);
      const visible = fresh && (this.showArchive ? !fresh.is_active : fresh.is_active);
      if (fresh && visible && !this.isDetailsDirty) {
        this.selectedCompany = { ...fresh };
        this.originalSelectedName = fresh.name;
        this.originalSelectedType = fresh.type ?? null;
      } else if (!visible) {
        this.selectedCompany = null;
        this.members = [];
      }
    },

    async onArchiveModeChange(value) {
      if (this.isDetailsDirty && !(await confirmIfAnyDirty())) return;
      this.showArchive = value === 'archive';
      this.selectedCompany = null;
      this.members = [];
      this.detailError = '';
    },

    openAddModal() {
      this.showAddModal = true;
      this.addForm.name = '';
      this.addForm.type = null;
      this.addError = '';
    },

    async requestCloseAdd() {
      if (this.isAddDirty && !(await confirmIfAnyDirty())) return;
      this.forceCloseAdd();
    },

    forceCloseAdd() {
      this.showAddModal = false;
      this.addForm.name = '';
      this.addForm.type = null;
      this.addError = '';
    },

    async submitAdd() {
      const name = this.addForm.name.trim();
      const type = this.addForm.type;
      if (!name || !type || this.isAdding) return;
      this.isAdding = true;
      this.addError = '';

      const result = await this.createCompany({ name, type }, { includeArchived: true });

      if (result.ok) {
        this.forceCloseAdd();
        if (this.showArchive) this.showArchive = false;
        const created = this.companiesWithUsers.find(comp => comp.id === result.data.id);
        if (created) {
          this.selectedCompany = { ...created };
          this.originalSelectedName = created.name;
          this.originalSelectedType = created.type ?? null;
          this.loadMembers(created.id);
        }
        useDeletionsStore().notify({ prefix: 'Компания ', bold: name, suffix: ' создана' });
      } else {
        this.addError = result.message || 'Не удалось создать компанию';
      }
      this.isAdding = false;
    },

    async selectCompany(comp) {
      if (this.selectedCompany && this.selectedCompany.id === comp.id) return;
      if (this.isDetailsDirty && !(await confirmIfAnyDirty())) return;
      this.selectedCompany = { ...comp };
      this.originalSelectedName = comp.name;
      this.originalSelectedType = comp.type ?? null;
      this.detailError = '';
      this.loadMembers(comp.id);
    },

    onDetailTypeChange(value) {
      if (this.selectedCompany) this.selectedCompany.type = value;
    },

    async saveSelectedName() {
      if (!this.isDetailsDirty || this.isSavingName) return;
      const name = this.selectedCompany.name.trim();
      const type = this.selectedCompany.type ?? null;
      this.isSavingName = true;
      this.detailError = '';

      const result = await this.updateCompany(this.selectedCompany.id, { name, type }, { includeArchived: true });

      if (result.ok) {
        this.originalSelectedName = name;
        this.originalSelectedType = type;
        this.selectedCompany.name = name;
        useDeletionsStore().notify({ prefix: 'Изменения сохранены в ', bold: name });
      } else {
        this.detailError = result.message || 'Не удалось сохранить';
      }
      this.isSavingName = false;
    },

    async loadMembers(id) {
      // seq-токен: быстрое переключение компаний может дать гонку, показываем
      // только ответ последнего запроса (см. урок про авто-fetch по выбору).
      const seq = ++this.membersSeq;
      this.members = [];
      this.membersLoading = true;
      try {
        const data = await getCompanyMembers(id);
        if (seq !== this.membersSeq) return;
        this.members = Array.isArray(data) ? data : [];
      } catch {
        // Список участников - вспомогательная информация, при сбое оставляем пустым.
        if (seq === this.membersSeq) this.members = [];
      } finally {
        if (seq === this.membersSeq) this.membersLoading = false;
      }
    },

    memberFullName(m) {
      const parts = [m.last_name, m.first_name, m.middle_name].filter(Boolean);
      return parts.join(' ') || m.username || '—';
    },

    orgTypeLabel(type) {
      return type || this.unspecifiedTypeLabel;
    },

    onArchiveClick(comp) {
      this.archiveConfirmComp = comp;
    },

    async performArchive() {
      const comp = this.archiveConfirmComp;
      this.archiveConfirmComp = null;
      if (!comp) return;

      const result = await this.deleteCompany(comp.id, { includeArchived: true });

      if (result.ok) {
        if (this.selectedCompany && this.selectedCompany.id === comp.id && !this.showArchive) {
          this.selectedCompany = null;
        }
        useDeletionsStore().notify({ prefix: 'Компания ', bold: comp.name, suffix: ' архивирована' });
      } else {
        useDeletionsStore().notify({ prefix: 'Не удалось архивировать: ', bold: result.message || 'ошибка', type: 'error' });
      }
    },

    async onRestore(comp) {
      const result = await this.restoreCompany(comp.id, { includeArchived: true });

      if (result.ok) {
        if (this.selectedCompany && this.selectedCompany.id === comp.id && this.showArchive) {
          this.selectedCompany = null;
        }
        useDeletionsStore().notify({ prefix: 'Компания ', bold: comp.name, suffix: ' восстановлена из архива' });
      } else {
        useDeletionsStore().notify({ prefix: 'Не удалось восстановить: ', bold: result.message || 'ошибка', type: 'error' });
      }
    },

    openHistory(comp) {
      this.historyComp = comp;
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

    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'asc';
      }
    },

    // Всегда тянем с архивными: режим фильтруется на клиенте, рефетча при смене не нужно.
    handleUsersUpdated() {
      this.fetchCompaniesWithUsers(true);
    },

    handlePlacesUpdated() {
      this.fetchCompaniesWithUsers(true);
    },

    handleTablesUpdated() {
      this.fetchCompaniesWithUsers(true);
    },
  }
};
</script>

<style scoped>
.companies-management {
  background: #fff;
  border-radius: 16px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
}

.management-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid #e6e6e6;
  height: 50px;
}

.management-title {
  font-size: 1.2em;
  margin: 0;
  font-weight: 600;
  color: #000;
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.archive-dropdown {
  min-width: 130px;
}

.add-header-button {
  padding: 8px 16px;
  background: #4F5BDF;
  color: white;
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
  background: #3a45b2;
}

.content-container {
  display: flex;
  height: 450px;
  width: 100%;
}

.table-section {
  width: 40%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #e6e6e6;
}

.table-container {
  background: #fff;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.table-header {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid #e6e6e6;
  background: #fff;
  height: 43px;
  align-items: center;
}

.header-col {
  padding: 0 8px;
  font-size: 14px;
  color: #a2a2a2;
  font-weight: 600;
  text-align: left;
  display: flex;
  align-items: center;
  gap: 5px;
  transition: .2s;
  cursor: pointer;
  user-select: none;
}

.header-col:hover {
  color: #333;
}

.header-col:hover .sort-icon {
  filter: brightness(0);
}

.sort-icon {
  width: 12px;
  height: 12px;
  transition: .2s;
}

.sort-icon.sorted {
  filter: brightness(0);
}

.sort-icon.desc {
  transform: rotate(180deg);
}

.active-sort {
  color: #333 !important;
  font-weight: 600 !important;
}

.id-col {
  width: 12%;
  min-width: 36px;
}

.name-col {
  width: 38%;
  min-width: 140px;
}

.type-col {
  width: 26%;
  min-width: 90px;
}

.users-col {
  width: 24%;
  min-width: 90px;
}

.type-value {
  font-size: 13px;
  color: #333;
}

.type-unspecified {
  color: #a2a2a2;
  font-style: italic;
}

.table-row.inactive .type-value {
  color: #6b7280;
}

.table-body {
  flex: 1;
  overflow-y: auto;
}

.table-row {
  display: flex;
  padding: 0 20px;
  border-bottom: 1px solid #f0f0f0;
  align-items: center;
  transition: background-color 0.2s ease;
  cursor: pointer;
  height: 42px;
  font-size: 14px;
}

.table-row:hover {
  background-color: #fafafa;
}

.table-row.selected {
  background-color: #f0f2ff;
}

.table-row.inactive {
  background: #fafafa;
  color: #6b7280;
}

.table-row.inactive .id-value,
.table-row.inactive .count-value {
  color: #6b7280;
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
  color: #000;
}

.user-count {
  display: flex;
  align-items: center;
  gap: 6px;
}

.count-value {
  font-weight: 600;
  color: #000;
}

.truncate-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
  display: block;
}

.inactive-badge {
  margin-left: 6px;
  font-size: 0.75em;
  color: #a2a2a2;
  font-style: italic;
}

.no-results-inline {
  text-align: center;
  padding: 40px 20px;
  color: #a2a2a2;
  width: 100%;
}

.companies-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 0;
}

.table-footer {
  margin-top: auto;
  padding: 6px 20px;
  border-top: 1px solid #e6e6e6;
  text-align: end;
  background: #f8fafc;
}

.items-count {
  font-size: 12px;
  color: #a2a2a2;
  font-weight: 500;
}

.details-section {
  width: fit-content;
  padding: 15px;
  overflow-y: auto;
  background: #fafafa;
  border-right: 1px solid #e6e6e6;
}

.details-content {
  height: 100%;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 10px;
  gap: 12px;
}

.details-title-wrapper {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.details-title {
  margin: 0;
  color: #000;
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

.archive-badge {
  background: #6b7280;
  color: #fff;
  padding: 4px 10px;
  border-radius: 50px;
  font-size: 0.75em;
  font-weight: 500;
  white-space: nowrap;
}

.action-btn {
  padding: 8px 16px;
  border: none;
  border-radius: 30px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  transition: background 0.2s, border-color 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
  white-space: nowrap;
}

.history-btn {
  background: #fff;
  color: #4F5BDF;
  border: 1px solid #4F5BDF;
}

.history-btn:hover {
  background: #eef0ff;
}

.archive-action-btn {
  background: #fff;
  color: #dc3545;
  border: 1px solid #fecaca;
}

.archive-action-btn:hover {
  background: #fff1f2;
  border-color: #dc3545;
}

.restore-btn {
  background: #10b981;
  color: #fff;
}

.restore-btn:hover {
  background: #0da271;
}

.details-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-bottom: 15px;
}

.field-label {
  font-size: 0.85em;
  color: #666;
  font-weight: 500;
}

.name-edit-row {
  display: flex;
  gap: 10px;
  align-items: center;
}

.name-edit-row .lk-input {
  flex: 1;
}

.form-error {
  color: #d73a3a;
  font-size: 0.85em;
}

.archive-hint {
  margin: 8px 0 0;
  color: #a2a2a2;
  font-size: 0.85em;
  line-height: 1.5;
}

.members-section {
  margin-top: 6px;
  padding-top: 12px;
  border-top: 1px solid #e6e6e6;
  min-width: 260px;
}

.members-title {
  font-size: 0.85em;
  font-weight: 600;
  color: #333;
  margin-bottom: 8px;
}

.members-loading {
  display: flex;
  justify-content: center;
  padding: 12px 0;
}

.members-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 180px;
  overflow-y: auto;
}

.members-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 6px 10px;
  background: #fff;
  border: 1px solid #eee;
  border-radius: 10px;
}

.members-name {
  font-size: 13px;
  color: #000;
  font-weight: 500;
}

.members-position {
  font-size: 12px;
  color: #a2a2a2;
}

.members-empty {
  margin: 0;
  font-size: 0.85em;
  color: #a2a2a2;
}

.responsible-section {
  flex: 1;
  background: #fff;
  display: flex;
  flex-direction: column;
}

.responsible-content {
  padding: 10px;
  height: 100%;
}

.no-selection-message {
  color: #a2a2a2;
  font-weight: 400;
  font-size: 14px;
  text-align: center;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 0 16px;
}

/* Модалка создания (эталон: radius 30px, lk-* классы) */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}

.companies-modal {
  width: 100%;
  max-width: 440px;
  background: #fff;
  border-radius: 30px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 18px 24px;
  border-bottom: 1px solid #e6e6e6;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.1em;
  font-weight: 600;
  color: #000;
}

.modal-close {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  line-height: 1;
  color: #999;
  background: none;
  border: none;
  cursor: pointer;
  border-radius: 50%;
  transition: all 0.2s;
}

.modal-close:hover {
  color: #333;
  background: #f5f5f5;
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
  color: #666;
  font-weight: 500;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 24px;
  border-top: 1px solid #e6e6e6;
}

/* Анимация открытия/закрытия */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: all 0.25s ease;
}

.modal-fade-enter-active .companies-modal,
.modal-fade-leave-active .companies-modal {
  transition: all 0.25s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  background: rgba(0, 0, 0, 0);
}

.modal-fade-enter-from .companies-modal,
.modal-fade-leave-to .companies-modal {
  opacity: 0;
  transform: translateY(20px);
}

@media (max-width: 968px) {
  .content-container {
    flex-direction: column;
    height: auto;
  }

  .table-section,
  .details-section,
  .responsible-section {
    width: 100% !important;
  }

  .table-section {
    border-right: none;
    border-bottom: 1px solid #e6e6e6;
    height: 255px;
  }
}

@media (max-width: 768px) {
  .management-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
    height: auto;
    padding: 16px;
  }

  .header-controls {
    width: 100%;
    flex-direction: column;
    align-items: stretch;
  }

  .add-header-button {
    justify-content: center;
  }

  .table-header,
  .table-row {
    padding: 0 16px;
  }

  .id-col {
    width: 14%;
  }

  .name-col {
    width: 34%;
  }

  .type-col {
    width: 28%;
  }

  .users-col {
    width: 24%;
  }

  .details-section,
  .responsible-content {
    padding: 16px;
  }

  .details-header {
    flex-direction: column;
    gap: 12px;
    align-items: flex-start;
  }

  .details-header-actions {
    align-self: flex-end;
  }
}
</style>
