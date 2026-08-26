<template>
  <Teleport to="body">
    <div
      class="modal-overlay"
      @click="$emit('close')"
    >
      <div
        class="modal-content"
        :class="{ 'is-dragging': sheetDragging }"
        :style="sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : null"
        @click.stop
        @touchstart="onSheetTouchStart"
        @touchmove="onSheetTouchMove"
        @touchend="onSheetTouchEnd"
      >
        <div
          class="sheet-handle"
          aria-hidden="true"
        />
        <div class="modal-header">
          <div class="modal-header__top">
            <h3>Привязка новых сотрудников</h3>
          </div>
          <button
            class="modal-close"
            @click="$emit('close')"
          >
            ×
          </button>
        </div>
        <div
          ref="modalBody"
          class="modal-body"
        >
          <div class="binding-info">
            <p class="binding-description">
              Все добавленные сотрудники будут <strong>автоматически привязаны</strong> к вашему аккаунту.
              Вы можете дополнительно привязать их к организации и/или компании для использования <strong>другими сотрудниками</strong>:
            </p>
                    
            <div class="employees-list-section">
              <p class="section-title">
                Новые сотрудники ({{ newEmployeesToBind.length }}):
              </p>
              <div class="employees-list">
                <div 
                  v-for="employee in newEmployeesToBind" 
                  :key="employee.passportSeriesNumber"
                  class="employee-item"
                >
                  <div class="employee-info">
                    <span class="employee-name">{{ formatFullName(employee) }}</span>
                    <span class="employee-position">{{ employee.position }}</span>
                  </div>
                </div>
              </div>
            </div>

            <div class="binding-options-section">
              <p class="section-title">
                Привязать всех сотрудников к:
              </p>
              <div class="binding-options">
                <label
                  v-if="hasOrganization"
                  class="binding-option"
                >
                  <input 
                    v-model="bindToOrganization" 
                    type="checkbox"
                  >
                  <span class="option-text">Организации "{{ organization }}"</span>
                </label>
                <label
                  v-if="hasCompany"
                  class="binding-option"
                >
                  <input 
                    v-model="bindToCompany" 
                    type="checkbox"
                  >
                  <span class="option-text">Компании "{{ company }}"</span>
                </label>
              </div>
            </div>

            <div class="warning-section">
              <p class="warning-text">
                <strong class="red">Внимание!</strong> При привязке сотрудника к организации или компании, он будет доступен для отображения и использования для всех сотрудников, привязанных к организации/компании.
              </p>
            </div>
          </div>
                
          <div class="modal-actions">
            <button
              class="confirm-btn"
              @click="handleConfirm"
            >
              {{ buttonText }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script>
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { ref } from 'vue';
import { useSwipeDismiss } from '@/composables/useSwipeDismiss';

export default {
    name: 'EmployeeBindingModal',
    props: {
        newEmployeesToBind: { type: Array, default: () => [] },
        organization: { type: String, default: null },
        company: { type: String, default: null },
        hasOrganization: Boolean,
        hasCompany: Boolean
    },
    emits: ['confirm-binding', 'skip-binding', 'close'],
    setup(_, { emit }) {
        // Контракт окна: свайп вниз за ползунок закрывает лист на мобилке;
        // Escape и блокировка прокрутки фона - в mounted/beforeUnmount ниже.
        const modalBody = ref(null);
        const swipe = useSwipeDismiss(() => emit('close'), {
            handleSelector: '.sheet-handle',
            getScrollTop: () => modalBody.value?.scrollTop ?? 0,
        });
        return {
            modalBody,
            sheetOffset: swipe.offset,
            sheetDragging: swipe.isDragging,
            onSheetTouchStart: swipe.onTouchStart,
            onSheetTouchMove: swipe.onTouchMove,
            onSheetTouchEnd: swipe.onTouchEnd,
        };
    },
    data() {
        return {
            bindToOrganization: false,
            bindToCompany: false
        }
    },
    computed: {
        buttonText() {
            if (this.bindToOrganization || this.bindToCompany) {
                return 'Привязать и отправить';
            } else {
                return 'Отправить';
            }
        }
    },
    mounted() {
        document.addEventListener('keydown', this.handleKeydown);
        setBodyScrollLock(this, true);
    },
    beforeUnmount() {
        document.removeEventListener('keydown', this.handleKeydown);
        releaseBodyScrollLock(this);
    },
    methods: {
        formatFullName(employee) {
            const parts = [];
            if (employee.lastName) parts.push(employee.lastName);
            if (employee.firstName) parts.push(employee.firstName);
            if (employee.middleName) parts.push(employee.middleName);
            return parts.join(' ') || 'Не указано';
        },
        
        handleConfirm() {
            // Отправляем данные о привязке
            this.$emit('confirm-binding', {
                bindToOrganization: this.bindToOrganization,
                bindToCompany: this.bindToCompany
            });
        },

        handleKeydown(e) {
            if (e.key === 'Escape') this.$emit('close');
        }
    },
}
</script>

<style scoped>
/* Модальное окно привязки */
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
    z-index: 1000;
    backdrop-filter: blur(0.1px);
    -webkit-backdrop-filter: blur(0.1px);
}

