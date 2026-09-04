<template>
  <header
    ref="header"
    class="header"
  >
    <button
      class="header__burger"
      aria-label="Открыть меню"
      type="button"
      @click="toggleMobileNav"
    >
      <span />
      <span />
      <span />
    </button>

    <div
      class="header__title"
      :class="{ 'header__title--shifted': uiStore.sidebarHidden }"
    >
      <template v-if="loading">
        <SkeletonLine
          width="200px"
          height="20px"
        />
        <SkeletonLine
          width="150px"
          height="14px"
        />
      </template>
      <template v-else>
        <h3>{{ greeting }}</h3>
        <p class="header__subtitle">
          Мы рады, что вы здесь!
        </p>
      </template>
    </div>

    <div class="header__info">
      <!-- Дата и время - только на десктопе (мобильную шапку не грузим). -->
      <!-- Подсказка снизу: сверху её обрезала бы граница окна - шапка прижата к верху.
           Она же объясняет расхождение с часами компьютера, если те сбиты (#2298). -->
      <span
        class="header__time hint-anchor hint-anchor--below"
        data-hint="Московское время"
        data-testid="header-time"
      >{{ currentDateTime }}</span>

      <!-- Объявление: текстовый pill на всех ширинах, включая мобилку (правка волны 3) -->
      <button
        v-if="activeAnnouncement"
        class="broadcast"
        :class="{ 'broadcast--important': activeAnnouncement.is_important }"
        data-testid="ob-header-broadcast"
        :aria-label="activeAnnouncement.is_important ? 'Важное объявление' : 'Объявление'"
        @click="showAnnouncement = true"
      >
        <span class="broadcast__label">{{ activeAnnouncement.is_important ? 'Важное объявление' : 'Объявление' }}</span>
      </button>

      <!-- Уведомления (колокольчик): всегда в самой шапке (A.2) -->
      <div
        class="user__notifications"
        :class="{ 'user__notifications--active': showNotifications }"
        data-testid="ob-header-notifications"
        role="button"
        tabindex="0"
        aria-label="Уведомления"
        :aria-expanded="showNotifications"
        @click.stop="showNotifications = !showNotifications"
        @keydown.enter.prevent="showNotifications = !showNotifications"
        @keydown.space.prevent="showNotifications = !showNotifications"
      >
        <AppIcon
          name="notifications"
          class="notifications__icon"
        />
        <span
          v-if="unreadCount > 0"
          class="notifications__badge"
        >{{ unreadCount }}</span>
        <UserNotifications
          :show="showNotifications"
          @update:unread-count="unreadCount = $event"
          @close="showNotifications = false"
        />
      </div>

      <!-- «Сообщить о проблеме»: на десктопе в шапке, на мобилке (<768) переезжает в
           бургер-drawer NavMenu (W3.3), поэтому из DOM шапки убрана - иначе тур нашёл бы
           скрытый дубль. Меню "⋯" и часы убраны совсем (правка волны 3). -->
      <button
        v-if="can('header.report_problem') && !isMobileHeader"
        class="feedback-btn"
        data-testid="header-button-feedback"
        @click="openFeedbackModal"
      >
        Сообщить о проблеме
      </button>
      <div
        v-if="can('header.create_application')"
        class="appl-btn__container"
      >
        <button
          class="appl-btn"
          data-testid="header-button-submit-app"
          @click="navigateToSubmit"
        >
          <span class="appl-btn__label">Подать заявку</span>
          <span class="appl-btn__label-short">Заявка</span>
        </button>
      </div>
      <!-- Поиск по системе: крайний правый элемент шапки. Нажатие открывает панель
           результатов справа и ставит в неё курсор -- ввод идёт уже там, рядом с
           найденным, а не в другом конце экрана. -->
      <button
        class="search-btn"
        type="button"
        title="Поиск по системе"
        aria-label="Поиск по системе"
        data-testid="header-button-search"
        @click="openGlobalSearch"
      >
        <NavIcon
          name="search"
          :size="18"
        />
      </button>
    </div>

    <!-- Используем отдельный компонент модального окна -->
    <FeedbackModal
      v-model:show="showFeedbackModal"
      @submitted="handleFeedbackSubmitted"
    />
    <AnnouncementModal
      :show="showAnnouncement"
      :announcement="activeAnnouncement"
      @close="showAnnouncement = false"
    />
  </header>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'
import { usePermissionsStore } from '@/stores/permissions'
import { useOnboardingStore } from '@/stores/onboarding'
import FeedbackModal from '@/components/FeedbackModal.vue';
import AnnouncementModal from '@/components/AnnouncementModal.vue';
import UserNotifications from '@/components/UserNotifications.vue';
import { SkeletonLine } from '@/components/ui';
import NavIcon from '@/components/icons/NavIcon.vue';
import AppIcon from '@/components/icons/AppIcon.vue';
import { serverNow, moscowHour, formatMoscowDateTime } from '@/utils/serverTime';

export default {
  name: 'TheHeader',
  components: {
    AppIcon,
    FeedbackModal,
    AnnouncementModal,
    UserNotifications,
    SkeletonLine,
    NavIcon,
  },
  emits: ['refresh-feedback'],
  setup() {
    // uiStore нужен для сдвига приветствия, когда рельс скрыт (плавающая кнопка
    // возврата перекрывала бы заголовок). permissionsStore - гейт кнопок шапки:
    // «Сообщить о проблеме» (header.report_problem) и «Подать заявку»
    // (header.create_application, отдельно от nav «Новая заявка» = page.new_application).
    const uiStore = useUiStore();
    const permissionsStore = usePermissionsStore();
    return { uiStore, permissionsStore };
  },
  data() {
    return {
      loading: true,
      userFirstName: '',
      userLastName: '',
      timer: null,
      isHeaderHidden: false,
      observer: null,
      showFeedbackModal: false,
      showAnnouncement: false,
      activeAnnouncement: null,
      showNotifications: false,
      // Список открыл тур, а не человек: закрываем по гашению сигнала только то,
      // что открыли сами (тот же приём, что у панели поиска в App.vue).
      notificationsOpenedByTour: false,
      unreadCount: 0,
      currentHour: moscowHour(),
      // Дата и время (ДД.ММ.ГГГГ ЧЧ:ММ:СС) в шапке - только на десктопе, как было
      // до правки волны 3 (на мобилке шапка тесная).
      currentDateTime: '',
      // <768: кнопка «Сообщить о проблеме» живёт в бургер-drawer, не в шапке (W3.3).
      isMobileHeader: false,
    };
  },
  computed: {
    displayName() {
      return this.userFirstName || this.userLastName || '';
    },
    greetingPrefix() {
      const h = this.currentHour;
      if (h >= 6 && h < 12) return 'Доброе утро';
      if (h >= 12 && h < 18) return 'Добрый день';
      if (h >= 18 && h < 23) return 'Добрый вечер';
      return 'Доброй ночи';
    },
    greeting() {
      const name = this.displayName;
      return name ? `${this.greetingPrefix}, ${name}!` : `${this.greetingPrefix}!`;
    },
    /** Сигнал раскрытия свёрнутого узла от онбординг-тура - см. watch ниже. */
    onboardingReveal() {
      return useOnboardingStore().revealOpen;
    },
  },
  watch: {
    '$route'() {
      this.fetchUserData();
    },
    /**
     * Тур просит показать список уведомлений (reveal.open): открываем его сам, а
     * по гашению сигнала закрываем - но только если открыли мы. Список, открытый
     * человеком до шага, тур не трогает.
     */
    onboardingReveal(target) {
      if (target === 'notifications') {
        // Флаг ставим и когда список уже открыт: на шаге про список им
        // распоряжается тур, кто бы его ни открыл. Иначе список, открытый
        // человеком по просьбе предыдущего шага, оставался висеть поверх
        // следующих шагов и закрывал собой то, о чём они рассказывают.
        this.notificationsOpenedByTour = true;
        this.showNotifications = true;
      } else if (this.notificationsOpenedByTour) {
        this.notificationsOpenedByTour = false;
        this.showNotifications = false;
      }
    },
  },
  mounted() {
    this.fetchUserData();
    this.fetchActiveAnnouncement();
    this.startDateTimeTimer();
    this.$nextTick(() => {
      this.initIntersectionObserver();
    });
    this._onDocumentClick = () => {
      // Пока список держит тур, клик мимо его не закрывает: шаг рассказывает
      // именно про открытый список, а окно шага живёт вне шапки - иначе клик по
      // окну гасил список и оставлял шаг ни с чем.
      if (useOnboardingStore().revealOpen === 'notifications') return;
      if (this.showNotifications) {
        this.showNotifications = false;
      }
    };
    document.addEventListener('click', this._onDocumentClick);
    this.initMobileWatcher();
  },
  beforeUnmount() {
    if (this.timer) {
      clearInterval(this.timer);
    }

    if (this.observer) {
      this.observer.disconnect();
    }
    document.removeEventListener('click', this._onDocumentClick);
    if (this._mobileMql && this._onMobileChange) {
      if (this._mobileMql.removeEventListener) {
        this._mobileMql.removeEventListener('change', this._onMobileChange);
      } else if (this._mobileMql.removeListener) {
        this._mobileMql.removeListener(this._onMobileChange);
      }
    }
  },
  methods: {
    // Гейтинг по правам (#187 Фаза 2). super -> всегда true, admin -> всё кроме
    // denied, обычный -> по эффективному гранту. Реактивно: читает стор прав.
    can(key) {
      return this.permissionsStore.hasPermission(key);
    },
    async fetchActiveAnnouncement() {
      try {
        const response = await apiRequest('/announcements/active', { method: 'GET' });
        if (response.ok) {
          this.activeAnnouncement = await response.json();
        }
      } catch (error) {
        console.error('Ошибка при загрузке объявления:', error);
      }
    },
    /** Открыть панель поиска и поставить в неё курсор. */
    openGlobalSearch() {
      this.$bus?.emit?.('global-search:open');
    },
    openFeedbackModal() {
      this.showFeedbackModal = true;
    },
    /**
     * Реактивно отслеживает мобильный брейкпоинт (совпадает с CSS @media 768):
     * на нём кнопка «Сообщить о проблеме» показывается в drawer, а не в шапке.
     */
    initMobileWatcher() {
      if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
      this._mobileMql = window.matchMedia('(max-width: 768px)');
      this.isMobileHeader = this._mobileMql.matches;
      this._onMobileChange = (e) => { this.isMobileHeader = e.matches; };
      if (this._mobileMql.addEventListener) {
        this._mobileMql.addEventListener('change', this._onMobileChange);
      } else if (this._mobileMql.addListener) {
        this._mobileMql.addListener(this._onMobileChange);
      }
    },
    toggleMobileNav() {
      this.$bus.emit('mobile-nav-toggle');
    },
    handleFeedbackSubmitted() {
      // Если мы на странице обратной связи, можно обновить список
      if (this.$route.path === '/feedback') {
        this.$emit('refresh-feedback');
      }
    },
    navigateToSubmit() {
      this.$router.push('/new-application');
    },
    async fetchUserData() {
      try {
        const authStore = useAuthStore();
        if (!authStore.token) {
          return;
        }

        const response = await apiRequest("/users/me", {
          method: "GET",
        });

        if (response.ok) {
          const userData = await response.json();
          this.userFirstName = userData.first_name || '';
          this.userLastName = userData.last_name || '';
        } else {
          console.error("Ошибка при загрузке данных пользователя");
        }
      } catch (error) {
        console.error("Ошибка сети при загрузке данных пользователя:", error);
      } finally {
        this.loading = false;
      }
    },
    // Дата и время в шапке (ДД.ММ.ГГГГ ЧЧ:ММ:СС, только десктоп) + текущий час для
    // приветствия («Доброе утро/день/вечер») - секундный таймер, как было до W3.
    //
    // Время московское и сверенное с сервером (#2298), а не с машиной: по этим часам
    // на посту сверяют срок пропуска и разрешённые часы въезда, и сбитые локальные
    // часы дали бы неверное решение, выглядя при этом правдоподобно.
    updateDateTime() {
      const now = serverNow();
      const h = moscowHour(now);
      if (h !== this.currentHour) {
        this.currentHour = h;
      }
      this.currentDateTime = formatMoscowDateTime(now);
    },
    startDateTimeTimer() {
      this.updateDateTime();
      this.timer = setInterval(() => {
        this.updateDateTime();
      }, 1000);
    },
    initIntersectionObserver() {
      this.observer = new IntersectionObserver(
        (entries) => {
          entries.forEach(entry => {
            this.isHeaderHidden = !entry.isIntersecting;
          });
        },
        {
          threshold: 0,
          rootMargin: '0px'
        }
      );

      if (this.$refs.header) {
        this.observer.observe(this.$refs.header);
      }
    }
  },
}
</script>

<style scoped>
h3 {
  font-size: 16px;
}

.header {
  width: 100%;
  min-height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--border);
  padding: 8px 20px;
  position: relative;
  z-index: 100;
  flex-wrap: wrap;
  gap: 12px;
}

