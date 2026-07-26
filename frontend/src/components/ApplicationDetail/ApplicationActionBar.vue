<template>
  <div class="action-buttons-wrapper">
    <!-- Режим центра заявок -->
    <div
      v-if="mode === 'center'"
      class="action-buttons"
    >
      <!-- Набор кнопок/статусов cross-fade при смене статуса заявки (out-in по barKey).
           .action-buttons держит min-height, чтобы высота не скакала при свапе (#1097 R4-7). -->
      <transition
        name="fade"
        mode="out-in"
      >
        <div
          :key="barKey"
          class="action-buttons-track"
        >
          <!-- Для пользователей, которые одновременно являются принимающими и ответственными -->
          <template v-if="!busy && isApproverUser && isResponsibleUser && application.status !== 'Отозвана'">
            <!-- Если пользователь еще не голосовал -->
            <template v-if="!hasUserVoted">
              <!-- Показываем кнопки согласования, если заявка не отклонена окончательно и не завершена -->
              <template v-if="application.confirmation !== 'Не согласовано' && application.status !== 'Завершено'">
                <button
                  class="accept-btn"
                  data-testid="app-detail-button-approve"
                  :disabled="processing || approvalBlockedByBlacklist"
                  :title="approvalBlockedByBlacklist ? blacklistGateHint : null"
                  @click="handleCombinedAction('accept')"
                >
                  <span
                    v-if="processing"
                    class="button-loading"
                  />
                  <span v-else>{{ approvingCompletesConfirmation ? 'Согласовать и принять' : 'Согласовать' }}</span>
                </button>
                <button
                  class="reject-btn"
                  data-testid="app-detail-button-reject"
                  :disabled="processing"
                  @click="handleCombinedAction('reject')"
                >
                  <span
                    v-if="processing"
                    class="button-loading"
                  />
                  <span v-else>Отказать</span>
                </button>
              </template>
              <!-- Если заявка завершена -->
              <div
                v-else-if="application.status === 'Завершено'"
                class="status-badge status-completed-badge"
              >
                Завершено
              </div>
              <!-- Если заявка отклонена окончательно -->
              <div
                v-else
                class="info-badge"
              >
                Заявка отклонена
              </div>
            </template>

            <!-- Если пользователь уже проголосовал -->
            <template v-else>
              <!-- Если заявка в работе - показываем статус и кнопку отзыва -->
              <template v-if="application.status === 'В работе'">
                <button
                  class="subtle-btn"
                  :disabled="processing"
                  @click="revokeApplication"
                >
                  <span
                    v-if="processing"
                    class="button-loading"
                  />
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
                  :disabled="processing"
                  @click="restoreApplication"
                >
                  <span
                    v-if="processing"
                    class="button-loading"
                  />
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
                  :disabled="processing"
                  @click="handleApplicationAction('accept')"
                >
                  <span
                    v-if="processing"
                    class="button-loading"
                  />
                  <span v-else>Принять</span>
                </button>
                <button
                  class="reject-btn"
                  data-testid="app-detail-button-reject"
                  :disabled="processing"
                  @click="handleApplicationAction('reject')"
                >
                  <span
                    v-if="processing"
                    class="button-loading"
                  />
                  <span v-else>Отказать</span>
                </button>
              </template>
              <!-- Если пользователь проголосовал, но заявка не согласована (ждет других) -->
              <div
                v-else
                class="vote-status-badge"
                :class="userVoteStatus.class"
              >
                {{ userVoteStatus.text }} (ожидание других)
              </div>
            </template>
          </template>

          <!-- Для принимающих заявки (не ответственных) -->
          <template v-else-if="!busy && isApproverUser && application.status !== 'Отозвана'">
            <!-- Если заявка в работе - показываем статус и кнопку отзыва -->
            <template v-if="application.status === 'В работе'">
              <button
                class="subtle-btn"
                :disabled="processing"
                @click="revokeApplication"
              >
                <span
                  v-if="processing"
                  class="button-loading"
                />
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
                :disabled="processing"
                @click="restoreApplication"
              >
                <span
                  v-if="processing"
                  class="button-loading"
                />
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
                :disabled="processing"
                @click="handleApplicationAction('accept')"
              >
                <span
                  v-if="processing"
                  class="button-loading"
                />
                <span v-else>Принять</span>
              </button>
              <button
                class="reject-btn"
                data-testid="app-detail-button-reject"
                :disabled="processing"
                @click="handleApplicationAction('reject')"
              >
                <span
                  v-if="processing"
                  class="button-loading"
                />
                <span v-else>Отказать</span>
              </button>
            </template>
            <!-- Если заявка не согласована - показываем информационное сообщение -->
            <div
              v-else
              class="info-badge"
            >
              {{ getApproverStatusMessage }}
            </div>
          </template>

          <!-- Для ответственных за согласование (не принимающих) -->
          <template v-else-if="!busy && isResponsibleUser && application.status !== 'Отозвана'">
            <!-- Если пользователь еще не голосовал -->
            <template v-if="!hasUserVoted">
              <!-- Показываем кнопки согласования, когда заявка не отклонена и не завершена -->
              <template v-if="application.confirmation !== 'Не согласовано' && application.status !== 'Завершено'">
                <button
                  class="confirm-btn"
                  data-testid="app-detail-button-approve"
                  :disabled="updatingConfirmation || processing || approvalBlockedByBlacklist"
                  :title="approvalBlockedByBlacklist ? blacklistGateHint : null"
                  @click="updateConfirmation('Согласовано')"
                >
                  <span
                    v-if="updatingConfirmation"
                    class="button-loading"
                  />
                  <span v-else>Согласовать</span>
                </button>
                <button
                  class="reject-btn"
                  data-testid="app-detail-button-reject"
                  :disabled="updatingConfirmation || processing"
                  @click="updateConfirmation('Не согласовано')"
                >
                  <span
                    v-if="updatingConfirmation"
                    class="button-loading"
                  />
                  <span v-else>Отказать</span>
                </button>
              </template>
              <!-- Если заявка завершена -->
              <div
                v-else-if="application.status === 'Завершено'"
                class="status-badge status-completed-badge"
              >
                Завершено
              </div>
              <!-- Если заявка отклонена окончательно -->
              <div
                v-else
                class="info-badge"
              >
                Заявка отклонена
              </div>
            </template>

            <!-- Если пользователь уже проголосовал -->
            <template v-else>
              <!-- Если заявка в работе - показываем только статус (нельзя отозвать) -->
              <template v-if="application.status === 'В работе'">
                <div
                  class="vote-status-badge"
                  :class="userVoteStatus.class"
                >
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
                  :disabled="processing"
                  @click="revokeOwnApproval"
                >
                  <span
                    v-if="processing"
                    class="button-loading"
                  />
                  <span v-else>Отозвать своё решение</span>
                </button>
                <div
                  class="vote-status-badge"
                  :class="userVoteStatus.class"
                >
                  {{ userVoteStatus.text }}
                </div>
              </template>
            </template>
          </template>

          <!-- Действие/рефетч (busy): единый лоадер в зарезервированной высоте вместо старых
           кнопок - не мигаем и не скачем высотой до приезда нового статуса (#1097 R4-7). -->
          <template v-else-if="busy">
            <span class="button-loading actions-ready-loader" />
          </template>

          <!-- Для остальных пользователей - только информация -->
          <template v-else>
            <div
              v-if="application.status === 'Отозвана'"
              class="status-badge status-rejected-badge"
            >
              Отозвана инициатором
            </div>
            <div
              v-else-if="application.status === 'В работе'"
              class="status-badge status-in-work-badge"
            >
              В работе
            </div>
            <div
              v-else-if="application.status === 'Отказано'"
              class="status-badge status-rejected-badge"
            >
              Отказано
            </div>
            <div
              v-else-if="application.status === 'Завершено'"
              class="status-badge status-completed-badge"
            >
              Завершено
            </div>
            <div
              v-else-if="application.confirmation === 'Согласовано'"
              class="status-badge status-approved-badge"
            >
              Согласовано
            </div>
            <div
              v-else-if="application.confirmation === 'Согласование'"
              class="status-badge status-pending-badge"
            >
              На согласовании
            </div>
          </template>

          <!-- Гейт ЧС (#481): подсказка, почему согласование заблокировано -->
          <div
            v-if="approvalBlockedByBlacklist"
            class="blacklist-gate-hint"
            data-testid="app-detail-blacklist-gate-hint"
          >
            Подтвердите пропуск по помеченным
          </div>
        </div>
      </transition>
    </div>

    <!-- Режим просмотра заявок пользователя -->
    <div
      v-if="mode === 'user'"
      class="view-buttons"
    >
      <slot name="user-actions" />
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useUiStore } from '@/stores/ui'

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
        },
        hasUnoverriddenBlacklistFlags: {
            type: Boolean,
            default: false
        },
        // П.46: пока false - показываем лоадер вместо кнопок (роли/голоса ещё грузятся)
        ready: {
            type: Boolean,
            default: true
        }
    },
    emits: ['action-completed', 'processing-change', 'updating-confirmation-change', 'comment-clear'],
    computed: {
        // Идёт действие (accept/reject/take-to-work), смена согласования ИЛИ рефетч после
        // действия (ready=false). Всё это время держим единый лоадер вместо кнопок в
        // зарезервированной высоте (.action-buttons min-height), чтобы старые кнопки не
        // висели до приезда нового статуса и высота не скакала (#1097 R4-7). На ошибке
        // reload не идёт, флаги спадают -> busy=false -> кнопки возвращаются для повтора.
        busy() {
            return this.processing || this.updatingConfirmation || !this.ready;
        },

        // Идентификатор текущего набора кнопок/статуса: меняется ровно когда меняется
        // отображаемая ветка (busy / роль / статус / согласование / голос / доступность
        // согласования) - это и триггерит cross-fade. canUserApprove включён, т.к. от него
        // зависит текст getApproverStatusMessage (ветка принимающего). Флаги ЧС в ключ НЕ
        // включаем: они меняют лишь disabled и подсказку внутри той же ветки.
        barKey() {
            if (this.busy) return 'busy';
            return [
                this.isApproverUser,
                this.isResponsibleUser,
                this.hasUserVoted,
                this.canUserApprove,
                this.application.status,
                this.application.confirmation
            ].join('|');
        },

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

        // Завершит ли голос текущего пользователя согласование целиком (заявка станет
        // "Согласовано" -> её можно сразу принять в работу). Зеркало бэкенд-логики
        // updateConfirmationBasedOnApprovals: все обязательные approved / при отсутствии
        // обязательных - хотя бы один approved; заявка без согласующих - принять можно.
        // По этому решаем: комбо-кнопка "Согласовать и принять" vs просто "Согласовать".
        approvingCompletesConfirmation() {
            const users = this.responsibleUsers.map(u =>
                u.id === this.currentUserId ? { ...u, approval_status: 'approved' } : u);
            if (users.length === 0) return true;
            const required = users.filter(u => u.required_approval);
            if (required.some(u => u.approval_status === 'rejected')) return false;
            if (required.length > 0) return required.every(u => u.approval_status === 'approved');
            const nonRequired = users.filter(u => !u.required_approval);
            if (nonRequired.length > 0) {
                return nonRequired.some(u => u.approval_status === 'approved')
                    && !nonRequired.some(u => u.approval_status === 'rejected');
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
        },

        // Кнопка "Согласовать"/"Согласовать и принять" видна только ответственному,
        // который ещё не голосовал, по не отклонённой и не завершённой заявке.
        showsApproveButton() {
            if (!this.isResponsibleUser || this.hasUserVoted) return false;
            return this.application.confirmation !== 'Не согласовано' && this.application.status !== 'Завершено';
        },

        // Гейт ЧС (#481): согласование заблокировано в UI, пока есть непереопределённые
        // флаги. Зеркало бэкенд-гейта (409) - кнопка disabled + подсказка.
        approvalBlockedByBlacklist() {
            return this.hasUnoverriddenBlacklistFlags && this.showsApproveButton;
        },

        blacklistGateHint() {
            return 'Подтвердите пропуск по всем помеченным элементам, чтобы согласовать заявку';
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
                    // Принять в работу можно, только если голос текущего пользователя реально
                    // завершает согласование; иначе только согласовываем (бэк всё равно отобьёт
                    // accept до завершения согласования - барьер там authoritative).
                    if (this.approvingCompletesConfirmation) {
                        await this.acceptApplication();
                    } else {
                        this.$emit('action-completed', { success: true, message: 'Ваш голос учтён. Заявка ожидает согласования остальных обязательных согласующих.', type: 'success' });
                    }
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
            const ok = await useUiStore().confirm({
                title: 'Отозвать решение?',
                message: 'Ваше решение по согласованию этой заявки будет отозвано.',
                confirmText: 'Отозвать',
                cancelText: 'Отмена',
                danger: true
            });
            if (!ok) return;

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
    align-items: center;
    /* Резерв высоты ряда кнопок: лоадер (16px) и бейджи занимают ту же высоту, что и
       кнопки (~35px), поэтому при переходе кнопки -> лоадер -> новый статус (cross-fade
       out-in по barKey) высота не скачет (#1097 R4-7). Пустой промежуток между leave и
       enter тоже держит min-height. Лоадер/трек центрируются по align-items. */
    min-height: 36px;
}

/* Реальный flex-контейнер набора кнопок (keyed-ребёнок <transition>): его подмена
   даёт cross-fade, а внешний .action-buttons при этом не теряет высоту. */
.action-buttons-track {
    display: flex;
    gap: 5px;
    align-items: center;
    flex-wrap: wrap;
}

/* Cross-fade набора кнопок при смене статуса (эталон ApplicationHistory.vue). */
.fade-enter-active,
.fade-leave-active {
    transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
    opacity: 0;
    transform: translateY(-10px);
}

.view-buttons {
    display: flex;
    gap: 10px;
}

/* Мобилка: кнопки/статусы согласования в ОДНУ строку, не переносить (detail 2).
   nowrap + компактный padding, чтобы Согласовать+Отказать влезали в шапку детали. */
@media (max-width: 768px) {
    .action-buttons-wrapper,
    .action-buttons,
    .action-buttons-track,
    .view-buttons {
        flex-wrap: nowrap;
    }
    .confirm-btn,
    .reject-btn,
    .accept-btn {
        padding: 8px 14px;
        white-space: nowrap;
    }
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
    border: 1px solid var(--border);
    position: relative;
    overflow: hidden;
}

.confirm-btn, .accept-btn {
    background: var(--success);
    color: var(--fill-text);
}

.confirm-btn:hover:not(:disabled), .accept-btn:hover:not(:disabled) {
    background: var(--success);
}

.reject-btn {
    background: var(--danger);
    color: var(--fill-text);
}

.reject-btn:hover:not(:disabled) {
    background: color-mix(in srgb, var(--danger) 85%, var(--text));
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
    color: var(--text-muted);
    border: 1px solid var(--border);
}

.subtle-btn:hover:not(:disabled) {
    background: var(--surface-2);
    color: var(--text-muted);
}

.subtle-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.revoke-approval-btn {
    border-color: var(--warning);
    color: var(--warning-text);
}

.revoke-approval-btn:hover:not(:disabled) {
    background: var(--warning-bg);
    border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
    color: var(--warning-text);
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
    background: color-mix(in srgb, var(--success) 10%, var(--surface));
    color: var(--success-text);
    border-color: rgba(9, 136, 0, 0.3);
}

.vote-status-badge.vote-rejected {
    background: color-mix(in srgb, var(--danger) 10%, var(--surface));
    color: var(--danger-text);
    border-color: rgba(255, 102, 104, 0.3);
}

.info-badge {
    padding: 6px 16px;
    border-radius: 50px;
    font-size: 14px;
    font-weight: 500;
    min-width: 200px;
    text-align: center;
    background: var(--border);
    color: var(--text-muted);
    border: 1px solid var(--border);
}

.blacklist-gate-hint {
    padding: 6px 14px;
    border-radius: 50px;
    font-size: 13px;
    font-weight: 500;
    text-align: center;
    max-width: 240px;
    background: color-mix(in srgb, var(--warning) 10%, var(--surface));
    color: var(--warning-text);
    border: 1px solid rgba(245, 158, 11, 0.35);
    cursor: help;
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
    background: color-mix(in srgb, var(--accent) 10%, var(--surface));
    color: var(--accent-text);
    border: 1px solid rgba(79, 91, 223, 0.3);
}

.status-rejected-badge {
    background: color-mix(in srgb, var(--danger) 10%, var(--surface));
    color: var(--danger-text);
    border: 1px solid rgba(220, 38, 38, 0.3);
}

.status-approved-badge {
    background: color-mix(in srgb, var(--success) 10%, var(--surface));
    color: var(--success-text);
    border: 1px solid color-mix(in srgb, var(--success) 45%, transparent);
}

.status-pending-badge {
    background: color-mix(in srgb, var(--warning) 10%, var(--surface));
    color: var(--warning-text);
    border: 1px solid rgba(217, 119, 6, 0.3);
}

.status-completed-badge {
    background: color-mix(in srgb, var(--success) 10%, var(--surface));
    color: var(--success-text);
    border: 1px solid color-mix(in srgb, var(--success) 45%, transparent);
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
    border-top-color: var(--surface);
    animation: spin 0.8s linear infinite;
}

/* П.46: лоадер готовности кнопок стоит на светлом фоне (не внутри цветной кнопки),
   поэтому белый спиннер не виден - перекрашиваем в серый + primary. */
.actions-ready-loader {
    border-color: var(--border);
    border-top-color: var(--accent-text);
}

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}
</style>
