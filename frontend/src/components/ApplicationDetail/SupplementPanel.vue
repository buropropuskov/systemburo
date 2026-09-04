<template>
  <div
    class="supplement-section"
    :class="{ 'is-collapsed': isNarrow && collapsed }"
    data-testid="supplement-panel"
  >
    <div class="supplement-header">
      <!-- На мобилке заголовок - кнопка-дисклоужер (панель свёрнута по умолчанию),
           на десктопе обычный заголовок: там панель видна целиком и сворачивать
           нечего. Тег меняем, а не дублируем разметку скрытой копией - иначе на
           узком экране в DOM два заголовка с одним data-testid. -->
      <component
        :is="isNarrow ? 'button' : 'div'"
        class="supplement-title"
        :type="isNarrow ? 'button' : null"
        :aria-expanded="isNarrow ? String(!collapsed) : null"
        :aria-controls="isNarrow ? bodyId : null"
        :data-testid="isNarrow ? 'supplement-panel-toggle' : null"
        @click="toggle"
      >
        <span
          v-if="isNarrow"
          class="supplement-chevron"
          aria-hidden="true"
        >&#9662;</span>
        <h4>Дополнения заявки</h4>
        <span
          v-if="isNarrow && supplements.length"
          class="supplement-count"
        >{{ supplements.length }}</span>
      </component>
      <div
        v-if="loading"
        class="supplement-loading"
      >
        <LoaderSpinner
          size="small"
          :label="''"
        />
      </div>
    </div>

    <!-- Раунды недоступны: панель остаётся на месте с человеческим текстом, а не
         пропадает - иначе «дополнение подавали, а его нигде нет». Ошибка живёт СНАРУЖИ
         сворачиваемой части: свёрнутая панель прятала бы её вместе с раундами, и сбой
         загрузки выглядел бы как «дополнений нет». -->
    <div
      v-if="error"
      class="supplement-error"
      data-testid="supplement-panel-error"
    >
      {{ error }}
    </div>

    <div
      :id="bodyId"
      class="supplement-body"
    >
      <div class="supplement-body-inner">
        <div class="supplement-body-pad">
          <!-- Показываем ВСЕ раунды, новые сверху (порядок задаёт бэк). Закрытые не прячем:
               по ним видно, почему добавленные строки так и не появились на посту, а самих
               раундов у заявки единицы. -->
          <div class="supplement-rounds">
            <article
              v-for="round in supplements"
              :key="round.id"
              class="supplement-round"
              :data-testid="`supplement-round-${round.id}`"
            >
              <div class="round-head">
                <span class="round-number">Дополнение №{{ round.number }}</span>
                <span
                  class="supplement-status"
                  :class="statusClass(round.status)"
                >
                  {{ statusText(round.status) }}
                </span>
              </div>

              <div class="round-author">
                Подал: {{ round.created_by_name || '-' }}
                <span class="round-time">{{ formatDateTime(round.created_at) }}</span>
              </div>

              <div
                v-if="countsLabel(round)"
                class="round-counts"
              >
                Добавлено: {{ countsLabel(round) }}
              </div>

              <div
                v-if="round.comment"
                class="round-comment"
              >
                <span class="comment-label">Комментарий:</span>
                <span class="comment-text">{{ round.comment }}</span>
              </div>

              <!-- Влитый раунд отдельного круга не проходил: голосовать по нему было некому,
                 и пустой заголовок «Согласующие (0)» только сбивал бы с толку. -->
              <div
                v-if="round.approvals && round.approvals.length"
                class="round-approvals"
              >
                <h5>Согласующие дополнения ({{ round.approvals.length }}):</h5>
                <div class="users-list">
                  <div
                    v-for="approval in round.approvals"
                    :key="approval.user_id"
                    class="user-item"
                    :class="{ 'is-me': approval.user_id === currentUserId }"
                  >
                    <div class="user-name-block">
                      {{ approval.full_name || approval.username }}
                      <span
                        v-if="approval.user_id === currentUserId"
                        class="user-self"
                      >(вы)</span>
                    </div>

                    <div class="user-badge-status-row">
                      <span
                        v-if="approval.required_approval"
                        class="badge required-badge"
                      >Обязательно</span>
                      <span
                        class="status-badge"
                        :class="getStatusClass(voteStatus(approval))"
                      >
                        {{ getStatusText(voteStatus(approval)) }}
                      </span>
                    </div>

                    <div
                      v-if="approval.approval_comment"
                      class="user-comment-block"
                    >
                      <span class="comment-label">Комментарий:</span>
                      <span class="comment-text">{{ approval.approval_comment }}</span>
                    </div>

                    <div
                      v-if="approval.approval_datetime"
                      class="user-time-block"
                    >
                      Время: {{ formatDateTime(approval.approval_datetime) }}
                    </div>
                  </div>
                </div>
              </div>

              <div
                v-if="round.decided_at"
                class="round-decision"
                :data-testid="`supplement-decision-${round.id}`"
              >
                <span class="decision-label">Решение:</span>
                <span class="decision-value">{{ statusText(round.status) }}</span>
                <span class="round-time">{{ formatDateTime(round.decided_at) }}</span>
                <div
                  v-if="round.decision_comment"
                  class="user-comment-block"
                >
                  <span class="comment-label">Комментарий:</span>
                  <span class="comment-text">{{ round.decision_comment }}</span>
                </div>
              </div>
            </article>
          </div>

          <div
            v-if="!supplements.length && !loading && !error"
            class="supplement-empty"
          >
            Дополнений по заявке нет
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { useId } from 'vue'

