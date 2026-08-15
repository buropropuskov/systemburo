<template>
  <BaseModal
    :show="show"
    title="Убрать из заявки?"
    width="460px"
    :z-index="10005"
    content-class="el-removal-modal"
    @close="$emit('close')"
  >
    <div class="removal-body">
      <p class="removal-lead">
        Строка перестанет действовать: она уйдёт из таблиц постов и пропуск по ней
        работать не будет. Сама заявка останется с историей - видно, кто и почему убрал.
      </p>

      <div
        v-if="label"
        class="removal-target"
        data-testid="removal-target"
      >
        {{ label }}
      </div>

      <FormField
        label="Причина удаления"
        required
      >
        <textarea
          v-model="reason"
          class="lk-textarea"
          rows="3"
          maxlength="1000"
          placeholder="Напишите причину"
          data-testid="removal-reason"
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
        class="lk-button lk-button--danger"
        :disabled="!canConfirm"
        data-testid="removal-confirm"
        @click="submit"
      >
        Убрать из заявки
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue'
import FormField from '@/components/ui/FormField.vue'

export default {
    name: 'ElementRemovalModal',
    components: { BaseModal, FormField },
    props: {
        show: {
            type: Boolean,
            required: true
        },
        /** Как называется убираемая строка: ФИО либо номер машины. */
        label: {
            type: String,
            default: ''
        },
        submitting: {
            type: Boolean,
            default: false
        }
    },
    emits: ['confirm', 'close'],
    data() {
        return {
            reason: ''
        }
    },
    computed: {
        canConfirm() {
            return !this.submitting && this.reason.trim().length > 0
        }
    },
    watch: {
        show(value) {
            // Причина не переносится на следующую строку: пояснение к одной машине
            // к другой не относится.
            if (value) this.reason = ''
        }
    },
    methods: {
        submit() {
            if (!this.canConfirm) return
            this.$emit('confirm', this.reason.trim())
        }
    }
}
</script>

<style scoped>
.removal-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 20px;
}

.removal-lead {
  margin: 0;
  font-size: 13px;
  line-height: 1.45;
  color: var(--text-muted);
}

.removal-target {
  padding: 10px 14px;
  border-radius: var(--radius-md);
  background: var(--danger-bg);
  color: var(--danger-text);
  font-weight: 600;
  font-size: 14px;
}
</style>

<!-- не scoped: см. BlacklistOverrideModal - контент телепортится в body. -->
<style>
.base-modal.el-removal-modal {
  border-radius: 30px;
}
</style>
