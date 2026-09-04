<template>
  <div class="account-header">    
    <div class="user-info">
      <!-- Основная информация -->
      <div class="main-info">
        <!-- Согласие на обработку данных: единственное место, где работник может
             его отозвать. Показываем, только когда согласие реально дано - иначе
             отзывать нечего, а окно согласия он и так видит. -->
        <div
          v-if="consentGrantedAt"
          class="consent-row"
        >
          <button
            type="button"
            class="detail-badge consent-badge clickable"
            :disabled="consentRevoking"
            :title="consentTitle"
            data-testid="cabinet-consent-badge"
            @click="revokeOwnConsent"
          >
            <span class="badge-content">
              <NavIcon
                class="icon"
                name="data-processing"
              />
              <span class="badge-text">{{ consentBadgeLabel }}</span>
            </span>
          </button>
        </div>
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
            <!-- Здание компании: жилой корпус с рядами окон и пристройкой. -->
            <svg
              class="icon"
              viewBox="0 0 24 24"
            >
              <path d="M4 20.5V6.2a1 1 0 0 1 1-1h6.5a1 1 0 0 1 1 1v14.3" />
              <path d="M12.5 10.5H19a1 1 0 0 1 1 1v9" />
              <line
                x1="6.6"
                y1="9.5"
                x2="9.9"
                y2="9.5"
              />
              <line
                x1="6.6"
                y1="14.5"
                x2="9.9"
                y2="14.5"
              />
              <line
                x1="15"
                y1="15.5"
                x2="17.6"
                y2="15.5"
              />
              <line
                x1="2.8"
                y1="20.5"
                x2="21.2"
                y2="20.5"
              />
            </svg>
            {{ company }}
          </div>
        </div>
      </div>
      
      <!-- Контакты и действия. Ряд показывается всегда: в нём живут постоянные
           элементы - кнопка смены пароля и предупреждение о неуказанной почте, -
           поэтому прежнее условие «есть хоть один контакт» скрывало бы их у
           работника с пустой карточкой. -->
      <div class="user-details-row">
        <div
          v-if="position"
          class="user-detail"
        >
          <span class="detail-badge position-badge">
            <!-- Должность: рабочий портфель. Прежний глиф - дом с меткой -
                 к названию должности отношения не имел. -->
            <svg
              class="icon"
              viewBox="0 0 24 24"
            >
              <rect
                x="3.5"
                y="7.5"
                width="17"
                height="12.5"
                rx="2.2"
              />
              <path d="M9 7.5V6.2a1.6 1.6 0 0 1 1.6-1.6h2.8A1.6 1.6 0 0 1 15 6.2v1.3" />
              <line
                x1="3.5"
                y1="12.8"
                x2="20.5"
                y2="12.8"
              />
            </svg>
            {{ position }}
          </span>
        </div>
        <div
          v-if="email"
          class="user-detail"
          :title="emailOwnerHint"
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
                <NavIcon
                  class="icon"
                  name="center"
                />
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
                <!-- Телефон: трубка по диагонали, скруглённая под наклоном. -->
                <svg
                  class="icon"
                  viewBox="0 0 24 24"
                >
                  <path d="M8.4 3.6 5.9 6.1a1.8 1.8 0 0 0-.4 1.9 19 19 0 0 0 10.5 10.5 1.8 1.8 0 0 0 1.9-.4l2.5-2.5-3.9-2.9-1.8 1.4a14.5 14.5 0 0 1-5.2-5.2l1.4-1.8-3.5-3.5Z" />
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
        <div class="user-detail">
          <button
            v-if="canChangePassword"
            type="button"
            class="detail-badge password-badge clickable"
            title="Сменить пароль"
            data-testid="cabinet-change-password"
            @click="passwordModalOpen = true"
          >
            <span class="badge-content">
              <!-- Пароль: замок с дужкой и скважиной. -->
              <svg
                class="icon"
                viewBox="0 0 24 24"
              >
                <rect
                  x="4.5"
                  y="10"
                  width="15"
                  height="10.5"
                  rx="2.5"
                />
                <path d="M8 10V6.9a4 4 0 0 1 8 0V10" />
                <circle
                  cx="12"
                  cy="15.2"
                  r="1.4"
                />
              </svg>
              <span class="badge-text">Пароль</span>
            </span>
          </button>
        </div>
      </div>
    </div>

    <ChangePasswordModal
      :show="passwordModalOpen"
      @close="passwordModalOpen = false"
    />
  </div>
