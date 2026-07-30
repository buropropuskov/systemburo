<template>
  <div class="user-management dashboard-card">
    <div class="management-header rt-header-inline">
      <h3 class="management-title">
        Учётные записи пользователей
        <span
          class="online-count"
          data-testid="users-online-count"
        >в сети: {{ onlineCount }}</span>
      </h3>
      <div class="search-container header-controls">
        <BaseDropdown
          class="archive-dropdown"
          :model-value="listMode"
          :options="archiveOptions"
          label-key="label"
          value-key="value"
          @update:model-value="onArchiveModeChange"
        />
        <SearchComponent
          v-model="userSearch"
          :title="'Поиск пользователей...'"
        />
        <button
          class="lk-button lk-button--primary rt-btn-compact"
          aria-label="Создать"
          @click="openCreateModal"
        >
          <span
            class="rt-btn-icon"
            aria-hidden="true"
          >+</span>
          <span class="rt-btn-label">Создать</span>
        </button>
        <RefreshButton
          :loading="refreshing"
          @refresh="refreshAllData"
        />
      </div>
    </div>

    <div
      v-if="selectedUsernames.length"
      class="bulk-bar"
      data-testid="users-bulk-bar"
    >
      <span class="bulk-count">Выбрано: {{ selectedUsernames.length }}</span>
      <div class="bulk-actions">
        <template v-if="!showArchive">
          <button
            class="pill pill-ghost"
            data-testid="users-bulk-type"
            @click="startBulkOperation('type')"
          >
            Тип
          </button>
          <button
            class="pill pill-ghost"
            data-testid="users-bulk-organization"
            @click="startBulkOperation('organization')"
          >
            Организация
          </button>
          <button
            class="pill pill-ghost"
            data-testid="users-bulk-company"
            @click="startBulkOperation('company')"
          >
            Компания
          </button>
          <button
            class="pill pill-danger"
            data-testid="users-bulk-ban"
            @click="openBulkBan"
          >
            Заблокировать
          </button>
          <button
            class="pill pill-ghost"
            data-testid="users-bulk-unban"
            @click="openBulkUnban"
          >
            Разблокировать
          </button>
        </template>
        <button
          v-if="!showArchive"
          class="pill pill-danger"
          data-testid="users-bulk-archive"
          @click="startBulkOperation('archive')"
        >
          В архив
        </button>
        <button
          v-else
          class="pill pill-restore"
          data-testid="users-bulk-restore"
          @click="startBulkOperation('restore')"
        >
          Восстановить
        </button>
        <button
          class="pill pill-ghost bulk-clear"
          data-testid="users-bulk-clear"
          @click="clearSelection"
        >
          Снять выбор
        </button>
      </div>
    </div>

    <div class="users-container">
      <div class="users-list rt-table">
        <!-- Заголовок таблицы -->
        <div class="users-header">
          <div class="header-row rt-head-row">
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
                data-testid="users-select-all"
                @change="toggleSelectAll"
              >
            </div>
            <div
              class="header-col login-col"
              @click="sortBy('username')"
            >
              <p :class="{ 'active-sort': sortField === 'username' }">
                Логин
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'username',
                  'desc': sortField === 'username' && sortDirection === 'desc'
                }" 
              >
            </div>
            <div
              class="header-col name-col"
              @click="sortBy('full_name')"
            >
              <p :class="{ 'active-sort': sortField === 'full_name' }">
                Фамилия И.О.
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'full_name',
                  'desc': sortField === 'full_name' && sortDirection === 'desc'
                }" 
              >
            </div>
            <div
              class="header-col org-col"
              @click="sortBy('organization')"
            >
              <p :class="{ 'active-sort': sortField === 'organization' }">
                Организация / Отдел
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'organization',
                  'desc': sortField === 'organization' && sortDirection === 'desc'
                }" 
              >
            </div>
            <div
              class="header-col company-col"
              @click="sortBy('company')"
            >
              <p :class="{ 'active-sort': sortField === 'company' }">
                Компания
              </p>
              <img 
                src="@/assets/icons/sort.png" 
                class="sort-icon" 
                :class="{ 
                  'sorted': sortField === 'company',
                  'desc': sortField === 'company' && sortDirection === 'desc'
                }" 
              >
            </div>
            <div
              class="header-col position-col"
              @click="sortBy('position')"
            >
              <p :class="{ 'active-sort': sortField === 'position' }">
                Должность
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'position',
                  'desc': sortField === 'position' && sortDirection === 'desc'
                }"
              >
            </div>
            <div
              class="header-col type-col"
              @click="sortBy('user_type')"
            >
              <p :class="{ 'active-sort': sortField === 'user_type' }">
                Тип
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'user_type',
                  'desc': sortField === 'user_type' && sortDirection === 'desc'
                }"
              >
            </div>
            <div
              class="header-col seen-col"
              @click="sortBy('last_seen')"
            >
              <p :class="{ 'active-sort': sortField === 'last_seen' }">
                В сети
              </p>
              <img
                src="@/assets/icons/sort.png"
                class="sort-icon"
                :class="{
                  'sorted': sortField === 'last_seen',
                  'desc': sortField === 'last_seen' && sortDirection === 'desc'
                }"
              >
            </div>
          </div>
        </div>
        
        <!-- Тело таблицы -->
        <div class="users-body">
          <div
            v-for="(user, index) in sortedUsers"
            :key="user.username"
            class="user-item"
            :class="{ 'inactive': user.is_active === false }"
            @click="selectUser(user)"
          >
            <div class="user-row rt-row">
              <div
                class="user-col check-col"
                @click.stop
              >
                <input
                  type="checkbox"
                  class="bulk-check"
                  :checked="isSelected(user.username)"
                  :aria-label="`Выбрать ${user.username}`"
                  data-testid="users-row-check"
                  @click="onRowCheck(user, index, $event)"
                >
              </div>
              <div
                class="user-col login-col"
                data-label="Логин"
              >
                <span class="user-login">{{ user.username }}</span>
                <span
                  v-if="user.is_active === false"
                  class="inactive-badge"
                >(архив)</span>
              </div>
              <div
                class="user-col name-col"
                data-label="Фамилия И.О."
              >
                {{ formatUserName(user) }}
              </div>
              <div
                class="user-col org-col"
                data-label="Организация / Отдел"
              >
                <span
                  class="truncate-text"
                  :title="user.organization || '-'"
                >
                  {{ user.organization || '-' }}
                </span>
              </div>
              <div
                class="user-col company-col"
                data-label="Компания"
              >
                <span
                  class="truncate-text"
                  :title="user.company || '-'"
                >
                  {{ user.company || '-' }}
                </span>
              </div>
              <div
                class="user-col position-col"
                data-label="Должность"
              >
                <span
                  class="truncate-text"
                  :title="user.position || '-'"
                >
                  {{ user.position || '-' }}
                </span>
              </div>
              <div
                class="user-col type-col"
                data-label="Тип"
              >
                <span
                  v-if="user.user_type"
                  class="type-badge"
                >{{ user.user_type }}</span>
                <span v-else>-</span>
              </div>
              <div
                class="user-col seen-col"
                data-label="В сети"
                data-testid="users-row-seen"
              >
                <!-- Подсказка на самом значении, а не нативный title: тот при каждой
                     смене атрибута (а давность тикает раз в секунду) гаснет и всплывает
                     заново прямо под курсором. -->
                <HintTooltip
                  :text="seenTitle(user, presenceNow)"
                  :width="240"
                  data-testid="users-row-seen-hint"
                >
                  <Badge
                    v-if="isOnline(user, presenceNow)"
                    variant="success"
                    size="sm"
                    dot
                    data-testid="users-row-online-badge"
                  >
                    Онлайн
                  </Badge>
                  <span
                    v-else
                    class="seen-text"
                  >{{ formatSeenShort(user, presenceNow) }}</span>
                </HintTooltip>
              </div>
            </div>
          </div>
        </div>
        <div class="users-footer">
          <span class="items-count">{{ countLabel }}: {{ sortedUsers.length }}</span>
        </div>
      </div>
    </div>

    <div
      v-if="filteredUsers.length === 0"
      class="no-users"
    >
      <p>{{ userSearch ? 'Пользователи не найдены' : 'Пользователи отсутствуют' }}</p>
    </div>

    <!-- Модальное окно редактирования пользователя -->
    <BaseModal
      :show="showEditModal && !!selectedUser"
      width="880px"
      content-class="user-edit-modal"
      radius="45px"
      :z-index="1001"
      @close="closeEditModal"
    >
      <template #header>
        <div class="modal-title-group">
          <h3 class="modal-title">
            Редактирование
          </h3>
          <p class="modal-subtitle">
            учётной записи <strong>{{ selectedUser && selectedUser.username }}</strong>
          </p>
        </div>
        <div
          v-if="selectedUser"
          class="modal-header-actions"
        >
          <button
            class="lk-button lk-button--secondary"
            @click="openHistory(selectedUser)"
          >
            История
          </button>
          <template v-if="selectedUser.is_active !== false">
            <button
              class="lk-button lk-button--secondary"
              data-testid="user-reset-onboarding"
              @click="resetOnboarding(selectedUser)"
            >
              Сбросить обучение
            </button>
            <button
              class="lk-button lk-button--secondary"
              data-testid="user-access"
              @click="openAccess(selectedUser)"
            >
              Права доступа
            </button>
            <button
              v-if="selectedUserIsSecurity"
              class="lk-button lk-button--secondary"
              data-testid="user-access-places"
              @click="openAccessPlaces(selectedUser)"
            >
              Места доступа
            </button>
          </template>
          <template v-else>
            <span class="archive-badge">В архиве</span>
            <button
              class="lk-button lk-button--primary"
              @click="restoreUser(selectedUser)"
            >
              Восстановить
            </button>
          </template>
        </div>
      </template>

      <div
        v-if="selectedUser"
        class="modal-body-inner"
      >
        <nav
          class="modal-tabs"
          role="tablist"
        >
          <button
            type="button"
            class="modal-tab"
            :class="{ 'modal-tab--active': activeTab === 'profile' }"
            role="tab"
            :aria-selected="activeTab === 'profile'"
            @click="activeTab = 'profile'"
          >
            Профиль
          </button>
          <button
            type="button"
            class="modal-tab"
            :class="{ 'modal-tab--active': activeTab === 'logins' }"
            role="tab"
            :aria-selected="activeTab === 'logins'"
            data-testid="user-tab-logins"
            @click="activeTab = 'logins'"
          >
            История входов
          </button>
        </nav>

        <div
          v-show="activeTab === 'profile'"
          class="tab-panel"
        >
          <div class="details-grid-two-columns">
            <div class="details-column">
              <div class="detail-group">
                <label class="detail-label">Фамилия:</label>
                <input
                  v-model="selectedUser.last_name"
                  class="lk-input"
                  placeholder="Введите фамилию"
                  autocomplete="new-password"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  @change="updateUserInfo(selectedUser)"
                >
              </div>

              <div class="detail-group">
                <label class="detail-label">Отчество:</label>
                <input
                  v-model="selectedUser.middle_name"
                  class="lk-input"
                  placeholder="Введите отчество"
                  autocomplete="new-password"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  @change="updateUserInfo(selectedUser)"
                >
              </div>

              <div class="detail-group">
                <label class="detail-label">Организация:</label>
                <BaseDropdown
                  :model-value="selectedUser.organization_id"
                  :options="orgOptionsWithNone"
                  label-key="name"
                  value-key="id"
                  placeholder="Не выбрано"
                  searchable
                  @update:model-value="onSelectOrganization"
                />
              </div>

              <div class="detail-group">
                <label class="detail-label">Должность:</label>
                <input
                  v-model="selectedUser.position"
                  class="lk-input"
                  placeholder="Введите должность"
                  autocomplete="new-password"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  @change="updateUserInfo(selectedUser)"
                >
              </div>
            </div>

            <div class="details-column">
              <div class="detail-group">
                <label class="detail-label">Имя:</label>
                <input
                  v-model="selectedUser.first_name"
                  class="lk-input"
                  placeholder="Введите имя"
                  autocomplete="new-password"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  @change="updateUserInfo(selectedUser)"
                >
              </div>

              <div class="detail-group">
                <label class="detail-label">Телефон:</label>
                <input
                  :value="selectedUser.phone"
                  class="lk-input"
                  placeholder="+7 (___) ___ __-__"
                  type="tel"
                  autocomplete="new-password"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  @input="onPhoneInput($event, 'selectedUser')"
                  @change="updateUserInfo(selectedUser)"
                >
              </div>

              <div class="detail-group">
                <label class="detail-label">Компания:</label>
                <BaseDropdown
                  :model-value="selectedUser.company_id"
                  :options="companyOptionsWithNone"
                  label-key="name"
                  value-key="id"
                  placeholder="Не выбрано"
                  searchable
                  @update:model-value="onSelectCompany"
                />
              </div>

              <div class="detail-group">
                <label class="detail-label">Email:</label>
                <input
                  v-model="selectedUser.email"
                  class="lk-input"
                  placeholder="Введите email"
                  type="email"
                  autocomplete="new-password"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  @change="updateUserInfo(selectedUser)"
                >
              </div>
            </div>
          </div>

          <div class="full-width-groups">
            <div class="detail-group detail-group--checkbox">
              <ToggleSwitch
                :model-value="!!selectedUser.is_important"
                @update:model-value="val => { selectedUser.is_important = val; updateUserInfo(selectedUser); }"
              >
                Важный пользователь
              </ToggleSwitch>
            </div>

            <div class="detail-group">
              <label class="detail-label">Тип пользователя:</label>
              <BaseDropdown
                :model-value="selectedUser.type_id"
                :options="userTypes"
                label-key="name"
                value-key="id"
                placeholder="Не выбрано"
                @update:model-value="onSelectUserType"
              />
            </div>

            <div class="detail-group password-group">
              <label class="detail-label">Новый пароль:</label>
              <div class="password-input-container">
                <input
                  v-model="selectedUser.newPassword"
                  :type="showNewPass ? 'text' : 'password'"
                  class="password-input-sm"
                  placeholder="Новый пароль"
                  autocomplete="new-password"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  @keyup="checkInputLanguage($event)"
                  @keyup.enter="changeUserPassword(selectedUser)"
                >
                <div class="password-actions">
                  <button
                    class="generate-password-btn"
                    type="button"
                    @click="generatePassword(selectedUser)"
                  >
                    <img
                      src="@/assets/icons/random.png"
                      class="generate-icon"
                    >
                    Генерировать
                  </button>
                  <button
                    :disabled="!changePasswordValid"
                    class="save-password-btn"
                    @click="changeUserPassword(selectedUser)"
                  >
                    <img
                      src="@/assets/icons/save.png"
                      class="save-icon"
                    >
                  </button>
                </div>
              </div>
              <div class="input-hints">
                <span
                  class="language-hint"
                  :class="{ 'warning': isCapsLockOn }"
                >
                  {{ currentLanguage }} {{ isCapsLockOn ? '| CAPS LOCK' : '' }}
                </span>
                <ul
                  v-if="selectedUser && selectedUser.newPassword"
                  class="password-checklist"
                >
                  <li
                    v-for="rule in changePasswordRules"
                    :key="rule.key"
                    :class="{ 'password-checklist__item--ok': rule.ok }"
                    class="password-checklist__item"
                  >
                    {{ rule.ok ? '✓' : '○' }} {{ rule.label }}
                  </li>
                </ul>
              </div>
            </div>
          </div>

          <div
            v-if="selectedUser.is_active !== false"
            class="danger-zone"
          >
            <span class="danger-zone__hint">Удаление переносит учётную запись в архив и блокирует вход.</span>
            <button
              class="lk-button lk-button--danger"
              @click="confirmDeleteUser(selectedUser)"
            >
              Удалить учётную запись
            </button>
          </div>
        </div>

        <div
          v-show="activeTab === 'logins'"
          class="tab-panel"
        >
          <UserLoginHistory
            v-if="activeTab === 'logins'"
            :username="selectedUser.username"
            :current-user-name="currentUserName"
          />
        </div>
      </div>
    </BaseModal>

    <!-- Модальное окно создания пользователя -->
    <BaseModal
      :show="showCreateModal"
      title="Создание пользователя"
      width="600px"
      content-class="user-create-modal"
      radius="45px"
      @close="closeCreateModal"
    >
      <div class="create-user-body">
        <div class="form-wrap">
          <div class="input-group half">
            <label class="input-label">Логин <span class="required">*</span></label>
            <input
              ref="usernameInput"
              v-model="newUser.username"
              placeholder="Введите логин"
              class="modal-input"
              @input="saveDraft"
            >
          </div>
          <div class="input-group half">
            <label class="input-label">Пароль <span class="required">*</span></label>
            <PasswordInput
              v-model="newUser.password"
              placeholder="Введите пароль"
              @input="saveDraft"
            />
            <ul
              v-if="newUser.password"
              class="password-checklist"
            >
              <li
                v-for="rule in createPasswordRules"
                :key="rule.key"
                :class="{ 'password-checklist__item--ok': rule.ok }"
                class="password-checklist__item"
              >
                {{ rule.ok ? '✓' : '○' }} {{ rule.label }}
              </li>
            </ul>
          </div>
          <div class="input-group half">
            <label class="input-label">Организация <span class="required">*</span></label>
            <BaseDropdown
              :model-value="newUser.organization_id"
              :options="orgOptionsWithNone"
              label-key="name"
              value-key="id"
              placeholder="Не выбрано"
              searchable
              @update:model-value="onSelectNewUserOrg"
            />
          </div>
          <div class="input-group half">
            <label class="input-label">Компания <span class="required">*</span></label>
            <BaseDropdown
              :model-value="newUser.company_id"
              :options="companyOptionsWithNone"
              label-key="name"
              value-key="id"
              placeholder="Не выбрано"
              searchable
              @update:model-value="onSelectNewUserCompany"
            />
          </div>
          <div class="input-group full">
            <label class="input-label">Тип пользователя <span class="required">*</span></label>
            <BaseDropdown
              :model-value="newUser.type_id"
              :options="userTypes"
              label-key="name"
              value-key="id"
              placeholder="Выберите тип"
              @update:model-value="onSelectNewUserType"
            />
          </div>
          <div class="input-group half">
            <label class="input-label">Фамилия</label>
            <input
              v-model="newUser.last_name"
              placeholder="Введите фамилию"
              class="modal-input"
              @input="saveDraft"
            >
          </div>
          <div class="input-group half">
            <label class="input-label">Имя</label>
            <input
              v-model="newUser.first_name"
              placeholder="Введите имя"
              class="modal-input"
              @input="saveDraft"
            >
          </div>
          <div class="input-group half">
            <label class="input-label">Отчество</label>
            <input
              v-model="newUser.middle_name"
              placeholder="Введите отчество"
              class="modal-input"
              @input="saveDraft"
            >
          </div>
          <div class="input-group half">
            <label class="input-label">Должность</label>
            <input
              v-model="newUser.position"
              placeholder="Введите должность"
              class="modal-input"
              @input="saveDraft"
            >
          </div>
          <div class="input-group half">
            <label class="input-label">Email</label>
            <input
              v-model="newUser.email"
              placeholder="Введите email"
              class="modal-input"
              type="email"
              @input="saveDraft"
            >
          </div>
          <div class="input-group half">
            <label class="input-label">Телефон</label>
            <input
              :value="newUser.phone"
              placeholder="+7 (___) ___ __-__"
              class="modal-input"
              type="tel"
              @input="onPhoneInput($event, 'newUser')"
            >
          </div>
        </div>
      </div>

      <template #actions>
        <button
          class="modal-btn modal-btn--cancel"
          @click="closeCreateModal"
        >
          Отмена
        </button>
        <button
          :disabled="!canCreateUser"
          class="modal-btn modal-btn--confirm"
          :class="{'modal-btn--disabled': !canCreateUser}"
          @click="createUser"
        >
          Создать
        </button>
      </template>
    </BaseModal>

    <ConfirmationModal
      :show="bulkConfirmVisible"
      :title="bulkConfirmTitle"
      :message="bulkConfirmMessage"
      :confirm-text="bulkConfirmText"
      :confirm-button-style="bulkConfirmButtonStyle"
      @confirm="applyBulkArchiveRestore"
      @cancel="cancelBulkConfirm"
    />

    <UserBulkOperationsModal
      :show="bulkModalVisible"
      :operation="pendingBulkOp"
      :selected-count="selectedUsernames.length"
      :user-types="userTypes"
      :organizations="organizations"
      :companies="companies"
      :submitting="bulkSubmitting"
      @apply="applyBulk"
      @close="cancelBulkModal"
    />

    <BaseModal
      :show="banModalVisible"
      title="Заблокировать пользователей"
      width="520px"
      @close="cancelBulkBan"
    >
      <div
        class="bulk-ban-body"
        data-testid="users-bulk-ban-modal"
      >
        <p class="bulk-ban-warn">
          Заблокировать <b>{{ selectedUsernames.length }}</b> выбранных пользователей?
          Их активные сессии будут завершены, вход станет недоступен до разблокировки.
          Супер-администраторов и себя заблокировать нельзя - они попадут в список непрошедших.
        </p>
        <label class="bulk-ban-label">Причина (необязательно, покажется заблокированному)</label>
        <textarea
          v-model="banReason"
          class="lk-textarea"
          rows="3"
          maxlength="500"
          data-testid="users-bulk-ban-reason"
          placeholder="Например: нарушение регламента"
        />
      </div>
      <template #actions>
        <button
          class="pill pill-ghost"
          data-testid="users-bulk-ban-cancel"
          @click="cancelBulkBan"
        >
          Отмена
        </button>
        <button
          class="pill pill-danger"
          :disabled="bulkSubmitting"
          data-testid="users-bulk-ban-apply"
          @click="applyBulkBan"
        >
          {{ bulkSubmitting ? 'Блокировка...' : `Заблокировать (${selectedUsernames.length})` }}
        </button>
      </template>
    </BaseModal>

    <ConfirmationModal
      :show="unbanConfirmVisible"
      title="Разблокировать пользователей"
      :message="`Разблокировать выбранных пользователей (${selectedUsernames.length})? Им нужно будет заново войти.`"
      confirm-text="Разблокировать"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#10b981', borderColor: '#10b981' }"
      @confirm="applyBulkUnban"
      @cancel="unbanConfirmVisible = false"
    />

    <!-- Кнопка удаления живёт ВНУТРИ карточки редактирования (:z-index 1001),
         поэтому подтверждение поднимаем над ней: на базовом слое 1000 оно
         открывалось под карточкой и было не видно. -->
    <ConfirmationModal
      :show="!!deleteConfirmUser"
      :z-index="1002"
      title="Удаление пользователя"
      :message="deleteConfirmUser ? `Удалить учётную запись «${deleteConfirmUser.username}»? Действие необратимо.` : ''"
      confirm-text="Удалить"
      cancel-text="Отмена"
      :confirm-button-style="{ background: '#c62828', borderColor: '#c62828' }"
      @confirm="performDeleteUser"
      @cancel="deleteConfirmUser = null"
    />

    <UserHistoryModal
      v-if="historyForUser"
      :user="historyForUser"
      :organizations="organizations"
      :companies="companies"
      :user-types="userTypes"
      :current-user-name="currentUserName"
      @close="historyForUser = null"
    />

    <UserAccessModal
      v-if="accessUser"
      :user="accessUser"
      @close="accessUser = null"
      @updated="onAccessUpdated"
    />

    <UserAccessPlacesModal
      v-if="accessPlacesUser"
      :user="accessPlacesUser"
      @close="accessPlacesUser = null"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { bulkArchiveUsers, bulkRestoreUsers, bulkUpdateUsersType, bulkAssignUsersOrganization, bulkAssignUsersCompany, bulkBanUsers, bulkUnbanUsers } from '@/api/users';
