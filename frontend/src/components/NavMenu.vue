<template>
  <div class="nav-root">
    <!-- Backdrop для мобильного drawer'а -->
    <transition name="nav-backdrop">
      <div
        v-if="mobileOpen || swipeOffset > 0"
        class="nav-menu__backdrop"
        :class="{ 'nav-menu__backdrop--dragging': swipeDragging }"
        :style="swipeDragging ? { opacity: swipeBackdropOpacity } : null"
        @click="closeMobile"
      />
    </transition>

    <nav
      class="nav-menu"
      role="navigation"
      aria-label="Основная навигация"
      data-testid="ob-nav-rail"
      :class="{
        expanded: railExpanded,
        'nav-menu--pinned': uiStore.sidebarExpanded,
        'nav-menu--hidden': uiStore.sidebarHidden,
        'nav-menu--mobile-open': mobileOpen,
        'nav-menu--dragging': swipeDragging,
        'nav-menu--banned': isBanned,
      }"
      :style="swipeOffset ? { transform: `translateX(min(0px, calc(-100% + ${swipeOffset}px)))` } : null"
      @mouseenter="expandMenu"
      @mouseleave="collapseMenu"
    >
      <!-- Кнопка закрытия для мобильного -->
      <button
        v-if="mobileOpen"
        class="nav-menu__close"
        aria-label="Закрыть меню"
        @click="closeMobile"
      >
        ✕
      </button>

      <!-- «Сообщить о проблеме» переехало из шапки в drawer на мобилке (W3.3).
           Тот же testid, что у шапочной кнопки: на десктопе она в шапке, на
           мобилке - здесь, в DOM всегда ровно один экземпляр (тур находит нужный). -->
      <button
        v-if="mobileOpen && can('header.report_problem')"
        class="nav-menu__feedback"
        data-testid="header-button-feedback"
        @click="openFeedbackFromDrawer"
      >
        <span>Сообщить о проблеме</span>
      </button>

      <div class="nav-content">
        <!-- Бренд + контролы (пин/скрыть). В свёрнутом виде - только лого по центру. -->
        <div class="nav-head">
          <div class="nav-brand">
            <span class="nav-logo">
              <NavIcon
                name="logo"
                :size="20"
              />
            </span>
            <span class="nav-brand__name">Бюро<span>пропусков</span></span>
          </div>
          <div class="nav-controls">
            <button
              class="nav-ctrl nav-ctrl--pin"
              type="button"
              :class="{ 'is-pinned': uiStore.sidebarExpanded }"
              :title="uiStore.sidebarExpanded ? 'Открепить рельс' : 'Закрепить раскрытым'"
              :aria-pressed="uiStore.sidebarExpanded"
              @click="togglePin"
            >
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="1.7"
                stroke-linecap="round"
                stroke-linejoin="round"
                aria-hidden="true"
              >
                <path d="M9 4h6l-1 7 3 3v2H7v-2l3-3z" />
                <line
                  x1="12"
                  y1="16"
                  x2="12"
                  y2="21"
                />
              </svg>
            </button>
            <button
              class="nav-ctrl"
              type="button"
              title="Скрыть меню"
              @click="hideRail"
            >
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="1.7"
                stroke-linecap="round"
                stroke-linejoin="round"
                aria-hidden="true"
              >
                <polyline points="11 7 6 12 11 17" />
                <polyline points="17 7 12 12 17 17" />
              </svg>
            </button>
          </div>
        </div>


        <div class="nav-scroll">
          <!-- ЗАЯВКИ -->
          <div
            v-show="sectionVisible.requests"
            class="nav-section"
          >
            <div class="section-title">
              ЗАЯВКИ
            </div>
            <div
              v-show="can('page.center')"
              class="nav-item"
              :class="{ active: isActive('/center') }"
              data-testid="nav-link-center"
              @click="navigateToCenter"
            >
              <div class="nav-icon-wrapper">
                <NavIcon
                  name="center"
                  :size="18"
                  class="nav-icon"
                />
                <span
                  v-if="newApplicationsCount > 0"
                  class="icon-badge"
                >
                  {{ newApplicationsCount > 9 ? '9+' : newApplicationsCount }}
                </span>
              </div>
              <span class="nav-text">Центр заявок</span>
              <span
                v-if="newApplicationsCount > 0"
                class="notification-badge"
              >
                {{ newApplicationsCount > 9 ? '9+' : newApplicationsCount }}
              </span>
            </div>
            <div
              v-show="can('page.new_application')"
              class="nav-item"
              :class="{ active: isActive('/new-application') }"
              data-testid="nav-link-new-application"
              @click="navigateToNewApplication"
            >
              <NavIcon
                name="new-application"
                :size="18"
                class="nav-icon"
              />
              <span class="nav-text">Новая заявка</span>
            </div>
            <div
              v-show="authStore.canViewAccessibleAttachments"
              class="nav-item"
              :class="{ active: isActive('/accessible-attachments') }"
              data-testid="nav-link-accessible-attachments"
              @click="navigateToAccessibleAttachments"
            >
              <NavIcon
                name="attachment-types"
                :size="18"
                class="nav-icon"
              />
              <span class="nav-text">Доступные мне</span>
            </div>
          </div>

          <!-- УПРАВЛЕНИЕ ДАННЫМИ -->
          <div
            v-show="sectionVisible.data"
            class="nav-section"
          >
            <div class="section-title">
              УПРАВЛЕНИЕ ДАННЫМИ
            </div>

            <!-- Таблицы: дропдаун раскрывается по клику и разворачивается под пунктом -->
            <div
              v-show="tablesItemVisible"
              class="nav-item-container"
            >
              <div
                class="nav-item has-dropdown"
                :class="{ active: isActive('/table') }"
                data-testid="nav-link-tables"
                @click="toggleDropdown('tables')"
              >
                <div class="nav-item-content">
                  <NavIcon
                    name="tables"
                    :size="18"
                    class="nav-icon"
                  />
                  <span class="nav-text">Таблицы</span>
                </div>
                <svg
                  class="dropdown-arrow"
                  :class="{ rotated: dropdowns.tables }"
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.7"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                >
                  <polyline points="6 9 12 15 18 9" />
                </svg>
              </div>

              <!-- Раскрытие/сворачивание через grid-rows 0fr<->1fr - высота
                   анимируется плавно и двигает пункты ниже. -->
              <div
                class="dropdown-below"
                :class="{ open: tablesDropdownOpen && (railExpanded || mobileOpen) }"
              >
                <div
                  class="dropdown-below__inner"
                  data-testid="nav-tables-list"
                >
                  <template
                    v-for="group in groupedTables"
                    :key="group.key"
                  >
                    <!-- Подпись типа показываем только когда групп несколько:
                         на одном типе она была бы лишним шумом. -->
                    <div
                      v-if="groupedTables.length > 1"
                      class="dropdown-group-title"
                    >
                      {{ group.label }}
                    </div>
                    <div
                      v-for="table in group.tables"
                      :key="getTableId(table)"
                      class="dropdown-item"
                      :class="{ active: isCurrentTable(getTableName(table)) }"
                      @click="navigateToTable(getTableName(table))"
                    >
                      {{ getTableDisplayName(table) }}
                    </div>
                  </template>
                  <div
                    v-if="filteredTables.length === 0"
                    class="dropdown-item disabled"
                  >
                    Нет доступных таблиц
                  </div>
                </div>
              </div>
            </div>

            <!-- Онбординг подсвечивает группу «Сотрудники + Автомобили» (без
                 «Таблиц» - их нет у большинства ролей). -->
            <div data-testid="ob-nav-group-data">
              <div
                v-show="can('page.employees')"
                class="nav-item"
                :class="{ active: isActive('/employeesview') }"
                data-testid="nav-link-employees"
                @click="navigateToEmployeesView"
              >
                <NavIcon
                  name="employees"
                  :size="18"
                  class="nav-icon"
                />
                <span class="nav-text">Сотрудники</span>
              </div>

              <div
                v-show="can('page.cars')"
                class="nav-item"
                :class="{ active: isActive('/carsview') }"
                data-testid="nav-link-cars"
                @click="navigateToCarsView"
              >
                <NavIcon
                  name="cars"
                  :size="18"
                  class="nav-icon"
                />
                <span class="nav-text">Автомобили</span>
              </div>
            </div>
          </div>

          <!-- АНАЛИТИКА -->
          <div
            v-show="sectionVisible.analytics"
            class="nav-section"
          >
            <div class="section-title">
              АНАЛИТИКА
            </div>
            <div
              v-show="can('page.statistics')"
              class="nav-item"
              :class="{ active: isActive('/analytics') }"
              data-testid="nav-link-analytics"
              @click="$router.push('/analytics')"
            >
              <NavIcon
                name="analytics"
                :size="18"
                class="nav-icon"
              />
              <span class="nav-text">Аналитика</span>
            </div>
            <div
              v-show="can('page.statistics')"
              class="nav-item"
              :class="{ active: isReportsTab }"
              data-testid="nav-link-reports"
              @click="$router.push({ path: '/analytics', query: { tab: 'reports' } })"
            >
              <NavIcon
                name="analytics"
                :size="18"
                class="nav-icon"
              />
              <span class="nav-text">Отчёты</span>
            </div>
          </div>

          <!-- АДМИНИСТРИРОВАНИЕ: виден если есть хотя бы один доступный раздел
               (super/admin или гранты). Клик открывает колонку Админки. -->
          <div
            v-if="canSeeAdmin"
            v-show="sectionVisible.admin"
            class="nav-section"
          >
            <div class="section-title">
              АДМИНИСТРИРОВАНИЕ
            </div>
            <div
              v-show="canSeeAdmin"
              class="nav-item has-dropdown"
              :class="{ active: adminOpen }"
              data-testid="nav-link-admin"
              @click="toggleAdmin"
            >
              <div class="nav-item-content">
                <div class="nav-icon-wrapper">
                  <NavIcon
                    name="admin"
                    :size="18"
                    class="nav-icon"
                  />
                  <span
                    v-if="newFeedbackCount > 0"
                    class="icon-badge admin-icon-badge"
                    data-testid="nav-feedback-badge"
                  >
                    {{ newFeedbackCount > 9 ? '9+' : newFeedbackCount }}
                  </span>
                </div>
                <span class="nav-text">Администрирование</span>
              </div>
              <!-- Не дропдаун, а открытие боковой колонки: прямая стрелка вправо. -->
              <svg
                class="panel-indicator"
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="1.7"
                stroke-linecap="round"
                stroke-linejoin="round"
                aria-hidden="true"
              >
                <line
                  x1="4"
                  y1="12"
                  x2="18"
                  y2="12"
                />
                <polyline points="13 7 18 12 13 17" />
              </svg>
            </div>
          </div>
        </div>

        <!-- ПОЛЬЗОВАТЕЛЬ (низ) -->
        <div
          v-show="sectionVisible.user"
          class="nav-section user-section"
        >
          <div class="section-title">
            ПОЛЬЗОВАТЕЛЬ
          </div>
          <div
            v-show="can('page.news')"
            class="nav-item"
            :class="{ active: isActive('/news') }"
            data-testid="nav-link-news"
            @click="navigateToNews"
          >
            <NavIcon
              name="news"
              :size="18"
              class="nav-icon"
            />
            <span class="nav-text">Обзор и новости</span>
          </div>
          <div
            v-show="can('page.personal_cabinet')"
            class="nav-item"
            :class="{ active: isActive('/personal-cabinet') }"
            data-testid="nav-link-cabinet"
            @click="navigateToAccount"
          >
            <NavIcon
              name="cabinet"
              :size="18"
              class="nav-icon"
            />
            <span class="nav-text">Личный кабинет</span>
          </div>
          <!-- Тёмная тема: тумблер, тем осталось две (#1415) -->
          <div
            v-show="true"
            class="nav-item nav-item--theme"
            data-testid="nav-theme-toggle"
            role="switch"
            :aria-checked="isDarkTheme ? 'true' : 'false'"
            @click="toggleTheme"
          >
            <div class="nav-item-content">
              <NavIcon
                name="moon"
                :size="18"
                class="nav-icon"
              />
              <span class="nav-text">Тёмная тема</span>
            </div>
            <!-- Индикатор: клик обрабатывает вся строка, поэтому сам тумблер
                 события не ловит (иначе переключение сработало бы дважды). -->
            <SwitchToggle
              :model-value="isDarkTheme"
              label=""
              title="Переключить светлую и тёмную тему"
              class="nav-theme-switch"
            />
          </div>

          <div
            class="nav-item"
            data-testid="nav-button-logout"
            @click="logout"
          >
            <NavIcon
              name="logout"
              :size="18"
              class="nav-icon"
            />
            <span class="nav-text exit">Выйти</span>
          </div>
        </div>
      </div>
    </nav>

    <!-- Обратная связь из drawer'а (W3.3). Teleport->body, поверх закрытого drawer'а. -->
    <FeedbackModal
      v-model:show="showFeedbackModal"
      :auto-focus="false"
      @submitted="onFeedbackSubmitted"
    />

    <!--
      Двухколоночная Админка (#510): колонка справа от свёрнутого в иконки рельса,
      поверх готовых /admin/* роутов. Палитра наследуется от .nav-root. Без теней.
    -->
    <transition name="admin-col">
      <aside
        v-if="adminOpen"
        class="admin-column"
        aria-label="Администрирование"
        data-testid="admin-column"
      >
        <button
          class="admin-back"
          type="button"
          data-testid="admin-back"
          @click="closeAdmin"
        >
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.7"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <polyline points="15 6 9 12 15 18" />
          </svg>
          Назад
        </button>

        <div class="admin-column__head">
          <NavIcon
            name="admin"
            :size="18"
            class="admin-column__icon"
          />
          <span class="admin-column__title">Админка</span>
          <span class="admin-count">{{ adminCountLabel }}</span>
        </div>

        <div
          class="admin-search-row"
          data-testid="ob-admin-search"
        >
          <span class="admin-search-ic">
            <NavIcon
              name="search"
              :size="16"
            />
          </span>
          <input
            v-model="adminSearch"
            type="text"
            class="admin-search"
            placeholder="Поиск по админке..."
            aria-label="Поиск по разделам админки"
          >
        </div>

        <div
          class="admin-column__scroll"
          data-testid="ob-admin-groups"
        >
          <div
            v-for="group in filteredAdminGroups"
            :key="group.title"
            class="admin-group"
          >
            <div class="admin-group__title">
              {{ group.title }}
            </div>
            <div
              v-for="item in group.items"
              :key="item.path"
              class="admin-link"
              :class="{ active: isActive(item.path) }"
              :data-testid="`admin-link-${item.icon}`"
              @click="navigateToAdminPath(item.path)"
            >
              <NavIcon
                :name="item.icon"
                :size="18"
                class="admin-link__icon"
              />
              <span class="admin-link__label">{{ item.label }}</span>
              <span
                v-if="item.path === '/admin/feedback' && newFeedbackCount > 0"
                class="admin-link__badge"
                data-testid="admin-link-feedback-badge"
              >
                {{ newFeedbackCount > 9 ? '9+' : newFeedbackCount }}
              </span>
            </div>
          </div>
          <div
            v-if="filteredAdminGroups.length === 0"
            class="admin-empty"
          >
            Ничего не найдено
          </div>
        </div>
      </aside>
    </transition>

    <!-- Возврат рельса из full-hide: плавающая кнопка у левого края -->
    <transition name="nav-backdrop">
      <button
        v-if="uiStore.sidebarHidden"
        class="nav-unhide"
        type="button"
        aria-label="Показать меню"
        title="Показать меню"
        @click="showRail"
      >
        <svg
          width="20"
          height="20"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.7"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <line
            x1="4"
            y1="7"
            x2="20"
            y2="7"
          />
          <line
            x1="4"
            y1="12"
            x2="20"
            y2="12"
          />
          <line
            x1="4"
            y1="17"
            x2="20"
            y2="17"
          />
        </svg>
      </button>
    </transition>
  </div>
</template>

<script>
import { getCurrentInstance } from 'vue'
import { apiRequest } from '@/api/client'
import { getUnreadCount } from '@/api/applications'
import { getFeedbackStats } from '@/api/feedback'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'
import { useSoundStore } from '@/stores/sound'
import { usePermissionsStore } from '@/stores/permissions'
import { ADMIN_GROUPS } from '@/constants/navSections';
import { useThemeStore } from '@/stores/theme'
import { useOnboardingStore } from '@/stores/onboarding'
import { playPreset } from '@/utils/notificationSound'
import eventStream from '@/services/eventStream'
import { useNarrowScreen } from '@/composables/useNarrowScreen'
import { useEdgeSwipeOpen } from '@/composables/useEdgeSwipeOpen'
import NavIcon from '@/components/icons/NavIcon.vue'
import SwitchToggle from '@/components/ui/SwitchToggle.vue'
import FeedbackModal from '@/components/FeedbackModal.vue'

// Длительность анимации ухода drawer'а (transform 0.28s) - ждём её перед показом
// модалки обратной связи, иначе уезжающая панель (z-index 10000) секунду рисуется
// поверх появляющегося overlay модалки (9999) и подрезает форму.
const DRAWER_CLOSE_MS = 300
// Номинальная ширина drawer'а из мобильного @media: на неё опирается и свайп открытия.
const DRAWER_WIDTH = 280

export default {
  name: 'NavMenu',
  components: { NavIcon, FeedbackModal, SwitchToggle },
  emits: ['logout'],
  setup() {
    // Сторы берём в setup для реактивности в шаблоне: authStore - гейт
    // Администрирования, uiStore - состояния рельса (пин/hide, персист),
    // permissionsStore - гейтинг пунктов/таблиц/админки по правам (#187, Фаза 2).
    const authStore = useAuthStore()
    const uiStore = useUiStore()
    const soundStore = useSoundStore()
    const permissionsStore = usePermissionsStore()
    const themeStore = useThemeStore()
    // Онбординг-тур просит раскрыть колонку Админки на своих шагах (reveal.open).
    const onboardingStore = useOnboardingStore()
    // Свайп открытия drawer'а (W4.1). Состояние drawer'а живёт в data(), поэтому до
    // него дотягиваемся через инстанс: колбэки жеста зовутся уже после монтирования,
    // когда и поля, и методы на месте.
    const instance = getCurrentInstance()
    const { isNarrow } = useNarrowScreen()
    const drawerSwipe = useEdgeSwipeOpen(() => instance.proxy.openMobile(), {
      width: DRAWER_WIDTH,
      // Открытая модалка блокирует прокрутку фона инлайн-стилем - там свайп вправо
      // принадлежит ей (лист, карусель внутри), а не навигации под ней.
      isEnabled: () => isNarrow.value
        && !instance.proxy.mobileOpen
        && document.body.style.overflow !== 'hidden',
    })
    return {
      authStore, uiStore, soundStore, permissionsStore, themeStore, onboardingStore,
      swipeOffset: drawerSwipe.offset,
      swipeDragging: drawerSwipe.isDragging,
    }
  },
  data() {
    return {
      isExpanded: false,
      // Реестр тем оформления (#1415) - список и палитра кружков берутся из
      // utils/theme.js, тот же источник валидирует бэк.
      dropdowns: {
        tables: false
      },
      hoverTimeout: null,
      systemTables: [],
      // Бейдж у "Центра заявок" = сумма непрочитанных + заявок с обновлённым статусом (#1349).
      newApplicationsCount: 0,
      // База роста ТОЛЬКО непрочитанных - для гейта звука. Обновление статуса уже
      // прочитанной заявки не "новая заявка", звук на него не играем; поэтому сравниваем
      // рост count, а не суммы newApplicationsCount (#1349).
      unreadBaseCount: 0,
      // Непрочитанные обращения обратной связи (бейдж у Администрирования).
      // Персонально для админа: считаем по праву page.admin.feedback (как и виден
      // сам пункт), а не по isSuperAdmin - иначе счётчик не показывался обычным
      // администраторам. Эндпоинт /feedback/stats открыт тем же правом.
      newFeedbackCount: 0,
      // Звук новой заявки: primed=true после ПЕРВОЙ загрузки счётчика (чтобы не играть на
      // логине при 0 -> N); lastSoundAt - метка для кулдауна (пачка заявок -> один звук).
      soundPrimed: false,
      lastSoundAt: 0,
      // Токен последовательности опросов счётчика: конкурентные вызовы (30с-таймер + SSE)
      // могут резолвиться не по порядку - устаревший ответ с бОльшим count не должен
      // затирать актуальный и играть ложный звук. Пишет только последний запущенный.
      unreadFetchSeq: 0,
      applicationsPollingInterval: null,
      tablesPollingInterval: null,
      eventStreamOff: null,
      eventStreamFeedbackOff: null,
      eventStreamTablesOff: null,
      eventStreamStatusOff: null,
      sseConnected: false,
      unreadReadHandler: null,
      mobileOpen: false,
      // «Сообщить о проблеме» из drawer'а (W3.3): модалка та же, что в шапке.
      showFeedbackModal: false,
      isBanned: false,
      adminOpen: false,
      // Колонку раскрыл тур (reveal), а не пользователь - только такую он и закроет.
      adminOpenedByTour: false,
      adminSearch: '',
      // Разделы Админки по группам мокапа (#510). permission - ключ права на
      // раздел (совпадает с meta.permission роутов, #187 Фаза 2): пункт виден
      // только если can(permission). super/admin проходят, Техработы - super-only
      // (бэкенд денаит ключ в admin-режиме через каталог прав). Настройки (#7) -
      // точечный ключ page.admin.settings, не super-only: обычным админам его
      // выдаёт adminAll, точечно снимается личным deny-override.
      adminGroups: ADMIN_GROUPS,
    };
  },
  computed: {
    /** Отчёты живут вкладкой внутри аналитики, поэтому активность считается по параметру адреса. */
    isReportsTab() {
      return this.$route.path === '/analytics' && this.$route.query.tab === 'reports';
    },
    /** Тумблер в меню: включён на тёмной теме (#1415). */
    isDarkTheme() {
      return this.themeStore.current === 'dark';
    },
    /**
     * Затемнение густеет вместе с вытянутой панелью. Считаем от номинальной ширины
     * drawer'а: на узком экране он упирается в 85vw и доходит до края раньше, поэтому
     * значение подрезаем единицей.
     */
    swipeBackdropOpacity() {
      return Math.min(1, this.swipeOffset / DRAWER_WIDTH);
    },
    // Рельс раскрыт если закреплён (пин) или временно по hover. В full-hide
    // не раскрываем - рельс схлопнут в 0. При открытой Админке рельс
    // зафиксирован в иконках, чтобы не перекрывать колонку.
    railExpanded() {
      if (this.uiStore.sidebarHidden) return false;
      if (this.adminOpen) return false;
      // tourForceExpand - оверлейный разворот для онбординг-тура: расширяет рельс
      // как hover-превью, БЕЗ сдвига контента (--nav-ml зависит только от пина).
      return this.uiStore.sidebarExpanded || this.isExpanded || this.uiStore.tourForceExpand;
    },
    // Множество путей Админки - по нему route-watcher решает, закрывать ли колонку
    // (переход на «РАБОТА»/кабинет закрывает, переключение между разделами - нет).
    adminPathSet() {
      const set = new Set();
      this.adminGroups.forEach((g) => g.items.forEach((i) => set.add(i.path)));
      return set;
    },
    // Разделы Админки, доступные по правам (#187 Фаза 2): пункт остаётся, только
    // если can(item.permission). super/admin проходят, обычный юзер - по гранту.
    // Пустые группы отбрасываем. Поверх этого работают поиск и счётчик.
    permittedAdminGroups() {
      return this.adminGroups
        .map((g) => ({
          ...g,
          items: g.items.filter((i) => this.can(i.permission)),
        }))
        .filter((g) => g.items.length > 0);
    },
    // Видимость пункта/секции «Администрирование»: хотя бы один доступный раздел.
    canSeeAdmin() {
      return this.permittedAdminGroups.length > 0;
    },
    // Клиентский фильтр доступных разделов по подстроке (поиск в колонке Админки).
    filteredAdminGroups() {
      const q = this.adminSearch.trim().toLowerCase();
      if (!q) return this.permittedAdminGroups;
      return this.permittedAdminGroups
        .map((g) => ({ ...g, items: g.items.filter((i) => i.label.toLowerCase().includes(q)) }))
        .filter((g) => g.items.length > 0);
    },
    adminCount() {
      return this.permittedAdminGroups.reduce((n, g) => n + g.items.length, 0);
    },
    adminCountLabel() {
      const n = this.adminCount;
      const mod10 = n % 10;
      const mod100 = n % 100;
      let word = 'разделов';
      if (mod10 === 1 && mod100 !== 11) word = 'раздел';
      else if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) word = 'раздела';
      return `${n} ${word}`;
    },
    // Таблицы под правами (#187 Фаза 2): по гранту table.<name>.view
    // (super/admin видят все). Поиск по таблицам живёт в окне сквозного поиска.
    filteredTables() {
      return this.systemTables.filter(
        (t) => this.can(`table.${this.getTableName(t)}.view`),
      );
    },
    // Таблицы, разложенные по типу: машины и люди - иначе на нескольких постах
    // получается один длинный список без ориентиров. Порядок групп фиксирован,
    // пустые не показываются; тип, которого нет в словаре, попадает в «Прочие».
    groupedTables() {
      const labels = [
        ['cars', 'Автомобили'],
        ['people', 'Люди'],
      ];
      const groups = labels.map(([key, label]) => ({
        key,
        label,
        tables: this.filteredTables.filter((t) => this.getTableType(t) === key),
      }));
      const known = labels.map(([key]) => key);
      const rest = this.filteredTables.filter((t) => !known.includes(this.getTableType(t)));
      if (rest.length) {
        groups.push({ key: 'other', label: 'Прочие', tables: rest });
      }
      return groups.filter((g) => g.tables.length > 0);
    },
    // Пункт «Таблицы» виден если есть доступные таблицы; при поиске - ещё и если
    // совпала его метка. Нет доступных таблиц (нет грантов) - пункт скрыт целиком.
    tablesItemVisible() {
      return this.filteredTables.length > 0;
    },
    tablesDropdownOpen() {
      return this.dropdowns.tables;
    },
    // Видимость секции = есть хотя бы один доступный по правам пункт. Пустая секция
    // скрывается целиком, без осиротевшего заголовка.
    sectionVisible() {
      return {
        requests: this.can('page.center')
          || this.can('page.new_application')
          || this.authStore.canViewAccessibleAttachments,
        data: this.tablesItemVisible
          || this.can('page.employees')
          || this.can('page.cars'),
        analytics: this.can('page.statistics'),
        admin: this.canSeeAdmin,
        // «Выйти» и «Оформление» доступны всегда - секция пользователя видна всегда.
        user: true,
      };
    },
  },
  watch: {
    // Закрываем drawer при переходе по пункту меню; покинули раздел Админки
    // (клик «РАБОТА»/кабинет) - закрываем колонку.
    '$route'() {
      this.closeMobile();
      if (this.adminOpen && !this.adminPathSet.has(this.$route.path)) {
        this.closeAdmin();
      }
    },
    'uiStore.sidebarExpanded': 'syncContentMargin',
    'uiStore.sidebarHidden': 'syncContentMargin',
    // Онбординг просит показать колонку Админки. Закрываем только то, что открыли
    // сами: колонку, уже открытую пользователем, тур схлопывать не должен.
    'onboardingStore.revealOpen'(target) {
      if (target === 'admin-column') {
        if (!this.adminOpen) {
          this.adminOpenedByTour = true;
          this.toggleAdmin();
        }
      } else if (this.adminOpenedByTour) {
        this.adminOpenedByTour = false;
        this.closeAdmin();
      }
    },
  },
  async mounted() {
    document.body.classList.add('auth-active');
    this.syncContentMargin();
    // Права нужны для гейтинга пунктов/таблиц/админки и опроса новых заявок.
    // Ждём загрузку (идемпотентно - вернёт кэш, если router-guard уже загрузил),
    // чтобы первый опрос не стартовал у пользователя без page.center.
    await this.permissionsStore.fetchPermissions();
    await this.fetchBanStatus();
    await this.fetchSystemTables();
    this.startApplicationsPolling();
    this.startTablesPolling();

    // Слушаем событие от burger-кнопки в TheHeader
    this.$bus.on('mobile-nav-toggle', this.toggleMobile);
    // Переход на десктоп (>=769) закрывает мобильный drawer: иначе он завис бы
    // панелью на широком экране, а drawer-кнопка feedback (тот же testid, что в
    // шапке) осталась бы в DOM рядом с вернувшейся шапочной - тур нашёл бы дубль.
    if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
      this._desktopMql = window.matchMedia('(min-width: 769px)');
      this._onDesktopChange = (e) => { if (e.matches) this.closeMobile(); };
      if (this._desktopMql.addEventListener) {
        this._desktopMql.addEventListener('change', this._onDesktopChange);
      } else if (this._desktopMql.addListener) {
        this._desktopMql.addListener(this._onDesktopChange);
      }
    }
    // Esc закрывает верхний слой: сначала колонку Админки, затем мобильный drawer.
    this._escHandler = (e) => {
      if (e.key !== 'Escape') return;
      if (this.adminOpen) {
        this.closeAdmin();
      } else if (this.mobileOpen) {
        this.closeMobile();
      }
    };
    window.addEventListener('keydown', this._escHandler);

    // Клик вне рельса и колонки Админки закрывает колонку.
    this._docClickHandler = (e) => {
      if (!this.adminOpen) return;
      if (e.target.closest('.nav-menu') || e.target.closest('.admin-column')) return;
      this.closeAdmin();
    };
    document.addEventListener('mousedown', this._docClickHandler);
  },
  beforeUnmount() {
    document.body.classList.remove('auth-active');
    document.body.style.removeProperty('--nav-ml');
    this.stopApplicationsPolling();

    if (this.hoverTimeout) {
      clearTimeout(this.hoverTimeout);
    }

    this.$bus.off('mobile-nav-toggle', this.toggleMobile);
    if (this._desktopMql && this._onDesktopChange) {
      if (this._desktopMql.removeEventListener) {
        this._desktopMql.removeEventListener('change', this._onDesktopChange);
      } else if (this._desktopMql.removeListener) {
        this._desktopMql.removeListener(this._onDesktopChange);
      }
    }
    if (this._escHandler) {
      window.removeEventListener('keydown', this._escHandler);
    }
    if (this._docClickHandler) {
      document.removeEventListener('mousedown', this._docClickHandler);
    }
    // Обязательно снимаем body lock если компонент unmount'нулся в открытом состоянии
    document.body.classList.remove('nav-drawer-open');
  },
  methods: {
    // Гейтинг по правам (#187 Фаза 2). super -> всегда true, admin -> всё кроме
    // denied, обычный -> по эффективному гранту. Реактивно: читает стор прав.
    can(key) {
      return this.permissionsStore.hasPermission(key);
    },
    isActive(path) {
      const current = this.$route.path;
      // Только список таблиц /table и /table/:name, но НЕ /table-constructor.
      if (path === '/table') return current === '/table' || current.startsWith('/table/');
      if (path === '/admin') return current.startsWith('/admin');
      // У отчётов свой пункт меню, поэтому «Аналитика» держит остальные её вкладки.
      if (path === '/analytics') return current === path && !this.isReportsTab;
      return current === path;
    },
    // Текущая открытая таблица - для подсветки пункта в списке «Таблицы».
    // Сверяем по декодированному параметру роута /table/:tableName (надёжно к
    // кодированию кириллицы/спецсимволов в URL).
    isCurrentTable(tableName) {
      if (!tableName) return false;
      return this.$route.params.tableName === tableName;
    },
    togglePin() {
      this.uiStore.toggleSidebarPinned();
      // Снимаем временный hover-стейт, чтобы не конфликтовал с пином.
      if (this.hoverTimeout) {
        clearTimeout(this.hoverTimeout);
        this.hoverTimeout = null;
      }
    },
    hideRail() {
      this.uiStore.hideSidebar();
      this.isExpanded = false;
      this.closeAllDropdowns();
      this.closeAdmin();
    },
    showRail() {
      this.uiStore.showSidebar();
    },
    // Открытие/закрытие колонки Админки. При открытии рельс схлопывается в иконки
    // (railExpanded -> false), при закрытии возвращается в прежнее состояние.
    toggleAdmin() {
      if (this.adminOpen) {
        this.closeAdmin();
        return;
      }
      this.adminOpen = true;
      this.isExpanded = false;
      this.closeAllDropdowns();
    },
    closeAdmin() {
      this.adminOpen = false;
      this.adminSearch = '';
      // Колонку закрыли (Esc, клик вне, смена раздела) - тур больше не «владелец».
      this.adminOpenedByTour = false;
    },
    navigateToAdminPath(path) {
      this.$router.push(path);
      // На мобильном колонка оверлеит контент - закрываем после перехода, чтобы
      // показать страницу; на десктопе оставляем открытой для смены разделов.
      if (window.innerWidth <= 768) this.closeAdmin();
    },
    // Контент (#main-content) отслеживает персистентную ширину рельса. Контент
    // заходит на 25px под рельс (свёрнутый рельс 50px -> отступ 25; пин 248 -> 223;
    // hide -> 0). Колонка Админки и hover-разворот сюда НЕ входят - они оверлеят
    // контент, а не раздвигают. Переменную читает App.vue.
    syncContentMargin() {
      const width = this.uiStore.sidebarHidden
        ? '0px'
        : (this.uiStore.sidebarExpanded ? '124px' : '25px');
      document.body.style.setProperty('--nav-ml', width);
    },
    toggleMobile() {
      this.mobileOpen = !this.mobileOpen;
      // Блокируем scroll body когда drawer открыт
      document.body.classList.toggle('nav-drawer-open', this.mobileOpen);
    },
    // Открытие свайпом (W4.1): жест уже дотянул панель до порога, дальше её доводит
    // обычная анимация - как если бы нажали бургер.
    openMobile() {
      if (this.mobileOpen) return;
      this.mobileOpen = true;
      document.body.classList.add('nav-drawer-open');
    },
    closeMobile() {
      if (!this.mobileOpen) return;
      this.mobileOpen = false;
      document.body.classList.remove('nav-drawer-open');
    },
    /**
     * Открыть форму обратной связи из drawer'а. Закрываем drawer и ЖДЁМ конца его
     * анимации ухода перед показом модалки: drawer (z-index 10000) выше overlay
     * модалки (9999), открытая сразу форма ~0.3с перекрывалась бы уезжающей панелью.
     */
    async openFeedbackFromDrawer() {
      this.closeMobile();
      await new Promise((resolve) => setTimeout(resolve, DRAWER_CLOSE_MS));
      this.showFeedbackModal = true;
    },
    onFeedbackSubmitted() {
      // Новое обращение -> обновляем бейдж непрочитанных у Администрирования
      // (метод сам гейтит по праву page.admin.feedback).
      this.fetchNewFeedbackCount();
    },
    expandMenu() {
      // При открытой Админке рельс зафиксирован в иконках - hover не разворачивает.
      if (this.adminOpen) return;
      // Тап по бургеру на touch-устройстве синтезирует mouseenter на рельсе, как
      // только drawer выезжает под пальцем (#1097) - без гейта рельс переключался
      // в desktop-режим "expanded" (248px) вместо мобильных 280px/85vw.
      if (this.mobileOpen) return;
      if (this.hoverTimeout) {
        clearTimeout(this.hoverTimeout);
        this.hoverTimeout = null;
      }
      this.isExpanded = true;
    },
    collapseMenu() {
      this.hoverTimeout = setTimeout(() => {
        this.isExpanded = false;
        // Если рельс реально схлопывается (не закреплён пином) - сворачиваем и
        // раскрытые дропдауны, чтобы при следующем наведении они были закрыты.
        // У закреплённого рельса (пин) состояние держится: он не сворачивается.
        if (!this.uiStore.sidebarExpanded) this.closeAllDropdowns();
        this.hoverTimeout = null;
      }, 150);
    },
    // Таблицы и оформление: дропдаун раскрывается/сворачивается по клику (не hover).
    toggleDropdown(type) {
      this.dropdowns[type] = !this.dropdowns[type];
    },
    /**
     * Переключение светлой и тёмной темы (#1415). Стор применяет тему к <html>
     * сразу и сохраняет в профиль.
     */
    toggleTheme() {
      this.themeStore.setTheme(this.isDarkTheme ? 'light' : 'dark');
    },
    closeAllDropdowns() {
      Object.keys(this.dropdowns).forEach(key => {
        this.dropdowns[key] = false;
      });
    },

    // Работа с данными таблиц
    getTableId(table) {
      if (table.table && table.table.id) {
        return table.table.id;
      }
      return table.id;
    },

    getTableName(table) {
      if (table.table && table.table.name) {
        return table.table.name;
      }
      return table.name;
    },

    getTableType(table) {
      if (table.table && table.table.table_type) {
        return table.table.table_type;
      }
      return table.table_type || '';
    },

    getTableDisplayName(table) {
      if (table.table && table.table.display_name) {
        return table.table.display_name;
      }
      return table.display_name || 'Без названия';
    },

    navigateToTable(tableName) {
      if (tableName) {
        this.$router.push(`/table/${tableName}`);
        // Дропдаун НЕ закрываем: список остаётся, активная таблица подсвечена.
      }
    },

    navigateToCenter() {
      this.$router.push('/center');
    },
    navigateToAccessibleAttachments() {
      this.$router.push('/accessible-attachments');
      this.closeMobile();
    },
    navigateToNewApplication() {
      this.$router.push('/new-application');
      this.closeMobile();
    },
    navigateToAccount() {
      this.$router.push('/personal-cabinet');
    },
    navigateToNews() {
      this.$router.push('/news');
    },
    navigateToCarsView() {
      this.$router.push('/carsview');
    },
    navigateToEmployeesView() {
      this.$router.push('/employeesview');
    },

    async fetchBanStatus() {
      try {
        const res = await apiRequest('/users/me');
        if (res.ok) {
          // apiRequest разворачивает envelope ({success,data}->data), поэтому
          // is_banned лежит плоско. Было data.data.is_banned (=undefined) - бан
          // не детектился вовсе.
          const data = await res.json();
          this.isBanned = !!data?.is_banned;
        }
      } catch {
        // Не критично -- игнорируем сетевые ошибки
      }
    },

    async fetchSystemTables() {
      try {
        const response = await apiRequest("/system-tables", {
        });
        if (response.ok) {
          const data = await response.json();

          // Фильтруем только активные таблицы
          this.systemTables = data.filter(table => {
            const isActive = table.table ? table.table.is_active : table.is_active;
            return isActive;
          });
        }
      } catch (error) {
        console.error("Error fetching system tables:", error);
      }
    },

    async fetchNewApplicationsCount() {
      // Счётчик и звук новых заявок - операторская фича «Центра заявок». Без права
      // page.center не опрашиваем и не играем звук: иначе заявитель, ставший
      // responsible своей же заявки, получает сигнал о «новой заявке в Центре».
      if (!this.can('page.center')) {
        this.newApplicationsCount = 0
        // База роста тоже в 0: иначе при возврате права в рамках сессии следующий опрос
        // сравнит count с протухшей базой и сыграет/проглотит звук на мнимом росте.
        this.unreadBaseCount = 0
        return
      }
      const SOUND_COOLDOWN_MS = 5000
      const seq = ++this.unreadFetchSeq
      try {
        const data = await getUnreadCount()
        // Устаревший конкурентный ответ (не последний запущенный) игнорируем целиком:
        // иначе его снимок count затирает актуальный и играет ложный звук на мнимом росте.
        if (seq !== this.unreadFetchSeq) return
        const count = data.count || 0
        const statusUpdates = data.status_updates || 0
        // Звук только при РОСТЕ НЕПРОЧИТАННЫХ (count, не суммы) после первичной загрузки
        // (не на логине) и не чаще кулдауна - пачка заявок в одном опросе даёт +N за раз =
        // один звук. Обновление статуса прочитанной заявки бейдж увеличивает, но звука не
        // даёт. Звук только вне Центра заявок: на /center ApplicationsCenter сам играет.
        if (this.soundPrimed && count > this.unreadBaseCount && this.soundStore.enabled &&
            this.$route?.path !== '/center') {
          const now = Date.now()
          if (now - this.lastSoundAt > SOUND_COOLDOWN_MS) {
            playPreset(this.soundStore.selectedPreset, this.soundStore.volume)
            this.lastSoundAt = now
          }
        }
        this.unreadBaseCount = count
        this.newApplicationsCount = count + statusUpdates
        this.soundPrimed = true
      } catch {
        // При ошибке опроса НЕ обнуляем счётчик: сброс базы в 0 заставит следующий успешный
        // опрос сыграть ложный звук на "росте с нуля" (0 -> реальный N). Сохраняем последнее
        // известное значение и soundPrimed как есть - молча пропускаем неудачный тик.
      }
    },

    async fetchNewFeedbackCount() {
      if (!this.can('page.admin.feedback')) {
        this.newFeedbackCount = 0;
        return;
      }
      try {
        const stats = await getFeedbackStats();
        this.newFeedbackCount = stats?.unread || 0;
      } catch {
        // При ошибке опроса сохраняем последнее известное значение, а не обнуляем:
        // сброс в 0 при сетевом сбое даёт мигание бейджа (был N -> 0 -> снова N).
        // На первом опросе значение и так дефолтное 0 - бейдж просто не появится.
      }
    },

    startApplicationsPolling() {
      this.fetchNewApplicationsCount();
      this.fetchNewFeedbackCount();
      this.applicationsPollingInterval = setInterval(() => {
        this.fetchNewApplicationsCount();
        this.fetchNewFeedbackCount();
      }, 30000);
      // Real-time (#840): по сигналу о новой заявке сразу пересчитываем счётчик и
      // (вне /center) играем звук - не дожидаясь следующего 30с-опроса. fetchNewApplicationsCount
      // сам гейтит звук по росту счётчика, праву page.center и route !== '/center'.
      eventStream.connect();
      this.eventStreamOff = eventStream.subscribe('applications-center', () => {
        this.fetchNewApplicationsCount();
      });
      // Новое обращение обратной связи -> мгновенно пересчитать бейдж (#840);
      // fetchNewFeedbackCount сам гейтит по праву page.admin.feedback.
      this.eventStreamFeedbackOff = eventStream.subscribe('feedback', () => {
        this.fetchNewFeedbackCount();
      });
      // Изменение набора системных таблиц -> обновить список в нав-меню без 60с-опроса (#840).
      this.eventStreamTablesOff = eventStream.subscribe('system-tables', () => {
        this.fetchSystemTables();
      });
      // Статус SSE гейтит 60с-опрос таблиц (см. startTablesPolling): на живом
      // соединении список обновляет сигнал, опрос молчит.
      this.eventStreamStatusOff = eventStream.onStatus((status) => {
        this.sseConnected = status === 'connected';
      });
      // Прочтение заявки в Центре гасит счётчик непрочитанных сразу, не дожидаясь 30с-опроса
      // (ApplicationsCenter эмитит 'application-read' после успешного POST /read).
      this.unreadReadHandler = () => this.fetchNewApplicationsCount();
      this.$bus.on('application-read', this.unreadReadHandler);
      // Прочтение обращения в админке гасит бейдж обратной связи сразу, не дожидаясь
      // 30с-опроса (FeedbackPage эмитит 'feedback-read' после успешного PUT /read).
      this.feedbackReadHandler = () => this.fetchNewFeedbackCount();
      this.$bus.on('feedback-read', this.feedbackReadHandler);
    },

    startTablesPolling() {
      this.tablesPollingInterval = setInterval(() => {
        // На живом SSE список таблиц обновляет сигнал system-tables.refresh -
        // опрос молчит; при разрыве возобновляется (#840).
        if (this.sseConnected) return;
        this.fetchSystemTables();
      }, 60000);
    },

    stopApplicationsPolling() {
      if (this.applicationsPollingInterval) {
        clearInterval(this.applicationsPollingInterval);
        this.applicationsPollingInterval = null;
      }
      if (this.tablesPollingInterval) {
        clearInterval(this.tablesPollingInterval);
        this.tablesPollingInterval = null;
      }
      if (this.eventStreamOff) {
        this.eventStreamOff();
        this.eventStreamOff = null;
      }
      if (this.eventStreamFeedbackOff) {
        this.eventStreamFeedbackOff();
        this.eventStreamFeedbackOff = null;
      }
      if (this.eventStreamTablesOff) {
        this.eventStreamTablesOff();
        this.eventStreamTablesOff = null;
      }
      if (this.eventStreamStatusOff) {
        this.eventStreamStatusOff();
        this.eventStreamStatusOff = null;
      }
      if (this.unreadReadHandler) {
        this.$bus.off('application-read', this.unreadReadHandler);
        this.unreadReadHandler = null;
      }
      if (this.feedbackReadHandler) {
        this.$bus.off('feedback-read', this.feedbackReadHandler);
        this.feedbackReadHandler = null;
      }
      eventStream.disconnect();
    },

    logout() {
      this.stopApplicationsPolling();
      this.$emit('logout');
    }
  }
};
</script>

