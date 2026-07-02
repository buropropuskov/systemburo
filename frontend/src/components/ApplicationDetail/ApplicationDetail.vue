<template>
  <!-- Внешний контейнер для модального окна -->
  <div
    class="application-detail-overlay"
    @click.self="closeApplicationDetail"
  >
    <!-- Модальное окно пересылки -->
    <ForwardModal
      :show="showForwardModal"
      :all-users="allUsers"
      :responsible-users="responsibleUsers"
      :existing-approvers="approvers"
      :existing-viewers="viewers"
      :attachments="attachments"
      :is-sending="isForwarding"
      @close="closeForwardModal"
      @send="sendForwardRequest"
    />

    <div class="application-detail">
      <!-- Заголовок и кнопки -->
      <div class="detail-header">
        <div class="detail-header-left">
          <div class="detail-title-row">
            <h3 class="detail-title">
              Заявка {{ applicationData.application_number }}
            </h3>
            <div class="detail-datetime">
              {{ formatDateTime(applicationData.sending_datetime) }}
              <span class="weekday">{{ getWeekday(applicationData.sending_datetime) }}</span>
            </div>
            <!-- Кнопка пересылки (рядом с датой) -->
            <button
              v-if="mode === 'center' && canForwardApplication && can('action.forward.application')"
              class="forward-btn"
              data-testid="app-detail-button-forward"
              :disabled="updatingConfirmation || processingApplication"
              @click="forwardApplication"
            >
              <span
                v-if="updatingConfirmation || processingApplication"
                class="button-loading"
              />
              <span v-else>Переслать</span>
            </button>
          </div>
        </div>
        <div class="detail-header-right">
          <ApplicationActionBar
            v-if="mode !== 'center' || can('action.approve.application')"
            :application="applicationData"
            :current-user-id="currentUserId"
            :responsible-users="responsibleUsers"
            :approvers="approvers"
            :mode="mode"
            :processing="processingApplication"
            :updating-confirmation="updatingConfirmation"
            :action-comment="actionComment"
            :has-unoverridden-blacklist-flags="!!applicationData.has_unoverridden_blacklist_flags"
            :ready="actionsReady"
            @action-completed="handleActionCompleted"
            @processing-change="processingApplication = $event"
            @updating-confirmation-change="updatingConfirmation = $event"
            @comment-clear="clearCommentFromLocalStorage"
          >
            <template #user-actions>
              <BaseDropdown
                class="duplicate-dropdown"
                :options="duplicatePresets"
                :model-value="null"
                label-key="label"
                value-key="key"
                placeholder="Продублировать"
                @update:model-value="handleDuplicatePreset"
              />
              <button
                v-if="canWithdraw"
                class="withdraw-btn"
                @click="withdrawApplication"
              >
                Отозвать
              </button>
            </template>
          </ApplicationActionBar>

          <button
            class="close-detail-btn"
            @click="close"
          >
            ×
          </button>
        </div>
      </div>

      <div class="detail-content">
        <!-- Левая колонка - вложения -->
        <div
          class="detail-left-column"
          :class="{ collapsed: isLeftColumnCollapsed }"
        >
          <ApplicationAttachments 
            :application-id="applicationData.id"
            :attachments="attachments"
            :collapsed="isLeftColumnCollapsed"
            @attachment-selected="selectAttachment"
            @toggle-collapse="toggleLeftColumn"
          />
        </div>

        <!-- Центральная колонка - детали -->
        <div class="detail-main-column">
          <!-- Сообщение заявки -->
          <div class="message-section">
            <div class="message-section-header">
              <h4>Сообщение к заявке {{ applicationData.application_number }}</h4>
              <button
                v-if="hasMessage"
                type="button"
                class="lk-button lk-button--secondary message-open-btn"
                @click="showMessageModal = true"
              >
                Открыть в окне
              </button>
            </div>
            <div
              v-if="hasMessage"
              ref="messagePreview"
              class="message-content text-constructor-content message-preview"
              :class="{ 'is-clamped': messageClamped }"
              v-html="sanitizedMessage"
            />
            <div
              v-else
              class="message-content message-empty"
            >
              Сообщение отсутствует
            </div>
          </div>

          <ApplicationMessageModal
            :show="showMessageModal"
            :message="applicationData.message || ''"
            :application-number="applicationData.application_number"
            @close="showMessageModal = false"
          />

          <!-- Сообщения при пересылке (#967), видны всем получателям -->
          <ForwardMessages
            ref="forwardMessagesComponent"
            :application-id="applicationData.id"
          />

          <!-- Детали выбранного вложения -->
          <ApplicationAttachmentDetail
            v-if="selectedAttachment"
            :attachment="selectedAttachment"
            :cars="attachmentCars"
            :employees="attachmentEmployees"
            :items="attachmentItems"
            :loading="loadingAttachmentDetails"
            :can-override="isResponsibleUser"
            @open-vehicle="openVehicleModal"
            @open-employee="openEmployeeModal"
            @override-element="openOverrideModal"
          />
        </div>

        <!-- Правая колонка - информация о заявке и согласовании -->
        <div class="detail-right-column">
          <!-- Основная информация -->
          <div class="basic-info-section">
            <h4>Основная информация</h4>
            <div class="info-grid">
              <div class="info-row">
                <span class="info-label">Организация / Отдел:</span>
                <span class="info-value">{{ applicationData.organization_name }}</span>
              </div>
              <div
                v-if="applicationData.company_name"
                class="info-row"
              >
                <span class="info-label">Компания:</span>
                <span class="info-value">{{ applicationData.company_name }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">Отправитель:</span>
                <span class="info-value sender-value">
                  <span>{{ applicationData.sender_full_name || applicationData.sender_name }}</span>
                  <Badge
                    v-if="applicationData.sender_is_important"
                    variant="info"
                    size="sm"
                    class="sender-important-tag"
                  >
                    <svg
                      width="12"
                      height="12"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    ><polygon points="12 2 15 8.6 22 9.3 16.8 14 18.3 21 12 17.3 5.7 21 7.2 14 2 9.3 9 8.6" /></svg>
                    Важный
                  </Badge>
                </span>
              </div>
            </div>
          </div>

          <!-- Блок статуса заявки (для принятых/отказанных/завершенных/отозванных).
               Отозвана держим здесь же: после отзыва инфо о принятии не должно пропадать. -->
          <div
            v-if="['В работе', 'Отказано', 'Завершено', 'Отозвана'].includes(applicationData.status)"
            class="application-status-section"
          >
            <div class="status-header">
              <h4>Статус заявки</h4>
              <span
                class="status-mini-badge"
                :class="getStatusBadgeClass(applicationData.status)"
              >
                {{ applicationData.status }}
              </span>
            </div>
                        
            <!-- Для статусов В работе и Отказано -->
            <div
              v-if="applicationData.status === 'В работе' || applicationData.status === 'Отказано'"
              class="status-info"
            >
              <div
                v-if="applicationData.responsible_user_id"
                class="status-info-row"
              >
                <span class="status-info-label">{{ applicationData.status === 'В работе' ? 'Принял(-а):' : 'Отказал(а):' }}</span>
                <span class="status-info-value">{{ applicationData.responsible_name || 'Не указан' }}</span>
              </div>
              <div
                v-if="applicationData.confirmation_datetime"
                class="status-info-row"
              >
                <span class="status-info-label">Время:</span>
                <span class="status-info-value">{{ formatDateTime(applicationData.confirmation_datetime) }}</span>
              </div>
              <div class="status-info-row comment-row">
                <span class="status-info-label">Комментарий:</span>
                <div class="status-info-value comment-text">
                  {{ applicationData.responsible_comment || 'Комментария нет' }}
                </div>
              </div>
            </div>

            <!-- Для статуса Завершено (показываем и принятие, и завершение) -->
            <div
              v-else-if="applicationData.status === 'Завершено'"
              class="status-info"
            >
              <!-- Информация о принятии -->
              <div
                v-if="applicationData.responsible_name"
                class="status-info-row"
              >
                <span class="status-info-label">Принял(-а):</span>
                <span class="status-info-value">{{ applicationData.responsible_name }}</span>
              </div>
              <div
                v-if="applicationData.confirmation_datetime"
                class="status-info-row"
              >
                <span class="status-info-label">Время принятия:</span>
                <span class="status-info-value">{{ formatDateTime(applicationData.confirmation_datetime) }}</span>
              </div>
              <!-- Информация о завершении -->
              <div
                v-if="applicationData.completed_by_name"
                class="status-info-row"
              >
                <span class="status-info-label">Завершил(-а):</span>
                <span class="status-info-value">{{ applicationData.completed_by_name }}</span>
              </div>
              <div
                v-if="applicationData.completed_at"
                class="status-info-row"
              >
                <span class="status-info-label">Время завершения:</span>
                <span class="status-info-value">{{ formatDateTime(applicationData.completed_at) }}</span>
              </div>
              <!-- Комментарий к завершению (или общий) -->
              <div class="status-info-row comment-row">
                <span class="status-info-label">Комментарий:</span>
                <div class="status-info-value comment-text">
                  {{ applicationData.completion_comment || 'Комментария нет' }}
                </div>
              </div>
            </div>

            <!-- Для статуса Отозвана: если заявку успели принять до отзыва - показываем,
                 кто принял, чтобы информация не пропадала после отзыва. -->
            <div
              v-else-if="applicationData.status === 'Отозвана' && applicationData.responsible_user_id"
              class="status-info"
            >
              <div class="status-info-row">
                <span class="status-info-label">Принял(-а):</span>
                <span class="status-info-value">{{ applicationData.responsible_name || 'Не указан' }}</span>
              </div>
              <div
                v-if="applicationData.confirmation_datetime"
                class="status-info-row"
              >
                <span class="status-info-label">Время принятия:</span>
                <span class="status-info-value">{{ formatDateTime(applicationData.confirmation_datetime) }}</span>
              </div>
              <div
                v-if="applicationData.responsible_comment"
                class="status-info-row comment-row"
              >
                <span class="status-info-label">Комментарий:</span>
                <div class="status-info-value comment-text">
                  {{ applicationData.responsible_comment }}
                </div>
              </div>
            </div>
          </div>

          <!-- Поле для комментария (только для пользователей, которые еще не выполнили действие) -->
          <div
            v-if="canLeaveComment && !hasUserVoted && !isApproverActionDone && (mode !== 'center' || can('action.approve.application'))"
            class="comment-action-section"
          >
            <h4>Комментарий</h4>
            <textarea
              v-model="actionComment"
              class="comment-action-textarea"
              placeholder="Вы можете написать здесь комментарий (необязательно)"
              rows="3"
              @input="saveCommentToLocalStorage"
            />
          </div>

          <!-- Компонент согласования (без информации о принявшем) -->
          <ApplicationConfirmation 
            ref="confirmationComponent"
            :application="applicationData"
            :responsible-users="responsibleUsers"
            :current-user-id="currentUserId"
            :updating-confirmation="updatingConfirmation"
          />

          <div
            v-if="can('center.application_history')"
            class="history-button-section"
          >
            <ApplicationHistory
              ref="historyComponent"
              :application-id="applicationData.id"
              :application-number="applicationData.application_number"
              :current-user-id="currentUserId"
              :current-user-name="currentUserName"
              :application-organization="applicationData.organization_name"
              @application-updated="handleApplicationUpdate"
            />
          </div>
        </div>
      </div>
    </div>
    <VehicleDetailsModal
      v-if="showVehicleModal"
      :show="showVehicleModal"
      :vehicle="selectedVehicle"
      :all-unloading-places="allUnloadingPlaces"
      :license-plate-formats="licensePlateFormats"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      :show-car-features="true"
      :source="'application'"
      :can-override="isResponsibleUser"
      :can-cancel-override="canManageBlacklistOverride"
      @close="showVehicleModal = false"
      @override="onCardOverride('vehicle')"
      @cancel-override="onCardCancelOverride('vehicle')"
    />

    <EmployeeDetailsModal
      v-if="showEmployeeModal"
      :show="showEmployeeModal"
      :employee="selectedEmployee"
      :all-tables="allTables"
      :current-user-id="currentUserId"
      :current-user-name="currentUserName"
      :source="'application'"
      :can-override="isResponsibleUser"
      :can-cancel-override="canManageBlacklistOverride"
      @close="showEmployeeModal = false"
      @override="onCardOverride('employee')"
      @cancel-override="onCardCancelOverride('employee')"
    />

    <BlacklistOverrideModal
      :show="showOverrideModal"
      :flag="overrideFlag"
      :submitting="overrideSubmitting"
      @confirm="confirmOverride"
      @close="showOverrideModal = false"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { markAsRead } from '@/api/applications'
import { useDeletionsStore } from '@/stores/deletions'
import { useUiStore } from '@/stores/ui'
import { usePermissionsStore } from '@/stores/permissions'
import ApplicationAttachments from './ApplicationAttachments.vue'
import ApplicationConfirmation from './ApplicationConfirmation.vue'
import ApplicationHistory from './ApplicationHistory.vue'
import ForwardModal from './ForwardModal.vue'
import ForwardMessages from './ForwardMessages.vue'
import ApplicationActionBar from './ApplicationActionBar.vue'
import ApplicationAttachmentDetail from './ApplicationAttachmentDetail.vue'
import BlacklistOverrideModal from './BlacklistOverrideModal.vue'
import VehicleDetailsModal from '../CreateApplication/VehicleDetailsModal.vue'
import EmployeeDetailsModal from '../CreateApplication/EmployeeDetailsModal.vue'
import Badge from '@/components/ui/Badge.vue'
import BaseDropdown from '@/components/ui/BaseDropdown.vue'
import { sanitizeHtml } from '@/utils/sanitize'
import ApplicationMessageModal from './ApplicationMessageModal.vue'

export default {
    name: 'ApplicationDetail',
    components: {
        ApplicationAttachments,
        ApplicationConfirmation,
        ApplicationHistory,
        ForwardModal,
        ForwardMessages,
        ApplicationActionBar,
        ApplicationAttachmentDetail,
        BlacklistOverrideModal,
        VehicleDetailsModal,
        EmployeeDetailsModal,
        Badge,
        BaseDropdown,
        ApplicationMessageModal
    },
    props: {
        application: {
            type: Object,
            required: true
        },
        currentUserId: {
            type: Number,
            default: null
        },
        currentUserName: {
            type: String,
            default: ''
        },
        mode: {
            type: String,
            default: 'center'
        }
    },
    emits: ['close', 'confirmation-updated', 'duplicate', 'withdraw', 'application-updated', 'update-application', 'application-changed'],
    setup() {
        const permissionsStore = usePermissionsStore();
        return { permissionsStore };
    },
    data() {
        return {
            applicationData: { ...this.application },
            showMessageModal: false,
            messageClamped: false,
            attachments: [],
            selectedAttachment: null,
            attachmentCars: [],
            attachmentEmployees: [],
            attachmentItems: [],
            responsibleUsers: [],
            viewers: [],              // читатели
            updatingConfirmation: false,
            processingApplication: false,
            loadingAttachmentDetails: false,
            actionsReady: false,
            // Пресеты срока для дропдауна "Продублировать": проставляют дату действия
            // во все вложения дубля. 'other' - дублирует без даты (пользователь задаёт сам).
            duplicatePresets: [
                { key: 'nextMonth', label: 'На следующий месяц' },
                { key: 'tomorrow', label: 'На завтра' },
                { key: 'other', label: 'Другой срок' }
            ],
            isLeftColumnCollapsed: false,
            showForwardModal: false,
            isForwarding: false,
            allUsers: [],
            approvers: [],
            actionComment: '',
            lastUserComment: '',
            storageKey: '',
            allUnloadingPlaces: [],
            licensePlateFormats: [],
            allTables: [],
            showVehicleModal: false,
            showEmployeeModal: false,
            selectedVehicle: null,
            selectedEmployee: null,
            showOverrideModal: false,
            overrideFlag: null,
            overrideLabel: '',
            overrideSubmitting: false
        }
    },
    computed: {
        hasMessage() {
            const m = this.applicationData?.message;
            if (!m) return false;
            return m.includes('<img') || m.replace(/<[^>]*>/g, '').trim().length > 0;
        },

        sanitizedMessage() {
            return this.applicationData?.message ? sanitizeHtml(this.applicationData.message) : '';
        },

        isResponsibleUser() {
            if (!this.currentUserId || !this.responsibleUsers.length) return false;
            return this.responsibleUsers.some(user => user.id === this.currentUserId);
        },

        isApprover() {
            if (!this.currentUserId || !this.approvers.length) return false;
            return this.approvers.some(approver => approver.user_id === this.currentUserId);
        },

        // Отозвать свою заявку может только отправитель и только пока она не в
        // терминальном (закрытом) статусе - зеркалит BE-гейт WithdrawApplication (#951).
        canWithdraw() {
            const a = this.applicationData;
            if (!a || a.sender_user_id !== this.currentUserId) return false;
            return !['Завершено', 'Не согласовано', 'Отказано', 'Отозвана'].includes(a.status);
        },

        // Отменить подтверждение пропуска может ответственный по заявке ИЛИ принимающий -
        // зеркалит право DELETE /blacklist-overrides на бэке (шире, чем создание override).
        canManageBlacklistOverride() {
            return this.isResponsibleUser || this.isApprover;
        },

        // Зеркалит BE-проверку canForward (sender OR responsible). Согласующего не включаем
        // сознательно: isApprover - глобальная роль, видит все заявки, и на чужой forward
        // вернул бы 403. Отправителя тоже нет - в режиме "Центр" у него нет UI-пути к кнопке.
        canForwardApplication() {
            return this.isResponsibleUser && this.applicationData.status !== 'Отозвана';
        },

        hasUserVoted() {
            if (!this.currentUserId || !this.responsibleUsers.length) return false;
            const currentUser = this.responsibleUsers.find(user => user.id === this.currentUserId);
            return currentUser && currentUser.approval_status !== 'pending';
        },

        isApproverActionDone() {
            if (!this.isApprover || this.isResponsibleUser) return false;
            return this.applicationData.status === 'В работе' || this.applicationData.status === 'Отказано' || this.applicationData.status === 'Завершено';
        },

        userVoteStatus() {
            if (!this.currentUserId || !this.responsibleUsers.length) return null;
            const currentUser = this.responsibleUsers.find(user => user.id === this.currentUserId);
            
            if (!currentUser) return null;
            
            if (currentUser.approval_status === 'approved') {
                return {
                    text: 'Вы согласовали',
                    class: 'vote-approved'
                };
            } else if (currentUser.approval_status === 'rejected') {
                return {
                    text: 'Вы отказали',
                    class: 'vote-rejected'
                };
            }
            
            return null;
        },

        canLeaveComment() {
            if (this.processingApplication) return false;
            // Отозванную заявку нельзя ни принять/согласовать, ни прокомментировать (#951).
            if (this.applicationData.status === 'Отозвана') return false;

            if (this.isApprover && !this.isResponsibleUser) {
                return !this.isApproverActionDone;
            }
            
            if (this.isResponsibleUser) {
                return !this.hasUserVoted;
            }
            
            return false;
        },

    },
    watch: {
        application: {
            immediate: true,
            handler(newApplication, oldApplication) {
                if (newApplication && newApplication.id) {
                    this.applicationData = { ...newApplication };
                    this.showMessageModal = false;
                    this.storageKey = `app_comment_${this.currentUserId}_${newApplication.id}`;
                    this.loadCommentFromLocalStorage();
                    this.loadApplicationDetails(newApplication);
                    if (!oldApplication || oldApplication.id !== newApplication.id) {
                        markAsRead(newApplication.id).catch(() => {});
                    }
                }
            },
            deep: true
        },
        sanitizedMessage() {
            this.$nextTick(() => this.updateMessageClamp());
        }
    },
    mounted() {
        this.loadCommonData();
        this.$nextTick(() => this.updateMessageClamp());
    },
    methods: {
        can(key) {
            return this.permissionsStore.hasPermission(key);
        },
        updateMessageClamp() {
            const el = this.$refs.messagePreview;
            this.messageClamped = !!el && el.scrollHeight > el.clientHeight + 2;
        },

        handleActionCompleted({ success, message, type }) {
            const resolvedType = type || (success ? 'success' : 'error');
            // ActionBar шлёт ошибку как "Ошибка: ...", а карточка тоста уже даёт заголовок
            // "Ошибка" - снимаем префикс, чтобы не дублировать.
            const text = resolvedType === 'error' ? String(message ?? '').replace(/^Ошибка:\s*/, '') : message;
            useDeletionsStore().notify({ bold: text, type: resolvedType });
            if (success) {
                this.loadApplicationDetails(this.applicationData);
                if (this.$refs.historyComponent) {
                    this.$refs.historyComponent.loadHistory();
                }
                this.$emit('application-changed', this.applicationData);
            }
        },

        getStatusBadgeClass(status) {
            const classes = {
                'В работе': 'status-mini-work',
                'Отказано': 'status-mini-rejected',
                'Завершено': 'status-mini-completed',
                'Отозвана': 'status-mini-rejected'
            };
            return classes[status] || '';
        },

        saveCommentToLocalStorage() {
            if (this.storageKey && this.currentUserId) {
                localStorage.setItem(this.storageKey, this.actionComment);
            }
        },

        loadCommentFromLocalStorage() {
            if (this.storageKey && this.currentUserId) {
                const savedComment = localStorage.getItem(this.storageKey);
                if (savedComment) {
                    this.actionComment = savedComment;
                    this.lastUserComment = savedComment;
                }
            }
        },

        clearCommentFromLocalStorage() {
            if (this.storageKey) {
                localStorage.removeItem(this.storageKey);
            }
        },

        async loadApplicationDetails(application) {
            this.actionsReady = false;
            try {
                const [appResponse, attachmentsResponse, viewersResponse] = await Promise.all([
                    apiRequest(`/applications/${application.id}/details`, {
                        method: "GET",
                    }),
                    apiRequest(`/applications/${application.id}/attachments`, {
                        method: "GET",
                    }),
                    apiRequest(`/applications/${application.id}/viewers`, {
                        method: "GET",
                    })
                ]);

                if (appResponse.ok) {
                    const appData = await appResponse.json();
                    
                    this.applicationData = {
                        ...this.applicationData,
                        ...appData
                    };
                    
                    if (appData.responsible_users) {
                        this.responsibleUsers = appData.responsible_users.map(user => ({
                            ...user,
                            approval_status: user.approval_status || 'pending'
                        }));
                        
                        if (this.currentUserId) {
                            const currentUser = this.responsibleUsers.find(u => u.id === this.currentUserId);
                            if (currentUser && currentUser.approval_comment) {
                                this.actionComment = currentUser.approval_comment;
                                this.lastUserComment = currentUser.approval_comment;
                                this.saveCommentToLocalStorage();
                            } else {
                                this.loadCommentFromLocalStorage();
                            }
                        }
                    }
                    
                    if (this.$refs.confirmationComponent) {
                        this.$refs.confirmationComponent.$forceUpdate();
                    }
                }

                if (attachmentsResponse.ok) {
                    this.attachments = await attachmentsResponse.json();
                    if (this.attachments.length > 0) {
                        this.selectedAttachment = this.attachments[0];
                        await this.loadAttachmentDetails(this.selectedAttachment.id);
                    }
                }

                if (viewersResponse.ok) {
                    this.viewers = await viewersResponse.json();
                }

                // Списки всех пользователей и согласующих нужны только в "Центре заявок"
                // (пересылка, определение согласующего). Рядовому отправителю в ЛК их не
                // отдают (403) - не дёргаем админ-эндпоинты, иначе всплывает generic-тост
                // "Недостаточно прав для этого действия" при открытии своей же заявки.
                if (this.mode === 'center') {
                    await this.fetchAllUsers();
                    await this.fetchApprovers();
                }

            } catch (error) {
                console.error("Ошибка при загрузке деталей заявки:", error);
            } finally {
                // П.46: кнопки действий показываем только после загрузки ролей/согласующих,
                // иначе мигают не те кнопки пока responsibleUsers/approvers пустые.
                this.actionsReady = true;
            }
        },

        async fetchAllUsers() {
            try {
                // silent403 - на случай контекста без права: без пугающего тоста, деградируем тихо.
                const response = await apiRequest("/users/all", { silent403: true });
                if (response.ok) {
                    this.allUsers = await response.json();
                }
            } catch (error) {
                console.error("Error fetching users:", error);
            }
        },

        async fetchApprovers() {
            try {
                const response = await apiRequest("/application-approvers", { silent403: true });
                if (response.ok) {
                    this.approvers = await response.json();
                }
            } catch (error) {
                console.error("Error fetching approvers:", error);
            }
        },

        async loadAttachmentDetails(attachmentId) {
            if (!attachmentId) return;

            this.loadingAttachmentDetails = true;
            try {
                this.attachmentCars = [];
                this.attachmentEmployees = [];
                this.attachmentItems = [];

                const attachment = this.attachments.find(a => a.id === attachmentId);
                if (!attachment) return;

                switch (attachment.attachment_type) {
                    case 'cars': {
                        const carsResponse = await apiRequest(`/attachments/${attachmentId}/cars`, {
                            method: "GET",
                        });
                        if (carsResponse.ok) {
                            this.attachmentCars = await carsResponse.json();
                        }
                        break;
                    }
                    
                    case 'people': {
                        const employeesResponse = await apiRequest(`/attachments/${attachmentId}/employees`, {
                            method: "GET",
                        });
                        if (employeesResponse.ok) {
                            this.attachmentEmployees = await employeesResponse.json();
                        }
                        break;
                    }
                    
                    case 'items': {
                        const itemsResponse = await apiRequest(`/attachments/${attachmentId}/items`, {
                            method: "GET",
                        });
                        if (itemsResponse.ok) {
                            this.attachmentItems = await itemsResponse.json();
                        }
                        break;
                    }
                }
            } catch (error) {
                console.error("Ошибка при загрузке деталей вложения:", error);
            } finally {
                this.loadingAttachmentDetails = false;
            }
        },

        selectAttachment(attachment) {
            this.selectedAttachment = attachment;
            this.loadAttachmentDetails(attachment.id);
        },

        handleApplicationUpdate() {
            this.loadApplicationDetails(this.applicationData);
        },

        forwardApplication() {
            this.showForwardModal = true;
        },

        closeForwardModal() {
            this.showForwardModal = false;
        },

        async sendForwardRequest({ users = [], attachment_ids = [], message = '' } = {}) {
            if (users.length === 0) return;

            this.isForwarding = true;
            try {
                const usersToSend = users.map(user => ({
                    user_id: user.user_id,
                    required_approval: user.required_approval || false,
                    can_view: user.can_view !== undefined ? user.can_view : !user.required_approval
                }));

                const response = await apiRequest(`/applications/${this.applicationData.id}/forward`, {
                    method: "POST",
                    body: JSON.stringify({
                        users: usersToSend,
                        attachment_ids,
                        message
                    })
                });

                if (response.ok) {
                    useDeletionsStore().notify({ bold: 'Заявка переслана', type: 'success' });
                    this.closeForwardModal();

                    await this.loadApplicationDetails(this.applicationData);

                    if (this.$refs.historyComponent) {
                        this.$refs.historyComponent.loadHistory();
                    }

                    if (this.$refs.forwardMessagesComponent) {
                        this.$refs.forwardMessagesComponent.load();
                    }

                    this.$emit('application-changed', this.applicationData);

                } else {
                    const errorText = await response.text();
                    useDeletionsStore().notify({ prefix: 'Не удалось переслать: ', bold: errorText || 'ошибка', type: 'error' });
                }
            } catch (error) {
                console.error("Ошибка при пересылке заявки:", error);
                useDeletionsStore().notify({ prefix: 'Не удалось переслать: ', bold: 'ошибка сети', type: 'error' });
            } finally {
                this.isForwarding = false;
            }
        },

        async duplicateApplication(preset = 'other') {
            try {
                const fetchResults = await Promise.all(
                    this.attachments.map(attachment => {
                        const endpoint = {
                            cars: `/attachments/${attachment.id}/cars`,
                            people: `/attachments/${attachment.id}/employees`,
                            items: `/attachments/${attachment.id}/items`,
                        }[attachment.attachment_type];

                        if (!endpoint) return Promise.resolve({ attachment, data: [] });

                        return apiRequest(endpoint, { method: 'GET' })
                            .then(r => r.ok ? r.json() : [])
                            .then(data => ({ attachment, data }));
                    })
                );

                // Шаблоны вложений нужны для поля title (категория): BlankSelector
                // раскладывает бланки по категориям именно по attachment.title, и без
                // него восстановленный черновик не отображается (0/0 в категориях).
                const templatesResp = await apiRequest('/attachments', { method: 'GET' });
                const templates = templatesResp.ok ? await templatesResp.json() : [];

                const newAttachments = [];
                const vehiclesByAttachment = {};
                const employeesByAttachment = {};
                const itemsByAttachment = {};
                const attachmentDatesByAttachment = {};
                // Дата действия по выбранному пресету - одна на все вложения ('other' -> null, без даты).
                const presetDate = this.buildPresetDate(preset);

                for (const { attachment, data } of fetchResults) {
                    // local_id как в BlankSelector.addAttachment — числовой ключ без id существующего вложения
                    const localId = Date.now() + Math.random();
                    const template = templates.find(t => t.id === attachment.unique_attachment_id)
                        || templates.find(t => t.attachment_type === attachment.attachment_type);

                    newAttachments.push({
                        id: template ? template.id : attachment.unique_attachment_id,
                        local_id: localId,
                        template_id: template ? template.id : attachment.unique_attachment_id,
                        title: template ? template.title : null,
                        name: template ? template.name : attachment.attachment_name,
                        display_name: attachment.attachment_display_name || (template && template.display_name),
                        attachment_type: attachment.attachment_type,
                        instruction: template ? template.instruction : null,
                        created_at: new Date().toISOString(),
                        is_active: true,
                    });

                    // Пресет проставляет дату действия во все вложения; время копируем из
                    // исходного вложения (обрезаем секунды: "12:00:00" -> "12:00").
                    if (presetDate) {
                        attachmentDatesByAttachment[localId] = {
                            ...presetDate,
                            startTime: (attachment.entry_time_from || '').slice(0, 5),
                            endTime: (attachment.entry_time_to || '').slice(0, 5),
                            roofAccess: false,
                            freeParking: false,
                            errors: {},
                        };
                    }

                    if (attachment.attachment_type === 'cars') {
                        vehiclesByAttachment[localId] = data.map((car, idx) => ({
                            id: idx + 1,
                            plateNumber: car.car_number,
                            mark: car.car_brand,
                            markId: null,
                            markName: car.car_brand || null,
                            unloadingPlace: car.unload_place || '',
                            unloadPlaces: car.unload_places || [],
                            formatId: null,
                            isExisting: false,
                        }));
                    } else if (attachment.attachment_type === 'people') {
                        employeesByAttachment[localId] = data.map((emp, idx) => ({
                            id: idx + 1,
                            lastName: emp.last_name,
                            firstName: emp.first_name,
                            middleName: emp.middle_name || '',
                            position: emp.position || '',
                            citizenshipId: emp.citizenship_id || null,
                            citizenshipName: emp.citizenship_name || '',
                            passportSeriesNumber: emp.passport_series_number || '',
                            patentNumber: emp.patent_number || null,
                            otherPermission: emp.other_permission || null,
                            targetTables: emp.target_tables || [],
                            passageTables: '',
                            isExisting: false,
                        }));
                    } else if (attachment.attachment_type === 'items') {
                        itemsByAttachment[localId] = data.map((item, idx) => ({
                            id: idx + 1,
                            itemName: item.name,
                            quantity: item.count,
                        }));
                    }
                }

                const draftState = {
                    message: this.applicationData.message || '',
                    attachments: newAttachments,
                    vehiclesByAttachment,
                    employeesByAttachment,
                    itemsByAttachment,
                    // Пресет проставил дату действия ('other' -> пусто, пользователь задаёт сам).
                    attachmentDatesByAttachment,
                    customFieldsByAttachment: {},
                    consentGiven: false,
                    vehicleIdCounter: 1,
                    employeeIdCounter: 1,
                    itemIdCounter: 1,
                };

                // Пишем во временный ключ, а НЕ в draftApplicationState: на странице
                // оформления может быть уже начатый черновик - CreateApplication сам решит
                // (заменить/объединить/отмена), забирать ли этот дубль (#952).
                localStorage.setItem('pendingDuplicateState', JSON.stringify(draftState));
                this.$emit('duplicate');
            } catch (error) {
                console.error('Ошибка при дублировании заявки:', error);
                useDeletionsStore().notify({ prefix: 'Не удалось продублировать заявку: ', bold: 'ошибка', type: 'error' });
            }
        },

        // Дропдаун "Продублировать": ключ пресета -> дублирование с проставленной датой.
        handleDuplicatePreset(preset) {
            this.duplicateApplication(preset);
        },

        // Дата действия для пресета срока (формат дд.мм.гггг, как ждёт форма заявки).
        // tomorrow - один день (завтра); nextMonth - весь следующий календарный месяц
        // (как пресет nextMonth в DateFilter); other/иное - null (дублируем без даты).
        buildPresetDate(preset) {
            const pad = n => String(n).padStart(2, '0');
            const fmt = d => `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()}`;
            const now = new Date();
            if (preset === 'tomorrow') {
                const t = new Date(now);
                t.setDate(now.getDate() + 1);
                return { isOneDay: true, singleDate: fmt(t), startDate: '', endDate: '' };
            }
            if (preset === 'nextMonth') {
                const start = new Date(now.getFullYear(), now.getMonth() + 1, 1);
                const end = new Date(now.getFullYear(), now.getMonth() + 2, 0);
                return { isOneDay: false, singleDate: '', startDate: fmt(start), endDate: fmt(end) };
            }
            return null;
        },

        async withdrawApplication() {
            const ok = await useUiStore().confirm({
                title: 'Отозвать заявку?',
                message: 'При отзыве все машины, люди и вложения в заявке станут неактивны, и заявка перестанет действовать - охрана не пропустит. Вернуть заявку в работу нельзя; можно только продублировать её для повторного согласования.',
                confirmText: 'Отозвать',
                cancelText: 'Отмена',
                danger: true,
            });
            if (!ok) return;

            try {
                const response = await apiRequest(`/applications/${this.applicationData.id}/withdraw`, { method: 'POST' });
                if (response.ok) {
                    useDeletionsStore().notify({ bold: 'Заявка отозвана', type: 'success' });
                    this.$emit('withdraw');
                    this.close();
                } else {
                    const data = await response.json().catch(() => ({}));
                    useDeletionsStore().notify({ prefix: 'Не удалось отозвать заявку: ', bold: data.message || 'ошибка', type: 'error' });
                }
            } catch {
                useDeletionsStore().notify({ prefix: 'Не удалось отозвать заявку: ', bold: 'ошибка сети', type: 'error' });
            }
        },

        formatDateTime(dateTimeString) {
            if (!dateTimeString) return '';
            const date = new Date(dateTimeString);
            return date.toLocaleString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric',
                hour: '2-digit',
                minute: '2-digit'
            });
        },

        getWeekday(dateTimeString) {
            if (!dateTimeString) return '';
            const date = new Date(dateTimeString);
            const weekdays = ['Воскресенье', 'Понедельник', 'Вторник', 'Среда', 'Четверг', 'Пятница', 'Суббота'];
            return weekdays[date.getDay()];
        },

        getUserDisplayName(user) {
            const names = [user.last_name, user.first_name, user.middle_name].filter(Boolean);
            return names.length > 0 ? names.join(' ') : user.username;
        },

        toggleLeftColumn() {
            this.isLeftColumnCollapsed = !this.isLeftColumnCollapsed;
        },

        closeApplicationDetail() {
            this.close();
        },

        close() {
            this.$emit('close');
        },

        async loadCommonData() {
            try {
                const [placesRes, formatsRes, tablesRes] = await Promise.all([
                    apiRequest("/unload-places", {}),
                    apiRequest("/license-plate-formats", {}),
                    apiRequest("/system-tables", {})
                ]);

                if (placesRes.ok) {
                    this.allUnloadingPlaces = await placesRes.json();
                }
                if (formatsRes.ok) {
                    this.licensePlateFormats = await formatsRes.json();
                }
                if (tablesRes.ok) {
                    this.allTables = await tablesRes.json();
                }
            } catch (error) {
                console.error("Ошибка при загрузке общих данных:", error);
            }
        },

        openVehicleModal(car) {
            this.selectedVehicle = {
                id: car.id,
                plateNumber: car.car_number,
                mark: car.car_brand,
                formatId: car.formatId || null,
                organization: car.organization || null,
                organizationId: car.organization_id || null,
                company: car.company || null,
                companyId: car.company_id || null,
                isExisting: true,
                unloadPlaces: car.unload_places ? car.unload_places.map(p => p.id) : [],
                entry_date_to: car.entry_date_to || null,
                entry_time_from: car.entry_time_from || null,
                entry_time_to: car.entry_time_to || null,
                applicationId: this.applicationData.id,
                territory_status: 0,
                entry_checked: false,
                exit_checked: false,
                blacklist_similar: car.blacklist_similar || null
            };
            this.showVehicleModal = true;
        },

        openEmployeeModal(employee) {
            this.selectedEmployee = {
                id: employee.id,
                last_name: employee.last_name,
                first_name: employee.first_name,
                middle_name: employee.middle_name,
                position: employee.position,
                citizenshipName: employee.citizenship_name,
                passport_series_number: employee.passport_series_number,
                patent_number: employee.patent_number,
                other_permission: employee.other_permission,
                organization: employee.organization || null,
                organizationId: employee.organization_id || null,
                company: employee.company || null,
                companyId: employee.company_id || null,
                entry_date_to: employee.entry_date_to || null,
                pass_time: employee.pass_time || null,
                target_tables: employee.target_tables ? employee.target_tables.map(t => t.id) : [],
                applicationId: this.applicationData.id,
                territory_status: 0,
                blacklist_similar: employee.blacklist_similar || null
            };
            this.showEmployeeModal = true;
        },

        openOverrideModal({ label, flag }) {
            this.overrideLabel = label || '';
            this.overrideFlag = flag || null;
            this.showOverrideModal = true;
        },

        async confirmOverride(comment) {
            if (!this.overrideFlag) return;
            this.overrideSubmitting = true;
            try {
                const response = await apiRequest(`/applications/${this.applicationData.id}/blacklist-overrides`, {
                    method: 'POST',
                    body: JSON.stringify({ flag_id: this.overrideFlag.flag_id, comment })
                });
                if (response.ok) {
                    useDeletionsStore().notify({
                        prefix: 'Пропуск подтверждён: ',
                        bold: this.overrideLabel || 'элемент',
                        type: 'success'
                    });
                    this.showOverrideModal = false;
                    await Promise.all([
                        this.selectedAttachment ? this.loadAttachmentDetails(this.selectedAttachment.id) : Promise.resolve(),
                        this.refreshApplicationGate()
                    ]);
                    this.syncSelectedDetailFlags();
                    this.$emit('application-changed', this.applicationData);
                } else if (!response.ok) {
                    const data = await response.json();
                    useDeletionsStore().notify({
                        prefix: 'Не удалось подтвердить пропуск: ',
                        bold: data.message || 'ошибка',
                        type: 'error'
                    });
                }
            } catch (error) {
                console.error('Ошибка при подтверждении пропуска:', error);
                useDeletionsStore().notify({
                    prefix: 'Не удалось подтвердить пропуск: ',
                    bold: 'ошибка сети',
                    type: 'error'
                });
            } finally {
                this.overrideSubmitting = false;
            }
        },

        // После override обновляем только заявочные поля (в т.ч. гейт
        // has_unoverridden_blacklist_flags), не сбрасывая выбранное вложение.
        async refreshApplicationGate() {
            try {
                const response = await apiRequest(`/applications/${this.applicationData.id}/details`, { method: 'GET' });
                if (response.ok) {
                    const appData = await response.json();
                    this.applicationData = { ...this.applicationData, ...appData };
                    return;
                }
            } catch (error) {
                console.error('Ошибка при обновлении состояния заявки:', error);
            }
            // Override прошёл, но детали не перечитались - кнопка согласования может
            // остаться заблокированной. Сообщаем, чтобы пользователь обновил вручную.
            useDeletionsStore().notify({
                prefix: 'Пропуск подтверждён, ',
                bold: 'обновите страницу для согласования',
                type: 'error'
            });
        },

        // Открытая карточка детали держит свою копию flag - после override/отмены
        // переносим в неё свежий blacklist_similar из перечитанного вложения, чтобы блок
        // "Подозрение на обход ЧС" сразу показал актуальный статус без закрытия карточки.
        syncSelectedDetailFlags() {
            if (this.showVehicleModal && this.selectedVehicle) {
                const fresh = this.attachmentCars.find(c => c.id === this.selectedVehicle.id);
                if (fresh) this.selectedVehicle.blacklist_similar = fresh.blacklist_similar || null;
            }
            if (this.showEmployeeModal && this.selectedEmployee) {
                const fresh = this.attachmentEmployees.find(emp => emp.id === this.selectedEmployee.id);
                if (fresh) this.selectedEmployee.blacklist_similar = fresh.blacklist_similar || null;
            }
        },

        // "Всё равно пропустить" из карточки детали - переиспользуем POST-флоу с причиной.
        onCardOverride(kind) {
            const entity = kind === 'vehicle' ? this.selectedVehicle : this.selectedEmployee;
            if (!entity || !entity.blacklist_similar) return;
            const label = kind === 'vehicle'
                ? (entity.plateNumber || 'Т/С')
                : [entity.last_name, entity.first_name, entity.middle_name].filter(Boolean).join(' ').trim() || 'Сотрудник';
            this.openOverrideModal({ label, flag: entity.blacklist_similar });
        },

        // "Отменить" подтверждение пропуска из карточки - подтверждение БЕЗ причины, затем DELETE.
        async onCardCancelOverride(kind) {
            const entity = kind === 'vehicle' ? this.selectedVehicle : this.selectedEmployee;
            const flag = entity && entity.blacklist_similar;
            if (!flag || !flag.flag_id) return;
            const label = kind === 'vehicle'
                ? (entity.plateNumber || 'Т/С')
                : [entity.last_name, entity.first_name, entity.middle_name].filter(Boolean).join(' ').trim() || 'Сотрудник';

            const ok = await useUiStore().confirm({
                title: 'Снять подтверждение пропуска?',
                message: 'Подтверждение пропуска будет снято, и согласование заявки снова заблокируется по этому элементу.',
                confirmText: 'Снять',
                cancelText: 'Отмена',
                danger: true
            });
            if (!ok) return;

            try {
                const response = await apiRequest(
                    `/applications/${this.applicationData.id}/blacklist-overrides?flag_id=${flag.flag_id}`,
                    { method: 'DELETE' }
                );
                if (response.ok) {
                    useDeletionsStore().notify({
                        prefix: 'Подтверждение пропуска снято: ',
                        bold: label,
                        type: 'success'
                    });
                    await Promise.all([
                        this.selectedAttachment ? this.loadAttachmentDetails(this.selectedAttachment.id) : Promise.resolve(),
                        this.refreshApplicationGate()
                    ]);
                    this.syncSelectedDetailFlags();
                    this.$emit('application-changed', this.applicationData);
                } else if (!response.ok) {
                    const data = await response.json();
                    useDeletionsStore().notify({
                        prefix: 'Не удалось снять подтверждение: ',
                        bold: data.message || 'ошибка',
                        type: 'error'
                    });
                }
            } catch (error) {
                console.error('Ошибка при отмене подтверждения пропуска:', error);
                useDeletionsStore().notify({
                    prefix: 'Не удалось снять подтверждение: ',
                    bold: 'ошибка сети',
                    type: 'error'
                });
            }
        }
    }
}
</script>

