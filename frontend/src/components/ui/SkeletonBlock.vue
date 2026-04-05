<template>
  <div
    class="skeleton-block"
    :style="{ width, height, borderRadius: radius }"
  />
</template>

<script>
export default {
  name: 'SkeletonBlock',
  props: {
    width: { type: String, default: '100%' },
    height: { type: String, default: '120px' },
    radius: { type: String, default: 'var(--radius-md)' },
  },
}
</script>

<style scoped>
@keyframes skeleton-shimmer {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(100%); }
}

.skeleton-block {
  background: var(--color-skeleton);
  overflow: hidden;
  position: relative;
}

.skeleton-block::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(90deg, transparent 0%, var(--color-skeleton-shine) 50%, transparent 100%);
  animation: skeleton-shimmer 1.5s cubic-bezier(0.4, 0, 0.2, 1) infinite;
}
</style>
