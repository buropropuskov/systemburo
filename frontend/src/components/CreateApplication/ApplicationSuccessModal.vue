<template>
    <transition name="modal-fade">
        <div v-if="show" class="modal-overlay" @click.self="handleClose">
            <div class="modal-content">
                <div class="modal-header">
                    <button class="modal-close" @click="handleClose">
                        <svg width="10" height="10" viewBox="0 0 14 14" fill="none">
                            <path d="M13 1L1 13M1 1L13 13" stroke="#666" stroke-width="2" stroke-linecap="round"/>
                        </svg>
                    </button>
                </div>

                <div class="modal-body">
                    <h2 class="modal-title">Заявка успешно оформлена!</h2>
                    
                    <div class="application-number">
                        <span class="label">Номер заявки:</span>
                        <strong class="number">{{ applicationNumber }}</strong>
                    </div>

                    <div class="info-message">
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                            <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z" fill="#4F5BDF"/>
                        </svg>
                        <span>Скоро мы начнём её обрабатывать</span>
                    </div>

                    <!-- Интересный прогресс-бар с анимированными элементами -->
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
                                        <span v-if="index < currentStep" class="check-icon">✓</span>
                                        <span v-else class="step-number">{{ index + 1 }}</span>
                                    </div>
                                    <div class="step-glow"></div>
                                </div>
                                <div class="step-label">{{ step }}</div>
                            </div>
                        </div>
                        <div class="progress-track">
                            <div class="progress-fill" :style="{ width: progressWidth + '%' }">
                                <div class="progress-sparkle"></div>
                            </div>
                        </div>
                    </div>

                    <!-- Вложения -->
                    <div class="attachments-section">
                        <div class="section-header">
                            <h3 class="section-title">Вложения в заявке</h3>
                            <span class="attachments-count">{{ attachmentsData.length }}</span>
                        </div>
                        
                        <div v-if="attachmentsData.length === 0" class="no-attachments">
                            <p>Нет вложений</p>
                        </div>
                        
                        <div v-else class="attachments-list">
                            <div
                                v-for="(att, idx) in attachmentsData"
                                :key="idx"
                                class="attachment-item"
                            >
                                <div class="attachment-content">
                                    <div class="attachment-name">{{ att.display_name }}</div>
                                    <div class="attachment-details">
                                        <span v-if="att.period" class="attachment-period">
                                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none">
                                                <path d="M19 3h-1V1h-2v2H8V1H6v2H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm0 16H5V9h14v10z" fill="#a2a2a2"/>
                                            </svg>
                                            {{ att.period }}
                                        </span>
                                        <span v-if="att.time" class="attachment-time">
                                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none">
                                                <path d="M11.99 2C6.47 2 2 6.48 2 12s4.47 10 9.99 10C17.52 22 22 17.52 22 12S17.52 2 11.99 2zM12 20c-4.42 0-8-3.58-8-8s3.58-8 8-8 8 3.58 8 8-3.58 8-8 8zm.5-13H11v6l5.25 3.15.75-1.23-4.5-2.67z" fill="#a2a2a2"/>
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
                    <button class="btn close-btn" @click="handleClose">Закрыть</button>
                </div>
            </div>
        </div>
    </transition>
</template>

