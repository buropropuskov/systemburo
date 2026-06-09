<template>
  <BaseModal
    :show="show"
    title="Всё равно пропустить?"
    width="460px"
    @close="$emit('close')"
  >
    <div class="override-body">
      <p class="override-lead">
        Элемент похож на активную запись чёрного списка, но это не точное совпадение.
        Подтвердите пропуск - действие фиксируется в аудите (кто, когда, причина).
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
          Причина совпадения: {{ flag.matched_reason }}
        </div>
      </div>

      <label
        class="override-field-label"
        for="blacklist-override-comment"
      >
        Причина пропуска
      </label>
      <textarea
        id="blacklist-override-comment"
        v-model="comment"
        class="lk-textarea"
        rows="3"
        placeholder="Например: проверено по СТС, это другой автомобиль"
        @keydown.enter.ctrl="submit"
      />
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

export default {
    name: 'BlacklistOverrideModal',
    components: { BaseModal },
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
    color: #666;
}

.override-matched {
    border: 1px solid #fecaca;
    background: #fff5f5;
    border-radius: 14px;
    padding: 12px 14px;
    margin-bottom: 16px;
}

.override-matched__label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-weight: 700;
    color: #991b1b;
}

.override-matched__value {
    font-size: 14px;
    font-weight: 600;
    color: #1f2330;
    margin-top: 4px;
}

.override-matched__reason {
    font-size: 12.5px;
    color: #7f1d1d;
    margin-top: 2px;
}

.override-field-label {
    display: block;
    font-size: 12.5px;
    font-weight: 600;
    color: #333;
    margin-bottom: 6px;
}
</style>