</template>

<script>
import { listMyConsents, revokeMyConsent } from '@/api/pdConsent';
import { usePDConsentStore } from '@/stores/pdConsent';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';
import { useContactsStore } from '@/stores/contacts';
import ChangePasswordModal from './ChangePasswordModal.vue';
import NavIcon from './icons/NavIcon.vue';
import { formatMomentDate } from '@/utils/datetime';

/** Вид согласия, которое спрашивают при первом входе. */
const PD_PROCESSING = 'pd_processing';

export default {
  components: { ChangePasswordModal, NavIcon },
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
      passwordModalOpen: false,
      copiedEmail: false,
      copiedPhone: false,
      timeoutEmail: null,
      timeoutPhone: null,
      emailBadgeWidth: null,
      phoneBadgeWidth: null,
      // Когда работник дал действующее согласие; null - согласия нет, отзывать нечего.
      consentGrantedAt: null,
      consentRevoking: false
    };
  },
  computed: {
    /**
     * Пароль работника поста ведёт бюро пропусков (#2280): своей формы смены у
     * него нет, и сервер такой запрос отклоняет.
     */
    canChangePassword() {
      return this.userType !== 'security';
    },
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
    consentBadgeLabel() {
      return this.consentRevoking ? 'Отзываем...' : 'Согласие на обработку данных';
    },

    // Адрес почты работник не меняет сам: на него система шлёт новые пароли при
    // плановой смене, поэтому канал доставки ведёт бюро. Подсказка объясняет,
    // куда обращаться, и подставляет контакты из системных настроек.
    bureauContactsSuffix() {
      const contacts = useContactsStore();
      const parts = [contacts.phone, contacts.email].filter(Boolean);
      return parts.length ? ` (${parts.join(', ')})` : '';
    },

    emailOwnerHint() {
      return `Адрес меняет бюро пропусков${this.bureauContactsSuffix}`;
    },

    consentTitle() {
      const at = this.consentGrantedAt ? new Date(this.consentGrantedAt) : null;
      const when = at && !Number.isNaN(at.getTime()) ? ` ${formatMomentDate(at)}` : '';
      return `Согласие дано${when}. Нажмите, чтобы отозвать`;
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
    // Контакты бюро нужны подсказке про почту; стор кэширует запрос.
    useContactsStore().fetch();
    this.$nextTick(() => {
      this.updateBadgeWidths();
    });
    this.loadOwnConsent();
  },
  beforeUnmount() {
    if (this.timeoutEmail) clearTimeout(this.timeoutEmail);
    if (this.timeoutPhone) clearTimeout(this.timeoutPhone);
  },
  methods: {
    /**
     * Читает собственное согласие работника. Молча пропускаем ошибку: бейдж -
     * дополнение к кабинету, и падать из-за него страница не должна.
     */
    async loadOwnConsent() {
      try {
        const consents = await listMyConsents();
        const active = (Array.isArray(consents) ? consents : [])
          .filter((c) => c.consent_type === PD_PROCESSING && c.granted && !c.revoked_at)
          .sort((a, b) => String(b.granted_at).localeCompare(String(a.granted_at)))[0];
        this.consentGrantedAt = active?.granted_at || null;
      } catch {
        this.consentGrantedAt = null;
      }
    },

    /**
     * Отзывает собственное согласие. Последствие серьёзное - доступ закрывается до
     * нового подтверждения, поэтому спрашиваем и говорим об этом прямо.
     */
    async revokeOwnConsent() {
      if (this.consentRevoking) return;
      const ok = await useUiStore().confirm({
        title: 'Отзыв согласия',
        message: 'Отозвать согласие на обработку персональных данных?'
          + ' Система сразу закроет доступ и покажет окно согласия -'
          + ' работать получится только после нового подтверждения.',
        confirmText: 'Отозвать',
        danger: true,
      });
      if (!ok) return;
      this.consentRevoking = true;
      try {
        await revokeMyConsent(PD_PROCESSING);
        this.consentGrantedAt = null;
        useDeletionsStore().notify({ prefix: 'Согласие на обработку данных отозвано' });
        // Окно согласия поднимает стор: без принудительного перечитывания оно
        // появилось бы только по истечении кэша или после перезагрузки страницы.
        await usePDConsentStore().refresh(true);
      } catch (error) {
        useDeletionsStore().notify({
          prefix: error?.message || 'Не удалось отозвать согласие',
          type: 'error',
        });
      } finally {
        this.consentRevoking = false;
      }
    },

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
  padding: 10px 45px 10px;
  border-radius: 30px;
  /* Карточка на фоне страницы: без своего фона она темнее соседней карточки уведомлений. */
  background: var(--surface);
  border: 1px solid var(--border);
  height: var(--cabinet-card-height, 200px);
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
  /* Глифы кабинета рисуются обводкой, как набор навигации (navIcons.js):
     единый вес линии и цвет от текста вместо заливки силуэтом. */
  fill: none;
  stroke: currentColor;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
  opacity: 0.8;
}

/* Стили для бейджей с информацией */
.user-details-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  /* Ряд отодвинут и от разделителя, и от нижней кромки: 8px снизу плюс поле
     карточки. Место под них взято у верхнего поля - высота остаётся 200px. */
  padding-top: 8px;
  padding-bottom: 8px;
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

/* Ряд с согласием стоит над именем: бейдж единственный в шапке, по которому
   работник что-то делает, и в контактной строке внизу его не замечали. */
.consent-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  margin-bottom: 10px;
  opacity: 0;
  transform: translateY(5px);
  animation: fadeInUp 0.3s ease-out 0.25s forwards;
}