import { ref } from 'vue';
import { mapState, mapActions } from 'pinia';
import { useOrganizationsStore } from '@/stores/organizations';
import { useCompaniesStore } from '@/stores/companies';
import { applyPhoneMask } from '@/composables/useRussianPhoneMask'
import { formatShortName } from '@/utils/formatName'
import { isOnline, formatSeenShort, seenTitle, lastSeenSortKey } from '@/utils/presence'
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants'
import SearchComponent from './SearchComponent.vue';
import RefreshButton from './RefreshButton.vue';
import PasswordInput from './ui/PasswordInput.vue';
import { getPasswordPolicy } from '@/api/settings';
import { evaluatePassword, passwordMeetsPolicy, generatePassword as buildPassword, DEFAULT_PASSWORD_POLICY } from '@/utils/passwordPolicy';
import ConfirmationModal from './ConfirmationModal.vue';
import BaseModal from './ui/BaseModal.vue';
import BaseDropdown from './ui/BaseDropdown.vue';
import Badge from './ui/Badge.vue';
import HintTooltip from './ui/HintTooltip.vue';
import ToggleSwitch from './ui/ToggleSwitch.vue';
import UserHistoryModal from './UserHistoryModal.vue';
import UserLoginHistory from './UserLoginHistory.vue';
import UserAccessModal from './admin/UserAccessModal.vue';
import UserAccessPlacesModal from './admin/UserAccessPlacesModal.vue';
import UserBulkOperationsModal from './UserBulkOperationsModal.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';
import { resetOnboardingForUser } from '@/api/onboarding';