<style scoped>
/*
 * Палитра навигации (#510), scoped на корне навигации (.nav-root): и рельс, и
 * колонка Админки, и кнопка возврата наследуют отсюда. Свои имена оставлены -
 * это слой смысла ИМЕННО навигации, но значения теперь берутся из темы (#1415),
 * а не из хексов мокапа, иначе меню оставалось бы светлым в тёмной теме.
 *
 * Акцент разведён на две роли: --nav-primary для ТЕКСТА (активный пункт, hover)
 * и --nav-accent для ЗАЛИВОК (бейджи, логотип) с подписью --nav-on-accent.
 * Слить их нельзя: в тёмной теме заливка светлая, и белая цифра на бейдже
 * стала бы нечитаемой.
 */
.nav-root {
  --nav-primary: var(--accent-text);
  --nav-accent: var(--accent);
  --nav-on-accent: var(--accent-contrast);
  --nav-primary-hover: var(--accent-hover);
  --nav-primary-soft: var(--accent-tint);
  --nav-primary-soft-strong: color-mix(in srgb, var(--accent) 16%, transparent);
  --nav-text: var(--text);
  --nav-text-muted: var(--text-muted);
  --nav-text-faint: color-mix(in srgb, var(--text-muted) 80%, var(--surface));
  --nav-border: var(--border);
  --nav-border-soft: color-mix(in srgb, var(--border) 55%, var(--surface));
  --nav-bg: var(--surface);
  --nav-hover: var(--row-hover);
  --nav-scrollbar: var(--border);
}

