<template>
    <!-- Внешний контейнер для модального окна -->
    <div class="application-detail-overlay" @click.self="closeApplicationDetail">
        <!-- Уведомление -->
        <div v-if="notification.show" class="notification" :class="notification.type">
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
                        <h3 class="detail-title">Заявка {{ applicationData.application_number }}</h3>
                        <div class="detail-datetime">
                            {{ formatDateTime(applicationData.sending_datetime) }}
                            <span class="weekday">{{ getWeekday(applicationData.sending_datetime) }}</span>
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
                        <!-- Для пользователей, которые одновременно являются принимающими и ответственными -->
                        <template v-if="isApprover && isResponsibleUser">
                            <!-- Если пользователь еще не голосовал -->
                            <template v-if="!hasUserVoted">
                                <!-- Показываем кнопки согласования, если заявка не отклонена окончательно и не завершена -->
                                <template v-if="applicationData.confirmation !== 'Не согласовано' && applicationData.status !== 'Завершено'">
                                    <button 
                                        class="accept-btn" 
                                        @click="handleCombinedAction('accept')"
                                        :disabled="processingApplication"
                                    >
                                        <span v-if="processingApplication" class="button-loading"></span>
                                        <span v-else>Согласовать и принять</span>
                                    </button>
                                    <button 
                                        class="reject-btn" 
                                        @click="handleCombinedAction('reject')"
                                        :disabled="processingApplication"
                                    >
                                        <span v-if="processingApplication" class="button-loading"></span>
                                        <span v-else>Отказать</span>
                                    </button>
                                </template>
                                <!-- Если заявка завершена -->
                                <div v-else-if="applicationData.status === 'Завершено'" class="status-badge status-completed-badge">
                                    Завершено
                                </div>
                                <!-- Если заявка отклонена окончательно -->
                                <div v-else class="info-badge">
                                    Заявка отклонена
                                </div>
                            </template>
                            
                            <!-- Если пользователь уже проголосовал -->
                            <template v-else>
                                <!-- Если заявка в работе - показываем статус и кнопку отзыва -->
                                <template v-if="applicationData.status === 'В работе'">
                                    <button 
                                        class="subtle-btn" 
                                        @click="revokeApplication"
                                        :disabled="processingApplication"
                                    >
                                        <span v-if="processingApplication" class="button-loading"></span>
                                        <span v-else>Отозвать из работы</span>
                                    </button>
                                    <div class="status-badge status-in-work-badge">
                                        В работе
                                    </div>
                                </template>
                                <!-- Если заявка отказана - показываем статус и кнопку возврата -->
                                <template v-else-if="applicationData.status === 'Отказано'">
                                    <button 
                                        class="subtle-btn" 
                                        @click="restoreApplication"
                                        :disabled="processingApplication"
                                    >
                                        <span v-if="processingApplication" class="button-loading"></span>
                                        <span v-else>Вернуть в работу</span>
                                    </button>
                                    <div class="status-badge status-rejected-badge">
                                        Отказано
                                    </div>
                                </template>
                                <!-- Если заявка завершена - просто показываем статус -->
                                <template v-else-if="applicationData.status === 'Завершено'">
                                    <div class="status-badge status-completed-badge">
                                        Завершено
                                    </div>
                                </template>
                                <!-- Если заявка не в работе, не отказана и не завершена, но согласована - показываем кнопки принять/отказать -->
                                <template v-else-if="applicationData.confirmation === 'Согласовано'">
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
                                <!-- Если пользователь проголосовал, но заявка не согласована (ждет других) -->
                                <div v-else class="vote-status-badge" :class="userVoteStatus.class">
                                    {{ userVoteStatus.text }} (ожидание других)
                                </div>
                            </template>
                        </template>
                        
                        <!-- Для принимающих заявки (не ответственных) -->
                        <template v-else-if="isApprover">
                            <!-- Если заявка в работе - показываем статус и кнопку отзыва -->
                            <template v-if="applicationData.status === 'В работе'">
                                <button 
                                    class="subtle-btn" 
                                    @click="revokeApplication"
                                    :disabled="processingApplication"
                                >
                                    <span v-if="processingApplication" class="button-loading"></span>
                                    <span v-else>Отозвать из работы</span>
                                </button>
                                <div class="status-badge status-in-work-badge">
                                    В работе
                                </div>
                            </template>
                            <!-- Если заявка отказана - показываем статус и кнопку возврата -->
                            <template v-else-if="applicationData.status === 'Отказано'">
                                <button 
                                    class="subtle-btn" 
                                    @click="restoreApplication"
                                    :disabled="processingApplication"
                                >
                                    <span v-if="processingApplication" class="button-loading"></span>
                                    <span v-else>Вернуть в работу</span>
                                </button>
                                <div class="status-badge status-rejected-badge">
                                    Отказано
                                </div>
                            </template>
                            <!-- Если заявка завершена -->
                            <template v-else-if="applicationData.status === 'Завершено'">
                                <div class="status-badge status-completed-badge">
                                    Завершено
                                </div>
                            </template>
                            <!-- Если заявка не в работе и согласована - показываем кнопки принять/отказать -->
                            <template v-else-if="applicationData.confirmation === 'Согласовано'">
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
                        
                        <!-- Для ответственных за согласование (не принимающих) -->
                        <template v-else-if="isResponsibleUser">
                            <!-- Если пользователь еще не голосовал -->
                            <template v-if="!hasUserVoted">
                                <!-- Показываем кнопки согласования, когда заявка не отклонена и не завершена -->
                                <template v-if="applicationData.confirmation !== 'Не согласовано' && applicationData.status !== 'Завершено'">
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
                                <!-- Если заявка завершена -->
                                <div v-else-if="applicationData.status === 'Завершено'" class="status-badge status-completed-badge">
                                    Завершено
                                </div>
                                <!-- Если заявка отклонена окончательно -->
                                <div v-else class="info-badge">
                                    Заявка отклонена
                                </div>
                            </template>
                            
                            <!-- Если пользователь уже проголосовал -->
                            <template v-else>
                                <!-- Если заявка в работе - показываем только статус (нельзя отозвать) -->
                                <template v-if="applicationData.status === 'В работе'">
                                    <div class="vote-status-badge" :class="userVoteStatus.class">
                                        {{ userVoteStatus.text }}
                                    </div>
                                </template>
                                <!-- Если заявка завершена -->
                                <template v-else-if="applicationData.status === 'Завершено'">
                                    <div class="status-badge status-completed-badge">
                                        Завершено
                                    </div>
                                </template>
                                <!-- Если заявка не в работе и не завершена - показываем кнопку отзыва согласования -->
                                <template v-else>
                                    <button 
                                        class="revoke-approval-btn subtle-btn" 
                                        @click="revokeOwnApproval"
                                        :disabled="processingApplication"
                                    >
                                        <span v-if="processingApplication" class="button-loading"></span>
                                        <span v-else>Отозвать своё решение</span>
                                    </button>
                                    <div class="vote-status-badge" :class="userVoteStatus.class">
                                        {{ userVoteStatus.text }}
                                    </div>
                                </template>
                            </template>
                        </template>
                        
                        <!-- Для остальных пользователей - только информация -->
                        <template v-else>
                            <div v-if="applicationData.status === 'В работе'" class="status-badge status-in-work-badge">
                                В работе
                            </div>
                            <div v-else-if="applicationData.status === 'Отказано'" class="status-badge status-rejected-badge">
                                Отказано
                            </div>
                            <div v-else-if="applicationData.status === 'Завершено'" class="status-badge status-completed-badge">
                                Завершено
                            </div>
                            <div v-else-if="applicationData.confirmation === 'Согласовано'" class="status-badge status-approved-badge">
                                Согласовано
                            </div>
                            <div v-else-if="applicationData.confirmation === 'Согласование'" class="status-badge status-pending-badge">
                                На согласовании
                            </div>
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
                        <div class="message-content">
                            {{ applicationData.message || 'Сообщение отсутствует' }}
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
                                <div v-if="selectedAttachment?.attachment_type === 'cars'" class="cars-section">
                                    <h5>Список автомобилей</h5>
                                    <template v-if="loadingAttachmentDetails">
                                        <div class="loading-container">
                                            <div class="loading-spinner"></div>
                                            <span class="loading-text">Загрузка...</span>
                                        </div>
                                    </template>
                                    <template v-else>
                                        <div v-if="attachmentCars.length > 0" class="cars-list">
                                            <div v-for="(car, index) in attachmentCars" :key="car.id" class="car-item">
                                                <div class="car-item-content">
                                                    <div class="item-number">
                                                        {{ index + 1 }}.
                                                    </div>
                                                    <div class="car-main-info">
                                                        <span class="car-number">{{ car.car_number }}</span>
                                                        <span class="car-brand">{{ car.car_brand }}</span>
                                                    </div>
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
                                    </template>
                                </div>

                                <!-- Сотрудники -->
                                <div v-if="selectedAttachment?.attachment_type === 'people'" class="employees-section">
                                    <h5>Сотрудники</h5>
                                    <template v-if="loadingAttachmentDetails">
                                        <div class="loading-container">
                                            <div class="loading-spinner"></div>
                                            <span class="loading-text">Загрузка...</span>
                                        </div>
                                    </template>
                                    <template v-else>
                                        <div v-if="attachmentEmployees.length > 0" class="employees-list">
                                            <div v-for="(employee, index) in attachmentEmployees" :key="employee.id" class="employee-item">
                                                <div class="employee-item-content">
                                                    <div class="item-number">
                                                        {{ index + 1 }}.
                                                    </div>
                                                    <div class="employee-main-info">
                                                        <span class="employee-name">{{ employee.last_name }} {{ employee.first_name }} {{ employee.middle_name || '' }}</span>
                                                        <span class="employee-position">{{ employee.position }}</span>
                                                    </div>
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
                                    </template>
                                </div>

                                <!-- ТМЦ -->
                                <div v-if="selectedAttachment?.attachment_type === 'items'" class="items-section">
                                    <h5>Товарно-материальные ценности</h5>
                                    <template v-if="loadingAttachmentDetails">
                                        <div class="loading-container">
                                            <div class="loading-spinner"></div>
                                            <span class="loading-text">Загрузка...</span>
                                        </div>
                                    </template>
                                    <template v-else>
                                        <div v-if="attachmentItems.length > 0" class="items-list">
                                            <div v-for="(item, index) in attachmentItems" :key="item.id" class="item-item">
                                                <div class="item-item-content">
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
                                    </template>
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
                                <span class="info-value">{{ applicationData.organization_name }}</span>
                            </div>
                            <div v-if="applicationData.company_name" class="info-row">
                                <span class="info-label">Компания:</span>
                                <span class="info-value">{{ applicationData.company_name }}</span>
                            </div>
                            <div class="info-row">
                                <span class="info-label">Отправитель:</span>
                                <span class="info-value">{{ applicationData.sender_full_name || applicationData.sender_name }}</span>
                            </div>
                        </div>
                    </div>

                    <!-- Блок статуса заявки (для принятых/отказанных/завершенных) -->
                    <div v-if="applicationData.status === 'В работе' || applicationData.status === 'Отказано' || applicationData.status === 'Завершено'" class="application-status-section">
                        <div class="status-header">
                            <h4>Статус заявки</h4>
                            <span class="status-mini-badge" :class="getStatusBadgeClass(applicationData.status)">
                                {{ applicationData.status }}
                            </span>
                        </div>
                        
                        <!-- Для статусов В работе и Отказано -->
                        <div class="status-info" v-if="applicationData.status === 'В работе' || applicationData.status === 'Отказано'">
                            <div class="status-info-row" v-if="applicationData.responsible_user_id">
                                <span class="status-info-label">{{ applicationData.status === 'В работе' ? 'Принял(-а):' : 'Отказал(а):' }}</span>
                                <span class="status-info-value">{{ applicationData.responsible_name || 'Не указан' }}</span>
                            </div>
                            <div v-if="applicationData.confirmation_datetime" class="status-info-row">
                                <span class="status-info-label">Время:</span>
                                <span class="status-info-value">{{ formatDateTime(applicationData.confirmation_datetime) }}</span>
                            </div>
                            <div class="status-info-row comment-row">
                                <span class="status-info-label">Комментарий:</span>
                                <div class="status-info-value comment-text">{{ applicationData.responsible_comment || 'Комментария нет' }}</div>
                            </div>
                        </div>

                        <!-- Для статуса Завершено (показываем и принятие, и завершение) -->
                        <div class="status-info" v-else-if="applicationData.status === 'Завершено'">
                            <!-- Информация о принятии -->
                            <div class="status-info-row" v-if="applicationData.responsible_name">
                                <span class="status-info-label">Принял(-а):</span>
                                <span class="status-info-value">{{ applicationData.responsible_name }}</span>
                            </div>
                            <div v-if="applicationData.confirmation_datetime" class="status-info-row">
                                <span class="status-info-label">Время принятия:</span>
                                <span class="status-info-value">{{ formatDateTime(applicationData.confirmation_datetime) }}</span>
                            </div>
                            <!-- Информация о завершении -->
                            <div class="status-info-row" v-if="applicationData.completed_by_name">
                                <span class="status-info-label">Завершил(-а):</span>
                                <span class="status-info-value">{{ applicationData.completed_by_name }}</span>
                            </div>
                            <div v-if="applicationData.completed_at" class="status-info-row">
                                <span class="status-info-label">Время завершения:</span>
                                <span class="status-info-value">{{ formatDateTime(applicationData.completed_at) }}</span>
                            </div>
                            <!-- Комментарий к завершению (или общий) -->
                            <div class="status-info-row comment-row">
                                <span class="status-info-label">Комментарий:</span>
                                <div class="status-info-value comment-text">{{ applicationData.completion_comment || 'Комментария нет' }}</div>
                            </div>
                        </div>
                    </div>

                    <!-- Поле для комментария (только для пользователей, которые еще не выполнили действие) -->
                    <div v-if="canLeaveComment && !hasUserVoted && !isApproverActionDone" class="comment-action-section">
                        <h4>Комментарий</h4>
                        <textarea
                            v-model="actionComment"
                            class="comment-action-textarea"
                            placeholder="Вы можете написать здесь комментарий (необязательно)"
                            rows="3"
                            @input="saveCommentToLocalStorage"
                        ></textarea>
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
    </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { markAsRead } from '@/api/applications'
