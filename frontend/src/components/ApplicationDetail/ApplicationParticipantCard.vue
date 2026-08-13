<!-- ApplicationParticipantCard.vue -->
<template>
  <!-- Слой 12500 - над окном получателей (12000), из которого карточку и открывают:
       список остаётся под ней, чтобы после закрытия человек вернулся туда, откуда
       пришёл. Ниже истории заявки (20000) и глобальных диалогов (ConfirmDialog
       22000, истёкшая сессия 25000, тосты 29000) - они обязаны накрывать карточку,
       иначе вопрос о выходе снова уедет под чужой оверлей (#481, #1594). -->
  <BaseModal
    :show="show"
    title="Участник заявки"
    width="460px"
    radius="30px"
    :z-index="12500"
    content-class="participant-card-modal"
    content-testid="app-participant-card"
    @close="$emit('close')"
  >
    <div class="pcard">
      <LoaderSpinner
        v-if="loading"
        label="Загрузка участника..."
      />

      <p
        v-else-if="error"
        class="pcard__error"
        data-testid="app-participant-card-error"
      >
        {{ error }}
      </p>

      <template v-else-if="participant">
        <div class="pcard__head">
          <h4
            class="pcard__name"
            :class="{ 'pcard__name--hidden': participant.pd_hidden }"
            data-testid="app-participant-card-name"
          >
            {{ displayName(participant) }}
          </h4>
          <div class="pcard__badges">
            <Badge
              :variant="roleVariant(participant.primary_role)"
              size="sm"
              data-testid="app-participant-card-role"
            >
              {{ roleLabel(participant.primary_role) }}
            </Badge>
            <!-- Остальные роли теми же бейджами, а не строкой «Также: ...» как в
                 списке: в карточке места хватает, и роль читается одинаково. -->
            <Badge
              v-for="role in extraRoles(participant)"
              :key="role"
              variant="neutral"
              size="sm"
              data-testid="app-participant-card-role"
            >
              {{ role }}
            </Badge>
            <!-- Обязательность голоса - той же подписью, что в блоке
                 «Ответственные за согласование» карточки заявки. -->
            <Badge
              v-if="participant.required_approval"
              variant="primary"
              size="sm"
              data-testid="app-participant-card-required"
            >
              Обязательно
            </Badge>
            <Badge
              v-if="participant.approval_status"
              :variant="voteVariant(participant.approval_status)"
              size="sm"
              dot
              data-testid="app-participant-card-vote"
            >
              {{ getStatusText(participant.approval_status) }}
            </Badge>
          </div>
        </div>

        <!-- Причина, по которой вместо ФИО стоит заглушка. Без неё карточка
             читается как «данные не заполнены», а это разные вещи. -->
        <p
          v-if="participant.pd_hidden"
          class="pcard__note"
          data-testid="app-participant-card-pd-note"
        >
          Работник не дал согласия на обработку персональных данных.
        </p>

        <dl class="pcard__rows">
          <div class="pcard__row">
            <dt class="pcard__label">
              Должность
            </dt>
            <dd class="pcard__value">
              {{ participant.position || DASH }}
            </dd>
          </div>
          <div class="pcard__row">
            <dt class="pcard__label">
              Организация
            </dt>
            <dd class="pcard__value">
              {{ participant.organization_name || DASH }}
            </dd>
          </div>
          <div class="pcard__row">
            <dt class="pcard__label">
              Компания
            </dt>
            <dd class="pcard__value">
              {{ participant.company_name || DASH }}
            </dd>
          </div>

          <!-- У скрытого по ПД контактов нет в ответе вовсе, и прочерк соврал бы
               «не заполнены». Одной строкой вместо двух пустых: скрыты они всегда
               вместе. -->
          <div
            v-if="participant.pd_hidden"
            class="pcard__row"
          >
            <dt class="pcard__label">
              Контакты
            </dt>
            <dd
              class="pcard__value pcard__value--hidden"
              data-testid="app-participant-card-contacts-hidden"
            >
              Скрыты вместе с ФИО.
            </dd>
          </div>
          <template v-else>
            <div class="pcard__row">
              <dt class="pcard__label">
                Почта
              </dt>
              <dd
                class="pcard__value"
                data-testid="app-participant-card-email"
              >
                <a
                  v-if="participant.email"
                  class="pcard__link"
                  :href="`mailto:${participant.email}`"
                >{{ participant.email }}</a>
                <template v-else>
                  {{ DASH }}
                </template>
              </dd>
            </div>
            <div class="pcard__row">
              <dt class="pcard__label">
                Телефон
              </dt>
              <dd
                class="pcard__value"
                data-testid="app-participant-card-phone"
              >
                <a
                  v-if="participant.phone"
                  class="pcard__link"
                  :href="phoneHref(participant.phone)"
                >{{ phoneText(participant.phone) }}</a>
                <template v-else>
                  {{ DASH }}
                </template>
              </dd>
            </div>
          </template>
        </dl>

        <!-- Решение согласующего целиком: в списке от него виден только бейдж, а
             комментарий и время - это и есть ответ на «почему отказал». -->
        <div
          v-if="participant.approval_comment"
          class="pcard__comment"
          data-testid="app-participant-card-comment"
        >
          <span class="pcard__comment-label">Комментарий:</span>
          <span class="pcard__comment-text">{{ participant.approval_comment }}</span>
        </div>
        <!-- approval_datetime - момент решения по ИСХОДНОЙ заявке (application_responsible_users),
             голоса по дополнениям сюда не попадают (application_participants.go не смотрит раунды).
             При открытом раунде дополнения дата тут может быть старой - подпись явно называет,
             к чему относится, чтобы не читалась как решение по текущему раунду. -->
        <p
          v-if="participant.approval_datetime"
          class="pcard__time"
          data-testid="app-participant-card-decided-at"
        >
          Решение по заявке: {{ formatDateTime(participant.approval_datetime) }}
        </p>
      </template>
    </div>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import Badge from '@/components/ui/Badge.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import { formatDateTime } from '@/utils/datetime';
import { formatRussianPhoneForDisplay } from '@/composables/useRussianPhoneMask';
import {
  participantDisplayName,
  participantRoleLabel,
  participantRoleVariant,
  approvalBadgeVariant,
  secondaryRoleLabels,
} from '@/utils/participantRoles';
import { useApprovalStatus } from '@/composables/useApprovalStatus';

/** Прочерк вместо незаполненного значения: пустая строка читается как обрыв вёрстки. */
const DASH = '—';

export default {
  name: 'ApplicationParticipantCard',
  components: { BaseModal, Badge, LoaderSpinner },
  props: {
    show: {
      type: Boolean,
      required: true,
    },
    // Участник в форме ответа `/applications/:id/participants`. Своих запросов
    // карточка не делает: оба входа (строка списка получателей и согласующий в
    // блоке согласования) отдают уже загруженную запись, поэтому повторное
    // открытие ничего не стоит.
    participant: {
      type: Object,
      default: null,
    },
    loading: {
      type: Boolean,
      default: false,
    },
    error: {
      type: String,
      default: '',
    },
  },
  emits: ['close'],
  // Словарь голосов общий с блоком согласования и списком получателей.
  setup() {
    return useApprovalStatus();
  },
  data() {
    return { DASH };
  },
  methods: {
    displayName: participantDisplayName,
    roleLabel: participantRoleLabel,
    roleVariant: participantRoleVariant,
    voteVariant: approvalBadgeVariant,
    extraRoles: secondaryRoleLabels,
    formatDateTime,

    /**
     * Номер к показу. Берём display-обёртку над общей маской `formatRussianPhone`:
     * она же и накладывает маску, но сперва проверяет, что номер российский -
     * добавочный, короткий сервисный и зарубежный маска исказила бы
     * («+1 202 555 0123» стало бы «+7 (120) 255 50-12»).
     * @param {string} phone
     * @returns {string}
     */
    phoneText(phone) {
      return formatRussianPhoneForDisplay(phone) || phone;
    },

    /**
     * `tel:` разбирает номер сам, но спотыкается о пробелы и скобки маски -
     * оставляем цифры и ведущий плюс.
     * @param {string} phone
     * @returns {string}
     */
    phoneHref(phone) {
      const raw = String(phone || '').trim();
      const digits = raw.replace(/[^\d]/g, '');
      return `tel:${raw.startsWith('+') ? '+' : ''}${digits}`;
    },
  },
};
</script>

<style scoped>
.pcard {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 15px 20px 20px;
}

.pcard__error {
  margin: 0;
  padding: 20px 0;
  text-align: center;
  font-size: 14px;
  color: var(--danger-text);
}

.pcard__head {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.pcard__name {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  line-height: 1.4;
  color: var(--text);
}

.pcard__name--hidden {
  font-style: italic;
  font-weight: 500;
  color: var(--text-muted);
}

.pcard__badges {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.pcard__note {
  margin: 0;
  font-size: 12px;
  line-height: 1.4;
  color: var(--warning-text);
}

.pcard__rows {
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 15px;
}

.pcard__row {
  display: flex;
  align-items: baseline;
  gap: 10px;
}

.pcard__label {
  flex: 0 0 96px;
  font-size: 12px;
  color: var(--text-muted);
}

.pcard__value {
  margin: 0;
  min-width: 0;
  font-size: 13px;
  line-height: 1.4;
  color: var(--text);
  overflow-wrap: anywhere;
}

.pcard__value--hidden {
  color: var(--text-muted);
  font-style: italic;
}

.pcard__link {
  color: var(--accent-text);
  text-decoration: none;
}

.pcard__link:hover,
.pcard__link:focus-visible {
  text-decoration: underline;
}

.pcard__comment {
  font-size: 12px;
  line-height: 1.4;
  background: var(--accent-tint);
  border-left: 3px solid var(--accent);
  border-radius: 10px;
  padding: 8px 10px;
}

.pcard__comment-label {
  color: var(--text-muted);
  margin-right: 4px;
}

.pcard__comment-text {
  color: var(--text);
}

.pcard__time {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}

@media (max-width: 768px) {
  .pcard {
    padding: 12px 16px 16px;
  }

  /* Подпись над значением: на 320-390 колонка в 96px оставляла контактам треть
     строки, и почта переносилась посимвольно. */
  .pcard__row {
    flex-direction: column;
    gap: 2px;
  }

  .pcard__label {
    flex: none;
  }

  /* Тач-таргет ссылки: цифры и почта - единственное, по чему здесь попадают пальцем. */
  .pcard__link {
    display: inline-block;
    padding: 4px 0;
    min-height: 24px;
  }
}
</style>
