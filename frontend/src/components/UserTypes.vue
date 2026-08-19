<template>
  <div class="user-types-container dashboard-card">
    <div class="management-header rt-header-inline">
      <h3 class="management-title">
        Типы пользователей
      </h3>
      <div class="header-controls">
        <SearchComponent
          v-model="searchQuery"
          :title="'Поиск типов...'"
        />
        <button
          class="add-header-button rt-btn-compact"
          aria-label="Создать тип"
          @click="showAddModal = true"
        >
          <span
            class="rt-btn-icon"
            aria-hidden="true"
          >+</span>
          <span class="rt-btn-label">Создать тип</span>
        </button>
        <RefreshButton
          :loading="refreshing"
          @refresh="refreshData"
        />
      </div>
    </div>

    <div class="content-container">
      <!-- Левая часть - список типов -->
      <div
        class="types-section"
        :class="{'with-details': selectedType}"
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
                :class="{
                  'sorted': sortField === 'id',
                  'desc': sortField === 'id' && sortDirection === 'desc'
                }"
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
                :class="{
                  'sorted': sortField === 'name',
                  'desc': sortField === 'name' && sortDirection === 'desc'
                }"
              />
            </div>
            <div
              class="header-col users-col"
              @click="sortBy('users_count')"
            >
              <p :class="{ 'active-sort': sortField === 'users_count' }">
                Пользователи
              </p>
              <AppIcon
                name="sort"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'users_count',
                  'desc': sortField === 'users_count' && sortDirection === 'desc'
                }"
              />
            </div>
          </div>

          <div class="table-body">
            <div
              v-for="type in sortedTypes"
              :key="type.id"
              class="table-row rt-row"
              :class="{'selected': selectedType && selectedType.id === type.id}"
              :data-testid="'utype-row-' + type.id"
              @click="selectType(type)"
            >
              <div
                class="table-col id-col"
                data-label="ID"
              >
                <span class="cell-content id-value">{{ type.id }}</span>
              </div>
              <div
                class="table-col name-col"
                data-label="Наименование"
              >
                <span
                  class="truncate-text"
                  :title="type.name"
                >
                  {{ type.name }}
                  <span
                    v-if="type.is_system"
                    class="system-badge"
                  >системный</span>
                </span>
              </div>
              <div
                class="table-col users-col"
                data-label="Пользователи"
              >
                <span class="users-count">{{ type.users_count }}</span>
              </div>
            </div>
            <div
              v-if="!sortedTypes.length"
              class="no-results"
            >
              {{ emptyText }}
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">Всего типов: {{ filteredTypes.length }}</span>
          </div>
        </div>
      </div>

      <!-- Правая часть - детали типа -->
      <div
        v-if="selectedType"
        class="details-section"
      >
        <div class="details-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">
                {{ selectedType.name }}
              </h3>
              <div class="type-info-row">
                <span class="system-name">{{ selectedType.code }}</span>
                <span class="users-count-badge">Пользователей: {{ selectedType.users_count }}</span>
              </div>
            </div>
            <div class="details-header-actions">
              <button
                class="lk-button lk-button--secondary"
                @click="openHistory(selectedType)"
              >
                История
              </button>
              <span
                v-if="selectedType.is_system"
                class="system-badge"
              >системный</span>
              <button
                v-else
                class="delete-icon-btn"
                title="Удалить тип"
                @click="confirmDeleteType(selectedType)"
              >
                <AppIcon
                  name="delete"
                  class="delete-icon"
                />
              </button>
            </div>
          </div>
          
          <div class="details-body">
            <div class="compact-form">
              <div class="form-column">
                <div class="form-group compact">
                  <label class="detail-label">Наименование типа:</label>
                  <input
                    v-model="selectedType.name"
                    class="lk-input"
                    placeholder="Название типа"
                    autocomplete="off"
                    :disabled="selectedType.is_system"
                    :title="selectedType.is_system ? 'Системный тип нельзя переименовать' : ''"
                    @change="updateTypeName"
                  >
                </div>
                <div class="form-group compact">
                  <label class="detail-label">Системное имя:</label>
                  <input
                    v-model="selectedType.code"
                    class="lk-input"
                    disabled
                    placeholder="Системное имя"
                  >
                </div>
              </div>
            </div>

            <!-- Пользователи типа: блокеры удаления (#1379) -->
            <div class="type-users">
              <div class="type-users__title">
                Пользователи типа
                <span class="count-badge">{{ typeUsers.length }}</span>
              </div>
              <div
                v-if="!selectedType.is_system && typeUsers.length"
                class="blocking-notice"
                data-testid="utype-blocking-notice"
              >
                <span class="blocking-notice__text">
                  Пока эти пользователи привязаны к типу, его нельзя удалить.
                </span>
                <button
                  v-if="canReassign"
                  type="button"
                  class="lk-button lk-button--secondary blocking-notice__btn"
                  data-testid="utype-reassign-open"
                  @click="openReassign"
                >
                  Перенести всех в другой тип
                </button>
              </div>
              <div
                v-if="typeUsersLoading"
                class="type-users__loading"
              >
                Загрузка пользователей...
              </div>
              <div
                v-else-if="typeUsers.length"
                class="type-users__list"
              >
                <div
                  v-for="u in typeUsers"
                  :key="u.id"
                  class="type-user"
                  :class="{ 'type-user--archived': !u.is_active }"
                >
                  <div class="type-user__who">
                    <b>{{ userFullName(u) }}</b>
                    <small v-if="u.position">{{ u.position }}</small>
                  </div>
                  <span
                    v-if="!u.is_active"
                    class="archived-badge"
                  >архив</span>
                </div>
              </div>
              <p
                v-else
                class="type-users__empty"
              >
                Нет пользователей этого типа
              </p>
            </div>
          </div>
        </div>
      </div>
      
      <div
        v-else
        class="no-selection-message"
      >
        <p>Выберите тип пользователя для просмотра и редактирования</p>
      </div>
    </div>


    <!-- Модальное окно создания типа -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showAddModal"
          class="modal-overlay"
          @mousedown="onOverlayMousedown"
          @mouseup="onOverlayMouseup"
        >
          <div
            class="modal-content"
            @mousedown.stop
          >
            <div class="modal-header">
              <h3 class="modal-title">
                Создать новый тип пользователя
              </h3>
              <button
                class="modal-close"
                @click="closeModal"
              >
                <svg
                  width="10"
                  height="10"
                  viewBox="0 0 14 14"
                  fill="none"
                >
                  <path
                    d="M13 1L1 13M1 1L13 13"
                    stroke="#666"
                    stroke-width="2"
                    stroke-linecap="round"
                  />
                </svg>
              </button>
            </div>
          
            <div class="modal-body">
              <div class="input-group">
                <label class="input-label">Наименование типа *</label>
                <input
                  ref="nameInput"
                  v-model="newType.name"
                  placeholder="Менеджер"
                  class="modal-input"
                  @keyup.enter="createType"
                >
                <div class="input-hint">
                  Обязательное поле
                </div>
              </div>
            
              <div class="input-group">
                <label class="input-label">Системное имя *</label>
                <input
                  v-model="newType.code"
                  placeholder="manager"
                  class="modal-input"
                  @input="validateSystemName"
                  @keyup.enter="createType"
                >
                <div class="input-hint">
                  Латинские буквы, цифры и подчеркивания
                </div>
                <span
                  v-if="nameError"
                  class="form-error"
                >{{ nameError }}</span>
              </div>
            </div>
          
            <div class="modal-footer">
              <button
                class="lk-button lk-button--ghost"
                @click="closeModal"
              >
                Отмена
              </button>
              <button
                class="lk-button lk-button--primary"
                :disabled="!isFormValid"
                @click="createType"
              >
                Создать
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <!-- Модальное окно подтверждения удаления -->
    <ConfirmationModal
      :show="showDeleteModal"
      title="Подтверждение удаления"
      :message="deleteMessage"
      confirm-text="Удалить"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#ff4444', borderColor: '#ff4444' }"
      @confirm="deleteType"
      @cancel="cancelDelete"
    />

    <UserTypeHistoryModal
      v-if="historyForType"
      :user-type="historyForType"
      :current-user-name="currentUserName"
      @close="historyForType = null"
    />

    <!-- Перенос всех пользователей в другой тип (#1379) -->
    <BaseModal
      :show="reassignVisible"
      title="Перенести всех пользователей"
      width="460px"
      radius="30px"
      @close="closeReassign"
    >
      <div
        class="reassign-body"
        data-testid="utype-reassign-modal"
      >
        <p class="reassign-intro">
          Пользователи типа «{{ reassignSourceName }}» ({{ typeUsers.length }})
          будут перенесены в выбранный тип. После этого исходный тип можно будет удалить.
        </p>
        <label class="field-label">Целевой тип</label>
        <BaseDropdown
          :model-value="reassignTargetId"
          :options="reassignTargetOptions"
          label-key="label"
          value-key="value"
          :searchable="true"
          :teleport="true"
          placeholder="Выберите тип"
          data-testid="utype-reassign-target"
          @update:model-value="reassignTargetId = $event"
        />
        <p
          v-if="!reassignTargetOptions.length"
          class="reassign-empty"
        >
          Нет других типов для переноса.
        </p>
      </div>
      <template #actions>
        <button
          type="button"
          class="lk-button lk-button--ghost"
          data-testid="utype-reassign-cancel"
          @click="closeReassign"
        >
          Отмена
        </button>
        <button
          type="button"
          class="lk-button lk-button--primary"
          :disabled="!reassignTargetId || reassignSubmitting"
          data-testid="utype-reassign-submit"
          @click="performReassign"
        >
          Перенести
        </button>
      </template>
    </BaseModal>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants'
import { getUserTypeBlockingUsers, reassignUserTypeUsers } from '@/api/user-types';
import RefreshButton from './RefreshButton.vue';
import SearchComponent from './SearchComponent.vue';
import ConfirmationModal from './ConfirmationModal.vue';
import UserTypeHistoryModal from './UserTypeHistoryModal.vue';
import BaseModal from './ui/BaseModal.vue';
import BaseDropdown from './ui/BaseDropdown.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { registerDirtyTracker } from '@/utils/dirtyTracker';
import AppIcon from '@/components/icons/AppIcon.vue';

export default {
  components: {
    SearchComponent,
    RefreshButton,
    ConfirmationModal,
    UserTypeHistoryModal,
    BaseModal,
    BaseDropdown,
    AppIcon,
  },
  setup() {
    // Holder: useOverlayClose требует колбэк в setup, а closeModal - метод
    // (сбрасывает форму). Привязываем метод в mounted через holder.
    const overlayCloser = { fn: () => {} };
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => overlayCloser.fn());
    return { onOverlayMousedown, onOverlayMouseup, overlayCloser };
  },
  data() {
    return {
      searchQuery: '',
      refreshing: false,
      newType: {
        name: '',
        code: ''
      },
      types: [],
      showAddModal: false,
      showDeleteModal: false,
      selectedType: null,
      typeToDelete: null,
      sortField: null,
      sortDirection: 'asc',
      nameError: '',
      isLoading: false,
      historyForType: null,
      currentUserName: '',
      // Блокеры удаления типа (#1379): ВСЕ пользователи типа, вкл. архивных.
      typeUsers: [],
      typeUsersLoading: false,
      typeUsersSeq: 0,
      reassignVisible: false,
      reassignTargetId: null,
      reassignSubmitting: false,
      reassignSourceName: ''
    };
  },
  computed: {
    // Гейт кнопки «Перенести» зеркалит BE: reassign-эндпоинт закрыт тем же
    // page.admin.directories, что открывает экран (#1982).
    canReassign() {
      return usePermissionsStore().hasPermission('page.admin.directories');
    },
    // Цели переноса - все типы, кроме источника. Системные НЕ исключаем: перенос
    // в дефолтный (системный) тип допустим, BE это принимает. Источник-системный
    // сюда не попадёт: у него блок и кнопка не рисуются (систему не удаляют).
    reassignTargetOptions() {
      if (!this.selectedType) return [];
      const srcId = this.selectedType.id;
      return this.types
        .filter(t => t.id !== srcId)
        .map(t => ({ label: t.name, value: t.id }));
    },
    emptyText() {
      return this.searchQuery.trim() ? 'Ничего не найдено по запросу' : 'Типов пока нет';
    },
    filteredTypes() {
      const variants = buildSearchVariants(this.searchQuery);
      if (!variants.length) return this.types;
      return this.types.filter(type => matchesSearch(
        `${type.name} ${type.code} ${type.id}`,
        variants,
      ));
    },
    sortedTypes() {
      const types = [...this.filteredTypes];
      
      if (!this.sortField) {
        return types.sort((a, b) => a.name.localeCompare(b.name));
      }
      
      return types.sort((a, b) => {
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
          case 'users_count':
            valueA = a.users_count;
            valueB = b.users_count;
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
    deleteMessage() {
      return `Вы точно хотите удалить тип пользователя "${this.typeToDelete?.name}"?`;
    },
    isFormValid() {
      return this.newType.name.trim() && 
             this.newType.code.trim() && 
             !this.nameError &&
             !this.isLoading;
    }
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
  mounted() {
    this.refreshData();
    this.fetchCurrentUser();
    this.overlayCloser.fn = () => this.closeModal();
    document.addEventListener('keydown', this.onKeydown);
    this._stopDirty = registerDirtyTracker({
      isDirty: () => this.showAddModal && Boolean(this.newType.name.trim() || this.newType.code.trim()),
      getChanges: () => [`Новый тип: ${this.newType.name || this.newType.code}`],
      save: async () => {
        // Бросаем, если форма невалидна: иначе DirtyConfirmModal посчитает
        // сохранение успешным и уведёт со страницы, потеряв ввод.
        if (!this.isFormValid) throw new Error('Форма типа заполнена некорректно');
        await this.createType();
      },
    });
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKeydown);
    this._stopDirty?.();
  },
  methods: {
    onKeydown(e) {
      if (e.key === 'Escape' && this.showAddModal) this.closeModal();
    },
    validateSystemName() {
      const nameRegex = /^[a-z0-9_]*$/;
      if (!nameRegex.test(this.newType.code)) {
        this.nameError = "Только латинские буквы, цифры и подчеркивания";
      } else {
        this.nameError = '';
      }
    },
    async refreshData() {
      this.refreshing = true;
      try {
        await this.fetchTypes();
      } finally {
        this.refreshing = false;
      }
    },
    async fetchTypes() {
      try {
        const response = await apiRequest("/user-types-management", {
        });
        if (response.ok) {
          const data = await response.json();
          this.types = data;
        }
      } catch (error) {
        console.error("Error fetching user types:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'типы пользователей', type: 'error' });
      }
    },
    async createType() {
      if (!this.isFormValid) {
        useDeletionsStore().notify({ prefix: 'Не удалось создать: ', bold: 'заполните поля корректно', type: 'error' });
        return;
      }

      if (this.isLoading) return;

      this.isLoading = true;
      const createdName = this.newType.name;

      try {
        const response = await apiRequest("/user-types-management", {
          method: "POST",
          body: JSON.stringify(this.newType),
        });

        if (response.ok) {
          this.newType = {
            name: '',
            code: ''
          };
          this.showAddModal = false;
          await this.refreshData();
          useDeletionsStore().notify({ prefix: 'Тип ', bold: createdName, suffix: ' создан' });
        } else {
          const err = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось создать: ', bold: err.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error("Error creating user type:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось создать: ', bold: 'нет связи с сервером', type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },
    async updateTypeName() {
      if (this.selectedType) {
        await this.updateType(this.selectedType);
      }
    },
    async updateType(type) {
      try {
        const response = await apiRequest(`/user-types-management/${type.id}`, {
          method: "PUT",
          body: JSON.stringify({
            name: type.name,
            code: type.code
          }),
        });

        if (response.ok) {
          useDeletionsStore().notify({ prefix: 'Изменения сохранены в ', bold: type.name });
          await this.refreshData();
        } else {
          const err = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось сохранить: ', bold: err.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error("Error updating user type:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось сохранить: ', bold: 'нет связи с сервером', type: 'error' });
      }
    },
    confirmDeleteType(type) {
      if (type.is_system) {
        useDeletionsStore().notify({ prefix: 'Нельзя удалить ', bold: type.name, suffix: ' - системный тип', type: 'error' });
        return;
      }
      if (type.users_count > 0) {
        useDeletionsStore().notify({ prefix: 'Нельзя удалить тип ', bold: type.name, suffix: ': есть привязанные пользователи', type: 'error' });
        return;
      }

      this.typeToDelete = type;
      this.showDeleteModal = true;
    },
    
    cancelDelete() {
      this.showDeleteModal = false;
      this.typeToDelete = null;
    },

    async deleteType() {
      if (!this.typeToDelete) return;
      
      try {
        const response = await apiRequest(`/user-types-management/${this.typeToDelete.id}`, {
          method: "DELETE",
        });
        
        if (response.ok) {
          const deletedName = this.typeToDelete.name;
          this.selectedType = null;
          this.showDeleteModal = false;
          this.typeToDelete = null;
          await this.refreshData();
          useDeletionsStore().notify({ prefix: 'Тип ', bold: deletedName, suffix: ' удалён' });
        } else {
          const error = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось удалить: ', bold: error.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error("Error deleting user type:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось удалить: ', bold: 'нет связи с сервером', type: 'error' });
      }
    },
    selectType(type) {
      this.selectedType = JSON.parse(JSON.stringify(type));
      this.loadTypeUsers(type.id);
    },
    async loadTypeUsers(typeId) {
      // seq-guard: быстрое переключение типов не даёт устаревшему ответу затереть
      // актуальный список блокеров (урок #632).
      const seq = ++this.typeUsersSeq;
      this.typeUsersLoading = true;
      this.typeUsers = [];
      try {
        const users = await getUserTypeBlockingUsers(typeId);
        if (seq === this.typeUsersSeq) this.typeUsers = Array.isArray(users) ? users : [];
      } catch {
        if (seq === this.typeUsersSeq) {
          this.typeUsers = [];
          useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'пользователей типа', type: 'error' });
        }
      } finally {
        if (seq === this.typeUsersSeq) this.typeUsersLoading = false;
      }
    },
    openReassign() {
      this.reassignTargetId = null;
      this.reassignSourceName = this.selectedType ? this.selectedType.name : '';
      this.reassignVisible = true;
    },
    closeReassign() {
      // Пока перенос летит, окно не закрываем: его оверлей блокирует список -
      // иначе смена типа дала бы гонку loadTypeUsers.
      if (this.reassignSubmitting) return;
      this.reassignVisible = false;
    },
    async performReassign() {
      if (!this.reassignTargetId || this.reassignSubmitting || !this.selectedType) return;
      const source = this.selectedType;
      const target = this.reassignTargetOptions.find(o => o.value === this.reassignTargetId);
      this.reassignSubmitting = true;
      try {
        const data = await reassignUserTypeUsers(source.id, this.reassignTargetId);
        const n = data?.reassigned ?? 0;
        this.reassignVisible = false;
        // Источник освобождён (все перенесены) - счётчик детали в 0, чтобы шапка не
        // врала до перечитывания списка.
        if (this.selectedType && this.selectedType.id === source.id) {
          this.selectedType.users_count = 0;
        }
        useDeletionsStore().notify({
          prefix: 'Перенесено ',
          bold: `${n} ${this.usersPlural(n)}`,
          suffix: ` в «${target ? target.label : ''}»`,
        });
        // Перечитываем блокеров источника (пусто -> тип можно удалить) и список
        // типов (users_count упал у источника, вырос у цели).
        this.loadTypeUsers(source.id);
        await this.refreshData();
      } catch (error) {
        useDeletionsStore().notify({ prefix: 'Не удалось перенести: ', bold: error.message || 'ошибка', type: 'error' });
      } finally {
        this.reassignSubmitting = false;
      }
    },
    usersPlural(n) {
      const mod10 = n % 10;
      const mod100 = n % 100;
      if (mod10 === 1 && mod100 !== 11) return 'пользователь';
      if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return 'пользователя';
      return 'пользователей';
    },
    userFullName(u) {
      const parts = [u.last_name, u.first_name, u.middle_name].filter(Boolean);
      return parts.join(' ') || u.username || '—';
    },
    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'asc';
      }
    },
    closeModal() {
      this.showAddModal = false;
      this.newType = {
        name: '',
        code: ''
      };
      this.nameError = '';
    },
    openHistory(type) {
      this.historyForType = type;
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
    }
  }
};
</script>

<style scoped>
.user-types-container {
  background: var(--surface);
  border-radius: 16px;
  border: 1px solid var(--border);
  overflow: hidden;
  width: 100%;
  height: 450px;
  position: relative;
}

.management-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  height: 50px;
}

