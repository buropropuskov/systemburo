<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="modal-overlay"
        @click.self="handleClose"
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
            <button
              class="modal-close"
              @click="handleClose"
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
            <h2 class="modal-title">
              Заявка успешно оформлена!
            </h2>

            <div class="application-number">
              <span class="label">Номер заявки:</span>
              <strong
                class="number number--copyable"
                data-tooltip="Копировать"
                role="button"
                tabindex="0"
                @click="copyNumber"
                @keydown.enter.prevent="copyNumber"
              >{{ applicationNumber }}</strong>
            </div>

            <div class="info-message">
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
              >
                <circle
                  cx="12"
                  cy="12"
                  r="8.5"
                  stroke="currentColor"
                  stroke-width="1.7"
                />
                <line
                  x1="12"
                  y1="11.2"
                  x2="12"
                  y2="16.5"
                  stroke="currentColor"
                  stroke-width="1.7"
                  stroke-linecap="round"
                />
                <circle
                  cx="12"
                  cy="7.9"
                  r="1"
                  fill="currentColor"
                />
              </svg>
              <span>Скоро мы начнём её обрабатывать</span>
            </div>

            <div class="progress-container">
              <div class="progress-steps">
                <div
                  v-for="(step, index) in steps"
                  :key="index"
                  class="progress-step"
                  :class="{
                    completed: index < currentStep,
                    active: index === currentStep
                  }"
                >
                  <div class="step-icon">
                    <div class="step-circle">
                      <span
                        v-if="index < currentStep"
                        class="check-icon"
                      >+</span>
                      <span
                        v-else
                        class="step-number"
                      >{{ index + 1 }}</span>
                    </div>
                    <div class="step-glow" />
                  </div>
                  <div class="step-label">
                    {{ step }}
                  </div>
                </div>
              </div>
              <div class="progress-track">
                <div
                  class="progress-fill"
                  :style="{ width: progressWidth + '%' }"
                >
                  <div class="progress-sparkle" />
                </div>
              </div>
            </div>

            <div class="attachments-section">
              <div class="section-header">
                <h3 class="section-title">
                  Вложения в заявке
                </h3>
                <span class="attachments-count">{{ attachmentsData.length }}</span>
              </div>

              <div
                v-if="attachmentsData.length === 0"
                class="no-attachments"
              >
                <p>Нет вложений</p>
              </div>

              <div
                v-else
                class="attachments-list"
              >
                <div
                  v-for="(att, idx) in attachmentsData"
                  :key="idx"
                  class="attachment-item"
                >
                  <div class="attachment-content">
                    <div class="attachment-name">
                      {{ att.display_name }}
                    </div>
                    <div class="attachment-details">
                      <span
                        v-if="att.period"
                        class="attachment-period"
                      >
                        <svg
                          width="12"
                          height="12"
                          viewBox="0 0 24 24"
                          fill="none"
                        >
                          <g
                            fill="none"
                            stroke="currentColor"
                            stroke-width="1.8"
                            stroke-linecap="round"
                          >
                            <rect
                              x="3.5"
                              y="5.5"
                              width="17"
                              height="15"
                              rx="2.5"
                            />
                            <line
                              x1="3.5"
                              y1="10.2"
                              x2="20.5"
                              y2="10.2"
                            />
                            <line
                              x1="8"
                              y1="3.4"
                              x2="8"
                              y2="7.2"
                            />
                            <line
                              x1="16"
                              y1="3.4"
                              x2="16"
                              y2="7.2"
                            />
                          </g>
                        </svg>
                        {{ att.period }}
                      </span>
                      <span
                        v-if="att.time"
                        class="attachment-time"
                      >
                        <svg
                          width="12"
                          height="12"
                          viewBox="0 0 24 24"
                          fill="none"
                        >
                          <g
                            fill="none"
                            stroke="currentColor"
                            stroke-width="1.8"
                            stroke-linecap="round"
                            stroke-linejoin="round"
                          >
                            <circle
                              cx="12"
                              cy="12"
                              r="8.5"
                            />
                            <polyline points="12 7 12 12 15.6 14.2" />
                          </g>
                        </svg>
                        {{ att.time }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="modal-footer">
            <button
              class="btn close-btn"
              @click="handleClose"
            >
              Закрыть
            </button>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script>
import { setBodyScrollLock, releaseBodyScrollLock } from '@/utils/bodyScrollLock';
import { ref } from 'vue';
import { useDeletionsStore } from '@/stores/deletions';
import { copyText } from '@/utils/clipboard';
import { useSwipeDismiss } from '@/composables/useSwipeDismiss';

export default {
    name: 'ApplicationSuccessModal',
    props: {
        show: { type: Boolean, default: false },
        applicationNumber: { type: String, default: '' },
        attachmentsData: { type: Array, default: () => [] }
    },
    emits: ['close'],
    setup(_, { emit }) {
        // Контракт окна: свайп вниз за ползунок закрывает лист на мобилке.
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
            steps: ['Оформлена', 'В обработке', 'Согласование', 'В работе', 'Завершена'],
            currentStep: 0
        }
    },
    computed: {
        progressWidth() {
            return ((this.currentStep + 1) / this.steps.length) * 100
        }
    },
    watch: {
        show(visible) {
            setBodyScrollLock(this, visible);
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
            if (e.key === 'Escape' && this.show) this.handleClose();
        },

        handleClose() {
            this.$emit('close')
        },
        async copyNumber() {
            const number = this.applicationNumber;
            if (!number) return;
            const copied = await copyText(number);
            useDeletionsStore().notify(copied
                ? { prefix: 'Номер ', bold: String(number), suffix: ' скопирован', type: 'success' }
                : { prefix: 'Не удалось ', bold: 'скопировать номер', type: 'error' });
        }
    }
}
</script>

