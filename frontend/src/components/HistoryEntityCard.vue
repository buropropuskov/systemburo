<template>
  <BaseModal v-if="!entity" :show="true" title="Карточка записи" width="460px" :z-index="14000" @close="$emit('close')">
    <p class="card-status" role="status">{{ error || 'Загрузка карточки…' }}</p>
  </BaseModal>
  <VehicleDetailsModal v-if="kind === 'car' && entity" :show="true" :vehicle="entity" source="history" readonly @close="$emit('close')" />
  <EmployeeDetailsModal v-if="kind === 'employee' && entity" :show="true" :employee="entity" source="history" readonly @close="$emit('close')" />
</template>

<script>
import { defineAsyncComponent } from 'vue';
import BaseModal from '@/components/ui/BaseModal.vue';
import { loadHistoryEntity } from '@/api/historyEntity';

export default {
  name: 'HistoryEntityCard',
  components: {
    BaseModal,
    VehicleDetailsModal: defineAsyncComponent(() => import('./CreateApplication/VehicleDetailsModal.vue')),
    EmployeeDetailsModal: defineAsyncComponent(() => import('./CreateApplication/EmployeeDetailsModal.vue')),
  },
  props: { kind: { type: String, required: true }, row: { type: Object, required: true }, tableId: { type: Number, default: null } },
  emits: ['close'],
  data: () => ({ entity: null, error: '', loadSequence: 0 }),
  watch: { row: { immediate: true, handler: 'load' } },
  beforeUnmount() { this.loadSequence++; },
  methods: {
    async load() {
      const sequence = ++this.loadSequence;
      this.entity = null; this.error = '';
      try {
        const entity = await loadHistoryEntity(this.kind, this.row, this.tableId);
        if (sequence === this.loadSequence) this.entity = entity;
      } catch (error) {
        if (sequence === this.loadSequence) this.error = error.message;
      }
    },
  },
};
</script>

<style scoped>
.card-status { margin: 0; padding: 16px 20px; }
</style>