.header__title {
  display:flex;
  flex-direction: column;
  gap: 0px;
  min-width: 0;
  flex-shrink: 1;
  transition: margin-left 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

/* Рельс скрыт: уводим приветствие вправо, чтобы не налезала плавающая кнопка
   возврата меню (#510). */
.header__title--shifted {
  margin-left: 45px;
}

.header__subtitle {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}

.header__info {
  display: flex;
  align-items: center;
  gap: 15px;
  position: relative;
  flex-wrap: wrap;
  justify-content: flex-end;
}

/* Порядок на десктопе: время, «Сообщить о проблеме», объявление, колокольчик, «Подать заявку». */
.header__time { order: 0; }
.feedback-btn { order: 1; }
.broadcast { order: 2; }
.user__notifications { order: 4; }
.appl-btn__container { order: 5; }
.search-btn { order: 6; }

/* Поиск: иконка у самого правого края. Размер как у прочих круглых контролов шапки. */
.search-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.search-btn:hover {
  background: var(--surface-2);
  color: var(--color-text);
}

/* Дата и время в шапке (только десктоп): серый цвет, размер, центрирование и
   min-width как в оригинале до W3; моноширинные цифры, чтобы секунды не дёргали
   ширину. */
.header__time {
  min-width: 160px;
  font-size: 16px;
  color: var(--text-muted);
  text-align: center;
  /* Якорь подсказки делает элемент inline-flex, а в нём text-align не работает:
     текст становится анонимным flex-элементом и прижимается влево (#2298). */
  justify-content: center;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.feedback-btn {
  height: 35px;
  font-size: 14px;
  color: var(--warning-text);
  border: none;
  outline: none;
  font-weight: 500;
  background: transparent;
  cursor: pointer;
  white-space: nowrap;
  padding: 0 15px;
  text-decoration: underline;
  text-decoration-color: transparent;
  transition: text-decoration-color 0.2s ease;
  text-underline-position: under;
}

/* Hover-эффекты - только для устройств с реальным ховером (мышь). На тач :hover
   «залипает» после тапа (иконка/кнопка остаётся в hover-состоянии) - гейтим их
   через @media (hover: hover), чтобы тап на мобилке не оставлял хвост (#1097 p2). */
@media (hover: hover) {
  .feedback-btn:hover {
    text-decoration-color: var(--warning-text);
  }
}

.broadcast {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: fit-content;
  padding: 0 15px;
  height: 35px;
  font-size: 14px;
  color: var(--warning-text);
  border: 1px solid color-mix(in srgb, var(--warning) 45%, var(--surface));
  outline: none;
  border-radius: 50px;
  font-weight: 500;
  background: var(--warning-bg);
  cursor: pointer;
  white-space: nowrap;
  transition: filter 0.2s ease;
}

@media (hover: hover) {
  .broadcast:hover {
    filter: brightness(0.96);
  }
}

.broadcast--important {
  color: var(--danger-text);
  background: var(--danger-bg);
  border-color: color-mix(in srgb, var(--danger) 45%, var(--surface));
}

@media (hover: hover) {
  .broadcast--important:hover {
    filter: brightness(0.96);
  }
}

.user__notifications {
  width: 35px;
  height: 35px;
  border-radius: 50%;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border);
  position: relative;
  cursor: pointer;
  transition: background-color 0.2s ease;
}

/* Состояния колокольчика выражены подложкой и прозрачностью, а НЕ подменой filter:
   локальный grayscale/contrast делал колокольчик то серым, то чёрным на тёмном
   фоне. Цвет глифа приходит от текста, менять его состоянию незачем. */
.user__notifications--active {
  background-color: var(--surface-2);
}

.user__notifications--active .notifications__icon,
.user__notifications--active .notifications__icon:hover {
  opacity: 0.55;
}

.notifications__badge {
  position: absolute;
  top: -4px;
  right: -4px;
  background-color: var(--danger);
  color: var(--fill-text);
  font-size: 11px;
  font-weight: 600;
  min-width: 18px;
  height: 18px;
  border-radius: 9px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 4px;
  line-height: 1;
}

.notifications__icon {
  width: 20px;
  height: 20px;
  cursor: pointer;
  transition: opacity 0.2s ease;
  color: var(--text);
}

@media (hover: hover) {
  .user__notifications:hover {
    background-color: var(--surface-2);
  }

  .notifications__icon:hover {
    opacity: 0.7;
  }
}

.appl-btn__container {
  width: fit-content;
  height: 26px;
}

.appl-btn {
  position: relative;
  height: 26px;
  width: fit-content;
  padding: 0 16px;
  font-size: 13px;
  color: var(--text);
  background-color: var(--surface);
  border: 1px solid var(--accent);
  outline: none;
  cursor: pointer;
  font-weight: 400;
  border-radius: 13px;
  transition: .2s;
}

@media (hover: hover) {
  .appl-btn:hover {
    background-color: var(--surface-2);
  }
}

/* Короткая подпись "Заявка" - отдельный класс (не модификатор общего),
   чтобы видимость не зависела от source-order при рефакторинге media-блока. */
.appl-btn__label-short {
  display: none;
}

.appl-btn--fixed {
  position: fixed;
  z-index: 1000;
}

/* Стили для фиксированной кнопки при скрытии шапки */
.appl-btn--fixed {
  position: fixed;
  top: 20px;
  right: 20px;
  z-index: 999;
  animation: slide-down 250ms cubic-bezier(0.4, 0, 0.2, 1);
}

@keyframes slide-down {
  from {
    transform: translateY(-12px);
    opacity: 0;
  }
  to {
    transform: translateY(0);
    opacity: 1;
  }
}

/* Burger-кнопка - только на мобильном. Оформлена как тайл свёрнутого навменю
   (белый бордюр-тайл с радиусом 12px, как .nav-item в рельсе) - B.2. */
.header__burger {
  display: none;
  width: 44px;
  height: 44px;
  padding: 0;
  border: 1px solid var(--border);
  background: var(--surface);
  cursor: pointer;
  flex-direction: column;
  gap: 4px;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: 12px;
  transition: background 0.2s, border-color 0.2s;
}

@media (hover: hover) {
  .header__burger:hover {
    background: var(--color-primary-tint);
    border-color: var(--accent);
  }
}

.header__burger span {
  display: block;
  width: 20px;
  height: 2px;
  background: var(--text);
  border-radius: 1px;
  transition: background 0.2s;
}

@media (hover: hover) {
  .header__burger:hover span {
    background: var(--color-primary);
  }
}

/* Адаптивность */
@media (max-width: 768px) {
  /* Шапка закреплена сверху при скролле - "Подать заявку" и колокольчик всегда доступны.
     Непрозрачный фон обязателен: контент уезжает под шапку.

     ПОЧЕМУ sticky, а не fixed: под шапкой на Центре/в ЛК стоит ВТОРАЯ закреплённая
     шапка (.center__header / .card-header - sticky). fixed прибивается композитором к
     visual viewport (следует за сворачиванием адресной строки), а sticky считается от
     layout-viewport - во время анимации адресной строки они едут по разным законам, между
     ними открывается и схлопывается зазор, и обе шапки визуально прыгают. Держим ОБЕ
     закреплённые полосы в одной системе отсчёта (sticky в общем скроллпорте документа) -
     относительный рассинхрон тогда невозможен. Синхронизировать JS-переменной
     visualViewport уже пробовали (#1214) и откатили: она отстаёт от композитора.
     sticky в потоке => padding-top у #main-content не нужен. */
  .header {
    position: sticky;
    top: 0;
    z-index: 100;
    background: var(--surface);
    padding: 0 12px;
    gap: 8px;
    /* Высота 55 (токен) синхронна sticky-top вложенных шапок. nowrap обязателен: при
       переносе (длинное «Важное объявление» на узком экране) реальная высота уходит на
       ~88px, а вложенные шапки/отступы считают 55 - контент и полосы прыгают. */
    min-height: var(--mobile-header-height);
    flex-wrap: nowrap;
  }

  /* Чтобы nowrap не распирал строку: блок действий может ужиматься. */
  .header__info {
    min-width: 0;
    flex-wrap: nowrap;
  }

  .header__burger {
    display: flex;
  }

  .header__info {
    gap: 8px;
  }

  /* Часы только на десктопе - мобильная шапка тесная (там их и убирали). */
  .header__time {
    display: none;
  }

  /* На мобилке порядок в строке: объявление, колокольчик, "Подать заявку" */
  .broadcast { order: 1; }
  .user__notifications { order: 2; }
  .appl-btn__container { order: 3; }

  /* Приветствие + подзаголовок "Мы рады..." скрыты на мобилке (директива юзера) */
  .header__title {
    display: none;
  }

  /* Объявление на мобилке - текстовый pill (правка волны 3), просто компактнее */
  .broadcast {
    height: 36px;
    font-size: 13px;
    padding: 0 14px;
  }

  /* Колокольчик - в самой шапке, укрупнён под тач (A.2) */
  .user__notifications {
    width: 36px;
    height: 36px;
  }

  /* Кнопка "Подать заявку" на мобилке - outline: белый фон, синий border+текст,
     radius 15px (не pill, менее круглая - директива юзера), без "+" иконки. */
  .appl-btn__container {
    width: auto;
    height: auto;
    border-radius: var(--radius-md);
    background: transparent;
    display: flex;
    align-items: center;
  }

  .appl-btn {
    height: 36px;
    width: auto;
    padding: 0 16px;
    border-radius: var(--radius-md);
    background: var(--surface);
    color: var(--accent-text);
    border: 1px solid var(--accent);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 14px;
    font-weight: 600;
    box-shadow: none;
  }

  @media (hover: hover) {
    .appl-btn:hover {
      background: var(--color-primary-tint);
    }
  }

  /* На мобилке полная надпись "Подать заявку" -> короткая "Заявка" (влезает в строку) */
  .appl-btn__label {
    display: none;
  }

  .appl-btn__label-short {
    display: inline;
  }
}

@media (max-width: 480px) {
  .header {
    padding: 0 10px;
  }
}

/* На очень узких экранах компактнее padding; "Заявка" оставляем видимой
   (без "+" иконки скрывать текст нельзя - кнопка стала бы пустой). */
@media (max-width: 360px) {
  .appl-btn {
    padding: 0 12px;
  }
}
</style>
