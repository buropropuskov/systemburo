<template>
  <BaseModal
    :show="show"
    title="Изменить срок"
    width="680px"
    radius="30px"
    :z-index="10005"
    :closable="!saving"
    :close-on-overlay="!saving"
    content-testid="entity-period-modal"
    @close="close"
  >
    <div class="entity-period__body" :aria-busy="loading || saving">
      <strong v-if="title">{{ title }}</strong>
      <p v-if="loading" role="status">Загрузка текущего срока…</p>
      <div v-if="error" class="entity-period__error" role="alert">
        {{ error }}
        <button
          v-if="!saving"
          type="button"
          class="lk-button lk-button--ghost lk-button--sm"
          data-testid="entity-period-refresh"
          @click="load"
        >
          Обновить срок
        </button>
      </div>
      <template v-if="snapshot && !loading">
        <div class="entity-period__current" data-testid="entity-period-current">
          Сейчас: {{ currentPeriod }}
          <small>{{ currentSource }}</small>
        </div>
        <fieldset class="entity-period__modes" :disabled="saving || conflict">
          <legend>Как задаётся срок</legend>
          <label>
            <input v-model="mode" type="radio" value="individual" name="entity-period-mode">
            Собственный срок
          </label>
          <label>
            <input v-model="mode" type="radio" value="inherit" name="entity-period-mode">
            {{ manual ? 'Срок ручного основания' : 'Срок вложения' }}
          </label>
        </fieldset>
        <fieldset v-if="mode === 'individual'" class="entity-period__fields" :disabled="saving || conflict">
          <legend class="entity-period__sr-only">Индивидуальный срок</legend>
          <DateRangeSection
            :is-one-day="form.isOneDay"
            :start-date="form.startDate"
            :end-date="form.endDate"
            :single-date="form.singleDate"
            :start-time="form.startTime"
            :end-time="form.endTime"
            :errors="validated ? dateErrors : {}"
            :field-config="$options.fieldConfig"
            @update:is-one-day="form.isOneDay = $event"
            @update:start-date="form.startDate = $event"
            @update:end-date="form.endDate = $event"
            @update:single-date="form.singleDate = $event"
            @update:start-time="form.startTime = $event"
            @update:end-time="form.endTime = $event"
          />
        </fieldset>
        <p v-else class="entity-period__note">
          Собственный срок будет удалён. {{ manual ? 'Будет действовать срок ручного основания; если он не ограничен, допуск останется бессрочным.' : 'Будет действовать актуальный срок вложения.' }}
        </p>
        <FormField label="Причина изменения" required :error="validated && !reason.trim() ? 'Укажите причину изменения' : ''">
          <textarea
            v-model="reason"
            class="lk-textarea"
            rows="2"
            maxlength="1000"
            :disabled="saving || conflict"
            placeholder="Например: продление по обращению"
            data-testid="entity-period-reason"
          />
        </FormField>
        <p class="entity-period__note">
          Время указано по Москве. Другие люди, машины и голоса согласующих не изменятся.
          Автор, причина и прежний срок попадут в историю.
        </p>
      </template>
    </div>
    <template #actions>
      <button type="button" class="lk-button lk-button--ghost" :disabled="saving" @click="close">Отмена</button>
      <button
        type="button"
        class="lk-button lk-button--primary"
        :disabled="!canSave"
        data-testid="entity-period-save"
        @click="save"
      >
        {{ saving ? 'Сохранение…' : 'Сохранить' }}
      </button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue';
import FormField from '@/components/ui/FormField.vue';
import DateRangeSection from '@/components/CreateApplication/DateRangeSection.vue';
import { apiRequest } from '@/api/client';
import { useDeletionsStore } from '@/stores/deletions';
import { formatPeriod, periodFormErrors, periodFormFromAttachment, periodPayloadFromForm } from '@/utils/entryWindow';

