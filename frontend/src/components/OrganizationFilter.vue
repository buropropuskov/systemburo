<template>
  <div
    class="field field--select"
    @click="toggleDropdown"
  >
    <span class="select-text">{{ displayText }}</span>
    <svg
      class="select-icon"
      :class="{ 'select-icon--rotated': isOpen }"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
    >
      <path
        d="M6 9L12 15L18 9"
        stroke="#555"
        stroke-width="2.5"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </svg>
        
    <transition name="dropdown">
      <div 
        v-if="isOpen"
        class="custom-dropdown"
        @click.stop
      >
        <div class="dropdown-search">
          <input 
            ref="searchInput" 
            v-model="searchQuery" 
            type="text"
            placeholder="Поиск..."
            class="dropdown-search__input"
            @click.stop
            @input="handleSearchInput"
          >
        </div>
                
        <div
          ref="listContainer"
          class="dropdown-list"
          :style="listMaxHeight ? { maxHeight: `${listMaxHeight}px` } : null"
        >
          <div
            class="dropdown-item"
            :class="{ 'dropdown-item--selected': !internalValue }"
            @click="selectItem(null, allLabel)"
          >
            <span
              class="item-text"
              :title="allLabel"
            >{{ allLabel }}</span>
          </div>
                    
          <div 
            v-for="org in filteredOrganizations"
            :key="org.id"
            class="dropdown-item"
            :class="{ 'dropdown-item--selected': internalValue === org.id }"
            @click="selectItem(org.id, org.name)"
          >
            <span
              class="item-text"
              :title="org.name"
            >{{ org.name }}</span>
          </div>
                    
          <div 
            v-if="filteredOrganizations.length === 0" 
            class="dropdown-no-results"
            :class="{ 'with-border': searchQuery }"
          >
            Ничего не найдено
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<script>
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import { wholeItemsHeight, measureChromeHeight } from '@/utils/dropdownMetrics';

// Запас под список, если у всплывающего блока не задана max-height.
const LIST_MAX_SPACE = 320;

