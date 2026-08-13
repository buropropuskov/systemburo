<!-- ApplicationParticipantsModal.vue -->
<template>
  <!-- Слой 12000 - полка справочных окон, открываемых поверх содержимого (истории
       справочников, выбор бланков, список онлайна). Окно обязано лечь выше всей
       стопки заявки: сама панель 10002, карточки Т/С и сотрудника из неё 10003,
       подтверждение чёрного списка 10005, назначение 10006, дополнение 10010. Выше
       брать незачем: на 20000 живёт история заявки, но открыть её при открытом
       списке получателей нечем - её кнопка под этим же затемнением, - а глобальные
       диалоги (ConfirmDialog 22000, истёкшая сессия 25000, тосты 29000) должны
       оставаться над окном. -->
  <BaseModal
    :show="show"
    title="Получатели заявки"
    width="620px"
    radius="30px"
    :z-index="12000"
    content-class="participants-modal"
    content-testid="app-participants-modal"
    @close="$emit('close')"
  >
    <div class="participants">
      <LoaderSpinner
        v-if="loading"
        label="Загрузка получателей..."
      />

      <p
        v-else-if="error"
        class="participants__error"
        data-testid="app-participants-error"
      >
        {{ error }}
      </p>

      <p
        v-else-if="participants.length === 0"
        class="participants__empty"
        data-testid="app-participants-empty"
      >
        У заявки нет ни одного участника.
      </p>

      <template v-else>
        <p class="participants__total">
          Всего: {{ participants.length }}
        </p>
        <ul class="participants__list">
          <li
            v-for="participant in participants"
            :key="participant.user_id"
            class="participant"
            data-testid="app-participants-row"
            role="button"
            tabindex="0"
            :aria-label="`Открыть карточку: ${displayName(participant)}`"
            @click="$emit('select', participant)"
            @keydown.enter.prevent="$emit('select', participant)"
            @keydown.space.prevent="$emit('select', participant)"
          >
            <div class="participant__head">
              <span
                class="participant__name"
                :class="{ 'participant__name--hidden': participant.pd_hidden }"
                data-testid="app-participants-name"
              >
                {{ displayName(participant) }}
              </span>
              <!-- Принимающим бейджа роли не рисуем: заявка уходит им по умолчанию, и
                   подпись «Принимающий» у каждого второго участника ничего не сообщала. -->
              <Badge
                v-if="participant.primary_role !== 'acceptor'"
                :variant="roleVariant(participant.primary_role)"
                size="sm"
                data-testid="app-participants-role"
              >
                {{ roleLabel(participant.primary_role) }}
              </Badge>
              <!-- Обязательность голоса - подпись рядом с ролью, как в блоке
                   «Ответственные за согласование»: там же и та же формулировка. -->
              <Badge
                v-if="participant.required_approval"
                variant="primary"
                size="sm"
                data-testid="app-participants-required"
              >
                Обязательно
              </Badge>
              <Badge
                v-if="participant.approval_status"
                :variant="voteVariant(participant.approval_status)"
                size="sm"
                dot
                data-testid="app-participants-vote"
              >
                {{ getStatusText(participant.approval_status) }}
              </Badge>
            </div>

            <!-- Скрытому по ПД пишем причину: без неё пустое имя читается как
                 «данные не заполнены», а это разные вещи. Должность и место работы
                 при этом остаются - бэкенд их не прячет. -->
            <p
              v-if="participant.pd_hidden"
              class="participant__note"
              data-testid="app-participants-hidden-note"
            >
              Работник не дал согласия на обработку персональных данных.
            </p>
            <p
              v-if="details(participant)"
              class="participant__details"
            >
              {{ details(participant) }}
            </p>

            <p
              v-if="extraRoles(participant).length"
              class="participant__extra-roles"
              data-testid="app-participants-extra-roles"
            >
              Также: {{ extraRoles(participant).join(', ').toLowerCase() }}
            </p>
          </li>
        </ul>
      </template>
    </div>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import Badge from '@/components/ui/Badge.vue';
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue';
import { getApplicationParticipants } from '@/api/applications';
import { useApprovalStatus } from '@/composables/useApprovalStatus';
import {
  participantDisplayName,
  participantRoleLabel,
  participantRoleVariant,
  approvalBadgeVariant,
  secondaryRoleLabels,
} from '@/utils/participantRoles';

export default {
  name: 'ApplicationParticipantsModal',
  components: { BaseModal, Badge, LoaderSpinner },
  props: {
    show: {
      type: Boolean,
      required: true,
    },
    applicationId: {
      type: Number,
      default: null,
    },
  },
  // select - клик по строке. Карточку участника рисует не окно, а родитель: из
  // блока согласования её открывают тем же компонентом, и два экземпляра
  // разошлись бы состоянием - что открыто и поверх чего.
  emits: ['close', 'select'],
  // Словарь голосов общий с согласованием заявки и раундами дополнения: у всех трёх
  // списков одни и те же три состояния.
  setup() {
    return useApprovalStatus();
  },
  data() {
    return {
      participants: [],
      loading: false,
      error: '',
      // Токен запроса: окно открывают и закрывают быстрее, чем отвечает сервер, и
      // ответ прошлого открытия иначе дописался бы в текущее.
      loadSeq: 0,
    };
  },
  watch: {
    show: {
      immediate: true,
      handler(visible) {
        if (visible) this.load();
      },
    },
  },
  methods: {
    displayName: participantDisplayName,
    roleLabel: participantRoleLabel,
    roleVariant: participantRoleVariant,
    voteVariant: approvalBadgeVariant,
    extraRoles: secondaryRoleLabels,

    /**
     * Должность и место работы одной строкой. Контакты сюда не идут - их показывает
     * карточка участника, а список остаётся списком.
     * @param {object} participant
     * @returns {string}
     */
    details(participant) {
      return [participant.position, participant.organization_name, participant.company_name]
        .filter(Boolean)
        .join(' · ');
    },

    async load() {
      if (!this.applicationId) return;
      const seq = ++this.loadSeq;
      this.loading = true;
      this.error = '';
      try {
        const list = await getApplicationParticipants(this.applicationId);
        if (seq !== this.loadSeq) return;
        this.participants = Array.isArray(list) ? list : [];
      } catch (e) {
        if (seq !== this.loadSeq) return;
        this.participants = [];
        this.error = e?.message || 'Не удалось загрузить получателей заявки';
      } finally {
        if (seq === this.loadSeq) this.loading = false;
      }
    },
  },
};
</script>

<style scoped>
.participants {
  padding: 15px 20px 20px;
}

.participants__total {
  margin: 0 0 10px;
  font-size: 13px;
  color: var(--text-muted);
}

.participants__error,
.participants__empty {
  margin: 0;
  padding: 20px 0;
  text-align: center;
  font-size: 14px;
  color: var(--text-muted);
}

.participants__error {
  color: var(--danger-text);
}

.participants__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.participant {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 15px;
  cursor: pointer;
  transition: border-color 0.2s ease, background 0.2s ease;
}

.participant:hover {
  border-color: var(--accent);
  background: var(--accent-tint);
}

.participant:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.participant__head {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.participant__name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  line-height: 1.4;
}

.participant__name--hidden {
  font-style: italic;
  font-weight: 500;
  color: var(--text-muted);
}

.participant__details,
.participant__extra-roles,
.participant__note {
  margin: 0;
  font-size: 12px;
  line-height: 1.4;
  color: var(--text-muted);
}

.participant__note {
  color: var(--warning-text);
}

@media (max-width: 768px) {
  .participants {
    padding: 12px 16px 16px;
  }
}
</style>
