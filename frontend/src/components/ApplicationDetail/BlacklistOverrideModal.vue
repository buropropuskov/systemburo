<template>
  <BaseModal
    :show="show"
    title="Всё равно пропустить?"
    width="460px"
    :z-index="10005"
    content-class="bl-override-modal"
    @close="$emit('close')"
  >
    <div class="override-body">
      <p class="override-lead">
        Элемент похож на активную запись чёрного списка, но это не точное совпадение.
        Подтвердите действие.
      </p>

      <div
        v-if="flag"
        class="override-matched"
      >
        <div class="override-matched__label">
          Похоже на запись ЧС
        </div>
        <div class="override-matched__value">
          {{ flag.matched_value }}
        </div>
        <div
          v-if="flag.matched_reason"
          class="override-matched__reason"
        >
          Причина: {{ flag.matched_reason }}
        </div>
      </div>

      <FormField
        label="Причина пропуска"
        required
      >
        <textarea
          v-model="comment"
          class="lk-textarea"
          rows="3"
          placeholder="Введите причину..."
          @keydown.enter.ctrl="submit"
        />
      </FormField>
    </div>

    <template #actions>
      <button
        type="button"
        class="lk-button lk-button--ghost"
        :disabled="submitting"
        @click="$emit('close')"
      >
        Отмена
      </button>
      <button
        type="button"
        class="lk-button lk-button--primary"
        :disabled="!canConfirm"
        @click="submit"
      >
        Всё равно пропустить
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue'
import FormField from '@/components/ui/FormField.vue'

export default {
    name: 'BlacklistOverrideModal',
    components: { BaseModal, FormField },
    props: {
        show: {
            type: Boolean,
            required: true
        },
        flag: {
            type: Object,
            default: null
        },
        submitting: {
            type: Boolean,
            default: false
        }
    },
    emits: ['confirm', 'close'],
    data() {
        return {
            comment: ''
        };
    },
    computed: {
        canConfirm() {
            return !this.submitting && this.comment.trim().length > 0;
        }
    },
    watch: {
        show(visible) {
            if (visible) {
                this.comment = '';
            }
        }
    },
    methods: {
        submit() {
            if (!this.canConfirm) return;
            this.$emit('confirm', this.comment.trim());
        }
    }
}
</script>

<style scoped>
.override-body {
    padding: 20px;
}

.override-lead {
    margin: 0 0 16px;
    font-size: 13.5px;
    line-height: 1.5;
    color: var(--text-muted);
}

/* красная палитра зеркалит Badge danger и подсветку строки (#481, срез 5) - семантических токенов под неё нет */
.override-matched {
    border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
    background: var(--danger-bg);
    border-radius: var(--radius-md);
    padding: 12px 14px;
    margin-bottom: 16px;
}

.override-matched__label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-weight: 700;
    color: var(--danger-text);
}

.override-matched__value {
    font-size: 14px;
    font-weight: 600;
    color: var(--accent-text);
    margin-top: 4px;
}

.override-matched__reason {
    font-size: 12.5px;
    color: var(--danger-text);
    margin-top: 2px;
}
</style>

<!-- не scoped: контент BaseModal телепортится в body и несёт data-v самого BaseModal,
     поэтому радиус задаём глобально двойным классом (бьёт scoped .base-modal BaseModal). -->
<style>
.base-modal.bl-override-modal {
    border-radius: 30px;
}
</style>
