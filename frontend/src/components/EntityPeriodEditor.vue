<template>
  <button
    v-if="eligible"
    type="button"
    class="lk-button lk-button--ghost lk-button--sm"
    data-testid="entity-period-open"
    @click="open = true"
  >
    Изменить срок
  </button>
  <EntityPeriodModal
    v-if="eligible && open"
    :show="true"
    :kind="kind"
    :entityID="entityId"
    :tableID="tableId"
    :title="title"
    @close="open = false"
    @changed="changed"
  />
</template>

<script>
import EntityPeriodModal from '@/components/EntityPeriodModal.vue';
import { usePermissionsStore } from '@/stores/permissions';

// IDs from registry cards belong to UniqueEmployee/UniqueCar. Never fall back
// to entity.id there, or to passport/plate identity, when the active row is absent.
export function periodEditorEntityID(kind, source, entity) {
  if (!entity || entity.isDraft || entity.isPending) return null;
  let id = null;
  if (kind === 'employee') {
    if (source === 'employeesview') id = entity.activeEmployeeId;
    else if (['application', 'peopletable'].includes(source)) id = entity.id;
  } else if (kind === 'car') {
    if (source === 'carsview') id = entity.activeCarId;
    else if (['application', 'carstable'].includes(source)) id = entity.id;
  }
  return Number.isSafeInteger(id) && id > 0 ? id : null;
}

export function canEditEntityPeriod(kind, source, entity, readonly = false) {
  return !readonly && periodEditorEntityID(kind, source, entity) !== null &&
    usePermissionsStore().hasPermission('detail.period.change');
}

export default {
  name: 'EntityPeriodEditor',
  components: { EntityPeriodModal },
  props: {
    show: { type: Boolean, default: false },
    kind: { type: String, required: true, validator: value => ['employee', 'car'].includes(value) },
    source: { type: String, default: 'general' },
    entity: { type: Object, default: null },
    readonly: { type: Boolean, default: false },
    tableId: { type: Number, default: null },
  },
  emits: ['changed'],
  data() { return { open: false }; },
  computed: {
    entityId() { return periodEditorEntityID(this.kind, this.source, this.entity); },
    eligible() {
      return this.show && (this.tableId === null || (Number.isSafeInteger(this.tableId) && this.tableId > 0)) &&
        canEditEntityPeriod(this.kind, this.source, this.entity, this.readonly);
    },
    identity() { return `${this.eligible}:${this.kind}:${this.source}:${this.entityId}:${this.tableId}`; },
    title() {
      return this.kind === 'employee'
        ? [this.entity?.last_name, this.entity?.first_name, this.entity?.middle_name].filter(Boolean).join(' ')
        : (this.entity?.plateNumber || this.entity?.car_number || '');
    },
  },
  watch: {
    identity() { this.open = false; },
  },
  methods: {
    changed(result) {
      this.open = false;
      this.$emit('changed', result);
    },
  },
};
</script>
