<template>
  <div class="car-access-flag-rows" data-testid="car-access-flags">
    <div v-for="row in rows" :key="row.key" class="detail-item" :data-testid="`car-access-${row.key}`">
      <span class="detail-label">{{ row.label }}</span>
      <span class="detail-value">{{ row.value }}</span>
    </div>
  </div>
</template>
<script>
export default {
  name: 'CarAccessFlagRows',
  props: { flags: { type: Object, default: null } },
  computed: {
    rows() {
      return [{ key: 'roof_access', label: 'Доступ на крышу' }, { key: 'free_parking', label: 'Бесплатная парковка' }]
        .map(row => {
          const value = this.flags?.[row.key], own = this.flags?.[`individual_${row.key}`];
          return { ...row, value: typeof value !== 'boolean' || typeof own !== 'boolean' ? 'Нет данных' :
            value ? (own ? 'Да — собственный признак' : 'Да — из вложения') : 'Нет' };
        });
    },
  },
};
</script>
<style scoped>
.car-access-flag-rows { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin-top: 12px; }
.detail-item { display: flex; flex-direction: column; gap: 4px; }
.detail-label { font-size: 11px; color: var(--text-muted); font-weight: 400; letter-spacing: 0.3px; }
.detail-value { font-size: 14px; color: var(--text); font-weight: 500; word-break: break-word; }
@media (max-width: 480px) { .car-access-flag-rows { grid-template-columns: 1fr; } }
</style>
