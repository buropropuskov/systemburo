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
      title="Изменить срок вложений"
      width="560px"
      radius="30px"
      :z-index="10005"
      content-class="dates-editor-modal"
      content-testid="dates-editor-modal"
      @close="close"
    >
      <div class="dates-editor__body">
        <p class="dates-editor__lead">
          Новый срок применяется к выбранным вложениям, людям и машинам, которые наследуют их срок. Участники получат уведомление,
          а поданные голоса согласующих сохранятся. Изменение и его автор будут записаны в историю.
        </p>

        <div
          class="dates-editor__current"
          data-testid="dates-editor-current"
        >
          Сейчас: {{ currentPeriods }}
        </div>

        <fieldset class="dates-editor__fields" :disabled="busy">
          <legend>Выберите вложения</legend>
          <label class="dates-editor__choice">
            <input type="checkbox" :checked="allSelected" data-testid="dates-editor-all" @change="selectAll($event.target.checked)"> Все вложения
          </label>
          <div class="dates-editor__attachments">
            <label v-for="attachment in selectableAttachments" :key="attachment.id" class="dates-editor__choice">
              <input v-model="selectedIDs" type="checkbox" :value="attachment.id" :data-testid="`dates-editor-attachment-${attachment.id}`">
              <span>{{ attachment.attachment_display_name || attachment.attachment_name || `Вложение №${attachment.id}` }}
                <small>{{ formatPeriod(attachment) }}</small></span>
            </label>
          </div>
          <p v-if="!selectedIDs.length" class="dates-editor__error" role="alert">Выберите хотя бы одно вложение.</p>
        </fieldset>
        <fieldset class="dates-editor__fields" :disabled="busy">
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

        <FormField label="Индивидуальные сроки">
          <select v-model="individualPolicy" class="lk-select" data-testid="dates-editor-policy">
            <option value="preserve">Сохранить индивидуальные сроки</option>
            <option value="replace">Заменить индивидуальные сроки новым сроком</option>
          </select>
        </FormField>
        <p class="dates-editor__lead">При замене индивидуальный режим сохранится. Вернуть наследование можно отдельно в карточке человека или машины.</p>
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
        </fieldset>
        <div v-if="snapshot" class="dates-editor__current" data-testid="dates-editor-preview" aria-live="polite">
          Вложений: {{ snapshot.attachment_count }}. Людей: {{ snapshot.employee_count }}. Машин: {{ snapshot.car_count }}.
          Индивидуальных сроков: {{ snapshot.individual_count }} — {{ individualPolicy === 'preserve' ? 'сохранятся' : 'будут заменены' }}.
          <small>Новый срок: {{ formatPeriod(snapshot.new_period) }}</small>
        </div>
        <p v-if="error" class="dates-editor__error" role="alert">{{ error }}</p>
      </div>

      <template #actions>
        <button
          type="button"
          class="lk-button lk-button--ghost"
          :disabled="busy"
          @click="close"
        >
          Отмена
        </button>
        <button v-if="!snapshot" type="button" class="lk-button lk-button--primary" :disabled="!canPreview"
          data-testid="dates-editor-preview-button" @click="preview">{{ conflict ? 'Обновить данные' : 'Проверить изменения' }}</button>
        <button v-else
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
import { previewApplicationPeriods, changeApplicationPeriods } from '@/api/applicationPeriodCommands'
import { usePermissionsStore } from '@/stores/permissions'
import { useDeletionsStore } from '@/stores/deletions'
import {
    canEditApplicationDates,
    formatPeriod,
    periodFormErrors,
    periodFormFromAttachment,
    periodPayloadFromForm
} from '@/utils/entryWindow'

