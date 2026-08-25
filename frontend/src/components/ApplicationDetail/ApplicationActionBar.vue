<template>
  <div class="action-bar-root">
    <!-- Действия по раунду дополнения (#1685) отдельным рядом НАД кнопками заявки:
         заявка при этом остаётся в работе, и её собственные действия (отозвать из
         работы, вернуть) должны оставаться доступными. -->
    <transition name="fade">
      <div
        v-if="supplementActionsVisible"
        class="supplement-actions"
        data-testid="supplement-actions"
      >
        <span class="supplement-actions__label">Дополнение №{{ actionableRound.number }}</span>

        <span
          v-if="supplementVoteBadge"
          class="vote-status-badge"
          :class="supplementVoteBadge.class"
          data-testid="supplement-my-vote"
        >
          {{ supplementVoteBadge.text }}
        </span>

        <template v-if="canVoteOnSupplement">
          <button
            class="lk-button lk-button--primary"
            data-testid="supplement-button-approve"
            :disabled="supplementBusy"
            @click="askSupplementAction('approve')"
          >
            Согласовать дополнение
          </button>
          <button
            class="lk-button lk-button--danger"
            data-testid="supplement-button-reject"
            :disabled="supplementBusy"
            @click="askSupplementAction('reject')"
          >
            Отказать в дополнении
          </button>
        </template>

        <button
          v-if="canRevokeSupplementVote"
          class="lk-button lk-button--ghost"
          data-testid="supplement-button-revoke"
          :disabled="supplementBusy"
          @click="askSupplementAction('revoke')"
        >
          Отозвать согласование дополнения
        </button>

        <template v-if="canDecideSupplement">
          <button
            class="lk-button lk-button--primary"
            data-testid="supplement-button-accept"
            :disabled="supplementBusy"
            @click="askSupplementAction('accept')"
          >
            Принять дополнение
          </button>
          <button
            class="lk-button lk-button--danger"
            data-testid="supplement-button-refuse"
            :disabled="supplementBusy"
            @click="askSupplementAction('refuse')"
          >
            Отказать
          </button>
        </template>

        <button
          v-if="canCancelSupplement"
          class="lk-button lk-button--ghost"
          data-testid="supplement-button-cancel"
          :disabled="supplementBusy"
          @click="askSupplementAction('cancel')"
        >
          Отозвать дополнение
        </button>
      </div>
    </transition>

    <!-- Подтверждение решения по раунду. z-index выше стопки карточки заявки
         (оверлей 10002, карточки из заявки 10003-10005), иначе окно откроется под ней.
         Видимостью управляет show, а не v-if по самому запросу: снятие окна родительским
         v-if убивает его анимацию закрытия, поэтому запрос переживает закрытие и
         заменяется только при следующем открытии. -->
    <ConfirmationModal
      v-if="supplementPrompt"
      :show="supplementPromptOpen"
      :title="supplementPrompt.title"
      :message="supplementPrompt.message"
      :confirm-text="supplementPrompt.confirmText"
      cancel-text="Отмена"
      :z-index="10006"
      @confirm="confirmSupplementAction"
      @cancel="closeSupplementPrompt"
    >
      <label class="supplement-comment">
        <span class="supplement-comment__label">Комментарий (необязательно)</span>
        <textarea
          v-model="supplementComment"
          class="lk-textarea supplement-comment__input"
          data-testid="supplement-comment"
          rows="3"
        />
      </label>
    </ConfirmationModal>

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
                <!-- Заявка не в работе, не отказана и не завершена: принять её можно, когда
                     согласование завершено ИЛИ согласовывать нечего (согласующих нет).
                     Своё решение здесь отзывается в любом случае (#1550): пользователь из
                     справочника принимающих - такой же согласующий, и раньше кнопка отзыва
                     ему не доставалась, потому что эта ветка перехватывала рендер. -->
                <template v-else>
                  <template v-if="canTakeToWork">
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
                  <button
                    class="revoke-approval-btn subtle-btn"
                    data-testid="app-detail-button-revoke-approval"
                    :disabled="processing"
                    @click="revokeOwnApproval"
                  >
                    <span
                      v-if="processing"
                      class="button-loading"
                    />
                    <span v-else>{{ revokeApprovalLabel }}</span>
                  </button>
                  <!-- Заявка ждёт других согласующих - показываем свой голос -->
                  <div
                    v-if="!canTakeToWork"
                    class="vote-status-badge"
                    :class="userVoteStatus.class"
                  >
                    {{ userVoteStatus.text }} (ожидание других)
                  </div>
                </template>
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
              <!-- Принять можно при завершённом согласовании либо когда согласующих нет. -->
              <template v-else-if="canTakeToWork">
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
                    data-testid="app-detail-button-revoke-approval"
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
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useUiStore } from '@/stores/ui'
import { useNarrowScreen } from '@/composables/useNarrowScreen'
import ConfirmationModal from '@/components/ConfirmationModal.vue'
import {
    approveSupplement,
    revokeSupplementApproval,
    decideSupplement,
    cancelSupplement
} from '@/api/applications'
import {
    SUPPLEMENT_PENDING,
    SUPPLEMENT_APPROVED,
    SUPPLEMENT_OPEN_STATUSES,
    SUPPLEMENT_REVOCABLE_STATUSES
} from '@/utils/supplementStatuses'

export default {
    name: 'ApplicationActionBar',
    components: { ConfirmationModal },
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
        },
        // Раунды дополнения заявки (#1685), новые сверху - как их отдаёт бэк.
        supplements: {
            type: Array,
            default: () => []
        }
    },
    emits: ['action-completed', 'processing-change', 'updating-confirmation-change', 'comment-clear'],
    setup() {
        return useNarrowScreen();
    },
    data() {
        return {
            // Запрошенное решение по раунду: { action, title, message, confirmText }.
            // Живёт и после закрытия окна - его тексты нужны, пока проигрывается уход.
            supplementPrompt: null,
            supplementPromptOpen: false,
            supplementComment: '',
            // Отдельный от processing флаг: действия самой заявки решением по дополнению
            // не блокируются - заявка остаётся в работе и её кнопки обязаны работать.
            supplementBusy: false
        };
    },
    computed: {
        // На узком экране ряд действий не переносится (nowrap), а у совмещённой роли рядом
        // стоят ещё "Принять" и "Отказать" - полная подпись в 390px не помещается.
        revokeApprovalLabel() {
            return this.isNarrow ? 'Отозвать' : 'Отозвать своё решение';
        },

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
                this.canTakeToWork,
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

        /**
         * Можно ли принимать заявку в работу прямо сейчас. Зеркало серверного гейта
         * (application_workflow_service, action=accept): согласование завершено либо
         * согласующих у заявки нет вовсе - тогда согласовывать нечего.
         *
         * Без второй половины условия заявка от организации, у которой ещё нет
         * привязанных пользователей (а такая организация и заводится из самой заявки),
         * навсегда оставалась в «Согласование»: сервер принять её позволял, а кнопки в
         * интерфейсе не появлялись.
         */
        canTakeToWork() {
            return this.application.confirmation === 'Согласовано' || this.responsibleUsers.length === 0;
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
        },

        /**
         * Раунд дополнения, по которому ещё возможно чьё-то действие (#1685): самый
         * свежий со статусом pending/approved/rejected. Не «открытый» (pending/approved):
         * по отклонённому раунду согласующий ещё может отозвать свой голос и тем открыть
         * его заново - зеркало supplementRevocableStatuses бэка.
         *
         * Раунды приходят по убыванию номера, поэтому берём первый подходящий.
         */
        actionableRound() {
            return this.supplements.find(round => SUPPLEMENT_REVOCABLE_STATUSES.includes(round.status)) || null;
        },

        // Голос текущего пользователя в раунде: null - он не в составе голосующих
        // (снимок ответственных на момент подачи дополнения), а не «ещё не голосовал».
        mySupplementVote() {
            if (!this.currentUserId || !this.actionableRound) return null;
            const approvals = this.actionableRound.approvals || [];
            return approvals.find(a => a.user_id === this.currentUserId) || null;
        },

        mySupplementVoteStatus() {
            return this.mySupplementVote ? (this.mySupplementVote.approval_status || 'pending') : null;
        },

        isSupplementAuthor() {
            return !!this.currentUserId && this.application.sender_user_id === this.currentUserId;
        },

        // Голосовать можно только по идущему кругу и только один раз - повторно кнопки
        // не предлагаем, вместо них показываем бейдж голоса и отзыв.
        canVoteOnSupplement() {
            return !!this.actionableRound
                && this.actionableRound.status === SUPPLEMENT_PENDING
                && this.mySupplementVoteStatus === 'pending';
        },

        canRevokeSupplementVote() {
            return !!this.mySupplementVoteStatus && this.mySupplementVoteStatus !== 'pending';
        },

        /**
         * Решение принимающего по согласованному раунду. Статус заявки здесь не
         * проверяем: принять нельзя только пока она не в работе, и это ловит серверный
         * гард - между рендером и кликом её могли вывести из работы, а прятать «Отказать»
         * из-за этого неверно (отказ по выведенной заявке остаётся законным).
         */
        canDecideSupplement() {
            return this.isApproverUser
                && !!this.actionableRound
                && this.actionableRound.status === SUPPLEMENT_APPROVED;
        },

        // Снять раунд автор может, пока по нему не принято решение (pending/approved).
        // Супер-админу сервер это тоже разрешает, но компонент про роли из стора не знает
        // (права приходят снаружи, как и у соседних кнопок) - пробел общий для файла.
        canCancelSupplement() {
            return this.isSupplementAuthor
                && !!this.actionableRound
                && SUPPLEMENT_OPEN_STATUSES.includes(this.actionableRound.status);
        },

        supplementVoteBadge() {
            if (this.mySupplementVoteStatus === 'approved') {
                return { text: 'Вы согласовали дополнение', class: 'vote-approved' };
            }
            if (this.mySupplementVoteStatus === 'rejected') {
                return { text: 'Вы отказали в дополнении', class: 'vote-rejected' };
            }
            return null;
        },

        // Ряд рисуем, только когда пользователю в нём есть что сделать или что увидеть про
        // себя: посторонним участникам раунд показывает панель дополнений, а не кнопки.
        supplementActionsVisible() {
            if (!this.actionableRound) return false;
            return this.canVoteOnSupplement || this.canRevokeSupplementVote
                || this.canDecideSupplement || this.canCancelSupplement;
        }
    },
    methods: {
        // Тексты подтверждений по действиям над раундом. Отдельной таблицей, чтобы
        // разметка не обрастала ветвлением, а формулировки лежали рядом друг с другом.
        supplementPromptFor(action, number) {
            const prompts = {
                approve: {
                    title: 'Согласовать дополнение?',
                    message: `Дополнение №${number} будет согласовано. Добавленные строки встанут на пост после решения принимающего.`,
                    confirmText: 'Согласовать'
                },
                reject: {
                    title: 'Отказать в дополнении?',
                    message: `Дополнение №${number} будет отклонено, добавленные строки на пост не попадут.`,
                    confirmText: 'Отказать'
                },
                revoke: {
                    title: 'Отозвать согласование дополнения?',
                    message: `Ваш голос по дополнению №${number} вернётся в ожидание.`,
                    confirmText: 'Отозвать'
                },
                accept: {
                    title: 'Принять дополнение?',
                    message: `Строки дополнения №${number} встанут на пост и станут видны охране.`,
                    confirmText: 'Принять'
                },
                refuse: {
                    title: 'Отказать в дополнении?',
                    message: `Дополнение №${number} будет отклонено, его строки на пост не встанут.`,
                    confirmText: 'Отказать'
                },
                cancel: {
                    title: 'Отозвать дополнение?',
                    message: `Дополнение №${number} будет снято, добавленные строки на пост не попадут.`,
                    confirmText: 'Отозвать'
                }
            };
            return prompts[action] || null;
        },

        askSupplementAction(action) {
            const round = this.actionableRound;
            if (!round) return;
            const prompt = this.supplementPromptFor(action, round.number);
            if (!prompt) return;
            // Комментарий и тексты сбрасываем на открытии, а не на закрытии: иначе они
            // обнулятся прямо на глазах, пока окно уходит.
            this.supplementComment = '';
            this.supplementPrompt = { action, ...prompt };
            this.supplementPromptOpen = true;
        },

        closeSupplementPrompt() {
            this.supplementPromptOpen = false;
        },

        // Вызов бэка и текст успеха по действию. Номер раунда берём из ответа: он же
        // подтверждает, по какому именно раунду решение записано.
        async runSupplementAction(action, applicationId, supplementId, comment) {
            if (action === 'approve' || action === 'reject') {
                const res = await approveSupplement(applicationId, supplementId, {
                    status: action === 'approve' ? 'approved' : 'rejected',
                    comment
                });
                return {
                    message: action === 'approve'
                        ? `Дополнение №${res.number} согласовано`
                        : `В дополнении №${res.number} отказано`,
                    type: action === 'approve' ? 'success' : 'error'
                };
            }
            if (action === 'revoke') {
                const res = await revokeSupplementApproval(applicationId, supplementId, { comment });
                return { message: `Согласование дополнения №${res.number} отозвано`, type: 'success' };
            }
            if (action === 'accept' || action === 'refuse') {
                const res = await decideSupplement(applicationId, supplementId, {
                    action: action === 'accept' ? 'accept' : 'reject',
                    comment
                });
                return {
                    message: action === 'accept'
                        ? `Дополнение №${res.number} принято, строк добавлено на пост: ${res.activated}`
                        : `В дополнении №${res.number} отказано, его строки на пост не встанут`,
                    type: action === 'accept' ? 'success' : 'error'
                };
            }
            // Явная ветка, а не хвост: седьмое действие с опечаткой иначе молча снимало бы раунд.
            if (action !== 'cancel') {
                throw new Error(`Неизвестное действие по дополнению: ${action}`);
            }
            const res = await cancelSupplement(applicationId, supplementId, { comment });
            return { message: `Дополнение №${res.number} отозвано`, type: 'success' };
        },

        async confirmSupplementAction() {
            const prompt = this.supplementPrompt;
            const round = this.actionableRound;
            if (!prompt || !this.supplementPromptOpen || !round || this.supplementBusy) return;

            const comment = this.supplementComment.trim() || null;
            this.supplementBusy = true;
            try {
                const { message, type } = await this.runSupplementAction(
                    prompt.action, this.application.id, round.id, comment);
                this.closeSupplementPrompt();
                this.$emit('action-completed', { success: true, message, type });
            } catch (error) {
                // Сообщение бэка человеческое (409 «голосование закрыто», 403 «решение
                // принимает только принимающий») - показываем его как есть, окно
                // оставляем открытым: комментарий не должен пропасть перед повтором.
                this.$emit('action-completed', {
                    success: false,
                    message: error.message || 'Не удалось выполнить действие по дополнению',
                    type: 'error'
                });
            } finally {
                this.supplementBusy = false;
            }
        },

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
/* Корень бара: ряд дополнения над рядом кнопок заявки. Колонка с выравниванием по
   правому краю - шапка детали складывает элементы в строку и прижимает их вправо,
   поэтому без flex-end ряды разъехались бы по левому краю. */
.action-bar-root {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 8px;
}

.supplement-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    flex-wrap: wrap;
    /* Без ограничения ширины переносить ряду не от чего: родитель - колонка, и ряд
       просто растёт за её правый край. На узких экранах это скрыто (там свой блок
       ниже), на широких места хватает, а между ними, около 780, кнопка уезжала
       за границу окна и обрезалась - замерено в браузере, правый край 788 при окне 780. */
    max-width: 100%;
}

