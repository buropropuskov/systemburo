<template>
    <div class="action-buttons-wrapper">
        <!-- Режим центра заявок -->
        <div v-if="mode === 'center'" class="action-buttons">
            <!-- Для пользователей, которые одновременно являются принимающими и ответственными -->
            <template v-if="isApproverUser && isResponsibleUser">
                <!-- Если пользователь еще не голосовал -->
                <template v-if="!hasUserVoted">
                    <!-- Показываем кнопки согласования, если заявка не отклонена окончательно и не завершена -->
                    <template v-if="application.confirmation !== 'Не согласовано' && application.status !== 'Завершено'">
                        <button
                            class="accept-btn"
                            data-testid="app-detail-button-approve"
                            @click="handleCombinedAction('accept')"
                            :disabled="processing"
                        >
                            <span v-if="processing" class="button-loading"></span>
                            <span v-else>Согласовать и принять</span>
                        </button>
                        <button
                            class="reject-btn"
                            data-testid="app-detail-button-reject"
                            @click="handleCombinedAction('reject')"
                            :disabled="processing"
                        >
                            <span v-if="processing" class="button-loading"></span>
                            <span v-else>Отказать</span>
                        </button>
                    </template>
                    <!-- Если заявка завершена -->
                    <div v-else-if="application.status === 'Завершено'" class="status-badge status-completed-badge">
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
                    <template v-if="application.status === 'В работе'">
                        <button
                            class="subtle-btn"
                            @click="revokeApplication"
                            :disabled="processing"
                        >
                            <span v-if="processing" class="button-loading"></span>
                            <span v-else>Отозвать из работы</span>
                        </button>
                        <div class="status-badge status-in-work-badge">
                            В работе
                        </div>
                    </template>
                    <!-- Если заявка отказана - показываем статус и кнопку возврата -->
                    <template v-else-if="application.status === 'Отказано'">
                        <button
                            class="subtle-btn"
                            @click="restoreApplication"
                            :disabled="processing"
                        >
                            <span v-if="processing" class="button-loading"></span>
                            <span v-else>Вернуть в работу</span>
                        </button>
                        <div class="status-badge status-rejected-badge">
                            Отказано
                        </div>
                    </template>
                    <!-- Если заявка завершена - просто показываем статус -->
                    <template v-else-if="application.status === 'Завершено'">
                        <div class="status-badge status-completed-badge">
                            Завершено
                        </div>
                    </template>
                    <!-- Если заявка не в работе, не отказана и не завершена, но согласована - показываем кнопки принять/отказать -->
                    <template v-else-if="application.confirmation === 'Согласовано'">
                        <button
                            class="accept-btn"
                            data-testid="app-detail-button-take-to-work"
                            @click="handleApplicationAction('accept')"
                            :disabled="processing"
                        >
                            <span v-if="processing" class="button-loading"></span>
                            <span v-else>Принять</span>
                        </button>
                        <button
                            class="reject-btn"
                            data-testid="app-detail-button-reject"
                            @click="handleApplicationAction('reject')"
                            :disabled="processing"
                        >
                            <span v-if="processing" class="button-loading"></span>
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
            <template v-else-if="isApproverUser">
                <!-- Если заявка в работе - показываем статус и кнопку отзыва -->
                <template v-if="application.status === 'В работе'">
                    <button
                        class="subtle-btn"
                        @click="revokeApplication"
                        :disabled="processing"
                    >
                        <span v-if="processing" class="button-loading"></span>
                        <span v-else>Отозвать из работы</span>
                    </button>
                    <div class="status-badge status-in-work-badge">
                        В работе
                    </div>
                </template>
                <!-- Если заявка отказана - показываем статус и кнопку возврата -->
                <template v-else-if="application.status === 'Отказано'">
                    <button
                        class="subtle-btn"
                        @click="restoreApplication"
                        :disabled="processing"
                    >
                        <span v-if="processing" class="button-loading"></span>
                        <span v-else>Вернуть в работу</span>
                    </button>
                    <div class="status-badge status-rejected-badge">
                        Отказано
                    </div>
                </template>
                <!-- Если заявка завершена -->
                <template v-else-if="application.status === 'Завершено'">
                    <div class="status-badge status-completed-badge">
                        Завершено
                    </div>
                </template>
                <!-- Если заявка не в работе и согласована - показываем кнопки принять/отказать -->
                <template v-else-if="application.confirmation === 'Согласовано'">
                    <button
                        class="accept-btn"
                        data-testid="app-detail-button-take-to-work"
                        @click="handleApplicationAction('accept')"
                        :disabled="processing"
                    >
                        <span v-if="processing" class="button-loading"></span>
                        <span v-else>Принять</span>
                    </button>
                    <button
                        class="reject-btn"
                        data-testid="app-detail-button-reject"
                        @click="handleApplicationAction('reject')"
                        :disabled="processing"
                    >
                        <span v-if="processing" class="button-loading"></span>
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
                    <template v-if="application.confirmation !== 'Не согласовано' && application.status !== 'Завершено'">
                        <button
                            class="confirm-btn"
                            data-testid="app-detail-button-approve"
                            @click="updateConfirmation('Согласовано')"
                            :disabled="updatingConfirmation || processing"
                        >
                            <span v-if="updatingConfirmation" class="button-loading"></span>
                            <span v-else>Согласовать</span>
                        </button>
                        <button
                            class="reject-btn"
                            data-testid="app-detail-button-reject"
                            @click="updateConfirmation('Не согласовано')"
                            :disabled="updatingConfirmation || processing"
                        >
                            <span v-if="updatingConfirmation" class="button-loading"></span>
                            <span v-else>Отказать</span>
                        </button>
                    </template>
                    <!-- Если заявка завершена -->
                    <div v-else-if="application.status === 'Завершено'" class="status-badge status-completed-badge">
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
                    <template v-if="application.status === 'В работе'">
                        <div class="vote-status-badge" :class="userVoteStatus.class">
                            {{ userVoteStatus.text }}
                        </div>
                    </template>
                    <!-- Если заявка завершена -->
                    <template v-else-if="application.status === 'Завершено'">
                        <div class="status-badge status-completed-badge">
                            Завершено
                        </div>
                    </template>
                    <!-- Если заявка не в работе и не завершена - показываем кнопку отзыва согласования -->
                    <template v-else>
                        <button
                            class="revoke-approval-btn subtle-btn"
                            @click="revokeOwnApproval"
                            :disabled="processing"
                        >
                            <span v-if="processing" class="button-loading"></span>
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
                <div v-if="application.status === 'В работе'" class="status-badge status-in-work-badge">
                    В работе
                </div>
                <div v-else-if="application.status === 'Отказано'" class="status-badge status-rejected-badge">
                    Отказано
                </div>
                <div v-else-if="application.status === 'Завершено'" class="status-badge status-completed-badge">
                    Завершено
                </div>
                <div v-else-if="application.confirmation === 'Согласовано'" class="status-badge status-approved-badge">
                    Согласовано
                </div>
                <div v-else-if="application.confirmation === 'Согласование'" class="status-badge status-pending-badge">
                    На согласовании
                </div>
            </template>
        </div>

        <!-- Режим просмотра заявок пользователя -->
        <div v-if="mode === 'user'" class="view-buttons">
            <slot name="user-actions"></slot>
        </div>
    </div>
