<!-- ForwardModal.vue -->
<template>
  <BaseModal
    :show="show"
    title="Переслать заявку"
    width="600px"
    :z-index="20000"
    content-class="forward-modal"
    @close="close"
  >
    <div class="forward-body">
      <div class="user-search-section">
        <input
          ref="searchInput"
          v-model="searchQuery"
          class="lk-input"
          data-testid="forward-modal-search"
          placeholder="Поиск пользователей..."
          type="text"
          @input="searchUsers"
          @focus="onSearchFocus"
          @blur="onSearchBlur"
        >
        <div
          v-if="showDropdown && filteredUsers.length > 0"
          class="forward-user-dropdown"
        >
          <div class="forward-user-dropdown-content">
            <div
              v-for="user in filteredUsers"
              :key="user.username"
              class="forward-user-item"
              data-testid="forward-modal-user-option"
              @mousedown.prevent="addUser(user)"
            >
              <div class="forward-user-info">
                <div class="forward-user-main">
                  <span class="forward-user-name">{{ getUserDisplayName(user) }}</span>
                  <span class="forward-user-username">@{{ user.username }}</span>
                </div>
                <div class="forward-user-details">
                  <span
                    v-if="user.position"
                    class="forward-user-position"
                  >{{ user.position }}</span>
                  <span
                    v-if="user.organization"
                    class="forward-user-organization"
                  >{{ user.organization }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div
          v-else-if="showDropdown && filteredUsers.length === 0"
          class="forward-user-dropdown no-results"
        >
          <div class="forward-user-dropdown-content">
            <div class="no-results-message">
              Пользователи не найдены
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="selectedUsers.length > 0"
        class="selected-forward-users"
      >
        <h4>Выбранные пользователи ({{ selectedUsers.length }})</h4>
        <div class="forward-users-list-container">
          <div class="forward-users-list">
            <div
              v-for="user in selectedUsers"
              :key="user.username"
              class="forward-selected-user"
            >
              <div class="forward-selected-user-info">
                <div class="forward-selected-user-main">
                  <span class="forward-selected-user-name">{{ getUserDisplayName(user) }}</span>
                  <span class="forward-selected-user-username">@{{ user.username }}</span>
                </div>
                <div class="forward-selected-user-details">
                  <span
                    v-if="user.position"
                    class="forward-selected-user-position"
                  >{{ user.position }}</span>
                  <span
                    v-if="user.organization"
                    class="forward-selected-user-organization"
                  >{{ user.organization }}</span>
                </div>

                <!-- Настройки доступа -->
                <div class="forward-selected-user-settings">
                  <!-- Тумблер "Требуется согласование" -->
                  <label class="setting-toggle">
                    <input
                      v-model="user.requires_approval"
                      type="checkbox"
                      class="setting-checkbox"
                      @change="onApprovalToggle(user)"
                    >
                    <span class="toggle-slider" />
                    <span class="toggle-text">Требуется согласование</span>
                  </label>

                  <!-- Тумблер "Согласование обязательно" (активен только если requires_approval = true) -->
                  <label
                    class="setting-toggle"
                    :class="{ 'toggle-disabled': !user.requires_approval }"
                  >
                    <input
                      v-model="user.required_approval"
                      type="checkbox"
                      :disabled="!user.requires_approval"
                      class="setting-checkbox"
                    >
                    <span class="toggle-slider" />
                    <span class="toggle-text">Согласование обязательно</span>
                  </label>
                </div>
              </div>
              <button
                class="remove-forward-user-btn"
                data-testid="forward-modal-remove-user"
                title="Удалить"
                @click="removeUser(user)"
              >
                ×
              </button>
            </div>
          </div>
        </div>
      </div>

      <div
        v-else
        class="no-forward-users"
      >
        <p>Выберите пользователей для пересылки заявки</p>
      </div>
    </div>

    <template #actions>
      <button
        type="button"
        class="lk-button lk-button--ghost"
        data-testid="forward-modal-button-cancel"
        @click="close"
      >
        Отмена
      </button>
      <button
        type="button"
        class="lk-button lk-button--primary"
        data-testid="forward-modal-button-send"
        :disabled="selectedUsers.length === 0 || isSending"
        @click="send"
      >
        {{ isSending ? 'Отправка...' : 'Отправить' }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue'

export default {
    name: 'ForwardModal',
    components: { BaseModal },
    props: {
        show: {
            type: Boolean,
            required: true
        },
        allUsers: {
            type: Array,
            required: true
        },
        responsibleUsers: {
            type: Array,
            default: () => []
        },
        // Пропсы для пользователей, у которых уже есть доступ
        existingApprovers: {
            type: Array,
            default: () => []
        },
        existingViewers: {
            type: Array,
            default: () => []
        },
        isSending: {
            type: Boolean,
            default: false
        }
    },
    emits: ['close', 'send', 'update:selected-users'],
    data() {
        return {
            searchQuery: '',
            searchResults: [],
            showDropdown: false,
            selectedUsers: [] // Каждый пользователь будет иметь поля: requires_approval, required_approval
        }
    },
    computed: {
        // ID пользователей, которые уже являются ответственными
        responsibleUserIds() {
            return this.responsibleUsers.map(user => user.id);
        },

        // ID пользователей, которые уже являются принимающими
        approverUserIds() {
            return this.existingApprovers.map(user => user.user_id);
        },

        // ID пользователей, которые уже являются просматривающими
        viewerUserIds() {
            return this.existingViewers.map(user => user.user_id);
        },

        // Все ID пользователей, у которых уже есть доступ (принимающие, ответственные, читатели)
        existingUserIds() {
            return [...this.responsibleUserIds, ...this.approverUserIds, ...this.viewerUserIds];
        },

        // Сортированный список всех пользователей, исключая тех, у кого уже есть доступ
        sortedAllUsers() {
            return [...this.allUsers]
                .filter(user => !this.existingUserIds.includes(user.id))
                .sort((a, b) => {
                    const nameA = this.getUserDisplayName(a).toLowerCase();
                    const nameB = this.getUserDisplayName(b).toLowerCase();
                    return nameA.localeCompare(nameB);
                });
        },

        // Отфильтрованные пользователи для отображения
        filteredUsers() {
            // Исключаем уже выбранных пользователей
            let availableUsers = this.sortedAllUsers.filter(user =>
                !this.selectedUsers.some(selected => selected.username === user.username)
            );

            // Если есть поисковый запрос, фильтруем по нему
            if (this.searchQuery.trim()) {
                const query = this.searchQuery.toLowerCase();
                availableUsers = availableUsers.filter(user => {
                    const fullName = this.getUserDisplayName(user).toLowerCase();
                    const username = user.username.toLowerCase();
                    const position = (user.position || '').toLowerCase();
                    const organization = (user.organization || '').toLowerCase();

                    return fullName.includes(query) ||
                           username.includes(query) ||
                           position.includes(query) ||
                           organization.includes(query);
                });
            }

            return availableUsers.slice(0, 15);
        }
    },
    watch: {
        // Модалка теперь всегда смонтирована (паттерн :show), поэтому чистим состояние
        // при открытии и переводим фокус на поиск.
        show(visible) {
            if (visible) {
                this.reset();
                this.$nextTick(() => {
                    this.$refs.searchInput?.focus();
                });
            }
        }
    },
    methods: {
        getUserDisplayName(user) {
            const names = [user.last_name, user.first_name, user.middle_name].filter(Boolean);
            return names.length > 0 ? names.join(' ') : user.username;
        },

        searchUsers() {
            this.showDropdown = true;
        },

        onSearchFocus() {
            this.showDropdown = true;
        },

        addUser(user) {
            const userWithSettings = {
                ...user,
                requires_approval: false,  // По умолчанию только просмотр
                required_approval: false    // По умолчанию необязательное
            };
            this.selectedUsers.push(userWithSettings);
            this.searchQuery = '';

            this.showDropdown = false;
            this.searchResults = [];
            this.$refs.searchInput.blur();

            this.$emit('update:selected-users', this.selectedUsers);
        },

        onApprovalToggle(user) {
            // Если выключили "Требуется согласование", сбрасываем "Согласование обязательно"
            if (!user.requires_approval) {
                user.required_approval = false;
            }
        },

        removeUser(user) {
            this.selectedUsers = this.selectedUsers.filter(u => u.username !== user.username);
            this.$emit('update:selected-users', this.selectedUsers);
        },

        onSearchBlur() {
            setTimeout(() => {
                this.showDropdown = false;
            }, 200);
        },

        close() {
            this.$emit('close');
        },

        send() {
            // Преобразуем данные для отправки на сервер
            const usersToSend = this.selectedUsers.map(user => {
                // Если требуется согласование - отправляем как ответственного
                if (user.requires_approval) {
                    return {
                        user_id: user.id,
                        required_approval: user.required_approval || false,
                        can_view: false  // Не может быть только просматривающим, если требуется согласование
                    };
                }
                // Если не требуется согласование - отправляем как просматривающего
                else {
                    return {
                        user_id: user.id,
                        required_approval: false,  // Не может быть обязательным согласующим
                        can_view: true  // Только просмотр
                    };
                }
            });

            this.$emit('send', usersToSend);
        },

        reset() {
            this.selectedUsers = [];
            this.searchQuery = '';
            this.showDropdown = false;
        }
    }
}
</script>

<style scoped>
.forward-body {
    padding: 20px;
}

.user-search-section {
    position: relative;
    margin-bottom: 20px;
}

.forward-user-dropdown {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    background: #fff;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    overflow: hidden;
    z-index: 1000;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    margin-top: 5px;
}

.forward-user-dropdown-content {
    max-height: 382px;
    overflow-y: auto;
}

.forward-user-item {
    padding: 10px;
    border-bottom: 1px solid #f0f0f0;
    cursor: pointer;
    transition: background-color 0.2s ease;
}

.forward-user-item:hover {
    background-color: #f0f0f0;
}

.forward-user-info {
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.forward-user-main {
    display: flex;
    flex-direction: column;
    gap: 3px;
}

.forward-user-name {
    font-weight: 600;
    font-size: 14px;
    color: #000;
}

.forward-user-username {
    font-size: 12px;
    color: #6b7280;
}

.forward-user-details {
    display: flex;
    gap: 5px;
}

.forward-user-position,
.forward-user-organization {
    font-size: 12px;
    color: #6b7280;
}

.no-results {
    padding: 15px;
}

.no-results-message {
    text-align: center;
    color: #6b7280;
    font-size: 14px;
    padding: 15px;
}

.selected-forward-users {
    display: flex;
    flex-direction: column;
}

.selected-forward-users h4 {
    font-size: 16px;
    color: var(--color-text);
    margin-bottom: 10px;
    font-weight: 600;
}

.forward-users-list-container {
    max-height: 320px;
    overflow-y: auto;
    padding-right: 5px;
}

.forward-users-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
}

.forward-selected-user {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: 12px;
    background: var(--color-bg-secondary);
    border-radius: 12px;
    border: 1px solid var(--color-border);
    transition: all 0.2s ease;
}

.forward-selected-user:hover {
    border-color: var(--color-primary);
    background: var(--color-bg);
}

.forward-selected-user-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.forward-selected-user-main {
    display: flex;
    flex-direction: column;
    gap: 3px;
}

.forward-selected-user-name {
    font-weight: 600;
    font-size: 15px;
    color: #000;
}

.forward-selected-user-username {
    font-size: 13px;
    color: #6b7280;
}

.forward-selected-user-details {
    display: flex;
    gap: 8px;
    margin-bottom: 4px;
}

.forward-selected-user-position,
.forward-selected-user-organization {
    font-size: 12px;
    color: #6b7280;
    background: #e8e8e8;
    padding: 2px 8px;
    border-radius: 12px;
    display: inline-block;
}

.forward-selected-user-settings {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-top: 4px;
}

.setting-toggle {
    display: flex;
    align-items: center;
    cursor: pointer;
    font-size: 13px;
    color: #666;
    gap: 8px;
    width: fit-content;
}

.setting-toggle.toggle-disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.setting-checkbox {
    display: none;
}

.toggle-slider {
    position: relative;
    width: 34px;
    height: 18px;
    background-color: #ccc;
    border-radius: 9px;
    transition: background-color 0.3s;
    display: inline-block;
}

.toggle-slider:before {
    content: "";
    position: absolute;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background-color: white;
    top: 2px;
    left: 2px;
    transition: transform 0.3s;
}

.setting-checkbox:checked + .toggle-slider {
    background-color: var(--color-primary);
}

.setting-checkbox:checked + .toggle-slider:before {
    transform: translateX(16px);
}

.setting-checkbox:disabled + .toggle-slider {
    background-color: #e0e0e0;
    cursor: not-allowed;
}

.toggle-text {
    font-size: 13px;
    color: var(--color-text);
}

.remove-forward-user-btn {
    background: none;
    border: none;
    color: var(--color-danger);
    font-size: 20px;
    cursor: pointer;
    padding: 6px 10px;
    border-radius: 5px;
    transition: background-color 0.2s ease;
    flex-shrink: 0;
}

.remove-forward-user-btn:hover {
    background-color: #fee;
}

.no-forward-users {
    text-align: center;
    padding: 40px;
    color: #6b7280;
    font-size: 15px;
    border: 1px dashed var(--color-border);
    border-radius: 12px;
    background: #fafafa;
}

.forward-user-dropdown-content::-webkit-scrollbar,
.forward-users-list-container::-webkit-scrollbar {
    width: 6px;
}

.forward-user-dropdown-content::-webkit-scrollbar-track,
.forward-users-list-container::-webkit-scrollbar-track {
    background: #f1f1f1;
    border-radius: 10px;
}

.forward-user-dropdown-content::-webkit-scrollbar-thumb,
.forward-users-list-container::-webkit-scrollbar-thumb {
    background: #c1c1c1;
    border-radius: 10px;
}

.forward-user-dropdown-content::-webkit-scrollbar-thumb:hover,
.forward-users-list-container::-webkit-scrollbar-thumb:hover {
    background: #a8a8a8;
}
</style>

<!-- не scoped: контент BaseModal телепортится в body и несёт data-v самого BaseModal,
     поэтому радиус и overflow задаём глобально двойным классом (бьёт scoped .base-modal BaseModal).
     overflow: visible нужен, чтобы выпадающий список поиска не обрезался скроллом модалки. -->
<style>
.base-modal.forward-modal {
    border-radius: 30px;
    overflow: visible;
}

.base-modal.forward-modal .base-modal__body {
    overflow: visible;
}
</style>
