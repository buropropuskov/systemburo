<template>
  <div
    v-if="canEdit"
    class="dates-editor"
  >
    <button
      type="button"
      class="lk-button lk-button--secondary lk-button--sm dates-editor__open"
      data-testid="app-detail-change-dates"
      @click="open"
    >
      Изменить срок
    </button>

    <BaseModal
      :show="show"
      title="Изменить срок заявки"
      width="560px"
      radius="30px"
      :z-index="10005"
      content-class="dates-editor-modal"
      content-testid="dates-editor-modal"
      @close="close"
    >
      <div class="dates-editor__body">
        <p class="dates-editor__lead">
          Новый срок ляжет на все вложения и машины заявки. Участники получат уведомление,
          а голоса согласующих, если они уже есть, будут сняты: одобряли другой срок.
        </p>

        <div
          class="dates-editor__current"
          data-testid="dates-editor-current"
        >
          Сейчас: {{ currentPeriods }}
        </div>

        <DateRangeSection
          :is-one-day="form.isOneDay"
          :start-date="form.startDate"
          :end-date="form.endDate"
          :single-date="form.singleDate"
          :start-time="form.startTime"
          :end-time="form.endTime"
          :errors="shownErrors"
          :field-config="$options.fieldConfig"
          @update:is-one-day="form.isOneDay = $event"
          @update:start-date="form.startDate = $event"
          @update:end-date="form.endDate = $event"
          @update:single-date="form.singleDate = $event"
          @update:start-time="form.startTime = $event"
          @update:end-time="form.endTime = $event"
        />

        <FormField
          label="Причина изменения"
          required
        >
          <textarea
            v-model="reason"
            class="lk-textarea"
            rows="3"
            maxlength="1000"
            placeholder="Например: заявитель ошибся датой"
            data-testid="dates-editor-reason"
          />
        </FormField>
      </div>

      <template #actions>
        <button
          type="button"
          class="lk-button lk-button--ghost"
          :disabled="submitting"
          @click="close"
        >
          Отмена
        </button>
        <button
          type="button"
          class="lk-button lk-button--primary"
          :disabled="!canSubmit"
          data-testid="dates-editor-save"
          @click="submit"
        >
          Сохранить
        </button>
      </template>
    </BaseModal>
  </div>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue'
import FormField from '@/components/ui/FormField.vue'
import DateRangeSection from '@/components/CreateApplication/DateRangeSection.vue'
import { changeApplicationDates } from '@/api/applicationAssignments'
import { useDeletionsStore } from '@/stores/deletions'
import {
    canEditApplicationDates,
    formatPeriod,
    periodFormErrors,
    periodFormFromAttachment,
    periodPayloadFromForm
} from '@/utils/entryWindow'

/**
 * Правка срока заявки принимающим (#2575): кнопка и окно с тем же блоком дат, что в
 * форме подачи. Сам решает, показывать ли кнопку, чтобы карточка заявки, которая
 * уже за порогом размера, не несла ещё одно вычисляемое свойство.
 */
export default {
    name: 'ApplicationDatesEditor',
    components: { BaseModal, FormField, DateRangeSection },
    // Крыша и парковка живут в том же блоке, но к сроку не относятся.
    fieldConfig: {
        roof_access: { visible: false },
        free_parking: { visible: false }
    },
    props: {
        application: {
            type: Object,
            default: null
        },
        attachments: {
            type: Array,
            default: () => []
        },
        isApprover: {
            type: Boolean,
            default: false
        }
    },
    emits: ['changed'],
    data() {
        return {
            show: false,
            form: periodFormFromAttachment(null),
            reason: '',
            submitting: false,
            validated: false
        }
    },
    computed: {
        canEdit() {
            return this.isApprover && this.attachments.length > 0 && canEditApplicationDates(this.application)
        },
        currentPeriods() {
            return [...new Set(this.attachments.map(formatPeriod))].join('; ')
        },
        errors() {
            return periodFormErrors(this.form)
        },
        // Ошибки показываем после первой попытки сохранить: пустые поля при открытии
        // окна не ошибка, а недописанный ввод.
        shownErrors() {
            return this.validated ? this.errors : {}
        },
        canSubmit() {
            return !this.submitting && this.reason.trim().length > 0
        }
    },
    methods: {
        open() {
            this.form = periodFormFromAttachment(this.attachments[0])
            this.reason = ''
            this.validated = false
            this.show = true
        },
        close() {
            if (this.submitting) return
            this.show = false
        },
        async submit() {
            if (!this.canSubmit) return
            this.validated = true
            if (Object.keys(this.errors).length) return

            this.submitting = true
            const notify = useDeletionsStore().notify
            try {
                const result = await changeApplicationDates(this.application.id, {
                    ...periodPayloadFromForm(this.form),
                    reason: this.reason.trim()
                })
                notify({
                    prefix: 'Срок заявки изменён:',
                    bold: result?.new_period || '',
                    suffix: result?.approvals_reset ? 'Голоса согласующих сняты.' : '',
                    type: 'success'
                })
                this.show = false
                this.$emit('changed')
            } catch (error) {
                notify({ bold: error.message || 'Не удалось изменить срок заявки', type: 'error' })
            } finally {
                this.submitting = false
            }
        }
    }
}
</script>

<style scoped>
.dates-editor {
  padding-bottom: 12px;
}

.dates-editor__open {
  width: 100%;
}

.dates-editor__body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px 20px;
}

.dates-editor__lead {
  margin: 0;
  font-size: 13px;
  line-height: 1.45;
  color: var(--text-muted);
}

.dates-editor__current {
  padding: 10px 14px;
  border-radius: var(--radius-md);
  background: var(--surface-2);
  color: var(--text);
  font-weight: 600;
  font-size: 14px;
}
</style>

