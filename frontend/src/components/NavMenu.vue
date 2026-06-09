<template>
  <div class="nav-root">
    <!-- Backdrop для мобильного drawer'а -->
    <transition name="nav-backdrop">
      <div
        v-if="mobileOpen"
        class="nav-menu__backdrop"
        @click="closeMobile"
      />
    </transition>

    <nav
      class="nav-menu"
      role="navigation"
      aria-label="Основная навигация"
      :class="{
        expanded: railExpanded,
        'nav-menu--pinned': uiStore.sidebarExpanded,
        'nav-menu--hidden': uiStore.sidebarHidden,
        'nav-menu--mobile-open': mobileOpen,
        'nav-menu--banned': isBanned,
      }"
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

      <div class="nav-content">
        <!-- Верх рельса (виден в развёрнутом виде): контролы + поиск -->
        <div class="nav-top">
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
                <polyline points="7 7 12 12 7 17" />
                <polyline points="13 7 18 12 13 17" />
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
                <line
                  x1="6"
                  y1="5"
                  x2="6"
                  y2="19"
                />
                <polyline points="13 7 8 12 13 17" />
              </svg>
            </button>
          </div>
          <input
            v-model="searchQuery"
            type="text"
            class="nav-search"
            placeholder="Поиск..."
            aria-label="Поиск по меню"
          >
        </div>

        <!-- РАБОТА -->
        <div class="nav-section">
          <div class="section-title">
            РАБОТА
          </div>

          <div
            v-show="matches('Центр заявок')"
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

          <!-- Таблицы (выпадающий список) -->
          <div
            v-show="matches('Таблицы')"
            class="nav-item-container"
          >
            <div
              class="nav-item has-dropdown"
              :class="{ active: isActive('/table') }"
              @mouseenter="openDropdown('tables')"
              @mouseleave="handleDropdownLeave('tables')"
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
                <polyline points="9 6 15 12 9 18" />
              </svg>
            </div>

            <transition name="dropdown-fade">
              <div
                v-show="dropdowns.tables"
                class="dropdown-list dropdown-right"
                @mouseenter="keepDropdownOpen('tables')"
                @mouseleave="closeDropdown('tables')"
              >
                <div
                  v-for="table in systemTables"
                  :key="getTableId(table)"
                  class="dropdown-item"
                  @click="navigateToTable(getTableName(table))"
                >
                  {{ getTableDisplayName(table) }}
                </div>
                <div
                  v-if="systemTables.length === 0"
                  class="dropdown-item disabled"
                >
                  Нет доступных таблиц
                </div>
              </div>
            </transition>
          </div>

          <div
            v-show="matches('Сотрудники')"
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
            v-show="matches('Автомобили')"
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

          <div
            v-show="matches('Новости')"
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
            <span class="nav-text">Новости</span>
          </div>

          <!-- Администрирование: видно только супер-админу. Клик открывает
               двухколоночную Админку, рельс при этом схлопывается в иконки. -->
          <div
            v-if="authStore.isSuperAdmin"
            v-show="matches('Администрирование')"
            class="nav-item has-dropdown"
            :class="{ active: adminOpen || isActive('/admin') }"
            data-testid="nav-link-admin"
            @click="toggleAdmin"
          >
            <div class="nav-item-content">
              <NavIcon
                name="admin"
                :size="18"
                class="nav-icon"
              />
              <span class="nav-text">Администрирование</span>
            </div>
            <svg
              class="dropdown-arrow"
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
              <polyline points="9 6 15 12 9 18" />
            </svg>
          </div>
        </div>

        <!-- Низ: личный кабинет + выход -->
        <div class="nav-section user-section">
          <div
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
          Назад в работу
        </button>

        <div class="admin-column__head">
          <span class="admin-column__title">Админка</span>
          <span class="admin-count">{{ adminCountLabel }}</span>
        </div>

        <input
          v-model="adminSearch"
          type="text"
          class="admin-search"
          placeholder="Поиск по админке..."
          aria-label="Поиск по разделам админки"
        >

        <div class="admin-column__scroll">
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
import { apiRequest } from '@/api/client'
import { getUnreadCount } from '@/api/applications'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'
import NavIcon from '@/components/icons/NavIcon.vue'

