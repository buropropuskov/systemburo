<template>
  <div class="confirmation-section">
    <div class="confirmation-header">
      <h4>Согласование заявки</h4>
      <div
        v-if="updatingConfirmation"
        class="confirmation-loading"
      >
        <LoaderSpinner
          size="small"
          :label="''"
        />
      </div>
    </div>
        
    <div class="confirmation-info">
      <!-- Статус заявки -->
      <div class="info-row">
        <span class="info-label">Статус:</span>
        <span
          class="confirmation-badge"
          :class="getConfirmationClass(application.confirmation)"
        >
          {{ application.confirmation }}
        </span>
      </div>
    </div>

    <!-- Ответственные пользователи -->
    <div
      v-if="sortedResponsibleUsers.length > 0"
      class="responsible-users-section"
    >
      <h5>Ответственные за согласование ({{ sortedResponsibleUsers.length }}):</h5>
      <div class="users-list">
        <!-- Строка согласующего открывает его карточку (#1952). Роль и клавиатура
             заданы руками: <button> сюда не годится - внутри блочная разметка с
             бейджами и комментарием, которую кнопке иметь нельзя. -->
        <div
          v-for="user in sortedResponsibleUsers"
          :key="user.id"
          class="user-item"
          data-testid="app-confirmation-user"
          role="button"
          tabindex="0"
          :aria-label="`Открыть карточку: ${getUserDisplayName(user)}`"
          @click="$emit('select-user', user)"
          @keydown.enter.prevent="$emit('select-user', user)"
          @keydown.space.prevent="$emit('select-user', user)"
        >
          <!-- ФИО -->
          <div class="user-name-block">
            {{ getUserDisplayName(user) }}
          </div>
                    
          <!-- Должность -->
          <div
            v-if="user.position"
            class="user-position-block"
          >
            {{ user.position }}
          </div>
                    
          <!-- Ряд с бейджем и статусом -->
          <div class="user-badge-status-row">
            <div
              v-if="user.required_approval"
              class="required-badge-container"
            >
              <span class="badge required-badge">Обязательно</span>
              <div class="required-tooltip">
                Согласование этого пользователя является обязательным. Без него принять заявку в работу будет невозможно.
              </div>
            </div>
            <span
              class="status-badge"
              :class="getStatusClass(user.approval_status)"
            >
              {{ getStatusText(user.approval_status) }}
            </span>
          </div>

          <!-- Молчащий согласующий: "не отвечает N дней, напомнили K раз" (#1315 S3) -->
          <div
            v-if="silenceLabel(user)"
            class="user-silence-block"
          >
            {{ silenceLabel(user) }}
          </div>

          <!-- Комментарий пользователя (только если есть) -->
          <div
            v-if="user.approval_comment"
            class="user-comment-block"
          >
            <span class="comment-label">Комментарий:</span>
            <span class="comment-text">{{ user.approval_comment }}</span>
          </div>
                    
          <!-- Время -->
          <div
            v-if="user.approval_datetime"
            class="user-time-block"
          >
            Время: {{ formatDateTime(user.approval_datetime) }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue'
import { isAwaitingApproval, approverSilenceDays, approverSilenceLabel } from '@/utils/pendingApproval'
import { useApprovalStatus } from '@/composables/useApprovalStatus'

export default {
    name: 'ApplicationConfirmation',
    components: { LoaderSpinner },
    props: {
        application: {
            type: Object,
            required: true
        },
        responsibleUsers: {
            type: Array,
            required: true
        },
        currentUserId: {
            type: Number,
            default: null
        },
        updatingConfirmation: {
            type: Boolean,
            default: false
        }
    },
    // select-user - клик по согласующему. Карточку открывает родитель: контакты и
    // роли лежат в ответе /applications/:id/participants, которого у этого блока нет.
    emits: ['select-user'],
    // Словарь голосов согласующих общий с панелью раундов дополнения (#1685) -
    // getStatusText/getStatusClass приходят оттуда под теми же именами, что были методами.
    setup() {
        return useApprovalStatus();
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
        },
        hasRequiredApprover() {
            return (this.responsibleUsers || []).some(user => user.required_approval === true);
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

        // Метка "не отвечает N дней, напомнили K раз" для молчащего согласующего
        // (#1315 S3). Показываем только тому, чей голос ещё нужен - зеркало предиката
        // "кому напоминаем" из ReminderService.selectCandidates: есть обязательные ->
        // только им (голос необязательного на исход не влияет), нет -> всем pending.
        // Иначе пометили бы "не отвечает" необязательного, чьё молчание ничего не решает.
        silenceLabel(user) {
            if (!isAwaitingApproval(this.application)) return '';
            const status = user.approval_status || 'pending';
            if (status !== 'pending') return '';
            if (this.hasRequiredApprover && !user.required_approval) return '';
            const days = approverSilenceDays(user);
            if (days === null) return '';
            return approverSilenceLabel(days, user.reminder_count || 0);
        }
    }
}
</script>

<style scoped>
.confirmation-section {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    padding: 15px;
    margin-bottom: 10px;
    box-shadow: 0 2px 12px var(--shadow-drop);
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
    color: var(--accent-text);
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
    border: 3px solid var(--surface-2);
    border-top: 3px solid var(--accent);
    border-radius: 50%;
    animation: spin 1s linear infinite;
}

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}

.confirmation-info {
    margin-bottom: 20px;
}

/* Список согласующих скрыт, когда их нет: тогда последним остаётся блок выше, и его
   margin-bottom складывался с padding секции в пустоту снизу (#1587). Зазоры между
   блоками при этом сохраняются - обнуляется только последний. */
.confirmation-section > *:last-child {
    margin-bottom: 0;
}

.info-row {
    display: flex;
    justify-content: space-between;
    padding: 6px 0;
}

.info-label {
    color: var(--text-muted);
    font-size: 14px;
    font-weight: 400;
    min-width: 120px;
}

.confirmation-badge {
    padding: 4px 12px;
    border-radius: 20px;
    font-size: 12px;
    font-weight: 500;
    display: inline-block;
    border: 1px solid;
    transition: all 0.3s ease;
}

.confirmation-badge.confirmation-approved {
    background-color: var(--success-bg);
    color: var(--success-text);
    border-color: color-mix(in srgb, var(--success) 30%, var(--surface));
}

.confirmation-badge.confirmation-pending {
    background-color: var(--warning-bg);
    color: var(--warning-text);
    border-color: color-mix(in srgb, var(--warning) 30%, var(--surface));
}

.confirmation-badge.confirmation-rejected {
    background-color: var(--danger-bg);
    color: var(--danger-text);
    border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.responsible-users-section h5 {
    font-size: 14px;
    color: var(--text-muted);
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
    background: var(--surface-2);
    border-radius: 15px;
    border: 1px solid var(--border);
    transition: all 0.2s ease;
    gap: 3px;
    position: relative;
    cursor: pointer;
}

.user-item:hover {
    border-color: var(--accent);
    background: var(--accent-tint);
}

.user-item:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
}

.user-name-block {
    font-weight: 600;
    color: var(--text);
    font-size: 14px;
    line-height: 1.4;
}

.user-position-block {
    color: var(--text-muted);
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
    flex-wrap: wrap;
}

/* Контейнер для бейджа и тултипа */
.required-badge-container {
    position: relative;
    display: inline-block;
}

.required-badge {
    background: var(--accent);
    color: var(--accent-contrast);
    padding: 3px 10px;
    border-radius: 20px;
    font-size: 11px;
    font-weight: 600;
    display: inline-block;
    white-space: nowrap;
    cursor: help;
}

/* Тултип */
.required-tooltip {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    background: var(--hint-bg);
    color: var(--hint-text);
    font-size: 10px;
    line-height: 1.4;
    padding: 6px 10px;
    border-radius: 8px;
    pointer-events: none;
    opacity: 0;
    visibility: hidden;
    transition: opacity 0.15s ease;
    z-index: 10;
    box-shadow: 0 2px 8px var(--shadow-drop);
    
    width: 250px;
    font-weight: 400;
}

/* Маленькая стрелочка сверху тултипа */
.required-tooltip::before {
    content: '';
    position: absolute;
    top: -4px;
    left: 12px;
    width: 8px;
    height: 8px;
    background: var(--hint-bg);
    transform: rotate(45deg);
    border-radius: 2px;
}

/* Показываем тултип при наведении на контейнер */
.required-badge-container:hover .required-tooltip {
    opacity: 1;
    visibility: visible;
}

.user-silence-block {
    font-size: 11px;
    font-weight: 500;
    color: var(--warning-text);
    background: var(--warning-bg);
    border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
    padding: 4px 10px;
    border-radius: 8px;
    align-self: flex-start;
    margin-top: 4px;
}

.user-comment-block {
    font-size: 11px;
    color: var(--text-muted);
    background: var(--accent-tint);
    padding: 6px 10px;
    border-radius: 10px;
    margin-top: 4px;
    border-left: 3px solid var(--accent);
}

.user-time-block {
    font-size: 11px;
    color: var(--text-muted);
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
    background-color: var(--success-bg);
    color: var(--success-text);
    border: 1px solid color-mix(in srgb, var(--success) 30%, var(--surface));
}

.status-rejected {
    background-color: var(--danger-bg);
    color: var(--danger-text);
    border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.status-pending {
    background-color: var(--warning-bg);
    color: var(--warning-text);
    border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
}

.status-default {
    background-color: var(--border);
    color: var(--text-muted);
    border: 1px solid var(--border);
}

.badge {
    padding: 3px 10px;
    border-radius: 20px;
    font-size: 11px;
    font-weight: 600;
    display: inline-block;
    white-space: nowrap;
}

.comment-label {
    color: var(--text-muted);
    font-size: 11px;
    margin-right: 4px;
}

.comment-text {
    color: var(--text);
    font-size: 12px;
}
</style>