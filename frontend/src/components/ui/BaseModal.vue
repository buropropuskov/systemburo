<template>
  <teleport to="body">
    <transition name="modal-fade">
      <div v-if="show" class="base-modal-overlay" @click.self="handleOverlayClick">
        <div class="base-modal" :style="{ maxWidth: width }" @click.stop role="dialog" aria-modal="true" :aria-label="title">
          <div class="base-modal__header" v-if="title || $slots.header || closable">
            <slot name="header">
              <h3 class="base-modal__title">{{ title }}</h3>
            </slot>
            <button v-if="closable" class="base-modal__close" @click="$emit('close')" aria-label="Закрыть">&times;</button>
          </div>
          <div class="base-modal__body">
            <slot></slot>
          </div>
          <div class="base-modal__actions" v-if="$slots.actions">
            <slot name="actions"></slot>
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script>
export default {
  name: 'BaseModal',
  props: {
    show: {
      type: Boolean,
      required: true,
    },
    title: {
      type: String,
      default: '',
    },
    width: {
      type: String,
      default: '500px',
    },
    closable: {
      type: Boolean,
      default: true,
    },
    closeOnOverlay: {
      type: Boolean,
      default: true,
    },
  },
  emits: ['close'],
  methods: {
    handleOverlayClick() {
      if (this.closeOnOverlay) {
        this.$emit('close');
      }
    },
    handleKeydown(e) {
      if (e.key === 'Escape' && this.closable) {
        this.$emit('close');
      }
      if (e.key === 'Tab') {
        this.trapFocus(e);
      }
    },
    trapFocus(e) {
      const focusable = this.$el.querySelectorAll(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      if (focusable.length === 0) return;

      const first = focusable[0];
      const last = focusable[focusable.length - 1];

      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    },
  },
  watch: {
    show(val) {
      document.body.style.overflow = val ? 'hidden' : '';
    },
  },
  mounted() {
    document.addEventListener('keydown', this.handleKeydown);
  },
  beforeUnmount() {
    document.removeEventListener('keydown', this.handleKeydown);
    document.body.style.overflow = '';
  },
};
</script>

<style scoped>
.base-modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  backdrop-filter: blur(1px);
}

.base-modal {
  background: #fff;
  border-radius: var(--radius-md);
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.15);
  max-height: 85vh;
  overflow-y: auto;
  width: 100%;
  margin: 0 20px;
}

.base-modal__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px;
  border-bottom: 1px solid var(--color-border);
}

.base-modal__title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text);
}

.base-modal__close {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  line-height: 1;
  color: #999;
  background: none;
  border: none;
  cursor: pointer;
  border-radius: 50%;
  transition: all 0.2s;
  flex-shrink: 0;
}

.base-modal__close:hover {
  color: var(--color-text);
  background: #f5f5f5;
}

.base-modal__body {
  padding: 20px;
}

.base-modal__actions {
  padding: 15px 20px;
  border-top: 1px solid var(--color-border);
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

/* Transition */
.modal-fade-enter-active {
  transition: opacity 0.3s ease;
}
.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .base-modal {
  animation: modal-scale-in 0.3s ease;
}
.modal-fade-leave-active .base-modal {
  animation: modal-scale-out 0.2s ease;
}

@keyframes modal-scale-in {
  from {
    transform: scale(0.95);
    opacity: 0;
  }
  to {
    transform: scale(1);
    opacity: 1;
  }
}

@keyframes modal-scale-out {
  from {
    transform: scale(1);
    opacity: 1;
  }
  to {
    transform: scale(0.95);
    opacity: 0;
  }
}
</style>
