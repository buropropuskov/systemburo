<template>
    <div class="application-detail-overlay" @click.self="closeApplicationDetail">
        <!-- Уведомление -->
        <div v-if="notification.show" class="notification" :class="notification.type">
            {{ notification.message }}
        </div>

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
                    </div>
                </div>
                <div class="detail-header-right">
                    <!-- Кнопки согласования для ответственных -->
                    <div v-if="isResponsibleUser && application.confirmation === 'Согласование'" class="confirmation-buttons">
                        <button 
                            class="confirm-btn" 
                            @click="updateConfirmation('Согласовано')"
                            :disabled="updatingConfirmation"
                        >
                            <span v-if="updatingConfirmation" class="button-loading"></span>
                            <span v-else>Согласовать</span>
                        </button>
                        <button 
                            class="reject-btn" 
                            @click="updateConfirmation('Не согласовано')"
                            :disabled="updatingConfirmation"
                        >
                            <span v-if="updatingConfirmation" class="button-loading"></span>
                            <span v-else>Отказать</span>
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

                    <!-- Статус согласования -->
                    <div class="confirmation-section">
                        <div class="confirmation-header">
                            <h4>Согласование заявки</h4>
                            <div v-if="updatingConfirmation" class="confirmation-loading">
                                <div class="loader"></div>
                            </div>
                        </div>
                        <div class="confirmation-info">
                            <div class="confirmation-status-row">
                                <span class="confirmation-label">Статус:</span>
                                <span class="confirmation-badge" :class="getConfirmationClass(application.confirmation)">
                                    {{ application.confirmation }}
                                </span>
                            </div>
                            <!-- Ответственный (отображается только при согласовании/отказе) -->
                            <div v-if="application.confirmation !== 'Согласование' && application.responsible_name" class="confirmation-info-row">
                                <span class="confirmation-label">Ответственный:</span>
                                <span class="confirmation-value">{{ application.responsible_name }}</span>
                            </div>
                            
                            <!-- Время согласования (отображается только при согласовании/отказе) -->
                            <div v-if="application.confirmation !== 'Согласование' && application.confirmation_datetime" class="confirmation-info-row">
                                <span class="confirmation-label">Время:</span>
                                <span class="confirmation-value">
                                    {{ formatDateTimeFull(application.confirmation_datetime) }}
                                </span>
                            </div>
                        </div>

                        <!-- Ответственные пользователи -->
                        <div v-if="responsibleUsers.length > 0" class="responsible-users-section">
                            <h5>Ответственные за согласование ({{ responsibleUsers.length }}):</h5>
                            <div class="users-list">
                                <div v-for="(user, index) in responsibleUsers" :key="user.id" class="user-item">
                                    <div class="user-info">
                                        <!-- Номер порядковый для ответственных -->
                                        <div class="user-number">
                                            {{ index + 1 }}.
                                        </div>
                                        <div class="user-details">
                                            <span class="user-name">{{ user.last_name }} {{ user.first_name }} {{ user.middle_name || '' }}</span>
                                            <span v-if="user.position" class="user-position">{{ user.position }}</span>
                                            <span v-if="user.phone" class="user-phone">{{ formatPhone(user.phone) }}</span>
                                        </div>
                                    </div>
                                    <span v-if="user.is_primary" class="primary-badge">Основной</span>
                                </div>
                            </div>
                        </div>
                    </div>

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

