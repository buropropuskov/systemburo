<template>
  <div class="skeleton-card">
    <SkeletonLine
      width="45%"
      height="16px"
    />
    <div class="skeleton-card__body">
      <SkeletonLine
        v-for="i in lines"
        :key="i"
        :width="lineWidth(i)"
        height="12px"
      />
    </div>
    <SkeletonLine
      v-if="showBadge"
      width="80px"
      height="10px"
    />
  </div>
</template>

<script>
import SkeletonLine from './SkeletonLine.vue'

export default {
  name: 'SkeletonCard',
  components: { SkeletonLine },
  props: {
    lines: { type: Number, default: 3 },
    showBadge: { type: Boolean, default: true },
  },
  methods: {
    lineWidth(i) {
      const widths = ['90%', '75%', '60%', '85%']
      return widths[(i - 1) % widths.length]
    },
  },
}
</script>

<style scoped>
.skeleton-card {
  background: var(--surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: var(--spacing-md);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.skeleton-card__body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
</style>
