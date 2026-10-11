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
      width="680px"
      radius="30px"
      :z-index="10005"
      content-class="dates-editor-modal"
      content-testid="dates-editor-modal"
      @close="close"
    >
      <div class="dates-editor__body">
        <p class="dates-editor__lead">
          Новый срок получат выбранные вложения и наследующие его записи. Участники будут уведомлены; голоса согласующих сохранятся.
        </p>

        <fieldset class="dates-editor__fields" :disabled="busy">
          <legend class="dates-editor__selection-heading"><span>Вложения для изменения</span>
          <ToggleSwitch :model-value="allSelected" :disabled="busy" data-testid="dates-editor-all" @update:model-value="selectAll">Все вложения</ToggleSwitch></legend>
          <div class="dates-editor__attachments" data-testid="dates-editor-current">
            <div v-for="attachment in selectableAttachments" :key="attachment.id" class="dates-editor__choice">
              <ToggleSwitch :model-value="selectedIDs.includes(attachment.id)" :disabled="busy" :data-testid="`dates-editor-attachment-${attachment.id}`" @update:model-value="toggleAttachment(attachment.id, $event)">
              <span>{{ attachment.attachment_display_name || attachment.attachment_name || `Вложение №${attachment.id}` }}
                <small>{{ formatPeriod(attachment) }}</small></span></ToggleSwitch>
            </div>
          </div>
          <p v-if="!selectedIDs.length" class="dates-editor__error" role="alert">Выберите хотя бы одно вложение.</p>
        </fieldset>
        <fieldset class="dates-editor__fields" :disabled="busy">
        <p v-if="mixedPeriods" class="dates-editor__mixed"><span aria-hidden="true">ⓘ</span> Сроки различаются. Укажите новый общий срок или выберите одно вложение. Введённые даты сохранятся.</p>
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

        <div class="dates-editor__policy-title">Индивидуальные сроки <HintTooltip :text="policyHint" /></div><FormField>
          <BaseDropdown v-model="individualPolicy" :options="policyOptions" label-key="label" value-key="id" :disabled="busy" teleport :menu-z-index="10007" data-testid="dates-editor-policy" />
        </FormField>

        <FormField
          label="Причина изменения"
          required
        >
          <textarea
            v-model="reason"
            class="lk-textarea"
            rows="2"
            maxlength="1000"
            placeholder="Например: заявитель ошибся датой"
            data-testid="dates-editor-reason"
          />
        </FormField>
        </fieldset>

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
        <button type="button" class="lk-button lk-button--primary" :disabled="!canPreview"
          data-testid="dates-editor-preview-button" @click="preview">{{ conflict ? 'Обновить данные' : 'Проверить изменения' }}</button>

      </template>
    </BaseModal>
    <BaseModal :show="show && !!snapshot" title="Подтвердить изменение срока" width="580px" radius="30px" :z-index="10006" :closable="!busy" :close-on-overlay="!busy" content-testid="dates-editor-confirmation" @close="snapshot = null">
      <div v-if="snapshot" class="dates-editor__body" data-testid="dates-editor-preview">
        <div v-for="item in snapshot.attachments" :key="item.attachment_id" class="dates-editor__current">
          <strong>{{ attachmentLabel(item.attachment_id) }}</strong>
          <small>Было: {{ formatPeriod(item.old_period) }}</small>
          <small>Станет: {{ formatPeriod(snapshot.new_period) }}</small>
        </div>
        <div class="confirmation-stats"><div><span>Людей</span><strong>{{ snapshot.employee_count }}</strong></div><div><span>Машин</span><strong>{{ snapshot.car_count }}</strong></div><div><span>Индивидуальных сроков</span><strong>{{ snapshot.individual_count }}</strong><small>{{ individualPolicy === 'preserve' ? 'Сохранятся' : 'Будут заменены' }}</small></div></div>
        <div class="confirmation-reason"><span>Причина изменения</span><p>{{ reason }}</p></div>
        <div class="confirmation-policy">✓ Голоса согласующих сохранятся <span>Время указано по Москве</span></div>
      </div>
      <template #actions>
        <button type="button" class="lk-button lk-button--ghost" :disabled="busy" @click="snapshot = null">Назад</button>
        <button type="button" class="lk-button lk-button--primary" :disabled="!canSubmit" data-testid="dates-editor-save" @click="submit">{{ submitting ? 'Сохранение…' : 'Подтвердить' }}</button>
      </template>
    </BaseModal>
  </div>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue'
