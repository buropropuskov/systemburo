<template>
  <BaseModal :show="show" title="Восстановление доступа" @close="$emit('close')">
    <p class="recovery-text">Для восстановления доступа обратитесь к администратору:</p>

    <div class="contact-list">
      <button class="contact-item" @click="copyToClipboard('buropropuskov@dreamisland.ru')">
        <span class="contact-icon">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="2" y="4" width="20" height="16" rx="2" />
            <path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7" />
          </svg>
        </span>
        <span class="contact-value">buropropuskov@dreamisland.ru</span>
        <span class="contact-hint">Нажмите, чтобы скопировать</span>
      </button>

      <button class="contact-item" @click="copyToClipboard('+7 (910) 083 00-55')">
        <span class="contact-icon">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72 12.84 12.84 0 0 0 .7 2.81 2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45 12.84 12.84 0 0 0 2.81.7A2 2 0 0 1 22 16.92z" />
          </svg>
        </span>
        <span class="contact-value">+7 (910) 083 00-55</span>
        <span class="contact-hint">Нажмите, чтобы скопировать</span>
      </button>
    </div>

    <transition name="toast-fade">
      <div v-if="showCopiedToast" class="copied-toast">Скопировано</div>
    </transition>

    <template #actions>
      <button class="btn btn--primary" @click="$emit('close')">Понятно</button>
    </template>
  </BaseModal>
</template>

<script>
import BaseModal from '@/components/ui/BaseModal.vue'

export default {
  name: 'PasswordRecoveryModal',
  components: { BaseModal },

  props: {
    show: {
      type: Boolean,
      required: true,
    },
  },

  emits: ['close'],

  data() {
    return {
      showCopiedToast: false,
      toastTimer: null,
    }
  },

  methods: {
    async copyToClipboard(text) {
      try {
        await navigator.clipboard.writeText(text)
      } catch {
        const textarea = document.createElement('textarea')
        textarea.value = text
        textarea.style.position = 'fixed'
        textarea.style.opacity = '0'
        document.body.appendChild(textarea)
        textarea.select()
        document.execCommand('copy')
        document.body.removeChild(textarea)
      }

      clearTimeout(this.toastTimer)
      this.showCopiedToast = true
      this.toastTimer = setTimeout(() => {
        this.showCopiedToast = false
      }, 2000)
    },
  },

  beforeUnmount() {
    clearTimeout(this.toastTimer)
  },
}
</script>

<style scoped>
.recovery-text {
  margin: 0 0 16px;
  font-size: 14px;
  line-height: 1.5;
  color: var(--color-text);
}

.contact-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.contact-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  background: var(--color-bg-secondary);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all 0.2s;
  width: 100%;
  text-align: left;
}

.contact-item:hover {
  background: var(--color-bg);
  border-color: var(--color-primary);
}

.contact-icon {
  flex-shrink: 0;
  color: var(--color-primary);
  display: flex;
  align-items: center;
}

.contact-value {
  flex: 1;
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text);
}

.contact-hint {
  font-size: 12px;
  color: var(--color-text-muted);
  white-space: nowrap;
}

.copied-toast {
  margin-top: 12px;
  padding: 8px 16px;
  background-color: var(--color-success);
  color: #fff;
  font-size: 13px;
  font-weight: 500;
  border-radius: var(--radius-sm);
  text-align: center;
}

.toast-fade-enter-active,
.toast-fade-leave-active {
  transition: opacity 0.3s;
}

.toast-fade-enter-from,
.toast-fade-leave-to {
  opacity: 0;
}

.btn {
  padding: 10px 24px;
  border: none;
  border-radius: var(--radius-sm);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s;
}

.btn--primary {
  background-color: var(--color-primary);
  color: #fff;
}

.btn--primary:hover {
  background-color: var(--color-primary-hover);
}

@media (max-width: 480px) {
  .contact-hint {
    display: none;
  }
}
</style>