import ApplicationAttachments from './ApplicationAttachments.vue'
import ApplicationConfirmation from './ApplicationConfirmation.vue'
import ApplicationHistory from './ApplicationHistory.vue'
import ForwardModal from './ForwardModal.vue'

export default {
    name: 'ApplicationDetail',
    components: {
        ApplicationAttachments,
        ApplicationConfirmation,
        ApplicationHistory,
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
            default: 'center'
        }
    },
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
            storageKey: ''
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

        canUserApprove() {
            if (!this.responsibleUsers.length) return true;
            
            const requiredUsers = this.responsibleUsers.filter(user => user.required_approval);
            
            if (requiredUsers.length === 0) return true;
            
            const hasRequiredRejected = requiredUsers.some(user => user.approval_status === 'rejected');
            
            if (hasRequiredRejected && this.applicationData.confirmation === 'Не согласовано') {
                return false;
            }
            
            return true;
        },

        getApproverStatusMessage() {
            if (this.applicationData.status === 'В работе') {
                return 'Заявка уже в работе';
            }
            
            if (this.applicationData.status === 'Отказано') {
                return 'Заявка отклонена';
            }

            if (this.applicationData.status === 'Завершено') {
                return 'Заявка завершена';
            }
            
            if (this.applicationData.confirmation !== 'Согласовано') {
                if (!this.canUserApprove) {
                    return 'Ожидание обязательных согласующих';
                }
                return 'Ожидает согласования';
            }
            
            return 'Готова к принятию';
        }
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
    methods: {
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

        async revokeOwnApproval() {
            if (!confirm('Вы уверены, что хотите отозвать своё решение?')) return;

            this.processingApplication = true;
            try {
                const response = await apiRequest(`/applications/${this.applicationData.id}/revoke-approval`, {
                    method: "POST",
                    body: JSON.stringify({
                        comment: null
                    })
                });

                if (response.ok) {
                    const result = await response.json();
                    
                    this.showNotification("Ваше решение отозвано", "success");
                    
                    this.applicationData = {
                        ...this.applicationData,
                        status: result.status,
                        confirmation: result.confirmation
                    };
                    
                    this.saveCommentToLocalStorage();
                    
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
                console.error("Ошибка при отзыве решения:", error);
                this.showNotification("Ошибка сети при отзыве решения", "error");
            } finally {
                this.processingApplication = false;
            }
        },

        async handleCombinedAction(action) {
            this.processingApplication = true;
            try {
                const commentToSend = this.actionComment;
                this.lastUserComment = commentToSend;

                const approvalResponse = await apiRequest(`/applications/${this.applicationData.id}/approve`, {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json"
                    },
                    body: JSON.stringify({
                        user_id: this.currentUserId,
                        status: action === 'accept' ? 'approved' : 'rejected',
                        comment: commentToSend || null
                    })
                });

                if (!approvalResponse.ok) {
                    const errorText = await approvalResponse.text();
                    throw new Error(errorText);
                }

                if (action === 'accept') {
                    await this.acceptApplication();
                } else {
                    await this.rejectApplication();
                }
                
            } catch (error) {
                console.error(`Ошибка при комбинированном действии:`, error);
                this.showNotification(`Ошибка: ${error.message}`, 'error');
            } finally {
                this.processingApplication = false;
            }
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
                const commentToSend = this.actionComment;
                this.lastUserComment = commentToSend;

                const response = await apiRequest(`/applications/${this.applicationData.id}/take-to-work`, {
                    method: "POST",
                    body: JSON.stringify({
                        user_id: this.currentUserId,
                        action: 'accept',
                        comment: commentToSend || null
                    })
                });

                if (response.ok) {
                    this.showNotification("Заявка принята в работу", "success");
                    
                    this.applicationData = {
                        ...this.applicationData,
                        status: 'В работе'
                    };
                    
                    this.clearCommentFromLocalStorage();
                    
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
                console.error("Ошибка при принятии заявки:", error);
                this.showNotification("Ошибка сети при принятии заявки", "error");
            }
        },

        async rejectApplication() {
            try {
                const commentToSend = this.actionComment;
                this.lastUserComment = commentToSend;

                const response = await apiRequest(`/applications/${this.applicationData.id}/take-to-work`, {
                    method: "POST",
                    body: JSON.stringify({
                        user_id: this.currentUserId,
                        action: 'reject',
                        comment: commentToSend || null
                    })
                });

                if (response.ok) {
                    this.showNotification("Заявка отклонена", "error");
                    
                    this.applicationData = {
                        ...this.applicationData,
                        status: 'Отказано'
                    };
                    
                    this.clearCommentFromLocalStorage();
                    
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
                console.error("Ошибка при отказе заявки:", error);
                this.showNotification("Ошибка сети при отказе заявки", "error");
            }
        },

        async revokeApplication() {
            this.processingApplication = true;
            try {
                const response = await apiRequest(`/applications/${this.applicationData.id}/revoke-from-work`, {
                    method: "POST",
                    body: JSON.stringify({
                        user_id: this.currentUserId,
                        comment: null
                    })
                });

                if (response.ok) {
                    this.showNotification("Заявка отозвана из работы", "success");
                    
                    this.applicationData = {
                        ...this.applicationData,
                        status: 'В обработке'
                    };
                    
                    this.saveCommentToLocalStorage();
                    
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
                console.error("Ошибка при отзыве заявки:", error);
                this.showNotification("Ошибка сети при отзыве заявки", "error");
            } finally {
                this.processingApplication = false;
            }
        },

        async restoreApplication() {
            this.processingApplication = true;
            try {
                const commentToSend = this.actionComment;
                this.lastUserComment = commentToSend;

                const response = await apiRequest(`/applications/${this.applicationData.id}/restore-to-work`, {
                    method: "POST",
                    body: JSON.stringify({
                        user_id: this.currentUserId,
                        comment: commentToSend || null
                    })
                });

                if (response.ok) {
                    this.showNotification("Заявка возвращена в работу", "success");
                    
                    this.applicationData = {
                        ...this.applicationData,
                        status: 'В обработке'
                    };
                    
                    this.saveCommentToLocalStorage();
                    
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
                console.error("Ошибка при возврате заявки:", error);
                this.showNotification("Ошибка сети при возврате заявки", "error");
            } finally {
                this.processingApplication = false;
            }
        },

        async updateConfirmation(confirmation) {
            if (!this.applicationData || !this.isResponsibleUser) return;

            this.updatingConfirmation = true;
            try {
                if (this.hasUserVoted) {
                    this.showNotification("Вы уже проголосовали по этой заявке", "error");
                    this.updatingConfirmation = false;
                    return;
                }

                const commentToSend = this.actionComment;
                this.lastUserComment = commentToSend;

                const userApprovalResponse = await apiRequest(`/applications/${this.applicationData.id}/approve`, {
                    method: "POST",
                    body: JSON.stringify({
                        user_id: this.currentUserId,
                        status: confirmation === 'Согласовано' ? 'approved' : 'rejected',
                        comment: commentToSend || null
                    })
                });

                if (!userApprovalResponse.ok) {
                    const errorText = await userApprovalResponse.text();
                    throw new Error(errorText || "Error updating application confirmation");
                }

                const checkResponse = await apiRequest(`/applications/${this.applicationData.id}/check-approval-status`, {
                    method: "GET"});

                if (checkResponse.ok) {
                    const approvalStatus = await checkResponse.json();
                    
                    this.applicationData = {
                        ...this.applicationData,
                        confirmation: approvalStatus.confirmation,
                        ...(approvalStatus.status && { status: approvalStatus.status })
                    };
                }
                
                this.clearCommentFromLocalStorage();
                
                await this.loadApplicationDetails(this.applicationData);
                
                if (this.$refs.historyComponent) {
                    this.$refs.historyComponent.loadHistory();
                }
                
                this.showNotification(
                    confirmation === 'Согласовано' 
                        ? 'Заявка согласована'
                        : 'Заявка отклонена',
                    confirmation === 'Согласовано' ? 'success' : 'error'
                );
                
                this.$emit('application-changed', this.applicationData);
                
            } catch (error) {
                console.error("Ошибка при обновлении подтверждения:", error);
                this.showNotification(`Ошибка: ${error.message}`, 'error');
            } finally {
                this.updatingConfirmation = false;
            }
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

        duplicateApplication() {
            console.log('Дублирование заявки:', this.applicationData.application_number);
            this.$emit('duplicate', this.applicationData);
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
    emits: ['close', 'confirmation-updated', 'duplicate', 'application-updated', 'update-application', 'application-changed']
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
    z-index: 10000;
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

.action-buttons {
    display: flex;
    gap: 5px;
    align-items: center;
    flex-wrap: wrap;
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

.subtle-btn {
    padding: 6px 24px;
    border: none;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s ease;
    min-width: 140px;
    background: transparent;
    color: #a2a2a2;
    border: 1px solid #e6e6e6;
}

.subtle-btn:hover:not(:disabled) {
    background: #f5f5f5;
    color: #666;
}

.subtle-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.revoke-approval-btn {
    border-color: #f59e0b;
    color: #f59e0b;
}

.revoke-approval-btn:hover:not(:disabled) {
    background: #fef3c7;
    color: #d97706;
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
.accept-btn:disabled,
.subtle-btn:disabled {
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
    max-height: 500px;
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
    max-height: 330px;
    overflow: scroll;
    gap: 8px;
}

.car-item, .employee-item, .item-item {
    padding: 12px;
    background: #f9f9f9;
    border-radius: 15px;
    border: 1px solid #e6e6e6;
    transition: all 0.2s ease;
    animation: slideIn 0.3s ease-out forwards;
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

.status-rejected-badge {
    background: rgba(220, 38, 38, 0.1);
    color: #dc2626;
    border: 1px solid rgba(220, 38, 38, 0.3);
    padding: 6px 24px;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    min-width: 120px;
    text-align: center;
}

.status-approved-badge {
    background: rgba(5, 150, 105, 0.1);
    color: #059669;
    border: 1px solid rgba(5, 150, 105, 0.3);
    padding: 6px 24px;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    min-width: 120px;
    text-align: center;
}

.status-pending-badge {
    background: rgba(217, 119, 6, 0.1);
    color: #d97706;
    border: 1px solid rgba(217, 119, 6, 0.3);
    padding: 6px 24px;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    min-width: 120px;
    text-align: center;
}

.status-completed-badge {
    background: rgba(5, 150, 105, 0.1);
    color: #059669;
    border: 1px solid rgba(5, 150, 105, 0.3);
    padding: 6px 24px;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    min-width: 120px;
    text-align: center;
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