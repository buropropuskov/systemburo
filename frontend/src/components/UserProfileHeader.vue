<template>
  <div class="account-header">    
    <div class="user-info">
      <!-- Основная информация -->
      <div class="main-info">
        <div class="name-and-type">
          <h2
            class="user-name"
            data-testid="cabinet-text-username"
          >
            {{ displayName }}
          </h2>
          <span class="user-type-badge">{{ userTypeDisplay }}</span>
        </div>
        <div class="org-company-block">
          <h3
            v-if="fullName && organization"
            class="user-organization"
          >
            {{ organization }}
          </h3>
          <div
            v-if="company"
            class="user-company"
          >
            <svg
              class="icon"
              viewBox="0 0 24 24"
            >
              <path d="M18,15H16V17H18M18,11H16V13H18M20,19H12V17H14V15H12V13H14V11H12V9H20M10,7H8V5H10M10,11H8V9H10M10,15H8V13H10M10,19H8V17H10M6,7H4V5H6M6,11H4V9H6M6,15H4V13H6M6,19H4V17H6M12,7V3H2V21H22V7H12Z" />
            </svg>
            {{ company }}
          </div>
        </div>
      </div>
      
      <!-- Дополнительные данные -->
      <div
        v-if="hasContactDetails"
        class="user-details-row"
      >
        <div
          v-if="position"
          class="user-detail"
        >
          <span class="detail-badge position-badge">
            <svg
              class="icon"
              viewBox="0 0 24 24"
            >
              <path d="M12,3L2,12H5V20H19V12H22L12,3M12,7.7C14.1,7.7 15.8,9.4 15.8,11.5C15.8,14.5 12,18 12,18C12,18 8.2,14.5 8.2,11.5C8.2,9.4 9.9,7.7 12,7.7M12,10A1.5,1.5 0 0,0 10.5,11.5A1.5,1.5 0 0,0 12,13A1.5,1.5 0 0,0 13.5,11.5A1.5,1.5 0 0,0 12,10Z" />
            </svg>
            {{ position }}
          </span>
        </div>
        <div
          v-if="email"
          class="user-detail"
          @click="copyEmail"
        >
          <span
            ref="emailBadge"
            class="detail-badge email-badge clickable"
            :style="{ width: emailBadgeWidth ? emailBadgeWidth + 'px' : null }"
          >
            <transition
              name="fade"
              mode="out-in"
            >
              <div
                v-if="!copiedEmail"
                key="original"
                class="badge-content"
              >
                <svg
                  class="icon"
                  viewBox="0 0 24 24"
                >
                  <path d="M22 6C22 4.9 21.1 4 20 4H4C2.9 4 2 4.9 2 6V18C2 19.1 2.9 20 4 20H20C21.1 20 22 19.1 22 18V6M20 6L12 11L4 6H20M20 18H4V8L12 13L20 8V18Z" />
                </svg>
                <span class="badge-text">{{ email }}</span>
              </div>
              <div
                v-else
                key="copied"
                class="badge-content"
              >
                <span class="badge-text">Почта скопирована!</span>
              </div>
            </transition>
          </span>
        </div>
        <div
          v-if="phone"
          class="user-detail"
          @click="copyPhone"
        >
          <span
            ref="phoneBadge"
            class="detail-badge phone-badge clickable"
            :style="{ width: phoneBadgeWidth ? phoneBadgeWidth + 'px' : null }"
          >
            <transition
              name="fade"
              mode="out-in"
            >
              <div
                v-if="!copiedPhone"
                key="original"
                class="badge-content"
              >
                <svg
                  class="icon"
                  viewBox="0 0 24 24"
                >
                  <path d="M6.62,10.79C8.06,13.62 10.38,15.94 13.21,17.38L15.41,15.18C15.69,14.9 16.08,14.82 16.43,14.93C17.55,15.3 18.75,15.5 20,15.5A1,1 0 0,1 21,16.5V20A1,1 0 0,1 20,21A17,17 0 0,1 3,4A1,1 0 0,1 4,3H7.5A1,1 0 0,1 8.5,4C8.5,5.25 8.7,6.45 9.07,7.57C9.18,7.92 9.1,8.31 8.82,8.59L6.62,10.79Z" />
                </svg>
                <span class="badge-text">{{ formattedPhone }}</span>
              </div>
              <div
                v-else
                key="copied"
                class="badge-content"
              >
                <span class="badge-text">Скопировано!</span>
              </div>
            </transition>
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  props: {
    organization: { type: String, default: null },
    company: { type: String, default: null },
    lastName: { type: String, default: null },
    firstName: { type: String, default: null },
    middleName: { type: String, default: null },
    position: { type: String, default: null },
    email: { type: String, default: null },
    phone: { type: String, default: null },
    userType: { type: String, default: null },
    typeId: { type: Number, default: null }
  },
  data() {
    return {
      copiedEmail: false,
      copiedPhone: false,
      timeoutEmail: null,
      timeoutPhone: null,
      emailBadgeWidth: null,
      phoneBadgeWidth: null
    };
  },
  computed: {
    userTypeDisplay() {
      const typeMap = {
        'user': 'Пользователь',
        'security': 'Охранник',
        'manager': 'Руководитель',
        'buropropuskov': 'Бюро пропусков',
        'renter': 'Арендатор',
        'contractor': 'Подрядчик'
      };
      return typeMap[this.userType] || this.userType;
    },
    fullName() {
      const parts = [this.lastName, this.firstName, this.middleName].filter(Boolean);
      return parts.join(' ') || null;
    },
    displayName() {
      return this.fullName || this.organization || '';
    },
    userInitials() {
      if (this.firstName && this.lastName) {
        return (this.lastName.charAt(0) + this.firstName.charAt(0).toUpperCase());
      }
      return 'П';
    },
    hasContactDetails() {
      // Показываем контактные данные, если есть email или телефон
      return this.email || this.phone;
    },
    formattedPhone() {
      if (!this.phone) return '';
      // Форматирование телефона в формат +7 (999) 999 99-99
      const cleaned = ('' + this.phone).replace(/\D/g, '');
      const match = cleaned.match(/^(\d{1})(\d{3})(\d{3})(\d{2})(\d{2})$/);
      if (match) {
        return `+${match[1]} (${match[2]}) ${match[3]} ${match[4]}-${match[5]}`;
      }
      return this.phone;
    }
  },
  watch: {
    email: {
      handler() {
        this.$nextTick(() => {
          this.updateBadgeWidths();
        });
      },
      immediate: false
    },
    phone: {
      handler() {
        this.$nextTick(() => {
          this.updateBadgeWidths();
        });
      },
      immediate: false
    }
  },
  mounted() {
    this.$nextTick(() => {
      this.updateBadgeWidths();
    });
  },
  beforeUnmount() {
    if (this.timeoutEmail) clearTimeout(this.timeoutEmail);
    if (this.timeoutPhone) clearTimeout(this.timeoutPhone);
  },
  methods: {
    updateBadgeWidths() {
      if (this.$refs.emailBadge && this.email) {
        const originalBadge = this.$refs.emailBadge;
        const clone = originalBadge.cloneNode(true);
        clone.style.position = 'absolute';
        clone.style.visibility = 'hidden';
        clone.style.top = '-9999px';
        document.body.appendChild(clone);

        const originalWidth = clone.offsetWidth;

        const contentDiv = clone.querySelector('.badge-content');
        if (contentDiv) {
          const icon = contentDiv.querySelector('.icon');
          if (icon) icon.remove();
          const textSpan = contentDiv.querySelector('.badge-text');
          if (textSpan) textSpan.textContent = 'Почта скопирована!';
        }
        const copiedWidth = clone.offsetWidth;

        this.emailBadgeWidth = Math.max(originalWidth, copiedWidth);
        document.body.removeChild(clone);
      }

      if (this.$refs.phoneBadge && this.phone) {
        const originalBadge = this.$refs.phoneBadge;
        const clone = originalBadge.cloneNode(true);
        clone.style.position = 'absolute';
        clone.style.visibility = 'hidden';
        clone.style.top = '-9999px';
        document.body.appendChild(clone);

        const originalWidth = clone.offsetWidth;

        const contentDiv = clone.querySelector('.badge-content');
        if (contentDiv) {
          const icon = contentDiv.querySelector('.icon');
          if (icon) icon.remove();
          const textSpan = contentDiv.querySelector('.badge-text');
          if (textSpan) textSpan.textContent = 'Скопировано!';
        }
        const copiedWidth = clone.offsetWidth;

        this.phoneBadgeWidth = Math.max(originalWidth, copiedWidth);
        document.body.removeChild(clone);
      }
    },
    copyEmail() {
      if (!this.email) return;
      navigator.clipboard.writeText(this.email).then(() => {
        this.copiedEmail = true;
        if (this.timeoutEmail) clearTimeout(this.timeoutEmail);
        this.timeoutEmail = setTimeout(() => {
          this.copiedEmail = false;
        }, 2000);
      }).catch(err => {
        console.error('Ошибка копирования email:', err);
      });
    },
    copyPhone() {
      if (!this.phone) return;
      const phoneDigits = this.phone.replace(/\D/g, '');
      navigator.clipboard.writeText(phoneDigits).then(() => {
        this.copiedPhone = true;
        if (this.timeoutPhone) clearTimeout(this.timeoutPhone);
        this.timeoutPhone = setTimeout(() => {
          this.copiedPhone = false;
        }, 2000);
      }).catch(err => {
        console.error('Ошибка копирования телефона:', err);
      });
    }
  }
};
</script>

