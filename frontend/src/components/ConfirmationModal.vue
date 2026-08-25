<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="modal-overlay"
        :style="{ zIndex }"
        @click.self="handleCancel"
      >
        <div class="modal">
          <div class="modal-header">
            {{ title }}
          </div>
          <div class="modal-message">
            {{ message }}
          </div>
          <!-- Необязательная вставка между текстом и кнопками: поле комментария к
               решению по дополнению (#1685). Без переданного содержимого не рисуется
               ничего, поэтому остальные вызовы окна выглядят как раньше. -->
          <div
            v-if="$slots.default"
            class="modal-extra"
          >
            <slot />
          </div>
          <div class="modal-actions">
            <button
              class="cancel-btn"
              data-testid="confirmation-cancel"
              :style="cancelButtonStyle"
              @click="handleCancel"
            >
              {{ cancelText }}
            </button>
            <button
              class="confirm-btn"
              data-testid="confirmation-confirm"
              :style="confirmButtonStyle"
              @click="handleConfirm"
            >
              {{ confirmText }}
            </button>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
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
        },
        // z-index оверлея. Дефолт 1000 - базовый слой модалок, как у BaseModal.
        // Поднимать, когда подтверждение вызывается ИЗ другой модалки: та стоит на
        // 1001+, и подтверждение на базовом слое оказывается под ней (окно вроде бы
        // открылось, а на экране его нет).
        zIndex: {
            type: Number,
            default: 1000
        }
    },
    emits: ['cancel', 'confirm'],
    mounted() {
        this.escHandler = (e) => {
            if (e.key === 'Escape' && this.show) this.handleCancel();
        };
        document.addEventListener('keydown', this.escHandler);
    },
    beforeUnmount() {
        document.removeEventListener('keydown', this.escHandler);
    },
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
  background: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  /* Фактическое значение приходит инлайном из пропа zIndex; правило оставлено
     запасным на случай, если стиль не применился. */
  z-index: 1000;
  backdrop-filter: blur(0.1px);
  -webkit-backdrop-filter: blur(0.1px);
}


.modal {
    background: var(--surface);
    border-radius: 15px;
    padding: 20px;
    width: 300px;
    box-shadow: 0 4px 6px var(--shadow-drop);
}

.modal-header {
    font-size: 14px;
    font-weight: bold;
    margin-bottom: 10px;
    color: var(--text);
}

.modal-message {
    font-size: 12px;
    color: var(--text-muted);
    margin-bottom: 20px;
    line-height: 1.4;
}

.modal-extra {
    margin-bottom: 16px;
}

.modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
}

.cancel-btn, .confirm-btn {
    padding: 8px 16px;
    border: 1px solid var(--border);
    border-radius: 8px;
    font-size: 12px;
    cursor: pointer;
    transition: all 0.3s ease;
    min-width: 80px;
}

.cancel-btn {
    background: var(--surface);
    color: var(--text-muted);
}

.cancel-btn:hover {
    background: var(--surface-2);
    border-color: var(--border);
}

.confirm-btn {
    background: var(--accent);
    color: var(--accent-contrast);
    border-color: var(--accent);
}

.confirm-btn:hover {
    background: var(--accent-hover);
    border-color: var(--accent-hover);
}

/* Анимации: фон overlay и .modal анимируются и на открытие, и на закрытие.
   ВАЖНО: классы перехода Vue вешает на КОРЕНЬ перехода (.modal-overlay), поэтому фон
   правим на самих .modal-fade-* (без вложенного .modal-overlay - такого потомка нет),
   иначе leave-фон не применяется и backdrop пропадает резко. */
.modal-fade-enter-active,
.modal-fade-leave-active {
    transition: background 0.3s ease;
}

.modal-fade-enter-active .modal,
.modal-fade-leave-active .modal {
    transition: opacity 0.3s ease, transform 0.3s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
    background: transparent;
}

.modal-fade-enter-from .modal,
.modal-fade-leave-to .modal {
    opacity: 0;
    transform: scale(1) translateY(-20px);
}

@media (max-width: 768px) {
    /* Глобальный оверлей App.vue прижимает окно к низу - даём ему вид листа:
       раньше узкая карточка 300px висела у нижней кромки со щелями по бокам. */
    .modal {
        width: 100%;
        max-width: 100%;
        border-radius: 16px 16px 0 0;
        padding: 20px 16px calc(20px + env(safe-area-inset-bottom));
    }

    .modal-header {
        font-size: 16px;
    }

    .modal-message {
        font-size: 14px;
    }

    .cancel-btn,
    .confirm-btn {
        min-height: 44px;
        font-size: 14px;
        flex: 1;
    }

    /* Лист прижат к низу, а десктопная modal-fade двигала его сверху
       (translateY(-20px)) - на мобилке выезжаем снизу, как остальные листы
       (образец - ExistingCarsModal). enter-from глушим: его сдвиг вверх
       спорил бы с выездом снизу. */
    .modal-fade-enter-from .modal {
        opacity: 1;
        transform: none;
    }

    .modal-fade-enter-active .modal {
        animation: confirm-sheet-up 0.3s cubic-bezier(0.32, 0.72, 0, 1);
    }

    .modal-fade-leave-active .modal {
        transition: transform 0.25s cubic-bezier(0.32, 0.72, 0, 1);
    }

    .modal-fade-leave-to .modal {
        opacity: 1;
        transform: translateY(100%);
    }
}

@keyframes confirm-sheet-up {
    from { transform: translateY(100%); }
    to { transform: translateY(0); }
}
</style>
