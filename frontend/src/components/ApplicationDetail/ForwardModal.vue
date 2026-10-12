<!-- ForwardModal.vue -->
<template>
  <BaseModal
    :show="show"
    title="Переслать заявку"
    width="600px"
    radius="30px"
    :z-index="20000"
    content-class="forward-modal"
    @close="close"
  >
    <template #header>
      <div class="forward-heading">
        <h3 class="base-modal__title">Переслать заявку</h3>
        <div class="forward-validation-slot">
          <p v-if="attachments.length && !selectedAttachmentIds.length" id="forward-attachments-warning" role="alert" class="forward-validation-alert">⚠ Выберите хотя бы одно вложение для пересылки</p>
        </div>
      </div>
    </template>
    <div class="forward-body">
      <div
        v-if="viewOnly"
        class="forward-reader-note"
        data-testid="forward-modal-reader-note"
      >
        <template v-if="withdrawn">Заявка отозвана. Пересылка доступна только для просмотра; назначение согласующих недоступно.</template>
        <template v-else>Заявка доступна вам только для просмотра - переслать её можно тоже только для
        просмотра. Назначать согласующих и ответственных вправе отправитель.</template>
      </div>

      <div class="forward-selection">
      <div class="filters-row">
        <div v-for="(label, key) in { organization: 'Организации', company: 'Компании', position: 'Должности' }" :key="key" class="filters-row__control">
          <BaseDropdown
            v-model="filters[key]"
            :options="[{ id: '', name: 'Все ' + label.toLowerCase() }, ...[...new Set(sortedAllUsers.map(user => user[key]).filter(Boolean))].map(value => ({ id: value, name: value }))]"
            :placeholder="'Все ' + label.toLowerCase()"
            :menu-z-index="20010"
            searchable
            teleport
          />
        </div>
      </div>
      <div class="forward-search-row">
      <div
        ref="userChooser"
        class="user-search-section"
        @focusout="onSearchBlur"
      >
        <input
          ref="searchInput"
          v-model="searchQuery"
          class="lk-input"
          data-testid="forward-modal-search"
          placeholder="Поиск пользователей..."
          type="text"
          @input="searchUsers"
          @focus="onSearchFocus"
        >
        <Transition name="forward-chooser">
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
              role="button"
              tabindex="0"
              @keydown.enter.prevent="addUser(user)"
              @keydown.space.prevent="addUser(user)"
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
        </Transition>
      </div>

      <button type="button" class="reset-filters-btn" :disabled="!Object.values(filters).some(Boolean)" @click="resetFilters">Сбросить фильтры</button>
      </div>

      <div
        class="selected-forward-users"
      >
        <div class="forward-users-heading"><h4>Выбранные пользователи ({{ selectedUsers.length }})</h4>
        <small class="forward-scroll-note">{{ selectedUsers.length > 2 ? 'Список прокручивается' : '' }}</small></div>
        <div class="forward-users-frame"><div class="forward-users-list-container" :class="{ 'forward-users-list-container--full': selectedUsers.length >= 2 }">
          <div class="forward-users-list"><p v-if="selectedUsers.length === 0" class="forward-empty">Выберите пользователей для пересылки заявки</p>
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

                <!-- Настройки доступа. Читателю не показываем: назначить согласующего
                     или ответственного он не вправе, сервер такой запрос отбивает. -->
                <div
                  v-if="!viewOnly"
                  class="forward-selected-user-settings"
                  data-testid="forward-modal-user-settings"
                >
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
      </div></div>

      </div>
      <div class="forward-materials">
      <div
        v-if="attachments.length > 0"
        class="forward-attachments"
        data-testid="forward-modal-attachments"
        :aria-describedby="selectedAttachmentIds.length === 0 ? 'forward-attachments-warning' : undefined"
      >
        <div class="forward-attachments-header">
          <h4>Вложения для пересылки ({{ selectedAttachmentIds.length }}/{{ attachments.length }})</h4>
          <label class="forward-attachments-all">
            <input
              type="checkbox"
              class="setting-checkbox"
              :checked="allAttachmentsSelected"
              data-testid="forward-modal-attachments-all"
              @change="toggleAllAttachments($event.target.checked)"
            >
            <span class="toggle-slider" />
            <span class="forward-attachments-all-text">Выбрать все</span>
          </label>
        </div>
        <div class="forward-attachments-list">
          <label
            v-for="attachment in attachments"
            :key="attachment.id"
            class="forward-attachment-item"
            data-testid="forward-modal-attachment"
          >
            <input
              v-model="selectedAttachmentIds"
              type="checkbox"
              class="setting-checkbox"
              :value="attachment.id"
            >
            <span class="toggle-slider" />
            <span class="forward-attachment-info">
              <span class="forward-attachment-name">
                {{ attachment.attachment_display_name || attachment.attachment_name || `Вложение №${attachment.id}` }}
              </span>
              <span
                v-if="attachment.unique_attachment_title"
                class="forward-attachment-group"
              >{{ attachment.unique_attachment_title }}</span>
            </span>
          </label>
        </div>

      </div>

      <FormField
        label="Сопроводительное сообщение"
        class="forward-message-field"
      >
        <div class="forward-message-wrapper">
          <textarea
            v-model="message"
            class="lk-textarea forward-message-textarea"
            data-testid="forward-modal-message"
            :maxlength="messageMaxLength"
            rows="3"
            placeholder="Например: Прошу дополнительно согласовать заявку с вами"
          />
          <div
            class="forward-message-counter"
            :class="{ 'forward-message-counter--warning': messageNearLimit }"
          >
            {{ message.length }}/{{ messageMaxLength }}
          </div>
        </div>
      </FormField>

      <div
        class="forward-message-warning"
        data-testid="forward-modal-warning"
      >
        <span class="forward-message-warning-icon">⚠</span>
        <span>Ваше сообщение увидят все получатели заявки и бюро пропусков, а не только выбранные вами.</span>
      </div>
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
        :disabled="!canSend"
        @click="send"
      >
        {{ isSending ? 'Отправка...' : 'Отправить' }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue'
import FormField from '@/components/ui/FormField.vue'
import BaseDropdown from '@/components/ui/BaseDropdown.vue'
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';

export default {
    name: 'ForwardModal',
    components: { BaseModal, FormField, BaseDropdown },
    props: {
        withdrawn: { type: Boolean, default: false },
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
        // Вложения заявки для выбора, какие переслать получателям (#680).
        attachments: {
            type: Array,
            default: () => []
        },
        /**
         * Заявка доступна пересылающему только на просмотр (#1948): выбор роли
         * получателя закрыт, пересылка идёт только на просмотр. Зеркалит
         * forwardAuthority.readerOnly - сервер иначе отвечает 403.
         */
        readerOnly: {
            type: Boolean,
            default: false
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
            filters: {organization:'',company:'',position:''},
            searchResults: [],
            showDropdown: false,
            searchBlurTimer: null,
            selectedUsers: [], // Каждый пользователь будет иметь поля: requires_approval, required_approval
            selectedAttachmentIds: [], // ID вложений для пересылки; по умолчанию выбраны все
            message: '', // Сопроводительное сообщение при пересылке (#967), необязательное
            messageMaxLength: 2000
        }
    },
    computed: {
        viewOnly() { return this.withdrawn || this.readerOnly; },
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
            const variants = buildSearchVariants(this.searchQuery);
            if (variants.length) {
                availableUsers = availableUsers.filter(user => {
                    const haystack = [
                        this.getUserDisplayName(user),
                        user.username,
                        user.position || '',
                        user.organization || '',
                    ].join(' ');

                    return matchesSearch(haystack, variants);
                });
            }

            return availableUsers.filter(u => Object.entries(this.filters).every(([key,value]) => !value || u[key] === value)).slice(0, 15);
        },

        // Все вложения отмечены (мастер-тумблер "Выбрать все" в положении вкл).
        allAttachmentsSelected() {
            return this.attachments.length > 0 &&
                this.selectedAttachmentIds.length === this.attachments.length;
        },

        // Отправка возможна: есть получатели и (нет вложений или выбрано хотя бы одно).
        // Пустой список вложений на бэке означает "видны все" - запрещаем такую отправку,
        // чтобы снятие всех галочек не оборачивалось показом всех вложений получателю.
        canSend() {
            if (this.isSending || this.selectedUsers.length === 0) {
                return false;
            }
            if (this.attachments.length > 0 && this.selectedAttachmentIds.length === 0) {
                return false;
            }
            return true;
        },

        // Счётчик подсвечивается у порога длины (последние 100 символов).
        messageNearLimit() {
            return this.message.length >= this.messageMaxLength - 100;
        }
    },
    watch: {
        // Модалка теперь всегда смонтирована (паттерн :show), поэтому чистим состояние
        // при открытии. Фокус и список не открываем без действия пользователя.
        show(visible) {
            if (visible) {
                this.reset();
            } else {
                clearTimeout(this.searchBlurTimer);
                this.searchBlurTimer = null;
            }
        }
    },
    beforeUnmount() {
        clearTimeout(this.searchBlurTimer);
    },
    methods: {
        resetFilters() { this.filters = { organization: '', company: '', position: '' }; },
        getUserDisplayName(user) {
            const names = [user.last_name, user.first_name, user.middle_name].filter(Boolean);
            return names.length > 0 ? names.join(' ') : user.username;
        },

        searchUsers() {
            this.showDropdown = true;
        },

        onSearchFocus() {
            clearTimeout(this.searchBlurTimer);
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

        onSearchBlur(event) {
            clearTimeout(this.searchBlurTimer);
            if (this.$refs.userChooser?.contains(event?.relatedTarget)) return;
            this.searchBlurTimer = setTimeout(() => {
                this.searchBlurTimer = null;
                if (!this.$refs.userChooser?.contains(document.activeElement)) {
                    this.showDropdown = false;
                }
            }, 200);
        },

        toggleAllAttachments(checked) {
            this.selectedAttachmentIds = checked ? this.attachments.map(a => a.id) : [];
        },

        close() {
            this.$emit('close');
        },

        send() {
            // Преобразуем данные для отправки на сервер
            const usersToSend = this.selectedUsers.map(user => {
                // Если требуется согласование - отправляем как ответственного.
                // У читателя тумблеров нет вовсе, но флаг мог остаться от выбора,
                // сделанного до смены роли - тогда сервер ответил бы 403.
                if (user.requires_approval && !this.viewOnly) {
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

            this.$emit('send', {
                users: usersToSend,
                attachment_ids: [...this.selectedAttachmentIds],
                message: this.message.trim()
            });
        },

        reset() {
            clearTimeout(this.searchBlurTimer);
            this.searchBlurTimer = null;
            this.resetFilters();
            this.selectedUsers = [];
            this.searchQuery = '';
            this.showDropdown = false;
            this.message = '';
            // По умолчанию пересылаем все вложения (старое поведение), пользователь сужает.
            this.selectedAttachmentIds = this.attachments.map(a => a.id);
        }
    }
}
</script>



<!-- Общий BaseModal сохраняет свои заголовок, кнопки и предел высоты. Локальная flex-раскладка удерживает действия в окне; длинное тело прокручивается. -->
<style>
.base-modal.forward-modal {
    border-radius: 30px;
}
</style>

<style>
.base-modal.forward-modal {display:flex;flex-direction:column;max-height:92vh;overflow:hidden!important;border-radius:30px;}
.base-modal.forward-modal>.base-modal__header,
.base-modal.forward-modal>.base-modal__actions {flex:0 0 auto;position:relative;z-index:1;background:var(--surface)}
.base-modal.forward-modal>.base-modal__body {flex:1 1 auto;min-height:0;overflow-y:auto;overflow-x:hidden;overscroll-behavior:contain}
@media(max-width:600px) {.base-modal.forward-modal{max-height:92dvh;margin:0 10px}}
</style>

<style scoped src="@/assets/forward-recipients.css"></style>
<style scoped>
.forward-chooser-enter-active, .forward-chooser-leave-active {
  transition: opacity 200ms ease, transform 200ms ease;
}
.forward-chooser-enter-from, .forward-chooser-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
.forward-body {
  grid-template-columns: minmax(0,1.1fr) minmax(0,1fr);
  align-items: start;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px 20px;
  padding-top: 10px;
  padding-bottom: 10px;
}
.forward-reader-note {
  padding: 10px 12px;
  background: var(--accent-tint);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  line-height: 1.4;
  color: var(--color-text);
  grid-column: 1/-1;
  font-size: 12px;
  margin-bottom: 12px;
}
.user-search-section {
  position: relative;
  margin-bottom: 20px;
}
.no-results {
  padding: 15px;
}
.no-results-message {
  text-align: center;
  color: var(--text-muted);
  font-size: 14px;
  padding: 15px;
}
.setting-toggle {
  display: flex;
  align-items: center;
  cursor: pointer;
  font-size: 13px;
  color: var(--text-muted);
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
  background-color: var(--border);
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
  background-color: var(--surface);
  top: 2px;
  left: 2px;
  transition: transform 0.3s;
}
.setting-checkbox:checked + .toggle-slider {
  background-color: var(--accent);
}
.setting-checkbox:checked + .toggle-slider:before {
  transform: translateX(16px);
}
.setting-checkbox:disabled + .toggle-slider {
  background-color: var(--border);
  cursor: not-allowed;
}
.toggle-text {
  font-size: 13px;
  color: var(--color-text);
}
.no-forward-users {
  text-align: center;
  padding: 40px;
  color: var(--text-muted);
  font-size: 15px;
  border: 1px dashed var(--color-border);
  border-radius: 12px;
  background: var(--surface-2);
}
.forward-attachments {
  display: flex;
  flex-direction: column;
  margin-top: 10px;
}
.forward-attachments-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 6px;
}
.forward-attachments-header h4 {
  font-size: 13px;
  color: var(--color-text);
  font-weight: 600;
  margin: 0;
}
.forward-attachments-all {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--color-text);
  cursor: pointer;
  user-select: none;
  flex-shrink: 0;
}
.forward-attachments-list {
  padding-right: 5px;
  border: 1px solid var(--border);
  border-radius: 15px;
  background: var(--surface-2);
  overflow-y: scroll;
  scrollbar-gutter: stable;
  box-sizing: border-box;
  position: relative;
  max-height: none;
  height: auto;
  overflow: visible;
  padding: 6px 10px;
  display: flex;
  flex-direction: column;
  gap: 0;
}
.forward-attachment-item {
  display: flex;
  align-items: center;
  gap: 12px;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: background-color 0.2s ease;
  min-height: 28px;
  padding: 4px 0;
}
.forward-attachment-item:hover {
  background: var(--accent-tint);
}
.forward-attachment-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.forward-attachment-name {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.forward-attachment-group {
  align-self: flex-start;
  font-size: 11px;
  font-weight: 500;
  color: var(--accent-text);
  background: var(--accent-tint);
  padding: 1px 8px;
  border-radius: 8px;
}
.forward-message-field {
  margin-bottom: 0;
  margin-top: 0;
}
.forward-message-wrapper {
  position: relative;
}
.forward-message-textarea {
  padding-bottom: 26px;
  box-sizing: border-box;
  height: 48px;
  min-height: 48px;
}
.forward-message-counter {
  position: absolute;
  right: 12px;
  bottom: 8px;
  font-size: 12px;
  color: var(--color-text-muted);
  pointer-events: none;
}
.forward-message-counter--warning {
  color: var(--danger-text);
}
.forward-message-warning {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  background: var(--warning-bg);
  border: 1px solid var(--color-warning);
  border-radius: var(--radius-md);
  color: var(--color-text);
  font-size: 11px;
  line-height: 1.4;
  margin-top: 8px;
  padding: 7px 10px;
}
.forward-message-warning-icon {
  flex-shrink: 0;
  font-size: 14px;
  line-height: 1.4;
}
/* Размеры trigger из истории, раскладка и сброс из Центра заявок. */
.filters-row {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
  flex-wrap: nowrap;
  gap: 6px;
}
.filters-row__control {
  flex: 1 1 0;
  min-width: 0;
  max-width: none;
}
.filters-row :deep(.base-dropdown__button) {
  border-radius: 20px;
  padding: 6px 9px;
  font-size: 11px;
}
.forward-search-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}
.forward-search-row .user-search-section {
  flex: 1;
  min-width: 0;
  margin-bottom: 0;
}
.reset-filters-btn {
  padding: 7px 14px;
  border: 1px solid var(--border);
  background: var(--surface);
  border-radius: var(--radius-pill);
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
}
.reset-filters-btn:disabled {
  opacity: .5;
  cursor: default;
}
.forward-selection,.forward-materials {
  min-width: 0;
  width: 100%;
}
.forward-search-row .lk-input {
  padding: 7px 12px;
  font-size: 13px;
  min-height: 34px;
}
.forward-validation-slot {
  position: relative;
  height: 34px;
  margin-top: 0;
  flex: none;
  min-height: 34px;
}
.forward-validation-alert {
  position: absolute;
  inset: 0;
  margin: 0;
  display: flex;
  align-items: center;
  padding: 4px 8px;
  border-radius: var(--radius-md);
  background: var(--danger-bg);
  color: var(--danger-text);
  border: 1px solid var(--danger-text);
  font-size: 12px;
}
.forward-heading { flex: 1; min-width: 0; }
.forward-users-heading { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 12px; min-height: 24px; }
.forward-scroll-note {
  margin-left: auto;
  font-size: 11px;
  color: var(--text-muted);
  margin-bottom: 4px;
}
@media(max-width:760px){
.forward-body {
  grid-template-columns: minmax(0,1fr);
}
}
@media(max-width:600px) {
.filters-row__control {
  min-width: 140px
}
.forward-search-row {
  flex-wrap: wrap
}
.forward-validation-slot {
  height: 52px
}
.forward-validation-alert {
  line-height: 1.3
}}
.forward-materials {
  display: flex;
  flex-direction: column;
  gap: 8px
}
.forward-materials .forward-attachments-list {
  scrollbar-width: auto
}
/* Revision 3: complete recipient cards and complete attachment rows. */
</style>
