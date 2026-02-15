<template>
    <!-- Внешний контейнер для модального окна -->
    <div class="application-detail-overlay" @click.self="closeApplicationDetail">
        <!-- Уведомление -->
        <div v-if="notification.show" class="notification" :class="notification.type">
            {{ notification.message }}
        </div>

        <!-- Модальное окно для пересылки -->
        <ForwardModal
            v-if="showForwardModal"
            :all-users="allUsers"
            :responsible-users="responsibleUsers"
            :is-sending="isForwarding"
            @close="closeForwardModal"
            @send="sendForwardRequest"
        />

        <div class="application-detail">
            <!-- Заголовок и кнопки -->
            <div class="detail-header">
                <div class="detail-header-left">
                    <div class="detail-title-row">
                        <h3 class="detail-title">Заявка {{ application.application_number }}</h3>
                        <div class="detail-datetime">
                            {{ formatDateTime(application.sending_datetime) }}
                            <span class="weekday">{{ getWeekday(application.sending_datetime) }}</span>
                        </div>
                        <!-- Кнопка пересылки (рядом с датой) -->
                        <button 
                            v-if="mode === 'center'"
                            class="forward-btn" 
                            @click="forwardApplication"
                            :disabled="updatingConfirmation || processingApplication"
                        >
                            <span v-if="updatingConfirmation || processingApplication" class="button-loading"></span>
                            <span v-else>Переслать</span>
                        </button>
                    </div>
                </div>
                <div class="detail-header-right">
                    <!-- Режим центра заявок -->
                    <div v-if="mode === 'center'" class="action-buttons">
                        <!-- Для принимающих заявки -->
                        <template v-if="isApprover">
                            <!-- Если заявка в работе - показываем статус -->
                            <div v-if="application.status === 'В работе'" class="status-badge status-in-work-badge">
                                В работе
                            </div>
                            <!-- Если заявка не в работе и согласована - показываем кнопки -->
                            <template v-else-if="application.confirmation === 'Согласовано'">
                                <button 
                                    class="accept-btn" 
                                    @click="handleApplicationAction('accept')"
                                    :disabled="processingApplication"
                                >
                                    <span v-if="processingApplication" class="button-loading"></span>
                                    <span v-else>Принять</span>
                                </button>
                                <button 
                                    class="reject-btn" 
                                    @click="handleApplicationAction('reject')"
                                    :disabled="processingApplication"
                                >
                                    <span v-if="processingApplication" class="button-loading"></span>
                                    <span v-else>Отказать</span>
                                </button>
                            </template>
                            <!-- Если заявка не согласована - показываем информационное сообщение -->
                            <div v-else class="info-badge">
                                {{ getApproverStatusMessage }}
                            </div>
                        </template>
                        
                        <!-- Для ответственных за согласование -->
                        <template v-else-if="isResponsibleUser">
                            <!-- Если пользователь уже проголосовал -->
                            <div v-if="hasUserVoted" class="vote-status-badge" :class="userVoteStatus.class">
                                {{ userVoteStatus.text }}
                            </div>
                            <!-- Если заявка на согласовании и пользователь еще не голосовал -->
                            <template v-else-if="application.confirmation === 'Согласование'">
                                <button 
                                    class="confirm-btn" 
                                    @click="updateConfirmation('Согласовано')"
                                    :disabled="updatingConfirmation || processingApplication"
                                >
                                    <span v-if="updatingConfirmation" class="button-loading"></span>
                                    <span v-else>Согласовать</span>
                                </button>
                                <button 
                                    class="reject-btn" 
                                    @click="updateConfirmation('Не согласовано')"
                                    :disabled="updatingConfirmation || processingApplication"
                                >
                                    <span v-if="updatingConfirmation" class="button-loading"></span>
                                    <span v-else>Отказать</span>
                                </button>
                            </template>
                            <!-- Если заявка уже согласована/отклонена и пользователь не голосовал (например, добавлен позже) -->
                            <div v-else class="info-badge">
                                Заявка {{ application.confirmation.toLowerCase() }}
                            </div>
                        </template>
                        
                        <!-- Для создателя заявки - кнопки управления -->
                        <template v-if="canManageApplication">
                            <!-- Если заявка в работе - показать кнопку отзыва -->
                            <button 
                                v-if="application.status === 'В работе'"
                                class="revoke-btn" 
                                @click="revokeApplication"
                                :disabled="processingApplication"
                            >
                                <span v-if="processingApplication" class="button-loading"></span>
                                <span v-else>Отозвать из работы</span>
                            </button>
                            <!-- Если заявка согласована и не в работе - показать кнопку возврата в работу (для повторного принятия) -->
                            <button 
                                v-else-if="application.confirmation === 'Согласовано' && application.status !== 'В работе' && application.status !== 'Отказано'"
                                class="restore-btn" 
                                @click="restoreApplication"
                                :disabled="processingApplication"
                            >
                                <span v-if="processingApplication" class="button-loading"></span>
                                <span v-else>Вернуть в работу</span>
                            </button>
                        </template>
                    </div>
                    
                    <!-- Режим просмотра заявок пользователя -->
                    <div v-if="mode === 'user'" class="view-buttons">
                        <button 
                            class="duplicate-btn" 
                            @click="duplicateApplication"
                        >
                            Продублировать
                        </button>
                    </div>
                    
                    <button class="close-detail-btn" @click="close">×</button>
                </div>
            </div>

            <div class="detail-content">
                <!-- Левая колонка - вложения -->
                <div class="detail-left-column" :class="{ collapsed: isLeftColumnCollapsed }">
                    <ApplicationAttachments 
                        :application-id="application.id"
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
                        <h4>Сообщение к заявке {{ application.application_number }}</h4>
                        <div class="message-content">
                            {{ application.message || 'Сообщение отсутствует' }}
                        </div>
                    </div>

                    <!-- Детали выбранного вложения -->
                    <div v-if="selectedAttachment" class="attachment-details">
                        <div class="attachment-header-section">
                            <h4>{{ selectedAttachment.attachment_display_name }}</h4>
                            
                            <!-- Даты действия -->
                            <div v-if="selectedAttachment.entry_date_from || selectedAttachment.entry_date_to" class="date-range">
                                <span class="date-label">Срок действия:</span>
                                <span class="date-value">
                                    {{ formatDateRange(selectedAttachment.entry_date_from, selectedAttachment.entry_date_to) }}
                                </span>
                            </div>

                            <!-- Время действия -->
                            <div v-if="selectedAttachment.entry_time_from || selectedAttachment.entry_time_to" class="time-range">
                                <span class="time-label">Время:</span>
                                <span class="time-value">
                                    {{ formatTimeRange(selectedAttachment.entry_time_from, selectedAttachment.entry_time_to) }}
                                </span>
                            </div>
                        </div>

                        <div class="attachment-data-section">
                            <!-- Данные вложения в зависимости от типа -->
                            <div class="attachment-data">
                                <!-- Автомобили -->
                                <div v-if="selectedAttachment.attachment_type === 'cars'" class="cars-section">
                                    <h5>Список автомобилей ({{ attachmentCars.length }})</h5>
                                    <div v-if="loadingAttachmentDetails" class="loading-container">
                                        <div class="loading-spinner"></div>
                                        <span class="loading-text">Загрузка автомобилей...</span>
                                    </div>
                                    <div v-else-if="attachmentCars.length > 0" class="cars-list">
                                        <div v-for="(car, index) in attachmentCars" :key="car.id" class="car-item">
                                            <div class="car-item-content">
                                                <!-- Номер порядковый -->
                                                <div class="item-number">
                                                    {{ index + 1 }}.
                                                </div>
                                                <div class="car-main-info">
                                                    <span class="car-number">{{ car.car_number }}</span>
                                                    <span class="car-brand">{{ car.car_brand }}</span>
                                                </div>
                                                <!-- Места разгрузки -->
                                                <div v-if="car.unload_places && car.unload_places.length > 0" 
                                                    class="unload-places-container"
                                                    :title="getFullPlacesList(car.unload_places)"
                                                >
                                                    <span class="places-list">
                                                        {{ getTruncatedPlacesList(car.unload_places) }}
                                                    </span>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                    <div v-else class="no-data">
                                        Нет данных об автомобилях
                                    </div>
                                </div>

                                <!-- Сотрудники -->
                                <div v-if="selectedAttachment.attachment_type === 'people'" class="employees-section">
                                    <h5>Сотрудники ({{ attachmentEmployees.length }})</h5>
                                    <div v-if="loadingAttachmentDetails" class="loading-container">
                                        <div class="loading-spinner"></div>
                                        <span class="loading-text">Загрузка сотрудников...</span>
                                    </div>
                                    <div v-else-if="attachmentEmployees.length > 0" class="employees-list">
                                        <div v-for="(employee, index) in attachmentEmployees" :key="employee.id" class="employee-item">
                                            <div class="employee-item-content">
                                                <!-- Номер порядковый -->
                                                <div class="item-number">
                                                    {{ index + 1 }}.
                                                </div>
                                                <div class="employee-main-info">
                                                    <span class="employee-name">{{ employee.last_name }} {{ employee.first_name }} {{ employee.middle_name || '' }}</span>
                                                    <span class="employee-position">{{ employee.position }}</span>
                                                </div>
                                                <!-- Целевые таблицы -->
                                                <div v-if="employee.target_tables && employee.target_tables.length > 0" 
                                                    class="target-tables-container"
                                                    :title="getFullTablesList(employee.target_tables)"
                                                >
                                                    <span class="tables-list">
                                                        {{ getTruncatedTablesList(employee.target_tables) }}
                                                    </span>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                    <div v-else class="no-data">
                                        Нет данных о сотрудниках
                                    </div>
                                </div>

                                <!-- ТМЦ -->
                                <div v-if="selectedAttachment.attachment_type === 'items'" class="items-section">
                                    <h5>Товарно-материальные ценности ({{ attachmentItems.length }})</h5>
                                    <div v-if="loadingAttachmentDetails" class="loading-container">
                                        <div class="loading-spinner"></div>
                                        <span class="loading-text">Загрузка ТМЦ...</span>
                                    </div>
                                    <div v-else-if="attachmentItems.length > 0" class="items-list">
                                        <div v-for="(item, index) in attachmentItems" :key="item.id" class="item-item">
                                            <div class="item-item-content">
                                                <!-- Номер порядковый -->
                                                <div class="item-number">
                                                    {{ index + 1 }}.
                                                </div>
                                                <span class="item-name">{{ item.name }}</span>
                                                <span class="item-count">Количество: {{ item.count }}</span>
                                            </div>
                                        </div>
                                    </div>
                                    <div v-else class="no-data">
                                        Нет данных о ТМЦ
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Правая колонка - информация о заявке и согласовании -->
                <div class="detail-right-column">
                    <!-- Основная информация -->
                    <div class="basic-info-section">
                        <h4>Основная информация</h4>
                        <div class="info-grid">
                            <div class="info-row">
                                <span class="info-label">Организация / Отдел:</span>
                                <span class="info-value">{{ application.organization_name }}</span>
                            </div>
                            <div v-if="application.company_name" class="info-row">
                                <span class="info-label">Компания:</span>
                                <span class="info-value">{{ application.company_name }}</span>
                            </div>
                            <div class="info-row">
                                <span class="info-label">Отправитель:</span>
                                <span class="info-value">{{ application.sender_full_name || application.sender_name }}</span>
                            </div>
                        </div>
                    </div>

                    <!-- Компонент согласования -->
                    <ApplicationConfirmation 
                        :application="application"
                        :responsible-users="responsibleUsers"
                        :current-user-id="currentUserId"
                        :updating-confirmation="updatingConfirmation"
                    />

                    <!-- Комментарий ответственного -->
                    <div v-if="application.responsible_comment" class="comment-section">
                        <h4>Комментарий ответственного</h4>
                        <div class="comment-content">
                            {{ application.responsible_comment }}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script>