.modal-content {
    background: var(--surface);
    border-radius: 20px;
    padding: 0;
    width: 500px;
    max-width: 90vw;
    max-height: 600px;
    overflow: hidden;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: 15px;
    border-bottom: 1px solid var(--border);
}

.modal-header__top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex: 1;
    height: 25px;
}

.modal-header h3 {
    margin: 0;
    color: var(--text);
    font-size: 18px;
}

.modal-close {
    background: none;
    border: none;
    font-size: 24px;
    cursor: pointer;
    color: var(--text-muted);
    padding: 0;
    width: 30px;
    height: 30px;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-left: 10px;
}

.modal-close:hover {
    color: var(--text);
}

.modal-body {
    padding: 20px;
    max-height: 550px;
    overflow-y: auto;
}

.binding-info {
    margin-bottom: 20px;
}

.binding-description {
    font-size: 14px;
    line-height: 1.5;
    color: var(--text-muted);
    margin-bottom: 20px;
    text-align: left;
}

.section-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--text);
    margin-bottom: 10px;
}

.employees-list-section {
    margin-bottom: 25px;
}

.employees-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: 150px;
    overflow-y: auto;
    padding-right: 5px;
}

.employee-item {
    padding: 10px 12px;
    border: 1px solid var(--border);
    border-radius: 15px;
    background: var(--surface-2);
}

.employee-info {
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.employee-name {
    font-weight: 600;
    color: var(--text);
    font-size: 14px;
}

.employee-position {
    color: var(--text-muted);
    font-size: 12px;
}

.binding-options-section {
    margin-bottom: 20px;
    padding-top: 20px;
    border-top: 1px solid var(--border);
}

.binding-options {
    display: flex;
    flex-direction: column;
    gap: 0;
}

.binding-option {
    display: flex;
    align-items: center;
    gap: 10px;
    cursor: pointer;
    font-size: 14px;
    padding: 8px 0;
}

.binding-option input[type="checkbox"] {
    width: 16px;
    height: 16px;
    cursor: pointer;
}

.option-text {
    color: var(--text);
}

.warning-section {
    margin-top: 15px;
    padding: 12px;
    background: var(--danger-bg);
    border-radius: 8px;
    border: 1px solid color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.warning-text {
    font-size: 12px;
    line-height: 1.5;
    color: var(--text-muted);
    margin: 0;
    text-align: left;
}

.modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
    padding-top: 20px;
    border-top: 1px solid var(--border);
}

.confirm-btn {
    background: var(--accent);
    color: var(--accent-contrast);
    border: none;
    border-radius: 12px;
    padding: 12px 30px;
    font-size: 14px;
    cursor: pointer;
    transition: background-color 0.2s;
    min-width: 180px;
}

.confirm-btn:hover {
    background: var(--accent-hover);
}

.blue {
    color: var(--accent-text);
}

.red {
    color: var(--danger-text);
}

@media (max-width: 768px) {
    /* Размеры листа приходят из глобального .modal-content (App.vue) с !important -
       локальные width/max-height/radius здесь были мёртвыми и вводили в заблуждение. */
    
    .modal-body {
        max-height: 350px;
    }
    
    .confirm-btn {
        width: 100%;
        min-width: auto;
    }
}

.sheet-handle {
    display: none;
    width: 40px;
    height: 4px;
    border-radius: 2px;
    background: var(--border);
    margin: 10px auto 2px;
    flex-shrink: 0;
}

@media (max-width: 768px) {
    /* Лист выезжает снизу глобальным паттерном .modal-content (App.vue), здесь -
       ползунок и возврат листа на место после недотянутого свайпа. */
    .sheet-handle {
        display: block;
    }

    .modal-content {
        transition: transform 0.3s ease;
    }

    .modal-content.is-dragging {
        transition: none;
    }

    .modal-close {
        min-width: 40px;
        min-height: 40px;
    }
}
</style>