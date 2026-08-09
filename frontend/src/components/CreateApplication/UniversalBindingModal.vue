<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="modal-overlay"
        @click.self="closeModal"
      >
        <div
          class="modal-content"
          :class="{ 'is-dragging': sheetDragging }"
          :style="sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : null"
          @touchstart="onSheetTouchStart"
          @touchmove="onSheetTouchMove"
          @touchend="onSheetTouchEnd"
        >
          <div
            class="sheet-handle"
            aria-hidden="true"
          />
          <div class="modal-header">
            <h3 class="modal-title">
              Привязка новых данных
            </h3>
            <button
              class="modal-close"
              :disabled="processing"
              @click="closeModal"
            >
              <svg
                width="10"
                height="10"
                viewBox="0 0 14 14"
                fill="none"
              >
                <path
                  d="M13 1L1 13M1 1L13 13"
                  stroke="#666"
                  stroke-width="2"
                  stroke-linecap="round"
                />
              </svg>
            </button>
          </div>

          <div
            ref="modalBody"
            class="modal-body"
          >
            <div class="binding-description">
              Все добавленные данные будут <strong>автоматически привязаны</strong> к вашему аккаунту.
              Вы можете дополнительно привязать их к организации и/или компании для использования <strong>другими сотрудниками</strong>:
            </div>

            <div
              v-if="newVehiclesToBind.length > 0"
              class="data-section"
            >
              <div class="section-header">
                <h4 class="section-title">
                  Новые автомобили
                </h4>
                <span class="count-badge">{{ newVehiclesToBind.length }}</span>
              </div>
              <div class="section-body">
                <div class="items-list">
                  <div
                    v-for="vehicle in newVehiclesToBind"
                    :key="vehicle.id"
                    class="item"
                    :class="{ 'item-fact': isVehicleByFact(vehicle) }"
                  >
                    <div class="item-info">
                      <span class="item-number">{{ vehicle.plateNumber }}</span>
                      <span class="item-detail">{{ vehicle.mark }}</span>
                      <span
                        v-if="isVehicleByFact(vehicle)"
                        class="fact-badge"
                      >По факту</span>
                    </div>
                  </div>
                </div>

                <div
                  v-if="hasVehiclesForBinding"
                  class="binding-options"
                >
                  <p class="options-title">
                    Привязать автомобили к:
                  </p>
                  <div class="options-group">
                    <label
                      v-if="hasOrganization"
                      class="binding-option"
                    >
                      <input
                        v-model="vehiclesBindToOrganization"
                        type="checkbox"
                      >
                      <span>Организации "{{ organization }}"</span>
                    </label>
                    <label
                      v-if="hasCompany"
                      class="binding-option"
                    >
                      <input
                        v-model="vehiclesBindToCompany"
                        type="checkbox"
                      >
                      <span>Компании "{{ company }}"</span>
                    </label>
                  </div>
                </div>
                <div
                  v-else
                  class="no-binding-message"
                >
                  Автомобили "По факту" не требуют привязки к организации/компании
                </div>
              </div>
            </div>

            <div
              v-if="newEmployeesToBind.length > 0"
              class="data-section"
            >
              <div class="section-header">
                <h4 class="section-title">
                  Новые сотрудники
                </h4>
                <span class="count-badge">{{ newEmployeesToBind.length }}</span>
              </div>
              <div class="section-body">
                <div class="items-list">
                  <div
                    v-for="employee in newEmployeesToBind"
                    :key="employee.id"
                    class="item"
                    :class="{ 'item-fact': isEmployeeByFact(employee) }"
                  >
                    <div class="item-info">
                      <span class="item-name">{{ formatFullName(employee) }}</span>
                      <span class="item-detail">{{ employee.position }}</span>
                      <span
                        v-if="isEmployeeByFact(employee)"
                        class="fact-badge"
                      >По факту</span>
                    </div>
                  </div>
                </div>

                <div
                  v-if="hasEmployeesForBinding"
                  class="binding-options"
                >
                  <p class="options-title">
                    Привязать сотрудников к:
                  </p>
                  <div class="options-group">
                    <label
                      v-if="hasOrganization"
                      class="binding-option"
                    >
                      <input
                        v-model="employeesBindToOrganization"
                        type="checkbox"
                      >
                      <span>Организации "{{ organization }}"</span>
                    </label>
                    <label
                      v-if="hasCompany"
                      class="binding-option"
                    >
                      <input
                        v-model="employeesBindToCompany"
                        type="checkbox"
                      >
                      <span>Компании "{{ company }}"</span>
                    </label>
                  </div>
                </div>
                <div
                  v-else
                  class="no-binding-message"
                >
                  Сотрудники "По факту" не требуют привязки к организации/компании
                </div>
              </div>
            </div>

            <div class="warning-section">
              <p class="warning-text">
                <strong class="warning-strong">Внимание!</strong> При привязке данных к организации или компании,
                они будут доступны для отображения и использования <strong>всем</strong> сотрудникам, которые в них числятся.
              </p>
            </div>

            <div class="modal-actions">
              <button
                class="btn skip-btn"
                :disabled="processing"
                @click="handleSkip"
              >
                <transition
                  name="fade"
                  mode="out-in"
                >
                  <span
                    :key="skipButtonText"
                    class="button-text"
                  >{{ skipButtonText }}</span>
                </transition>
              </button>
              <button
                class="btn confirm-btn"
                :disabled="processing"
                @click="handleConfirm"
              >
                <transition
                  name="fade"
                  mode="out-in"
                >
                  <span
                    :key="buttonText"
                    class="button-text"
                  >{{ buttonText }}</span>
                </transition>
              </button>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { ref } from 'vue';