<style scoped>
/* Стили остаются без изменений, как в вашем коде */
.application-status-section {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    padding: 15px;
    margin-bottom: 10px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.status-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 15px;
}

.status-header h4 {
    font-size: 18px;
    color: #4F5BDF;
    font-weight: 700;
    margin: 0;
}

.status-mini-badge {
    padding: 4px 12px;
    border-radius: 20px;
    font-size: 12px;
    font-weight: 500;
    display: inline-block;
    border: 1px solid;
}

.status-mini-work {
    background-color: rgba(79, 91, 223, 0.1);
    color: #4F5BDF;
    border-color: rgba(79, 91, 223, 0.3);
}

.status-mini-rejected {
    background-color: rgba(220, 38, 38, 0.1);
    color: #dc2626;
    border-color: rgba(220, 38, 38, 0.3);
}

.status-mini-completed {
    background-color: rgba(5, 150, 105, 0.1);
    color: #059669;
    border-color: rgba(5, 150, 105, 0.3);
}

.status-info {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.status-info-row {
    display: flex;
    justify-content: space-between;
    padding: 4px 0;
}

.status-info-row.comment-row {
    align-items: flex-start;
    flex-direction: column;
    gap: 5px;
}

.status-info-label {
    color: #a2a2a2;
    font-size: 14px;
    font-weight: 400;
    min-width: 120px;
}

.status-info-value {
    color: #000;
    font-size: 15px;
    font-weight: 400;
    text-align: end;
    flex: 1;
}

.status-info-value.comment-text {
    font-weight: 400;
    white-space: pre-wrap;
    word-break: break-word;
    line-height: 1.5;
    font-size: 13px;
    text-align: start;
    color: #333;
    
}

.comment-action-section {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    padding: 15px;
    margin-bottom: 10px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.comment-action-section h4 {
    font-size: 18px;
    color: #4F5BDF;
    font-weight: 700;
    margin-bottom: 10px;
}

.comment-action-textarea {
    width: 100%;
    padding: 12px;
    border: 1px solid #e6e6e6;
    border-radius: 15px;
    font-size: 14px;
    font-family: inherit;
    resize: none;
    transition: all 0.2s ease;
    background-color: #fff;
}

.comment-action-textarea:focus {
    outline: none;
    border-color: #4F5BDF;
    box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.1);
}

/* Остальные стили остаются без изменений */
.application-detail-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 10002;
    backdrop-filter: blur(0.1px);
    -webkit-backdrop-filter: blur(0.1px);
    animation: fadeIn 0.2s ease-out;
}

@keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
}

.application-detail {
    background: white;
    border-radius: 30px;
    width: 1600px;
    max-width: 95%;
    height: 90vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
    overflow: hidden;
}

.detail-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 15px 20px;
    border-bottom: 1px solid #e6e6e6;
    background: #fafafa;
    min-height: 40px;
}

.detail-header-left {
    display: flex;
    flex-direction: column;
    gap: 5px;
    flex: 1;
}

.detail-title-row {
    display: flex;
    align-items: center;
    gap: 20px;
    flex-wrap: wrap;
}

.detail-title {
    font-size: 20px;
    font-weight: 700;
    color: #000;
    margin: 0;
    line-height: 1.2;
}

.detail-datetime {
    font-size: 15px;
    color: #a2a2a2;
    line-height: 1.2;
    font-weight: 500;
    display: flex;
    align-items: center;
    gap: 5px;
}

.weekday {
    font-size: 15px;
    color: #a2a2a2;
}

.forward-btn {
    padding: 6px 24px;
    border: none;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
    min-width: 120px;
    border: 1px solid #e6e6e6;
    background: #4F5BDF;
    color: white;
    margin-left: 10px;
}

.forward-btn:hover:not(:disabled) {
    background: #3a45c0;
}

.detail-header-right {
    display: flex;
    align-items: center;
    gap: 15px;
}