import FormField from '@/components/ui/FormField.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'
import BaseDropdown from '@/components/ui/BaseDropdown.vue'
import HintTooltip from '@/components/ui/HintTooltip.vue'
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
    components: { BaseModal, FormField, DateRangeSection, ToggleSwitch, BaseDropdown, HintTooltip },
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
            formSourceKey: '',
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
            return this.selectableAttachments.filter(item => this.selectedIDs.includes(item.id))
                .map(item => ({ id: item.id, label: this.attachmentLabel(item.id), period: formatPeriod(item) }))
        },
        policyOptions() { return [{ id: 'preserve', label: 'Сохранить индивидуальные сроки' }, { id: 'replace', label: 'Заменить индивидуальные сроки' }] },
        policyHint() { return this.individualPolicy === 'preserve'
            ? 'Собственные сроки не изменятся: если вложение до 18 октября, а человек до 20 октября, у него останется 20 октября.'
            : 'Собственные даты заменятся новыми. Индивидуальный режим сохранится; наследование можно включить в карточке.' },
        mixedPeriods() { return new Set(this.currentPeriods.map(item => item.period)).size > 1 },
        formDirty() { return JSON.stringify(this.form) !== this.formSourceKey },
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
        selectedIDs() { if (this.show && !this.formDirty) this.syncFormSource() },
        identity() { this.invalidate(); this.show = false }
    },
    beforeUnmount() { this.requestVersion++ },
    methods: {
        formatPeriod,
        invalidate() { this.requestVersion++; this.snapshot = null; this.error = ''; this.conflict = false; this.loading = false },
        attachmentLabel(id) { const item = this.selectableAttachments.find(item => item.id === id); return item?.attachment_display_name || item?.attachment_name || `Вложение №${id}` },
        toggleAttachment(id, checked) { this.selectedIDs = checked ? [...new Set([...this.selectedIDs, id])] : this.selectedIDs.filter(value => value !== id) },
        syncFormSource() {
            const selected = this.selectableAttachments.filter(item => this.selectedIDs.includes(item.id))
            const periods = new Set(selected.map(formatPeriod))
            this.form = periodFormFromAttachment(periods.size === 1 ? selected[0] : null)
            this.formSourceKey = JSON.stringify(this.form)
        },
        selectAll(checked) { this.selectedIDs = checked ? this.selectableAttachments.map(item => item.id) : [] },
        open() {
            if (!this.canEdit) return
            this.invalidate()
            this.selectAll(true)
            this.syncFormSource()
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
  padding: 16px 20px;
  min-width: 0;
  gap: 14px;
}
.dates-editor__fields {
  border: 0;
  padding: 0;
  margin: 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.dates-editor__fields legend {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  margin-bottom: 8px;
}
.dates-editor__attachments {
  overflow-y: auto;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  max-height: 160px;
  gap: 8px;
}
.dates-editor__choice {
  display: flex;
  gap: 8px;
  font-size: 13px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  border-radius: var(--radius-md);
  align-items: center;
  padding: 9px 10px;
}
.dates-editor__summary {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 8px 16px;
  margin: 0;
  font-size: 13px;
}
.dates-editor__summary dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.dates-editor__period-row {
  display: grid;
  align-items: center;
  grid-template-columns: minmax(140px,1fr) minmax(0,2fr);
  gap: 12px;
}
.dates-editor__body :deep(.lk-textarea) {
  height: 55px;
  min-height: 55px;
}
@media(max-width:600px) {
.dates-editor__attachments {
  grid-template-columns: 1fr;
}
.dates-editor__period-row {
  grid-template-columns: 1fr;
  gap: 2px;
} }
.dates-editor__period-row + .dates-editor__period-row {
  margin-top: 8px;
}
.dates-editor__current small, .dates-editor__choice small {
  display: block;
  font-weight: normal;
  color: var(--text-muted);
}
.dates-editor__error {
  margin: 0;
  color: var(--danger-text);
  font-size: 13px;
}
.dates-editor__lead {
  margin: 0;
  line-height: 1.45;
  color: var(--text-muted);
  font-size: 12px;
}
.dates-editor__current {
  border-radius: var(--radius-md);
  color: var(--text);
  padding: 10px 12px;
  background: transparent;
  border: 1px solid var(--border);
  font-size: 12px;
  font-weight: 400;
}
.dates-editor__period-row strong {
  font-weight: 500;
  font-size: 12px
}
.dates-editor__period-row small {
  font-size: 11px;
  line-height: 1.5
}
.dates-editor__period-row+.dates-editor__period-row {
  margin-top: 0;
  border-top: 1px solid var(--border);
  padding-top: 8px;
  margin-top: 8px
}
.dates-editor__choice :deep(.toggle-switch) {
  width: 100%;
  align-items: flex-start
}
.dates-editor__mixed {
  display: flex;
  gap: 8px;
  margin: 0;
  background: var(--surface-2);
  border-left: 3px solid var(--accent);
  border-radius: 8px;
  color: var(--text-muted);
  line-height: 1.5;
  font-size: 11px;
  padding: 8px 10px
}
.dates-editor__policy-title {
  display: flex;
  gap: 8px;
  align-items: center;
  font-size: 13px;
  margin-top: 8px
}
.dates-editor__body :deep(.form-field) {
  margin-bottom: 0
}
.dates-editor__body :deep(.date-range-section) {
  gap: 14px
}
.dates-editor__body :deep(.toggle-container) {
  font-size: 12px
}
.dates-editor__choice span {
  font-size: 12px;
  line-height: 1.4
}
.dates-editor__choice small {
  margin-top: 4px;
  font-size: 10px;
  line-height: 1.5;
  font-weight: 400;
  color: var(--text-muted)
}
.dates-editor__body :deep(.form-field__label) {
  font-size: 12px
}
.dates-editor__body :deep(.date__label-row) {
  font-size: 12px
}
.dates-editor__selection-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
  font-size: 12px;
  color: var(--text-muted)
}
.confirmation-stats {
  display: grid;
  grid-template-columns: 1fr 1fr 2fr;
  gap: 10px
}
.confirmation-stats>div {
  border: 1px solid var(--border);
  border-radius: 15px;
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  background: var(--surface-2)
}
.confirmation-stats span,.confirmation-stats small {
  font-size: 12px;
  color: var(--text-muted)
}
.confirmation-stats strong {
  font-size: 18px
}
.confirmation-reason {
  padding: 12px;
  border-radius: 15px;
  border: 1px solid var(--border)
}
.confirmation-reason>span {
  font-size: 12px;
  color: var(--text-muted)
}
.confirmation-reason p {
  margin: 8px 0 0;
  font-size: 13px
}
.confirmation-policy {
  padding: 10px 12px;
  border-radius: 15px;
  background: var(--surface-2);
  font-size: 12px;
  display: flex;
  flex-direction: column;
  gap: 5px
}
.confirmation-policy span {
  font-size: 11px;
  color: var(--text-muted)
}
</style>
