<template>
  <div 
    class="nav-menu" 
    :class="{ expanded: isExpanded }"
    @mouseenter="expandMenu"
    @mouseleave="collapseMenu"
  >
    <!-- Внутренний контейнер для контента -->
    <div class="nav-content">
      <!-- ЗАЯВКИ -->
      <div class="nav-section">
        <div class="section-title">ЗАЯВКИ</div>
        <div class="nav-item">
          <img src="@/assets/icons/envelope.png" alt="Центр заявок" class="nav-icon">
          <span class="nav-text">Центр заявок</span>
        </div>
        <div class="nav-item">
          <img src="@/assets/icons/archive.png" alt="Архив" class="nav-icon">
          <span class="nav-text">Архив</span>
        </div>
      </div>

      <!-- УПРАВЛЕНИЕ ДАННЫМИ -->
      <div class="nav-section">
        <div class="section-title">УПРАВЛЕНИЕ ДАННЫМИ</div>
        
        <!-- Элемент с выпадающим списком таблиц -->
        <div class="nav-item-container">
          <div 
            class="nav-item has-dropdown" 
            @mouseenter="openDropdown('tables')"
            @mouseleave="handleDropdownLeave('tables')"
          >
            <div class="nav-item-content">
              <img src="@/assets/icons/table.png" alt="Таблицы" class="nav-icon">
              <span class="nav-text">Таблицы</span>
            </div>
            <img src="@/assets/icons/arrow.png" alt="▼" class="dropdown-arrow" :class="{ rotated: dropdowns.tables }">
          </div>
          
          <!-- Выпадающий список таблиц (справа от меню) -->
          <transition name="dropdown-fade">
            <div 
              v-show="dropdowns.tables" 
              class="dropdown-list dropdown-right"
              @mouseenter="keepDropdownOpen('tables')"
              @mouseleave="closeDropdown('tables')"
            >
              <div class="dropdown-item" @click="navigateToTable">КПП №4</div>
              <div class="dropdown-item disabled">СЕВЕР + 72 ПОСТ</div>
              <div class="dropdown-item disabled">ПОСТ №21</div>
              <div class="dropdown-item disabled">ПОСТ №27</div>
              <div class="dropdown-item disabled">DREAM BEACH CLUB</div>
              <div class="dropdown-item disabled">КИНО-КОНЦЕРТНЫЙ ЗАЛ</div>
              <div class="dropdown-item disabled">СТРОЙКА</div>
              <div class="dropdown-item disabled">СВОДНЫЕ ДАННЫЕ</div>
            </div>
          </transition>
        </div>

        <!-- Элемент с выпадающим списком сотрудников -->
        <div class="nav-item-container">
          <div 
            class="nav-item has-dropdown" 
            @mouseenter="openDropdown('employees')"
            @mouseleave="handleDropdownLeave('employees')"
          >
            <div class="nav-item-content">
              <img src="@/assets/icons/employees.png" alt="Сотрудники" class="nav-icon">
              <span class="nav-text">Сотрудники</span>
            </div>
            <img src="@/assets/icons/arrow.png" alt="▼" class="dropdown-arrow" :class="{ rotated: dropdowns.employees }">
          </div>
          
          <!-- Выпадающий список сотрудников (справа от меню) -->
          <transition name="dropdown-fade">
            <div 
              v-show="dropdowns.employees" 
              class="dropdown-list dropdown-right"
              @mouseenter="keepDropdownOpen('employees')"
              @mouseleave="closeDropdown('employees')"
            >
              <div class="dropdown-item disabled">Арендаторы</div>
              <div class="dropdown-item disabled">Подрядчики</div>
              <div class="dropdown-item disabled">Сотрудники компании</div>
            </div>
          </transition>
        </div>
        <div class="nav-item" @click="navigateToCarsView">
          <img src="@/assets/icons/car.png" alt="Архив" class="nav-icon">
          <span class="nav-text">Автомобили</span>
        </div>
      </div>

      <!-- АНАЛИТИКА -->
      <div class="nav-section">
        <div class="section-title">АНАЛИТИКА</div>
        <div class="nav-item">
          <img src="@/assets/icons/stats.png" alt="Статистика" class="nav-icon">
          <span class="nav-text">Статистика</span>
        </div>
        <div class="nav-item">
          <img src="@/assets/icons/clipboard.png" alt="Отчёты" class="nav-icon">
          <span class="nav-text">Отчёты</span>
        </div>
      </div>

      <!-- ПОЛЬЗОВАТЕЛЬ (в нижней части) -->
      <div class="nav-section user-section">
        <div class="section-title">ПОЛЬЗОВАТЕЛЬ</div>
        <div class="nav-item">
          <img src="@/assets/icons/newspaper.png" alt="Новости" class="nav-icon">
          <span class="nav-text">Обзор и новости</span>
        </div>
        <div class="nav-item" @click="navigateToAccount">
          <img src="@/assets/icons/user.png" alt="Личный кабинет" class="nav-icon">
          <span class="nav-text">Личный кабинет</span>
        </div>
        <div class="nav-item" @click="logout">
          <img src="@/assets/icons/logout.png" alt="Выйти" class="nav-icon">
          <span class="nav-text">Выйти</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'NavMenu',
  data() {
    return {
      isExpanded: false,
      dropdowns: {
        tables: false,
        employees: false
      },
      hoverTimeout: null,
      dropdownTimeout: null,
      dropdownLeaveTimeout: null
    };
  },
  methods: {
    expandMenu() {
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
    navigateToTable() {
      this.$router.push('/table');
      this.closeAllDropdowns();
    },
    navigateToAccount() {
      this.$router.push('/personal-cabinet');
    },
    navigateToCarsView() {
      this.$router.push('/carsview');
    },
    
    logout() {
      this.$emit('logout');
    }
  },
  mounted() {
    document.body.classList.add('auth-active');
  },
  beforeUnmount() {
    document.body.classList.remove('auth-active');
    if (this.hoverTimeout) {
      clearTimeout(this.hoverTimeout);
    }
    if (this.dropdownTimeout) {
      clearTimeout(this.dropdownTimeout);
    }
    if (this.dropdownLeaveTimeout) {
      clearTimeout(this.dropdownLeaveTimeout);
    }
  }
};
</script>