.management-title {
  font-size: 1.2em;
  margin: 0;
  font-weight: 600;
  color: var(--text);
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
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

.content-container {
  display: flex;
  height: 400px;
  width: 100%;
}

.types-section {
  width: 40%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--border);
}

.types-section.with-details {
  width: 40%;
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
  transition: .2s;
  cursor: pointer;
  user-select: none;
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
  transition: .2s;
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
  width: 20%;
  min-width: 60px;
}

.name-col {
  width: 50%;
  min-width: 200px;
}

.users-col {
  width: 30%;
  min-width: 100px;
}

.table-body {
  flex: 1;
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

.users-count {
  font-size: 14px;
  font-weight: bold;
  color: var(--text);
}

.table-footer {
  margin-top: auto;
  padding: 6px 20px;
  border-top: 1px solid var(--border);
  text-align: end;
  background: var(--accent-tint);
}

.items-count {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}

.details-section {
  width: 60%;
  padding: 15px;
  overflow-y: auto;
  background: var(--surface-2);
}

.details-content {
  height: 100%;
}

.details-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 15px;
}

.details-title-wrapper {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.details-title {
  margin: 0;
  color: var(--text);
  font-size: 1.2em;
  font-weight: 600;
}

.type-info-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.system-name {
  font-size: 0.85em;
  color: var(--text-muted);
  background: var(--surface-2);
  border-radius: 6px;
}

.users-count-badge {
  font-size: 0.8em;
  color: var(--text-muted);
  background: var(--accent-tint);
  padding: 4px 8px;
  border-radius: 999px;
}

.system-badge {
  display: inline-block;
  margin-left: 8px;
  font-size: 11px;
  font-weight: 500;
  color: var(--warning-text);
  background: var(--warning-bg);
  border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
  padding: 2px 8px;
  border-radius: 999px;
  vertical-align: middle;
}

.details-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.delete-icon-btn {
  outline: none;
  border: none;
  width: 30px;
  height: 30px;
  padding: 5px;
  border-radius: 10px;
  display: flex;
  align-items:center;
  justify-content: center;
  transition: .2s;
}

.delete-icon {
  color: var(--danger);
  width: 20px;
  height: 20px;
}

.delete-icon-btn:hover {
  background-color: var(--border);
  cursor:pointer;
}

.details-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.compact-form {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.form-column {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-group.compact {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.detail-label {
  font-size: 0.85em;
  color: var(--text-muted);
  font-weight:400;
}

.form-hint {
  font-size: 0.7em;
  color: var(--text-muted);
  margin-top: 4px;
}

.form-error {
  font-size: 0.7em;
  color: var(--danger-text);
  margin-top: 4px;
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

.no-results {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-muted);
  width: 100%;
}

/* Блокеры удаления типа: пользователи + перенос (#1379) */
.type-users {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.type-users__title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.9em;
  font-weight: 600;
  color: var(--text);
}

.count-badge {
  font-size: 0.75em;
  font-weight: 600;
  color: var(--accent-text);
  background: var(--accent-tint);
  padding: 2px 8px;
  border-radius: 999px;
}

.blocking-notice {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--color-border);
  background: var(--color-primary-tint);
  border-radius: var(--radius-md);
}

.blocking-notice__text {
  flex: 1 1 200px;
  min-width: 0;
  font-size: 0.82em;
  color: var(--text-muted);
  line-height: 1.45;
}

.blocking-notice__btn {
  flex-shrink: 0;
}

.type-users__loading,
.type-users__empty {
  font-size: 0.85em;
  color: var(--text-muted);
  margin: 0;
}

.type-users__list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.type-user {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 10px;
  background: var(--surface);
  border: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  border-radius: var(--radius-md);
}

.type-user__who {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.type-user__who b {
  font-size: 0.85em;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.type-user__who small {
  font-size: 0.75em;
  color: var(--text-muted);
}

.type-user--archived {
  opacity: 0.7;
}

.archived-badge {
  flex-shrink: 0;
  font-size: 11px;
  font-weight: 500;
  color: var(--text-muted);
  background: var(--surface-2);
  padding: 2px 8px;
  border-radius: 999px;
}

/* Модалка переноса блокеров */
.reassign-intro {
  margin: 0 0 14px;
  font-size: 0.9em;
  color: var(--text-muted);
  line-height: 1.5;
}

.field-label {
  display: block;
  margin-bottom: 6px;
  font-size: 0.85em;
  font-weight: 500;
  color: var(--text);
}

.reassign-empty {
  margin: 10px 0 0;
  font-size: 0.85em;
  color: var(--danger-text);
}

/* Стили для улучшенного модального окна */
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
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
  animation: overlayAppear 0.3s ease-out;
}

@keyframes overlayAppear {
  from {
    background: var(--overlay);
    backdrop-filter: blur(0px);
  }
  to {
    background: var(--overlay);
    backdrop-filter: blur(0.1px);
  }
}

.modal-content {
  background: var(--surface);
  border-radius: 30px;
  padding: 0;
  width: 420px;
  max-width: 90vw;
  box-shadow: 0 20px 60px var(--shadow-drop);
  animation: modalAppear 0.3s ease-out;
}

@keyframes modalAppear {
  from {
    opacity: 0;
    transform: scale(0.8) translateY(-20px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 24px 16px;
  border-bottom: 1px solid var(--border);
}

.modal-title {
  margin: 0;
  font-size: 1.1em;
  font-weight: 600;
  color: var(--text);
}

.modal-close {
  background: none;
  border: none;
  cursor: pointer;
  padding: 6px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
}

.modal-close:hover {
  background-color: var(--surface-2);
  transform: rotate(90deg);
}

.modal-body {
  padding: 20px 24px;
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 16px;
}

.input-label {
  font-size: 0.85em;
  font-weight: 500;
  color: var(--text);
  margin-bottom: 2px;
}

.modal-input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  font-size: 0.9em;
  transition: all 0.2s ease;
  background: var(--surface);
}

.modal-input:focus {
  border-color: var(--accent);
  outline: none;
  box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.1);
}

.modal-input::placeholder {
  color: var(--text-muted);
}

.input-hint {
  font-size: 0.75em;
  color: var(--text-muted);
  margin-top: 2px;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 24px 20px;
  border-top: 1px solid var(--border);
}


/* Анимации для модального окна */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: all 0.3s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .modal-overlay,
.modal-fade-leave-active .modal-overlay {
  transition: all 0.3s ease;
}

.modal-fade-enter-active .modal-content,
.modal-fade-leave-active .modal-content {
  transition: all 0.3s ease;
}

.modal-fade-enter-from .modal-overlay,
.modal-fade-leave-to .modal-overlay {
  background: transparent;
  backdrop-filter: blur(0px);
}

.modal-fade-enter-from .modal-content,
.modal-fade-leave-to .modal-content {
  opacity: 0;
  transform: scale(0.8) translateY(-20px);
}

/* Стили для уведомлений */
@media (max-width: 767.98px) {
  /* Направление/высоту шапки берёт на себя глобальный .rt-header-inline
     (responsive-tables.css, !important - перебивает scoped-специфичность). */
  .header-controls {
    flex-wrap: wrap;
    row-gap: 8px;
  }

  :deep(.search) {
    width: 150px;
  }

  .content-container {
    flex-direction: column;
    height: auto;
  }

  .types-section,
  .details-section,
  .no-selection-message {
    width: 100% !important;
  }
  
  .types-section.with-details {
    border-right: none;
    border-bottom: 1px solid var(--border);
    height: 255px;
  }
  
  .modal-content {
    height: auto;
    max-height: 80vh;
    width: 95%;
  }
  
  .type-info-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
}
</style>