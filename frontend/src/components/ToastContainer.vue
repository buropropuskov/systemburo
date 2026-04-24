<template>
  <div class="toast-container">
    <transition-group name="toast">
      <div
        v-for="toast in toasts"
        :key="toast.id"
        class="toast-item"
        :class="'toast-' + toast.type"
      >
        <span class="toast-icon">
          <template v-if="toast.type === 'success'">&#10003;</template>
          <template v-else-if="toast.type === 'error'">&#10007;</template>
          <template v-else-if="toast.type === 'warning'">&#9888;</template>
          <template v-else>&#8505;</template>
        </span>
        <span class="toast-message">{{ toast.message }}</span>
        <button
          class="toast-close"
          @click="removeToast(toast.id)"
        >
          &times;
        </button>
      </div>
    </transition-group>
  </div>
</template>

<script>
import { useUiStore } from '@/stores/ui'

export default {
  name: 'ToastContainer',
  computed: {
    toasts() {
      const ui = useUiStore()
      return ui.toasts
    }
  },
  methods: {
    removeToast(id) {
      const ui = useUiStore()
      ui.toasts = ui.toasts.filter(t => t.id !== id)
    }
  }
}
</script>

<style scoped>
.toast-container {
  position: fixed;
  top: 20px;
  right: 20px;
  z-index: 10000;
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-width: 400px;
}

.toast-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  border-radius: 8px;
  color: #fff;
  font-size: 14px;
  line-height: 1.4;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  min-width: 280px;
}

.toast-success {
  background-color: #22c55e;
}

.toast-error {
  background-color: #ef4444;
}

.toast-warning {
  background-color: #f59e0b;
}

.toast-info {
  background-color: #3b82f6;
}

.toast-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.toast-message {
  flex: 1;
  word-break: break-word;
}

.toast-close {
  background: none;
  border: none;
  color: rgba(255, 255, 255, 0.8);
  font-size: 18px;
  cursor: pointer;
  padding: 0 4px;
  line-height: 1;
  flex-shrink: 0;
}

.toast-close:hover {
  color: #fff;
}

.toast-enter-active {
  transition: all 0.3s ease;
}

.toast-leave-active {
  transition: all 0.3s ease;
}

.toast-enter-from {
  opacity: 0;
  transform: translateX(40px);
}

.toast-leave-to {
  opacity: 0;
  transform: translateX(40px);
}
</style>