import ApplicationAttachments from './ApplicationAttachments.vue'
import ApplicationConfirmation from './ApplicationConfirmation.vue'
import ForwardModal from './ForwardModal.vue'

export default {
    name: 'ApplicationDetail',
    components: {
        ApplicationAttachments,
        ApplicationConfirmation,
        ForwardModal
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
            default: 'center' // 'center' или 'user'
        }
    },
    data() {
        return {
            attachments: [],
            selectedAttachment: null,
            attachmentCars: [],
            attachmentEmployees: [],
            attachmentItems: [],
            responsibleUsers: [],
            updatingConfirmation: false,
            processingApplication: false,
            loadingAttachmentDetails: false,
            loadingApplicationDetails: false,
            isLeftColumnCollapsed: false,
            notification: {
                show: false,
                message: '',
                type: 'success'
            },
            showForwardModal: false,
            isForwarding: false,
            allUsers: [],
            approvers: []
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

        // Может ли пользователь управлять заявкой (отозвать/вернуть)
        canManageApplication() {
            // Только в режиме центра
            if (this.mode !== 'center') return false;
            
            // Создатель заявки или админ
            return this.application.sender_user_id === this.currentUserId;
        },

        hasUserVoted() {
            if (!this.currentUserId || !this.responsibleUsers.length) return false;
            const currentUser = this.responsibleUsers.find(user => user.id === this.currentUserId);
            return currentUser && currentUser.approval_status !== 'pending';
        },

        userVoteStatus() {
            if (!this.currentUserId || !this.responsibleUsers.length) return null;
            const currentUser = this.responsibleUsers.find(user => user.id === this.currentUserId);
            
            if (!currentUser) return null;
            
            if (currentUser.approval_status === 'approved') {
                return {
                    text: 'Вы согласовали заявку',
                    class: 'vote-approved'
                };
            } else if (currentUser.approval_status === 'rejected') {
                return {
                    text: 'Вы отказали в заявке',
                    class: 'vote-rejected'
                };
            }
            
            return null;
        },

        getApproverStatusMessage() {
            if (this.application.status === 'В работе') {
                return 'Заявка уже в работе';
            }
            
            if (this.application.status === 'Отказано') {
                return 'Заявка отклонена';
            }
            
            if (this.application.confirmation !== 'Согласовано') {
                return 'Ожидает согласования';
            }
            
            return 'Готова к принятию';
        }
    },
    watch: {
        application: {
            immediate: true,
            handler(newApplication) {
                if (newApplication && newApplication.id) {
                    this.loadApplicationDetails(newApplication);
                }
            }
        }
    },
    methods: {
        async loadApplicationDetails(application) {
            this.loadingApplicationDetails = true;
            try {
                const token = localStorage.getItem("token");
                
                const appResponse = await fetch(`http://localhost:8080/applications/${application.id}/details`, {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                        "Content-Type": "application/json"
                    },
                });

                if (appResponse.ok) {
                    const appData = await appResponse.json();
                    
                    Object.assign(this.application, appData);
                    
                    if (appData.responsible_users) {
                        this.responsibleUsers = appData.responsible_users.map(user => ({
                            ...user,
                            approval_status: user.approval_status || 'pending'
                        }));
                    }
                }

                const attachmentsResponse = await fetch(`http://localhost:8080/applications/${application.id}/attachments`, {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                        "Content-Type": "application/json"
                    },
                });

                if (attachmentsResponse.ok) {
                    this.attachments = await attachmentsResponse.json();
                    if (this.attachments.length > 0) {
                        this.selectedAttachment = this.attachments[0];
                        await this.loadAttachmentDetails(this.selectedAttachment.id);
                    }
                }

                await this.fetchAllUsers();
                await this.fetchApprovers();

            } catch (error) {
                console.error("Ошибка при загрузке деталей заявки:", error);
            } finally {
                this.loadingApplicationDetails = false;
            }
        },

        async fetchAllUsers() {
            try {
                const token = localStorage.getItem("token");
                const response = await fetch("http://localhost:8080/users/all", {
                    headers: {
                        "Authorization": `Bearer ${token}`,
                    },
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
                const token = localStorage.getItem("token");
                const response = await fetch("http://localhost:8080/application-approvers", {
                    headers: {
                        "Authorization": `Bearer ${token}`,
                    },
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
                const token = localStorage.getItem("token");
                
                this.attachmentCars = [];
                this.attachmentEmployees = [];
                this.attachmentItems = [];

                const attachment = this.attachments.find(a => a.id === attachmentId);
                if (!attachment) return;

                let carsResponse, employeesResponse, itemsResponse;
                
                switch (attachment.attachment_type) {
                    case 'cars':
                        carsResponse = await fetch(`http://localhost:8080/attachments/${attachmentId}/cars`, {
                            method: "GET",
                            headers: {
                                "Authorization": `Bearer ${token}`,
                            },
                        });
                        if (carsResponse.ok) {
                            this.attachmentCars = await carsResponse.json();
                        }
                        break;
                    
                    case 'people':
                        employeesResponse = await fetch(`http://localhost:8080/attachments/${attachmentId}/employees`, {
                            method: "GET",
                            headers: {
                                "Authorization": `Bearer ${token}`,
                            },
                        });
                        if (employeesResponse.ok) {
                            this.attachmentEmployees = await employeesResponse.json();
                        }
                        break;
                    
                    case 'items':
                        itemsResponse = await fetch(`http://localhost:8080/attachments/${attachmentId}/items`, {
                            method: "GET",
                            headers: {
                                "Authorization": `Bearer ${token}`,
                            },
                        });
                        if (itemsResponse.ok) {
                            this.attachmentItems = await itemsResponse.json();
                        }
                        break;
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

        async handleApplicationAction(action) {
            this.processingApplication = true;
            try {
                if (action === 'accept') {
                    await this.acceptApplication();
                } else {
                    await this.rejectApplication();
                }
            } catch (error) {
                console.error(`Ошибка при ${action === 'accept' ? 'принятии' : 'отказе'} заявки:`, error);
            } finally {
                this.processingApplication = false;
            }
        },

        async acceptApplication() {
            try {
                const token = localStorage.getItem("token");
                
                const response = await fetch(`http://localhost:8080/applications/${this.application.id}/take-to-work`, {
                    method: "POST",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                        "Content-Type": "application/json"
                    },
                    body: JSON.stringify({
                        user_id: this.currentUserId,
                        action: 'accept'
                    })
                });

                if (response.ok) {
                    this.showNotification("Заявка принята в работу", "success");
                    
                    this.application.status = 'В работе';
                    
                    await this.loadApplicationDetails(this.application);
                    
                    this.$emit('application-updated', {
                        id: this.application.id,
                        status: 'В работе',
                        confirmation: this.application.confirmation
                    });
                } else {
                    const errorText = await response.text();
                    this.showNotification(`Ошибка: ${errorText}`, 'error');
                }
                
            } catch (error) {
                console.error("Ошибка при принятии заявки:", error);
                this.showNotification("Ошибка сети при принятии заявки", "error");
            }
        },

        async rejectApplication() {
            try {
                const token = localStorage.getItem("token");
                
                const response = await fetch(`http://localhost:8080/applications/${this.application.id}/take-to-work`, {
                    method: "POST",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                        "Content-Type": "application/json"
                    },
                    body: JSON.stringify({
                        user_id: this.currentUserId,
                        action: 'reject'
                    })
                });

                if (response.ok) {
                    this.showNotification("Заявка отклонена", "error");
                    
                    this.application.status = 'Отказано';
                    
                    await this.loadApplicationDetails(this.application);
                    
                    this.$emit('application-updated', {
                        id: this.application.id,
                        status: 'Отказано',
                        confirmation: this.application.confirmation
                    });
                } else {
                    const errorText = await response.text();
                    this.showNotification(`Ошибка: ${errorText}`, 'error');
                }
                
            } catch (error) {
                console.error("Ошибка при отказе заявки:", error);
                this.showNotification("Ошибка сети при отказе заявки", "error");
            }
        },

        async revokeApplication() {
            this.processingApplication = true;
            try {
                const token = localStorage.getItem("token");
                
                const response = await fetch(`http://localhost:8080/applications/${this.application.id}/revoke-from-work`, {
                    method: "POST",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                        "Content-Type": "application/json"
                    },
                    body: JSON.stringify({
                        user_id: this.currentUserId
                    })
                });

                if (response.ok) {
                    this.showNotification("Заявка отозвана из работы", "success");
                    
                    this.application.status = 'Согласовано';
                    
                    await this.loadApplicationDetails(this.application);
                    
                    this.$emit('application-updated', {
                        id: this.application.id,
                        status: 'Согласовано',
                        confirmation: this.application.confirmation
                    });
                } else {
                    const errorText = await response.text();
                    this.showNotification(`Ошибка: ${errorText}`, 'error');
                }
                
            } catch (error) {
                console.error("Ошибка при отзыве заявки:", error);
                this.showNotification("Ошибка сети при отзыве заявки", "error");
            } finally {
                this.processingApplication = false;
            }
        },

        async restoreApplication() {
            this.processingApplication = true;
            try {
                const token = localStorage.getItem("token");
                
                const response = await fetch(`http://localhost:8080/applications/${this.application.id}/restore-to-work`, {
                    method: "POST",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                        "Content-Type": "application/json"
                    },
                    body: JSON.stringify({
                        user_id: this.currentUserId
                    })
                });

                if (response.ok) {
                    this.showNotification("Заявка возвращена в работу", "success");
                    
                    this.application.status = 'Согласовано';
                    
                    await this.loadApplicationDetails(this.application);
                    
                    this.$emit('application-updated', {
                        id: this.application.id,
                        status: 'Согласовано',
                        confirmation: this.application.confirmation
                    });
                } else {
                    const errorText = await response.text();
                    this.showNotification(`Ошибка: ${errorText}`, 'error');
                }
                
            } catch (error) {
                console.error("Ошибка при возврате заявки:", error);
                this.showNotification("Ошибка сети при возврате заявки", "error");
            } finally {
                this.processingApplication = false;
            }
        },

        async updateConfirmation(confirmation) {
            if (!this.application || !this.isResponsibleUser || this.hasUserVoted) return;

            this.updatingConfirmation = true;
            try {
                const token = localStorage.getItem("token");
                
                const userApprovalResponse = await fetch(`http://localhost:8080/applications/${this.application.id}/approve`, {
                    method: "POST",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                        "Content-Type": "application/json"
                    },
                    body: JSON.stringify({
                        user_id: this.currentUserId,
                        status: confirmation === 'Согласовано' ? 'approved' : 'rejected',
                        comment: confirmation === 'Согласовано' ? 
                            `Заявка согласована пользователем ${this.currentUserName}` : 
                            `Заявка отклонена пользователем ${this.currentUserName}`
                    })
                });

                if (!userApprovalResponse.ok) {
                    const errorText = await userApprovalResponse.text();
                    throw new Error(errorText);
                }

                const checkResponse = await fetch(`http://localhost:8080/applications/${this.application.id}/check-approval-status`, {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                    }
                });

                if (checkResponse.ok) {
                    const approvalStatus = await checkResponse.json();
                    
                    this.$emit('confirmation-updated', {
                        confirmation: approvalStatus.confirmation,
                        status: this.application.status
                    });
                }
                
                await this.loadApplicationDetails(this.application);
                
                this.showNotification(
                    confirmation === 'Согласовано' 
                        ? 'Заявка согласована'
                        : 'Заявка отклонена',
                    confirmation === 'Согласовано' ? 'success' : 'error'
                );
                
            } catch (error) {
                console.error("Ошибка при обновлении подтверждения:", error);
                this.showNotification(`Ошибка: ${error.message}`, 'error');
                throw error;
            } finally {
                this.updatingConfirmation = false;
            }
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
                const token = localStorage.getItem("token");
                
                const response = await fetch(`http://localhost:8080/applications/${this.application.id}/forward`, {
                    method: "POST",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                        "Content-Type": "application/json"
                    },
                    body: JSON.stringify({
                        users: selectedUsers.map(user => ({
                            user_id: user.id,
                            required_approval: user.required_approval || false
                        }))
                    })
                });

                if (response.ok) {
                    this.showNotification("Заявка успешно переслана", "success");
                    this.closeForwardModal();
                    
                    await this.loadApplicationDetails(this.application);
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

        duplicateApplication() {
            console.log('Дублирование заявки:', this.application.application_number);
            this.$emit('duplicate', this.application);
            this.showNotification('Функция дублирования пока не реализована', 'error');
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

        formatDate(date) {
            if (!date) return '';
            if (typeof date === 'string') {
                date = new Date(date);
            }
            return date.toLocaleDateString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric'
            });
        },

        formatDateRange(dateFrom, dateTo) {
            if (!dateFrom && !dateTo) return '';
            
            const from = dateFrom ? this.formatDate(dateFrom) : '';
            const to = dateTo ? this.formatDate(dateTo) : '';
            
            if (from && to) {
                const fromDate = new Date(dateFrom);
                const toDate = new Date(dateTo);
                if (fromDate.toDateString() === toDate.toDateString()) {
                    return from;
                }
                return `${from} - ${to}`;
            } else if (from) {
                return `с ${from}`;
            } else if (to) {
                return `по ${to}`;
            }
            return '';
        },

        formatTime(time) {
            if (!time) return '';
            const timeParts = time.split(':');
            if (timeParts.length >= 2) {
                return `${timeParts[0]}:${timeParts[1]}`;
            }
            return time;
        },

        formatTimeRange(timeFrom, timeTo) {
            if (!timeFrom && !timeTo) return '';
            
            const from = timeFrom ? this.formatTime(timeFrom) : '';
            const to = timeTo ? this.formatTime(timeTo) : '';
            
            if (from && to) {
                return `${from} - ${to}`;
            } else if (from) {
                return `с ${from}`;
            } else if (to) {
                return `до ${to}`;
            }
            return '';
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

        getApplicationStatusClass(status) {
            const classes = {
                'В работе': 'status-in-work',
                'Отказано': 'status-rejected',
                'Согласовано': 'status-approved',
                'Согласование': 'status-pending'
            };
            return classes[status] || '';
        },

        getUserDisplayName(user) {
            const names = [user.last_name, user.first_name, user.middle_name].filter(Boolean);
            return names.length > 0 ? names.join(' ') : user.username;
        },

        getFullPlacesList(places) {
            if (!places || !places.length) return '';
            return places.map(p => p.name).join(', ');
        },

        getTruncatedPlacesList(places) {
            if (!places || !places.length) return '';
            
            const maxPlaces = 2;
            const placeNames = places.map(p => p.name);
            
            if (placeNames.length <= maxPlaces) {
                return placeNames.join(', ');
            }
            
            const shownPlaces = placeNames.slice(0, maxPlaces);
            return `${shownPlaces.join(', ')} и др.`;
        },

        getFullTablesList(tables) {
            if (!tables || !tables.length) return '';
            return tables.map(t => t.display_name).join(', ');
        },

        getTruncatedTablesList(tables) {
            if (!tables || !tables.length) return '';
            
            const maxTables = 2;
            const tableNames = tables.map(t => t.display_name);
            
            if (tableNames.length <= maxTables) {
                return tableNames.join(', ');
            }
            
            const shownTables = tableNames.slice(0, maxTables);
            return `${shownTables.join(', ')} и др.`;
        },

        toggleLeftColumn() {
            this.isLeftColumnCollapsed = !this.isLeftColumnCollapsed;
        },

        closeApplicationDetail() {
            this.close();
        },

        close() {
            this.$emit('close');
        }
    },
    emits: ['close', 'confirmation-updated', 'duplicate', 'application-updated']
}
</script>