import LoaderSpinner from '@/components/ui/LoaderSpinner.vue'
import { useApprovalStatus } from '@/composables/useApprovalStatus'
import { useNarrowScreen } from '@/composables/useNarrowScreen'
import { supplementStatusText, supplementStatusClass, supplementCountsLabel } from '@/utils/supplementStatuses'
import { formatDateTime } from '@/utils/datetime';

export default {
    name: 'SupplementPanel',
    components: { LoaderSpinner },
    props: {
        supplements: {
            type: Array,
            default: () => []
        },
        currentUserId: {
            type: Number,
            default: null
        },
        loading: {
            type: Boolean,
            default: false
        },
        error: {
            type: String,
            default: ''
        }
    },
    setup() {
        // Порог 768, а не 767.98: панель сворачивается ровно там, где ApplicationDetail
        // промоутит колонки в одну ленту (`@media (max-width: 768px)`) и где включаются
        // её собственные мобильные стили. Разъедь пороги - на 768px (портретный iPad)
        // получился бы гибрид: блок уже в общей ленте, но ещё несворачиваемый.
        const { isNarrow } = useNarrowScreen(768);
        return { ...useApprovalStatus(), isNarrow, bodyId: `supplement-body-${useId()}` };
    },
    data() {
        return {
            // Свёрнуто при каждом открытии заявки: деталь размонтируется на закрытии
            // (`v-if` у потребителей), состояние не переживает открытие следующей.
            collapsed: true
        };
    },
    methods: {
        // На десктопе заголовок - не кнопка, но обработчик висит на том же узле:
        // гасим клик здесь, чтобы разметка оставалась одна на оба режима.
        toggle() {
            if (!this.isNarrow) return;
            this.collapsed = !this.collapsed;
        },

        statusText(status) {
            return supplementStatusText(status);
        },

        statusClass(status) {
            return supplementStatusClass(status);
        },

        countsLabel(round) {
            return supplementCountsLabel(round.counts);
        },

        // approval_status у раунда nullable, а «голоса ещё нет» и «pending» - одно и то
        // же состояние: приводим здесь, словарь голосов этого за потребителя не делает.
        voteStatus(approval) {
            return approval.approval_status || 'pending';
        },

        formatDateTime
    }
}
</script>

<style scoped>
/* Карточка построена по ApplicationConfirmation - тот же контейнер, тот же вид
   строки голосующего: обе живут в одной колонке и обязаны читаться как одно целое. */
.supplement-section {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    padding: 15px;
    margin-bottom: 10px;
    box-shadow: 0 2px 12px var(--shadow-drop);
    position: relative;
}

.supplement-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 15px;
}

.supplement-title {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
}

/* Кнопка-дисклоужер (только мобилка): тач-таргет 44px по WCAG 2.5.5, вид - обычный
   заголовок панели. */
button.supplement-title {
    flex: 1;
    min-height: 44px;
    padding: 0;
    background: none;
    border: 0;
    font: inherit;
    color: inherit;
    text-align: left;
    cursor: pointer;
}

.supplement-chevron {
    flex-shrink: 0;
    font-size: 12px;
    line-height: 1;
    color: var(--text-muted);
    transition: transform 0.2s ease;
}

.supplement-section.is-collapsed .supplement-chevron {
    transform: rotate(-90deg);
}

/* Счётчик раундов в свёрнутом заголовке - та же пилюля, что у ветки заявки
   (ForwardMessages .fm-count): свёрнутый блок обязан сказать, сколько в нём. */
.supplement-count {
    flex-shrink: 0;
    font-size: 12px;
    font-weight: 600;
    color: var(--accent-text);
    background: color-mix(in srgb, var(--accent) 8%, var(--surface));
    padding: 2px 9px;
    border-radius: 999px;
}

.supplement-header h4 {
    font-size: 18px;
    color: var(--accent-text);
    font-weight: 700;
    margin: 0;
}

.supplement-loading {
    display: flex;
    align-items: center;
}

.supplement-section > *:last-child {
    margin-bottom: 0;
}

