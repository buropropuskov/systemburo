<!-- ApplicationBureauNote.vue -->
<!-- Заметка бюро по заявке: принимающий оставляет себе и коллегам объяснение, почему
     заявка не сделана и что по ней осталось. Одна заметка на заявку, общая для всех
     принимающих.

     Блок рисуется только принимающему, и решает это родитель (v-if="isApprover").
     Гейт здесь - удобство, а не защита: текст заметки бэк отдаёт в детали заявки тоже
     только принимающему, поэтому у остальных его в applicationData просто нет. -->
<template>
  <section
    class="bureau-note-section"
    data-testid="bureau-note"
  >
    <div class="bureau-note-header">
      <h4>Заметка бюро</h4>
      <span class="bureau-note-hint">видна только принимающим</span>
    </div>

    <div class="bureau-note-body">
      <template v-if="editing">
        <textarea
          ref="input"
          v-model="draft"
          class="lk-textarea bureau-note-input"
          rows="3"
          :maxlength="maxLength"
          :disabled="saving"
          placeholder="Почему заявка не сделана, что по ней осталось"
          data-testid="bureau-note-input"
        />
        <div class="bureau-note-actions">
          <button
            type="button"
            class="lk-button lk-button--primary"
            :disabled="saving || !changed"
            data-testid="bureau-note-save"
            @click="save"
          >
            Сохранить
          </button>
          <button
            type="button"
            class="lk-button lk-button--secondary"
            :disabled="saving"
            @click="cancel"
          >
            Отмена
          </button>
          <span class="bureau-note-counter">{{ draft.length }} / {{ maxLength }}</span>
        </div>
      </template>

      <template v-else-if="hasNote">
        <p
          class="bureau-note-text"
          data-testid="bureau-note-text"
        >
          {{ note.text }}
        </p>
        <div class="bureau-note-meta">
          <span v-if="note.author_name">{{ note.author_name }}</span>
          <span v-if="formattedDate">{{ formattedDate }}</span>
        </div>
        <div class="bureau-note-actions">
          <button
            type="button"
            class="lk-button lk-button--secondary"
            data-testid="bureau-note-edit"
            @click="startEditing"
          >
            Изменить
          </button>
          <button
            type="button"
            class="lk-button lk-button--ghost"
            :disabled="saving"
            data-testid="bureau-note-clear"
            @click="clear"
          >
            Очистить
          </button>
        </div>
      </template>

      <template v-else>
        <p class="bureau-note-empty">
          Заметки нет
        </p>
        <button
          type="button"
          class="lk-button lk-button--secondary"
          data-testid="bureau-note-add"
          @click="startEditing"
        >
          Добавить заметку
        </button>
      </template>
    </div>
  </section>
</template>

<script>
import { setBureauNote } from '@/api/applications'
import { useDeletionsStore } from '@/stores/deletions'

export default {
    name: 'ApplicationBureauNote',
    props: {
        applicationId: {
            type: Number,
            required: true
        },
        // Заметка из детали заявки: {text, author_id, author_name, updated_at} либо null.
        note: {
            type: Object,
            default: null
        }
    },
    emits: ['update'],
    data() {
        return {
            editing: false,
            draft: '',
            saving: false,
            // Предел повторяет BureauNoteMaxLen на бэке: заметка - короткое напоминание,
            // для длинных обсуждений есть вопросы к заявке.
            maxLength: 2000
        }
    },
    computed: {
        hasNote() {
            return !!(this.note && this.note.text);
        },

        // Кнопка сохранения гаснет, пока текст не отличается от сохранённого: повторное
        // нажатие иначе переписывало бы автора и время без единой правки текста.
        changed() {
            return this.draft.trim() !== this.savedText;
        },

        savedText() {
            return this.hasNote ? this.note.text : '';
        },

        formattedDate() {
            if (!this.note || !this.note.updated_at) return '';
            const date = new Date(this.note.updated_at);
            if (Number.isNaN(date.getTime())) return '';
            return date.toLocaleString('ru-RU', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric',
                hour: '2-digit',
                minute: '2-digit'
            });
        }
    },
    watch: {
        // Переключение детали на другую заявку без размонтирования: незаконченная правка
        // чужой заметки не должна переехать на соседнюю заявку.
        applicationId() {
            this.editing = false;
            this.draft = '';
        }
    },
    methods: {
        startEditing() {
            this.draft = this.savedText;
            this.editing = true;
            this.$nextTick(() => {
                if (this.$refs.input) this.$refs.input.focus();
            });
        },

        cancel() {
            this.editing = false;
            this.draft = '';
        },

        save() {
            this.submit(this.draft.trim(), 'Заметка сохранена');
        },

        clear() {
            this.submit('', 'Заметка удалена');
        },

        // Пустой текст снимает заметку - на бэке та же развилка, отдельного метода
        // удаления нет.
        async submit(text, successText) {
            if (this.saving) return;
            this.saving = true;
            try {
                const saved = await setBureauNote(this.applicationId, text);
                this.$emit('update', saved || null);
                this.editing = false;
                this.draft = '';
                useDeletionsStore().notify({ bold: successText, type: 'success' });
            } catch (error) {
                useDeletionsStore().notify({
                    prefix: 'Не удалось сохранить заметку: ',
                    bold: error.message || 'ошибка сети',
                    type: 'error'
                });
            } finally {
                this.saving = false;
            }
        }
    }
}
</script>

<style scoped>
.bureau-note-section {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-md, 15px);
    margin-bottom: 15px;
}

.bureau-note-header {
    display: flex;
    align-items: baseline;
    gap: 10px;
    padding: 12px 15px;
    border-bottom: 1px solid var(--border);
}

.bureau-note-header h4 {
    margin: 0;
    font-size: 14px;
    font-weight: 400;
    color: var(--text-muted);
}

.bureau-note-hint {
    font-size: 12px;
    color: var(--text-muted);
}

.bureau-note-body {
    padding: 15px;
}

.bureau-note-text {
    margin: 0 0 8px;
    font-size: 14px;
    line-height: 150%;
    color: var(--text);
    white-space: pre-wrap;
    word-break: break-word;
}

.bureau-note-empty {
    margin: 0 0 10px;
    font-size: 14px;
    color: var(--text-muted);
}

.bureau-note-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    margin-bottom: 10px;
    font-size: 12px;
    color: var(--text-muted);
}

.bureau-note-input {
    width: 100%;
    margin-bottom: 10px;
    resize: vertical;
}

.bureau-note-actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 10px;
}

.bureau-note-counter {
    margin-left: auto;
    font-size: 12px;
    color: var(--text-muted);
}
</style>
