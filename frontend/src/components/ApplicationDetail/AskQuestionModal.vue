<!-- AskQuestionModal.vue -->
<!-- Модалка создания вопроса к заявке (#973): тема + текст + необязательный выбор вложений
     тумблерами. Эталон - ForwardModal (пикер вложений). -->
<template>
  <BaseModal
    :show="show"
    title="Задать вопрос"
    width="600px"
    :z-index="20000"
    content-class="ask-question-modal"
    @close="close"
  >
    <div class="ask-body">
      <FormField
        label="Тема вопроса"
        class="ask-field"
      >
        <input
          ref="subjectInput"
          v-model="subject"
          class="lk-input"
          data-testid="ask-modal-subject"
          :maxlength="subjectMaxLength"
          type="text"
          placeholder="Например: Прицеп у фуры"
        >
      </FormField>

      <FormField
        label="Вопрос"
        class="ask-field"
      >
        <div class="ask-text-wrapper">
          <textarea
            v-model="text"
            class="lk-textarea ask-textarea"
            data-testid="ask-modal-text"
            :maxlength="textMaxLength"
            rows="4"
            placeholder="Опишите вопрос по заявке"
          />
          <div
            class="ask-text-counter"
            :class="{ 'ask-text-counter--warning': textNearLimit }"
          >
            {{ text.length }}/{{ textMaxLength }}
          </div>
        </div>
      </FormField>

      <div
        v-if="attachments.length > 0"
        class="ask-attachments"
        data-testid="ask-modal-attachments"
      >
        <div class="ask-attachments-header">
          <h4>Вложения, по которым вопрос <span class="ask-optional">(необязательно)</span></h4>
          <label class="ask-toggle">
            <input
              type="checkbox"
              class="setting-checkbox"
              :checked="allSelected"
              data-testid="ask-modal-attachments-all"
              @change="toggleAll($event.target.checked)"
            >
            <span class="toggle-slider" />
            <span class="ask-toggle-text">Выбрать все</span>
          </label>
        </div>
        <label
          v-for="attachment in attachments"
          :key="attachment.id"
          class="ask-attachment-item"
          data-testid="ask-modal-attachment"
        >
          <input
            v-model="selectedAttachmentIds"
            type="checkbox"
            class="setting-checkbox"
            :value="attachment.id"
          >
          <span class="toggle-slider" />
          <span class="ask-attachment-name">
            {{ attachment.attachment_display_name || attachment.attachment_name }}
          </span>
        </label>
      </div>
    </div>

    <template #actions>
      <button
        type="button"
        class="lk-button lk-button--ghost"
        data-testid="ask-modal-cancel"
        @click="close"
      >
        Отмена
      </button>
      <button
        type="button"
        class="lk-button lk-button--primary"
        data-testid="ask-modal-send"
        :disabled="!canSend"
        @click="send"
      >
        {{ isSubmitting ? 'Отправка...' : 'Отправить' }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue'
import FormField from '@/components/ui/FormField.vue'

export default {
    name: 'AskQuestionModal',
    components: { BaseModal, FormField },
    props: {
        show: {
            type: Boolean,
            required: true
        },
        attachments: {
            type: Array,
            default: () => []
        },
        isSubmitting: {
            type: Boolean,
            default: false
        }
    },
    emits: ['close', 'send'],
    data() {
        return {
            subject: '',
            text: '',
            selectedAttachmentIds: [],
            subjectMaxLength: 150,
            textMaxLength: 2000
        }
    },
    computed: {
        // Вложения опциональны: отправка возможна при непустых теме и тексте.
        canSend() {
            return !!this.subject.trim() && !!this.text.trim() && !this.isSubmitting;
        },
        allSelected() {
            return this.attachments.length > 0 &&
                this.selectedAttachmentIds.length === this.attachments.length;
        },
        textNearLimit() {
            return this.text.length >= this.textMaxLength - 100;
        }
    },
    watch: {
        show(visible) {
            // Без автофокуса: на мобилке фокус в поле сразу поднимал клавиатуру при
            // открытии и дёргал высоту листа - пусть юзер тапает поле сам (#1097 R3-9).
            if (visible) {
                this.reset();
            }
        }
    },
    methods: {
        toggleAll(checked) {
            this.selectedAttachmentIds = checked ? this.attachments.map(a => a.id) : [];
        },

        send() {
            if (!this.canSend) return;
            this.$emit('send', {
                subject: this.subject.trim(),
                text: this.text.trim(),
                attachment_ids: [...this.selectedAttachmentIds]
            });
        },

        close() {
            this.$emit('close');
        },

        reset() {
            this.subject = '';
            this.text = '';
            this.selectedAttachmentIds = [];
        }
    }
}
</script>

<style scoped>
.ask-body {
    padding: 20px;
}

.ask-field {
    margin-bottom: 16px;
}

.ask-optional {
    font-weight: 400;
    font-size: 13px;
    color: var(--color-text-muted);
}

.ask-text-wrapper {
    position: relative;
}

.ask-textarea {
    min-height: 96px;
    padding-bottom: 26px;
}

.ask-text-counter {
    position: absolute;
    right: 12px;
    bottom: 8px;
    font-size: 12px;
    color: var(--color-text-muted);
    pointer-events: none;
}

.ask-text-counter--warning {
    color: var(--danger-text);
}

.ask-attachments {
    display: flex;
    flex-direction: column;
    margin-top: 8px;
}

.ask-attachments-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 10px;
}

.ask-attachments-header h4 {
    font-size: 15px;
    color: var(--color-text);
    font-weight: 600;
    margin: 0;
}

.ask-toggle,
.ask-attachment-item {
    display: flex;
    align-items: center;
    gap: 10px;
    cursor: pointer;
    user-select: none;
}

.ask-toggle {
    flex-shrink: 0;
    font-size: 13px;
}

.ask-attachment-item {
    padding: 9px 8px;
    border-radius: var(--radius-md);
    transition: background-color 0.2s ease;
}

.ask-attachment-item:hover {
    background: var(--accent-tint);
}

.ask-attachment-name {
    font-size: 14px;
    color: var(--color-text);
}

/* Тумблеры (1:1 с ForwardModal .toggle-slider). */
.setting-checkbox {
    display: none;
}

.toggle-slider {
    position: relative;
    width: 34px;
    height: 18px;
    background-color: var(--border);
    border-radius: 9px;
    transition: background-color 0.3s;
    display: inline-block;
    flex-shrink: 0;
}

.toggle-slider:before {
    content: "";
    position: absolute;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background-color: var(--surface);
    top: 2px;
    left: 2px;
    transition: transform 0.3s;
}

.setting-checkbox:checked + .toggle-slider {
    background-color: var(--accent);
}

.setting-checkbox:checked + .toggle-slider:before {
    transform: translateX(16px);
}
</style>

<!-- не scoped: контент BaseModal телепортится в body, радиус задаём глобально двойным классом. -->
<style>
.base-modal.ask-question-modal {
    border-radius: 30px;
}

/* Мобилка: bottom-sheet - только верхнее скругление (низ прижат к кромке экрана).
   Глобальный двойной класс (0,2,0) иначе перебивает мобильный 16px 16px 0 0
   BaseModal (0,1,0) и рисует скругление снизу (#1097 R3-9). */
@media (max-width: 768px) {
    .base-modal.ask-question-modal {
        border-radius: 16px 16px 0 0;
    }
}
</style>