// Тик подписей присутствия: раз в секунду, потому что младшая единица подписи -
// секунды, и на более редком тике «12 с» висело бы неверным до полминуты. Пересчёт
// идёт по уже загруженным данным, без запросов. Опрос списка реже - он ходит на бэк,
// а last_seen там всё равно пишется с троттлингом 60с (internal/middleware/last_seen.go).
const PRESENCE_TICK_MS = 1000;
const PRESENCE_POLL_MS = 60_000;

export default {
  components: {
    SearchComponent,
    RefreshButton,
    PasswordInput,
    ConfirmationModal,
    BaseModal,
    BaseDropdown,
    Badge,
    HintTooltip,
    ToggleSwitch,
    UserHistoryModal,
    UserLoginHistory,
    UserAccessModal,
    UserAccessPlacesModal,
    UserBulkOperationsModal
  },
  props: {
    allUsers: {
      type: Array,
      required: true
    }
  },
  emits: ['fetch-users', 'user-updated'],
  setup() {
    const showCreateModal = ref(false);
    const showEditModal = ref(false);

    function closeEditModal() {
      showEditModal.value = false;
    }

    return { showCreateModal, showEditModal, closeEditModal };
  },
  data() {
    return {
      userSearch: '',
      refreshing: false,
      selectedUser: null,
      activeTab: 'profile',
      historyForUser: null,
      accessUser: null,
      accessPlacesUser: null,
      currentUserName: '',
      deleteConfirmUser: null,
      // Групповой выбор (по username). lastSelectedUsername - якорь shift-диапазона.
      selectedUsernames: [],
      lastSelectedUsername: null,
      pendingBulkOp: null,
      bulkConfirmVisible: false,
      bulkModalVisible: false,
      banModalVisible: false,
      banReason: '',
      unbanConfirmVisible: false,
      bulkSubmitting: false,
      // Режим списка: активные / только присутствующие сейчас / архив. Наборы
      // bulk-операций у активных и архивных разные, поэтому режим один на всё,
      // а не отдельный тумблер «онлайн» поверх архива.
      listMode: 'active',
      archiveOptions: [
        { value: 'active', label: 'Активные' },
        { value: 'online', label: 'В сети' },
        { value: 'archive', label: 'Архив' },
      ],
      // presenceNow - тикающее «сейчас» для колонки присутствия. Держим в data, а не
      // читаем Date.now() внутри computed: иначе пересчёт не триггерится и точка
      // никогда не гаснет без перезагрузки.
      presenceNow: Date.now(),
      presenceTimer: null,
      presencePollTimer: null,
      showNewPass: false,
      currentLanguage: '',
      isCapsLockOn: false,
      userTypes: [],
      sortField: null,
      sortDirection: 'desc',
      passwordPolicy: { ...DEFAULT_PASSWORD_POLICY },
      newUser: {
        username: '',
        password: '',
        last_name: '',
        first_name: '',
        middle_name: '',
        email: '',
        phone: '',
        position: '',
        type_id: null,
        organization_id: null,
        company_id: null
      }
    };
  },
  computed: {
    ...mapState(useOrganizationsStore, { organizations: 'items' }),
    ...mapState(useCompaniesStore, { companies: 'items' }),
    showArchive() {
      return this.listMode === 'archive';
    },
    onlineOnly() {
      return this.listMode === 'online';
    },
    // Подпись футера идёт от режима списка: «Всего пользователей» под отфильтрованным
    // числом читалось бы как «в системе всего один», хотя это только те, кто в сети.
    countLabel() {
      if (this.showArchive) return 'В архиве';
      return this.onlineOnly ? 'В сети' : 'Всего пользователей';
    },
    // Счётчик шапки считается по всем учёткам, а не по видимым: он отвечает на
    // «сколько людей в системе сейчас», и поиск с режимом списка не должны его менять.
    onlineCount() {
      return this.allUsers.filter(user => isOnline(user, this.presenceNow)).length;
    },
    // Опции с пунктом "Не выбрано" (null) для дропдаунов создания - орг/компания опциональны.
    orgOptionsWithNone() {
      return [{ id: null, name: 'Не выбрано' }, ...this.organizations];
    },
    companyOptionsWithNone() {
      return [{ id: null, name: 'Не выбрано' }, ...this.companies];
    },
    // Места доступа настраиваются только охранникам ЧОП — резолвим по стабильному
    // user_types.code, не по числовому type_id (нестабилен между средами).
    selectedUserIsSecurity() {
      if (!this.selectedUser) return false;
      const type = this.userTypes.find(t => t.id === this.selectedUser.type_id);
      return type?.code === 'security';
    },
    allSelected() {
      return this.sortedUsers.length > 0 && this.selectedUsernames.length === this.sortedUsers.length;
    },
    someSelected() {
      return this.selectedUsernames.length > 0 && !this.allSelected;
    },
    bulkConfirmTitle() {
      return this.pendingBulkOp === 'restore' ? 'Восстановление пользователей' : 'Архивация пользователей';
    },
    bulkConfirmMessage() {
      const n = this.selectedUsernames.length;
      return this.pendingBulkOp === 'restore'
        ? `Восстановить выбранных пользователей (${n})?`
        : `Архивировать выбранных пользователей (${n})? Активные сессии будут завершены.`;
    },
    bulkConfirmText() {
      return this.pendingBulkOp === 'restore' ? 'Восстановить' : 'В архив';
    },
    bulkConfirmButtonStyle() {
      return this.pendingBulkOp === 'restore'
        ? { background: '#10b981', borderColor: '#10b981' }
        : { background: '#c62828', borderColor: '#c62828' };
    },
    filteredUsers() {
      const variants = buildSearchVariants(this.userSearch);
      return this.allUsers
        .filter(user => (this.showArchive ? user.is_active === false : user.is_active !== false))
        .filter(user => !this.onlineOnly || isOnline(user, this.presenceNow))
        .filter(user => matchesSearch(
          `${user.username} ${user.organization || ''} ${user.company || ''} `
          + `${user.user_type || ''} ${user.position || ''} ${this.formatUserName(user)}`,
          variants,
        ))
        .map(user => ({
          ...user,
          newPassword: '',
          last_name: user.last_name || '',
          first_name: user.first_name || '',
          middle_name: user.middle_name || '',
          position: user.position || '',
          email: user.email || '',
          phone: user.phone || ''
        }));
    },
    sortedUsers() {
      const users = [...this.filteredUsers];
      
      if (!this.sortField) {
        return users.sort((a, b) => a.username.localeCompare(b.username));
      }
      
      return users.sort((a, b) => {
        let valueA, valueB;
        
        switch (this.sortField) {
          case 'username':
            valueA = a.username;
            valueB = b.username;
            break;
          case 'full_name':
            valueA = this.formatUserName(a);
            valueB = this.formatUserName(b);
            break;
          case 'organization':
            valueA = a.organization || '';
            valueB = b.organization || '';
            break;
          case 'company':
            valueA = a.company || '';
            valueB = b.company || '';
            break;
          case 'position':
            valueA = a.position || '';
            valueB = b.position || '';
            break;
          case 'user_type':
            valueA = a.user_type || '';
            valueB = b.user_type || '';
            break;
          // Числовые ключи, а не строки: ISO-даты сравнились бы лексикографически
          // и разъехались бы на разной длине дробной части. Не заходившие получают
          // -Infinity, то есть читаются как «бесконечно давно» и держатся в том же
          // конце списка, что и самые старые визиты.
          case 'last_seen':
            valueA = lastSeenSortKey(a);
            valueB = lastSeenSortKey(b);
            break;
          default:
            return 0;
        }
        
        if (valueA < valueB) {
          return this.sortDirection === 'asc' ? 1 : -1;
        }
        if (valueA > valueB) {
          return this.sortDirection === 'asc' ? -1 : 1;
        }
        return 0;
      });
    },
    createPasswordRules() {
      return evaluatePassword(this.passwordPolicy, this.newUser.password || '');
    },
    createPasswordValid() {
      return passwordMeetsPolicy(this.passwordPolicy, this.newUser.password || '');
    },
    changePasswordRules() {
      return evaluatePassword(this.passwordPolicy, (this.selectedUser && this.selectedUser.newPassword) || '');
    },
    changePasswordValid() {
      return passwordMeetsPolicy(this.passwordPolicy, (this.selectedUser && this.selectedUser.newPassword) || '');
    },
    canCreateUser() {
      return (
        this.newUser.username &&
        this.newUser.password &&
        this.createPasswordValid &&
        this.newUser.type_id &&
        this.hasOrgOrCompany
      );
    },
    hasOrgOrCompany() {
      return Boolean(this.newUser.organization_id || this.newUser.company_id);
    }
  },
  watch: {
    // После рефетча списка (роль/группы/блокировка менялись в модалке прав)
    // пере-резолвим открытую карточку на свежий объект из allUsers. Иначе
    // selectedUser держит копию старого user и роль/права в карточке остаются
    // устаревшими до перезагрузки страницы.
    allUsers(list) {
      if (!this.selectedUser) return;
      const fresh = list.find((u) => u.username === this.selectedUser.username);
      if (fresh) this.selectedUser = { ...fresh, newPassword: this.selectedUser.newPassword || '' };
    },
    // Подрезаем выбор до видимых строк (поиск/смена архив-режима могли скрыть часть).
    sortedUsers(list) {
      if (!this.selectedUsernames.length) return;
      const visible = new Set(list.map(u => u.username));
      const pruned = this.selectedUsernames.filter(n => visible.has(n));
      if (pruned.length !== this.selectedUsernames.length) this.selectedUsernames = pruned;
    },
  },
  async created() {
    // Подтягиваем актуальные справочники из stores. Дальнейшая синхронизация
    // (после CRUD в OrganizationsManagement/CompaniesManagement) идёт через
    // pinia reactivity - listener'ы на window.event больше не нужны.
    await Promise.all([
      this.fetchOrganizations(),
      this.fetchCompanies(),
      this.fetchUserTypes(),
      this.fetchCurrentUser()
    ]);
    this.loadPasswordPolicy();
  },
  mounted() {
    this.fetchAllUsers();
    this.loadDraft();
    this.presenceTimer = setInterval(this.tickPresence, PRESENCE_TICK_MS);
    this.presencePollTimer = setInterval(this.pollPresence, PRESENCE_POLL_MS);
  },
  beforeUnmount() {
    clearInterval(this.presenceTimer);
    clearInterval(this.presencePollTimer);
    this.presenceTimer = null;
    this.presencePollTimer = null;
  },

  methods: {
    isOnline,
    formatSeenShort,
    seenTitle,

    // Пересчёт подписей присутствия из уже загруженного last_seen: запросов не делает,
    // поэтому тик частый - иначе «в сети» висело бы у ушедшего до следующего опроса.
    tickPresence() {
      this.presenceNow = Date.now();
    },

    // Тихий перезапрос списка: без спиннера и тостов, чтобы присутствие оживало само.
    // Пропускаем, когда открыта карточка или модалка bulk-операции: watch allUsers
    // пере-резолвит selectedUser из свежего ответа и затёр бы незасохранённый ввод
    // формы. В скрытой вкладке не опрашиваем вовсе - смотреть там всё равно некому.
    pollPresence() {
      if (document.hidden) return;
      if (this.showEditModal || this.showCreateModal) return;
      if (this.bulkModalVisible || this.bulkConfirmVisible || this.banModalVisible || this.unbanConfirmVisible) return;
      this.fetchAllUsers();
    },

    ...mapActions(useOrganizationsStore, {
      fetchOrganizations: 'fetchOrganizations',
      refreshOrganizations: 'refresh',
    }),
    ...mapActions(useCompaniesStore, {
      fetchCompanies: 'fetchCompanies',
      refreshCompanies: 'refresh',
    }),

    // onPhoneInput применяет российскую маску к введённому телефону и записывает
    // результат обратно в reactive model. Используется для newUser/selectedUser
    // чтобы явно триггерить saveDraft (где он нужен) отдельно.
    onPhoneInput(event, modelKey) {
      this[modelKey].phone = applyPhoneMask(event.target, event, this[modelKey].phone)
      if (modelKey === 'newUser') {
        this.saveDraft()
      }
    },

    async refreshAllData() {
      this.refreshing = true;
      try {
        await Promise.all([
          this.fetchOrganizations(),
          this.fetchCompanies(),
          this.fetchUserTypes(),
          this.fetchAllUsers()
        ]);
      } finally {
        this.refreshing = false;
      }
    },

    onArchiveModeChange(value) {
      this.listMode = value;
      this.selectedUser = null;
      // Наборы операций активных и архивных разные - выбор не переносим. Режим «В сети»
      // тоже сбрасывает выбор: набор строк меняется сам по себе, по мере ухода людей.
      this.clearSelection();
    },

    // --- Групповой выбор (по username) ---
    isSelected(username) {
      return this.selectedUsernames.includes(username);
    },
    toggleSelect(username) {
      const i = this.selectedUsernames.indexOf(username);
      if (i === -1) this.selectedUsernames.push(username);
      else this.selectedUsernames.splice(i, 1);
    },
    // onRowCheck: обычный клик - toggle; shift-клик - диапазон от якоря до текущей
    // строки включительно (см. OrganizationsManagement). Якорь по username,
    // переиндексируется через findIndex - устойчив к пересортировке. @click без
    // .prevent: нативный тоггл синхронизирует :checked кликнутого чекбокса.
    onRowCheck(user, index, event) {
      if (event.shiftKey && window.getSelection) window.getSelection().removeAllRanges();
      if (event.shiftKey && this.lastSelectedUsername != null && this.lastSelectedUsername !== user.username) {
        const anchor = this.sortedUsers.findIndex(u => u.username === this.lastSelectedUsername);
        if (anchor !== -1) {
          const target = !this.isSelected(user.username);
          const [from, to] = anchor < index ? [anchor, index] : [index, anchor];
          for (let i = from; i <= to; i += 1) {
            const name = this.sortedUsers[i].username;
            const sel = this.isSelected(name);
            if (target && !sel) this.selectedUsernames.push(name);
            else if (!target && sel) this.selectedUsernames.splice(this.selectedUsernames.indexOf(name), 1);
          }
          this.lastSelectedUsername = user.username;
          return;
        }
      }
      this.toggleSelect(user.username);
      this.lastSelectedUsername = user.username;
    },
    toggleSelectAll() {
      this.selectedUsernames = this.allSelected ? [] : this.sortedUsers.map(u => u.username);
      this.lastSelectedUsername = null;
    },
    clearSelection() {
      this.selectedUsernames = [];
      this.lastSelectedUsername = null;
      this.pendingBulkOp = null;
    },

    // --- Групповые операции ---
    // архив/восстановление - через ConfirmationModal; тип/организация/компания -
    // через UserBulkOperationsModal (нужен выбор значения).
    startBulkOperation(operation) {
      this.pendingBulkOp = operation;
      if (operation === 'archive' || operation === 'restore') {
        this.bulkConfirmVisible = true;
      } else {
        this.bulkModalVisible = true;
      }
    },
    cancelBulkConfirm() {
      if (this.bulkSubmitting) return;
      this.bulkConfirmVisible = false;
      this.pendingBulkOp = null;
    },
    cancelBulkModal() {
      if (this.bulkSubmitting) return;
      this.bulkModalVisible = false;
      this.pendingBulkOp = null;
    },
    // Применение операции с выбором значения (тип/организация/компания).
    async applyBulk(value) {
      const names = [...this.selectedUsernames];
      const op = this.pendingBulkOp;
      if (this.bulkSubmitting) return;
      if (!names.length || value === null || value === undefined) {
        this.bulkModalVisible = false;
        this.pendingBulkOp = null;
        return;
      }
      this.bulkSubmitting = true;
      let result;
      try {
        if (op === 'type') result = await bulkUpdateUsersType(names, value);
        else if (op === 'organization') result = await bulkAssignUsersOrganization(names, value);
        else if (op === 'company') result = await bulkAssignUsersCompany(names, value);
        else {
          this.bulkSubmitting = false;
          this.bulkModalVisible = false;
          this.pendingBulkOp = null;
          return;
        }
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось выполнить групповую операцию', type: 'error' });
        this.bulkSubmitting = false;
        return;
      }
      this.bulkSubmitting = false;
      if (this.handleBulkResult(op, result, names.length)) {
        this.bulkModalVisible = false;
        this.pendingBulkOp = null;
      }
    },
    async applyBulkArchiveRestore() {
      const names = [...this.selectedUsernames];
      const op = this.pendingBulkOp;
      if (this.bulkSubmitting) return;
      if (!names.length || (op !== 'archive' && op !== 'restore')) {
        this.bulkConfirmVisible = false;
        this.pendingBulkOp = null;
        return;
      }
      this.bulkSubmitting = true;
      let result;
      try {
        result = op === 'archive' ? await bulkArchiveUsers(names) : await bulkRestoreUsers(names);
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось выполнить групповую операцию', type: 'error' });
        this.bulkSubmitting = false;
        return;
      }
      this.bulkSubmitting = false;
      if (this.handleBulkResult(op, result, names.length)) {
        this.bulkConfirmVisible = false;
        this.pendingBulkOp = null;
      }
    },
    // --- Групповой бан/разбан (чувствительно: сессии рвутся, супер-админ и себя нельзя) ---
    openBulkBan() {
      this.banReason = '';
      this.banModalVisible = true;
    },
    cancelBulkBan() {
      if (this.bulkSubmitting) return;
      this.banModalVisible = false;
    },
    async applyBulkBan() {
      const names = [...this.selectedUsernames];
      if (this.bulkSubmitting || !names.length) return;
      this.bulkSubmitting = true;
      let result;
      try {
        result = await bulkBanUsers(names, this.banReason);
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось выполнить блокировку', type: 'error' });
        this.bulkSubmitting = false;
        return;
      }
      this.bulkSubmitting = false;
      if (this.handleBulkResult('ban', result, names.length)) {
        this.banModalVisible = false;
        this.banReason = '';
      }
    },
    openBulkUnban() {
      this.unbanConfirmVisible = true;
    },
    async applyBulkUnban() {
      const names = [...this.selectedUsernames];
      if (this.bulkSubmitting || !names.length) {
        this.unbanConfirmVisible = false;
        return;
      }
      this.bulkSubmitting = true;
      let result;
      try {
        result = await bulkUnbanUsers(names);
      } catch {
        useDeletionsStore().notify({ prefix: 'Не удалось выполнить разблокировку', type: 'error' });
        this.bulkSubmitting = false;
        return;
      }
      this.bulkSubmitting = false;
      if (this.handleBulkResult('unban', result, names.length)) {
        this.unbanConfirmVisible = false;
      }
    },
    // Разбор BulkOpResult: полный успех -> notify, частичный -> ui.warning с
    // перечнем непрошедших. false при ошибке-envelope (держим модалку для повтора).
    handleBulkResult(op, result, total) {
      if (!result || typeof result.success_count !== 'number') {
        useDeletionsStore().notify({ prefix: result?.message || 'Не удалось выполнить групповую операцию', type: 'error' });
        return false;
      }
      const label = {
        restore: 'Восстановлено',
        archive: 'Архивировано',
        type: 'Тип изменён',
        organization: 'Организация назначена',
        company: 'Компания назначена',
        ban: 'Заблокировано',
        unban: 'Разблокировано',
      }[op] || 'Обновлено';
      if (result.error_count > 0) {
        const failed = (result.errors || []).map(e => e.name || `#${e.id}`).join(', ');
        useDeletionsStore().notify({ prefix: 'Выполнено ', bold: `${result.success_count} из ${total}`, suffix: `. Не удалось: ${failed}`, type: 'warning' });
      } else {
        useDeletionsStore().notify({ prefix: `${label}: `, bold: String(result.success_count) });
      }
      this.clearSelection();
      this.fetchAllUsers();
      return true;
    },

    async restoreUser(user) {
      try {
        const response = await apiRequest(`/users/${user.username}/restore`, { method: 'POST' });
        if (response.ok) {
          this.closeEditModal();
          this.selectedUser = null;
          this.$emit('fetch-users');
          this.refreshOrganizations();
          this.refreshCompanies();
          useDeletionsStore().notify({ prefix: 'Пользователь ', bold: user.username, suffix: ' восстановлен из архива' });
        } else {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось восстановить: ', bold: errorData.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error('Ошибка сети при восстановлении пользователя:', error);
        useDeletionsStore().notify({ prefix: 'Не удалось восстановить: ', bold: 'нет связи с сервером', type: 'error' });
      }
    },

    async resetOnboarding(user) {
      const ok = await useUiStore().confirm({
        title: 'Сбросить обучение?',
        message: `Пользователь «${user.username}» снова увидит автозапуск обучающего тура при следующем входе.`,
        confirmText: 'Сбросить',
        cancelText: 'Отмена',
        danger: false,
      });
      if (!ok) return;
      try {
        await resetOnboardingForUser(user.username);
        useDeletionsStore().notify({ prefix: 'Обучение сброшено для ', bold: user.username, suffix: ' — тур запустится снова при входе' });
      } catch (error) {
        useDeletionsStore().notify({ prefix: 'Не удалось сбросить обучение: ', bold: error?.message || 'ошибка', type: 'error' });
      }
    },

    openHistory(user) {
      this.historyForUser = user;
    },

    openAccess(user) {
      this.accessUser = user;
    },

    openAccessPlaces(user) {
      this.accessPlacesUser = user;
    },

    onAccessUpdated() {
      // Роль/группы/блокировка/индивидуальные права изменились - перечитываем список.
      this.$emit('fetch-users');
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

    formatUserName(user) {
      const formatted = formatShortName(user);
      return formatted || '-';
    },

    onSelectOrganization(id) {
      this.selectedUser.organization_id = id;
      this.updateUserOrganization(this.selectedUser);
    },

    onSelectCompany(id) {
      this.selectedUser.company_id = id;
      this.updateUserCompany(this.selectedUser);
    },

    onSelectUserType(id) {
      this.selectedUser.type_id = id;
      this.updateUserType(this.selectedUser);
    },

    onSelectNewUserType(id) {
      this.newUser.type_id = id;
      this.saveDraft();
    },

    onSelectNewUserOrg(id) {
      this.newUser.organization_id = id;
      this.saveDraft();
    },

    onSelectNewUserCompany(id) {
      this.newUser.company_id = id;
      this.saveDraft();
    },
    
    // Методы для работы с черновиком
    saveDraft() {
      localStorage.setItem('newUserDraft', JSON.stringify(this.newUser));
    },
    
    loadDraft() {
      const saved = localStorage.getItem('newUserDraft');
      if (saved) {
        try {
          this.newUser = JSON.parse(saved);
        } catch (e) {
          console.error('Error loading draft:', e);
          localStorage.removeItem('newUserDraft');
        }
      }
    },
    
    clearDraft() {
      localStorage.removeItem('newUserDraft');
    },
    
    openCreateModal() {
      this.loadDraft();
      this.showCreateModal = true;
      this.$nextTick(() => {
        if (this.$refs.usernameInput) {
          this.$refs.usernameInput.focus();
        }
      });
    },
    
    closeCreateModal() {
      this.showCreateModal = false;
    },
    
    handleUserCreated() {
      this.showCreateModal = false;
      this.clearDraft();
      this.resetNewUser();
      this.$emit('fetch-users');
      // Обновляем оба представления (items + itemsWithUsers) - чтобы синхронно
      // отрисовался user_count в OrganizationsManagement/CompaniesManagement.
      this.refreshOrganizations();
      this.refreshCompanies();
    },
    
    resetNewUser() {
      this.newUser = {
        username: '',
        password: '',
        last_name: '',
        first_name: '',
        middle_name: '',
        email: '',
        phone: '',
        position: '',
        type_id: null,
        organization_id: null,
        company_id: null
      };
      this.clearDraft();
    },
    
    async createUser() {
      if (!this.canCreateUser) return;

      try {
        // Админская регистрация через POST /users (JWT-защищённый).
        // Публичный /register намеренно не экспонируется.
        const response = await apiRequest("/users", {
          method: "POST",
          body: JSON.stringify({
            username: this.newUser.username,
            password: this.newUser.password,
            last_name: this.newUser.last_name || null,
            first_name: this.newUser.first_name || null,
            middle_name: this.newUser.middle_name || null,
            email: this.newUser.email || null,
            phone: this.newUser.phone || null,
            position: this.newUser.position || null,
            type_id: this.newUser.type_id,
            organization_id: this.newUser.organization_id || null,
            company_id: this.newUser.company_id || null
          }),
        });

        if (response.ok) {
          const createdName = this.newUser.username;
          this.handleUserCreated();
          useDeletionsStore().notify({ prefix: 'Пользователь ', bold: createdName, suffix: ' создан' });
        } else {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось создать пользователя: ', bold: errorData.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error("Ошибка сети при создании пользователя:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось создать пользователя: ', bold: 'нет связи с сервером', type: 'error' });
      }
    },
    
    async updateUserInfo(user) {
      try {
        const response = await apiRequest(`/users/${user.username}/info`,
          {
            method: "PUT",
            body: JSON.stringify({
              last_name: user.last_name || null,
              first_name: user.first_name || null,
              middle_name: user.middle_name || null,
              position: user.position || null,
              email: user.email || null,
              phone: user.phone || null,
              is_important: !!user.is_important,
            }),
          }
        );

        if (!response.ok) {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось обновить данные: ', bold: errorData.message || 'ошибка', type: 'error' });
          this.$emit('fetch-users');
        } else {
          this.$emit('user-updated', user);
        }
      } catch (error) {
        console.error("Ошибка сети при обновлении информации:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось обновить данные: ', bold: 'нет связи с сервером', type: 'error' });
        this.$emit('fetch-users');
      }
    },
    
    confirmDeleteUser(user) {
      // Открываем ConfirmationModal вместо window.confirm. Реальное удаление - performDeleteUser.
      this.deleteConfirmUser = user;
    },

    async performDeleteUser() {
      const user = this.deleteConfirmUser;
      this.deleteConfirmUser = null;
      if (!user) return;
      try {
        const response = await apiRequest(`/users/${user.username}`,
          {
            method: "DELETE",
          }
        );

        if (response.ok) {
          this.closeEditModal();
          this.selectedUser = null;
          this.$emit('fetch-users');
          // user_count меняется - подтягиваем оба представления.
          this.refreshOrganizations();
          this.refreshCompanies();
          useDeletionsStore().notify({ prefix: 'Пользователь ', bold: user.username, suffix: ' удалён' });
        } else {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось удалить пользователя: ', bold: errorData.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error("Ошибка сети при удалении пользователя:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось удалить пользователя: ', bold: 'нет связи с сервером', type: 'error' });
      }
    },
    
    generatePassword(user) {
      user.newPassword = buildPassword(this.passwordPolicy);
      this.showNewPass = true;
    },
    
    async loadPasswordPolicy() {
      try {
        this.passwordPolicy = await getPasswordPolicy();
      } catch (error) {
        console.error('Не удалось загрузить политику паролей:', error);
        this.passwordPolicy = { ...DEFAULT_PASSWORD_POLICY };
      }
    },

    async fetchUserTypes() {
      try {
        const response = await apiRequest("/user-types", {
        });
        if (response.ok) {
          this.userTypes = await response.json();
        }
      } catch (error) {
        console.error("Error fetching user types:", error);
      }
    },
    
    checkInputLanguage(event) {
      if (!event || typeof event.getModifierState !== 'function') return;
      
      const isRussian = /[а-яА-ЯЁё]/.test(event.key);
      this.currentLanguage = isRussian ? 'RU' : 'EN';
      this.isCapsLockOn = event.getModifierState('CapsLock');
    },
    
    async updateUserType(user) {
      try {
        const response = await apiRequest(`/users/${user.username}/type`,
          {
            method: "PUT",
            body: JSON.stringify({ type_id: user.type_id }),
          }
        );

        if (!response.ok) {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось сменить тип: ', bold: errorData.message || 'ошибка', type: 'error' });
          this.$emit('fetch-users');
        } else {
          const type = this.userTypes.find(t => t.id === user.type_id);
          if (type) user.user_type = type.name;
          this.$emit('user-updated', user);
        }
      } catch (error) {
        console.error("Ошибка сети при обновлении типа пользователя:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось сменить тип: ', bold: 'нет связи с сервером', type: 'error' });
        this.$emit('fetch-users');
      }
    },
    
    async updateUserOrganization(user) {
      try {
        const response = await apiRequest(`/users/${user.username}/organization`,
          {
            method: "PUT",
            body: JSON.stringify({ organization_id: user.organization_id }),
          }
        );

        if (!response.ok) {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось сменить организацию: ', bold: errorData.message || 'ошибка', type: 'error' });
          this.$emit('fetch-users');
        } else {
          const org = this.organizations.find(o => o.id === user.organization_id);
          if (org) user.organization = org.name;
          this.$emit('user-updated', user);
        }
      } catch (error) {
        console.error("Ошибка сети при обновлении организации:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось сменить организацию: ', bold: 'нет связи с сервером', type: 'error' });
        this.$emit('fetch-users');
      }
    },
    
    async updateUserCompany(user) {
      try {
        const response = await apiRequest(`/users/${user.username}/company`,
          {
            method: "PUT",
            body: JSON.stringify({ company_id: user.company_id }),
          }
        );

        if (!response.ok) {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось сменить компанию: ', bold: errorData.message || 'ошибка', type: 'error' });
          this.$emit('fetch-users');
        } else {
          const comp = this.companies.find(c => c.id === user.company_id);
          if (comp) user.company = comp.name;
          this.$emit('user-updated', user);
        }
      } catch (error) {
        console.error("Ошибка сети при обновлении компании:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось сменить компанию: ', bold: 'нет связи с сервером', type: 'error' });
        this.$emit('fetch-users');
      }
    },
    
    async changeUserPassword(user) {
      if (!user.newPassword) {
        useDeletionsStore().notify({ bold: 'Введите новый пароль', type: 'error' });
        return;
      }

      if (!passwordMeetsPolicy(this.passwordPolicy, user.newPassword)) {
        useDeletionsStore().notify({ bold: 'Пароль не соответствует требованиям', type: 'error' });
        return;
      }

      try {
        const response = await apiRequest(`/users/${user.username}/password`,
          {
            method: "PUT",
            body: JSON.stringify({ password: user.newPassword }),
          }
        );

        if (response.ok) {
          user.newPassword = "";
          this.$emit('fetch-users');
          useDeletionsStore().notify({ prefix: 'Пароль пользователя ', bold: user.username, suffix: ' изменён' });
        } else {
          const errorData = await response.json();
          useDeletionsStore().notify({ prefix: 'Не удалось изменить пароль: ', bold: errorData.message || 'ошибка', type: 'error' });
        }
      } catch (error) {
        console.error("Ошибка сети при изменении пароля:", error);
        useDeletionsStore().notify({ prefix: 'Не удалось изменить пароль: ', bold: 'нет связи с сервером', type: 'error' });
      }
    },
    
    fetchAllUsers() {
      this.$emit('fetch-users');
    },
    
    selectUser(user) {
      this.selectedUser = { ...user };
      this.activeTab = 'profile';
      this.showEditModal = true;
    },

    closeDetails() {
      this.closeEditModal();
      this.selectedUser = null;
    },

    sortBy(field) {
      if (this.sortField === field) {
        this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortField = field;
        this.sortDirection = 'desc';
      }
    }
  }
};
</script>

<style scoped>
/* --- Групповой выбор: панель, чекбоксы, pill-кнопки (эталон OrganizationsManagement) --- */
.bulk-bar {
  /* Оверлей поверх .management-header (не reflow - список не прыгает при выборе,
     урок #510). Высота = высоте шапки (50px), карточка - position:relative. */
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 6;
  display: flex;
  align-items: center;
  gap: 14px;
  height: 50px;
  padding: 0 20px;
  border-bottom: 1px solid var(--border);
  background: var(--accent-tint);
  overflow-x: auto;
  overflow-y: hidden;
}
.bulk-count {
  font-size: 14px;
  font-weight: 600;
  color: var(--accent-text);
  white-space: nowrap;
}
.bulk-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: nowrap;
  margin-left: auto;
}
.bulk-actions .pill {
  flex: 0 0 auto;
  white-space: nowrap;
}
.bulk-clear {
  color: var(--text-muted);
  border-color: color-mix(in srgb, var(--accent) 25%, var(--surface));
}
.bulk-clear:hover {
  background: var(--surface-2);
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
.bulk-check {
  width: 15px;
  height: 15px;
  cursor: pointer;
  accent-color: var(--accent-text);
  margin: 0;
}
.bulk-ban-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.bulk-ban-warn {
  margin: 0;
  font-size: 14px;
  color: var(--text-muted);
  line-height: 1.5;
}
.bulk-ban-warn b {
  color: var(--danger-text);
}
.bulk-ban-label {
  font-size: 0.78em;
  color: var(--text-muted);
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
}
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
.pill-ghost {
  background: var(--surface);
  color: var(--accent-text);
  border: 1px solid var(--accent);
}
.pill-ghost:hover {
  background: var(--accent-tint);
}
.pill-danger {
  background: var(--surface);
  color: var(--danger-text);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}
.pill-danger:hover {
  background: var(--danger-bg);
  border-color: var(--danger);
}
.pill-restore {
  background: var(--success);
  color: var(--fill-text);
}
.pill-restore:hover {
  background: color-mix(in srgb, var(--success) 85%, var(--text));
}

.user-management {
  position: relative; /* контекст для оверлей-панели .bulk-bar (top:0 поверх шапки) */
  background-color: var(--surface);
  border-radius: 30px;
  border: 1px solid var(--border);
  overflow: hidden;
  width: 100%;
}

.management-header {
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0px 20px;
  height: 50px;
}

.management-title {
  font-size: 1.1em;
  margin: 0;
  font-weight: 600;
  color: var(--text);
}

.search-container {
  display: flex;
  align-items: center;
  gap: 8px;
}

.users-container {
  display: flex;
  height: fit-content;
  width: 100%;
  /* Горизонтальная деградация живёт здесь, а не на .users-body: у тела свой
     overflow-y, и его же горизонтальный скролл увёл бы строки из-под шапки.
     Скролля контейнер, шапка и строки едут вместе. */
  overflow-x: auto;
}

.users-list {
  flex: 1 1 auto;
  /* Сумма минимумов восьми колонок с падингами ячеек и строки. Ниже этой ширины
     список не сжимается, а .users-container отдаёт честный горизонтальный скролл:
     %-ширины иначе схлопывают текст колонки в ноль вместо прокрутки. */
  min-width: 770px;
  display: flex;
  flex-direction: column;
}

.users-footer {
  padding: 6px 16px;
  border-top: 1px solid var(--border);
  text-align: right;
  background: var(--accent-tint);
  flex-shrink: 0;
}

.users-footer .items-count {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}

.type-badge {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 500;
  background: var(--accent-tint);
  color: var(--accent-text);
}

.archive-dropdown {
  min-width: 140px;
}

.user-item.inactive {
  background: var(--surface-2);
  color: var(--text-muted);
}

.inactive-badge {
  margin-left: 6px;
  font-size: 11px;
  color: var(--text-muted);
  font-style: italic;
}

.archive-badge {
  background: var(--text-muted);
  color: var(--surface);
  padding: 4px 12px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
}

/* Заголовок таблицы */
.users-header {
  border-bottom: 1px solid var(--border);
  padding: 12px 16px;
  flex-shrink: 0;
}

.header-row {
  display: flex;
  width: 100%;
}

.header-col {
  font-weight: 500;
  color: var(--text-muted);
  text-align: left;
  padding: 0 4px;
  font-size: 14px;
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
  filter: var(--icon-ink-filter);
}

.sort-icon {
  width: 12px;
  height: 12px;
  transition: .2s;
}

.sort-icon.sorted {
  filter: var(--icon-ink-filter);
}

.sort-icon.desc {
  transform: rotate(180deg);
}

.active-sort {
  color: var(--text) !important;
  font-weight: 500 !important;
}

/* Колонки с фиксированной шириной */
/* check-col 6% забюджетирован в сумму 100% (6+12+14+16+13+13+11+15).
   seen-col шире прочих узких: подпись из двух единиц («3 мин 20 с», «5 мес 12 дн»)
   длиннее одиночной, а обрезать её ellipsis'ом значит терять младшую единицу.
   Проценты под неё сняты с ФИО и должности, но НЕ с org-col: там живут длинные
   названия отделов («Технический департамент»), которые сразу уходят в ellipsis. */
.login-col { width: 12%; min-width: 100px; }
.name-col { width: 14%; min-width: 100px; }
.org-col { width: 16%; min-width: 110px; }
.company-col { width: 13%; min-width: 100px; }
.position-col { width: 13%; min-width: 95px; }
.type-col { width: 11%; min-width: 90px; }
.seen-col { width: 15%; min-width: 104px; }

/* Ячейка присутствия: точка и подпись в одну строку, подпись не переносится -
   иначе строка таблицы прыгала бы по высоте на «12 мин назад». */
.seen-col {
  display: flex;
  align-items: center;
  gap: 6px;
}

.seen-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Счётчик присутствия в шапке блока. Не жирный и не крупный: подпись к заголовку,
   а не второй заголовок. */
.online-count {
  margin-left: 10px;
  font-size: 13px;
  font-weight: 400;
  color: var(--text-muted);
  white-space: nowrap;
}

/* Тело таблицы */
.users-body {
  overflow-y: auto;
  flex-grow: 1;
  height: 258px;
  max-height: 258px;
}

.user-item {
  border-bottom: 1px solid var(--border);
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.user-item:hover {
  background-color: var(--surface-2);
}

.user-row {
  display: flex;
  width: 100%;
  padding: 12px 16px;
  align-items: center;
}

.user-col {
  padding: 0 4px;
  text-align: left;
  font-size: 14px;
}

.user-login {
  color: var(--accent-text);
  font-weight: 600;
  white-space: nowrap;
  overflow: visible;
  text-overflow: clip;
}

.truncate-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
  display: block;
}

.access-icon {
  width: 15px;
  height: 15px;
}

.delete-icon-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 25px;
  height: 25px;
  border: none;
  background: none;
  cursor: pointer;
  padding: 0;
  transition: opacity 0.2s;
}

.delete-icon-btn:hover {
  opacity: 0.7;
}

.delete-icon {
  width: 20px;
  height: 20px;
}

.details-grid-two-columns {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-bottom: 16px;
}

.details-column {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.full-width-groups {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.detail-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.detail-label {
  font-size: 0.85em;
  color: var(--text-muted);
}

/* Значения дропдаунов (Организация/Компания/Тип) в деталях - как обычный текст,
   вровень с соседними инпутами (а не полужирным весом BaseDropdown по умолчанию). */
.detail-group :deep(.base-dropdown__text) {
  font-weight: 400;
}

.full-width {
  width: 100%;
}

.password-group {
  margin-top: 8px;
}

.password-checklist {
  list-style: none;
  margin: 6px 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.password-checklist__item {
  font-size: 11px;
  color: var(--text-muted);
  font-family: 'Montserrat', sans-serif;
  transition: color 0.15s ease;
}

.password-checklist__item--ok {
  color: var(--success-text);
}

.password-input-container {
  display: flex;
  align-items: center;
  gap: 8px;
}

.password-input-sm {
  padding: 6px 10px;
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 14px;
  height: 32px;
  width: 150px;
  transition: border-color 0.2s;
}

.password-input-sm:focus {
  border-color: var(--accent);
  outline: none;
}

.password-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.generate-password-btn {
  padding: 6px 10px;
  background-color: var(--surface);
  border: 1px solid var(--border);
  border-radius: 50px;
  cursor: pointer;
  white-space: nowrap;
  font-size: 13px;
  height: 30px;
  transition: background-color 0.2s;
  color: var(--accent-text);
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 3px;
}

.generate-password-btn:hover {
  background: var(--border);
}

.generate-icon {
  width: 15px;
  height: 15px;
}

.save-password-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  background-color: var(--accent);
  color: var(--accent-contrast);
  border: none;
  border-radius: 6px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.save-password-btn:disabled {
  background-color: var(--border);
  cursor: not-allowed;
}

.save-password-btn:hover:not(:disabled) {
  background-color: var(--accent-hover);
}

.save-icon {
  width: 16px;
  height: 16px;
}

.input-hints {
  margin-top: 4px;
  font-size: 0.75em;
}

.language-hint {
  color: var(--text-muted);
}

.language-hint.warning {
  color: var(--danger-text);
  font-weight: bold;
}

.no-users {
  text-align: center;
  padding: 15px;
  color: var(--text-muted);
}

/* Шапка модалки редактирования */
.modal-title-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.modal-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--text);
}

.modal-subtitle {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}

.modal-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  /* Прижать кнопки «История»/«Сбросить обучение» к правому краю шапки (перед крестиком). */
  margin-left: auto;
}

/* Вкладки модалки редактирования */
.modal-tabs {
  display: flex;
  gap: 4px;
  border-bottom: 1px solid var(--border);
  margin-bottom: 20px;
}

.modal-tab {
  border: none;
  background: none;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-muted);
  padding: 8px 16px 12px;
  cursor: pointer;
  position: relative;
  transition: color 0.15s ease;
}

.modal-tab:hover {
  color: var(--text);
}

.modal-tab--active {
  color: var(--accent-text);
}

.modal-tab--active::after {
  content: '';
  position: absolute;
  left: 12px;
  right: 12px;
  bottom: -1px;
  height: 3px;
  border-radius: 3px 3px 0 0;
  background: var(--accent);
}

.tab-panel {
  animation: tab-fade 0.18s ease;
  /* Единая высота вкладок - размер окна не прыгает при переключении. Значение
     подобрано под самую высокую вкладку («Профиль»), короткие дотягиваются до неё. */
  min-height: 524px;
  /* flex-колонка: вкладка «История входов» тянет свою таблицу на всю высоту,
     футер (пагинация+легенда) остаётся внизу окна. */
  display: flex;
  flex-direction: column;
}

@keyframes tab-fade {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}

@media (prefers-reduced-motion: reduce) {
  .tab-panel {
    animation: none;
  }
}

/* Опасное действие внизу "Профиля" */
.danger-zone {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px dashed var(--border);
}

.danger-zone__hint {
  font-size: 12px;
  color: var(--text-muted);
}

/* Тело модалки редактирования */
.modal-body-inner {
  padding: 12px 24px 22px;
}

/* Тело модалки создания */
.create-user-body {
  padding: 24px;
}

.form-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}

.form-hint {
  font-size: 13px;
  color: var(--text);
  margin-bottom: 12px;
  padding: 8px 12px;
  border-radius: 8px;
  background: var(--accent-tint);
}

.form-hint--warning {
  background: var(--warning);
  color: var(--danger-text);
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.input-group.half {
  flex: 1 1 calc(50% - 8px);
  min-width: 220px;
}

.input-group.full {
  flex: 1 1 100%;
}

.input-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
}

.required {
  color: var(--danger-text);
}

.modal-input {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 15px;
  font-size: 14px;
  transition: border-color 0.2s ease;
  background: var(--surface);
}


.modal-input:focus {
  border-color: var(--accent);
  outline: none;
}

.input-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
}