<style scoped>
/* Все стили остаются без изменений, добавляем новые классы для кнопок и статусов */

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

.status-in-work-badge {
    background: rgba(79, 91, 223, 0.1);
    color: #4F5BDF;
    border: 1px solid rgba(79, 91, 223, 0.3);
    padding: 6px 24px;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    min-width: 120px;
    text-align: center;
}

.status-in-work {
    color: #4F5BDF;
    font-weight: 600;
    background: rgba(79, 91, 223, 0.1);
    padding: 4px 12px;
    border-radius: 20px;
    display: inline-block;
}

.status-rejected {
    color: #dc2626;
    font-weight: 600;
    background: rgba(220, 38, 38, 0.1);
    padding: 4px 12px;
    border-radius: 20px;
    display: inline-block;
}

.status-approved {
    color: #059669;
    font-weight: 600;
    background: rgba(5, 150, 105, 0.1);
    padding: 4px 12px;
    border-radius: 20px;
    display: inline-block;
}

.status-pending {
    color: #d97706;
    font-weight: 600;
    background: rgba(217, 119, 6, 0.1);
    padding: 4px 12px;
    border-radius: 20px;
    display: inline-block;
}

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
    z-index: 10000;
    animation: fadeIn 0.3s ease-out;
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
    animation: slideInDown 0.3s ease-out, slideOutUp 0.3s ease-out 2.7s forwards;
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