import { useSwipeDismiss } from '@/composables/useSwipeDismiss';

export default {
    name: 'UniversalBindingModal',
    props: {
        show: {
            type: Boolean,
            default: false
        },
        newVehiclesToBind: {
            type: Array,
            default: () => []
        },
        newEmployeesToBind: {
            type: Array,
            default: () => []
        },
        organization: {
            type: String,
            default: ''
        },
        company: {
            type: String,
            default: ''
        },
        hasOrganization: {
            type: Boolean,
            default: false
        },
        hasCompany: {
            type: Boolean,
            default: false
        },
        // true с момента клика по "Привязать и отправить"/"Отправить без привязки" и до
        // завершения обработки родителем - блокирует свои кнопки и все пути закрытия
        // (крестик/оверлей/Escape/свайп), чтобы повторный клик/закрытие "вслепую" не
        // ушли вторым набором запросов поверх фонового прогона confirmBinding/skipBinding.
        processing: {
            type: Boolean,
            default: false
        }
    },
    emits: ['confirm-binding', 'skip-binding', 'close'],
    setup(props, { emit }) {
        // Контракт окна: свайп вниз за ползунок закрывает лист на мобилке. Пока идёт
        // обработка (processing) - жест не стартует вовсе, тот же гард, что у closeModal().
        const modalBody = ref(null);
        const swipe = useSwipeDismiss(() => emit('close'), {
            handleSelector: '.sheet-handle',
            getScrollTop: () => modalBody.value?.scrollTop ?? 0,
        });
        function onSheetTouchStart(e) {
            if (props.processing) return;
            swipe.onTouchStart(e);
        }
        return {
            modalBody,
            sheetOffset: swipe.offset,
            sheetDragging: swipe.isDragging,
            onSheetTouchStart,
            onSheetTouchMove: swipe.onTouchMove,
            onSheetTouchEnd: swipe.onTouchEnd,
        };
    },
    data() {
        return {
            vehiclesBindToOrganization: false,
            vehiclesBindToCompany: false,
            employeesBindToOrganization: false,
            employeesBindToCompany: false,
            // Какую именно кнопку нажали - чтобы "Отправляем..." показывалось на
            // кликнутой кнопке, а не на обеих сразу.
            pendingAction: null
        }
    },
    computed: {
        buttonText() {
            if (this.processing && this.pendingAction === 'confirm') return 'Отправляем...';
            const hasAnyBinding = this.vehiclesBindToOrganization || this.vehiclesBindToCompany ||
                                 this.employeesBindToOrganization || this.employeesBindToCompany;
            return hasAnyBinding ? 'Привязать и отправить' : 'Отправить заявку';
        },
        skipButtonText() {
            return (this.processing && this.pendingAction === 'skip') ? 'Отправляем...' : 'Отправить без привязки';
        },
        hasVehiclesForBinding() {
            return this.newVehiclesToBind.some(vehicle => !this.isVehicleByFact(vehicle));
        },
        hasEmployeesForBinding() {
            return this.newEmployeesToBind.some(employee => !this.isEmployeeByFact(employee));
        }
    },
    watch: {
        // Модалка теперь всегда смонтирована (для leave-анимации), поэтому
        // сбрасываем галочки при каждом открытии, как было при v-if.
        show(visible) {
            setBodyScrollLock(this, visible);
            if (visible) {
                this.vehiclesBindToOrganization = false;
                this.vehiclesBindToCompany = false;
                this.employeesBindToOrganization = false;
                this.employeesBindToCompany = false;
                this.pendingAction = null;
            }
        }
    },
    mounted() {
        document.addEventListener('keydown', this.handleKeydown);
    },
    beforeUnmount() {
        document.removeEventListener('keydown', this.handleKeydown);
        releaseBodyScrollLock(this);
    },
    methods: {
        handleKeydown(e) {
            if (e.key === 'Escape' && this.show) this.closeModal();
        },

        formatFullName(employee) {
            const parts = [];
            if (employee.lastName) parts.push(employee.lastName);
            if (employee.firstName) parts.push(employee.firstName);
            if (employee.middleName) parts.push(employee.middleName);
            return parts.join(' ') || 'Не указано';
        },
        isVehicleByFact(vehicle) {
            return vehicle.plateNumber === 'По факту' || vehicle.mark === 'По факту';
        },
        isEmployeeByFact(employee) {
            return employee.passportSeriesNumber === 'По факту' ||
                   employee.position === 'По факту';
        },
        closeModal() {
            // Пока обработка уже идёт (processing) - крестик/оверлей/Escape не закрывают
            // модалку: confirmBinding/skipBinding в родителе всё равно доведут дело до
            // sendCompleteApplication, а тихо "отменённая" модалка это бы замаскировала.
            if (this.processing) return;
            this.$emit('close');
        },
        handleConfirm() {
            if (this.processing) return;
            this.pendingAction = 'confirm';
            this.$emit('confirm-binding', {
                vehicles: {
                    bindToOrganization: this.vehiclesBindToOrganization,
                    bindToCompany: this.vehiclesBindToCompany,
                    hasVehiclesForBinding: this.hasVehiclesForBinding
                },
                employees: {
                    bindToOrganization: this.employeesBindToOrganization,
                    bindToCompany: this.employeesBindToCompany,
                    hasEmployeesForBinding: this.hasEmployeesForBinding
                }
            });
        },
        handleSkip() {
            if (this.processing) return;
            this.pendingAction = 'skip';
            this.$emit('skip-binding');
        }
    }
}
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
    transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
    opacity: 0;
    transform: scale(0.95);
}

