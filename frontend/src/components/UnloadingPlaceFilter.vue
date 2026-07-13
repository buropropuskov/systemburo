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
        >
          <div 
            class="dropdown-item"
            :class="{ 'dropdown-item--selected': !internalValue }"
            @click="selectItem(null, 'Все места разгрузки')"
          >
            <span
              class="item-text"
              title="Все места разгрузки"
            >Все места разгрузки</span>
          </div>
                    
          <div 
            v-for="place in filteredPlaces"
            :key="place.id"
            class="dropdown-item"
            :class="{ 'dropdown-item--selected': internalValue === place.id }"
            @click="selectItem(place.id, place.name)"
          >
            <span
              class="item-text"
              :title="place.name"
            >{{ place.name }}</span>
          </div>
                    
          <div 
            v-if="filteredPlaces.length === 0" 
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
import { apiRequest } from '@/api/client'
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
export default {
    name: 'UnloadingPlaceFilter',
    props: {
        value: {
            type: [Number, String],
            default: null
        }
    },
    emits: ['change', 'input'],
    data() {
        return {
            isOpen: false,
            searchQuery: '',
            internalValue: null,
            selectedName: '',
            unloadingPlaces: [],
            clickOutsideHandler: null,
            isLoading: false
        };
    },
    computed: {
        displayText() {
            if (this.selectedName && this.selectedName.trim()) return this.selectedName;
            return 'Место разгрузки';
        },
        
        filteredPlaces() {
            const variants = buildSearchVariants(this.searchQuery);
            if (!variants.length) return this.unloadingPlaces;

            return this.unloadingPlaces.filter(place => {
                const haystack = `${place.name} ${place.description || ''}`;
                return matchesSearch(haystack, variants);
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
        
        isOpen(newValue) {
            if (newValue) {
                this.$nextTick(() => {
                    this.focusSearchInput();
                    this.setupClickOutside();
                });
            } else {
                this.searchQuery = '';
                this.removeClickOutside();
            }
        }
    },
    mounted() {
        this.fetchUnloadingPlaces();
        this.updateSelectedName();
    },
    beforeUnmount() {
        this.removeClickOutside();
    },
    methods: {
        async fetchUnloadingPlaces() {
            this.isLoading = true;
            try {
                const response = await apiRequest("/unload-places", {
                    method: "GET",
                });

                if (response.ok) {
                    const data = await response.json();
                    this.unloadingPlaces = data;
                    this.updateSelectedName();
                } else {
                    console.error("Ошибка при загрузке мест разгрузки");
                    this.unloadingPlaces = [];
                }
            } catch (error) {
                console.error("Ошибка сети при загрузке мест разгрузки:", error);
                this.unloadingPlaces = [];
            } finally {
                this.isLoading = false;
            }
        },
        
        toggleDropdown() {
            this.isOpen = !this.isOpen;
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
            
            const selectedPlace = this.unloadingPlaces.find(place => place.id === this.internalValue);
            if (selectedPlace) {
                this.selectedName = selectedPlace.name;
            } else {
                this.selectedName = '';
            }
        },
        
        focusSearchInput() {
            if (this.$refs.searchInput) {
                this.$refs.searchInput.focus();
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
    width: 200px;
    min-width: 200px;
}

.select-text {
    font-size: 13px;
    color: #000;
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: calc(100% - 20px);
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
    width: 200px;
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    max-height: 355px;
    overflow: hidden;
    z-index: 1001;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    display: flex;
    flex-direction: column;
}

.dropdown-search {
    padding: 10px;
    border-bottom: 1px solid #f0f0f0;
    position: sticky;
    top: 0;
    background: white;
    z-index: 1002;
}

.dropdown-search__input {
    width: 100%;
    padding: 8px 12px;
    border: 1px solid #e6e6e6;
    border-radius: var(--radius-md);
    font-size: 14px;
    outline: none;
    transition: border-color 0.2s;
    background: #fafafa;
}

.dropdown-search__input:focus {
    border-color: #4F5BDF;
    box-shadow: 0 0 0 2px rgba(79, 91, 223, 0.1);
    background: white;
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
    overflow: hidden;
}

.dropdown-item:hover {
    background-color: #f8f9ff;
}

.item-text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
    min-width: 0;
    position: relative;
}

.dropdown-item--selected {
    background-color: #f0f2ff !important;
    font-weight: 500;
}

.dropdown-item--selected .item-text {
    color: #4F5BDF !important;
}

.dropdown-item--selected:hover {
    background-color: #e8ebff !important;
}

.dropdown-item:not(:last-child) {
    border-bottom: 1px solid #f0f0f0;
}

.dropdown-item:last-child:not(.dropdown-no-results) {
    border-bottom: none;
}

.dropdown-no-results {
    padding: 16px 15px;
    text-align: center;
    color: #999;
    font-size: 14px;
    border-top: 1px solid #f0f0f0;
    margin: 4px 2px 2px 2px;
    border-radius: 6px;
    background: #fafafa;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.dropdown-list::-webkit-scrollbar {
    width: 6px;
}

.dropdown-list::-webkit-scrollbar-track {
    background: #f5f5f5;
    border-radius: 3px;
    margin: 4px 0;
}

.dropdown-list::-webkit-scrollbar-thumb {
    background: #c5c5c5;
    border-radius: 3px;
}

.dropdown-list::-webkit-scrollbar-thumb:hover {
    background: #a8a8a8;
}

.item-text:hover::after {
    content: attr(title);
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    background: #333;
    color: white;
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
    border-top-color: #333;
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