.action-buttons {
    display: flex;
    gap: 5px;
    align-items: center;
}

.view-buttons {
    display: flex;
    gap: 10px;
}

.confirm-btn, .reject-btn, .accept-btn {
    padding: 6px 24px;
    border: none;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
    min-width: 120px;
    border: 1px solid #e6e6e6;
    position: relative;
    overflow: hidden;
}

.confirm-btn, .accept-btn {
    background: rgba(9, 136, 0, 1);
    color: white;
}

.confirm-btn:hover:not(:disabled), .accept-btn:hover:not(:disabled) {
    background: #45b371;
}

.reject-btn {
    background: #FF6668;
    color: white;
}

.reject-btn:hover:not(:disabled) {
    background: #ff4d4f;
}

.vote-status-badge {
    padding: 6px 16px;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    min-width: 140px;
    text-align: center;
    border: 1px solid;
}

.vote-status-badge.vote-approved {
    background: rgba(9, 136, 0, 0.1);
    color: rgba(9, 136, 0, 1);
    border-color: rgba(9, 136, 0, 0.3);
}

.vote-status-badge.vote-rejected {
    background: rgba(255, 102, 104, 0.1);
    color: #FF6668;
    border-color: rgba(255, 102, 104, 0.3);
}

.info-badge {
    padding: 6px 16px;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 500;
    min-width: 200px;
    text-align: center;
    background: #f0f0f0;
    color: #666;
    border: 1px solid #e6e6e6;
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

.confirm-btn:disabled,
.reject-btn:disabled,
.forward-btn:disabled,
.accept-btn:disabled {
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
    white-space: pre-wrap;
}

.attachment-details {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
    overflow: hidden;
}

.attachment-header-section {
    padding: 15px;
    border-bottom: 1px solid #e6e6e6;
}

.attachment-details h4 {
    font-size: 18px;
    color: #4F5BDF;
    padding-bottom: 15px;
    font-weight: 700;
    margin: 0;
}

.date-range, .time-range {
    display: flex;
    flex-direction: column;
    gap: 0px;
    margin-bottom: 8px;
    font-size: 14px;
}

.date-range:last-child, .time-range:last-child {
    margin-bottom: 0;
}

.date-label, .time-label {
    color: #a2a2a2;
    font-weight: 400;
    min-width: 110px;
    font-size: 14px;
}

.date-value, .time-value {
    color: #000;
    font-weight: 400;
    font-size: 15px;
}

.attachment-data-section {
    padding: 15px;
    min-height: 300px;
}

.attachment-data {
    margin-top: 0;
}

.cars-section h5,
.employees-section h5,
.items-section h5 {
    font-size: 16px;
    color: #333;
    margin: 0 0 15px 0;
    font-weight: 700;
    padding-top: 10px;
    border-top: 1px solid #e6e6e6;
}

.cars-section:first-child h5,
.employees-section:first-child h5,
.items-section:first-child h5 {
    border-top: none;
    padding-top: 0;
}

.loading-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 40px;
    gap: 15px;
}