.modal-btn {
  padding: 10px 24px;
  border: none;
  border-radius: 30px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  min-width: 100px;
}

.modal-btn--cancel {
  background: var(--surface-2);
  color: var(--text-muted);
  border: 1px solid var(--border);
}

.modal-btn--cancel:hover {
  background: var(--row-hover);
}

.modal-btn--confirm {
  background: var(--accent);
  color: var(--accent-contrast);
}

.modal-btn--confirm:hover:not(.modal-btn--disabled) {
  background: var(--accent-hover);
}

.modal-btn--disabled {
  background: var(--border);
  cursor: not-allowed;
}

/* Скроллбары */
.select-dropdown::-webkit-scrollbar,
.users-body::-webkit-scrollbar {
  width: 6px;
}

.select-dropdown::-webkit-scrollbar-track,
.users-body::-webkit-scrollbar-track {
  background: var(--surface-2);
  border-radius: 3px;
}

.select-dropdown::-webkit-scrollbar-thumb,
.users-body::-webkit-scrollbar-thumb {
  background: var(--border);
  border-radius: 3px;
}

.select-dropdown::-webkit-scrollbar-thumb:hover,
.users-body::-webkit-scrollbar-thumb:hover {
  background: var(--text-muted);
}

@media (max-width: 767.98px) {
  /* Построчный чекбокс в карточке - увеличенный тач-таргет. */
  .check-col {
    min-height: 44px;
  }
  /* rt-head-row прячет внутренний .header-row, но обёртка .users-header несёт
     свой border-bottom+padding и без этого осталась бы пустой полосой над
     карточками (у остальных справочников rt-head-row на самом заголовке, тут
     двухуровневая структура). */
  .users-header {
    display: none;
  }

  .users-container {
    flex-direction: column;
    height: auto;
    /* В card-режиме горизонтальной деградации нет: строки стали карточками, ширины
       колонок не действуют. Оставленный скролл давал бы пустую прокрутку на 760px. */
    overflow-x: visible;
  }

  /* Тот же откат для минимума ширины - иначе карточки распирают вьюпорт телефона. */
  .users-list {
    min-width: 0;
  }

  .users-body {
    height: auto;
    max-height: 300px;
  }

  /* В карточке ячейка несёт ТРИ элемента: подпись data-label (::before из
     responsive-tables.css), точку и текст. При space-between точка повисла бы
     ровно посередине строки - отжимаем подпись влево, чтобы точка с подписью
     времени держались парой справа, как значение любой другой ячейки. */
  .seen-col::before {
    margin-right: auto;
  }

  .seen-text {
    overflow: visible;
    text-overflow: clip;
  }

  /* Спейсинг карточек: rt-row сидит на .user-row, а не на v-for-корне
     .user-item - сиблинг-селектор .rt-row + .rt-row из responsive-tables.css
     поэтому не матчит (соседние .user-item, не .user-row), добираем тут. */
  .user-item + .user-item {
    margin-top: 8px;
  }

  .user-item {
    border-bottom: none;
  }

  .rt-row .truncate-text {
    white-space: normal;
    overflow: visible;
    text-overflow: clip;
  }

  .details-grid-two-columns {
    grid-template-columns: 1fr;
  }

  .password-input-container {
    flex-direction: column;
    align-items: flex-start;
  }

  .password-actions {
    justify-content: flex-end;
  }

  .management-header {
    height: auto;
    padding: 16px;
  }

  .search-container {
    width: 100%;
    justify-content: flex-end;
  }

  .archive-dropdown {
    min-width: 90px;
  }

  :deep(.search) {
    width: 110px;
  }

  .form-wrap {
    gap: 12px;
  }

  .input-group.half {
    flex: 1 1 100%;
    min-width: 0;
  }
}

/* Радиус окон редактирования/создания задаётся пропом radius у BaseModal
   (content-class телепортится в body, scoped :deep до него не достаёт). */
</style>

<!-- Глобальный (не scoped): контент BaseModal телепортится в body, scoped-хэш до
     него не достаёт - шапку окна редактирования целим напрямую по content-class. -->
<style>
.base-modal.user-edit-modal .base-modal__header {
  padding-left: 30px;
  padding-right: 30px;
}
</style>