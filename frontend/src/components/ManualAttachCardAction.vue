<template>
  <template v-if="eligible">
    <HintTooltip :text="context ? 'Привязать к заявке' : error" compact>
    <button v-if="context" type="button" class="lk-button lk-button--ghost lk-button--sm" :class="{ 'manual-attach-icon': iconOnly }" aria-label="Привязать к заявке" data-testid="manual-attach-card-action" @click="open = true">
      <AppIcon v-if="iconOnly" name="applicationOpen" :size="16" /><span v-else>Привязать к заявке</span>
    </button>
    <button v-else-if="error" type="button" class="lk-button lk-button--ghost lk-button--sm" :class="{ 'manual-attach-icon': iconOnly }" aria-label="Повторить проверку привязки" :title="error" data-testid="manual-attach-card-retry" @click="load">
      <AppIcon v-if="iconOnly" name="refresh" :size="16" /><span v-else>Повторить проверку привязки</span>
    </button>
    </HintTooltip>
    <ManualAttachExistingModal
      :show="open && !!context" :kind="kind" :entity-i-d="entityID" :table-i-d="tableID" :title="title"
      @close="open = false" @changed="changed"
    />
  </template>
</template>

<script>
import HintTooltip from '@/components/ui/HintTooltip.vue';
import AppIcon from '@/components/icons/AppIcon.vue';
import ManualAttachExistingModal from './ManualAttachExistingModal.vue';
import { getManualAttachContext } from '@/api/manualAttachSingle';
import { usePermissionsStore } from '@/stores/permissions';

export default {
  name: 'ManualAttachCardAction',
  components: { ManualAttachExistingModal, HintTooltip, AppIcon },
  props: {
    iconOnly: { type: Boolean, default: false },
    show: { type: Boolean, default: false },
    kind: { type: String, required: true },
    entityID: { type: Number, default: null },
    tableID: { type: Number, default: null },
    title: { type: String, default: '' },
    readonly: { type: Boolean, default: false },
  },
  emits: ['changed'],
  data() { return { context: null, open: false, error: '', version: 0 }; },
  computed: {
    eligible() {
      return this.show && !this.readonly && ['employee', 'car'].includes(this.kind) &&
        Number.isSafeInteger(this.entityID) && this.entityID > 0 && Number.isSafeInteger(this.tableID) && this.tableID > 0 &&
        usePermissionsStore().hasPermission('page.admin');
    },
    identity() { return `${this.eligible}:${this.kind}:${this.entityID}:${this.tableID}`; },
  },
  watch: {
    identity: { immediate: true, handler() {
      this.version++;
      this.context = null;
      this.open = false;
      this.error = '';
      if (this.eligible) this.load();
    } },
  },
  beforeUnmount() { this.version++; },
  methods: {
    async load() {
      if (!this.eligible) return;
      const version = ++this.version;
      this.context = null;
      this.error = '';
      try {
        const context = await getManualAttachContext(this.kind, this.entityID, this.tableID);
        if (version === this.version) this.context = context;
      } catch (error) {
        if (version === this.version && ![403, 404, 409, 422].includes(error.status)) this.error = error.message;
      }
    },
    changed(result) {
      this.open = false;
      this.context = null;
      this.version++;
      this.$emit('changed', result);
    },
  },
};
</script>

<style scoped>
.manual-attach-icon { width: 30px; height: 30px; padding: 0; flex-shrink: 0; }
</style>