.loading-spinner {
    width: 40px;
    height: 40px;
    border: 3px solid #f3f3f3;
    border-top: 3px solid #4F5BDF;
    border-radius: 50%;
    animation: spin 1s linear infinite;
}

.loading-text {
    color: #666;
    font-size: 14px;
    font-weight: 500;
}

.no-data {
    text-align: center;
    color: #a2a2a2;
    padding: 40px 20px;
    font-size: 14px;
    font-style: italic;
}

.cars-list, .employees-list, .items-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.car-item, .employee-item, .item-item {
    padding: 12px;
    background: #f9f9f9;
    border-radius: 15px;
    border: 1px solid #e6e6e6;
    transition: all 0.2s ease;
    animation: slideIn 0.4s ease-out forwards;
    opacity: 0;
    transform: translateY(10px);
}

@keyframes slideIn {
    from {
        opacity: 0;
        transform: translateY(10px);
    }
    to {
        opacity: 1;
        transform: translateY(0);
    }
}

.car-item:hover, .employee-item:hover, .item-item:hover {
    border-color: #4F5BDF;
    background: #f8f9ff;
}

.car-item-content, .employee-item-content, .item-item-content {
    display: flex;
    align-items: center;
    justify-content: flex-start;
    width: 100%;
    gap: 12px;
}

