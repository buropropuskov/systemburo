<template>
  <div v-if="visible" class="header-actions">
    <HintTooltip v-if="historyVisible" text="Полная история" compact>
    <button v-if="historyVisible" class="history-btn" aria-label="Полная история" @click="$emit('history')">
      <AppIcon name="history" :size="16" /><span class="header-action-label">История</span>
    </button>
    </HintTooltip>
    <HintTooltip v-if="applicationVisible" text="Открыть заявку" compact>
    <button v-if="applicationVisible" class="application-btn" aria-label="Открыть заявку" @click="$emit('application')">
      <AppIcon name="applicationOpen" :size="16" /><span class="header-action-label">Открыть заявку</span>
    </button>
    </HintTooltip>
    <ManualAttachCardAction
      icon-only
      :show="show"
      :kind="kind"
      :entityID="manualEntityID"
      :tableID="periodTableId"
      :readonly="readonly"
      @changed="$emit('manual-attached', $event)"
    />
    <EntityPeriodEditor
      icon-only
      :show="show"
      :kind="kind"
      :entity="entity"
      :source="source"
      :readonly="readonly"
      :table-id="periodTableId"
      :license-plate-formats="licensePlateFormats"
      @changed="$emit('period-changed', $event)"
    />
    <HintTooltip v-if="blacklistVisible" text="Добавить в чёрный список" compact>
    <button v-if="blacklistVisible" class="blacklist-add-btn" aria-label="Добавить в чёрный список" @click="$emit('blacklist')">
      <AppIcon name="ban" :size="16" /><span class="header-action-label">В ЧС</span>
    </button>
    </HintTooltip>
  </div>
</template>

<script>
import HintTooltip from '@/components/ui/HintTooltip.vue';
import AppIcon from '@/components/icons/AppIcon.vue';
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
  components: { HintTooltip, AppIcon, EntityPeriodEditor, ManualAttachCardAction },
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
.header-actions { display: flex; flex-wrap: nowrap; flex: 0 1 auto; min-width: 0; align-items: center; gap: 6px; margin-left: auto; }
.history-btn, .application-btn, .blacklist-add-btn { display: inline-flex; align-items: center; justify-content: center; gap: 6px; height: 30px; padding: 6px 10px; background: var(--surface); border: 1px solid var(--border); border-radius: 20px; font-size: 12px; color: var(--text); cursor: pointer; white-space: nowrap; transition: background-color .2s ease, border-color .2s ease; }
.application-btn, .blacklist-add-btn { width: 30px; padding: 0; }
.application-btn .header-action-label, .blacklist-add-btn .header-action-label { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
.history-btn:hover, .application-btn:hover { background: var(--surface-2); border-color: var(--accent); }
.blacklist-add-btn { color: var(--danger-text); }
.blacklist-add-btn:hover { background: var(--danger-bg); border-color: var(--danger); }
</style>