.supplement-error {
    font-size: 13px;
    color: var(--danger-text);
    background: var(--danger-bg);
    border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
    border-radius: 10px;
    padding: 8px 12px;
    margin-bottom: 10px;
}

.supplement-empty {
    font-size: 13px;
    color: var(--text-muted);
}

.supplement-rounds {
    display: flex;
    flex-direction: column;
    gap: 12px;
}

.supplement-round {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 12px;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: 15px;
}

.round-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    flex-wrap: wrap;
}

.round-number {
    font-size: 14px;
    font-weight: 600;
    color: var(--text);
}

.supplement-status {
    padding: 4px 12px;
    border-radius: 20px;
    font-size: 11px;
    font-weight: 600;
    white-space: nowrap;
    border: 1px solid;
}

.supplement-status--pending {
    background: var(--warning-bg);
    color: var(--warning-text);
    border-color: color-mix(in srgb, var(--warning) 30%, var(--surface));
}

.supplement-status--approved,
.supplement-status--accepted {
    background: var(--success-bg);
    color: var(--success-text);
    border-color: color-mix(in srgb, var(--success) 30%, var(--surface));
}

.supplement-status--rejected {
    background: var(--danger-bg);
    color: var(--danger-text);
    border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.supplement-status--neutral {
    background: var(--border);
    color: var(--text-muted);
    border-color: var(--border);
}

.round-author,
.round-counts {
    font-size: 12px;
    color: var(--text-muted);
    line-height: 1.4;
}

.round-time {
    color: var(--text-muted);
    white-space: nowrap;
}

.round-comment,
.user-comment-block {
    font-size: 11px;
    color: var(--text-muted);
    background: var(--accent-tint);
    padding: 6px 10px;
    border-radius: 10px;
    border-left: 3px solid var(--accent);
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

.round-approvals h5 {
    font-size: 13px;
    color: var(--text-muted);
    margin: 6px 0 8px 0;
    font-weight: 400;
}

.users-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.user-item {
    display: flex;
    flex-direction: column;
    gap: 3px;
    padding: 10px 12px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 15px;
    transition: border-color 0.2s ease, background 0.2s ease;
}

.user-item.is-me {
    border-color: var(--accent);
    background: var(--accent-tint);
}

.user-name-block {
    font-weight: 600;
    color: var(--text);
    font-size: 14px;
    line-height: 1.4;
}

.user-self {
    color: var(--text-muted);
    font-weight: 400;
    font-size: 12px;
    margin-left: 4px;
}

.user-badge-status-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 2px 0;
    flex-wrap: wrap;
}

.badge,
.status-badge {
    padding: 3px 10px;
    border-radius: 20px;
    font-size: 11px;
    font-weight: 600;
    display: inline-block;
    white-space: nowrap;
}

.required-badge {
    background: var(--accent);
    color: var(--accent-contrast);
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

.user-time-block {
    font-size: 11px;
    color: var(--text-muted);
    padding-top: 4px;
}

.round-decision {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
    font-size: 12px;
    color: var(--text-muted);
    padding-top: 4px;
    border-top: 1px solid var(--border);
}

.decision-value {
    color: var(--text);
    font-weight: 600;
}

.round-decision .user-comment-block {
    flex-basis: 100%;
}

/* Мобилка: статус уходит под номер раунда, а строка решения перестаёт быть рядом -
   в 390px «Отказано в согласовании» рядом с номером не помещается. */
@media (max-width: 768px) {
    .supplement-section {
        padding: 12px;
        /* Зазор между секциями на мобилке задаёт .detail-content { gap: 10px } -
           собственный margin давал бы под панелью двойной отступ. */
        margin-bottom: 0;
    }

    .round-head,
    .round-decision {
        align-items: flex-start;
        flex-direction: column;
    }

    .supplement-header h4 {
        font-size: 16px;
    }

    /* Отступ заголовка от содержимого живёт ВНУТРИ сворачиваемой части: оставь его
       на .supplement-header - у свёрнутой панели снизу висело бы 15px пустоты, а при
       раскрытии он появлялся бы рывком мимо анимации высоты. */
    .supplement-header {
        margin-bottom: 0;
    }

    .supplement-error {
        margin: 12px 0 0;
    }

    /* Плавное сворачивание высотой (grid-rows 1fr<->0fr + min-height:0, урок #510):
       height/display не анимируются, а раскрытие обязано двигать соседние секции. */
    .supplement-body {
        display: grid;
        grid-template-rows: 1fr;
        transition: grid-template-rows 0.25s ease;
    }

    .supplement-section.is-collapsed .supplement-body {
        grid-template-rows: 0fr;
    }

    .supplement-body-inner {
        min-height: 0;
        overflow: hidden;
    }

    .supplement-body-pad {
        padding-top: 15px;
    }
}

@media (prefers-reduced-motion: reduce) {
    .supplement-body,
    .supplement-chevron {
        transition: none;
    }
}
</style>