</template>

<script>
import { apiRequest } from '@/api/client'

export default {
    name: 'ApplicationActionBar',
    props: {
        application: {
            type: Object,
            required: true
        },
        currentUserId: {
            type: Number,
            default: null
        },
        responsibleUsers: {
            type: Array,
            default: () => []
        },
        approvers: {
            type: Array,
            default: () => []
        },
        mode: {
            type: String,
            default: 'center'
        },
        processing: {
            type: Boolean,
            default: false
        },
        updatingConfirmation: {
            type: Boolean,
            default: false
        },
        actionComment: {
            type: String,
            default: ''
        }
    },
    emits: ['action-completed', 'processing-change', 'updating-confirmation-change', 'comment-clear'],
    computed: {
        isResponsibleUser() {
            if (!this.currentUserId || !this.responsibleUsers.length) return false;
            return this.responsibleUsers.some(user => user.id === this.currentUserId);
        },

        isApproverUser() {
            if (!this.currentUserId || !this.approvers.length) return false;
            return this.approvers.some(approver => approver.user_id === this.currentUserId);
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
                return { text: 'Вы согласовали', class: 'vote-approved' };
            } else if (currentUser.approval_status === 'rejected') {
                return { text: 'Вы отказали', class: 'vote-rejected' };
            }
            return null;
        },

        canUserApprove() {
            if (!this.responsibleUsers.length) return true;
            const requiredUsers = this.responsibleUsers.filter(user => user.required_approval);
            if (requiredUsers.length === 0) return true;
            const hasRequiredRejected = requiredUsers.some(user => user.approval_status === 'rejected');
            if (hasRequiredRejected && this.application.confirmation === 'Не согласовано') {
                return false;
            }
            return true;
        },

        getApproverStatusMessage() {
            if (this.application.status === 'В работе') return 'Заявка уже в работе';
            if (this.application.status === 'Отказано') return 'Заявка отклонена';
            if (this.application.status === 'Завершено') return 'Заявка завершена';
            if (this.application.confirmation !== 'Согласовано') {
                if (!this.canUserApprove) return 'Ожидание обязательных согласующих';
                return 'Ожидает согласования';
            }
            return 'Готова к принятию';
        }
    },
    methods: {
        async handleCombinedAction(action) {
            this.$emit('processing-change', true);
            try {
                const approvalResponse = await apiRequest(`/applications/${this.application.id}/approve`, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                        user_id: this.currentUserId,
                        status: action === 'accept' ? 'approved' : 'rejected',
                        comment: this.actionComment || null
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
                this.$emit('action-completed', { success: false, message: `Ошибка: ${error.message}` });
            } finally {
                this.$emit('processing-change', false);
            }
        },

        async handleApplicationAction(action) {
            this.$emit('processing-change', true);
            try {
                if (action === 'accept') {
                    await this.acceptApplication();
                } else {
                    await this.rejectApplication();
                }
            } catch (error) {
                console.error(`Ошибка при ${action === 'accept' ? 'принятии' : 'отказе'} заявки:`, error);
            } finally {
                this.$emit('processing-change', false);
            }
        },

        async acceptApplication() {
            const response = await apiRequest(`/applications/${this.application.id}/take-to-work`, {
                method: "POST",
                body: JSON.stringify({
                    user_id: this.currentUserId,
                    action: 'accept',
                    comment: this.actionComment || null
                })
            });

            if (response.ok) {
                this.$emit('comment-clear');
                this.$emit('action-completed', { success: true, message: 'Заявка принята в работу', type: 'success' });
            } else {
                const errorText = await response.text();
                this.$emit('action-completed', { success: false, message: `Ошибка: ${errorText}`, type: 'error' });
            }
        },

        async rejectApplication() {
            const response = await apiRequest(`/applications/${this.application.id}/take-to-work`, {
                method: "POST",
                body: JSON.stringify({
                    user_id: this.currentUserId,
                    action: 'reject',
                    comment: this.actionComment || null
                })
            });

            if (response.ok) {
                this.$emit('comment-clear');
                this.$emit('action-completed', { success: true, message: 'Заявка отклонена', type: 'error' });
            } else {
                const errorText = await response.text();
                this.$emit('action-completed', { success: false, message: `Ошибка: ${errorText}`, type: 'error' });
            }
        },

        async revokeApplication() {
            this.$emit('processing-change', true);
            try {
                const response = await apiRequest(`/applications/${this.application.id}/revoke-from-work`, {
                    method: "POST",
                    body: JSON.stringify({
                        user_id: this.currentUserId,
                        comment: null
                    })
                });

                if (response.ok) {
                    this.$emit('action-completed', { success: true, message: 'Заявка отозвана из работы', type: 'success' });
                } else {
                    const errorText = await response.text();
                    this.$emit('action-completed', { success: false, message: `Ошибка: ${errorText}`, type: 'error' });
                }
            } catch (error) {
                console.error("Ошибка при отзыве заявки:", error);
                this.$emit('action-completed', { success: false, message: 'Ошибка сети при отзыве заявки', type: 'error' });
            } finally {
                this.$emit('processing-change', false);
            }
        },

        async restoreApplication() {
            this.$emit('processing-change', true);
            try {
                const response = await apiRequest(`/applications/${this.application.id}/restore-to-work`, {
                    method: "POST",
                    body: JSON.stringify({
                        user_id: this.currentUserId,
                        comment: this.actionComment || null
                    })
                });

                if (response.ok) {
                    this.$emit('action-completed', { success: true, message: 'Заявка возвращена в работу', type: 'success' });
                } else {
                    const errorText = await response.text();
                    this.$emit('action-completed', { success: false, message: `Ошибка: ${errorText}`, type: 'error' });
                }
            } catch (error) {
                console.error("Ошибка при возврате заявки:", error);
                this.$emit('action-completed', { success: false, message: 'Ошибка сети при возврате заявки', type: 'error' });
            } finally {
                this.$emit('processing-change', false);
            }
        },

        async revokeOwnApproval() {
            if (!confirm('Вы уверены, что хотите отозвать своё решение?')) return;

            this.$emit('processing-change', true);
            try {
                const response = await apiRequest(`/applications/${this.application.id}/revoke-approval`, {
                    method: "POST",
                    body: JSON.stringify({ comment: null })
                });

                if (response.ok) {
                    this.$emit('action-completed', { success: true, message: 'Ваше решение отозвано', type: 'success' });
                } else {
                    const errorText = await response.text();
                    this.$emit('action-completed', { success: false, message: `Ошибка: ${errorText}`, type: 'error' });
                }
            } catch (error) {
                console.error("Ошибка при отзыве решения:", error);
                this.$emit('action-completed', { success: false, message: 'Ошибка сети при отзыве решения', type: 'error' });
            } finally {
                this.$emit('processing-change', false);
            }
        },

        async updateConfirmation(confirmation) {
            if (!this.isResponsibleUser) return;

            this.$emit('updating-confirmation-change', true);
            try {
                if (this.hasUserVoted) {
                    this.$emit('action-completed', { success: false, message: 'Вы уже проголосовали по этой заявке', type: 'error' });
                    return;
                }

                const userApprovalResponse = await apiRequest(`/applications/${this.application.id}/approve`, {
                    method: "POST",
                    body: JSON.stringify({
                        user_id: this.currentUserId,
                        status: confirmation === 'Согласовано' ? 'approved' : 'rejected',
                        comment: this.actionComment || null
                    })
                });

                if (!userApprovalResponse.ok) {
                    const errorText = await userApprovalResponse.text();
                    throw new Error(errorText || "Error updating application confirmation");
                }

                this.$emit('comment-clear');
                this.$emit('action-completed', {
                    success: true,
                    message: confirmation === 'Согласовано' ? 'Заявка согласована' : 'Заявка отклонена',
                    type: confirmation === 'Согласовано' ? 'success' : 'error'
                });
            } catch (error) {
                console.error("Ошибка при обновлении подтверждения:", error);
                this.$emit('action-completed', { success: false, message: `Ошибка: ${error.message}`, type: 'error' });
            } finally {
                this.$emit('updating-confirmation-change', false);
            }
        }
    }
}
</script>

<style scoped>
.action-buttons-wrapper {
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

.status-badge {
    padding: 6px 24px;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 600;
    min-width: 120px;
    text-align: center;
}

.status-in-work-badge {
    background: rgba(79, 91, 223, 0.1);
    color: #4F5BDF;
    border: 1px solid rgba(79, 91, 223, 0.3);
}

.status-rejected-badge {
    background: rgba(220, 38, 38, 0.1);
    color: #dc2626;
    border: 1px solid rgba(220, 38, 38, 0.3);
}

.status-approved-badge {
    background: rgba(5, 150, 105, 0.1);
    color: #059669;
    border: 1px solid rgba(5, 150, 105, 0.3);
}

.status-pending-badge {
    background: rgba(217, 119, 6, 0.1);
    color: #d97706;
    border: 1px solid rgba(217, 119, 6, 0.3);
}

.status-completed-badge {
    background: rgba(5, 150, 105, 0.1);
    color: #059669;
    border: 1px solid rgba(5, 150, 105, 0.3);
}

.confirm-btn:disabled,
.reject-btn:disabled,
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
</style>