.nav-menu {
  position: fixed;
  left: 0;
  top: 0;
  width: 50px;
  /* height:100% (не 100vh): под корневым CSS zoom единица vh считается от
   * НЕзумленной высоты окна, и рельс становится выше видимой области - нижние
   * разделы (Обзор/ЛК/Выйти) уезжают под экран. Для position:fixed height:100%
   * = высота вьюпорта и корректна и с zoom, и без него. */
  height: 100%;
  background: var(--nav-bg);
  border-right: 1px solid var(--nav-border);
  z-index: 1000;
  transition: width 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  display: flex;
}

.nav-menu * {
  box-sizing: border-box;
}

/* nav-content следует за шириной рельса (100%), а не зафиксирован 248px. Тогда
   при разворачивании пункты/иконки/лейблы анимируются единым transition ширины
   рельса, без телепорта, и центрируются в свёрнутом естественно. */
.nav-content {
  width: 100%;
  display: flex;
  flex-direction: column;
  position: relative;
}

.nav-menu.expanded {
  width: 248px;
}

/* full-hide: рельс схлопнут в 0, не перехватывает клики */
.nav-menu.nav-menu--hidden {
  width: 0;
  border-right-color: transparent;
  pointer-events: none;
}

/* Шапка: лого + контролы (пин/скрыть). Контролы видны только в развёрнутом.
   Лого (32px) с padding-left 9 центрируется в свёрнутых 50px (9px с каждой стороны). */
