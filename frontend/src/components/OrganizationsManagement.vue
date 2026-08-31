<template>
  <div class="organizations-management dashboard-card">
    <div class="management-header rt-header-inline">
      <h3 class="management-title">
        Управление организациями / отделами
      </h3>
      <div class="header-controls">
        <BaseDropdown
          class="archive-dropdown"
          data-testid="orgs-list-mode"
          :model-value="listMode"
          :options="archiveOptions"
          label-key="label"
          value-key="value"
          @update:model-value="onArchiveModeChange"
        />
        <BaseDropdown
          class="type-filter-dropdown"
          data-testid="orgs-type-filter"
          multiple
          :model-value="typeFilters"
          :options="typeFilterOptions"
          label-key="label"
          value-key="value"
          :placeholder="typeFilterAllLabel"
          summary-label="Тип"
          @update:model-value="typeFilters = $event"
        />
        <SearchComponent
          v-model="searchQuery"
          :title="'Поиск организаций...'"
        />
        <button
          class="add-header-button rt-btn-compact"
          data-testid="orgs-add-btn"
          aria-label="Добавить"
          @click="openAddModal"
        >
          <span
            class="rt-btn-icon"
            aria-hidden="true"
          >+</span>
          <span class="rt-btn-label">Добавить</span>
        </button>
        <RefreshButton
          :loading="isLoading"
          @refresh="refreshData"
        />
      </div>
    </div>

    <!-- Панель групповых операций (появляется при выделении строк) -->
    <transition name="bulk-slide">
      <div
        v-if="selectedIds.length"
        class="bulk-bar"
        data-testid="orgs-bulk-bar"
      >
        <span class="bulk-count">Выбрано: {{ selectedIds.length }}</span>
        <div class="bulk-actions">
          <template v-if="!showArchive">
            <button
              class="pill pill-ghost"
              data-testid="orgs-bulk-type"
              @click="startBulkOperation('type')"
            >
              Тип
            </button>
            <button
              class="pill pill-ghost"
              data-testid="orgs-bulk-unload-places"
              @click="startBulkOperation('unload-places')"
            >
              Места
            </button>
            <button
              class="pill pill-ghost"
              data-testid="orgs-bulk-tables"
              @click="startBulkOperation('tables')"
            >
              Таблицы
            </button>
            <button
              class="pill pill-ghost"
              data-testid="orgs-bulk-users"
              @click="startBulkOperation('users')"
            >
              Ответственные
            </button>
            <button
              class="pill pill-danger"
              data-testid="orgs-bulk-archive"
              @click="startBulkOperation('archive')"
            >
              В архив
            </button>
          </template>
          <button
            v-else
            class="pill pill-restore"
            data-testid="orgs-bulk-restore"
            @click="startBulkOperation('restore')"
          >
            Восстановить
          </button>
          <button
            class="pill pill-ghost bulk-clear"
            data-testid="orgs-bulk-clear"
            @click="clearSelection"
          >
            Снять выбор
          </button>
        </div>
      </div>
    </transition>

    <div class="content-container">
      <!-- Левая часть - таблица организаций -->
      <div class="table-section">
        <div class="table-container rt-table">
          <div class="table-header rt-head-row">
            <div
              class="header-col check-col"
              @click.stop
            >
              <input
                type="checkbox"
                class="bulk-check"
                :checked="allSelected"
                :indeterminate.prop="someSelected"
                aria-label="Выбрать все"
                data-testid="orgs-select-all"
                @change="toggleSelectAll"
              >
            </div>
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
              class="header-col type-col"
              @click="sortBy('type')"
            >
              <p :class="{ 'active-sort': sortField === 'type' }">
                Тип
              </p>
              <AppIcon
                name="sort"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'type',
                  'desc': sortField === 'type' && sortDirection === 'desc'
                }"
              />
            </div>
            <div
              class="header-col users-col"
              @click="sortBy('user_count')"
            >
              <p :class="{ 'active-sort': sortField === 'user_count' }">
                Пользователи
              </p>
              <AppIcon
                name="sort"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'user_count',
                  'desc': sortField === 'user_count' && sortDirection === 'desc'
                }"
              />
            </div>
          </div>

          <div class="table-body">
            <div
              v-for="(org, index) in sortedOrganizations"
              :key="org.id"
              class="table-row rt-row"
              data-testid="orgs-row"
              :class="{
                'selected': selectedOrganization && selectedOrganization.id === org.id,
                'inactive': !org.is_active
              }"
              @click="selectOrganization(org)"
            >
              <div
                class="table-col check-col"
                @click.stop
              >
                <input
                  type="checkbox"
                  class="bulk-check"
                  :checked="isSelected(org.id)"
                  :aria-label="`Выбрать ${org.name}`"
                  data-testid="orgs-row-check"
                  @click="onRowCheck(org, index, $event)"
                >
              </div>
              <div
                class="table-col id-col"
                data-label="ID"
              >
                <span class="cell-content id-value">{{ org.id }}</span>
              </div>
              <div
                class="table-col name-col"
                data-label="Наименование"
              >
                <span
                  class="truncate-text"
                  :title="org.name"
                >
                  {{ org.name }}
                  <span
                    v-if="!org.is_active"
                    class="inactive-badge"
                  >(архив)</span>
                </span>
                <!-- Запись заведена подачей заявки и ждёт разбора (#1437): в справочнике
                     она уже живая, поэтому её отличает бейдж, а не отдельный список.
                     Бейдж - сосед усечённого наименования, а не его часть: внутри
                     truncate-text его срезало бы многоточием вместе с длинным именем. -->
                <span
                  v-if="isPendingModeration(org)"
                  class="moderation-badge"
                  data-testid="orgs-row-pending"
                >на проверке</span>
              </div>
              <div
                class="table-col type-col"
                data-label="Тип"
              >
                <span
                  class="truncate-text type-value"
                  :class="{ 'type-unspecified': !org.type }"
                  :title="orgTypeLabel(org.type)"
                >{{ orgTypeLabel(org.type) }}</span>
              </div>
              <div
                class="table-col users-col"
                data-label="Пользователи"
              >
                <span class="cell-content user-count">
                  <span class="count-value">{{ org.user_count }}</span>
                </span>
              </div>
            </div>

            <div
              v-if="!sortedOrganizations.length && !isLoading"
              class="no-results-inline"
            >
              {{ emptyText }}
            </div>
            <div
              v-if="isLoading && !organizationsWithUsers.length"
              class="orgs-loading"
            >
              <LoaderSpinner label="Загрузка организаций..." />
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">
              {{ countLabel }}: {{ sortedOrganizations.length }}
            </span>
          </div>
        </div>
      </div>

      <!-- Правая часть - детали (рабочая область, вариант 1) -->
      <div class="details-column">
        <div
          v-if="selectedOrganization"
          class="v1"
          data-testid="orgs-details"
        >
          <!-- Хедер деталей -->
          <div class="d-head">
            <div class="d-head-info">
              <h2 class="d-title">
                {{ originalSelectedName }}
              </h2>
              <div class="d-meta">
                <span class="pill pill-type">Тип: {{ orgTypeLabel(originalSelectedType) }}</span>
                <span class="muted-hint">
                  · {{ members.length }} участников<template v-if="selectedOrganization.is_active"> · {{ responsiblesCount }} ответственных</template>
                </span>
              </div>
            </div>
            <div class="d-acts">
              <span
                v-if="isPendingModeration(selectedOrganization)"
                class="moderation-badge"
                data-testid="orgs-details-pending"
              >На проверке</span>
              <span
                v-if="!selectedOrganization.is_active"
                class="archive-badge"
              >В архиве</span>
              <button
                v-if="canViewHistory"
                class="pill pill-ghost"
                data-testid="orgs-history"
                @click="openHistory(selectedOrganization)"
              >
                История
              </button>
              <button
                v-if="selectedOrganization.is_active"
                class="pill pill-danger"
                data-testid="orgs-archive"
                @click="onArchiveClick(selectedOrganization)"
              >
                В архив
              </button>
              <button
                v-else
                class="pill pill-restore"
                data-testid="orgs-restore"
                @click="onRestore(selectedOrganization)"
              >
                Восстановить
              </button>
            </div>
          </div>

          <!-- Разбор записи, заведённой подачей заявки (#1875). Секция даёт заголовок
               раздела, сам разбор внутри неё - жёлтая плашка предупреждения. -->
          <div
            v-if="canModerate && isPendingModeration(selectedOrganization)"
            class="card"
            data-testid="orgs-moderation-card"
          >
            <div class="sec-title">
              Разбор записи
            </div>
            <DirectoryModeration
              kind="organization"
              variant="panel"
              :entry-id="selectedOrganization.id"
              :entry-name="originalSelectedName"
              @resolved="onModerationResolved"
            />
          </div>

          <!-- Основное -->
          <div class="card">
            <div class="sec-title">
              Основное
            </div>
            <div class="basic">
              <div class="basic-field">
                <label class="field-label">Наименование</label>
                <input
                  v-model.trim="selectedOrganization.name"
                  type="text"
                  class="lk-input"
                  maxlength="100"
                  placeholder="Введите название организации"
                  autocomplete="off"
                  :disabled="!selectedOrganization.is_active || isSavingName || isNameLockedByModeration"
                  data-testid="orgs-detail-name"
                  @keyup.enter="saveSelectedName"
                >
              </div>
              <div class="basic-field">
                <label class="field-label">Тип</label>
                <BaseDropdown
                  data-testid="orgs-detail-type"
                  :model-value="selectedOrganization.type"
                  :options="typeDetailOptions"
                  label-key="label"
                  value-key="value"
                  :placeholder="unspecifiedTypeLabel"
                  :disabled="!selectedOrganization.is_active || isSavingName"
                  @update:model-value="onDetailTypeChange"
                />
              </div>
            </div>

            <p
              v-if="nameLockHint"
              class="lock-note"
              data-testid="orgs-name-lock-hint"
            >
              {{ nameLockHint }}
            </p>

            <div
              v-if="detailError"
              class="form-error"
            >
              {{ detailError }}
            </div>

            <div
              v-if="selectedOrganization.is_active"
              class="save-actions"
            >
              <button
                class="lk-button lk-button--primary"
                :disabled="!isDetailsDirty || isSavingName"
                data-testid="orgs-save-name"
                @click="saveSelectedName"
              >
                Сохранить
              </button>
              <span class="muted-hint">{{ saveHint }}</span>
            </div>
          </div>

          <!-- Редактируемые секции (только для активной организации) -->
          <template v-if="selectedOrganization.is_active">
            <SelectUnloadPlaces
              ref="places"
              :entity="selectedOrganization"
              :entity-type="'organization'"
              @places-updated="handlePlacesUpdated"
              @dirty-change="placesDirty = $event"
            />

            <SelectTables
              ref="tables"
              :entity="selectedOrganization"
              :entity-type="'organization'"
              @tables-updated="handleTablesUpdated"
              @dirty-change="tablesDirty = $event"
            />

            <ResponsibleUsersSection
              ref="responsibles"
              :entity="selectedOrganization"
              :entity-type="'organization'"
              @users-updated="handleUsersUpdated"
              @dirty-change="responsiblesDirty = $event"
              @count-change="responsiblesCount = $event"
            />
          </template>
          <div
            v-else
            class="card"
          >
            <p class="archive-hint">
              Восстановите организацию, чтобы редактировать места разгрузки, таблицы и ответственных.
            </p>
          </div>

          <!-- Привязанные пользователи -->
          <div
            class="card"
            data-testid="orgs-members"
          >
            <div class="sec-title">
              Пользователи, привязанные к организации
              <span class="count-badge">{{ members.length }}</span>
            </div>
            <div
              v-if="selectedOrganization.is_active && members.length"
              class="blocking-notice"
              data-testid="orgs-blocking-notice"
            >
              <span class="blocking-notice__text">
                Пока эти пользователи привязаны, организацию нельзя архивировать.
              </span>
              <button
                v-if="canReassign"
                type="button"
                class="lk-button lk-button--secondary blocking-notice__btn"
                data-testid="orgs-reassign-open"
                @click="openReassign"
              >
                Перенести всех в другую организацию
              </button>
            </div>
            <div
              v-if="membersLoading"
              class="members-loading"
            >
              <LoaderSpinner label="Загрузка пользователей..." />
            </div>
            <div
              v-else-if="members.length"
              class="stack"
            >
              <div
                v-for="m in members"
                :key="m.id"
                class="person"
              >
                <div class="avatar">
                  {{ memberInitials(m) }}
                </div>
                <div class="who">
                  <b>{{ memberFullName(m) }}</b>
                  <small v-if="m.position">{{ m.position }}</small>
                </div>
              </div>
            </div>
            <p
              v-else
              class="members-empty"
            >
              Нет привязанных пользователей
            </p>
          </div>
        </div>

        <div
          v-else
          class="no-selection-message"
        >
          <p>Выберите организацию для просмотра</p>
        </div>
      </div>
    </div>

    <!-- Модальное окно добавления -->
    <BaseModal
      :show="showAddModal"
      title="Новая организация"
      width="440px"
      radius="30px"
      content-testid="orgs-modal"
      @close="requestCloseAdd"
    >
      <div class="orgs-modal-body">
        <div class="form-group">
          <label class="form-label">Название организации</label>
          <input
            ref="nameInput"
            v-model.trim="addForm.name"
            type="text"
            placeholder="Введите название организации"
            maxlength="100"
            class="lk-input"
            data-testid="orgs-input-name"
            @keyup.enter="submitAdd"
          >
        </div>
        <div class="form-group">
          <label class="form-label">Тип</label>
          <BaseDropdown
            data-testid="orgs-input-type"
            teleport
            :menu-z-index="1100"
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

      <template #actions>
        <button
          class="lk-button lk-button--ghost"
          data-testid="orgs-modal-cancel"
          @click="requestCloseAdd"
        >
          Отмена
        </button>
        <button
          class="lk-button lk-button--primary"
          :disabled="!addForm.name || !addForm.type || isAdding"
          data-testid="orgs-modal-save"
          @click="submitAdd"
        >
          Создать
        </button>
      </template>
    </BaseModal>

    <!-- Модальное окно подтверждения архивации -->
    <ConfirmationModal
      :show="!!archiveConfirmOrg"
      title="Архивация организации"
      :message="archiveConfirmOrg ? `Архивировать организацию «${archiveConfirmOrg.name}»? Её можно будет восстановить из архива.` : ''"
      confirm-text="В архив"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
      @confirm="performArchive"
      @cancel="archiveConfirmOrg = null"
    />

    <!-- Групповые операции: type/места/таблицы/ответственные -->
    <BulkOperationsModal
      :show="bulkModalVisible"
      entity-type="organization"
      :operation="pendingBulkOp"
      :selected-ids="selectedIds"
      :submitting="bulkSubmitting"
      @close="closeBulkModal"
      @apply="applyBulk"
    />

    <!-- Групповой архив/восстановление -->
    <ConfirmationModal
      :show="bulkConfirmVisible"
      :title="bulkConfirmTitle"
      :message="bulkConfirmMessage"
      :confirm-text="bulkConfirmText"
      cancel-text="Отмена"
      :confirm-button-style="bulkConfirmButtonStyle"
      @confirm="applyBulkArchiveRestore"
      @cancel="cancelBulkConfirm"
    />

    <OrgHistoryModal
      v-if="historyOrg"
      :organization="historyOrg"
      :current-user-name="currentUserName"
      @close="historyOrg = null"
    />

    <!-- Перенос всех блокирующих участников в другую организацию -->
    <BaseModal
      :show="reassignVisible"
      title="Перенести всех пользователей"
      width="460px"
      radius="30px"
      @close="closeReassign"
    >
      <div
        class="reassign-body"
        data-testid="orgs-reassign-modal"
      >
        <p class="reassign-intro">
          Пользователи организации «{{ originalSelectedName }}»
          ({{ members.length }}) будут перенесены в выбранную. После этого её
          можно будет архивировать.
        </p>
        <label class="field-label">Целевая организация</label>
        <BaseDropdown
          :model-value="reassignTargetId"
          :options="reassignTargetOptions"
          label-key="label"
          value-key="value"
          :searchable="true"
          :teleport="true"
          placeholder="Выберите организацию"
          data-testid="orgs-reassign-target"
          @update:model-value="reassignTargetId = $event"
        />
        <p
          v-if="!reassignTargetOptions.length"
          class="reassign-empty"
        >
          Нет других активных организаций для переноса.
        </p>
      </div>
      <template #actions>
        <button
          type="button"
          class="lk-button lk-button--ghost"
          data-testid="orgs-reassign-cancel"
          @click="closeReassign"
        >
          Отмена
        </button>
        <button
          type="button"
          class="lk-button lk-button--primary"
          :disabled="!reassignTargetId || reassignSubmitting"
          data-testid="orgs-reassign-submit"
          @click="performReassign"
        >
          Перенести
        </button>
      </template>
    </BaseModal>
  </div>
</template>

<script>
import { mapState, mapActions } from 'pinia';
import {
  getOrganizationMembers,
  reassignOrganizationUsers,
  bulkUpdateOrganizationType,
  bulkAssignOrganizationUnloadPlaces,
  bulkAssignOrganizationTables,
  bulkAssignOrganizationUsers,
  bulkArchiveOrganizations,
  bulkRestoreOrganizations,
} from '@/api/organizations';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import {
  ORG_TYPE_CREATE_OPTIONS,
  ORG_TYPE_DETAIL_OPTIONS,
  ORG_TYPE_FILTER_OPTIONS,
  ORG_TYPE_FILTER_ALL_LABEL,
  ORG_TYPE_FILTER_UNSPECIFIED,
  ORG_TYPE_UNSPECIFIED_LABEL,
} from '@/constants/orgTypes';
import { useOrganizationsStore } from '@/stores/organizations';
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';
import { registerDirtyTracker, confirmIfAnyDirty } from '@/utils/dirtyTracker';
import RefreshButton from './RefreshButton.vue';
import SearchComponent from './SearchComponent.vue';
import ResponsibleUsersSection from './ResponsibleUsersSection.vue';
import SelectUnloadPlaces from './SelectUnloadPlaces.vue';
import SelectTables from './SelectTables.vue';
import ConfirmationModal from './ConfirmationModal.vue';
import BaseDropdown from './ui/BaseDropdown.vue';
import BaseModal from './ui/BaseModal.vue';
import LoaderSpinner from './ui/LoaderSpinner.vue';
import OrgHistoryModal from './OrgHistoryModal.vue';
import BulkOperationsModal from './directories/BulkOperationsModal.vue';
import DirectoryModeration from './directory/DirectoryModeration.vue';
import AppIcon from '@/components/icons/AppIcon.vue';
import { fetchCurrentUserName } from '@/utils/currentUserName';
import { openFromSearchLink } from '@/mixins/openFromSearchLink';

export default {
  name: 'OrganizationsManagement',
  components: {
    DirectoryModeration,
    SearchComponent,
    RefreshButton,
    ResponsibleUsersSection,
    SelectUnloadPlaces,
    SelectTables,
    ConfirmationModal,
    BaseDropdown, BaseModal,
    LoaderSpinner,
    OrgHistoryModal,
    BulkOperationsModal,
    AppIcon,
  },
  mixins: [openFromSearchLink((vm) => vm.organizationsWithUsers, 'selectOrganization')],
  data() {
    return {
      searchQuery: '',
      listMode: 'active',
      typeFilters: [],
      selectedIds: [],
      // Якорь для shift-выделения диапазона строк (id последней кликнутой строки).
      lastSelectedId: null,
      // Выбранная групповая операция ('type'|'unload-places'|'tables'|'users'|
      // 'archive'|'restore'). type/places/tables/users открывают BulkOperationsModal,
      // archive/restore — ConfirmationModal ниже.
      pendingBulkOp: null,
      bulkModalVisible: false,
      bulkConfirmVisible: false,
      bulkSubmitting: false,
      showAddModal: false,
      addForm: { name: '', type: null },
      addError: '',
      isAdding: false,
      selectedOrganization: null,
      originalSelectedName: '',
      originalSelectedType: null,
      members: [],
      membersLoading: false,
      membersSeq: 0,
      responsiblesDirty: false,
      placesDirty: false,
      tablesDirty: false,
      responsiblesCount: 0,
      detailError: '',
      isSavingName: false,
      archiveConfirmOrg: null,
      reassignVisible: false,
      reassignTargetId: null,
      reassignSubmitting: false,
      historyOrg: null,
      currentUserName: '',
      sortField: null,
      sortDirection: 'asc',
      archiveOptions: [
        { label: 'Активные', value: 'active' },
        { label: 'Архив', value: 'archive' },
        { label: 'На проверке', value: 'pending' },
      ],
      typeCreateOptions: ORG_TYPE_CREATE_OPTIONS,
      typeDetailOptions: ORG_TYPE_DETAIL_OPTIONS,
      typeFilterOptions: ORG_TYPE_FILTER_OPTIONS,
      typeFilterAllLabel: ORG_TYPE_FILTER_ALL_LABEL,
      unspecifiedTypeLabel: ORG_TYPE_UNSPECIFIED_LABEL,
    };
  },
  computed: {
    ...mapState(useOrganizationsStore, {
      organizationsWithUsers: 'itemsWithUsers',
      isLoading: 'isLoading',
    }),
    // Гейты зеркалят BE: перенос пользователей и история организации закрыты тем же
    // page.admin.directories, что открывает экран (#1982). Ключ держим явно, а не
    // считаем кнопки всегда доступными: разойдётся право маршрута - разойдётся и здесь.
    canReassign() {
      return usePermissionsStore().hasPermission('page.admin.directories');
    },
    canViewHistory() {
      return usePermissionsStore().hasPermission('page.admin.directories');
    },
    // Архивный режим - производная от режима списка: наборы групповых операций и
    // подписи по-прежнему делятся на «активные» и «архив», а «на проверке» - срез
    // активных, поэтому он идёт по ветке активных.
    showArchive() {
      return this.listMode === 'archive';
    },
    // Разбор записи гейтится своим правом, не page.admin: справочник открыт по
    // page.admin.directories, а moderation-эндпоинты - по application.organization.moderate
    // (иначе «видно, но 403», уроки #976/#1083).
    canModerate() {
      return usePermissionsStore().hasPermission('application.organization.moderate');
    },
    // Наименование неразобранной записи правится ТОЛЬКО разбором (#1876): обычный
    // PUT оставляет moderation_status=pending, админ видел «сохранено» и тот же
    // бейдж «На проверке». Тип не блокируем - он к разбору отношения не имеет.
    isNameLockedByModeration() {
      return !!this.selectedOrganization && this.isPendingModeration(this.selectedOrganization);
    },
    // Подсказка обязана следовать за состоянием: без права разбора блок разбора не
    // отрисован, и отсылать к нему бессмысленно - человеку нужен адрес, куда идти.
    nameLockHint() {
      if (!this.isNameLockedByModeration) return '';
      if (!this.canModerate) {
        return 'Запись ещё не разобрана, а права на разбор у вас нет: '
          + 'наименование исправит сотрудник, которому разбор доступен.';
      }
      return 'Запись ещё не разобрана, поэтому наименование правится только действием '
        + '«Исправить наименование» в блоке «Разбор записи» выше.';
    },
    saveHint() {
      return this.isNameLockedByModeration
        ? 'Пока запись не разобрана, сохраняется только тип'
        : 'Имя и тип сохраняются вместе';
    },
    countLabel() {
      if (this.listMode === 'archive') return 'В архиве';
      if (this.listMode === 'pending') return 'На проверке';
      return 'Всего организаций';
    },
    // Цели переноса - активные организации, кроме исходной (BE отвергает архивную
    // и == источнику). Архивных в списке нет by design.
    reassignTargetOptions() {
      if (!this.selectedOrganization) return [];
      const srcId = this.selectedOrganization.id;
      return this.organizationsWithUsers
        .filter(org => org.is_active && org.id !== srcId)
        .map(org => ({ label: org.name, value: org.id }));
    },
    filteredOrganizations() {
      let list = this.organizationsWithUsers.filter(org => this.matchesListMode(org));
      if (this.typeFilters.length) {
        // «не указан» - такой же элемент набора, как остальные типы: NULL/пусто
        // приводим к его сентинелу, чтобы «Отдел + не указан» отдавал и то, и то.
        const types = new Set(this.typeFilters);
        list = list.filter(org => types.has(org.type || ORG_TYPE_FILTER_UNSPECIFIED));
      }
      const variants = buildSearchVariants(this.searchQuery);
      if (!variants.length) return list;
      return list.filter(org => matchesSearch(`${org.name} ${org.id} ${org.type || ''}`, variants));
    },
    sortedOrganizations() {
      const organizations = [...this.filteredOrganizations];

      if (!this.sortField) {
        return organizations.sort((a, b) => a.name.localeCompare(b.name));
      }

      return organizations.sort((a, b) => {
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
      if (this.typeFilters.length) {
        return this.typeFilters.length === 1 ? 'Нет организаций с таким типом' : 'Нет организаций с выбранными типами';
      }
      if (this.listMode === 'archive') return 'В архиве пусто';
      if (this.listMode === 'pending') return 'Записей на проверке нет';
      return 'Организаций пока нет';
    },
    isAddDirty() {
      return this.showAddModal && (this.addForm.name.trim() !== '' || !!this.addForm.type);
    },
    isDetailsDirty() {
      if (!this.selectedOrganization || !this.selectedOrganization.is_active) return false;
      const nameChanged = this.selectedOrganization.name.trim() !== this.originalSelectedName;
      const typeChanged = (this.selectedOrganization.type ?? null) !== (this.originalSelectedType ?? null);
      return nameChanged || typeChanged;
    },
    // fix 5: несохранённые изменения в области деталей = имя/тип ИЛИ любой из
    // детей (места/таблицы/ответственные), поднявших свой dirty через emit.
    detailsAreaDirty() {
      return this.isDetailsDirty || this.placesDirty || this.tablesDirty || this.responsiblesDirty;
    },
    isDirty() {
      return this.isAddDirty || this.detailsAreaDirty;
    },
    // Выбор всегда подмножество видимых строк (watch подрезает при фильтрации),
    // поэтому равенство длин = выбраны все видимые.
    allSelected() {
      return this.sortedOrganizations.length > 0
        && this.selectedIds.length === this.sortedOrganizations.length;
    },
    someSelected() {
      return this.selectedIds.length > 0 && !this.allSelected;
    },
    bulkConfirmTitle() {
      return this.pendingBulkOp === 'restore' ? 'Восстановление организаций' : 'Архивация организаций';
    },
    bulkConfirmMessage() {
      const n = this.selectedIds.length;
      return this.pendingBulkOp === 'restore'
        ? `Восстановить выбранные организации (${n}) из архива?`
        : `Архивировать выбранные организации (${n})? Их можно будет восстановить из архива. Организации с привязанными пользователями останутся активными.`;
    },
    bulkConfirmText() {
      return this.pendingBulkOp === 'restore' ? 'Восстановить' : 'В архив';
    },
    bulkConfirmButtonStyle() {
      return this.pendingBulkOp === 'restore'
        ? { background: '#10b981', borderColor: '#10b981' }
        : { background: '#c62828', borderColor: '#c62828' };
    },
  },
  watch: {
    organizationsWithUsers() { this.openFromSearchLink(); },
    showAddModal(newVal) {
      if (newVal) {
        this.$nextTick(() => {
          this.$refs.nameInput?.focus();
        });
      }
    },
    // Подрезаем выбор до видимых строк при смене фильтра/поиска/режима -
    // счётчик «Выбрано:N» и select-all остаются честными, скрытое не участвует.
    filteredOrganizations(list) {
      if (!this.selectedIds.length) return;
      const visible = new Set(list.map(o => o.id));
      const pruned = this.selectedIds.filter(id => visible.has(id));
      if (pruned.length !== this.selectedIds.length) this.selectedIds = pruned;
    },
  },
  mounted() {
    this.refreshData();
    this.fetchCurrentUser();
    this._stopGuard = registerDirtyTracker({
      isDirty: () => this.isDirty,
      getChanges: () => {
        if (this.isAddDirty) return [`Новая организация: "${this.addForm.name.trim()}"`];
        const changes = [];
        if (this.isDetailsDirty) {
          if (this.selectedOrganization.name.trim() !== this.originalSelectedName) {
            changes.push({ label: 'Наименование', from: this.originalSelectedName, to: this.selectedOrganization.name.trim() });
          }
          if ((this.selectedOrganization.type ?? null) !== (this.originalSelectedType ?? null)) {
            changes.push({ label: 'Тип', from: this.orgTypeLabel(this.originalSelectedType), to: this.orgTypeLabel(this.selectedOrganization.type) });
          }
        }
        if (this.placesDirty) changes.push('Места разгрузки (несохранённые)');
        if (this.tablesDirty) changes.push('Целевые таблицы (несохранённые)');
        if (this.responsiblesDirty) changes.push('Ответственные (несохранённые)');
        return changes;
      },
      save: async () => {
        if (this.isAddDirty) await this.submitAdd();
        if (this.isDetailsDirty) await this.saveSelectedName();
        if (this.placesDirty) await this.$refs.places?.saveUnloadPlaces();
        if (this.tablesDirty) await this.$refs.tables?.saveOrganizationTables();
        if (this.responsiblesDirty) await this.$refs.responsibles?.saveResponsibleUsers();
      },
    });
  },
  beforeUnmount() {
    this._stopGuard?.();
  },
  methods: {
    ...mapActions(useOrganizationsStore, [
      'refresh',
      'createOrganization',
      'updateOrganization',
      'deleteOrganization',
      'restoreOrganization',
      'fetchOrganizationsWithUsers',
    ]),


    async refreshData() {
      // Тянем и архивные тоже - переключение режима фильтрует на клиенте без рефетча.
      await this.refresh(true);
      this.syncSelected();
    },

    /**
     * Попадает ли запись в текущий режим списка. «На проверке» - срез активных, а не
     * третье состояние: разобранная или заархивированная запись из него выпадает.
     * @param {{ is_active?: boolean, moderation_status?: string }} org
     * @returns {boolean}
     */
    matchesListMode(org) {
      if (this.listMode === 'archive') return !org.is_active;
      if (this.listMode === 'pending') return org.is_active && this.isPendingModeration(org);
      return org.is_active;
    },

    syncSelected() {
      if (!this.selectedOrganization) return;
      const fresh = this.organizationsWithUsers.find(o => o.id === this.selectedOrganization.id);
      const visible = fresh && this.matchesListMode(fresh);
      if (fresh && visible && !this.isDetailsDirty) {
        this.selectedOrganization = { ...fresh };
        this.originalSelectedName = fresh.name;
        this.originalSelectedType = fresh.type ?? null;
      } else if (!visible) {
        this.selectedOrganization = null;
        this.members = [];
        this.resetChildDirty();
      }
    },

    async onArchiveModeChange(value) {
      if (this.detailsAreaDirty && !(await confirmIfAnyDirty())) return;
      this.listMode = value;
      this.selectedOrganization = null;
      this.members = [];
      this.detailError = '';
      this.resetChildDirty();
      // Наборы операций для активных и архивных разные - выбор не переносим.
      this.selectedIds = [];
      this.lastSelectedId = null;
      this.pendingBulkOp = null;
    },

    isSelected(id) {
      return this.selectedIds.includes(id);
    },

    toggleSelect(id) {
      const i = this.selectedIds.indexOf(id);
      if (i === -1) this.selectedIds.push(id);
      else this.selectedIds.splice(i, 1);
    },

    // onRowCheck обрабатывает клик по чекбоксу строки. Обычный клик - toggle.
    // Shift-клик - выделяет диапазон от якоря (последней кликнутой строки) до
    // текущей включительно, приводя его к состоянию, в которое переходит
    // shift-кликнутый чекбокс (снят -> выделить весь диапазон, и наоборот).
    // Якорь хранится по id и переиндексируется на лету - устойчив к пересортировке.
    onRowCheck(org, index, event) {
      // shift-клик не должен выделять текст (селект начинается на mousedown, .prevent его не гасит) -
      // гасим для любого shift-клика, включая fallback без валидного якоря.
      if (event.shiftKey && window.getSelection) window.getSelection().removeAllRanges();
      if (event.shiftKey && this.lastSelectedId != null && this.lastSelectedId !== org.id) {
        const anchor = this.sortedOrganizations.findIndex(o => o.id === this.lastSelectedId);
        if (anchor !== -1) {
          const target = !this.isSelected(org.id);
          const [from, to] = anchor < index ? [anchor, index] : [index, anchor];
          for (let i = from; i <= to; i += 1) {
            const id = this.sortedOrganizations[i].id;
            const sel = this.isSelected(id);
            if (target && !sel) this.selectedIds.push(id);
            else if (!target && sel) this.selectedIds.splice(this.selectedIds.indexOf(id), 1);
          }
          this.lastSelectedId = org.id;
          return;
        }
      }
      this.toggleSelect(org.id);
      this.lastSelectedId = org.id;
    },

    toggleSelectAll() {
      this.selectedIds = this.allSelected
        ? []
        : this.sortedOrganizations.map(o => o.id);
      this.lastSelectedId = null; // "выбрать всё" не задаёт якорь для shift-диапазона
    },

    clearSelection() {
      this.selectedIds = [];
      this.lastSelectedId = null;
      this.pendingBulkOp = null;
    },

    startBulkOperation(operation) {
      this.pendingBulkOp = operation;
      if (operation === 'archive' || operation === 'restore') {
        this.bulkConfirmVisible = true;
      } else {
        this.bulkModalVisible = true;
      }
    },

    closeBulkModal() {
      if (this.bulkSubmitting) return;
      this.bulkModalVisible = false;
      this.pendingBulkOp = null;
    },

    cancelBulkConfirm() {
      if (this.bulkSubmitting) return;
      this.bulkConfirmVisible = false;
      this.pendingBulkOp = null;
    },

    // Применение операций type/places/tables/users из BulkOperationsModal.
    async applyBulk(payload) {
      const ids = [...this.selectedIds];
      const op = this.pendingBulkOp;
      if (this.bulkSubmitting) return;
      if (!ids.length) {
        // выбор мог опустеть (напр. Обновить подрезал видимый список) - не молчим
        this.closeBulkModal();
        return;
      }
      this.bulkSubmitting = true;
      let result;
      try {
        switch (op) {
          case 'type':
            result = await bulkUpdateOrganizationType(ids, payload.type);
            break;
          case 'unload-places':
            result = await bulkAssignOrganizationUnloadPlaces(ids, payload.unloadPlaceIds, payload.mode);
            break;
          case 'tables':
            result = await bulkAssignOrganizationTables(ids, payload.tableIds, payload.mode);
            break;
          case 'users':
            result = await bulkAssignOrganizationUsers(ids, payload.users, payload.mode);
            break;
          default:
            this.bulkSubmitting = false;
            return;
        }
      } catch {
        // сеть/таймаут - модалку оставляем открытой с настройками для повтора
        useDeletionsStore().notify({ prefix: 'Не удалось выполнить групповую операцию', type: 'error' });
        this.bulkSubmitting = false;
        return;
      }
      this.bulkSubmitting = false;
      // Закрываем модалку только когда операция реально применилась. Ошибку-envelope
      // handleBulkResult вернёт false -> модалку держим открытой для повтора.
      if (this.handleBulkResult(op, result, ids.length)) {
        this.bulkModalVisible = false;
        this.pendingBulkOp = null;
      }
    },

    // Применение архива/восстановления из ConfirmationModal.
    async applyBulkArchiveRestore() {
      const ids = [...this.selectedIds];
      const op = this.pendingBulkOp;
      if (this.bulkSubmitting) return;
      if (!ids.length || (op !== 'archive' && op !== 'restore')) {
        this.bulkConfirmVisible = false;
        this.pendingBulkOp = null;
        return;
      }
      this.bulkSubmitting = true;
      let result;
      try {
        result = op === 'archive'
          ? await bulkArchiveOrganizations(ids)
          : await bulkRestoreOrganizations(ids);
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось выполнить групповую операцию', type: 'error' });
        this.bulkSubmitting = false;
        return;
      }
      this.bulkSubmitting = false;
      if (this.handleBulkResult(op, result, ids.length)) {
        this.bulkConfirmVisible = false;
        this.pendingBulkOp = null;
      }
    },

    // Разбор BulkOpResult: полный успех -> notify, частичный -> ui.warning с
    // перечнем непрошедших. Возвращает true, если операция применилась (валидный
    // BulkOpResult) - тогда сбрасываем выбор и обновляем список; false при
    // ошибке-envelope (success:false -> {message}), чтобы можно было повторить.
    handleBulkResult(op, result, total) {
      if (!result || typeof result.success_count !== 'number') {
        useDeletionsStore().notify({ prefix: result?.message || 'Не удалось выполнить групповую операцию', type: 'error' });
        return false;
      }
      const label = {
        type: 'Тип изменён',
        'unload-places': 'Места разгрузки назначены',
        tables: 'Целевые таблицы назначены',
        users: 'Ответственные назначены',
        archive: 'Архивировано',
        restore: 'Восстановлено',
      }[op] || 'Готово';

      if (result.error_count > 0) {
        const failed = (result.errors || []).map(e => e.name || `#${e.id}`).join(', ');
        useDeletionsStore().notify({ prefix: 'Выполнено ', bold: `${result.success_count} из ${total}`, suffix: `. Не удалось: ${failed}`, type: 'warning' });
      } else {
        useDeletionsStore().notify({ prefix: `${label}: `, bold: String(result.success_count) });
      }
      this.clearSelection();
      this.refreshData();
      return true;
    },

    // fix 5: сбрасываем поднятые dirty-флаги детей при смене/сбросе выбора -
    // при уходе с активной сущности дети размонтируются и больше не эмитят.
    resetChildDirty() {
      this.placesDirty = false;
      this.tablesDirty = false;
      this.responsiblesDirty = false;
      this.responsiblesCount = 0;
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
      if (this.isAdding) return;
      // Молча выходить нельзя: сюда приходит не только кнопка (она заблокирована
      // при пустых полях), но и «Сохранить все изменения» из диалога несохранённого.
      // Оттуда тихий выход выглядел как «нажал и ничего не произошло»: форма
      // оставалась грязной, и диалог считал сохранение неудавшимся.
      if (!name || !type) {
        this.addError = !name
          ? 'Укажите название - без него организацию не создать.'
          : 'Выберите тип - без него организацию не создать.';
        // Ошибку в форме не видно, когда сохранение пришло из диалога
        // несохранённого: он лежит выше окна и перекрывает её. Тост -
        // единственный слой поверх диалога.
        useDeletionsStore().notify({ prefix: this.addError, type: 'error' });
        return;
      }
      this.isAdding = true;
      this.addError = '';

      const result = await this.createOrganization({ name, type }, { includeArchived: true });

      if (result.ok) {
        this.forceCloseAdd();
        // Созданная вручную запись сразу проверенная - в архиве и «на проверке» её нет.
        if (this.listMode !== 'active') this.listMode = 'active';
        const created = this.organizationsWithUsers.find(org => org.id === result.data.id);
        if (created) {
          this.selectedOrganization = { ...created };
          this.originalSelectedName = created.name;
          this.originalSelectedType = created.type ?? null;
          this.loadMembers(created.id);
        }
        useDeletionsStore().notify({ prefix: 'Организация ', bold: name, suffix: ' создана' });
      } else {
        this.addError = result.message || 'Не удалось создать организацию';
      }
      this.isAdding = false;
    },

    async selectOrganization(org) {
      if (this.selectedOrganization && this.selectedOrganization.id === org.id) return;
      if (this.detailsAreaDirty && !(await confirmIfAnyDirty())) return;
      this.resetChildDirty();
      this.selectedOrganization = { ...org };
      this.originalSelectedName = org.name;
      this.originalSelectedType = org.type ?? null;
      this.detailError = '';
      this.loadMembers(org.id);
    },

    onDetailTypeChange(value) {
      if (this.selectedOrganization) this.selectedOrganization.type = value;
    },

    async saveSelectedName() {
      if (!this.isDetailsDirty || this.isSavingName) return;
      const name = this.selectedOrganization.name.trim();
      const type = this.selectedOrganization.type ?? null;
      this.isSavingName = true;
      this.detailError = '';

      const result = await this.updateOrganization(this.selectedOrganization.id, { name, type }, { includeArchived: true });

      if (result.ok) {
        this.originalSelectedName = name;
        this.originalSelectedType = type;
        this.selectedOrganization.name = name;
        useDeletionsStore().notify({ prefix: 'Изменения сохранены в ', bold: name });
      } else {
        this.detailError = result.message || 'Не удалось сохранить';
      }
      this.isSavingName = false;
    },

    async loadMembers(id) {
      // seq-токен: быстрое переключение организаций может дать гонку, показываем
      // только ответ последнего запроса (см. урок про авто-fetch по выбору).
      const seq = ++this.membersSeq;
      this.members = [];
      this.membersLoading = true;
      try {
        const data = await getOrganizationMembers(id);
        if (seq !== this.membersSeq) return;
        this.members = Array.isArray(data) ? data : [];
      } catch {
        // Список участников - вспомогательная информация, при сбое оставляем пустым.
        if (seq === this.membersSeq) this.members = [];
      } finally {
        if (seq === this.membersSeq) this.membersLoading = false;
      }
    },

    openReassign() {
      this.reassignTargetId = null;
      this.reassignVisible = true;
    },

    closeReassign() {
      // Пока перенос летит, модалку не закрываем (её оверлей блокирует список -
      // иначе смена организации дала бы гонку loadMembers, как у closeBulkModal).
      if (this.reassignSubmitting) return;
      this.reassignVisible = false;
    },

    async performReassign() {
      if (!this.reassignTargetId || this.reassignSubmitting || !this.selectedOrganization) return;
      const source = this.selectedOrganization;
      const target = this.reassignTargetOptions.find(o => o.value === this.reassignTargetId);
      this.reassignSubmitting = true;
      try {
        const data = await reassignOrganizationUsers(source.id, this.reassignTargetId);
        const n = data?.reassigned ?? 0;
        this.reassignVisible = false;
        useDeletionsStore().notify({
          prefix: 'Перенесено ',
          bold: `${n} ${this.usersPlural(n)}`,
          suffix: ` в «${target ? target.label : ''}»`,
        });
        // Перечитываем блокеров: источник освобождён, список пуст -> организацию
        // можно архивировать. И обновляем user_count в списке: у источника он упал,
        // у цели вырос (тот же предикат активных участников).
        this.loadMembers(source.id);
        this.fetchOrganizationsWithUsers(true);
      } catch (e) {
        useDeletionsStore().notify({ prefix: 'Не удалось перенести: ', bold: e.message || 'ошибка', type: 'error' });
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

    memberFullName(m) {
      const parts = [m.last_name, m.first_name, m.middle_name].filter(Boolean);
      return parts.join(' ') || m.username || '—';
    },

    memberInitials(m) {
      const a = (m.last_name || m.first_name || m.username || '').trim().charAt(0);
      const b = (m.last_name ? m.first_name : '') || '';
      return ((a + b.charAt(0)).toUpperCase()) || '?';
    },

    orgTypeLabel(type) {
      return type || this.unspecifiedTypeLabel;
    },

    /**
     * Организация заведена подачей заявки и ещё не разобрана (#1437). Сверяем с
     * реальным значением поля, а не с отсутствием approved: у записей, пришедших
     * из выборок без moderation_status, поле пустое - «неизвестно» бейджа не даёт.
     * @param {{ moderation_status?: string }} org
     * @returns {boolean}
     */
    isPendingModeration(org) {
      return org?.moderation_status === 'pending';
    },

    /**
     * Запись разобрана: перечитываем список и переводим выбор на её итог. При привязке
     * (merge) исходная запись физически удалена, а `id` в событии - уже цель привязки,
     * поэтому ищем по нему, а не по прежнему `selectedOrganization.id`. Не нашли или
     * итог выпал из текущего режима (подтвердили запись, стоя в «На проверке») - гасим
     * детали, иначе панель осталась бы на мёртвом выборе.
     * @param {{ kind: string, id: number|null, name: string }} result
     */
    async onModerationResolved(result) {
      await this.fetchOrganizationsWithUsers(true);

      const alive = new Set(this.organizationsWithUsers.map(o => o.id));
      this.selectedIds = this.selectedIds.filter(id => alive.has(id));
      if (this.lastSelectedId != null && !alive.has(this.lastSelectedId)) this.lastSelectedId = null;

      this.resetChildDirty();
      this.detailError = '';

      const resolved = result?.id != null
        ? this.organizationsWithUsers.find(o => o.id === result.id)
        : null;
      if (!resolved || !this.matchesListMode(resolved)) {
        this.selectedOrganization = null;
        this.members = [];
        return;
      }

      this.selectedOrganization = { ...resolved };
      this.originalSelectedName = resolved.name;
      this.originalSelectedType = resolved.type ?? null;
      this.loadMembers(resolved.id);
    },

    onArchiveClick(org) {
      this.archiveConfirmOrg = org;
    },

    async performArchive() {
      const org = this.archiveConfirmOrg;
      this.archiveConfirmOrg = null;
      if (!org) return;

      const result = await this.deleteOrganization(org.id, { includeArchived: true });

      if (result.ok) {
        if (this.selectedOrganization && this.selectedOrganization.id === org.id && !this.showArchive) {
          this.selectedOrganization = null;
          this.resetChildDirty();
        }
        useDeletionsStore().notify({ prefix: 'Организация ', bold: org.name, suffix: ' архивирована' });
      } else {
        useDeletionsStore().notify({ prefix: 'Не удалось архивировать: ', bold: result.message || 'ошибка', type: 'error' });
      }
    },

    async onRestore(org) {
      const result = await this.restoreOrganization(org.id, { includeArchived: true });

      if (result.ok) {
        if (this.selectedOrganization && this.selectedOrganization.id === org.id && this.showArchive) {
          this.selectedOrganization = null;
          this.resetChildDirty();
        }
        useDeletionsStore().notify({ prefix: 'Организация ', bold: org.name, suffix: ' восстановлена из архива' });
      } else {
        useDeletionsStore().notify({ prefix: 'Не удалось восстановить: ', bold: result.message || 'ошибка', type: 'error' });
      }
    },

    openHistory(org) {
      this.historyOrg = org;
    },

    async fetchCurrentUser() {
      this.currentUserName = await fetchCurrentUserName();
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
      this.fetchOrganizationsWithUsers(true);
    },

    handlePlacesUpdated() {
      this.fetchOrganizationsWithUsers(true);
    },

    handleTablesUpdated() {
      this.fetchOrganizationsWithUsers(true);
    },
  }
};
</script>

<style scoped>
@import '@/assets/directory-management.css';
@import '@/assets/directory-bulk-bar.css';

.organizations-management {
  position: relative;
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
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* Панель групповых операций - оверлей поверх шапки, не двигает контент (reflow).
   Высота = высоте .management-header (50px), карточка = position:relative. */
.bulk-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 6;
  height: 50px;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  background: var(--accent-tint-solid);
  /* фиксированная высота оверлея + перенос кнопок = наезд на таблицу на узкой
     карточке (expanded-nav ~800-965px). Держим одну строку, узко - горизонтальный скролл. */
  overflow-x: auto;
  overflow-y: hidden;
}

.bulk-slide-enter-active,
.bulk-slide-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.bulk-slide-enter-from,
.bulk-slide-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

.content-container {
  display: flex;
  height: 540px;
  width: 100%;
}

.table-section {
  width: 40%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--border);
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

.sort-icon {
  color: var(--text-muted);
  width: 12px;
  height: 12px;
  transition: .2s;
}

.check-col {
  width: 6%;
  min-width: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 8px;
  cursor: default;
}

.header-col.check-col:hover {
  color: var(--text-muted);
}

.id-col {
  width: 10%;
  min-width: 36px;
}

.name-col {
  width: 36%;
  min-width: 140px;
  /* Наименование усекается, бейдж «на проверке» - нет: flex с min-width:0 у имени
     отдаёт остаток бейджу, а не режет его многоточием вместе с именем (#1437). */
  display: flex;
  align-items: center;
}

.name-col .truncate-text {
  min-width: 0;
}

.type-col {
  width: 24%;
  min-width: 90px;
}

.users-col {
  width: 24%;
  min-width: 90px;
}

.type-value {
  font-size: 13px;
  color: var(--text);
}

.type-unspecified {
  color: var(--text-muted);
  font-style: italic;
}

.table-row.inactive .type-value {
  color: var(--text-muted);
}

.table-body {
  flex: 1;
  overflow-y: auto;
}

.table-row.inactive .id-value,
.table-row.inactive .count-value {
  color: var(--text-muted);
}

.user-count {
  display: flex;
  align-items: center;
  gap: 6px;
}

.count-value {
  font-weight: 600;
  color: var(--text);
}

/* Бейдж записи «на проверке» (#1437) - тот же вид, что у плашки разбора в детали
   заявки (ApplicationOrgModeration): тёплая подложка вместо серой архивной. */
.moderation-badge {
  flex: none;
  margin-left: 6px;
  padding: 2px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--warning) 22%, var(--surface));
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}

.no-results-inline {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-muted);
  width: 100%;
}

.orgs-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 0;
}

.table-footer {
  margin-top: auto;
  padding: 6px 20px;
  border-top: 1px solid var(--border);
  text-align: end;
  background: var(--surface-2);
}

/* ===== Детали: рабочая область (вариант 1) ===== */
.details-column {
  flex: 1;
  min-width: 0;
  background: var(--surface);
  display: flex;
  flex-direction: column;
}

.v1 {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 18px 22px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

/* хедер деталей */
.d-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.d-head-info {
  min-width: 0;
}

.d-title {
  margin: 0;
  font-size: 1.3em;
  font-weight: 700;
  color: var(--accent-text);
  word-break: break-word;
}

.d-meta {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-top: 8px;
  flex-wrap: wrap;
}

.d-acts {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-shrink: 0;
}

.muted-hint {
  color: var(--text-muted);
  font-size: 0.82em;
}

/* pill-кнопки (эталон мокапа) */
.pill {
  display: inline-flex;
  align-items: center;
  height: 30px;
  padding: 0 14px;
  border-radius: 50px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  border: none;
  font-family: inherit;
  white-space: nowrap;
  transition: background 0.2s, border-color 0.2s;
}

.pill-type {
  background: var(--surface-2);
  color: var(--accent-text);
  cursor: default;
}

/* карточка-секция */
.card {
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 16px;
  background: var(--surface-sunken);
}

.sec-title {
  font-size: 0.82em;
  font-weight: 700;
  color: var(--accent-text);
  letter-spacing: 0.02em;
  text-transform: uppercase;
  margin: 0 0 12px;
  display: flex;
  align-items: center;
  /* единая высота шапок карточек (совпадает с sec-title детей, где появляются
     кнопки) - чтобы стек карточек не дёргался */
  min-height: 28px;
  gap: 8px;
}

.count-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 22px;
  height: 20px;
  padding: 0 7px;
  border-radius: 50px;
  background: var(--surface);
  color: var(--accent-text);
  font-size: 11px;
  font-weight: 700;
}

/* карточка "Основное" */
.basic {
  display: grid;
  grid-template-columns: 1fr 230px;
  gap: 14px;
  align-items: end;
}

.basic-field {
  min-width: 0;
}

.field-label {
  font-size: 0.78em;
  color: var(--text-muted);
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
  display: block;
  margin-bottom: 6px;
}

/* Объяснение к заблокированному наименованию. Лежит под сеткой полей, а не внутри
   ячейки: у .basic align-items:end, и выросшая ячейка утащила бы дропдаун типа вниз. */
.lock-note {
  margin: 10px 0 0;
  color: var(--text-muted);
  font-size: 0.85em;
  line-height: 1.45;
}

.save-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 14px;
}

.form-error {
  color: var(--danger-text);
  font-size: 0.85em;
  margin-top: 10px;
}

.archive-hint {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.9em;
  line-height: 1.5;
}

/* привязанные пользователи */
.stack {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.person {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid color-mix(in srgb, var(--accent) 25%, var(--surface));
  border-radius: 12px;
  background: var(--surface);
}

.avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--surface-2);
  color: var(--accent-text);
  font-weight: 700;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.who {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.who b {
  font-size: 13px;
  font-weight: 600;
  color: var(--accent-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.who small {
  font-size: 11px;
  color: var(--text-muted);
}

.members-loading {
  display: flex;
  justify-content: center;
  padding: 12px 0;
}

.members-empty {
  margin: 0;
  font-size: 0.85em;
  color: var(--text-muted);
}

/* блокеры архивации: уведомление + кнопка переноса */
.blocking-notice {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 12px;
  padding: 10px 12px;
  border: 1px solid var(--color-border);
  background: var(--color-primary-tint);
  border-radius: var(--radius-md);
}

.blocking-notice__text {
  flex: 1 1 220px;
  min-width: 0;
  font-size: 0.85em;
  color: var(--text-muted);
  line-height: 1.45;
}

.blocking-notice__btn {
  flex-shrink: 0;
}

/* модалка переноса блокеров */
.reassign-intro {
  margin: 0 0 14px;
  font-size: 0.9em;
  color: var(--text-muted);
  line-height: 1.5;
}

.reassign-empty {
  margin: 10px 0 0;
  font-size: 0.85em;
  color: var(--danger-text);
}

.no-selection-message {
  color: var(--text-muted);
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
/* Окно, затемнение, анимация и закрытие живут в BaseModal. Здесь остаются только
   отступы содержимого: base-modal__body идёт без padding, их несёт содержимое. */
.orgs-modal-body {
  padding: 22px 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

@media (max-width: 968px) {
  .content-container {
    flex-direction: column;
    height: auto;
  }

  .table-section,
  .details-column {
    width: 100% !important;
  }

  .table-section {
    border-right: none;
    border-bottom: 1px solid var(--border);
    height: 255px;
  }

  .basic {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 767.98px) {
  /* Направление/высоту шапки берёт на себя глобальный .rt-header-inline
     (responsive-tables.css, !important - перебивает scoped-специфичность).
     Здесь только сужаем дропдауны/поиск, чтобы 5 контролов уместились в строку. */
  .management-header {
    padding: 10px var(--gutter, 16px);
  }

  .header-controls {
    flex-wrap: wrap;
    row-gap: 8px;
  }

  .archive-dropdown,
  .type-filter-dropdown {
    min-width: 92px;
  }

  :deep(.search) {
    width: 110px;
  }

  /* rt-header-inline может сделать шапку auto-высоты (перенос controls строкой
     ниже) - фиксированный оверлей bulk-bar (height:50px) больше не накрывает
     её целиком, возвращаем панель в обычный поток. */
  .bulk-bar {
    position: static;
    height: auto;
    padding: 12px 16px;
    overflow-x: visible;
  }

  .bulk-actions {
    flex-wrap: wrap;
  }

  /* Тач-таргет чекбокса выбора строки в card-режиме. */
  .check-col {
    min-height: 44px;
  }

  .v1 {
    padding: 16px;
  }

  .d-head {
    flex-direction: column;
    gap: 12px;
    align-items: flex-start;
  }

  .d-acts {
    align-self: flex-end;
  }
}
</style>