export default {
    name: 'ApplicationDetail',
    components: {
        ApplicationAttachments
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
            loadingAttachmentDetails: false,
            loadingApplicationDetails: false,
            isLeftColumnCollapsed: false,
            notification: {
                show: false,
                message: '',
                type: 'success' // 'success' или 'error'
            }
        }
    },
    computed: {
        isResponsibleUser() {
            if (!this.currentUserId || !this.responsibleUsers.length) return false;
            return this.responsibleUsers.some(user => user.id === this.currentUserId);
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
                
                // Загружаем детали заявки
                const appResponse = await fetch(`http://localhost:8080/applications/${application.id}/details`, {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                        "Content-Type": "application/json"
                    },
                });

                if (appResponse.ok) {
                    const appData = await appResponse.json();
                    
                    // Если есть ответственные пользователи
                    if (appData.responsible_users) {
                        this.responsibleUsers = appData.responsible_users;
                    }
                }

                // Загружаем вложения (теперь с информацией о unique_attachments)
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

            } catch (error) {
                console.error("Ошибка при загрузке деталей заявки:", error);
            } finally {
                this.loadingApplicationDetails = false;
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

        async updateConfirmation(confirmation) {
            if (!this.application || !this.isResponsibleUser) return;

            this.updatingConfirmation = true;
            try {
                const token = localStorage.getItem("token");
                
                let newStatus = confirmation === 'Согласовано' ? 'В работе' : 'Отказано';
                
                // 1. Обновление подтверждения заявки
                const response = await fetch(`http://localhost:8080/applications/${this.application.id}`, {
                    method: "PUT",
                    headers: {
                        "Authorization": `Bearer ${token}`,
                        "Content-Type": "application/json"
                    },
                    body: JSON.stringify({
                        confirmation: confirmation,
                        status: newStatus,
                        responsible_comment: confirmation === 'Согласовано' ? 
                            `Заявка согласована пользователем ${this.currentUserName}` : 
                            `Заявка отклонена пользователем ${this.currentUserName}`,
                        responsible_name: this.currentUserName,
                        confirmation_datetime: new Date().toISOString()
                    })
                });

                if (response.ok) {
                    // 2. Если заявка согласована, обновляем статусы машин и сотрудников
                    if (confirmation === 'Согласовано') {
                        await fetch(`http://localhost:8080/applications/${this.application.id}/update-items-status`, {
                            method: "POST",
                            headers: {
                                "Authorization": `Bearer ${token}`,
                                "Content-Type": "application/json"
                            },
                        });
                    }
                    
                    // Обновляем локальные данные
                    this.$emit('confirmation-updated', {
                        confirmation,
                        status: newStatus,
                        confirmation_datetime: new Date().toISOString(),
                        responsible_comment: confirmation === 'Согласовано' ? 
                            `Заявка согласована пользователем ${this.currentUserName}` : 
                            `Заявка отклонена пользователем ${this.currentUserName}`,
                        responsible_name: this.currentUserName
                    });
                    
                    // Показываем уведомление
                    this.showNotification(
                        confirmation === 'Согласовано' 
                            ? 'Заявка согласована'
                            : 'Заявка отклонена',
                        confirmation === 'Согласовано' ? 'success' : 'error'
                    );
                } else {
                    const errorText = await response.text();
                    console.error("Ошибка при обновлении подтверждения:", errorText);
                    this.showNotification(`Ошибка: ${errorText}`, 'error');
                }
            } catch (error) {
                console.error("Ошибка сети при обновлении подтверждения:", error);
                this.showNotification("Ошибка сети при обновлении статуса", 'error');
            } finally {
                this.updatingConfirmation = false;
            }
        },

        showNotification(message, type = 'success') {
            this.notification = {
                show: true,
                message,
                type
            };
            
            // Автоматически скрыть уведомление через 3 секунды
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
                // Если даты одинаковые, показываем только одну
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
            // Убираем секунды, если они есть
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

        formatDateTimeFull(dateTimeString) {
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

        formatTimeOnly(dateTimeString) {
            if (!dateTimeString) return '';
            const date = new Date(dateTimeString);
            return date.toLocaleTimeString('ru-RU', {
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

        formatPhone(phone) {
            if (!phone) return '';
            const cleaned = phone.replace(/\D/g, '');
            
            if (cleaned.length === 11) {
                return `+${cleaned[0]} (${cleaned.substring(1, 4)}) ${cleaned.substring(4, 7)}-${cleaned.substring(7, 9)}-${cleaned.substring(9)}`;
            } else if (cleaned.length === 10) {
                return `+7 (${cleaned.substring(0, 3)}) ${cleaned.substring(3, 6)}-${cleaned.substring(6, 8)}-${cleaned.substring(8)}`;
            }
            
            return phone;
        },

        getConfirmationClass(confirmation) {
            const classes = {
                'Согласовано': 'confirmation-approved',
                'Согласование': 'confirmation-pending',
                'Не согласовано': 'confirmation-rejected'
            };
            return classes[confirmation] || 'confirmation-default';
        },

        toggleLeftColumn() {
            this.isLeftColumnCollapsed = !this.isLeftColumnCollapsed;
        },

        close() {
            this.$emit('close');
        }
    },
    emits: ['close', 'confirmation-updated']
}
</script>

<style scoped>
/* Стили для детального представления заявки */
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
    z-index: 1000;
    animation: fadeIn 0.3s ease-out;
}

@keyframes fadeIn {
    from {
        opacity: 0;
    }
    to {
        opacity: 1;
    }
}

/* Стили для уведомлений */
.notification {
    position: fixed;
    top: 40px;
    left: 50%;
    transform: translateX(-50%);
    padding: 8px 8px;
    border-radius: 50px;
    z-index: 2000;
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
    animation: scaleIn 0.3s ease-out;
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

.detail-header-right {
    display: flex;
    align-items: center;
    gap: 15px;
}

.confirmation-buttons {
    display: flex;
    gap: 10px;
}

.confirm-btn, .reject-btn {
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

.confirm-btn {
    background: rgba(9, 136, 0, 1);
    color: white;
}

.confirm-btn:hover:not(:disabled) {
    background: #45b371;
}

.reject-btn {
    background: #FF6668;
    color: white;
}

.reject-btn:hover:not(:disabled) {
    background: #ff4d4f;
}

.confirm-btn:disabled,
.reject-btn:disabled {
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
    gap: 12px;
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
    min-width: 25px;
    flex-shrink: 0;
}

.car-main-info, .employee-main-info {
    display: flex;
    gap: 10px;
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

.unload-places-container:hover::after,
.target-tables-container:hover::after {
    opacity: 1;
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
    border-radius: 6px;
    white-space: nowrap;
}

.basic-info-section,
.confirmation-section,
.comment-section {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    padding: 15px;
    margin-bottom: 10px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.confirmation-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 15px;
}

.confirmation-header h4 {
    font-size: 18px;
    color: #4F5BDF;
    font-weight: 700;
    margin: 0;
}

.confirmation-loading {
    display: flex;
    align-items: center;
}

.confirmation-loading .loader {
    width: 20px;
    height: 20px;
    border: 3px solid #f3f3f3;
    border-top: 3px solid #4F5BDF;
    border-radius: 50%;
    animation: spin 1s linear infinite;
}

.confirmation-status-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    padding: 3px 0;
}

.confirmation-label {
    color: #a2a2a2;
    font-size: 14px;
    font-weight: 400;
    white-space: nowrap;
}

.confirmation-badge {
    padding: 4px 8px;
    border-radius: 12px;
    font-size: 11px;
    font-weight: 500;
    display: inline-block;
    border: 1px solid;
    transition: all 0.3s ease;
}

.confirmation-badge.confirmation-approved {
    background-color: #f0f9ff;
    color: #059669;
    border-color: #a7f3d0;
}

.confirmation-badge.confirmation-pending {
    background-color: #fffbeb;
    color: #d97706;
    border-color: #fcd34d;
}

.confirmation-badge.confirmation-rejected {
    background-color: #fef2f2;
    color: #dc2626;
    border-color: #fecaca;
}

.confirmation-info {
    margin-bottom: 20px;
    transition: opacity 0.3s ease;
}

.confirmation-info-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 5px 0;
}

.confirmation-info-row:last-child {
    border-bottom: none;
}

.confirmation-label {
    color: #a2a2a2;
    font-size: 14px;
    font-weight: 400;
    min-width: 120px;
}

.confirmation-value {
    color: #000;
    font-size: 14px;
    font-weight: 500;
    text-align: right;
    flex: 1;
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

.responsible-users-section h5 {
    font-size: 14px;
    color: #a2a2a2;
    margin: 0 0 10px 0;
    font-weight: 400;
}

.users-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
}

.user-item {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: 12px;
    background: #f9f9f9;
    border-radius: 10px;
    border: 1px solid #e6e6e6;
    transition: all 0.2s ease;
}

.user-item:hover {
    border-color: #4F5BDF;
    background: #f8f9ff;
}

.user-info {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    flex: 1;
}

.user-number {
    color: #a2a2a2;
    font-size: 14px;
    font-weight: 500;
    margin-top: 2px;
    flex-shrink: 0;
}

.user-details {
    display: flex;
    flex-direction: column;
    gap: 4px;
    flex: 1;
}

.user-name {
    font-weight: 600;
    color: #333;
    font-size: 14px;
}

.user-position {
    color: #666;
    font-size: 12px;
    font-style: italic;
}

.user-phone {
    color: #4F5BDF;
    font-size: 12px;
    font-weight: 500;
    margin-top: 2px;
}

.primary-badge {
    background: linear-gradient(135deg, #4F5BDF 0%, #3a45c0 100%);
    color: white;
    padding: 4px 10px;
    border-radius: 12px;
    font-size: 11px;
    font-weight: 600;
    margin-left: 10px;
    white-space: nowrap;
    box-shadow: 0 2px 4px rgba(79, 91, 223, 0.2);
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