<template>
  <div
    class="skeleton-line"
    :class="{ 'skeleton-line--square': !rounded }"
    :style="{ width, height }"
  />
</template>

<script>
export default {
  name: 'SkeletonLine',
  props: {
    width: { type: String, default: '100%' },
    height: { type: String, default: '14px' },
    rounded: { type: Boolean, default: true },
  },
}
</script>

<style scoped>
@keyframes skeleton-shimmer {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(100%); }
}

.skeleton-line {
  background: var(--color-skeleton);
  border-radius: 50px;
  overflow: hidden;
  position: relative;
}

.skeleton-line--square {
  border-radius: var(--radius-sm);
}

.skeleton-line::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(90deg, transparent 0%, var(--color-skeleton-shine) 50%, transparent 100%);
  animation: skeleton-shimmer 1.5s cubic-bezier(0.4, 0, 0.2, 1) infinite;
}
</style>