export default {
  name: 'NavMenu',
  components: { NavIcon },
  emits: ['logout'],
  setup() {
    // Сторы берём в setup для реактивности в шаблоне: authStore - гейт
    // Администрирования, uiStore - состояния рельса (пин/hide, персист).
    const authStore = useAuthStore()
    const uiStore = useUiStore()
    return { authStore, uiStore }
  },
  data() {
    return {
      isExpanded: false,
      dropdowns: {
        tables: false
      },
      hoverTimeout: null,
      dropdownTimeout: null,
      dropdownLeaveTimeout: null,
      systemTables: [],
      newApplicationsCount: 0,
      applicationsPollingInterval: null,
      tablesPollingInterval: null,
      mobileOpen: false,
      isBanned: false,
      searchQuery: '',
      adminOpen: false,
      adminSearch: '',
      // Разделы Админки по группам мокапа (#510). Все роуты /admin/* уже заведены
      // (срезы 1,2) с requiresBuro - доступ супер-админа без правок бэкенда прав.
      adminGroups: [
        {
          title: 'Доступ и роли',
          items: [
            { label: 'Пользователи', icon: 'users', path: '/admin/users' },
            { label: 'Роли', icon: 'roles', path: '/admin/roles' },
            { label: 'Группы прав', icon: 'permission-groups', path: '/admin/permission-groups' },
            { label: 'Журнал отказов', icon: 'access-denials', path: '/admin/access-denials' },
            { label: 'Чёрный список', icon: 'blacklist', path: '/admin/blacklist' },
          ],
        },
        {
          title: 'Справочники',
          items: [
            { label: 'Организации', icon: 'organizations', path: '/admin/organizations' },
            { label: 'Компании', icon: 'companies', path: '/admin/companies' },
            { label: 'Места разгрузки', icon: 'unload-places', path: '/admin/unload-places' },
            { label: 'Форматы номеров', icon: 'number-formats', path: '/admin/number-formats' },
            { label: 'Гражданства', icon: 'citizenship', path: '/admin/citizenship' },
            { label: 'Марки авто', icon: 'marks', path: '/admin/marks' },
            { label: 'Типы вложений', icon: 'attachment-types', path: '/admin/attachment-types' },
            { label: 'Типы пользователей', icon: 'user-types', path: '/admin/user-types' },
            { label: 'Принимающие', icon: 'approvers', path: '/admin/approvers' },
          ],
        },
        {
          title: 'Система',
          items: [
            { label: 'Настройки', icon: 'settings', path: '/admin/settings' },
            { label: 'Конструктор таблиц', icon: 'table-constructor', path: '/table-constructor' },
            { label: 'Техработы', icon: 'system-control', path: '/admin/system-control' },
          ],
        },
        {
          title: 'Аудит и связь',
          items: [
            { label: 'Обратная связь', icon: 'feedback', path: '/admin/feedback' },
            { label: 'Мониторинг запросов', icon: 'requests', path: '/admin/requests' },
          ],
        },
      ],
    };
  },
  computed: {
    // Рельс раскрыт если закреплён (пин) или временно по hover. В full-hide
    // не раскрываем - рельс схлопнут в 0. При открытой Админке рельс
    // зафиксирован в иконках, чтобы не перекрывать колонку.
    railExpanded() {
      if (this.uiStore.sidebarHidden) return false;
      if (this.adminOpen) return false;
      return this.uiStore.sidebarExpanded || this.isExpanded;
    },
    // Множество путей Админки - по нему route-watcher решает, закрывать ли колонку
    // (переход на «РАБОТА»/кабинет закрывает, переключение между разделами - нет).
    adminPathSet() {
      const set = new Set();
      this.adminGroups.forEach((g) => g.items.forEach((i) => set.add(i.path)));
      return set;
    },
    // Клиентский фильтр разделов по подстроке (поиск в колонке Админки).
    filteredAdminGroups() {
      const q = this.adminSearch.trim().toLowerCase();
      if (!q) return this.adminGroups;
      return this.adminGroups
        .map((g) => ({ ...g, items: g.items.filter((i) => i.label.toLowerCase().includes(q)) }))
        .filter((g) => g.items.length > 0);
    },
    adminCount() {
      return this.adminGroups.reduce((n, g) => n + g.items.length, 0);
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
  },
  async mounted() {
    document.body.classList.add('auth-active');
    this.syncContentMargin();
    await this.fetchBanStatus();
    await this.fetchSystemTables();
    this.startApplicationsPolling();
    this.startTablesPolling();

    // Слушаем событие от burger-кнопки в TheHeader
    this.$bus.on('mobile-nav-toggle', this.toggleMobile);
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
  },
  beforeUnmount() {
    document.body.classList.remove('auth-active');
    document.body.style.removeProperty('--nav-ml');
    this.stopApplicationsPolling();

    if (this.hoverTimeout) {
      clearTimeout(this.hoverTimeout);
    }
    if (this.dropdownTimeout) {
      clearTimeout(this.dropdownTimeout);
    }
    if (this.dropdownLeaveTimeout) {
      clearTimeout(this.dropdownLeaveTimeout);
    }

    this.$bus.off('mobile-nav-toggle', this.toggleMobile);
    if (this._escHandler) {
      window.removeEventListener('keydown', this._escHandler);
    }
    // Обязательно снимаем body lock если компонент unmount'нулся в открытом состоянии
    document.body.classList.remove('nav-drawer-open');
  },
  methods: {
    isActive(path) {
      const current = this.$route.path;
      if (path === '/table') return current.startsWith('/table');
      if (path === '/admin') return current.startsWith('/admin');
      return current === path;
    },
    // Клиентский фильтр пунктов рельса по подстроке (поиск в развёрнутом виде).
    matches(label) {
      const q = this.searchQuery.trim().toLowerCase();
      if (!q) return true;
      return label.toLowerCase().includes(q);
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
    },
    navigateToAdminPath(path) {
      this.$router.push(path);
      // На мобильном колонка оверлеит контент - закрываем после перехода, чтобы
      // показать страницу; на десктопе оставляем открытой для смены разделов.
      if (window.innerWidth <= 768) this.closeAdmin();
    },
    // Контент (#main-content) отслеживает персистентную ширину рельса: hide -> 0,
    // пин -> развёрнутый, иначе свёрнутый. Колонка Админки и hover-разворот сюда
    // НЕ входят - они оверлеят контент, а не раздвигают его. Переменную читает App.vue.
    syncContentMargin() {
      const width = this.uiStore.sidebarHidden
        ? '0px'
        : (this.uiStore.sidebarExpanded ? '248px' : '50px');
      document.body.style.setProperty('--nav-ml', width);
    },
    toggleMobile() {
      this.mobileOpen = !this.mobileOpen;
      // Блокируем scroll body когда drawer открыт
      document.body.classList.toggle('nav-drawer-open', this.mobileOpen);
    },
    closeMobile() {
      if (!this.mobileOpen) return;
      this.mobileOpen = false;
      document.body.classList.remove('nav-drawer-open');
    },
    expandMenu() {
      // При открытой Админке рельс зафиксирован в иконках - hover не разворачивает.
      if (this.adminOpen) return;
      if (this.hoverTimeout) {
        clearTimeout(this.hoverTimeout);
        this.hoverTimeout = null;
      }
      this.isExpanded = true;
    },
    collapseMenu() {
      this.hoverTimeout = setTimeout(() => {
        this.isExpanded = false;
        this.closeAllDropdowns();
        this.hoverTimeout = null;
      }, 150);
    },
    openDropdown(type) {
      if (this.isExpanded) {
        if (this.dropdownLeaveTimeout) {
          clearTimeout(this.dropdownLeaveTimeout);
          this.dropdownLeaveTimeout = null;
        }
        Object.keys(this.dropdowns).forEach(key => {
          if (key !== type) {
            this.dropdowns[key] = false;
          }
        });
        this.dropdowns[type] = true;
      }
    },
    handleDropdownLeave(type) {
      this.dropdownLeaveTimeout = setTimeout(() => {
        if (!this.isDropdownHovered) {
          this.dropdowns[type] = false;
        }
      }, 150);
    },
    keepDropdownOpen(type) {
      if (this.dropdownLeaveTimeout) {
        clearTimeout(this.dropdownLeaveTimeout);
      }
      this.dropdowns[type] = true;
    },
    closeDropdown(type) {
      this.dropdownLeaveTimeout = setTimeout(() => {
        this.dropdowns[type] = false;
      }, 150);
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

    getTableDisplayName(table) {
      if (table.table && table.table.display_name) {
        return table.table.display_name;
      }
      return table.display_name || 'Без названия';
    },

    navigateToTable(tableName) {
      if (tableName) {
        this.$router.push(`/table/${tableName}`);
        this.closeAllDropdowns();
      }
    },

    navigateToCenter() {
      this.$router.push('/center');
      this.closeAllDropdowns();
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
          const data = await res.json();
          this.isBanned = !!data?.data?.is_banned;
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
      try {
        const data = await getUnreadCount()
        this.newApplicationsCount = data.count || 0
      } catch {
        this.newApplicationsCount = 0
      }
    },

    startApplicationsPolling() {
      this.fetchNewApplicationsCount();
      this.applicationsPollingInterval = setInterval(() => {
        this.fetchNewApplicationsCount();
      }, 30000);
    },

    startTablesPolling() {
      this.tablesPollingInterval = setInterval(() => {
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
 * Палитра навигации (#510) - точные хексы мокапа, scoped на корне навигации
 * (.nav-root). И рельс, и колонка Админки, и кнопка возврата наследуют отсюда;
 * глобальный tokens.css не трогаем, чтобы цвета мокапа не утекли на весь сайт.
 * Радиусы/шрифт - из проектных токенов.
 */
.nav-root {
  --nav-primary: #4F5BDF;
  --nav-primary-hover: #3d49c7;
  --nav-primary-soft: rgba(79, 91, 223, .10);
  --nav-primary-soft-strong: rgba(79, 91, 223, .16);
  --nav-text: #1f2330;
  --nav-text-muted: #8a90a2;
  --nav-text-faint: #aab0c0;
  --nav-border: #e9eaf0;
  --nav-border-soft: #f0f1f6;
  --nav-bg: #ffffff;
  --nav-hover: #f4f5fb;
  --nav-scrollbar: #e0e2ee;
}

.nav-menu {
  position: fixed;
  left: 0;
  top: 0;
  width: 50px;
  height: 100vh;
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

.nav-content {
  width: 248px;
  display: flex;
  flex-direction: column;
  position: relative;
}

.nav-menu.expanded {
  width: 248px;
  overflow: visible;
}

/* full-hide: рельс схлопнут в 0, не перехватывает клики */
.nav-menu.nav-menu--hidden {
  width: 0;
  border-right-color: transparent;
  pointer-events: none;
}

/* Верх рельса: контролы (пин/скрыть) + поиск. Видны только в развёрнутом виде;
   в свёрнутом схлопнуты в 0, чтобы не занимать место над пунктами. */
.nav-top {
  height: 0;
  padding: 0;
  overflow: hidden;
  opacity: 0;
  pointer-events: none;
  transition: height 0.25s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1), padding 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.nav-menu.expanded .nav-top {
  height: auto;
  padding: 12px 0 6px;
  opacity: 1;
  pointer-events: auto;
  transition-delay: 0.05s;
}

.nav-controls {
  display: flex;
  justify-content: flex-end;
  gap: 4px;
  padding: 0 12px 8px;
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

.nav-ctrl--pin svg {
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.nav-ctrl--pin.is-pinned svg {
  transform: rotate(180deg);
}

.nav-search {
  width: calc(100% - 24px);
  margin: 0 12px;
  height: 34px;
  padding: 0 12px;
  border: 1px solid var(--nav-border);
  border-radius: var(--radius-md, 15px);
  background: var(--nav-bg);
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  color: var(--nav-text);
  outline: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.nav-search::placeholder {
  color: var(--nav-text-faint);
}

.nav-search:focus {
  border-color: var(--nav-primary);
  box-shadow: 0 0 0 3px var(--nav-primary-soft);
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
}

.user-section {
  margin-top: auto;
  margin-bottom: 16px;
}

.section-title {
  font-family: 'Montserrat', sans-serif;
  font-weight: 700;
  font-size: 10px;
  line-height: 10px;
  letter-spacing: 0.04em;
  color: var(--nav-text-faint);
  text-transform: uppercase;
  margin: 16px 0 8px 24px;
  height: 10px;
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

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 9px 15px;
  margin: 2px 8px;
  border-radius: 12px;
  cursor: pointer;
  position: relative;
  overflow: hidden;
  min-height: 38px;
  width: 232px;
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

.nav-item.active .nav-icon,
.nav-item.active .nav-text {
  color: var(--nav-primary);
}

.nav-item:hover .exit {
  color: #e5484d;
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
  transform: rotate(90deg);
}

.nav-menu.expanded .dropdown-arrow {
  opacity: 0.7;
  transition-delay: 0.15s;
}

.dropdown-list {
  position: absolute;
  left: 248px;
  top: 0;
  background: var(--nav-bg);
  border-radius: 0 15px 15px 0;
  z-index: 1001;
  border: 1px solid var(--nav-border);
  border-left: none;
  min-width: 210px;
  transform-origin: left center;
  overflow: hidden;
  padding: 6px;
}

.user-section .dropdown-list {
  top: auto;
  bottom: 0;
}

.dropdown-item {
  padding: 8px 14px;
  border-radius: 10px;
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  color: var(--nav-text);
  font-weight: 500;
  cursor: pointer;
  white-space: nowrap;
  transition: background-color 0.2s ease, color 0.2s ease;
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

.icon-badge {
  position: absolute;
  top: -6px;
  right: -6px;
  background-color: var(--nav-primary);
  color: #fff;
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
  background-color: var(--nav-primary);
  color: #fff;
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

.dropdown-fade-enter-active,
.dropdown-fade-leave-active {
  transition: opacity 0.25s cubic-bezier(0.4, 0, 0.2, 1), transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.dropdown-fade-enter-from,
.dropdown-fade-leave-to {
  opacity: 0;
  transform: translateX(-10px) scale(0.97);
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
  height: 100vh;
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
  font-size: 12px;
  font-weight: 500;
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

.admin-search {
  margin: 0 16px 8px;
  height: 34px;
  padding: 0 12px;
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
  color: var(--nav-text-faint);
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
  color: var(--nav-text-faint);
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

.admin-link:hover {
  background-color: var(--nav-hover);
}

.admin-link:hover .admin-link__icon {
  color: var(--nav-primary);
}

.admin-link.active {
  background-color: var(--nav-primary-soft);
}

.admin-link.active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 7px;
  bottom: 7px;
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
  color: var(--nav-text-faint);
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

.nav-backdrop-enter-active,
.nav-backdrop-leave-active {
  transition: opacity 0.25s ease;
}
.nav-backdrop-enter-from,
.nav-backdrop-leave-to {
  opacity: 0;
}

@media (max-width: 768px) {
  .nav-menu {
    width: 280px;
    max-width: 85vw;
    transform: translateX(-100%);
    transition: transform 0.28s cubic-bezier(0.2, 0.8, 0.2, 1);
    z-index: 10000;
  }

  .nav-menu.nav-menu--mobile-open {
    transform: translateX(0);
  }

  .nav-menu .nav-content {
    width: 100%;
    padding-top: 48px;
  }

  /* На мобильном содержимое всегда развёрнуто - hover не работает на touch */
  .nav-menu .nav-text,
  .nav-menu .section-title {
    opacity: 1 !important;
    transform: none !important;
  }

  .nav-menu .nav-item {
    width: auto;
    min-height: 48px;
  }

  .nav-menu__backdrop {
    display: block;
    position: fixed;
    inset: 0;
    background: rgba(15, 17, 41, 0.5);
    z-index: 9999;
  }

  .nav-menu__close {
    display: flex;
    align-items: center;
    justify-content: center;
  }

  /* dropdown'ы в drawer - inline (не абсолютные) на мобильном */
  .dropdown-list {
    position: static !important;
    width: 100% !important;
    border: none !important;
    margin-left: 20px !important;
  }

  /* Колонка Админки на мобильном оверлеит как самостоятельная панель поверх
     drawer'а (рельс уезжает за край), а не лепится к 50px-рельсу. */
  .admin-column {
    left: 0;
    width: 280px;
    max-width: 85vw;
    z-index: 10001;
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
