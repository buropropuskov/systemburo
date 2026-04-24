<template>
  <transition name="modal-fade">
    <div
      v-if="show"
      class="modal-overlay"
      @click.self="handleCancel"
    >
      <div class="modal">
        <div class="modal-header">
          {{ title }}
        </div>
        <div class="modal-content">
          {{ message }}
        </div>
        <div class="modal-actions">
          <button 
            class="cancel-btn" 
            :style="cancelButtonStyle"
            @click="handleCancel"
          >
            {{ cancelText }}
          </button>
          <button 
            class="confirm-btn" 
            :style="confirmButtonStyle"
            @click="handleConfirm"
          >
            {{ confirmText }}
          </button>
        </div>
      </div>
    </div>
  </transition>
</template>

<script>
export default {
    name: 'ConfirmationModal',
    props: {
        title: {
            type: String,
            default: 'Подтверждение действия'
        },
        message: {
            type: String,
            required: true
        },
        confirmText: {
            type: String,
            default: 'Подтвердить'
        },
        cancelText: {
            type: String,
            default: 'Отмена'
        },
        confirmButtonStyle: {
            type: Object,
            default: () => ({})
        },
        cancelButtonStyle: {
            type: Object,
            default: () => ({})
        },
        show: {
            type: Boolean,
            default: false
        }
    },
    emits: ['cancel', 'confirm'],
    methods: {
        handleConfirm() {
            this.$emit('confirm');
        },
        
        handleCancel() {
            this.$emit('cancel');
        }
    }
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  animation: overlayAppear 0.3s ease-out;
}


.modal {
    background: white;
    border-radius: 15px;
    padding: 20px;
    width: 300px;
    box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
}

.modal-header {
    font-size: 14px;
    font-weight: bold;
    margin-bottom: 10px;
    color: #333;
}

.modal-content {
    font-size: 12px;
    color: #666;
    margin-bottom: 20px;
    line-height: 1.4;
}

.modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
}

.cancel-btn, .confirm-btn {
    padding: 8px 16px;
    border: 1px solid #e6e6e6;
    border-radius: 8px;
    font-size: 12px;
    cursor: pointer;
    transition: all 0.3s ease;
    min-width: 80px;
}

.cancel-btn {
    background: white;
    color: #666;
}

.cancel-btn:hover {
    background: #f5f5f5;
    border-color: #d4d4d4;
}

.confirm-btn {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.confirm-btn:hover {
    background: #3a45c4;
    border-color: #3a45c4;
}

/* Анимации для модального окна */
.modal-fade-enter-active,
.modal-fade-leave-active {
    transition: all 0.3s ease;
}

.modal-fade-enter-active .modal-overlay,
.modal-fade-leave-active .modal-overlay {
    transition: all 0.3s ease;
}

.modal-fade-enter-active .modal,
.modal-fade-leave-active .modal {
    transition: all 0.3s ease;
}

.modal-fade-enter-from .modal-overlay,
.modal-fade-leave-to .modal-overlay {
    background: rgba(0, 0, 0, 0);
}

.modal-fade-enter-from .modal,
.modal-fade-leave-to .modal {
    opacity: 0;
    transform: scale(1) translateY(-20px);
}

.modal-fade-enter-to .modal-overlay,
.modal-fade-leave-from .modal-overlay {
    background: rgba(0, 0, 0, 0.5);
}

.modal-fade-enter-to .modal,
.modal-fade-leave-from .modal {
    opacity: 1;
    transform: scale(1) translateY(0);
}


@keyframes overlayAppear {
  from {
    background: rgba(0, 0, 0, 0);
  }
  to {
    background: rgba(0, 0, 0, 0.5);
  }
}
</style>