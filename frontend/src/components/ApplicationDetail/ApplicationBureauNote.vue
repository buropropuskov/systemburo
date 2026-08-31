<!-- ApplicationBureauNote.vue -->
<!-- Заметка бюро по заявке: принимающий оставляет себе и коллегам объяснение, почему
     заявка не сделана и что по ней осталось. Одна заметка на заявку, общая для всех
     принимающих.

     Намеренно не отдельный блок с заголовком, а строка в потоке карточки: заметка
     здесь не раздел, а пометка на полях. Пустая - это одна ссылка «Заметка бюро»,
     заполненная - строка с текстом; поле ввода появляется только на время правки.

     Блок рисуется только принимающему, и решает это родитель (v-if="isApprover").
     Гейт здесь - удобство, а не защита: текст заметки бэк отдаёт в детали заявки тоже
     только принимающему, поэтому у остальных его в applicationData просто нет. -->
<template>
  <div
    class="bureau-note"
    :class="{ 'bureau-note--editing': editing }"
    data-testid="bureau-note"
  >
    <template v-if="editing">
      <textarea
        ref="input"
        v-model="draft"
        class="lk-textarea bureau-note__input"
        rows="2"
        :maxlength="maxLength"
        :disabled="saving"
        placeholder="Почему заявка не сделана, что по ней осталось"
        data-testid="bureau-note-input"
        @keydown.esc="cancel"
      />
      <div class="bureau-note__row">
        <button
          type="button"
          class="bureau-note__action bureau-note__action--primary"
          :disabled="saving || !changed"
          data-testid="bureau-note-save"
          @click="save"
        >
          Сохранить
        </button>
        <button
          type="button"
          class="bureau-note__action"
          :disabled="saving"
          @click="cancel"
        >
          Отмена
        </button>
        <span class="bureau-note__counter">{{ draft.length }} / {{ maxLength }}</span>
      </div>
    </template>

    <div
      v-else-if="hasNote"
      class="bureau-note__row"
    >
      <span
        class="bureau-note__label"
        :title="noteTitle"
      >Заметка бюро:</span>
      <span
        class="bureau-note__text"
        :title="noteTitle"
        data-testid="bureau-note-text"
      >{{ note.text }}</span>
      <button
        type="button"
        class="bureau-note__action"
        data-testid="bureau-note-edit"
        @click="startEditing"
      >
        Изменить
      </button>
      <button
        type="button"
        class="bureau-note__action"
        :disabled="saving"
        data-testid="bureau-note-clear"
        @click="clear"
      >
        Очистить
      </button>
    </div>

    <button
      v-else
      type="button"
      class="bureau-note__add"
      data-testid="bureau-note-add"
      @click="startEditing"
    >
      Заметка бюро
    </button>
  </div>
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

        // Автор и время - в подсказке строки: в самой строке им места нет, а знать,
        // кто и когда оставил заметку, по-прежнему нужно.
        noteTitle() {
            if (!this.hasNote) return '';
            const parts = [this.note.text];
            const signature = [this.note.author_name, this.formattedDate].filter(Boolean).join(', ');
            if (signature) parts.push(signature);
            return parts.join('\n\n');
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
/* Строка, а не карточка: ни рамки, ни фона, ни заголовка - заметка живёт в потоке
   карточки заявки как пометка на полях. Собственных отступов минимум, чтобы при
   пустой заметке ряд занимал одну строку. */
.bureau-note {
    margin: 0 0 10px;
}

.bureau-note__row {
    display: flex;
    align-items: baseline;
    gap: 8px;
    min-width: 0;
}

.bureau-note__label {
    color: var(--text-muted);
    font-size: 13px;
    white-space: nowrap;
}

/* Длинная заметка не растит карточку: одна строка с многоточием, целиком - в
   подсказке (там же автор и время). */
.bureau-note__text {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 13px;
    color: var(--text);
}

/* Действия - текстовые ссылки, а не пилюли: три кнопки рядом с текстом в строке
   перевесили бы саму заметку. */
.bureau-note__action {
    background: none;
    border: none;
    padding: 0;
    font: inherit;
    font-size: 12px;
    color: var(--text-muted);
    cursor: pointer;
    white-space: nowrap;
    transition: color 0.15s ease;
}

.bureau-note__action:hover:not(:disabled) {
    color: var(--accent);
}

.bureau-note__action:disabled {
    opacity: 0.5;
    cursor: default;
}

.bureau-note__action--primary {
    color: var(--accent);
    font-weight: 600;
}

.bureau-note__add {
    background: none;
    border: none;
    padding: 0;
    font: inherit;
    font-size: 13px;
    color: var(--text-muted);
    cursor: pointer;
    text-decoration: underline dotted;
    text-underline-offset: 3px;
    transition: color 0.15s ease;
}

.bureau-note__add:hover {
    color: var(--accent);
}

.bureau-note--editing {
    margin-bottom: 14px;
}

.bureau-note__input {
    width: 100%;
    margin-bottom: 8px;
    resize: vertical;
    font-size: 13px;
}

.bureau-note__counter {
    margin-left: auto;
    font-size: 11px;
    color: var(--text-muted);
    white-space: nowrap;
}
</style>
