<template>
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
        </div>

        <!-- Ответственные пользователи -->
        <div v-if="sortedResponsibleUsers.length > 0" class="responsible-users-section">
            <h5>Ответственные за согласование ({{ sortedResponsibleUsers.length }}):</h5>
            <div class="users-list">
                <div v-for="user in sortedResponsibleUsers" :key="user.id" class="user-item">
                    <!-- ФИО -->
                    <div class="user-name-block">
                        {{ getUserDisplayName(user) }}
                    </div>
                    
                    <!-- Должность -->
                    <div v-if="user.position" class="user-position-block">
                        {{ user.position }}
                    </div>
                    
                    <!-- Ряд с бейджем и статусом -->
                    <div class="user-badge-status-row">
                        <span v-if="user.required_approval" class="badge required-badge">Обязательно</span>
                        <span class="status-badge" :class="getStatusClass(user.approval_status)">
                            {{ getStatusText(user.approval_status) }}
                        </span>
                    </div>
                    
                    <!-- Время -->
                    <div v-if="user.approval_datetime" class="user-time-block">
                        Время: {{ formatDateTime(user.approval_datetime) }}
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script>
export default {
    name: 'ApplicationConfirmation',
    props: {
        application: {
            type: Object,
            required: true
        },
        responsibleUsers: {
            type: Array,
            required: true
        },
        updatingConfirmation: {
            type: Boolean,
            default: false
        }
    },
    computed: {
        sortedResponsibleUsers() {
            if (!this.responsibleUsers || this.responsibleUsers.length === 0) {
                return [];
            }
            
            // Разделяем пользователей на обязательных и остальных
            const required = this.responsibleUsers.filter(user => user.required_approval === true);
            const others = this.responsibleUsers.filter(user => !user.required_approval);
            
            // Сортируем по алфавиту (по ФИО)
            const sortByName = (a, b) => {
                const nameA = this.getUserDisplayName(a).toLowerCase();
                const nameB = this.getUserDisplayName(b).toLowerCase();
                return nameA.localeCompare(nameB);
            };
            
            required.sort(sortByName);
            others.sort(sortByName);
            
            // Объединяем: сначала обязательные, потом остальные
            return [...required, ...others];
        }
    },
    methods: {
        getConfirmationClass(confirmation) {
            const classes = {
                'Согласовано': 'confirmation-approved',
                'Согласование': 'confirmation-pending',
                'Не согласовано': 'confirmation-rejected'
            };
            return classes[confirmation] || 'confirmation-default';
        },
        
        getUserDisplayName(user) {
            const names = [user.last_name, user.first_name, user.middle_name].filter(Boolean);
            return names.length > 0 ? names.join(' ') : user.username;
        },
        
        getStatusText(status) {
            const statusMap = {
                'approved': 'Согласовано',
                'rejected': 'Отказано',
                'pending': 'Ожидание'
            };
            return statusMap[status] || 'Неизвестно';
        },
        
        getStatusClass(status) {
            const classes = {
                'approved': 'status-approved',
                'rejected': 'status-rejected',
                'pending': 'status-pending'
            };
            return classes[status] || 'status-default';
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

        formatPhone(phone) {
            if (!phone) return '';
            const cleaned = phone.replace(/\D/g, '');
            
            if (cleaned.length === 11) {
                return `+${cleaned[0]} (${cleaned.substring(1, 4)}) ${cleaned.substring(4, 7)}-${cleaned.substring(7, 9)}-${cleaned.substring(9)}`;
            } else if (cleaned.length === 10) {
                return `+7 (${cleaned.substring(0, 3)}) ${cleaned.substring(3, 6)}-${cleaned.substring(6, 8)}-${cleaned.substring(8)}`;
            }
            
            return phone;
        }
    }
}
</script>

<style scoped>
.confirmation-section {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    padding: 15px;
    margin-bottom: 10px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
    position: relative;
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

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}

.confirmation-status-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    padding: 3px 0;
}

.confirmation-info {
    margin-bottom: 20px;
    transition: opacity 0.3s ease;
}

.confirmation-label {
    color: #a2a2a2;
    font-size: 14px;
    font-weight: 400;
    min-width: 120px;
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
    flex-direction: column;
    padding: 12px;
    background: #f9f9f9;
    border-radius: 15px;
    border: 1px solid #e6e6e6;
    transition: all 0.2s ease;
    gap: 3px;
}

.user-item:hover {
    border-color: #4F5BDF;
    background: #f8f9ff;
}

.user-name-block {
    font-weight: 600;
    color: #333;
    font-size: 14px;
    line-height: 1.4;
}

.user-position-block {
    color: #666;
    font-size: 12px;
    font-style: italic;
    line-height: 1.4;
    margin-bottom: 2px;
}

.user-badge-status-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 2px 0;
}

.user-time-block {
    font-size: 11px;
    color: #8f8f8f;
    border-radius: 6px;
    display: inline-block;
    align-self: flex-start;
    padding-top: 5px;
}

.status-badge {
    padding: 4px 12px;
    border-radius: 20px;
    font-size: 11px;
    font-weight: 600;
    white-space: nowrap;
    display: inline-block;
}

.status-approved {
    background-color: #f0f9ff;
    color: #059669;
    border: 1px solid #a7f3d0;
}

.status-rejected {
    background-color: #fef2f2;
    color: #dc2626;
    border: 1px solid #fecaca;
}

.status-pending {
    background-color: #fffbeb;
    color: #d97706;
    border: 1px solid #fcd34d;
}

.status-default {
    background-color: #f0f0f0;
    color: #666;
    border: 1px solid #ddd;
}

.badge {
    padding: 3px 10px;
    border-radius: 20px;
    font-size: 11px;
    font-weight: 600;
    display: inline-block;
    white-space: nowrap;
}

.required-badge {
    background: #4F5BDF;
    color: #fff;
}
</style>