<script>
export default {
    name: 'ApplicationSuccessModal',
    props: {
        show: { type: Boolean, default: false },
        applicationNumber: { type: String, default: '' },
        attachmentsData: { type: Array, default: () => [] }
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
    methods: {
        handleClose() {
            this.$emit('close')
        }
    }
}
</script>

<style scoped>
/* Анимация */
.modal-fade-enter-active,
.modal-fade-leave-active {
    transition: all 0.4s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
    opacity: 0;
}

.modal-fade-enter-active .modal-overlay,
.modal-fade-leave-active .modal-overlay {
    transition: all 0.4s ease;
}

.modal-fade-enter-active .modal-content,
.modal-fade-leave-active .modal-content {
    transition: all 0.4s ease;
}

.modal-fade-enter-from .modal-overlay,
.modal-fade-leave-to .modal-overlay {
    background: rgba(0, 0, 0, 0);
    backdrop-filter: blur(0px);
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
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    backdrop-filter: blur(1px);
}

.modal-content {
    background: #fff;
    border-radius: 50px;
    width: 540px;
    max-width: 90vw;
    max-height: 80vh;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
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
    background-color: #f5f5f5;
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
    color: #1a1a1a;
    text-align: center;
    margin: 0 0 20px 0;
}

.application-number {
    background: #f8f9fa;
    border-radius: 20px;
    padding: 12px 20px;
    text-align: center;
    margin-bottom: 20px;
    border: 1px solid #e6e6e6;
}

.application-number .label {
    font-size: 13px;
    color: #666;
    margin-right: 8px;
}

.application-number .number {
    font-size: 18px;
    font-weight: 700;
    color: #4F5BDF;
    letter-spacing: 0.5px;
}

.info-message {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    background: #f0f4ff;
    border-radius: 20px;
    padding: 10px 16px;
    margin-bottom: 28px;
    color: #4F5BDF;
    font-size: 13px;
}

.info-message svg {
    flex-shrink: 0;
}

/* Интересный прогресс-бар */
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
    background: white;
    border: 2px solid #e6e6e6;
    color: #a2a2a2;
    width: 100%;
    height: 100%;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.3s ease;
}

.check-icon {
    background: #4F5BDF;
    color: white;
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
    background: radial-gradient(circle, rgba(79, 91, 223, 0.3) 0%, rgba(79, 91, 223, 0) 70%);
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
    border-color: #4F5BDF;
    background: #4F5BDF;
    color: white;
    transform: scale(1.05);
}

.progress-step.active .step-number {
    border-color: #4F5BDF;
    color: #4F5BDF;
    background: white;
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
    color: #a2a2a2;
    text-align: center;
    transition: all 0.3s ease;
    white-space: nowrap;
}

.progress-step.completed .step-label {
    color: #4F5BDF;
    font-weight: 600;
}

.progress-step.active .step-label {
    color: #4F5BDF;
    font-weight: 600;
}

.progress-track {
    position: absolute;
    top: 20px;
    left: 0;
    right: 0;
    height: 3px;
    background: #e6e6e6;
    border-radius: 2px;
    z-index: 1;
}

.progress-fill {
    position: absolute;
    left: 0;
    top: 0;
    height: 100%;
    background: linear-gradient(90deg, #4F5BDF, #8B9AFF);
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

/* Вложения */
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
    color: #333;
    margin: 0;
}

.attachments-count {
    background: #e6e6e6;
    padding: 2px 8px;
    border-radius: 20px;
    font-size: 12px;
    color: #666;
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
    background: #fafafa;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    transition: all 0.2s ease;
    cursor: pointer;
}

.attachment-item:hover {
    border-color: #4F5BDF;
    background: white;
    transform: translateX(4px);
}

.attachment-content {
    flex: 1;
}

.attachment-name {
    font-weight: 600;
    font-size: 14px;
    color: #333;
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
    color: #a2a2a2;
}

.attachment-period svg,
.attachment-time svg {
    flex-shrink: 0;
}

.no-attachments {
    text-align: center;
    padding: 40px 20px;
    background: #fafafa;
    border-radius: 20px;
    border: 1px solid #e6e6e6;
}

.no-attachments p {
    margin: 0;
    font-size: 13px;
    color: #a2a2a2;
}

.modal-footer {
    padding: 16px 30px 24px;
    border-top: 1px solid #f0f0f0;
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
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
    min-width: 140px;
}

.close-btn:hover {
    background: #3a45c0;
    border-color: #3a45c0;
}

@media (max-width: 768px) {
    .modal-content {
        width: 95vw;
        margin: 16px;
        max-height: 85vh;
        border-radius: 30px;
    }

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
</style>