<style scoped>
.modal-fade-enter-active,
.modal-fade-leave-active {
    transition: all 0.4s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
    opacity: 0;
}

.modal-fade-enter-from .modal-content,
.modal-fade-leave-to .modal-content {
    opacity: 0;
    transform: scale(0.9) translateY(-20px);
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
    backdrop-filter: blur(1px);
}

.modal-content {
    background: var(--surface);
    border-radius: 50px;
    width: 540px;
    max-width: 90vw;
    max-height: calc(var(--app-vh, 1vh) * 91);
    box-shadow: 0 20px 60px var(--shadow-drop);
    display: flex;
    flex-direction: column;
    overflow: hidden;
}

.modal-header {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    padding: 20px 30px 0;
    flex-shrink: 0;
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
    transition: background-color 0.2s ease;
}

.modal-close:hover {
    background-color: var(--surface-2);
}

.modal-body {
    padding: 0 30px 20px;
    overflow-y: auto;
    flex: 1;
    scrollbar-width: none;
    -ms-overflow-style: none;
}

.modal-body::-webkit-scrollbar {
    display: none;
}

.modal-title {
    font-size: 24px;
    font-weight: 700;
    color: var(--text);
    text-align: center;
    margin: 0 0 20px 0;
}

.application-number {
    background: var(--surface-2);
    border-radius: 20px;
    padding: 12px 20px;
    text-align: center;
    margin-bottom: 20px;
    border: 1px solid var(--border);
}

.application-number .label {
    font-size: 13px;
    color: var(--text-muted);
    margin-right: 8px;
}

.application-number .number {
    font-size: 18px;
    font-weight: 700;
    color: var(--accent-text);
    letter-spacing: 0.5px;
}

.application-number .number--copyable {
    position: relative;
    cursor: pointer;
    user-select: none;
    border-radius: 50px;
    outline: none;
    padding: 2px 6px;
    transition: background-color 0.15s;
}

.application-number .number--copyable:hover {
    background-color: color-mix(in srgb, var(--accent) 10%, var(--surface));
}

.application-number .number--copyable:focus-visible {
    box-shadow: 0 0 0 2px rgba(79, 91, 223, 0.4);
}

.application-number .number--copyable::after {
    content: attr(data-tooltip);
    position: absolute;
    top: calc(100% + 6px);
    left: 50%;
    transform: translateX(-50%);
    background: var(--hint-bg);
    color: var(--hint-text);
    padding: 4px 8px;
    border-radius: 6px;
    font-size: 11px;
    font-weight: 500;
    white-space: nowrap;
    z-index: 1000;
    pointer-events: none;
    opacity: 0;
    transition: opacity 0.15s;
    box-shadow: 0 2px 8px var(--shadow-drop);
}

.application-number .number--copyable:hover::after {
    opacity: 1;
}

.info-message {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    background: var(--accent-tint);
    border-radius: 20px;
    padding: 10px 16px;
    margin-bottom: 28px;
    color: var(--accent-text);
    font-size: 13px;
}

.info-message svg {
    flex-shrink: 0;
}

.progress-container {
    position: relative;
    margin-bottom: 28px;
}

.progress-steps {
    display: flex;
    justify-content: space-between;
    position: relative;
    z-index: 2;
    margin-bottom: 20px;
}

.progress-step {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    position: relative;
}

.step-icon {
    position: relative;
    width: 40px;
    height: 40px;
}

.step-circle {
    width: 40px;
    height: 40px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 14px;
    font-weight: 600;
    position: relative;
    z-index: 2;
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.step-number {
    background: var(--surface);
    border: 2px solid var(--border);
    color: var(--text-muted);
    width: 100%;
    height: 100%;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.3s ease;
}

.check-icon {
    background: var(--accent);
    color: var(--accent-contrast);
    width: 100%;
    height: 100%;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    animation: checkPop 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
    font-size: 18px;
    font-weight: bold;
    position: relative;
    z-index: 2;
}

.step-glow {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    border-radius: 50%;
    background: radial-gradient(circle, color-mix(in srgb, var(--accent) 30%, var(--surface)) 0%, color-mix(in srgb, var(--accent) 8%, var(--surface)) 70%);
    opacity: 0;
    transition: opacity 0.3s ease;
    pointer-events: none;
}

.progress-step.active .step-glow {
    opacity: 1;
    animation: glowPulse 1.5s ease-in-out infinite;
}

@keyframes glowPulse {
    0%, 100% {
        transform: scale(1);
        opacity: 0.3;
    }
    50% {
        transform: scale(1.3);
        opacity: 0.6;
    }
}

@keyframes checkPop {
    0% {
        transform: scale(0.8);
        opacity: 0;
    }
    100% {
        transform: scale(1);
        opacity: 1;
    }
}

.progress-step.completed .step-number {
    border-color: var(--accent);
    background: var(--accent);
    color: var(--accent-contrast);
    transform: scale(1.05);
}

.progress-step.active .step-number {
    border-color: var(--accent);
    color: var(--accent-text);
    background: var(--surface);
    animation: pulseBorder 1.5s ease-in-out infinite;
    box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.2);
}

