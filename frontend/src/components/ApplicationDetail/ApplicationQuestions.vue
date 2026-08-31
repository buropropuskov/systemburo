<!-- ApplicationQuestions.vue -->
<!-- Блок "Обсуждение заявки" (#973): список тем + тред ответов (YouTube-стиль).
     Сворачивается по заголовку (свёрнут по умолчанию), "Начать обсуждение" - в шапке. Виден
     всем с доступом к заявке. Начать обсуждение может любой с доступом, включая инициатора. -->
<template>
  <section
    class="questions-section"
    :class="{ collapsed }"
    data-testid="application-questions"
  >
    <div class="questions-header">
      <button
        type="button"
        class="questions-toggle"
        :aria-expanded="!collapsed"
        :aria-controls="bodyId"
        data-testid="questions-toggle"
        @click="toggleCollapse"
      >
        <span
          class="q-chevron"
          aria-hidden="true"
        >▾</span>
        <h4>Обсуждение заявки</h4>
        <span
          v-if="questions.length > 0"
          class="q-count"
        >{{ questions.length }}</span>
        <span
          v-if="hasUnreadInSession"
          class="q-new-dot"
          data-testid="questions-new-indicator"
          aria-label="Есть непрочитанные сообщения в обсуждении"
        >Новое</span>
      </button>
      <button
        v-if="canAsk"
        type="button"
        class="lk-button lk-button--primary lk-button--sm question-ask-button"
        data-testid="question-ask-button"
        @click="showAskModal = true"
      >
        <svg
          class="qab-icon"
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2.2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        ><circle
          cx="12"
          cy="12"
          r="10"
        /><path d="M9.1 9a3 3 0 0 1 5.82 1c0 2-3 3-3 3" /><line
          x1="12"
          y1="17"
          x2="12.01"
          y2="17"
        /></svg>
        Начать обсуждение
      </button>
    </div>

    <div
      :id="bodyId"
      class="questions-body"
    >
      <div class="questions-body-inner">
        <div class="questions-body-pad">
          <p
            v-if="questions.length === 0"
            class="questions-empty"
            data-testid="questions-empty"
          >
            Обсуждение заявки ещё не начиналось.
          </p>
          <ul
            v-else
            class="questions-list"
          >
            <li
              v-for="question in questions"
              :key="question.id"
            >
              <ApplicationQuestionItem
                :question="question"
                :application-id="applicationId"
                :current-user-id="currentUserId"
                :current-user-name="currentUserName"
                :initiator-user-id="initiatorUserId"
                :is-new="snapshotNewIds.includes(question.id) && !readIds.includes(question.id)"
                @answered="load"
                @read="onTopicRead"
              />
            </li>
          </ul>
        </div>
      </div>
    </div>

    <AskQuestionModal
      :show="showAskModal"
      :attachments="attachments"
      :is-submitting="submitting"
      @close="showAskModal = false"
      @send="submitQuestion"
    />
  </section>
</template>

<script>
import { getQuestions, createQuestion, markQuestionRead } from '@/api/applications'
import { useDeletionsStore } from '@/stores/deletions'
import ApplicationQuestionItem from './ApplicationQuestionItem.vue'
import AskQuestionModal from './AskQuestionModal.vue'

export default {
    name: 'ApplicationQuestions',
    components: { ApplicationQuestionItem, AskQuestionModal },
    props: {
        applicationId: {
            type: Number,
            required: true
        },
        attachments: {
            type: Array,
            default: () => []
        },
        currentUserId: {
            type: Number,
            default: null
        },
        currentUserName: {
            type: String,
            default: ''
        },
        initiatorUserId: {
            type: Number,
            default: null
        },
        canAsk: {
            type: Boolean,
            default: false
        }
    },
    emits: ['all-questions-read'],
    data() {
        return {
            questions: [],
            loadSeq: 0,
            collapsed: this.readCollapsed(),
            showAskModal: false,
            submitting: false,
            // Снимок новизны при открытии заявки: id топиков, что были новыми. Нужен, чтобы
            // прочитанные ранее (is_new=false с бэка) не светились, а бейдж показывался только
            // тем, что реально было новым на момент захода.
            snapshotNewIds: [],
            snapshotTaken: false,
            // Топики, прочитанные в этом сеансе (клик). Гасят бейдж топика И индикатор заголовка
            // сразу; недочитанные остаются новыми.
            readIds: []
        }
    },
    computed: {
        bodyId() {
            return `application-questions-body-${this.applicationId}`;
        },
        // Индикатор заголовка горит, пока есть новый топик, ещё не прочитанный в этом сеансе.
        hasUnreadInSession() {
            return this.snapshotNewIds.some(id => !this.readIds.includes(id));
        }
    },
    watch: {
        applicationId() {
            this.snapshotTaken = false;
            this.snapshotNewIds = [];
            this.readIds = [];
            // Свёрнутость - per-заявка: перечитываем состояние новой заявки.
            this.collapsed = this.readCollapsed();
            this.load();
        }
    },
    mounted() {
        this.load();
    },
    methods: {
        // Помечаем топик прочитанным по клику (#973): его бейдж и (когда прочитаны все) индикатор
        // заголовка гаснут сразу. Когда в сеансе не осталось непрочитанного - сообщаем наверх,
        // чтобы маркер обсуждений в списке заявок тоже погас, не дожидаясь перезагрузки списка.
        onTopicRead(questionId) {
            if (this.readIds.includes(questionId)) return;
            this.readIds.push(questionId);
            markQuestionRead(this.applicationId, questionId).catch(() => {});
            if (!this.hasUnreadInSession) {
                this.$emit('all-questions-read', this.applicationId);
            }
        },

        // Ключ свёрнутости - per-заявка: разные заявки помнят своё состояние независимо.
        collapseKey() {
            return `applicationQuestions.collapsed.${this.applicationId}`;
        },

        readCollapsed() {
            try {
                return localStorage.getItem(this.collapseKey()) !== 'false';
            } catch {
                return true;
            }
        },

        toggleCollapse() {
            this.collapsed = !this.collapsed;
            try {
                localStorage.setItem(this.collapseKey(), String(this.collapsed));
            } catch {
                // Персист необязателен.
            }
        },

        // Seq-токен от гонки при быстрой смене заявки (урок #632).
        async load() {
            if (!this.applicationId) return;
            const seq = ++this.loadSeq;
            try {
                const data = await getQuestions(this.applicationId);
                if (seq !== this.loadSeq) return;
                this.questions = Array.isArray(data) ? data : [];
                // Снимок берём один раз за сеанс просмотра заявки: повторные load (после
                // отправки темы/ответа) не пересобирают его, иначе бейджи мигали бы.
                if (!this.snapshotTaken) {
                    this.snapshotNewIds = this.questions.filter(q => q.is_new).map(q => q.id);
                    this.snapshotTaken = true;
                }
            } catch {
                // Блок вспомогательный: при сбое сохраняем отрисованное, деталь не роняем.
                if (seq !== this.loadSeq) return;
            }
        },

        async submitQuestion(payload) {
            this.submitting = true;
            try {
                await createQuestion(this.applicationId, payload);
                useDeletionsStore().notify({ bold: 'Обсуждение начато', type: 'success' });
                this.showAskModal = false;
                await this.load();
            } catch (err) {
                useDeletionsStore().notify({ prefix: 'Не удалось начать обсуждение: ', bold: err.message, type: 'error' });
            } finally {
                this.submitting = false;
            }
        }
    }
}
</script>

<style scoped>
.questions-section {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    box-shadow: 0 2px 12px var(--shadow-drop);
    overflow: hidden;
}

.questions-header {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 15px;
    border-bottom: 1px solid var(--border);
    transition: border-color 0.2s ease;
}

.questions-section.collapsed .questions-header {
    border-bottom-color: transparent;
}

.questions-toggle {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 10px;
    background: none;
    border: none;
    padding: 0;
    margin: 0;
    cursor: pointer;
    font-family: inherit;
    text-align: left;
}

.q-chevron {
    color: var(--text-muted);
    font-size: 12px;
    transition: transform 0.2s ease;
}

.questions-section.collapsed .q-chevron {
    transform: rotate(-90deg);
}

.questions-header h4 {
    margin: 0;
    font-size: 14px;
    color: var(--text-muted);
    font-weight: 400;
}

.q-count {
    font-size: 12px;
    color: var(--color-primary, var(--accent-text));
    background: color-mix(in srgb, var(--accent) 8%, var(--surface));
    padding: 2px 9px;
    border-radius: 999px;
    font-weight: 600;
}

/* Индикатор "есть непрочитанное" на заголовке блока: гаснет, когда все новые топики
   прочитаны в этом сеансе (#973). */
.q-new-dot {
    font-size: 11px;
    line-height: 1;
    color: var(--warning-text);
    background: var(--warning-bg);
    border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
    padding: 3px 8px;
    border-radius: 999px;
    font-weight: 600;
    letter-spacing: 0.2px;
}

.question-ask-button {
    flex-shrink: 0;
    height: 24px;
    padding: 0 12px;
    font-size: 12px;
    display: inline-flex;
    align-items: center;
    gap: 6px;
}

.qab-icon {
    flex-shrink: 0;
}

/* Плавное сворачивание высотой (grid-rows, урок #510). */
.questions-body {
    display: grid;
    grid-template-rows: 1fr;
    transition: grid-template-rows 0.25s ease;
}

.questions-section.collapsed .questions-body {
    grid-template-rows: 0fr;
}

.questions-body-inner {
    min-height: 0;
    overflow: hidden;
}

.questions-body-pad {
    padding: 15px;
}

.questions-empty {
    margin: 0;
    font-size: 13px;
    color: var(--color-text-muted);
}

.questions-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
}

/* Разделитель между топиками - на уровне li (не внутри .qi, где :last-child ловил бы
   каждый топик). Линия идёт по центру симметричного padding соседних .qi. */
.questions-list li:not(:last-child) {
    border-bottom: 1px solid var(--border);
}

.questions-list li:first-child :deep(.qi) {
    padding-top: 0;
}

.questions-list li:last-child :deep(.qi) {
    padding-bottom: 0;
}
</style>
