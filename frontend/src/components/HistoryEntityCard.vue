<template>
  <VehicleDetailsModal v-if="kind === 'car'" :show="visible" :vehicle="entity" :loading="loading" :load-error="error" source="history" :license-plate-formats="licensePlateFormats" :all-unloading-places="historyUnloadPlaces" readonly @close="close" @closed="$emit('close')" />
  <EmployeeDetailsModal v-else :show="visible" :employee="entity" :loading="loading" :load-error="error" source="history" readonly @close="close" @closed="$emit('close')" />
</template>

<script>
import { useLicensePlateDisplay } from '@/composables/useLicensePlateDisplay';
import { defineAsyncComponent } from 'vue';
import { loadHistoryEntity } from '@/api/historyEntity';

export default {
  name: 'HistoryEntityCard',
  setup() { return useLicensePlateDisplay(); },
  components: {
    VehicleDetailsModal: defineAsyncComponent(() => import('./CreateApplication/VehicleDetailsModal.vue')),
    EmployeeDetailsModal: defineAsyncComponent(() => import('./CreateApplication/EmployeeDetailsModal.vue')),
  },
  props: { kind: { type: String, required: true }, row: { type: Object, required: true }, tableId: { type: Number, default: null } },
  emits: ['close'],
  computed: {
    historyUnloadPlaces() {
      return (this.entity?.unload_place_ids || []).map((id, i) => ({ id, name: this.entity.unload_places?.[i] || `ID: ${id}` }));
    },
  },
  data: () => ({ entity: null, error: '', loading: true, visible: true, loadSequence: 0 }),
  watch: { row: { immediate: true, handler: 'load' } },
  beforeUnmount() { this.loadSequence++; },
  methods: {
    close() { this.visible = false; this.loadSequence++; },
    async load() {
      const sequence = ++this.loadSequence;
      this.entity = null; this.error = ''; this.loading = true;
      try {
        const entity = await loadHistoryEntity(this.kind, this.row, this.tableId);
        if (sequence === this.loadSequence) this.entity = entity;
      } catch (error) {
        if (sequence === this.loadSequence) this.error = error.message;
      } finally {
        if (sequence === this.loadSequence) this.loading = false;
      }
    },
  },
};
</script>