export default {
    name: 'OrganizationFilter',
    props: {
        value: {
            type: [Number, String],
            default: null
        },
        organizations: {
            type: Array,
            default: () => []
        },
        // Add-only пропсы, чтобы переиспользовать фильтр под другую сущность
        // (например "Компания"), не плодя копию компонента. Дефолты сохраняют
        // прежнее поведение фильтра организаций.
        allLabel: {
            type: String,
            default: 'Все организации'
        },
        placeholderText: {
            type: String,
            default: 'Организация'
        }
    },
    emits: ['change', 'input'],
    data() {
        return {
            isOpen: false,
            searchQuery: '',
            internalValue: null,
            selectedName: '',
            clickOutsideHandler: null,
            // Высота списка, кратная высоте пункта: иначе последний виден наполовину.
            listMaxHeight: null
        };
    },
    computed: {
        displayText() {
            if (this.selectedName && this.selectedName.trim()) return this.selectedName;
            return this.placeholderText;
        },
        
        filteredOrganizations() {
            const variants = buildSearchVariants(this.searchQuery);
            if (!variants.length) return this.organizations;

            return this.organizations.filter(org => {
                return matchesSearch(org.name, variants);
            });
        }
    },
    watch: {
        value: {
            immediate: true,
            handler(newValue) {
                this.internalValue = newValue;
                this.updateSelectedName();
            }
        },
        
        organizations: {
            immediate: true,
            handler() {
                this.updateSelectedName();
            }
        },
        
        isOpen(newValue) {
            if (newValue) {
                // Автофокус на поле поиска убран: на мобилке он выбрасывает клавиатуру
                // поверх списка вариантов - выбрать значение сразу нельзя.
                this.$nextTick(() => {
                    this.setupClickOutside();
                });
            } else {
                this.searchQuery = '';
                this.removeClickOutside();
            }
        }
    },
    mounted() {
        this.updateSelectedName();
    },
    beforeUnmount() {
        this.removeClickOutside();
    },
    methods: {
        toggleDropdown() {
            this.isOpen = !this.isOpen;
            if (this.isOpen) {
                this.$nextTick(this.updateListHeight);
            } else {
                this.listMaxHeight = null;
            }
        },

        /**
         * Ограничивает список целым числом пунктов - иначе последний обрывается по
         * середине строки. Доступное место считаем из ограничения всплывающего блока
         * за вычетом строки поиска, ничего не сбрасывая: сброс высоты разворачивал бы
         * список во весь рост и схлопывал обратно на каждом пересчёте.
         */
        updateListHeight() {
            if (!this.isOpen) return;
            const box = this.$refs.listContainer;
            if (!box) return;
            const items = Array.from(box.querySelectorAll('.dropdown-item'));
            const wrap = box.parentElement;
            const wrapLimit = wrap ? parseFloat(getComputedStyle(wrap).maxHeight) : NaN;
            const limit = Number.isNaN(wrapLimit) ? LIST_MAX_SPACE : wrapLimit;
            const available = Math.max(0, limit - measureChromeHeight(wrap, box));
            this.listMaxHeight = wholeItemsHeight(box, items, available);
        },

        setupClickOutside() {
            this.clickOutsideHandler = (e) => {
                if (!this.$el.contains(e.target)) {
                    this.isOpen = false;
                }
            };
            setTimeout(() => {
                document.addEventListener('click', this.clickOutsideHandler);
            }, 0);
        },
        
        removeClickOutside() {
            if (this.clickOutsideHandler) {
                document.removeEventListener('click', this.clickOutsideHandler);
                this.clickOutsideHandler = null;
            }
        },
        
        handleSearchInput() {
            this.$nextTick(() => {
                if (this.$refs.listContainer) {
                    this.$refs.listContainer.scrollTop = 0;
                }
                // Пока поиск ничего не нашёл, мерить нечего - пересчитываем, когда
                // пункты вернулись.
                this.updateListHeight();
            });
        },
        
        selectItem(id, name) {
            this.internalValue = id;
            this.selectedName = name;
            this.$emit('input', id);
            this.$emit('change', { id, name });
            this.isOpen = false;
        },
        
        updateSelectedName() {
            if (!this.internalValue) {
                this.selectedName = '';
                return;
            }
            
            const selectedOrg = this.organizations.find(org => org.id === this.internalValue);
            if (selectedOrg) {
                this.selectedName = selectedOrg.name;
            } else {
                this.selectedName = '';
            }
        },
        
        
        // Метод для сброса фильтра из родительского компонента
        reset() {
            this.internalValue = null;
            this.selectedName = '';
            this.$emit('input', null);
            this.$emit('change', { id: null, name: '' });
        }
    }
};
</script>

<style scoped>
.field--select {
    cursor: pointer;
    position: relative;
    width: 230px; /* Фиксированная ширина */
    min-width: 230px; /* Минимальная ширина */
}

.select-text {
    font-size: 13px;
    color: var(--text);
    flex: 1;
    /* Обрезаем текст с многоточием в основном поле */
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: calc(100% - 20px); /* Оставляем место для стрелки */
}

.select-icon {
    width: 9px;
    height: 9px;
    transition: transform 0.3s ease;
    flex-shrink: 0;
}

.select-icon--rotated {
    transform: rotate(180deg);
}

/* Анимации для выпадающего меню */
.dropdown-enter-active,
.dropdown-leave-active {
    transition: all 0.2s ease;
    transform-origin: top center;
}

.dropdown-enter-from,
.dropdown-leave-to {
    opacity: 0;
    transform: scale(0.95) translateY(-10px);
}

.dropdown-enter-to,
.dropdown-leave-from {
    opacity: 1;
    transform: scale(1) translateY(0);
}

.custom-dropdown {
    position: absolute;
    top: calc(100% + 5px);
    left: 0;
    width: 230px; /* Такая же ширина как у поля */
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    max-height: 360px;
    overflow: hidden;
    z-index: 1001;
    box-shadow: 0 4px 12px var(--shadow-drop);
    display: flex;
    flex-direction: column;
}

