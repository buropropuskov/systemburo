<!-- ForwardModal.vue -->
<template>
    <div class="modal-overlay" @click.self="close">
        <div class="modal">
            <div class="modal-header">
                <h3>Переслать заявку</h3>
                <button class="modal-close" @click="close">×</button>
            </div>
            <div class="modal-content">
                <div class="user-search-section">
                    <input
                        v-model="searchQuery"
                        @input="searchUsers"
                        @focus="onSearchFocus"
                        @blur="onSearchBlur"
                        class="forward-search-input"
                        placeholder="Поиск пользователей..."
                        type="text"
                        ref="searchInput"
                    />
                    <div v-if="showDropdown && filteredUsers.length > 0" class="forward-user-dropdown">
                        <div class="forward-user-dropdown-content">
                            <div 
                                v-for="user in filteredUsers" 
                                :key="user.username"
                                class="forward-user-item"
                                @mousedown.prevent="addUser(user)"
                            >
                                <div class="forward-user-info">
                                    <div class="forward-user-main">
                                        <span class="forward-user-name">{{ getUserDisplayName(user) }}</span>
                                        <span class="forward-user-username">@{{ user.username }}</span>
                                    </div>
                                    <div class="forward-user-details">
                                        <span v-if="user.position" class="forward-user-position">{{ user.position }}</span>
                                        <span v-if="user.organization" class="forward-user-organization">{{ user.organization }}</span>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                    <div v-else-if="showDropdown && filteredUsers.length === 0" class="forward-user-dropdown no-results">
                        <div class="forward-user-dropdown-content">
                            <div class="no-results-message">
                                Пользователи не найдены
                            </div>
                        </div>
                    </div>
                </div>
                
                <div v-if="selectedUsers.length > 0" class="selected-forward-users">
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
                                        <span v-if="user.position" class="forward-selected-user-position">{{ user.position }}</span>
                                        <span v-if="user.organization" class="forward-selected-user-organization">{{ user.organization }}</span>
                                    </div>
                                    
                                    <!-- Настройки доступа -->
                                    <div class="forward-selected-user-settings">
                                        <!-- Тумблер "Требуется согласование" -->
                                        <label class="setting-toggle">
                                            <input 
                                                type="checkbox" 
                                                v-model="user.requires_approval"
                                                @change="onApprovalToggle(user)"
                                                class="setting-checkbox"
                                            />
                                            <span class="toggle-slider"></span>
                                            <span class="toggle-text">Требуется согласование</span>
                                        </label>

                                        <!-- Тумблер "Согласование обязательно" (активен только если requires_approval = true) -->
                                        <label class="setting-toggle" :class="{ 'toggle-disabled': !user.requires_approval }">
                                            <input 
                                                type="checkbox" 
                                                v-model="user.required_approval"
                                                :disabled="!user.requires_approval"
                                                class="setting-checkbox"
                                            />
                                            <span class="toggle-slider"></span>
                                            <span class="toggle-text">Согласование обязательно</span>
                                        </label>
                                    </div>
                                </div>
                                <button 
                                    @click="removeUser(user)"
                                    class="remove-forward-user-btn"
                                    title="Удалить"
                                >
                                    ×
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
                
                <div v-else class="no-forward-users">
                    <p>Выберите пользователей для пересылки заявки</p>
                </div>
            </div>
            <div class="modal-footer">
                <button 
                    class="modal-cancel-btn"
                    @click="close"
                >
                    Отмена
                </button>
                <button 
                    class="modal-send-btn"
                    @click="send"
                    :disabled="selectedUsers.length === 0 || isSending"
                >
                    {{ isSending ? 'Отправка...' : 'Отправить' }}
                </button>
            </div>
        </div>
    </div>
</template>