export default {
  name: 'EntityPeriodModal',
  components: { BaseModal, FormField, DateRangeSection },
  fieldConfig: { roof_access: { visible: false }, free_parking: { visible: false } },
  props: {
    show: { type: Boolean, default: false },
    kind: { type: String, required: true, validator: value => ['employee', 'car'].includes(value) },
    entityID: { type: Number, required: true },
    tableID: { type: Number, default: null },
    title: { type: String, default: '' },
  },
  emits: ['close', 'changed'],
  data() {
    return {
      snapshot: null, mode: 'individual', form: periodFormFromAttachment(null), reason: '',
      loading: false, saving: false, error: '', conflict: false, validated: false, requestVersion: 0,
    };
  },
  computed: {
    identity() { return `${this.show}:${this.kind}:${this.entityID}:${this.tableID}`; },
    endpoint() {
      if (!['employee', 'car'].includes(this.kind) || !Number.isInteger(this.entityID) || this.entityID <= 0) return '';
      if (this.tableID !== null && (!Number.isInteger(this.tableID) || this.tableID <= 0)) return '';
      return `/${this.kind === 'employee' ? 'employees' : 'cars'}/${this.entityID}/period`;
    },
    manual() { return this.snapshot?.application_id == null; },
    currentPeriod() { return this.snapshot.effective_period.bounded ? formatPeriod(this.snapshot.effective_period) : 'Бессрочно'; },
    currentSource() {
      return { individual: 'Индивидуальный срок', attachment: 'Наследуется от вложения', manual_unbounded: 'Ручной бессрочный допуск' }[this.snapshot.effective_period.source] || 'Срок основания';
    },
    dateErrors() { return this.mode === 'individual' ? periodFormErrors(this.form) : {}; },
    canSave() { return !!this.snapshot && !!this.endpoint && !this.loading && !this.saving && !this.conflict && !this.error && !!this.reason.trim(); },
  },
  watch: {
    identity: {
      immediate: true,
      handler() {
        this.requestVersion++;
        this.snapshot = null;
        this.reason = '';
        this.error = '';
        this.conflict = false;
        this.loading = false;
        if (this.show) this.load();
      },
    },
  },
  beforeUnmount() { this.requestVersion++; },
  methods: {
    close() { if (!this.saving) this.$emit('close'); },
    async load() {
      const version = ++this.requestVersion;
      this.snapshot = null;
      this.error = '';
      this.conflict = false;
      this.validated = false;
      if (!this.endpoint) { this.error = 'Некорректная запись или таблица'; return; }
      this.loading = true;
      try {
        const query = this.tableID === null ? '' : `?table_id=${this.tableID}`;
        const response = await apiRequest(this.endpoint + query);
        const data = await response.json(); // client.js уже раскрывает envelope.data.
        if (version !== this.requestVersion) return;
        if (!response.ok) throw new Error(response.status >= 500 ? 'Не удалось загрузить срок' : (data?.message || 'Не удалось загрузить срок'));
        if (!data || data.entity_id !== this.entityID || !/^[a-f0-9]{64}$/i.test(data.period_revision || '') ||
            !['inherit', 'individual'].includes(data.period_mode) || typeof data.effective_period?.bounded !== 'boolean') {
          throw new Error('Не удалось прочитать текущую версию срока');
        }
        this.snapshot = data;
        this.mode = data.period_mode;
        this.form = periodFormFromAttachment(data.effective_period);
      } catch (error) {
        if (version === this.requestVersion) this.error = error.message || 'Не удалось загрузить срок';
      } finally {
        if (version === this.requestVersion) this.loading = false;
      }
    },
    async save() {
      if (!this.canSave) return;
      this.validated = true;
      if (Object.keys(this.dateErrors).length) return;
      const version = this.requestVersion;
      this.saving = true;
      try {
        const body = {
          period_mode: this.mode,
          reason: this.reason.trim(),
          expected_revision: this.snapshot.period_revision,
          ...(this.mode === 'individual' ? { period: periodPayloadFromForm(this.form) } : {}),
          ...(this.tableID === null ? {} : { table_id: this.tableID }),
        };
        const response = await apiRequest(this.endpoint, { method: 'PUT', body: JSON.stringify(body) });
        const result = await response.json();
        if (version !== this.requestVersion) return;
        if (response.status === 409) {
          this.conflict = true;
          this.error = 'Срок или состояние записи изменились. Обновите срок и проверьте новые значения перед сохранением. Введённые даты будут заменены актуальными.';
          return;
        }
        if (!response.ok) throw new Error(response.status >= 500 ? 'Не удалось изменить срок' : (result?.message || 'Не удалось изменить срок'));
        useDeletionsStore().notify({ prefix: 'Срок изменён. Голоса согласующих сохранены.', type: 'success' });
        this.$emit('changed', result);
        this.$emit('close');
      } catch (error) {
        if (version === this.requestVersion) this.error = error.message || 'Не удалось изменить срок';
      } finally { this.saving = false; }
    },
  },
};
</script>

<style scoped>
.entity-period__body { display: flex; flex-direction: column; gap: 14px; padding: 16px 20px; min-width: 0; }
.entity-period__current { padding: 10px 14px; border-radius: var(--radius-md); background: var(--surface-2); font-size: 13px; }
.entity-period__current small { display: block; margin-top: 6px; color: var(--text-muted); }
.entity-period__modes, .entity-period__fields { border: 0; padding: 0; margin: 0; min-width: 0; }
.entity-period__modes { display: flex; flex-wrap: wrap; gap: 10px 16px; font-size: 13px; }
.entity-period__modes legend { margin-bottom: 8px; }
.entity-period__modes label { display: flex; gap: 6px; align-items: center; }
.entity-period__modes input { accent-color: var(--primary); }
.entity-period__note { margin: 0; color: var(--text-muted); font-size: 12px; line-height: 1.6; }
.entity-period__error { display: flex; flex-direction: column; align-items: flex-start; gap: 8px; color: var(--danger-text); font-size: 13px; }
.entity-period__sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
</style>
