<template>
  <div v-if="visible" class="header-actions">
    <ManualAttachCardAction
      :show="show"
      :kind="kind"
      :entityID="manualEntityID"
      :tableID="periodTableId"
      :readonly="readonly"
      @changed="$emit('manual-attached', $event)"
    />
    <EntityPeriodEditor
      :show="show"
      :kind="kind"
      :entity="entity"
      :source="source"
      :readonly="readonly"
      :table-id="periodTableId"
      :license-plate-formats="licensePlateFormats"
      @changed="$emit('period-changed', $event)"
    />
    <button v-if="historyVisible" class="history-btn" @click="$emit('history')">
      <span>Полная история</span>
    </button>
    <button v-if="applicationVisible" class="application-btn" @click="$emit('application')">
      <span>Открыть заявку</span>
    </button>
    <button v-if="blacklistVisible" class="blacklist-add-btn" @click="$emit('blacklist')">
      <span>В ЧС</span>
    </button>
  </div>
</template>

<script>
import EntityPeriodEditor from '@/components/EntityPeriodEditor.vue';
import ManualAttachCardAction from '@/components/ManualAttachCardAction.vue';

// Keep the existing title contract, including its directly tested computed caller.
export function detailHeaderTitle(kind, narrow, actionCount) {
  if (narrow || actionCount >= 2) return 'Информация';
  if (actionCount === 1) return 'Детальная информация';
  return kind === 'employee' ? 'Детальная информация о сотруднике' : 'Детальная информация о Т/С';
}

export default {
  name: 'DetailHeaderActions',
  components: { EntityPeriodEditor, ManualAttachCardAction },
  props: {
    visible: { type: Boolean, default: true },
    show: { type: Boolean, default: false },
    kind: { type: String, required: true, validator: value => ['employee', 'car'].includes(value) },
    entity: { type: Object, default: null },
    source: { type: String, default: 'general' },
    readonly: { type: Boolean, default: false },
    periodTableId: { type: Number, default: null },
    licensePlateFormats: { type: Array, default: () => [] },
    historyVisible: { type: Boolean, default: false },
    applicationVisible: { type: Boolean, default: false },
    blacklistVisible: { type: Boolean, default: false },
  },
  emits: ['history', 'application', 'blacklist', 'period-changed', 'manual-attached'],
  computed: {
    manualEntityID() {
      if (!['peopletable', 'carstable'].includes(this.source) || this.entity?.isDraft || this.entity?.isPending) return null;
      const id = this.entity?.id;
      return Number.isSafeInteger(id) && id > 0 ? id : null;
    },
  },
};
</script>

<style scoped>
.header-actions {
  display: flex;
  flex-wrap: wrap;
  order: 3;
  flex-basis: 100%;
  min-width: 0;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}
.history-btn, .application-btn {
  padding: 6px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  font-size: 12px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
}
.history-btn:hover, .application-btn:hover {
  background: var(--surface-2);
  border-color: var(--accent);
}
.blacklist-add-btn {
  padding: 6px 12px;
  background: var(--surface);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
  border-radius: 20px;
  font-size: 12px;
  color: var(--danger-text);
  cursor: pointer;
  transition: background-color 0.15s ease, border-color 0.15s ease, color 0.15s ease;
  white-space: nowrap;
}
.blacklist-add-btn:hover {
  background: var(--danger-bg);
  border-color: var(--danger);
}
@media (max-width: 767.98px) {
  .header-actions {
    order: 3;
    width: 100%;
    justify-content: flex-start;
    margin-top: 10px;
  }
}
</style>