<script>
export default {
    name: 'ForwardModal',
    props: {
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
            console.log('All users:', this.allUsers.length);
            console.log('Existing user IDs:', this.existingUserIds);
            
            const filteredUsers = [...this.allUsers]
                .filter(user => !this.existingUserIds.includes(user.id));
            
            console.log('Filtered users:', filteredUsers.length);
            
            return filteredUsers.sort((a, b) => {
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
            console.log('Adding user:', user);
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
            console.log('Selected users after add:', this.selectedUsers);
        },

        onApprovalToggle(user) {
            console.log('Toggle changed for user:', user.username, 'requires_approval:', user.requires_approval);
            // Если выключили "Требуется согласование", сбрасываем "Согласование обязательно"
            if (!user.requires_approval) {
                user.required_approval = false;
            }
            console.log('User after toggle:', user);
        },

        removeUser(user) {
            this.selectedUsers = this.selectedUsers.filter(u => u.username !== user.username);
            this.$emit('update:selected-users', this.selectedUsers);
            console.log('Selected users after remove:', this.selectedUsers);
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
            console.log('Sending selected users:', this.selectedUsers);
            
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
            
            console.log('Users to send to server:', usersToSend);
            this.$emit('send', usersToSend);
        },

        reset() {
            this.selectedUsers = [];
            this.searchQuery = '';
            this.showDropdown = false;
        }
    },
    emits: ['close', 'send', 'update:selected-users']
}
</script>

<style scoped>
.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.7);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 20000;
    animation: fadeIn 0.3s ease-out;
}

@keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
}

.modal {
    background: white;
    border-radius: 20px;
    width: 600px;
    max-width: 90%;
    height: 85vh;
    max-height: 85vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.25);
    animation: scaleIn 0.3s ease-out;
    overflow: hidden;
}

@keyframes scaleIn {
    from {
        opacity: 0;
        transform: scale(0.95);
    }
    to {
        opacity: 1;
        transform: scale(1);
    }
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px;
    border-bottom: 1px solid #e6e6e6;
    flex-shrink: 0;
    background: white;
}

.modal-header h3 {
    font-size: 18px;
    font-weight: 700;
    color: #333;
    margin: 0;
}

.modal-close {
    background: none;
    border: none;
    font-size: 24px;
    color: #a2a2a2;
    cursor: pointer;
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    transition: all 0.2s ease;
}

.modal-close:hover {
    background: #f0f0f0;
    color: #333;
}

.modal-content {
    flex: 1;
    padding: 20px;
    min-height: 0;
    display: flex;
    flex-direction: column;
}

.user-search-section {
    position: relative;
    margin-bottom: 20px;
    flex-shrink: 0;
}

.forward-search-input {
    width: 100%;
    padding: 12px 15px;
    border: 1px solid #e6e6e6;
    border-radius: 10px;
    font-size: 14px;
    transition: border-color 0.2s ease;
}

.forward-search-input:focus {
    border-color: #4F5BDF;
    outline: none;
}

.forward-user-dropdown {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    max-height: 382px;
    overflow-y: auto;
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
    margin-top: 20px;
    display: flex;
    flex-direction: column;
    min-height: 0;
    flex: 1;
}

.selected-forward-users h4 {
    font-size: 16px;
    color: #333;
    margin-bottom: 10px;
    font-weight: 600;
    flex-shrink: 0;
}

.forward-users-list-container {
    flex: 1;
    overflow-y: auto;
    min-height: 0;
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
    background: #f9f9f9;
    border-radius: 12px;
    border: 1px solid #e6e6e6;
    transition: all 0.2s ease;
}

.forward-selected-user:hover {
    border-color: #4F5BDF;
    background: #f8f9ff;
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
    background-color: #4F5BDF;
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
    color: #333;
}

.remove-forward-user-btn {
    background: none;
    border: none;
    color: #ef4444;
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
    border: 1px dashed #e6e6e6;
    border-radius: 12px;
    background: #fafafa;
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
}

.modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    padding: 20px;
    border-top: 1px solid #e6e6e6;
    flex-shrink: 0;
    background: white;
}

.modal-cancel-btn {
    padding: 10px 24px;
    background: #f0f0f0;
    color: #333;
    border: none;
    border-radius: 10px;
    font-size: 15px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
}

.modal-cancel-btn:hover {
    background: #e0e0e0;
}

.modal-send-btn {
    padding: 10px 24px;
    background: #4F5BDF;
    color: white;
    border: none;
    border-radius: 10px;
    font-size: 15px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
}

.modal-send-btn:hover:not(:disabled) {
    background: #3a45c0;
}

.modal-send-btn:disabled {
    background: #9ca3af;
    cursor: not-allowed;
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