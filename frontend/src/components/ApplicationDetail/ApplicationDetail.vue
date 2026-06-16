<template>
  <!-- Внешний контейнер для модального окна -->
  <div
    class="application-detail-overlay"
    @click.self="closeApplicationDetail"
  >
    <!-- Уведомление -->
    <div
      v-if="notification.show"
      class="notification"
      :class="notification.type"
    >
      {{ notification.message }}
    </div>

    <!-- Модальное окно пересылки -->
    <ForwardModal
      v-if="showForwardModal"
      :all-users="allUsers"
      :responsible-users="responsibleUsers"
      :existing-approvers="approvers"  
      :existing-viewers="viewers"       
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
              v-if="mode === 'center'"
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
              <button
                class="duplicate-btn"
                @click="duplicateApplication"
              >
                Продублировать
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
            <h4>Сообщение к заявке {{ applicationData.application_number }}</h4>
            <div
              class="message-content"
              v-html="applicationData.message ? sanitizeHtml(applicationData.message) : 'Сообщение отсутствует'"
            />
          </div>

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

          <!-- Блок статуса заявки (для принятых/отказанных/завершенных) -->
          <div
            v-if="applicationData.status === 'В работе' || applicationData.status === 'Отказано' || applicationData.status === 'Завершено'"
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
          </div>

          <!-- Поле для комментария (только для пользователей, которые еще не выполнили действие) -->
          <div
            v-if="canLeaveComment && !hasUserVoted && !isApproverActionDone"
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

          <div class="history-button-section">
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
import { sanitizeHtml } from '@/utils/sanitize'
import { useDeletionsStore } from '@/stores/deletions'
import { useUiStore } from '@/stores/ui'
import ApplicationAttachments from './ApplicationAttachments.vue'
import ApplicationConfirmation from './ApplicationConfirmation.vue'
import ApplicationHistory from './ApplicationHistory.vue'
import ForwardModal from './ForwardModal.vue'
import ApplicationActionBar from './ApplicationActionBar.vue'
import ApplicationAttachmentDetail from './ApplicationAttachmentDetail.vue'
import BlacklistOverrideModal from './BlacklistOverrideModal.vue'
import VehicleDetailsModal from '../CreateApplication/VehicleDetailsModal.vue'
import EmployeeDetailsModal from '../CreateApplication/EmployeeDetailsModal.vue'
import Badge from '@/components/ui/Badge.vue'