.forward-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.button-loading {
    display: inline-block;
    width: 16px;
    height: 16px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-radius: 50%;
    border-top-color: white;
    animation: spin 0.8s linear infinite;
}

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}

.close-detail-btn {
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

.close-detail-btn:hover {
    background: #f0f0f0;
    color: #333;
}

.detail-content {
    display: flex;
    flex: 1;
    overflow: hidden;
}

.detail-left-column {
    width: 240px;
    border-right: 1px solid #e6e6e6;
    overflow-y: auto;
    background: #fafafa;
    padding: 15px;
    transition: width 0.3s ease;
}

.detail-left-column.collapsed {
    width: 85px;
    padding: 10px;
}

.detail-main-column {
    flex: 1;
    padding: 15px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 10px;
}

/* Блоки колонки держат свою высоту, а колонка скроллится. Без этого flex-column
   сжимает дочерние блоки (у части overflow:hidden) и контент режется вместо скролла. */
.detail-main-column > * {
    flex-shrink: 0;
}

.detail-right-column {
    width: 360px;
    border-left: 1px solid #e6e6e6;
    overflow-y: auto;
    padding: 15px;
    background: #fafafa;
}

.message-section {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    padding: 15px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
    overflow: hidden;
}

.message-section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    margin: -15px -15px 12px;
    padding: 12px 15px;
    border-bottom: 1px solid #e6e6e6;
}

