<template>
  <div class="skeleton-table">
    <div class="skeleton-table__header">
      <SkeletonLine
        v-for="c in columns"
        :key="'h' + c"
        :width="colWidth(c, true)"
        height="12px"
      />
    </div>
    <div
      v-for="r in rows"
      :key="r"
      class="skeleton-table__row"
    >
      <SkeletonLine
        v-for="c in columns"
        :key="'r' + r + 'c' + c"
        :width="colWidth(c, false)"
        height="12px"
      />
    </div>
  </div>
</template>

<script>
import SkeletonLine from './SkeletonLine.vue'

export default {
  name: 'SkeletonTable',
  components: { SkeletonLine },
  props: {
    rows: { type: Number, default: 6 },
    columns: { type: Number, default: 4 },
  },
  methods: {
    colWidth(col, isHeader) {
      const widths = ['40%', '70%', '55%', '80%', '45%', '65%', '50%']
      const idx = (col - 1) % widths.length
      return isHeader ? '60%' : widths[idx]
    },
  },
}
</script>

<style scoped>
.skeleton-table {
  width: 100%;
}

.skeleton-table__header {
  display: flex;
  gap: 16px;
  padding: 12px 16px;
  background: var(--color-bg-secondary);
  border-bottom: 1px solid var(--color-border);
  border-radius: var(--radius-sm) var(--radius-sm) 0 0;
}

.skeleton-table__header > * {
  flex: 1;
}

.skeleton-table__row {
  display: flex;
  gap: 16px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--color-border);
}

.skeleton-table__row > * {
  flex: 1;
}

.skeleton-table__row:last-child {
  border-bottom: none;
}
</style>
