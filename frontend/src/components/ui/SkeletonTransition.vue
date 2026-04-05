<template>
  <transition name="skeleton-fade" mode="out-in">
    <div v-if="showSkeleton" key="skeleton">
      <slot name="skeleton" />
    </div>
    <div v-else key="content">
      <slot />
    </div>
  </transition>
</template>

<script>
export default {
  name: 'SkeletonTransition',
  props: {
    loading: { type: Boolean, required: true },
    delay: { type: Number, default: 200 },
    minDuration: { type: Number, default: 400 },
  },
  data() {
    return {
      showSkeleton: false,
      delayTimer: null,
      shownAt: null,
    }
  },
  watch: {
    loading: {
      immediate: true,
      handler(val) {
        if (val) {
          this.delayTimer = setTimeout(() => {
            this.showSkeleton = true
            this.shownAt = Date.now()
          }, this.delay)
        } else {
          clearTimeout(this.delayTimer)
          if (this.showSkeleton && this.shownAt) {
            const elapsed = Date.now() - this.shownAt
            const remaining = this.minDuration - elapsed
            if (remaining > 0) {
              setTimeout(() => { this.showSkeleton = false }, remaining)
            } else {
              this.showSkeleton = false
            }
          } else {
            this.showSkeleton = false
          }
        }
      },
    },
  },
  beforeUnmount() {
    clearTimeout(this.delayTimer)
  },
}
</script>

<style scoped>
.skeleton-fade-enter-active {
  transition: opacity 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.skeleton-fade-leave-active {
  transition: opacity 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.skeleton-fade-enter-from,
.skeleton-fade-leave-to {
  opacity: 0;
}
</style>