.supplement-actions__label {
    font-size: 13px;
    font-weight: 600;
    color: var(--accent-text);
    white-space: nowrap;
}

.supplement-comment {
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.supplement-comment__label {
    font-size: 12px;
    color: var(--text-muted);
}

.supplement-comment__input {
    min-height: 64px;
}

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

    /* Ряд дополнения из nowrap-списка выше исключён намеренно: подписи «Согласовать
       дополнение» и «Отказать в дополнении» в 390px рядом не помещаются, им нужен
       перенос.
       Прижим влево - потому что мобильная шапка детали стоит на justify-content:
       flex-start: при десктопном flex-end широкий ряд дополнения растягивал корень, и
       ряд кнопок самой заявки уезжал к правому краю (замер: x=134 при остальном на 0). */
    .action-bar-root {
        align-items: flex-start;
    }

    .supplement-actions {
        align-self: stretch;
        justify-content: flex-start;
    }
    .confirm-btn,
    .reject-btn,
    .accept-btn {
        padding: 8px 14px;
        white-space: nowrap;
    }
    /* Отзыв решения стоит в ряду с "Принять"/"Отказать" (#1550) - без своей ширины,
       иначе тройка кнопок вылезает за 390. Селектор с двумя классами обязателен:
       .subtle-btn объявлен ниже по файлу и при равной специфичности перебивал
       min-width обратно на 140px. */
    .subtle-btn.revoke-approval-btn {
        padding: 8px 14px;
        min-width: auto;
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
    transition: background-color 0.2s ease, border-color 0.2s ease, color 0.2s ease, opacity 0.2s ease;
    min-width: 120px;
    border: 1px solid var(--border);
    position: relative;
    overflow: hidden;
}

.confirm-btn, .accept-btn {
    background: var(--success);
    color: var(--fill-text);
}

/* Приём .reject-btn: подмешанный цвет текста темнит фон в светлой теме и светлит
   в тёмной. Раньше hover повторял базовый var(--success) - отклика не было. */
.confirm-btn:hover:not(:disabled), .accept-btn:hover:not(:disabled) {
    background: color-mix(in srgb, var(--success) 85%, var(--text));
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