.nav-head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 56px;
  padding: 14px 9px 8px;
  flex-shrink: 0;
}

.nav-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

.nav-logo {
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  border-radius: 10px;
  display: grid;
  place-items: center;
  color: var(--nav-on-accent);
  background: linear-gradient(135deg, var(--nav-primary-hover), var(--nav-accent));
}

.nav-brand__name {
  display: flex;
  flex-direction: column;
  font-family: 'Montserrat', sans-serif;
  font-weight: 700;
  font-size: 14px;
  line-height: 1.1;
  color: var(--nav-text);
  white-space: nowrap;
  overflow: hidden;
  opacity: 0;
  transform: translateX(-6px);
  transition: opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1), transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.nav-brand__name span {
  font-weight: 500;
  font-size: 11px;
  color: var(--nav-text-muted);
}

.nav-menu.expanded .nav-brand__name {
  opacity: 1;
  transform: translateX(0);
  transition-delay: 0.05s;
}

.nav-controls {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
  opacity: 0;
  pointer-events: none;
  transform: translateX(6px);
  transition: opacity 0.2s ease, transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.nav-menu.expanded .nav-controls {
  opacity: 1;
  pointer-events: auto;
  transform: translateX(0);
  transition-delay: 0.05s;
}

.nav-ctrl {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  background: transparent;
  border-radius: 8px;
  color: var(--nav-text-muted);
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.nav-ctrl:hover {
  background: var(--nav-hover);
  color: var(--nav-primary);
}

.nav-ctrl--pin.is-pinned {
  color: var(--nav-primary);
  background: var(--nav-primary-soft);
}

/* Прокручиваемая середина (секции выше ПОЛЬЗОВАТЕЛЬ). */
.nav-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  scrollbar-width: thin;
  scrollbar-color: var(--nav-scrollbar) transparent;
}

/* Резервируем колонку под скролл-бар только в РАЗВЁРНУТОМ меню: когда пунктов
   больше высоты экрана (напр. после добавления раздела «Оформление»), полоса
   появляется в зарезервированном жёлобе, а не поверх правого края пунктов
   (бейджи/стрелки ⌄→). Без этого overlay-бар Chrome рисуется на контенте, а
   классический дёргает раскладку при появлении. В свёрнутом рельсе (50px) НЕ
   резервируем - там пункты только иконки по центру, 5px-бар их не пересекает,
   а жёлоб зря ужимал бы content-box (см. хрупкое центрирование рельса #510). */
.nav-menu.expanded .nav-scroll {
  scrollbar-gutter: stable;
}

.nav-scroll::-webkit-scrollbar {
  width: 5px;
}

.nav-scroll::-webkit-scrollbar-thumb {
  background: var(--nav-scrollbar);
  border-radius: 3px;
}

/*
 * Плавающая кнопка возврата рельса из full-hide. Внутри .nav-root - наследует
 * палитру навигации.
 */
.nav-unhide {
  position: fixed;
  left: 12px;
  top: 12px;
  z-index: 1100;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border: 1px solid var(--nav-border);
  background: var(--nav-bg);
  border-radius: 12px;
  color: var(--nav-text-muted);
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.nav-unhide:hover {
  background: var(--nav-hover);
  color: var(--nav-primary);
}

.nav-section {
  display: flex;
  flex-direction: column;
  padding-bottom: 4px;
}

/* Линии-разделители между секциями (видны в развёрнутом виде). Толщина 1px
   зарезервирована всегда - меняется только цвет, плавно вместе с разворотом. */
.nav-scroll .nav-section + .nav-section,
.user-section {
  border-top: 1px solid var(--nav-border-soft);
  transition: border-top-color 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.nav-menu:not(.expanded) .nav-scroll .nav-section + .nav-section,
.nav-menu:not(.expanded) .user-section {
  border-top-color: transparent;
}

.user-section {
  flex-shrink: 0;
  padding-bottom: 12px;
}

/* Заголовок секции: высота и отступы зарезервированы в ОБОИХ состояниях, поэтому
   вертикальная раскладка пунктов одинакова в свёрнутом и развёрнутом виде. При
   наведении меняется только ширина рельса, а заголовок лишь проявляется (opacity
   + transform) - без анимации height/margin, то есть без reflow и без скачка высоты. */
.section-title {
  font-family: 'Montserrat', sans-serif;
  font-weight: 700;
  font-size: 10px;
  line-height: 10px;
  letter-spacing: 0.04em;
  /* Подпись секции - текст, а не выключенный контрол: faint-тон оставлен
     заблокированным пунктам (для них контраст не нормируется), здесь нужен
     читаемый muted (замер faint: 3.72 в светлой теме, ниже нормы AA). */
  color: var(--nav-text-muted);
  text-transform: uppercase;
  height: 10px;
  margin: 14px 0 6px 24px;
  overflow: hidden;
  white-space: nowrap;
  opacity: 0;
  transform: translateX(-10px);
  transition: opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1), transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.nav-menu.expanded .section-title {
  opacity: 1;
  transform: translateX(0);
  transition-delay: 0.05s;
}

.nav-item-container {
  position: relative;
}

/* Пункт растягивается на ширину nav-content минус margin 7px с каждой стороны.
   В свёрнутом - центрированный inset-блок ~36px; иконка (margin 7 + padding-left 9)
   имеет центр на 25px = центр 50px-рельса. Анимируется вместе с шириной рельса. */
.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 12px 0 9px;
  margin: 4px 7px;
  border-radius: 12px;
  cursor: pointer;
  position: relative;
  overflow: hidden;
  min-height: 40px;
  flex-shrink: 0;
  color: var(--nav-text);
  text-decoration: none;
  transition: background-color 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.nav-item.has-dropdown {
  justify-content: space-between;
}

.nav-item-content {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.nav-item:hover:not(.disabled) {
  background-color: var(--nav-hover);
}

.nav-item:hover:not(.disabled) .nav-icon {
  color: var(--nav-primary);
}

.nav-item.disabled {
  cursor: default;
  opacity: 0.45;
}

.nav-item.disabled .nav-icon {
  color: var(--nav-text-faint);
}

/* active: фон primary-soft + левая полоса primary + текст/иконка primary */
.nav-item.active {
  background-color: var(--nav-primary-soft);
}

.nav-item.active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 7px;
  bottom: 7px;
  width: 3px;
  border-radius: 0 3px 3px 0;
  background: var(--nav-primary);
}

/* В свёрнутом виде у активного пункта только центрированный фон-блок (без полосы
   у края) - пункт не прижат ни к левому, ни к правому краю. */
.nav-menu:not(.expanded) .nav-item.active::before {
  display: none;
}

.nav-item.active .nav-icon,
.nav-item.active .nav-text {
  color: var(--nav-primary);
}

.nav-item:hover .exit {
  color: var(--danger-text);
}

.nav-icon-wrapper {
  position: relative;
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}

.nav-icon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  color: var(--nav-text-muted);
  transition: color 0.2s ease;
}

.nav-text {
  font-family: 'Montserrat', sans-serif;
  font-weight: 500;
  font-size: 13px;
  line-height: 16px;
  color: var(--nav-text);
  white-space: nowrap;
  overflow: hidden;
  flex-shrink: 0;
  opacity: 0;
  transform: translateX(-5px);
  transition: opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1), transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.nav-menu.expanded .nav-text {
  opacity: 1;
  transform: translateX(0);
  transition-delay: 0.1s;
}

.dropdown-arrow {
  flex-shrink: 0;
  color: var(--nav-text-muted);
  opacity: 0;
  transform: rotate(0deg);
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.dropdown-arrow.rotated {
  transform: rotate(180deg);
}

.nav-menu.expanded .dropdown-arrow {
  opacity: 0.7;
  transition-delay: 0.15s;
}

/* Индикатор «открывается боковая панель» у «Администрирования» - виден только
   в развёрнутом рельсе, как и dropdown-arrow, но без поворота. */
.panel-indicator {
  flex-shrink: 0;
  color: var(--nav-text-muted);
  opacity: 0;
  transition: opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.nav-menu.expanded .panel-indicator {
  opacity: 0.7;
  transition-delay: 0.15s;
}

/* Список таблиц раскрывается под пунктом «Таблицы». Высота анимируется через
   grid-template-rows 0fr<->1fr - плавно, и двигает пункты ниже. */
.dropdown-below {
  display: grid;
  grid-template-rows: 0fr;
  transition: grid-template-rows 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.dropdown-below.open {
  grid-template-rows: 1fr;
}

.dropdown-below__inner {
  overflow: hidden;
  min-height: 0;
  margin: 0 7px 4px;
  padding-left: 22px;
}

/* Подпись типа таблиц внутри выпадающего списка - тот же язык, что у подписей
   секций рельса, но мельче и без внешних отступов секции. */
.dropdown-group-title {
  font-family: 'Montserrat', sans-serif;
  font-weight: 700;
  font-size: 9.5px;
  letter-spacing: 0.04em;
  /* Подпись группы - текст, а не выключенный контрол: faint-тон оставлен
     заблокированным пунктам, здесь нужен читаемый muted (замер на staging
     давал 4.09). Та же правка уже сделана у подписи секции и группы Админки. */
  color: var(--nav-text-muted);
  text-transform: uppercase;
  padding: 6px 12px 4px;
  white-space: nowrap;
  overflow: hidden;
}

/* Между группами нужен заметный воздух, иначе списки читаются как один. */
.dropdown-group-title + .dropdown-item {
  margin-top: 2px;
}

.dropdown-item + .dropdown-group-title {
  margin-top: 14px;
}

.dropdown-item {
  padding: 7px 12px;
  border-radius: 10px;
  font-family: 'Montserrat', sans-serif;
  font-size: 12.5px;
  color: var(--nav-text-muted);
  font-weight: 500;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.dropdown-item.active {
  color: var(--nav-primary);
  background-color: var(--nav-primary-soft);
  font-weight: 600;
}

.dropdown-item:hover {
  background-color: var(--nav-hover);
  color: var(--nav-primary);
}

.dropdown-item.disabled {
  color: var(--nav-text-faint);
  cursor: not-allowed;
}

.dropdown-item.disabled:hover {
  background-color: transparent;
  color: var(--nav-text-faint);
}

/* Пункт «Тёмная тема»: тумблер справа, как подпись - проявляется только на
   раскрытом рельсе, в иконках остаётся один значок темы. */
.nav-item--theme {
  justify-content: space-between;
}

.nav-theme-switch {
  flex-shrink: 0;
  /* Клик ловит вся строка: сам тумблер здесь индикатор, иначе клик по нему
     переключал бы тему дважды. */
  pointer-events: none;
  opacity: 0;
  transform: translateX(-5px);
  transition: opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1), transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.nav-menu.expanded .nav-theme-switch {
  opacity: 1;
  transform: translateX(0);
  transition-delay: 0.1s;
}

.icon-badge {
  position: absolute;
  top: -6px;
  right: -6px;
  background-color: var(--nav-accent);
  color: var(--nav-on-accent);
  font-size: 10px;
  font-weight: 600;
  min-width: 16px;
  height: 16px;
  border-radius: 15px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 3px;
  border: 1.5px solid var(--nav-bg);
  z-index: 2;
  line-height: 1;
  opacity: 1;
  transform: scale(1);
  transition: opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1), transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.nav-menu.expanded .icon-badge {
  opacity: 0;
  transform: scale(0);
}

.notification-badge {
  position: absolute;
  right: 15px;
  background-color: var(--nav-accent);
  color: var(--nav-on-accent);
  font-size: 10px;
  font-weight: 600;
  min-width: 16px;
  height: 16px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 4px;
  z-index: 2;
  opacity: 0;
  transform: scale(0);
  transition: opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1), transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.nav-menu.expanded .notification-badge {
  opacity: 1;
  transform: scale(1);
  transition-delay: 0.2s;
}

/* У Администрирования длинный лейбл + стрелка-индикатор не оставляют места справа,
   поэтому счётчик держим на иконке и в развёрнутом рельсе (не прячем как у других). */
.nav-menu.expanded .admin-icon-badge {
  opacity: 1;
  transform: scale(1);
}

/*
 * Двухколоночная Админка (#510): колонка справа от свёрнутого в иконки рельса
 * (left: 50px). Без теней - отделение бордером. Палитра - от .nav-root.
 */
.admin-column {
  position: fixed;
  left: 50px;
  top: 0;
  width: 264px;
  height: 100%; /* не 100vh: зум-безопасно, см. .nav-menu выше */
  background: var(--nav-bg);
  border-right: 1px solid var(--nav-border);
  z-index: 999;
  display: flex;
  flex-direction: column;
  padding: 12px 0 16px;
  overflow: hidden;
}

.admin-back {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  align-self: flex-start;
  margin: 0 14px 10px;
  padding: 4px 6px;
  border: none;
  background: transparent;
  border-radius: 8px;
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  font-weight: 600;
  color: var(--nav-text-muted);
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.admin-back:hover {
  background: var(--nav-hover);
  color: var(--nav-primary);
}

.admin-column__head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 16px 10px;
}

.admin-column__icon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  color: var(--nav-text);
}

.admin-column__title {
  font-family: 'Montserrat', sans-serif;
  font-weight: 700;
  font-size: 18px;
  line-height: 22px;
  color: var(--nav-text);
}

.admin-count {
  font-family: 'Montserrat', sans-serif;
  font-size: 11px;
  font-weight: 600;
  color: var(--nav-primary);
  background: var(--nav-primary-soft);
  padding: 2px 8px;
  border-radius: var(--radius-pill, 999px);
  white-space: nowrap;
}

.admin-search-row {
  position: relative;
  display: flex;
  align-items: center;
  margin: 0 16px 8px;
}

.admin-search-ic {
  position: absolute;
  left: 11px;
  display: flex;
  align-items: center;
  color: var(--nav-text-faint);
  pointer-events: none;
  z-index: 1;
}

.admin-search {
  width: 100%;
  height: 34px;
  padding: 0 12px 0 34px;
  border: 1px solid var(--nav-border);
  border-radius: var(--radius-md, 15px);
  background: var(--nav-bg);
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  color: var(--nav-text);
  outline: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.admin-search::placeholder {
  color: var(--nav-text-muted);
}

.admin-search:focus {
  border-color: var(--nav-primary);
  box-shadow: 0 0 0 3px var(--nav-primary-soft);
}

.admin-column__scroll {
  flex: 1;
  overflow-y: auto;
  padding: 4px 0;
  scrollbar-width: thin;
  scrollbar-color: var(--nav-scrollbar) transparent;
}

.admin-column__scroll::-webkit-scrollbar {
  width: 6px;
}

.admin-column__scroll::-webkit-scrollbar-thumb {
  background: var(--nav-scrollbar);
  border-radius: 3px;
}

.admin-group__title {
  font-family: 'Montserrat', sans-serif;
  font-weight: 700;
  font-size: 10px;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--nav-text-muted);
  margin: 14px 0 6px 24px;
}

.admin-link {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 9px 15px;
  margin: 2px 12px;
  border-radius: 12px;
  cursor: pointer;
  position: relative;
  color: var(--nav-text);
  transition: background-color 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.admin-link__icon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  color: var(--nav-text-muted);
  transition: color 0.2s ease;
}

.admin-link__label {
  font-family: 'Montserrat', sans-serif;
  font-weight: 500;
  font-size: 13px;
  line-height: 16px;
  color: var(--nav-text);
  white-space: nowrap;
}

.admin-link__badge {
  margin-left: auto;
  background-color: var(--nav-accent);
  color: var(--nav-on-accent);
  font-size: 10px;
  font-weight: 600;
  min-width: 16px;
  height: 16px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 4px;
  line-height: 1;
}

.admin-link:hover {
  background-color: var(--nav-hover);
}

.admin-link:hover .admin-link__icon {
  color: var(--nav-primary);
}

.admin-link.active {
  background-color: var(--nav-primary-soft);
}

/* Активная полоса вынесена к левому краю колонки (к линии свёрнутого рельса),
   отдельно от фонового блока пункта (у которого свой inset-фон). */
.admin-link.active::before {
  content: '';
  position: absolute;
  left: -12px;
  top: 6px;
  bottom: 6px;
  width: 3px;
  border-radius: 0 3px 3px 0;
  background: var(--nav-primary);
}

.admin-link.active .admin-link__icon,
.admin-link.active .admin-link__label {
  color: var(--nav-primary);
}

.admin-empty {
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  color: var(--nav-text-muted);
  padding: 16px 24px;
}

/* Открытие/закрытие колонки - только transform + opacity (150-300ms). */
.admin-col-enter-active,
.admin-col-leave-active {
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.admin-col-enter-from,
.admin-col-leave-to {
  transform: translateX(-12px);
  opacity: 0;
}

/*
 * Mobile drawer (<768px): nav скрыт по-умолчанию, открывается через burger
 * в TheHeader (emits $bus mobile-nav-toggle). Без теней по требованию #510 -
 * отделение от контента через backdrop и border-right.
 */
.nav-menu__backdrop {
  display: none;
}

.nav-menu__close {
  display: none;
  position: absolute;
  top: 12px;
  right: 12px;
  width: 44px;
  height: 44px;
  border: none;
  background: transparent;
  font-size: 22px;
  color: var(--nav-text);
  cursor: pointer;
  border-radius: 50%;
  z-index: 2;
}

.nav-menu__close:hover {
  background: var(--nav-hover);
}

/* «Сообщить о проблеме» в drawer'е (W3.3) - только на мобилке (@media768). */
.nav-menu__feedback {
  display: none;
}

.nav-backdrop-enter-active,
.nav-backdrop-leave-active {
  transition: opacity 0.25s ease;
}
.nav-backdrop-enter-from,
.nav-backdrop-leave-to {
  opacity: 0;
}

/* Невысокие десктоп-окна: полный набор разделов (+«Оформление») не помещается по
   высоте близок к переполнению. Занижаем ТОЛЬКО высоту пунктов (min-height
   40->36), чтобы на типовом ноуте (высота окна ~825) меню влезало без скролл-бара
   с ПОЛНЫМИ отступами - ни зазор между пунктами (margin 4px), ни отступы между
   секциями (14px) не трогаем (проверено: на 825 всё влезает; заголовки секций не
   прилипают к разделителям). Только десктоп-рельс (min-width:769): у мобильного
   drawer <=768 свои тач-таргеты 48px, а телефоны тоже ниже 880 по высоте и попали
   бы под правило без гарда ширины. Высокие экраны (>880) не тронуты. Правило НЕ
   заскоуплено на .expanded намеренно: свёрнутый рельс занижается так же, иначе
   высота пунктов прыгала бы при hover-разворачивании; центрирование иконки
   (padding-left) не задето (урок #510). Ниже ~800px меню длиннее окна и честно
   скроллится - бар в зарезервированном жёлобе (scrollbar-gutter), не поверх
   пунктов. */
@media (min-width: 769px) and (max-height: 880px) {
  .nav-item {
    min-height: 36px;
  }
}

@media (max-width: 768px) {
  .nav-menu {
    width: 280px;
    max-width: 85vw;
    transform: translateX(-100%);
    transition: transform 0.28s cubic-bezier(0.2, 0.8, 0.2, 1);
    z-index: 10000;
    /* Нативный dvh: композитор держит высоту по видимой области без reflow-лага.
       Под дефолтным meta viewport dvh не реагирует на клавиатуру - drawer остаётся
       на всю высоту, без прыжков под баром браузера (#1097 R5-S3). */
    height: 100dvh;
  }

  /* Drawer всегда фиксированной "мобильной" ширины - даже если применился
     .expanded (десктопный hover-разворот 248px, напр. синтетический mouseenter
     от тапа по бургеру или персистентный пин с десктопа): та же специфичность,
     побеждает по порядку объявления. */
  .nav-menu.expanded {
    width: 280px;
    max-width: 85vw;
  }

  .nav-menu.nav-menu--mobile-open {
    transform: translateX(0);
  }

  /* Пока панель идёт за пальцем, transition только мешает - она должна стоять ровно
     там, где палец, а не догонять его. По отпусканию класс снимается, и панель
     доезжает обычной кривой от текущего места. */
  .nav-menu.nav-menu--dragging {
    transition: none;
  }

  /* padding-top держит запас под pill «Сообщить о проблеме» (top:12 + height:34 = 46)
     + отступ вниз, чтобы бренд/пункты не липли к кнопке. */
  .nav-menu .nav-content {
    width: 100%;
    padding-top: 66px;
  }

  /* В drawer'е всё всегда развёрнуто - hover/collapse не работают на touch.
     Перебиваем свёрнутые desktop-оверрайды. */
  .nav-menu .nav-brand__name,
  .nav-menu .nav-text,
  .nav-menu .section-title,
  .nav-menu .dropdown-arrow,
  .nav-menu .panel-indicator {
    opacity: 1 !important;
    transform: none !important;
    pointer-events: auto;
  }

  /* Тумблер темы проявляется по .expanded, а drawer этот класс не получает
     (expandMenu гейтит hover-разворот на мобилке) - показываем отдельно.
     pointer-events остаются выключены: клик ловит вся строка. */
  .nav-menu .nav-theme-switch {
    opacity: 1 !important;
    transform: none !important;
  }

  /* Пин/сворачивание рельса бессмысленны в тач-drawer'е (B.1, W3.3) - убираем. */
  .nav-menu .nav-controls {
    display: none !important;
  }

  .nav-menu .nav-item,
  .nav-menu:not(.expanded) .nav-item {
    width: auto;
    padding: 9px 15px;
    gap: 12px;
    margin: 2px 8px;
    min-height: 48px;
    justify-content: flex-start;
  }

  .nav-menu .nav-item.has-dropdown,
  .nav-menu:not(.expanded) .nav-item.has-dropdown,
  .nav-menu .nav-item--theme,
  .nav-menu:not(.expanded) .nav-item--theme {
    justify-content: space-between;
  }

  .nav-menu .nav-item-content,
  .nav-menu:not(.expanded) .nav-item-content {
    gap: 12px;
    justify-content: flex-start;
  }

  .nav-menu:not(.expanded) .section-title {
    height: auto;
    margin: 14px 0 6px 24px;
  }

  .nav-menu .nav-head,
  .nav-menu:not(.expanded) .nav-head {
    justify-content: flex-start;
    padding: 14px 12px 8px;
  }

  .nav-menu:not(.expanded) .nav-scroll {
    overflow-y: auto;
  }

  .nav-menu:not(.expanded) .nav-scroll .nav-section + .nav-section,
  .nav-menu:not(.expanded) .user-section {
    border-top-color: var(--nav-border-soft);
  }

  .nav-menu__backdrop {
    display: block;
    position: fixed;
    inset: 0;
    background: var(--overlay);
    z-index: 9999;
  }

  /* Затемнение густеет ровно настолько, насколько вытянута панель, поэтому догонять
     палец переходом ему нельзя - только мгновенное значение. */
  .nav-menu__backdrop--dragging {
    transition: none;
  }

  .nav-menu__close {
    display: flex;
    align-items: center;
    justify-content: center;
  }

  /* «Сообщить о проблеме» - компактный pill в верхней полосе drawer'а, слева от
     крестика (W3.3). fit-content вместо растяжки, круглая, меньше padding; nav-content
     держит padding-top под неё с запасом (отступ вниз). */
  .nav-menu__feedback {
    display: inline-flex;
    align-items: center;
    position: absolute;
    /* top 17 (не 12) центрирует 34px-пилюлю по вертикали с 44px-крестиком справа
       (их центры совпадают на одной линии) - навпанель 1. */
    top: 17px;
    left: 12px;
    right: auto;
    width: fit-content;
    max-width: calc(100% - 76px);
    height: 34px;
    padding: 0 16px;
    border: 1px solid var(--nav-border);
    background: transparent;
    color: var(--nav-text);
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    border-radius: var(--radius-pill);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    z-index: 2;
  }

  .nav-menu__feedback:hover {
    background: var(--nav-hover);
  }

  /* Тач-таргеты >=44px (WCAG 2.5.5): в drawer'е и колонке Админки контролы
     рельса, поиск, пункты таблиц - под палец, не под курсор. */
  .nav-menu .nav-ctrl {
    width: 44px;
    height: 44px;
  }

  .nav-menu .dropdown-item {
    display: flex;
    align-items: center;
    min-height: 44px;
  }

  .nav-unhide {
    width: 44px;
    height: 44px;
  }

  /* Колонка Админки на мобильном оверлеит как самостоятельная панель поверх
     drawer'а (рельс уезжает за край), а не лепится к 50px-рельсу. */
  .admin-column {
    left: 0;
    width: 280px;
    max-width: 85vw;
    z-index: 10001;
    /* Нативный dvh, как у .nav-menu - не обрезается/не прыгает под баром браузера. */
    height: 100dvh;
  }

  .admin-back {
    display: inline-flex;
    align-items: center;
    min-height: 44px;
  }

  .admin-search-row {
    height: 44px;
  }

  .admin-search {
    height: 100%;
  }

  .admin-link {
    min-height: 44px;
  }
}

/*
 * Заблокированный пользователь (#230). Все nav-items серым/disabled,
 * pointer-events отключены кроме pages из списка allowed (личный кабинет).
 */
.nav-menu--banned .nav-item:not([data-testid="nav-link-cabinet"]):not(.banned-passthrough),
.nav-menu--banned .dropdown-item,
.nav-menu--banned .has-dropdown {
  pointer-events: none !important;
  opacity: 0.4;
  cursor: not-allowed !important;
}

.nav-menu--banned .nav-item:not([data-testid="nav-link-cabinet"]):not(.banned-passthrough):hover,
.nav-menu--banned .dropdown-item:hover {
  background: transparent !important;
  color: inherit !important;
}
</style>
