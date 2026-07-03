<!-- ApplicationQuestionItem.vue -->
<!-- Один вопрос-топик к заявке (#973) + тред ответов (как комментарии на YouTube).
     Тема - крупный заголовок; автор/время - отдельной приглушённой строкой. Ответы
     свёрнуты по умолчанию, разворачиваются по кнопке. Форма ответа - кнопка внутри поля. -->
<template>
  <div
    class="qi"
    data-testid="question-item"
  >
    <div
      class="qi-subject"
      data-testid="question-subject"
      @click="onSubjectClick"
    >
      <span class="qi-subject-text">{{ question.subject }}</span>
      <span
        v-if="isNew"
        class="qi-new"
        data-testid="question-new-badge"
      >Новое</span>
    </div>
    <div class="qi-meta">
      <span class="qi-author">
        {{ question.author_name || 'Пользователь' }}<span
          v-if="isInitiator(question.author_user_id)"
          class="qi-init"
        > · Инициатор заявки</span>
      </span>
      <span class="qi-when">{{ formatDateTime(question.created_at) }}</span>
    </div>

    <p
      v-if="attachmentNames.length > 0"
      class="qi-attachments"
    >
      <span class="qi-attachments-label">по вложениям:</span>
      <span
        v-for="name in attachmentNames"
        :key="name"
        class="qi-chip"
      >{{ name }}</span>
    </p>

    <p class="qi-text">
      {{ question.text }}
    </p>

    <button
      v-if="answers.length > 0"
      type="button"
      class="qi-toggle"
      :aria-expanded="expanded"
      data-testid="question-toggle-answers"
      @click="toggleAnswers"
    >
      <span
        class="qi-tri"
        aria-hidden="true"
      >▸</span>
      {{ expanded ? 'Скрыть ответы' : 'Показать ответы' }} ({{ answers.length }})
    </button>

    <div
      v-show="expanded || answers.length === 0"
      class="qi-thread"
    >
      <div
        v-for="answer in answers"
        :key="answer.id"
        class="qi-answer"
        data-testid="answer-item"
      >
        <div class="qi-meta">
          <span class="qi-author">
            {{ answer.author_name || 'Пользователь' }}<span
              v-if="isInitiator(answer.author_user_id)"
              class="qi-init"
            > · Инициатор заявки</span>
          </span>
          <span class="qi-when">{{ formatDateTime(answer.created_at) }}</span>
        </div>
        <p class="qi-answer-text">
          {{ answer.text }}
        </p>
      </div>

      <div class="qi-reply">
        <textarea
          v-model="replyText"
          class="qi-reply-input"
          data-testid="answer-input"
          placeholder="Написать ответ..."
          maxlength="2000"
          rows="2"
        />
        <button
          type="button"
          class="qi-reply-send"
          data-testid="answer-send"
          :disabled="!canReply"
          @click="submitReply"
        >
          {{ submittingReply ? '...' : 'Ответить' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import { createAnswer } from '@/api/applications'
import { useDeletionsStore } from '@/stores/deletions'

export default {
    name: 'ApplicationQuestionItem',
    props: {
        question: {
            type: Object,
            required: true
        },
        applicationId: {
            type: Number,
            required: true
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
        isNew: {
            type: Boolean,
            default: false
        }
    },
    emits: ['answered', 'read'],
    data() {
        return {
            expanded: false,
            replyText: '',
            submittingReply: false,
            // Локальная копия ответов для оптимистичного добавления.
            answers: Array.isArray(this.question.answers) ? [...this.question.answers] : []
        }
    },
    computed: {
        canReply() {
            return !!this.replyText.trim() && !this.submittingReply;
        },
        attachmentNames() {
            if (!Array.isArray(this.question.attachments)) return [];
            return this.question.attachments.map(a => a.display_name).filter(Boolean);
        }
    },
    watch: {
        // Обновление вопроса извне (перезагрузка списка) -> пересобрать ответы.
        question: {
            deep: true,
            handler(q) {
                this.answers = Array.isArray(q.answers) ? [...q.answers] : [];
            }
        }
    },
    methods: {
        isInitiator(userId) {
            return this.initiatorUserId != null && userId === this.initiatorUserId;
        },

        // Клик по теме = "открыл вопрос" -> помечаем топик прочитанным (#973) и разворачиваем
        // тред, если есть ответы. Бейдж "Новое" держится по снимку до перезахода.
        onSubjectClick() {
            this.$emit('read', this.question.id);
            if (this.answers.length > 0) {
                this.expanded = true;
            }
        },

        // Разворачивание треда кнопкой тоже засчитывается как прочтение топика.
        toggleAnswers() {
            this.expanded = !this.expanded;
            if (this.expanded) {
                this.$emit('read', this.question.id);
            }
        },

        async submitReply() {
            if (!this.canReply) return;
            const text = this.replyText.trim();
            const optimistic = {
                id: `tmp-${Date.now()}`,
                author_user_id: this.currentUserId,
                author_name: this.currentUserName || 'Вы',
                text,
                created_at: new Date().toISOString()
            };
            this.answers.push(optimistic);
            this.replyText = '';
            this.expanded = true;
            this.submittingReply = true;
            try {
                const saved = await createAnswer(this.applicationId, this.question.id, { text });
                // Заменяем оптимистичную запись реальной.
                const idx = this.answers.findIndex(a => a.id === optimistic.id);
                if (idx !== -1 && saved) {
                    this.answers.splice(idx, 1, saved);
                }
                this.$emit('answered', this.question.id);
            } catch (err) {
                // Откатываем оптимистичный ответ и показываем ошибку.
                this.answers = this.answers.filter(a => a.id !== optimistic.id);
                this.replyText = text;
                useDeletionsStore().notify({ prefix: 'Не удалось отправить ответ: ', bold: err.message, type: 'error' });
            } finally {
                this.submittingReply = false;
            }
        },

        formatDateTime(value) {
            if (!value) return '';
            const date = new Date(value);
            if (Number.isNaN(date.getTime())) return '';
            return date.toLocaleString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric',
                hour: '2-digit',
                minute: '2-digit'
            });
        }
    }
}
</script>

<style scoped>
.qi {
    padding: 16px 0;
    border-bottom: 1px solid #c9c9c9;
}

.qi:first-child {
    padding-top: 0;
}

.qi:last-child {
    padding-bottom: 0;
    border-bottom: none;
}

.qi-subject {
    display: flex;
    align-items: baseline;
    gap: 8px;
    font-size: 15px;
    font-weight: 700;
    color: #111;
    margin-bottom: 4px;
    word-break: break-word;
    cursor: pointer;
}

.qi-subject-text {
    flex: 1;
    min-width: 0;
}

/* Бейдж "Новое" на топике держится весь сеанс по снимку новизны (#973). */
.qi-new {
    flex-shrink: 0;
    font-size: 11px;
    line-height: 1;
    font-weight: 600;
    letter-spacing: 0.2px;
    color: #c0392b;
    background: rgba(220, 53, 69, 0.12);
    padding: 3px 8px;
    border-radius: 999px;
}

.qi-meta {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 10px;
    margin-bottom: 6px;
}

.qi-author {
    font-size: 13px;
    font-weight: 500;
    color: #555;
}

.qi-init {
    color: var(--color-primary);
    font-weight: 500;
}

.qi-when {
    font-size: 12px;
    color: var(--color-text-muted);
    flex-shrink: 0;
}

.qi-attachments {
    margin: 0 0 6px;
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
    align-items: center;
}

.qi-attachments-label {
    font-size: 12px;
    color: var(--color-text-muted);
}

.qi-chip {
    font-size: 11px;
    font-weight: 600;
    color: var(--color-primary);
    background: var(--color-primary-tint, rgba(79, 91, 223, 0.08));
    padding: 2px 9px;
    border-radius: 8px;
}

.qi-text {
    margin: 0 0 8px;
    font-size: 14px;
    line-height: 150%;
    color: #222;
    white-space: pre-wrap;
    word-break: break-word;
}

.qi-toggle {
    background: none;
    border: none;
    color: var(--color-primary);
    font-family: inherit;
    font-weight: 600;
    font-size: 13px;
    cursor: pointer;
    padding: 2px 0;
    display: inline-flex;
    align-items: center;
    gap: 6px;
}

.qi-tri {
    font-size: 10px;
    transition: transform 0.2s ease;
}

.qi-toggle[aria-expanded="true"] .qi-tri {
    transform: rotate(90deg);
}

.qi-thread {
    margin: 8px 0 0 14px;
    padding-left: 14px;
    border-left: 2px solid #eef0ff;
}

.qi-answer {
    padding: 8px 0;
}

.qi-answer + .qi-answer {
    border-top: 1px solid #f2f2f2;
}

.qi-answer-text {
    margin: 0;
    font-size: 13.5px;
    line-height: 150%;
    color: #333;
    white-space: pre-wrap;
    word-break: break-word;
}

/* Форма ответа: кнопка внутри поля ввода (как в мессенджерах). */
.qi-reply {
    position: relative;
    margin-top: 10px;
}

.qi-reply-input {
    width: 100%;
    font-family: inherit;
    font-size: 13px;
    padding: 9px 108px 9px 12px;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    resize: vertical;
    min-height: 44px;
}

.qi-reply-input:focus {
    outline: none;
    border-color: var(--color-primary);
    box-shadow: 0 0 0 3px var(--color-primary-tint, rgba(79, 91, 223, 0.08));
}

.qi-reply-send {
    position: absolute;
    right: 10px;
    bottom: 15px;
    font-family: inherit;
    font-size: 12px;
    font-weight: 600;
    color: #fff;
    background: var(--color-primary);
    border: none;
    border-radius: var(--radius-pill, 999px);
    padding: 6px 14px;
    cursor: pointer;
    transition: background 0.15s ease;
}

.qi-reply-send:hover:not(:disabled) {
    background: var(--color-primary-hover, #3d49c7);
}

.qi-reply-send:disabled {
    opacity: 0.5;
    cursor: not-allowed;
}
</style>