.message-section h4 {
    margin: 0;
    font-size: 14px;
    color: #a2a2a2;
    font-weight: 400;
}

.message-open-btn {
    flex-shrink: 0;
    padding: 4px 18px;
}

.message-content {
    font-size: 15px;
    line-height: 150%;
    color: #000;
}

.message-empty {
    white-space: pre-wrap;
    color: #a2a2a2;
}

.message-preview {
    position: relative;
    max-height: 150px;
    overflow: hidden;
    word-break: break-word;
}

.message-preview.is-clamped::after {
    content: "";
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    height: 46px;
    background: linear-gradient(to bottom, rgba(255, 255, 255, 0), #fff);
    pointer-events: none;
}

.message-preview :deep(img) {
    max-width: 100%;
    height: auto;
}

.message-preview :deep(.constructor-image.img-align-left) { float: left; margin: 0 14px 10px 0; }
.message-preview :deep(.constructor-image.img-align-right) { float: right; margin: 0 0 10px 14px; }
.message-preview :deep(.constructor-image.img-align-center) { display: block; margin: 10px auto; float: none; }

.message-preview :deep(p) { margin: 4px 0; }

.message-preview :deep(ul),
.message-preview :deep(ol) {
    padding-left: 22px;
    margin: 6px 0;
}

.message-preview :deep(strong) { font-weight: 700; }
.message-preview :deep(em) { font-style: italic; }
.message-preview :deep(u) { text-decoration: underline; }

.message-preview :deep(h1),
.message-preview :deep(.heading-h1) { font-size: 20px; font-weight: 700; margin: 6px 0; }

.message-preview :deep(h2),
.message-preview :deep(.heading-h2) { font-size: 17px; font-weight: 600; margin: 6px 0; }

.message-preview :deep(.black-text) { color: #000 !important; }
.message-preview :deep(.red-text) { color: #FF0000 !important; }
.message-preview :deep(.green-text) { color: #079D1D !important; }
.message-preview :deep(.blue-text) { color: #4F5BDF !important; }

.message-preview :deep(.font-size-10) { font-size: 10px !important; }
.message-preview :deep(.font-size-12) { font-size: 12px !important; }
.message-preview :deep(.font-size-14) { font-size: 14px !important; }
.message-preview :deep(.font-size-16) { font-size: 16px !important; }
.message-preview :deep(.font-size-18) { font-size: 18px !important; }
.message-preview :deep(.font-size-20) { font-size: 20px !important; }

.message-preview :deep(.font-weight-300) { font-weight: 300 !important; }
.message-preview :deep(.font-weight-400) { font-weight: 400 !important; }
.message-preview :deep(.font-weight-500) { font-weight: 500 !important; }
.message-preview :deep(.font-weight-600) { font-weight: 600 !important; }
.message-preview :deep(.font-weight-900) { font-weight: 900 !important; }

.message-preview :deep(.text-align-left) { text-align: left !important; }
.message-preview :deep(.text-align-center) { text-align: center !important; }
.message-preview :deep(.text-align-right) { text-align: right !important; }

.basic-info-section {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    padding: 15px;
    margin-bottom: 10px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.basic-info-section h4 {
    font-size: 18px;
    color: #4F5BDF;
    font-weight: 700;
    margin-bottom: 15px;
}

.info-grid {
    display: flex;
    flex-direction: column;
    gap: 15px;
}

.info-row {
    display: flex;
    flex-direction: column;
    gap: 0px;
}

.info-label {
    color: #a2a2a2;
    font-size: 14px;
    font-weight: 400;
    min-width: 140px;
    text-align: left;
}

.info-value {
    color: #000;
    font-size: 15px;
    text-align: left;
    flex: 1;
    font-weight: 400;
}

/* Имя отправителя и тег "Важный" в одну строку: тег - пилюль рядом с именем (как теги
   Крыша/Парковка), а не отдельной строкой под ним. */
.sender-value {
    display: inline-flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
}

.sender-important-tag {
    flex-shrink: 0;
}

/* Дропдаун "Продублировать": перекрашиваем триггер BaseDropdown в синюю primary-кнопку
   (как была прежняя .duplicate-btn), меню остаётся штатным белым. */
.duplicate-dropdown {
    min-width: 160px;
}

/* Меню шире кнопки, чтобы длинные пункты ("На следующий месяц") не обрезались. */
.duplicate-dropdown :deep(.base-dropdown__menu) {
    width: max-content;
    min-width: 100%;
}

.duplicate-dropdown :deep(.base-dropdown__button) {
    min-height: 34px;
    justify-content: center;
    gap: 8px;
    border-color: #4F5BDF;
    background: #4F5BDF;
}

.duplicate-dropdown :deep(.base-dropdown__button:hover:not(:disabled)) {
    border-color: #3a45c0;
    background: #3a45c0;
}

.duplicate-dropdown :deep(.base-dropdown__text),
.duplicate-dropdown :deep(.base-dropdown__text--placeholder) {
    color: #fff;
    font-weight: 600;
}

.duplicate-dropdown :deep(.base-dropdown__arrow) {
    color: #fff;
}

.withdraw-btn {
    padding: 6px 24px;
    border: 1px solid #e6e6e6;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
    min-width: 140px;
    background: #e53935;
    color: white;
}

.withdraw-btn:hover {
    background: #c62828;
}

.revoke-btn, .restore-btn {
    padding: 6px 24px;
    border: none;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
    min-width: 140px;
    border: 1px solid #e6e6e6;
    background: #FFA500;
    color: white;
}

.revoke-btn:hover:not(:disabled) {
    background: #e69500;
}

.restore-btn {
    background: #4CAF50;
}

.restore-btn:hover:not(:disabled) {
    background: #45a049;
}

.revoke-btn:disabled,
.restore-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.history-button-section {
    margin: 10px 0;
    display: flex;
    justify-content: flex-end;
}
</style>