export default {
    name: 'ApplicationDetail',
    components: {
        ApplicationAttachments,
        ApplicationConfirmation,
        ApplicationHistory,
        ForwardModal,
        ApplicationActionBar,
        ApplicationAttachmentDetail,
        BlacklistOverrideModal,
        VehicleDetailsModal,
        EmployeeDetailsModal,
        Badge
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
    emits: ['close', 'confirmation-updated', 'duplicate', 'application-updated', 'update-application', 'application-changed'],
    data() {
        return {
            applicationData: { ...this.application },
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
            isLeftColumnCollapsed: false,
            notification: {
                show: false,
                message: '',
                type: 'success'
            },
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
        isResponsibleUser() {
            if (!this.currentUserId || !this.responsibleUsers.length) return false;
            return this.responsibleUsers.some(user => user.id === this.currentUserId);
        },

        isApprover() {
            if (!this.currentUserId || !this.approvers.length) return false;
            return this.approvers.some(approver => approver.user_id === this.currentUserId);
        },

        // Отменить подтверждение пропуска может ответственный по заявке ИЛИ принимающий -
        // зеркалит право DELETE /blacklist-overrides на бэке (шире, чем создание override).
        canManageBlacklistOverride() {
            return this.isResponsibleUser || this.isApprover;
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
                    this.storageKey = `app_comment_${this.currentUserId}_${newApplication.id}`;
                    this.loadCommentFromLocalStorage();
                    this.loadApplicationDetails(newApplication);
                    if (!oldApplication || oldApplication.id !== newApplication.id) {
                        markAsRead(newApplication.id).catch(() => {});
                    }
                }
            },
            deep: true
        }
    },
    mounted() {
        this.loadCommonData();
    },
    methods: {
        sanitizeHtml,

        handleActionCompleted({ success, message, type }) {
            this.showNotification(message, type || (success ? 'success' : 'error'));
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
                'Завершено': 'status-mini-completed'
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

                await this.fetchAllUsers();
                await this.fetchApprovers();

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
                const response = await apiRequest("/users/all", {
                });
                if (response.ok) {
                    this.allUsers = await response.json();
                }
            } catch (error) {
                console.error("Error fetching users:", error);
            }
        },

        async fetchApprovers() {
            try {
                const response = await apiRequest("/application-approvers", {
                });
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

        async sendForwardRequest(selectedUsers) {
            if (selectedUsers.length === 0) return;
            
            this.isForwarding = true;
            try {
                const usersToSend = selectedUsers.map(user => ({
                    user_id: user.user_id,
                    required_approval: user.required_approval || false,
                    can_view: user.can_view !== undefined ? user.can_view : !user.required_approval
                }));
                
                const response = await apiRequest(`/applications/${this.applicationData.id}/forward`, {
                    method: "POST",
                    body: JSON.stringify({
                        users: usersToSend
                    })
                });

                if (response.ok) {
                    this.showNotification("Заявка успешно переслана", "success");
                    this.closeForwardModal();
                    
                    await this.loadApplicationDetails(this.applicationData);
                    
                    if (this.$refs.historyComponent) {
                        this.$refs.historyComponent.loadHistory();
                    }
                    
                    this.$emit('application-changed', this.applicationData);
                    
                } else {
                    const errorText = await response.text();
                    this.showNotification(`Ошибка: ${errorText}`, 'error');
                }
            } catch (error) {
                console.error("Ошибка при пересылке заявки:", error);
                this.showNotification("Ошибка сети", 'error');
            } finally {
                this.isForwarding = false;
            }
        },

        async duplicateApplication() {
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
                    // Даты не копируем — пользователь выберет новый период
                    attachmentDatesByAttachment: {},
                    customFieldsByAttachment: {},
                    consentGiven: false,
                    vehicleIdCounter: 1,
                    employeeIdCounter: 1,
                    itemIdCounter: 1,
                };

                localStorage.setItem('draftApplicationState', JSON.stringify(draftState));
                this.$emit('duplicate');
            } catch (error) {
                console.error('Ошибка при дублировании заявки:', error);
                this.showNotification('Ошибка при дублировании заявки', 'error');
            }
        },

        showNotification(message, type = 'success') {
            this.notification = {
                show: true,
                message,
                type
            };
            
            setTimeout(() => {
                this.hideNotification();
            }, 6000);
        },

        hideNotification() {
            this.notification.show = false;
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
                } else if (response.status !== 403) {
                    // 403 уже показывает тост через client.js - второй не дублируем
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
                } else if (response.status !== 403) {
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

.notification {
    position: fixed;
    top: 40px;
    left: 50%;
    transform: translateX(-50%);
    padding: 8px 8px;
    border-radius: 50px;
    z-index: 29000;
    min-width: 180px;
    width: 180px;
    height: 30px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 14px;
    font-weight: 500;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
    animation: slideInDown 0.2s ease-out, slideOutUp 0.2s ease-out 5.8s forwards;
}

.notification.success {
    background: #4CAF50;
    color: white;
    border: 1px solid #45a049;
}

.notification.error {
    background: #f44336;
    color: white;
    border: 1px solid #d32f2f;
}

@keyframes slideInDown {
    from {
        transform: translate(-50%, -100%);
        opacity: 0;
    }
    to {
        transform: translate(-50%, 0);
        opacity: 1;
    }
}

@keyframes slideOutUp {
    from {
        transform: translate(-50%, 0);
        opacity: 1;
    }
    to {
        transform: translate(-50%, -100%);
        opacity: 0;
    }
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
}

.message-section h4 {
    font-size: 14px;
    color: #a2a2a2;
    padding-bottom: 5px;
    font-weight: 400;
}

.message-content {
    font-size: 15px;
    line-height: 150%;
    color: #000;
}

.message-content :deep(ul),
.message-content :deep(ol) {
    padding-left: 24px;
    margin: 6px 0;
}

.message-content :deep(li) {
    line-height: 1.5;
    margin: 2px 0;
}

.message-content :deep(em) {
    font-style: italic;
}

.message-content :deep(u) {
    text-decoration: underline;
}

.message-content :deep(strong) {
    font-weight: 600;
}

.message-content :deep(h1),
.message-content :deep(.heading-h1) {
    font-size: 22px;
    font-weight: 700;
    margin: 12px 0 6px;
    line-height: 1.2;
    color: #000;
}

.message-content :deep(h2),
.message-content :deep(.heading-h2) {
    font-size: 18px;
    font-weight: 600;
    margin: 10px 0 5px;
    line-height: 1.3;
    color: #000;
}

.message-content :deep(.black-text) { color: #000; }
.message-content :deep(.red-text) { color: #FF0000; }
.message-content :deep(.green-text) { color: #079D1D; }
.message-content :deep(.blue-text) { color: #4F5BDF; }

.message-content :deep(.font-size-10) { font-size: 10px; }
.message-content :deep(.font-size-12) { font-size: 12px; }
.message-content :deep(.font-size-14) { font-size: 14px; }
.message-content :deep(.font-size-16) { font-size: 16px; }
.message-content :deep(.font-size-18) { font-size: 18px; }
.message-content :deep(.font-size-20) { font-size: 20px; }

.message-content :deep(.font-weight-300) { font-weight: 300; }
.message-content :deep(.font-weight-400) { font-weight: 400; }
.message-content :deep(.font-weight-500) { font-weight: 500; }
.message-content :deep(.font-weight-600) { font-weight: 600; }
.message-content :deep(.font-weight-900) { font-weight: 900; }

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

.duplicate-btn {
    padding: 6px 24px;
    border: none;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
    min-width: 140px;
    border: 1px solid #e6e6e6;
    background: #4F5BDF;
    color: white;
}

.duplicate-btn:hover {
    background: #3a45c0;
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