/* Бейдж согласия - кнопка, а не span: он единственный в шапке что-то делает.
   Обнуляем браузерные стили кнопки, чтобы он не выбивался из ряда бейджей. */
.consent-badge {
  font-family: inherit;
  line-height: inherit;
}

/* Ниже контактных бейджей: над именем он служебный, а высоту карточки делит с
   ними в пределах 200px. Селектор двойной намеренно: одиночный .consent-badge
   стоит в файле выше .detail-badge, и её shorthand padding перебивал бы его. */
.detail-badge.consent-badge {
  padding-top: 2px;
  padding-bottom: 2px;
}

.consent-badge:disabled {
  cursor: default;
  opacity: 0.7;
}

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

/* Единственный бейдж-действие в ряду контактов, поэтому нейтральная подложка:
   зелёный и жёлтый рядом уже заняты почтой и телефоном, третий цвет читался бы
   как ещё один вид контакта. font-family и line-height - как у consent-badge:
   у button они свои и не наследуются. */
/* Единственный бейдж-действие в ряду контактов, поэтому нейтральная подложка:
   зелёный и жёлтый рядом заняты почтой и телефоном, третий цвет читался бы как
   ещё один вид контакта. font-family и line-height - как у consent-badge:
   у button они свои и не наследуются. Подпись короткая намеренно: с полной
   «Сменить пароль» бейдж выдавливал ряд контактов на вторую строку. */
.password-badge {
  font-family: inherit;
  line-height: inherit;
  background: var(--surface-2);
  color: var(--text);
  border-color: var(--border);
  white-space: nowrap;
  flex-shrink: 0;
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
  
  .user-details-row,
  .consent-row {
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