.item-number {
    color: #a2a2a2;
    font-size: 14px;
    font-weight: 500;
    min-width: 20px;
    flex-shrink: 0;
    pointer-events: none;
    user-select: none;
}

.car-main-info, .employee-main-info {
    display: flex;
    gap: 15px;
    min-width: 250px;
    flex-shrink: 0;
}

.car-number {
    font-weight: 600;
    color: #333;
    font-size: 15px;
}

.car-brand {
    color: #666;
    font-size: 14px;
}

.employee-name {
    font-weight: 600;
    color: #333;
    font-size: 15px;
}

.employee-position {
    color: #666;
    font-size: 14px;
}

.unload-places-container, .target-tables-container {
    display: flex;
    gap: 8px;
    font-size: 13px;
    align-items: flex-start;
    flex: 1;
    min-width: 0;
    position: relative;
    cursor: help;
    z-index: 1000;
}

.places-list, .tables-list {
    color: #333;
    line-height: 1.4;
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: end;
    user-select: none;
}

.item-item-content {
    display: flex;
    justify-content: flex-start;
    align-items: center;
    width: 100%;
}

.item-name {
    font-weight: 600;
    color: #333;
    font-size: 15px;
    flex: 1;
}

.item-count {
    color: #4F5BDF;
    font-size: 14px;
    font-weight: 600;
    background: rgba(79, 91, 223, 0.1);
    padding: 4px 10px;
    border-radius: 15px;
    white-space: nowrap;
}

.basic-info-section,
.comment-section {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    padding: 15px;
    margin-bottom: 10px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.comment-section h4,
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

.comment-content {
    font-size: 14px;
    color: #333;
    white-space: pre-wrap;
    background: #f9f9f9;
    border-radius: 10px;
    margin-top: 10px;
}
</style>