<template>
  <BaseModal :show="show" title="Привязать к заявке" width="680px" radius="30px" :z-index="10005"
    :closable="!saving" :close-on-overlay="!saving" content-testid="manual-attach-existing-modal" @close="close">
    <div class="manual-attach-existing" :aria-busy="loading || previewing || saving">
      <strong v-if="title">{{ title }}</strong>
      <p v-if="loading" role="status">Загрузка ручного основания и заявок…</p>
      <div v-if="error" class="manual-attach-existing__error" role="alert">
        {{ error }}
        <button v-if="!saving" type="button" class="lk-button lk-button--ghost lk-button--sm" data-testid="manual-attach-refresh" @click="load">
          Обновить данные
        </button>
      </div>
      <template v-if="context && !loading">
        <p class="manual-attach-existing__note">Будет привязан только открытый {{ kind === 'employee' ? 'человек' : 'автомобиль' }}. Остальные ручные записи не изменятся.</p>
        <p class="manual-attach-existing__current">Текущий срок: {{ periodText(context.effective_period) }}</p>
        <fieldset class="manual-attach-existing__fields" :disabled="busy || conflict">
          <legend class="manual-attach-existing__sr-only">Заявка и основание</legend>
          <FormField label="Заявка" required>
            <BaseDropdown :model-value="applicationID" :options="applicationOptions" label-key="label" value-key="id" searchable
              :disabled="busy || conflict" placeholder="Выберите согласованную заявку в работе" data-testid="manual-attach-application" @update:model-value="selectApplication" />
          </FormField>
          <fieldset class="manual-attach-existing__modes">
            <legend>Куда привязать</legend>
            <label><input v-model="destination" type="radio" value="new" name="manual-attach-destination"> Новое вложение в заявке</label>
            <label><input v-model="destination" type="radio" value="existing" name="manual-attach-destination"> Существующее вложение</label>
          </fieldset>
          <FormField v-if="destination === 'existing'" label="Вложение" required>
            <BaseDropdown v-model="targetID" :options="targetOptions" label-key="label" value-key="id" searchable
              :disabled="busy || conflict || !applicationID || loadingAttachments" placeholder="Выберите вложение того же типа" data-testid="manual-attach-target" />
          </FormField>
          <p v-if="loadingAttachments" role="status">Загрузка вложений…</p>
          <p v-if="kind === 'car' && destination === 'existing'" class="manual-attach-existing__note">
            Машина получит общие места разгрузки целевого вложения. Его места и параметры не изменятся.
          </p>
          <template v-if="context.requires_period_choice">
            <p v-if="!context.can_assign_period" class="manual-attach-existing__error" role="alert">Для назначения конечного срока нужно право изменения срока записи.</p>
            <fieldset class="manual-attach-existing__modes" :disabled="!context.can_assign_period">
              <legend>У бессрочного допуска нужно явно выбрать конечный срок</legend>
              <label><input v-model="periodChoice" type="radio" value="source" name="manual-attach-period"> Срок вложения заявки</label>
              <label><input v-model="periodChoice" type="radio" value="individual" name="manual-attach-period"> Собственный срок</label>
            </fieldset>
            <FormField v-if="periodChoice === 'source'" label="Источник срока" required>
              <BaseDropdown v-model="sourceID" :options="sourceOptions" label-key="label" value-key="id" searchable
                :disabled="busy || conflict || !context.can_assign_period || loadingAttachments" placeholder="Выберите вложение с конечным сроком" data-testid="manual-attach-source" />
            </FormField>
            <DateRangeSection v-if="periodChoice === 'individual' && context.can_assign_period"
              :is-one-day="form.isOneDay" :start-date="form.startDate" :end-date="form.endDate" :single-date="form.singleDate"
              :start-time="form.startTime" :end-time="form.endTime" :errors="validated ? dateErrors : {}" :field-config="$options.fieldConfig"
              @update:is-one-day="form.isOneDay = $event" @update:start-date="form.startDate = $event" @update:end-date="form.endDate = $event"
              @update:single-date="form.singleDate = $event" @update:start-time="form.startTime = $event" @update:end-time="form.endTime = $event" />
          </template>
          <p v-else class="manual-attach-existing__note">Конечный срок открытой записи сохранится; сроки других записей не меняются.</p>
          <FormField label="Причина привязки" required :error="validated && !reason.trim() ? 'Укажите причину' : ''">
            <textarea v-model="reason" class="lk-textarea" rows="2" maxlength="1000" data-testid="manual-attach-reason" />
          </FormField>
        </fieldset>
        <p class="manual-attach-existing__note">Время указано по Москве. После предварительной проверки проверь выбранную заявку и срок перед привязкой.</p>
        <div v-if="preview" class="manual-attach-existing__current" data-testid="manual-attach-preview">
          <strong>{{ preview.destination_mode === 'new_attachment' ? 'Новое вложение' : 'Существующее вложение' }} · {{ selectedApplicationLabel }}</strong>
          <p>Будущий срок: {{ periodText(preview.new_effective) }}</p>
          <p>{{ preview.new_mode === 'individual' ? 'Индивидуальный срок' : 'Срок вложения' }}</p>
          <template v-if="kind === 'car'">
            <p>Признаки до привязки:</p>
            <CarAccessFlagRows :flags="preview.current_flags" />
            <p>Признаки после привязки:</p>
            <CarAccessFlagRows :flags="preview.new_flags" />
            <p>Собственные признаки сохраняются у этой машины. Общие признаки целевого вложения применяются дополнительно; его настройки не меняются.</p>
          </template>
        </div>
      </template>
    </div>
    <template #actions>
      <button type="button" class="lk-button lk-button--ghost" :disabled="saving" @click="close">Отмена</button>
      <button v-if="!preview" type="button" class="lk-button lk-button--primary" :disabled="!canPreview" data-testid="manual-attach-preview-button" @click="check">
        {{ previewing ? 'Проверка…' : 'Проверить привязку' }}
      </button>
      <button v-else type="button" class="lk-button lk-button--primary" :disabled="busy || conflict" data-testid="manual-attach-save" @click="save">
        {{ saving ? 'Привязка…' : 'Привязать' }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';
import FormField from '@/components/ui/FormField.vue';
import DateRangeSection from '@/components/CreateApplication/DateRangeSection.vue';
import CarAccessFlagRows from '@/components/CreateApplication/CarAccessFlagRows.vue';
import { getAttachableApplications } from '@/api/applications';
import { getManualAttachContext, getManualAttachAttachments, previewManualAttachSingle, executeManualAttachSingle } from '@/api/manualAttachSingle';
import { formatPeriod, periodFormFromAttachment, periodFormErrors, periodPayloadFromForm } from '@/utils/entryWindow';
import { useDeletionsStore } from '@/stores/deletions';

export default {
  name: 'ManualAttachExistingModal',
  components: { BaseModal, BaseDropdown, FormField, DateRangeSection, CarAccessFlagRows },
  fieldConfig: { roof_access: { visible: false }, free_parking: { visible: false } },
  props: {
    show: { type: Boolean, default: false }, kind: { type: String, required: true },
    entityID: { type: Number, required: true }, tableID: { type: Number, required: true }, title: { type: String, default: '' },
  },
  emits: ['close', 'changed'],
  data() {
    return { context: null, applications: [], attachments: [], applicationID: null, targetID: null, sourceID: null,
      destination: 'new', periodChoice: '', form: periodFormFromAttachment(null), reason: '', preview: null,
      loading: false, loadingAttachments: false, attachmentsReady: false, previewing: false, saving: false, error: '', conflict: false, validated: false,
      version: 0, attachmentVersion: 0, previewVersion: 0 };
  },
  computed: {
    identity() { return `${this.show}:${this.kind}:${this.entityID}:${this.tableID}`; },
    busy() { return this.loading || this.loadingAttachments || this.previewing || this.saving; },
    applicationOptions() {
      return this.applications.filter(a => a.status === 'В работе' && a.confirmation === 'Согласовано')
        .map(a => ({ id: a.id, label: `${a.application_number || `Заявка ${a.id}`}${a.organization_name ? ' — ' + a.organization_name : ''}` }));
    },
    activeAttachments() { return this.attachments.filter(a => a.status === 1 && !a.is_manual); },
    targetOptions() { return this.activeAttachments.filter(a => a.attachment_type === (this.kind === 'employee' ? 'people' : 'cars')).map(this.attachmentOption); },
    sourceOptions() { return this.activeAttachments.filter(a => a.entry_date_to).map(this.attachmentOption); },
    selectedApplicationLabel() { return this.applicationOptions.find(a => a.id === this.applicationID)?.label || ''; },
    dateErrors() { return this.context?.requires_period_choice && this.periodChoice === 'individual' ? periodFormErrors(this.form) : {}; },
    canPreview() {
      return !!this.context && this.attachmentsReady && !this.busy && !this.conflict && !!this.reason.trim() &&
        this.applicationOptions.some(a => a.id === this.applicationID) &&
        (this.destination === 'new' || this.targetOptions.some(a => a.id === this.targetID)) &&
        (!this.context.requires_period_choice || (this.context.can_assign_period &&
          (this.periodChoice === 'individual' || (this.periodChoice === 'source' && this.sourceOptions.some(a => a.id === this.sourceID)))));
    },
    request() {
      return { ...(this.destination === 'new' ? { application_id: this.applicationID } : { target_attachment_id: this.targetID }),
        reason: this.reason.trim(), ...(this.context?.requires_period_choice ? { period_choice: this.periodChoice,
          ...(this.periodChoice === 'source' ? { source_attachment_id: this.sourceID } : { period: periodPayloadFromForm(this.form) }) } : {}) };
    },
    signature() { return JSON.stringify(this.request); },
  },
  watch: {
    identity: { immediate: true, handler() {
      this.invalidate();
      this.saving = this.loading = this.loadingAttachments = this.previewing = false;
      if (this.show) this.load();
    } },
    signature() { this.preview = null; this.previewVersion++; },
  },
  beforeUnmount() { this.invalidate(); },
  methods: {
    periodText(period) { return period?.bounded ? formatPeriod(period) : 'Бессрочно'; },
    attachmentOption(a) { return { id: a.id, label: `${a.attachment_display_name || a.attachment_name || `Вложение ${a.id}`} · ${formatPeriod(a)}` }; },
    invalidate() { this.version++; this.attachmentVersion++; this.previewVersion++; this.preview = null; this.context = null; },
    close() { if (!this.saving) { this.invalidate(); this.$emit('close'); } },
    async load() {
      if (this.saving || !this.show) return;
      this.invalidate();
      const version = this.version;
      Object.assign(this, { loading: true, loadingAttachments: false, previewing: false, error: '', conflict: false,
        applicationID: null, targetID: null, sourceID: null, destination: 'new', periodChoice: '', reason: '', validated: false, applications: [], attachments: [], attachmentsReady: false });
      try {
        const [context, applications] = await Promise.all([
          getManualAttachContext(this.kind, this.entityID, this.tableID), getAttachableApplications(),
        ]);
        if (version !== this.version) return;
        if (!Array.isArray(applications)) throw new Error('Не удалось прочитать список заявок');
        this.context = context;
        this.applications = applications;
        this.form = periodFormFromAttachment(context.effective_period);
      } catch (error) { if (version === this.version) this.error = error.message || 'Не удалось загрузить данные'; }
      finally { if (version === this.version) this.loading = false; }
    },
    async selectApplication(id) {
      if (this.saving || this.previewing) return;
      this.applicationID = id;
      this.targetID = this.sourceID = null;
      this.periodChoice = '';
      this.attachments = [];
      this.attachmentsReady = false;
      this.preview = null;
      this.error = '';
      const version = this.version, seq = ++this.attachmentVersion;
      this.loadingAttachments = !!id;
      if (!id) return;
      try {
        const attachments = await getManualAttachAttachments(this.kind, this.entityID, this.tableID, id);
        if (version !== this.version || seq !== this.attachmentVersion) return;
        if (!Array.isArray(attachments)) throw new Error('Не удалось прочитать вложения заявки');
        this.attachments = attachments;
        this.attachmentsReady = true;
      } catch (error) { if (version === this.version && seq === this.attachmentVersion) this.error = error.message || 'Не удалось загрузить вложения'; }
      finally { if (version === this.version && seq === this.attachmentVersion) this.loadingAttachments = false; }
    },
    validResult(result, saved = false) {
      return result?.entity_id === this.entityID && result?.entity_kind === this.kind &&
        result?.old_attachment_id === this.context?.attachment_id && result?.application_id === this.applicationID &&
        /^[a-f0-9]{64}$/i.test(result?.revision || '') && result?.new_effective?.bounded === true &&
        ['inherit', 'individual'].includes(result?.new_mode) &&
        result?.destination_mode === (this.destination === 'new' ? 'new_attachment' : 'existing_attachment') &&
        (this.destination === 'existing' ? result?.destination_attachment_id === this.targetID :
          (saved ? Number.isSafeInteger(result?.destination_attachment_id) && result.destination_attachment_id > 0 : result?.destination_attachment_id == null));
    },
    async check() {
      if (!this.canPreview) return;
      this.validated = true;
      if (Object.keys(this.dateErrors).length) return;
      const version = this.version, seq = ++this.previewVersion;
      this.previewing = true;
      this.error = '';
      try {
        const result = await previewManualAttachSingle(this.kind, this.entityID, this.tableID, this.request);
        if (version !== this.version || seq !== this.previewVersion) return;
        if (!this.validResult(result)) throw new Error('Не удалось проверить подтверждение привязки');
        this.preview = result;
      } catch (error) {
        if (version === this.version && seq === this.previewVersion) this.handleError(error);
      } finally { if (version === this.version) this.previewing = false; }
    },
    handleError(error) {
      this.preview = null;
      this.conflict = error.status === 409;
      this.error = this.conflict ? `${error.message}. Обновите данные и проверьте выбор заново; привязка автоматически не повторяется.` : error.message;
    },
    async save() {
      if (!this.preview || this.busy || this.conflict) return;
      const version = this.version;
      this.saving = true;
      try {
        const result = await executeManualAttachSingle(this.kind, this.entityID, this.tableID, { ...this.request, expected_revision: this.preview.revision });
        if (version !== this.version) return;
        if (!this.validResult(result, true)) {
          const error = new Error('Ответ привязки не соответствует открытой записи. Обновите карточку перед дальнейшими действиями');
          error.status = 409;
          throw error;
        }
        useDeletionsStore().notify({ prefix: 'Запись привязана к заявке.', type: 'success' });
        this.$emit('changed', result);
        this.$emit('close');
      } catch (error) { if (version === this.version) this.handleError(error); }
      finally { if (version === this.version) this.saving = false; }
    },
  },
};
</script>

<style scoped>
.manual-attach-existing { display: flex; flex-direction: column; gap: 14px; padding: 16px 20px; min-width: 0; }
.manual-attach-existing__fields { display: flex; flex-direction: column; gap: 14px; border: 0; padding: 0; margin: 0; min-width: 0; }
.manual-attach-existing__modes { display: flex; flex-wrap: wrap; gap: 10px 16px; border: 0; padding: 0; margin: 0; font-size: 13px; }
.manual-attach-existing__modes legend { margin-bottom: 8px; }
.manual-attach-existing__modes label { display: flex; gap: 6px; align-items: center; }
.manual-attach-existing__modes input { accent-color: var(--primary); }
.manual-attach-existing__note { margin: 0; color: var(--text-muted); font-size: 12px; line-height: 1.6; }
.manual-attach-existing__current { margin: 0; padding: 10px 14px; border-radius: var(--radius-md); background: var(--surface-2); font-size: 13px; }
.manual-attach-existing__current p { margin: 8px 0 0; }
.manual-attach-existing__error { display: flex; flex-direction: column; align-items: flex-start; gap: 8px; color: var(--danger-text); font-size: 13px; }
.manual-attach-existing__sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
</style>
