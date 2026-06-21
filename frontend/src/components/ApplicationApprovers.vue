<template>
  <div class="approvers-container dashboard-card">
    <div class="management-header">
      <h3 class="management-title">
        Принимающие заявки
      </h3>
      <div class="header-controls">
        <SearchComponent
          v-model="searchQuery"
          :title="'Поиск принимающих...'"
        />
        <button
          class="lk-button lk-button--secondary"
          data-testid="approver-history"
          @click="openHistory"
        >
          История
        </button>
        <button
          class="add-header-button"
          :disabled="isAdding"
          @click="openAddModal"
        >
          Добавить принимающего
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
        :class="{ 'with-details': selectedApprover }"
      >
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
                :class="{ sorted: sortField === 'id', desc: sortField === 'id' && sortDirection === 'desc' }"
              >
            </div>
            <div
              class="header-col name-col"
              @click="sortBy('full_name')"
            >
              <p :class="{ 'active-sort': sortField === 'full_name' }">
                ФИО
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{ sorted: sortField === 'full_name', desc: sortField === 'full_name' && sortDirection === 'desc' }"
              >
            </div>
            <div
              class="header-col date-col"
              @click="sortBy('created_at')"
            >
              <p :class="{ 'active-sort': sortField === 'created_at' }">
                Добавлен
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{ sorted: sortField === 'created_at', desc: sortField === 'created_at' && sortDirection === 'desc' }"
              >
            </div>
          </div>

          <div class="table-body">
            <div
              v-for="approver in sortedApprovers"
              :key="approver.id"
              class="table-row"
              :class="{ selected: selectedApprover && selectedApprover.id === approver.id }"
              @click="selectApprover(approver)"
            >
              <div class="table-col id-col">
                <span class="cell-content id-value">{{ approver.id }}</span>
              </div>
              <div class="table-col name-col">
                <span
                  class="truncate-text"
                  :title="getFullName(approver)"
                >
                  {{ getFullName(approver) }}
                </span>
              </div>
              <div class="table-col date-col">
                <span class="cell-content">{{ formatDate(approver.created_at) }}</span>
              </div>
            </div>

            <div
              v-if="!sortedApprovers.length && !isLoading"
              class="no-results"
            >
              {{ searchQuery.trim() ? 'Ничего не найдено по запросу' : 'Принимающих пока нет' }}
            </div>
          </div>

          <div class="table-footer">
            <span class="items-count">
              Всего: {{ filteredApprovers.length }}
            </span>
          </div>
        </div>
      </div>

      <div
        v-if="selectedApprover"
        class="details-section"
      >
        <div class="tab-content">
          <div class="details-header">
            <div class="details-title-wrapper">
              <h3 class="details-title">
                {{ getFullName(selectedApprover) }}
              </h3>
            </div>
            <div class="details-header-actions">
              <button
                class="action-btn delete-action-btn"
                title="Удалить принимающего"
                @click="removeApproverWithUndo(selectedApprover)"
              >
                Удалить
              </button>
            </div>
          </div>

          <div class="details-body">
            <div class="info-row">
              <span class="info-label">Должность:</span>
              <span class="info-value">{{ selectedApprover.position || '—' }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">Организация:</span>
              <span class="info-value">{{ selectedApprover.organization || '—' }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">Компания:</span>
              <span class="info-value">{{ selectedApprover.company || '—' }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">Добавлен:</span>
              <span class="info-value">{{ formatDate(selectedApprover.created_at) }}</span>
            </div>

            <div class="details-meta">
              <span>ID: {{ selectedApprover.id }}</span>
            </div>
          </div>
        </div>
      </div>

      <div
        v-else
        class="no-selection-message"
      >
        <p>Выберите принимающего для просмотра</p>
      </div>
    </div>

    <!-- Модалка добавления принимающих -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showAddModal"
          class="modal-overlay"
          @mousedown="onOverlayMousedown"
          @mouseup="onOverlayMouseup"
        >
          <div
            class="approvers-modal"
            @mousedown.stop
          >
            <div class="modal-header">
              <h3>Добавить принимающего</h3>
              <button
                class="modal-close"
                aria-label="Закрыть"
                @click="requestCloseAdd"
              >
                ×
              </button>
            </div>

            <div class="modal-body">
              <div class="user-search-section">
                <input
                  ref="searchInput"
                  v-model="userSearchQuery"
                  class="lk-input"
                  placeholder="Поиск пользователей..."
                  type="text"
                  autocomplete="off"
                  @input="onUserSearchInput"
                  @focus="showUserDropdown = true"
                  @blur="onSearchBlur"
                >
                <div
                  v-if="showUserDropdown && filteredAvailableUsers.length > 0"
                  class="user-dropdown"
                >
                  <div class="user-dropdown-content">
                    <div
                      v-for="user in filteredAvailableUsers"
                      :key="user.id"
                      class="user-item"
                      @mousedown.prevent="addUserToSelection(user)"
                    >
                      <div class="user-info">
                        <div class="user-name">
                          {{ getFullName(user) }}
                        </div>
                        <div class="user-details">
                          <span class="user-username">@{{ user.username }}</span>
                          <span
                            v-if="user.position"
                            class="user-position"
                          >{{ user.position }}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
                <div
                  v-else-if="showUserDropdown && userSearchQuery.trim() && filteredAvailableUsers.length === 0"
                  class="user-dropdown"
                >
                  <div class="user-dropdown-content">
                    <div class="no-results-message">
                      Нет доступных пользователей
                    </div>
                  </div>
                </div>
              </div>

              <div class="selected-users">
                <div class="selected-users-header">
                  <span>Выбрано пользователей:</span>
                  <span class="selected-count">
                    {{ selectedUsers.length }}
                  </span>
                </div>

                <div class="users-list-container">
                  <div class="users-list">
                    <div
                      v-for="user in selectedUsers"
                      :key="user.id"
                      class="selected-user"
                    >
                      <div class="selected-user-info">
                        <span class="selected-user-name">{{ getFullName(user) }}</span>
                        <span class="selected-user-username">@{{ user.username }}</span>
                        <span
                          v-if="user.position"
                          class="selected-user-position"
                        >{{ user.position }}</span>
                      </div>
                      <button
                        class="remove-user-btn"
                        title="Убрать из списка"
                        @click="removeUserFromSelection(user)"
                      >
                        ×
                      </button>
                    </div>
                    <div
                      v-if="!selectedUsers.length"
                      class="no-selected-hint"
                    >
                      Выберите пользователей из поиска выше
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="modal-footer">
              <button
                class="lk-button lk-button--ghost"
                @click="requestCloseAdd"
              >
                Отмена
              </button>
              <button
                class="lk-button lk-button--primary"
                :disabled="selectedUsers.length === 0 || isAdding"
                @click="submitAdd"
              >
                {{ isAdding ? 'Добавление...' : `Добавить (${selectedUsers.length})` }}
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <ApplicationApproverHistoryModal
      v-if="showHistory"
      :current-user-name="currentUserName"
      @close="showHistory = false"
    />
  </div>
</template>

<script>
import SearchComponent from './SearchComponent.vue';
import RefreshButton from './RefreshButton.vue';
import ApplicationApproverHistoryModal from './ApplicationApproverHistoryModal.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useOverlayClose } from '@/composables/useOverlayClose';
import { apiRequest } from '@/api/client';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import { getApprovers, getAllUsers, addApprover, deleteApprover } from '@/api/approvers';

export default {
  name: 'ApplicationApproversManagement',
  components: { SearchComponent, RefreshButton, ApplicationApproverHistoryModal },
  setup() {
    const overlay = { close: () => {} };
    const { onOverlayMousedown, onOverlayMouseup } = useOverlayClose(() => overlay.close());
    return { onOverlayMousedown, onOverlayMouseup, overlay };
  },
  data() {
    return {
      approvers: [],
      allUsers: [],
      searchQuery: '',
      sortField: 'full_name',
      sortDirection: 'asc',
      isLoading: false,
      selectedApprover: null,
      pendingDeleteIds: [],
      showAddModal: false,
      userSearchQuery: '',
      showUserDropdown: false,
      selectedUsers: [],
      isAdding: false,
      showHistory: false,
      currentUserName: '',
    };
  },
  computed: {
    filteredApprovers() {
      const variants = buildSearchVariants(this.searchQuery);
      let list = this.approvers.filter(a => !this.pendingDeleteIds.includes(a.id));
      if (variants.length) {
        list = list.filter(a => matchesSearch(this.getFullName(a), variants));
      }
      return list;
    },
    sortedApprovers() {
      const arr = [...this.filteredApprovers];
      return arr.sort((a, b) => {
        let va, vb;
        if (this.sortField === 'id') {
          va = a.id;
          vb = b.id;
        } else if (this.sortField === 'full_name') {
          va = this.getFullName(a);
          vb = this.getFullName(b);
        } else {
          va = a.created_at || '';
          vb = b.created_at || '';
        }
        if (va < vb) return this.sortDirection === 'asc' ? -1 : 1;
        if (va > vb) return this.sortDirection === 'asc' ? 1 : -1;
        return 0;
      });
    },
    approverUserIds() {
      return this.approvers.map(a => a.user_id);
    },
    selectedUserIds() {
      return this.selectedUsers.map(u => u.id);
    },
    availableUsers() {
      return this.allUsers.filter(
        u => !this.approverUserIds.includes(u.id) && !this.selectedUserIds.includes(u.id),
      );
    },
    filteredAvailableUsers() {
      const variants = buildSearchVariants(this.userSearchQuery);
      if (!variants.length) return this.availableUsers.slice(0, 10);
      return this.availableUsers.filter(u => matchesSearch(
        `${this.getFullName(u)} ${u.username} ${u.position || ''}`,
        variants,
      )).slice(0, 10);
    },
  },
  created() {
    this.overlay.close = () => { this.requestCloseAdd(); };
  },
  mounted() {
    this.refresh();
    this.fetchCurrentUser();
    document.addEventListener('keydown', this.onKeydown);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.onKeydown);
  },
  methods: {
    onKeydown(e) {
      if (e.key === 'Escape' && this.showAddModal) this.requestCloseAdd();
    },
    getFullName(user) {
      const parts = [user.last_name, user.first_name, user.middle_name].filter(Boolean);
      return parts.length > 0 ? parts.join(' ') : (user.username || String(user.id));
    },
    formatDate(s) {
      if (!s) return '—';
      return new Date(s).toLocaleString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
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
    selectApprover(approver) {
      this.selectedApprover = approver;
    },
    openHistory() {
      this.showHistory = true;
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
    async refresh() {
      this.isLoading = true;
      try {
        const [approvers, users] = await Promise.all([
          getApprovers(),
          getAllUsers(),
        ]);
        this.approvers = Array.isArray(approvers) ? approvers : [];
        this.allUsers = Array.isArray(users) ? users : [];
        if (this.selectedApprover) {
          const fresh = this.approvers.find(a => a.id === this.selectedApprover.id);
          if (fresh) {
            this.selectedApprover = fresh;
          } else if (!this.pendingDeleteIds.includes(this.selectedApprover.id)) {
            this.selectedApprover = null;
          }
        }
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось загрузить ', bold: 'принимающих', type: 'error' });
      } finally {
        this.isLoading = false;
      }
    },
    removeApproverWithUndo(approver) {
      if (this.pendingDeleteIds.includes(approver.id)) return;
      const id = approver.id;
      const fullName = this.getFullName(approver);
      this.pendingDeleteIds.push(id);
      if (this.selectedApprover && this.selectedApprover.id === id) {
        this.selectedApprover = null;
      }
      useDeletionsStore().enqueue({
        prefix: 'Принимающий ',
        bold: fullName,
        suffix: ' удалён',
        onConfirm: () => this.commitDelete(id),
        onUndo: () => this.unhidePending(id),
      });
    },
    unhidePending(id) {
      this.pendingDeleteIds = this.pendingDeleteIds.filter(i => i !== id);
    },
    async commitDelete(id) {
      try {
        await deleteApprover(id);
      } catch {
        this.unhidePending(id);
        useDeletionsStore().notify({ prefix: 'Не удалось удалить принимающего', type: 'error' });
        return;
      }
      this.unhidePending(id);
      await this.refresh();
    },
    openAddModal() {
      this.showAddModal = true;
      this.userSearchQuery = '';
      this.selectedUsers = [];
      this.showUserDropdown = false;
    },
    requestCloseAdd() {
      this.showAddModal = false;
      this.userSearchQuery = '';
      this.selectedUsers = [];
      this.showUserDropdown = false;
    },
    onUserSearchInput() {
      this.showUserDropdown = true;
    },
    onSearchBlur() {
      setTimeout(() => {
        this.showUserDropdown = false;
      }, 200);
    },
    addUserToSelection(user) {
      this.selectedUsers.push(user);
      this.userSearchQuery = '';
      this.showUserDropdown = false;
      this.$refs.searchInput?.blur();
    },
    removeUserFromSelection(user) {
      this.selectedUsers = this.selectedUsers.filter(u => u.id !== user.id);
    },
    async submitAdd() {
      if (this.selectedUsers.length === 0 || this.isAdding) return;
      this.isAdding = true;
      try {
        const results = await Promise.allSettled(
          this.selectedUsers.map(u => addApprover(u.id)),
        );
        const successCount = results.filter(r => r.status === 'fulfilled').length;
        const errorCount = results.length - successCount;
        this.requestCloseAdd();
        await this.refresh();
        if (errorCount === 0) {
          useDeletionsStore().notify({ prefix: 'Добавлено принимающих: ', bold: String(successCount) });
        } else if (successCount > 0) {
          useDeletionsStore().notify({
            prefix: `Добавлено ${successCount}, не удалось: `,
            bold: String(errorCount),
            type: 'warning',
          });
        } else {
          useDeletionsStore().notify({ prefix: 'Не удалось добавить принимающих', type: 'error' });
        }
      } finally {
        this.isAdding = false;
      }
    },
  },
};
</script>

<style scoped>
.approvers-container {
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
  gap: 12px;
}

.management-title {
  margin: 0;
  font-size: 1.2em;
  font-weight: 600;
  color: #000;
}

.header-controls {
  display: flex;
  gap: 10px;
  align-items: center;
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

/* Master-detail layout */
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
  border-right: 1px solid #e6e6e6;
  background: #fff;
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
  transition: 0.2s;
  cursor: pointer;
  user-select: none;
}

.header-col p {
  margin: 0;
}

.header-col:hover {
  color: #000;
}

.header-col:hover .sort-icon {
  filter: brightness(0);
}

.sort-icon {
  width: 12px;
  height: 12px;
  transition: 0.2s;
}

.sort-icon.sorted {
  filter: brightness(0);
}

.sort-icon.desc {
  transform: rotate(180deg);
}

.active-sort {
  color: #000 !important;
  font-weight: 600 !important;
}

.id-col {
  width: 15%;
  min-width: 55px;
}

.name-col {
  width: 55%;
  min-width: 160px;
}

.date-col {
  width: 30%;
  min-width: 110px;
}

.table-body {
  flex: 1;
  min-height: 0;
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
  background-color: #f8f9ff;
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

.truncate-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
  display: block;
}

.no-results {
  text-align: center;
  padding: 40px 20px;
  color: #a2a2a2;
  width: 100%;
}

.table-footer {
  padding: 6px 20px;
  border-top: 1px solid #e6e6e6;
  text-align: right;
  background: #f8fafc;
}

.items-count {
  font-size: 12px;
  color: #a2a2a2;
  font-weight: 500;
}

/* Details */
.details-section {
  width: 60%;
  display: flex;
  flex-direction: column;
  background: #fff;
  overflow: hidden;
}

.tab-content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background: #fff;
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
  gap: 12px;
  flex-wrap: wrap;
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

.delete-action-btn {
  background: #fff;
  color: #dc3545;
  border: 1px solid #fecaca;
}

.delete-action-btn:hover {
  background: #fff1f2;
  border-color: #dc3545;
}

.details-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.info-row {
  display: flex;
  padding: 8px 0;
  border-bottom: 1px solid #f0f0f0;
  gap: 16px;
}

.info-row:last-child {
  border-bottom: none;
}

.info-label {
  width: 120px;
  font-size: 0.85em;
  color: #a2a2a2;
  flex-shrink: 0;
}

.info-value {
  flex: 1;
  font-size: 0.95em;
  color: #000;
}

.details-meta {
  display: flex;
  gap: 16px;
  margin-top: 12px;
  font-size: 12px;
  color: #a2a2a2;
}

.no-selection-message {
  width: 60%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a2a2a2;
  font-weight: 400;
  font-size: 14px;
}

/* Модалка */
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

.approvers-modal {
  width: 100%;
  max-width: 480px;
  background: #fff;
  border-radius: 30px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
  overflow: hidden;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 18px 24px;
  border-bottom: 1px solid #e6e6e6;
  flex-shrink: 0;
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
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 24px;
  border-top: 1px solid #e6e6e6;
  flex-shrink: 0;
}

/* Поиск пользователей в модалке */
.user-search-section {
  position: relative;
  flex-shrink: 0;
}

.user-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: white;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  max-height: 250px;
  overflow-y: auto;
  z-index: 1000;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  margin-top: 4px;
}

.user-dropdown-content {
  max-height: 250px;
  overflow-y: auto;
}

.user-item {
  padding: 8px 12px;
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.user-item:last-child {
  border-bottom: none;
}

.user-item:hover {
  background-color: #f8f9ff;
}

.user-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.user-name {
  font-weight: 500;
  font-size: 14px;
  color: #000;
}

.user-details {
  display: flex;
  gap: 8px;
  font-size: 12px;
  color: #666;
}

.user-username {
  color: #4F5BDF;
}

.user-position {
  color: #999;
}

.no-results-message {
  padding: 12px;
  text-align: center;
  color: #999;
  font-size: 14px;
}

/* Выбранные пользователи */
.selected-users {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.selected-users-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 14px;
  color: #666;
  flex-shrink: 0;
}

.selected-count {
  background: #4F5BDF;
  color: white;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
}

.users-list-container {
  overflow-y: auto;
  max-height: 200px;
  border: 1px solid #e6e6e6;
  border-radius: 15px;
  background: #fafafa;
}

.users-list {
  display: flex;
  flex-direction: column;
}

.selected-user {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  border-bottom: 1px solid #e6e6e6;
  transition: background-color 0.15s ease;
}

.selected-user:last-child {
  border-bottom: none;
}

.selected-user:hover {
  background-color: #f0f0f0;
}

.selected-user-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.selected-user-name {
  font-weight: 500;
  font-size: 14px;
  color: #000;
}

.selected-user-username {
  font-size: 12px;
  color: #4F5BDF;
}

.selected-user-position {
  font-size: 11px;
  color: #999;
  margin-top: 2px;
}

.no-selected-hint {
  padding: 16px 12px;
  text-align: center;
  color: #a2a2a2;
  font-size: 13px;
}

.remove-user-btn {
  background: none;
  border: none;
  color: #999;
  font-size: 18px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 4px;
  transition: all 0.15s ease;
}

.remove-user-btn:hover {
  background-color: #fee2e2;
  color: #ef4444;
}

/* Анимация открытия/закрытия (зеркало CitizenshipManagement) */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: all 0.25s ease;
}

.modal-fade-enter-active .approvers-modal,
.modal-fade-leave-active .approvers-modal {
  transition: all 0.25s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  background: rgba(0, 0, 0, 0);
}

.modal-fade-enter-from .approvers-modal,
.modal-fade-leave-to .approvers-modal {
  opacity: 0;
  transform: translateY(20px);
}

/* Скроллбары */
.user-dropdown-content::-webkit-scrollbar,
.users-list-container::-webkit-scrollbar,
.table-body::-webkit-scrollbar,
.modal-body::-webkit-scrollbar {
  width: 4px;
}

.user-dropdown-content::-webkit-scrollbar-track,
.users-list-container::-webkit-scrollbar-track,
.table-body::-webkit-scrollbar-track,
.modal-body::-webkit-scrollbar-track {
  background: #f1f1f1;
}

.user-dropdown-content::-webkit-scrollbar-thumb,
.users-list-container::-webkit-scrollbar-thumb,
.table-body::-webkit-scrollbar-thumb,
.modal-body::-webkit-scrollbar-thumb {
  background: #ccc;
  border-radius: 4px;
}

@media (max-width: 768px) {
  .management-header {
    flex-direction: column;
    align-items: flex-start;
    height: auto;
    padding: 16px;
  }
  .header-controls {
    width: 100%;
    flex-direction: column;
    align-items: stretch;
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
    border-bottom: 1px solid #e6e6e6;
  }
  .table-body {
    max-height: 300px;
  }
}
</style>
