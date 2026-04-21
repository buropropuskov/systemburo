<template>
  <BaseModal
    :show="show"
    :closable="false"
    width="560px"
    @close="$emit('close')"
  >
    <template #header>
      <h2 class="recovery-title">Восстановление доступа</h2>
    </template>

    <p class="recovery-text">
      Если вы забыли логин или пароль учётной записи, напишите нам или позвоните:
    </p>

    <div class="recovery-contacts">
      <button type="button" class="recovery-contact" @click.stop="copyEmail" data-testid="recovery-copy-email">
        <img src="@/assets/icons/email-blue.png" class="recovery-contact__icon" alt="" />
        <span class="recovery-contact__text">buropropuskov@dreamisland.ru</span>
      </button>
      <button type="button" class="recovery-contact" @click.stop="copyPhone" data-testid="recovery-copy-phone">
        <img src="@/assets/icons/phone-blue.png" class="recovery-contact__icon" alt="" />
        <span class="recovery-contact__text">+7 (910) 083 00-55</span>
      </button>
    </div>

    <div class="recovery-notifications">
      <transition name="recovery-notification" mode="out-in">
        <div v-if="showNotification" :key="notificationText" class="recovery-notification" data-testid="recovery-notification">
          {{ notificationText }}
        </div>
      </transition>
    </div>

    <template #actions>
      <button type="button" class="recovery-button" @click="$emit('close')" data-testid="recovery-close">
        Понятно
      </button>
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
      showNotification: false,
      notificationText: '',
      notificationTimeout: null,
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
    },

    showNotificationMessage(text) {
      if (this.notificationTimeout) {
        clearTimeout(this.notificationTimeout)
        this.notificationTimeout = null
      }
      this.notificationText = text
      this.showNotification = true
      this.notificationTimeout = setTimeout(() => {
        this.showNotification = false
        this.notificationTimeout = null
      }, 2000)
    },

    async copyEmail() {
      await this.copyToClipboard('buropropuskov@dreamisland.ru')
      this.showNotificationMessage('E-mail скопирован')
    },

    async copyPhone() {
      await this.copyToClipboard('+7 (910) 083 00-55')
      this.showNotificationMessage('Номер телефона скопирован')
    },
  },

  beforeUnmount() {
    if (this.notificationTimeout) {
      clearTimeout(this.notificationTimeout)
    }
  },
}
</script>

<style scoped>
/* BaseModal header/actions overrides — убираем border между секциями
   и выравниваем по old_branch-стилю (цельный плоский контент). */
:deep(.base-modal__header) {
  border-bottom: none;
  padding: 24px 32px 16px;
}

:deep(.base-modal__body) {
  padding: 8px 32px 16px;
}

:deep(.base-modal__actions) {
  justify-content: center;
  border-top: none;
  padding: 0 32px 28px;
}

.recovery-title {
  margin: 0;
  font-size: 22px;
  font-weight: 800;
  color: #333;
  padding: 10px 24px;
  text-align: center;
  border: 2px solid var(--color-border);
  border-radius: 40px;
  width: 100%;
}

.recovery-text {
  margin: 0 0 24px;
  font-size: 15px;
  line-height: 1.5;
  color: #000;
  text-align: center;
}

.recovery-contacts {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.recovery-contact {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 6px 10px;
  background: transparent;
  border: none;
  cursor: pointer;
  transition: opacity 0.2s ease;
}

.recovery-contact:hover {
  opacity: 0.8;
}

.recovery-contact__icon {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
}

.recovery-contact__text {
  font-size: 15px;
  color: var(--color-primary);
  font-weight: 500;
}

.recovery-contact:hover .recovery-contact__text {
  text-decoration: underline;
  text-underline-position: under;
}

.recovery-notifications {
  position: relative;
  display: flex;
  justify-content: center;
  min-height: 28px;
  margin-top: 8px;
}

.recovery-notification {
  padding: 4px 16px;
  border-radius: 40px;
  background-color: #fff;
  font-size: 13px;
  color: #000;
  font-weight: 500;
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.15);
  min-width: 160px;
  text-align: center;
  white-space: nowrap;
}

.recovery-notification-enter-active,
.recovery-notification-leave-active {
  transition: opacity 0.3s ease, transform 0.3s ease;
}

.recovery-notification-enter-from,
.recovery-notification-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

.recovery-notification-enter-to,
.recovery-notification-leave-from {
  opacity: 1;
  transform: translateY(0);
}

.recovery-button {
  background: var(--color-primary);
  color: #fff;
  border: none;
  border-radius: 40px;
  padding: 12px 40px;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  min-width: 200px;
  margin: 0 auto;
  transition: background-color 0.2s;
}

.recovery-button:hover {
  background-color: #3f4bc9;
}
</style>
