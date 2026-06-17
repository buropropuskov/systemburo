<template>
  <div
    ref="root"
    class="account-dashboard"
    data-testid="cabinet-page"
  >
    <!-- Первая строка: заголовок и заявки -->
    <div class="first-row">
      <SkeletonTransition :loading="loading">
        <template #skeleton>
          <div style="display: flex; gap: 20px; width: 100%;">
            <SkeletonBlock
              height="200px"
              style="flex: 1;"
            />
            <SkeletonBlock
              height="200px"
              style="flex: 1;"
            />
          </div>
        </template>

        <!-- Заголовок с информацией о пользователе -->
        <UserProfileHeader
          :organization="organization"
          :company="company"
          :last-name="lastName"
          :first-name="firstName"
          :middle-name="middleName"
          :position="position"
          :email="email"
          :phone="phone"
          :user-type="user_type"
          :type-id="type_id"
          class="dashboard-card-animated"
        />

        <UserNotificationsInline class="dashboard-card-animated notifications" />
      </SkeletonTransition>
    </div>
    
    <div class="dashboard-row">
      <!-- Блок заявок -->
      <div class="applications-wrapper">
        <UserApplications
          :user-organization-id="userOrganizationId"
          :user-company-id="userCompanyId"
          :user-id="userId"
          :user-organization="organization"
          :user-company="company"
          class="dashboard-card-animated"
        />
      </div>

      <!-- Настройки звука -->
      <div class="sound-settings dashboard-card-animated">
        <div class="sound-settings__header">
          <h3 class="sound-settings__title">
            Звук
          </h3>
          <label
            class="sound-toggle"
            :aria-label="soundStore.enabled ? 'Выключить звук' : 'Включить звук'"
          >
            <input
              type="checkbox"
              class="sound-toggle__input"
              :checked="soundStore.enabled"
              @change="soundStore.setEnabled($event.target.checked)"
            >
            <span class="sound-toggle__track" />
          </label>
        </div>

        <template v-if="soundStore.enabled">
          <div class="sound-settings__field">
            <label class="sound-settings__label">Пресет</label>
            <select
              class="lk-select"
              :value="soundStore.selectedPreset"
              @change="soundStore.setPreset($event.target.value)"
            >
              <option
                v-for="p in soundPresets"
                :key="p.value"
                :value="p.value"
              >
                {{ p.label }}
              </option>
            </select>
          </div>

          <div class="sound-settings__field">
            <label class="sound-settings__label">Громкость {{ Math.round(soundStore.volume * 100) }}%</label>
            <input
              type="range"
              class="sound-volume"
              min="0"
              max="1"
              step="0.05"
              :value="soundStore.volume"
              @input="soundStore.setVolume($event.target.value)"
            >
          </div>

          <button
            type="button"
            class="lk-button lk-button--ghost sound-settings__preview"
            @click="previewSound"
          >
            Прослушать
          </button>
        </template>
      </div>
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { useSoundStore } from '@/stores/sound'
import { playPreset, SOUND_PRESETS } from '@/utils/notificationSound'
import UserApplications from './UserApplications.vue';
import UserProfileHeader from './UserProfileHeader.vue';
import UserNotificationsInline from './UserNotificationsInline.vue';
import { SkeletonTransition, SkeletonBlock } from '@/components/ui';

