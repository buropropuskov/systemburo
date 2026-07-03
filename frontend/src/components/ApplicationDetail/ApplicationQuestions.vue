<!-- ApplicationQuestions.vue -->
<!-- Блок "Вопросы к заявке" (#973): список вопросов-топиков + тред ответов (YouTube-стиль).
     Сворачивается по заголовку (свёрнут по умолчанию), "Задать вопрос" - в шапке. Виден всем
     с доступом к заявке. Задать вопрос может любой с доступом, включая инициатора. -->
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
        <h4>Вопросы к заявке</h4>
        <span
          v-if="questions.length > 0"
          class="q-count"
        >{{ questions.length }}</span>
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
        Задать вопрос
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
            Пока нет вопросов к заявке.
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
                @answered="load"
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
import { getQuestions, createQuestion, markQuestionsSeen } from '@/api/applications'
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
    data() {
        return {
            questions: [],
            loadSeq: 0,
            collapsed: this.readCollapsed(),
            showAskModal: false,
            submitting: false
        }
    },
    computed: {
        bodyId() {
            return `application-questions-body-${this.applicationId}`;
        }
    },
    watch: {
        applicationId() {
            this.load();
            this.markSeen();
        }
    },
    mounted() {
        this.load();
        this.markSeen();
    },
    methods: {
        // Отмечаем вопросы/ответы просмотренными при открытии заявки -> гасит маркер
        // has_unseen_questions в списке (#973). Fire-and-forget, как markAsRead.
        markSeen() {
            if (!this.applicationId) return;
            markQuestionsSeen(this.applicationId).catch(() => {});
        },

        readCollapsed() {
            try {
                return localStorage.getItem('applicationQuestions.collapsed') !== 'false';
            } catch {
                return true;
            }
        },

        toggleCollapse() {
            this.collapsed = !this.collapsed;
            try {
                localStorage.setItem('applicationQuestions.collapsed', String(this.collapsed));
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
            } catch {
                // Блок вспомогательный: при сбое сохраняем отрисованное, деталь не роняем.
                if (seq !== this.loadSeq) return;
            }
        },

        async submitQuestion(payload) {
            this.submitting = true;
            try {
                await createQuestion(this.applicationId, payload);
                useDeletionsStore().notify({ bold: 'Вопрос отправлен', type: 'success' });
                this.showAskModal = false;
                await this.load();
            } catch (err) {
                useDeletionsStore().notify({ prefix: 'Не удалось отправить вопрос: ', bold: err.message, type: 'error' });
            } finally {
                this.submitting = false;
            }
        }
    }
}
</script>

<style scoped>
.questions-section {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
    overflow: hidden;
}

.questions-header {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 15px;
    border-bottom: 1px solid #e6e6e6;
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
    color: #a2a2a2;
    font-size: 12px;
    transition: transform 0.2s ease;
}

.questions-section.collapsed .q-chevron {
    transform: rotate(-90deg);
}

.questions-header h4 {
    margin: 0;
    font-size: 14px;
    color: #a2a2a2;
    font-weight: 400;
}

.q-count {
    font-size: 12px;
    color: var(--color-primary, #4F5BDF);
    background: rgba(79, 91, 223, 0.08);
    padding: 2px 9px;
    border-radius: 999px;
    font-weight: 600;
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
</style>