.dropdown-search {
    padding: 10px;
    border-bottom: 1px solid var(--border);
    position: sticky;
    top: 0;
    background: var(--surface);
    z-index: 1002;
}

.dropdown-search__input {
    width: 100%;
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    font-size: 14px;
    outline: none;
    transition: border-color 0.2s;
    background: var(--surface-2);
}

.dropdown-search__input:focus {
    border-color: var(--accent);
    box-shadow: 0 0 0 2px rgba(79, 91, 223, 0.1);
    background: var(--surface);
}

.dropdown-list {
    max-height: 320px;
    overflow-y: auto;
    flex: 1;
}

.dropdown-item {
    padding: 10px 12px;
    cursor: pointer;
    font-size: 13px;
    border-bottom: 1px solid transparent;
    transition: all 0.2s ease;
    display: flex;
    align-items: center;
    min-height: 36px;
    position: relative;
    overflow: hidden; /* Важно для обрезания текста */
}

.dropdown-item:hover {
    background-color: var(--accent-tint);
}

/* Обрезаем текст в элементах списка */
.item-text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
    min-width: 0; /* Важно для работы text-overflow в flex-контейнере */
    position: relative;
}

/* Выбранный пункт: подложка и текст от акцента темы - светлые литералы в тёмной
   теме давали светлое пятно в списке. */
.dropdown-item--selected {
    background-color: var(--accent-tint) !important;
    font-weight: 500;
}

.dropdown-item--selected .item-text {
    color: var(--accent-text) !important;
}

.dropdown-item--selected:hover {
    background-color: color-mix(in srgb, var(--accent) 18%, var(--surface)) !important;
}

/* Убираем нижнюю границу у последнего элемента, если это не "Ничего не найдено" */
.dropdown-item:not(:last-child) {
    border-bottom: 1px solid var(--border);
}

.dropdown-item:last-child:not(.dropdown-no-results) {
    border-bottom: none;
}

.dropdown-no-results {
    padding: 16px 15px;
    text-align: center;
    color: var(--text-muted);
    font-size: 14px;
    border-top: 1px solid var(--border);
    margin: 4px 2px 2px 2px;
    border-radius: 6px;
    background: var(--surface-2);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

/* Прокрутка */
.dropdown-list::-webkit-scrollbar {
    width: 6px;
}

.dropdown-list::-webkit-scrollbar-track {
    background: var(--surface-2);
    border-radius: 3px;
    margin: 4px 0;
}

.dropdown-list::-webkit-scrollbar-thumb {
    background: var(--border);
    border-radius: 3px;
}

.dropdown-list::-webkit-scrollbar-thumb:hover {
    background: var(--text-muted);
}

/* Тулатип для полного названия при наведении */
.item-text:hover::after {
    content: attr(title);
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    background: var(--hint-bg);
    color: var(--hint-text);
    padding: 6px 10px;
    border-radius: 4px;
    font-size: 12px;
    white-space: nowrap;
    z-index: 1000;
    pointer-events: none;
    opacity: 0;
    transition: opacity 0.2s;
    margin-bottom: 5px;
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
}

.item-text:hover::before {
    content: '';
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    border: 5px solid transparent;
    border-top-color: var(--hint-bg);
    z-index: 1000;
    pointer-events: none;
    opacity: 0;
    transition: opacity 0.2s;
    margin-bottom: -5px;
}

.item-text:hover::after,
.item-text:hover::before {
    opacity: 1;
}

/* Адаптивность */
@media (max-width: 768px) {
    .field--select {
        width: 100%;
        min-width: auto;
    }
    
    .custom-dropdown {
        position: fixed;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        width: 90vw;
        max-width: 400px;
        max-height: 80vh;
    }
    
    .dropdown-list {
        max-height: 60vh;
    }
    
    .item-text {
        max-width: 100%;
    }
}
</style>