.fade-enter-to,
.fade-leave-from {
    opacity: 1;
    transform: scale(1);
}

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
    animation: overlayAppear 0.4s ease-out;
}

@keyframes overlayAppear {
    from {
        background: var(--overlay);
        backdrop-filter: blur(0px);
    }
    to {
        background: var(--overlay);
        backdrop-filter: blur(0.1px);
    }
}

.modal-content {
    background: var(--surface);
    border-radius: 50px;
    width: 540px;
    max-width: 90vw;
    max-height: calc(var(--app-vh, 1vh) * 80);
    box-shadow: 0 20px 60px var(--shadow-drop);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    animation: contentAppear 0.4s cubic-bezier(0.25, 0.1, 0.15, 1);
}

@keyframes contentAppear {
    from {
        opacity: 0;
        transform: scale(0.9) translateY(-20px);
    }
    to {
        opacity: 1;
        transform: scale(1) translateY(0);
    }
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px 30px 16px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
}

.modal-title {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: var(--text);
}

.modal-close {
    background: none;
    border: none;
    cursor: pointer;
    padding: 6px;
    border-radius: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s ease;
}

.modal-close:hover {
    background-color: var(--surface-2);
}

.modal-close:disabled {
    opacity: 0.4;
    cursor: not-allowed;
}

.modal-close:disabled:hover {
    background-color: transparent;
}