<style scoped>
.nav-menu {
  position: fixed;
  left: 0;
  top: 0;
  width: 50px;
  height: 100vh;
  background: #fafafa;
  border-right: 1px solid #e6e6e6;
  z-index: 1000;
  transition: width 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  display: flex;
}

.nav-content {
  width: 188px;
  display: flex;
  flex-direction: column;
  position: relative;
}

.nav-menu.expanded {
  width: 188px;
  overflow: visible;
}

.user-section {
  margin-top: auto;
  margin-bottom: 20px;
}

.section-title {
  font-family: 'Montserrat', sans-serif;
  font-style: normal;
  font-weight: 700;
  font-size: 10px;
  line-height: 10px;
  color: #A2A2A2;
  text-transform: uppercase;
  margin: 15px 0 8px 19px;
  height: 10px;
  overflow: hidden;
  white-space: nowrap;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  opacity: 0;
  transform: translateX(-10px);
}

.nav-menu.expanded .section-title {
  opacity: 1;
  transform: translateX(0);
  transition-delay: 0.1s;
}

.nav-item-container {
  position: relative;
}

.nav-item {
  display: flex;
  align-items: center;
  padding: 10px 15px;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  min-height: 35px;
  justify-content: flex-start;
  width: 187px;
}

.nav-item.has-dropdown {
  justify-content: space-between;
}

.nav-item-content {
  display: flex;
  align-items: center;
  min-width: 0;
}

.nav-item:hover {
  background-color: #f8f9fa;
}

.nav-icon {
  width: 16px;
  height: 16px;
  min-width: 16px;
  margin-right: 12px;
  flex-shrink: 0;
  transition: transform 0.2s ease;
}


.nav-text {
  font-family: 'Montserrat', sans-serif;
  font-style: normal;
  font-weight: 500;
  font-size: 13px;
  line-height: 16px;
  color: #000;
  white-space: nowrap;
  overflow: hidden;
  opacity: 0;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  transform: translateX(-5px);
  flex-shrink: 0;
}

.nav-menu.expanded .nav-text {
  opacity: 1;
  transform: translateX(0);
  transition-delay: 0.15s;
}

/* Стрелки для выпадающих списков */
.dropdown-arrow {
  width: 6px;
  height: 6px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  transform: rotate(0deg);
  flex-shrink: 0;
  opacity: 0;
}

.dropdown-arrow.rotated {
  transform: rotate(90deg);
}

.nav-menu.expanded .dropdown-arrow {
  opacity: 0.6;
  transition-delay: 0.2s;
}

.nav-item:hover .dropdown-arrow {
  opacity: 1;
}

/* Выпадающие списки (справа от меню) */
.dropdown-list {
  position: absolute;
  left: 188px;
  top: 0;
  background: #fafafa;
  border-radius: 0 8px 8px 0;
  z-index: 1001;
  border: 1px solid #e6e6e6;
  border-left: none;
  min-width: 220px;
  transform-origin: left center;
}

.dropdown-item {
  padding: 8px 16px;
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  color: #000;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  border-left: 3px solid transparent;
}

.dropdown-item:hover {
  background-color: #f8f9fa;
}

.disabled {
     color: #95a5a6;
  cursor: not-allowed;
}

.dropdown-item.disabled {
  color: #95a5a6;
  cursor: not-allowed;
}

.dropdown-item.disabled:hover {
  background-color: transparent;
  border-left-color: transparent;
  padding-left: 16px;
}

/* Анимации для выпадающих списков */
.dropdown-fade-enter-active,
.dropdown-fade-leave-active {
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.dropdown-fade-enter-from,
.dropdown-fade-leave-to {
  opacity: 0;
  transform: translateX(-10px) scale(0.95);
}

/* Гарантируем, что все элементы сохраняют свои размеры */
.nav-menu * {
  box-sizing: border-box;
}

/* Фиксируем высоту элементов чтобы не прыгали */
.nav-item {
  flex-shrink: 0;
}

/* Плавная анимация для всего контента */
.nav-content > * {
  transition: opacity 0.3s ease;
}

/* Улучшенная анимация для hover эффектов */
.nav-item {
  position: relative;
  overflow: hidden;
}
</style>