export default {
  components: {
    UserApplications,
    UserProfileHeader,
    UserNotificationsInline,
    SkeletonTransition,
    SkeletonBlock,
  },
  setup() {
    const soundStore = useSoundStore()
    return { soundStore }
  },
  data() {
    return {
      loading: true,
      soundPresets: SOUND_PRESETS,
      organization: "",
      company: "",
      username: "",
      type_id: 1,
      user_type: "user",
      userId: null,
      userOrganizationId: null,
      userCompanyId: null,

      // Поля пользователя
      lastName: "",
      firstName: "",
      middleName: "",
      position: "",
      email: "",
      phone: ""
    };
  },
  mounted() {
    this.headerObserver = null;
    this.lastDashboardHeight = -1;
    this.fetchUserData();
    this.$nextTick(this.applyDashboardHeight);
    window.addEventListener('resize', this.applyDashboardHeight);
    // Шапка in-flow и может менять высоту (анонс, перенос строки) - пересчитываем
    // под неё. Наблюдаем саму шапку, а не body, чтобы Teleport-модалки не будили
    // лишний reflow. Тот же приём, что в AdminPageShell.vue.
    const header = document.querySelector('.theheader');
    if (header && typeof ResizeObserver !== 'undefined') {
      this.headerObserver = new ResizeObserver(this.applyDashboardHeight);
      this.headerObserver.observe(header);
    }
  },
  beforeUnmount() {
    window.removeEventListener('resize', this.applyDashboardHeight);
    if (this.headerObserver) {
      this.headerObserver.disconnect();
      this.headerObserver = null;
    }
  },
  methods: {
    /**
     * Тянет дашборд кабинета на доступную высоту вьюпорта (под шапкой), чтобы
     * список заявок занимал весь экран без скролла всей страницы. Высоту меряем
     * фактически (innerHeight - top), а не 100vh минус хардкод шапки. На планшете
     * и мобильном (<=1200px) возвращаем естественный поток - там работает
     * адаптивная фикс-высота карточки и строки складываются в колонку.
     */
    applyDashboardHeight() {
      const el = this.$refs.root;
      if (!el) return;
      if (window.innerWidth <= 1200) {
        el.style.height = '';
        this.lastDashboardHeight = -1;
        return;
      }
      const top = el.getBoundingClientRect().top;
      // 15px - отступ снизу чтобы блок заявок не прилипал к краю экрана.
      // Padding 15px сверху уже входит в el, поэтому вычитаем только снизу.
      const BOTTOM_GAP = 15;
      const height = Math.max(0, Math.round(window.innerHeight - top - BOTTOM_GAP));
      // Защита от ResizeObserver-петли: пишем стиль только при реальном изменении.
      if (height === this.lastDashboardHeight) return;
      this.lastDashboardHeight = height;
      el.style.height = `${height}px`;
    },
    async fetchUserData() {
      try {
        const authStore = useAuthStore();
        if (!authStore.token) {
          alert("Пользователь не авторизован.");
          return;
        }

        const response = await apiRequest("/users/me", {
          method: "GET",
        });

        if (response.ok) {
          const userData = await response.json();
          this.updateUserData(userData);
        } else {
          alert("Ошибка при загрузке данных пользователя.");
        }
      } catch (error) {
        console.error("Ошибка сети при загрузке данных пользователя:", error);
      } finally {
        this.loading = false;
      }
    },
    previewSound() {
      playPreset(this.soundStore.selectedPreset, this.soundStore.volume)
    },

    updateUserData(userData) {
      this.organization = userData.organization || "";
      this.company = userData.company || "";
      this.username = userData.username || "";
      this.type_id = userData.type_id || 1;
      this.user_type = userData.user_type || "user";
      this.userId = userData.id || null;
      this.userOrganizationId = userData.organization_id || null;
      this.userCompanyId = userData.company_id || null;
      
      // Дополнительные поля
      this.lastName = userData.last_name || "";
      this.firstName = userData.first_name || "";
      this.middleName = userData.middle_name || "";
      this.position = userData.position || "";
      this.email = userData.email || "";
      this.phone = userData.phone || "";
    }
  }
};
</script>

<style scoped>
.account-dashboard {
  width: 100%;
  padding: 15px;
  position: relative;
  /* Высоту на desktop задаёт applyDashboardHeight под доступный вьюпорт;
     flex-колонка тянет блок заявок на остаток высоты. overflow:hidden
     обязателен - без него flex-дети могут расти за пределы установленной
     высоты. На <=1200px высота сбрасывается и блок снова в естественном
     потоке (страница скроллится). */
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* Стили для строк */
.first-row {
  display: flex;
  gap: 15px;
  margin-bottom: 15px;
  flex-shrink: 0;
}

.first-row :deep(.notifications) {
  max-width: 38%;
}

/* SkeletonTransition оборачивает детей в div — раскладываем его в flex row
   чтобы UserProfileHeader и UserNotificationsInline были side-by-side */
.first-row > div {
  display: flex;
  flex-direction: row;
  gap: 15px;
  flex: 1;
  min-width: 0;
  width: 100%;
}

.dashboard-row {
  padding: 15px 0;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.applications-wrapper {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

/* Карточки личного кабинета (профиль/уведомления/заявки) */
.dashboard-card-animated {
  opacity: 0;
  transform: translateY(20px);
  animation: fadeInUp 0.5s cubic-bezier(0.16, 1, 0.3, 1) forwards;
}

/* Анимации */
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

/* Блок настроек звука */
.sound-settings {
  margin-top: 15px;
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: 30px;
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  flex-shrink: 0;
  animation-delay: 0.15s;
}

.sound-settings__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.sound-settings__title {
  font-size: 14px;
  font-weight: 600;
  margin: 0;
  color: #1a1a1a;
}

/* Toggle-переключатель в стиле карточек ЛК */
.sound-toggle {
  position: relative;
  display: inline-flex;
  align-items: center;
  cursor: pointer;
  flex-shrink: 0;
}

.sound-toggle__input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
}

.sound-toggle__track {
  display: inline-block;
  width: 36px;
  height: 20px;
  background: var(--color-border);
  border-radius: 999px;
  transition: background 0.2s ease;
  position: relative;
}

.sound-toggle__track::after {
  content: '';
  position: absolute;
  top: 3px;
  left: 3px;
  width: 14px;
  height: 14px;
  background: #fff;
  border-radius: 50%;
  transition: transform 0.2s ease;
  box-shadow: 0 1px 3px rgba(0,0,0,0.15);
}

.sound-toggle__input:checked + .sound-toggle__track {
  background: var(--color-primary);
}

.sound-toggle__input:checked + .sound-toggle__track::after {
  transform: translateX(16px);
}

.sound-settings__field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.sound-settings__label {
  font-size: 12px;
  color: var(--color-text-muted);
  font-weight: 500;
}

.sound-volume {
  width: 100%;
  accent-color: var(--color-primary);
  cursor: pointer;
  height: 4px;
  border-radius: 4px;
}

.sound-settings__preview {
  align-self: flex-start;
  font-size: 12px;
  padding: 6px 16px;
}

/* Адаптивность */
@media (max-width: 1200px) {
  .first-row {
    flex-direction: column;
  }

  .first-row > div {
    flex-direction: column;
  }

  .first-row :deep(.notifications) {
    max-width: 100%;
  }
}
</style>