<style scoped>
.account-header {
  flex: 1;
  display: flex;
  align-items: flex-start;
  gap: 20px;
  padding: 20px 45px;
  border-radius: 30px;
  border: 1px solid var(--border);
  height: 200px;
  position: relative;
  overflow: hidden;
  opacity: 0;
  transform: translateY(20px);
  animation: fadeInUp 0.6s cubic-bezier(0.16, 1, 0.3, 1) forwards;
  width: fit-content;
}

.account-header::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  width: 20px;
  height: 100%;
  background: var(--accent);
}

.user-avatar {
  flex-shrink: 0;
  position: relative;
  opacity: 0;
  transform: scale(0.8);
  animation: scaleIn 0.5s cubic-bezier(0.175, 0.885, 0.32, 1.275) 0.2s forwards;
}

.avatar-placeholder {
  width: 80px;
  height: 80px;
  border-radius: 50%;
  background: linear-gradient(135deg, color-mix(in srgb, var(--accent) 70%, var(--surface)) 0%, var(--accent) 100%);
  color: var(--accent-contrast);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.8em;
  font-weight: bold;
  box-shadow: 0 1px 6px rgba(74, 111, 165, 0.3);
  border: 3px solid var(--surface);
}

.user-info {
  flex-grow: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.main-info {
  padding-bottom: 12px;
  /* Разделитель - цветом рамки, а не тени: --shadow-drop в тёмных темах почти
     чёрный, и линия под именем читалась чёрной полосой. */
  border-bottom: 1px solid var(--border);
}

.name-and-type {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 6px;
  flex-wrap: wrap;
}

.user-name {
  font-size: 1.6em;
  margin: 0;
  color: var(--text);
  font-weight: 700;
  line-height: 1.3;
  opacity: 0;
  transform: translateX(-10px);
  animation: fadeInRight 0.4s ease-out 0.3s forwards;
  overflow-wrap: anywhere;
  word-break: break-word;
  max-width: 100%;
}

.org-company-block {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.user-organization,
.user-company {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  line-height: 1.4;
  opacity: 0;
  transform: translateX(-10px);
}

.user-organization {
  font-size: 1.2em;
  color: var(--text);
  font-weight: 600;
  animation: fadeInRight 0.4s ease-out 0.35s forwards;
}

.user-company {
  font-size: 1em;
  color: var(--text-muted);
  animation: fadeInRight 0.4s ease-out 0.4s forwards;
}

.icon {
  width: 16px;
  height: 16px;
  fill: currentColor;
  opacity: 0.8;
}

/* Стили для бейджей с информацией */
.user-details-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding-top: 12px;
  /* П.43: контактные бейджи прижаты к нижней части блока */
  margin-top: auto;
}

.user-detail {
  opacity: 0;
  transform: translateY(5px);
  animation: fadeInUp 0.3s ease-out forwards;
}

.user-detail:nth-child(1) { animation-delay: 0.4s; }
.user-detail:nth-child(2) { animation-delay: 0.45s; }
.user-detail:nth-child(3) { animation-delay: 0.5s; }

.detail-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 5px 12px;
  border-radius: 999px;
  font-size: 0.85em;
  font-weight: 500;
  white-space: nowrap;
  box-sizing: border-box;
  min-width: 0;
  cursor: pointer;
  border: 1px solid transparent;
  background: var(--accent-tint);
  color: var(--text-muted);
}