.modal-body {
    padding: 20px 30px;
    overflow-y: auto;
    flex: 1;
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.modal-body::-webkit-scrollbar {
    display: none;
}

.binding-description {
    font-size: 14px;
    line-height: 1.5;
    color: var(--text);
    margin-bottom: 20px;
    padding-bottom: 12px;
    border-bottom: 1px solid var(--border);
}

.binding-description strong {
    color: var(--text);
    font-weight: 600;
}

.data-section {
    border: 1px solid var(--border);
    border-radius: 20px;
    background: var(--surface-2);
    margin-bottom: 20px;
    overflow: hidden;
}

.data-section:last-of-type {
    margin-bottom: 16px;
}

.section-header {
    padding: 12px 20px;
    border-bottom: 1px solid var(--border);
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.section-title {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    color: var(--text);
}

.count-badge {
    background: var(--border);
    padding: 2px 8px;
    border-radius: 20px;
    font-size: 12px;
    color: var(--text-muted);
}

.section-body {
    padding: 16px 20px;
}

.items-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 16px;
}

.item {
    padding: 10px 14px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    transition: all 0.2s ease;
}

.item-fact {
    background: var(--warning-bg);
    border-color: var(--warning);
}

.item-info {
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 13px;
    flex-wrap: wrap;
}

.item-number,
.item-name {
    font-weight: 500;
    color: var(--text);
}

.item-detail {
    color: var(--text-muted);
    background: var(--border);
    padding: 2px 8px;
    border-radius: 20px;
    font-size: 12px;
}

.fact-badge {
    background: var(--warning);
    color: var(--fill-text);
    font-size: 11px;
    font-weight: 500;
    padding: 2px 8px;
    border-radius: 20px;
    margin-left: auto;
}

.binding-options {
    margin-top: 12px;
    padding-top: 12px;
    border-top: 1px solid var(--border);
}

.options-title {
    font-size: 13px;
    font-weight: 500;
    color: var(--text);
    margin-bottom: 8px;
}

.options-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.binding-option {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    font-size: 13px;
    padding: 4px;
    border-radius: 20px;
    transition: background 0.2s;
}

.binding-option:hover {
    background: var(--border);
}

.binding-option input {
    width: 16px;
    height: 16px;
    cursor: pointer;
    accent-color: var(--accent-text);
}

.binding-option span {
    color: var(--text);
}

.no-binding-message {
    font-size: 12px;
    color: var(--text-muted);
    font-style: italic;
    padding: 8px 12px;
    background: var(--surface-2);
    border-radius: 20px;
    border-left: 3px solid var(--warning);
    margin-top: 8px;
}

.warning-section {
    background: var(--warning-bg);
    border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--surface));
    border-radius: 20px;
    padding: 12px 20px;
    margin: 8px 0 20px;
}

.warning-text {
    margin: 0;
    font-size: 12px;
    line-height: 1.5;
    color: var(--warning-text);
}

.warning-strong {
    font-weight: 600;
}

.modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    margin-top: 16px;
}

.btn {
    padding: 8px 20px;
    font-size: 13px;
    font-weight: 500;
    border-radius: 30px;
    cursor: pointer;
    border: 1px solid;
    transition: all 0.2s ease;
}

.btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
}

.skip-btn {
    background: var(--surface);
    color: var(--text-muted);
    border-color: var(--border);
}

.skip-btn:hover:not(:disabled) {
    background: var(--surface-2);
    border-color: var(--border);
    color: var(--text);
}

.confirm-btn {
    background: var(--accent);
    color: var(--accent-contrast);
    border-color: var(--accent);
    width: 205px;
    min-width: 200px;
    position: relative;
    overflow: hidden;
}

.confirm-btn:hover:not(:disabled) {
    background: var(--accent-hover);
    border-color: var(--accent-hover);
}

.button-text {
    display: inline-block;
}

@media (max-width: 768px) {
    /* Размеры листа приходят из глобального .modal-content (App.vue) с !important -
       локальные width/max-height/radius здесь были мёртвыми и вводили в заблуждение. */

    .modal-header {
        padding: 16px 20px;
    }

    .modal-body {
        padding: 16px 20px;
    }

    .section-header {
        padding: 10px 16px;
    }

    .section-body {
        padding: 14px 16px;
    }

    .item-info {
        flex-direction: column;
        align-items: flex-start;
        gap: 6px;
    }

    .fact-badge {
        margin-left: 0;
        align-self: flex-start;
    }

    .modal-actions {
        flex-direction: column;
        gap: 8px;
    }

    .btn {
        width: 100%;
        min-width: auto;
    }

    .confirm-btn {
        width: 100%;
        min-width: auto;
    }
}

.modal-fade-enter-active,
.modal-fade-leave-active {
    transition: opacity 0.25s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
    opacity: 0;
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