@keyframes pulseBorder {
    0%, 100% {
        box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.2);
        transform: scale(1);
    }
    50% {
        box-shadow: 0 0 0 6px rgba(79, 91, 223, 0.1);
        transform: scale(1.02);
    }
}

.step-label {
    font-size: 11px;
    font-weight: 500;
    color: var(--text-muted);
    text-align: center;
    transition: all 0.3s ease;
    white-space: nowrap;
}

.progress-step.completed .step-label {
    color: var(--accent-text);
    font-weight: 600;
}

.progress-step.active .step-label {
    color: var(--accent-text);
    font-weight: 600;
}

.progress-track {
    position: absolute;
    top: 20px;
    left: 0;
    right: 0;
    height: 3px;
    background: var(--border);
    border-radius: 2px;
    z-index: 1;
}

.progress-fill {
    position: absolute;
    left: 0;
    top: 0;
    height: 100%;
    background: linear-gradient(90deg, var(--accent), color-mix(in srgb, var(--accent) 55%, var(--surface)));
    border-radius: 2px;
    transition: width 0.5s cubic-bezier(0.4, 0, 0.2, 1);
    overflow: hidden;
}

.progress-sparkle {
    position: absolute;
    top: 0;
    right: 0;
    width: 20px;
    height: 100%;
    background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.8), transparent);
    animation: sparkleMove 1s ease-in-out infinite;
}

@keyframes sparkleMove {
    0% {
        transform: translateX(-100%);
        opacity: 0;
    }
    50% {
        opacity: 0.5;
    }
    100% {
        transform: translateX(200%);
        opacity: 0;
    }
}

.attachments-section {
    margin-top: 8px;
}

.section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
    padding: 0 4px;
}

.section-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--text);
    margin: 0;
}

.attachments-count {
    background: var(--border);
    padding: 2px 8px;
    border-radius: 20px;
    font-size: 12px;
    color: var(--text-muted);
}

.attachments-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
    max-height: 240px;
    overflow-y: auto;
    padding-right: 4px;
}

.attachment-item {
    padding: 12px 16px;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: 20px;
    transition: all 0.2s ease;
}

.attachment-content {
    flex: 1;
}

.attachment-name {
    font-weight: 600;
    font-size: 14px;
    color: var(--text);
    margin-bottom: 6px;
}

.attachment-details {
    display: flex;
    gap: 12px;
    flex-wrap: wrap;
}

.attachment-period,
.attachment-time {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: 11px;
    color: var(--text-muted);
}

.attachment-period svg,
.attachment-time svg {
    flex-shrink: 0;
}

.no-attachments {
    text-align: center;
    padding: 40px 20px;
    background: var(--surface-2);
    border-radius: 20px;
    border: 1px solid var(--border);
}

.no-attachments p {
    margin: 0;
    font-size: 13px;
    color: var(--text-muted);
}

.modal-footer {
    padding: 16px 30px 24px;
    border-top: 1px solid var(--border);
    display: flex;
    justify-content: center;
    flex-shrink: 0;
}

.btn {
    padding: 10px 32px;
    font-size: 14px;
    font-weight: 500;
    border-radius: 30px;
    cursor: pointer;
    border: 1px solid;
    transition: background-color 0.2s ease;
}

.close-btn {
    background: var(--accent);
    color: var(--accent-contrast);
    border-color: var(--accent);
    min-width: 140px;
}

.close-btn:hover {
    background: var(--accent-hover);
    border-color: var(--accent-hover);
}

@media (max-width: 768px) {
    /* Размеры листа приходят из глобального .modal-content (App.vue) с !important -
       локальные width/max-height/radius здесь были мёртвыми и вводили в заблуждение. */

    .modal-header {
        padding: 16px 20px 0;
    }

    .modal-body {
        padding: 0 20px 16px;
    }

    .modal-footer {
        padding: 16px 20px 20px;
    }

    .modal-title {
        font-size: 18px;
        margin-bottom: 16px;
    }

    .step-icon {
        width: 32px;
        height: 32px;
    }

    .step-circle {
        width: 32px;
        height: 32px;
        font-size: 12px;
    }

    .check-icon {
        font-size: 14px;
    }

    .step-label {
        font-size: 9px;
        white-space: normal;
        word-break: break-word;
        line-height: 1.2;
    }

    .progress-track {
        top: 16px;
    }

    .attachment-item {
        padding: 10px 12px;
    }

    .attachment-name {
        font-size: 13px;
    }

    .btn {
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