.badge-content {
  display: flex;
  align-items: center;
  gap: 6px;
  justify-content: center;
}

.badge-text {
  display: inline-block;
  text-align: center;
}

.clickable {
  cursor: pointer;
  transition: transform 0.2s;
}

.clickable:hover {
  transform: translateY(-2px);
}

.position-badge {
  background: var(--accent-tint);
  /* accent-hover на бледной подложке давал ~2.4 - должность не читалась. */
  color: var(--accent-text);
  border-color: color-mix(in srgb, var(--accent) 40%, var(--surface));
  cursor: default;
  gap: 6px;
}

.email-badge {
  background: var(--success-bg);
  color: var(--success-text);
  border-color: color-mix(in srgb, var(--success) 30%, var(--surface));
}

.phone-badge {
  background: var(--warning-bg);
  color: var(--warning-text);
  border-color: color-mix(in srgb, var(--warning) 30%, var(--surface));
}

.detail-badge .icon {
  flex-shrink: 0;
  width: 14px;
  height: 14px;
  opacity: 0.7;
}

/* Бейдж с типом пользователя */
.user-type-badge {
  display: inline-block;
  padding: 5px 12px;
  background: var(--color-primary, var(--accent));
  color: var(--accent-contrast);
  border-radius: 999px;
  font-size: 0.85em;
  font-weight: 500;
  white-space: nowrap;
  opacity: 0;
  animation: fadeIn 0.4s ease-out 0.5s forwards;
}

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

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes fadeInRight {
  from {
    opacity: 0;
    transform: translateX(-10px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

@keyframes scaleIn {
  from {
    opacity: 0;
    transform: scale(0.8);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

@keyframes fadeIn {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

/* Адаптивность */
@media (max-width: 768px) {
  .account-header {
    flex-direction: column;
    align-items: center;
    text-align: center;
    padding: 15px;
    width: 100%;
    height: auto;
  }
  
  .user-avatar {
    margin-bottom: 15px;
  }
  
  .user-details-row {
    justify-content: center;
  }
  
  .user-name {
    font-size: 1.4em;
  }
  
  .user-organization {
    font-size: 1.1em;
    justify-content: center;
  }
  
  .user-company {
    justify-content: center;
  }

  .name-and-type {
    justify-content: center;
  }
}

@media (max-width: 480px) {
  .account-header {
    padding: 15px 10px;
  }
  
  .avatar-placeholder {
    width: 70px;
    height: 70px;
    font-size: 1.6em;
  }
  
  .user-name {
    font-size: 1.3em;
  }
  
  .detail-badge {
    font-size: 0.8em;
    padding: 5px 10px;
  }

  .name-and-type {
    gap: 15px;
    flex-direction: column;
    align-items: center;
  }
}
</style>