// Selected attachment periods use managed rights and a server preview revision.
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
            validated: false,
            selectedIDs: [],
            individualPolicy: 'preserve',
            snapshot: null,
            loading: false,
            error: '',
            conflict: false,
            requestVersion: 0
        }
    },
    computed: {
        selectableAttachments() { return this.attachments.filter(item => Number.isSafeInteger(item.id) && item.id > 0) },
        canEdit() {
            return Number.isSafeInteger(this.application?.id) && this.application.id > 0 && this.selectableAttachments.length > 0 &&
                usePermissionsStore().hasPermission('application.period.change') && canEditApplicationDates(this.application)
        },
        busy() { return this.loading || this.submitting },
        allSelected() { return this.selectableAttachments.length > 0 && this.selectedIDs.length === this.selectableAttachments.length },
        currentPeriods() {
            const periods = this.snapshot ? this.snapshot.attachments.map(item => item.old_period)
                : this.selectableAttachments.filter(item => this.selectedIDs.includes(item.id))
            return [...new Set(periods.map(formatPeriod))].join('; ') || '—'
        },
        errors() {
            return periodFormErrors(this.form)
        },
        // Ошибки показываем после первой попытки сохранить: пустые поля при открытии
        // окна не ошибка, а недописанный ввод.
        shownErrors() {
            return this.validated ? this.errors : {}
        },
        canPreview() { return this.canEdit && this.show && !this.busy && this.selectedIDs.length > 0 && !!this.reason.trim() },
        canSubmit() { return this.canPreview && !!this.snapshot && !this.conflict },
        requestBody() {
            return { attachment_ids: [...this.selectedIDs].sort((a, b) => a - b), period: periodPayloadFromForm(this.form),
                individual_policy: this.individualPolicy, reason: this.reason.trim() }
        },
        requestKey() { return JSON.stringify([this.application?.id, this.requestBody]) },
        identity() { return JSON.stringify([this.canEdit, this.application?.id, this.attachments]) }
    },
    watch: {
        requestKey() { this.invalidate() },
        identity() { this.invalidate(); this.show = false }
    },
    beforeUnmount() { this.requestVersion++ },
    methods: {
        formatPeriod,
        invalidate() { this.requestVersion++; this.snapshot = null; this.error = ''; this.conflict = false; this.loading = false },
        selectAll(checked) { this.selectedIDs = checked ? this.selectableAttachments.map(item => item.id) : [] },
        open() {
            if (!this.canEdit) return
            this.invalidate()
            this.form = periodFormFromAttachment(this.selectableAttachments[0])
            this.selectAll(true)
            this.individualPolicy = 'preserve'
            this.reason = ''
            this.validated = false
            this.show = true
        },
        close() {
            if (this.busy) return
            this.invalidate()
            this.show = false
        },
        async preview() {
            if (!this.canPreview) return
            this.validated = true
            if (Object.keys(this.errors).length) return
            const version = ++this.requestVersion
            this.loading = true
            this.error = ''
            this.snapshot = null
            try {
                const result = await previewApplicationPeriods(this.application.id, this.requestBody)
                if (version !== this.requestVersion || !this.canEdit || !this.show) return
                this.snapshot = result
                this.conflict = false
            } catch (error) {
                if (version === this.requestVersion) this.error = error.message || 'Не удалось проверить изменения'
            } finally {
                if (version === this.requestVersion) this.loading = false
            }
        },
        async submit() {
            if (!this.canSubmit) return
            this.validated = true
            if (Object.keys(this.errors).length) return

            const version = this.requestVersion
            this.submitting = true
            this.error = ''
            const notify = useDeletionsStore().notify
            try {
                const result = await changeApplicationPeriods(this.application.id, {
                    ...this.requestBody,
                    expected_revision: this.snapshot.period_revision
                })
                if (version !== this.requestVersion || !this.canEdit || !this.show) return
                notify({
                    prefix: 'Срок выбранных вложений изменён. Голоса согласующих сохранены.',
                    type: 'success'
                })
                this.show = false
                this.snapshot = null
                this.$emit('changed', result)
            } catch (error) {
                if (version !== this.requestVersion) return
                this.snapshot = null
                this.conflict = error.status === 409
                this.error = this.conflict ? 'Срок или состав заявки изменились. Обновите данные и проверьте изменения перед сохранением.'
                    : (error.message || 'Не удалось изменить срок вложений')
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
  min-width: 0;
}
.dates-editor__fields { border: 0; padding: 0; margin: 0; min-width: 0; display: flex; flex-direction: column; gap: 10px; }
.dates-editor__fields legend { margin-bottom: 8px; }
.dates-editor__attachments { max-height: 140px; overflow-y: auto; }
.dates-editor__choice { display: flex; gap: 8px; align-items: flex-start; font-size: 13px; padding: 4px 0; }
.dates-editor__choice input { accent-color: var(--primary); }
.dates-editor__current small, .dates-editor__choice small { display: block; font-weight: normal; color: var(--text-muted); }
.dates-editor__error { margin: 0; color: var(--danger-text); font-size: 13px; }

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

