<template>
  <div
    ref="dropdown"
    class="base-dropdown"
  >
    <button
      type="button"
      class="base-dropdown__button"
      :class="{ 'base-dropdown__button--open': isOpen, 'base-dropdown__button--disabled': disabled }"
      :disabled="disabled"
      @click="toggle"
    >
      <span
        class="base-dropdown__text"
        :class="{ 'base-dropdown__text--placeholder': !selectedOption }"
      >
        <slot
          v-if="selectedOption"
          name="selected"
          :option="selectedOption"
        >
          {{ selectedOption[labelKey] }}
        </slot>
        <template v-else>{{ placeholder }}</template>
      </span>
      <svg
        class="base-dropdown__arrow"
        :class="{ 'base-dropdown__arrow--open': isOpen }"
        viewBox="0 0 10 6"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          d="M1 1L5 5L9 1"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      </svg>
    </button>

    <transition name="dropdown">
      <div
        v-if="isOpen"
        class="base-dropdown__menu"
      >
        <div
          v-if="searchable"
          class="base-dropdown__search"
        >
          <input
            ref="searchInput"
            v-model="searchQuery"
            type="text"
            class="base-dropdown__search-input"
            placeholder="Поиск..."
            @click.stop
          >
        </div>
        <div class="base-dropdown__options">
          <div
            v-for="option in filteredOptions"
            :key="option[valueKey]"
            class="base-dropdown__item"
            :class="{ 'base-dropdown__item--selected': isSelected(option) }"
            @click="select(option)"
          >
            <slot
              name="option"
              :option="option"
            >
              <span class="base-dropdown__item-text">{{ option[labelKey] }}</span>
            </slot>
          </div>
          <div
            v-if="filteredOptions.length === 0"
            class="base-dropdown__empty"
          >
            Ничего не найдено
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<script>
export default {
  name: 'BaseDropdown',
  props: {
    modelValue: {
      type: [String, Number, Object, Array, Boolean],
      default: null,
    },
    options: {
      type: Array,
      required: true,
    },
    labelKey: {
      type: String,
      default: 'name',
    },
    valueKey: {
      type: String,
      default: 'id',
    },
    placeholder: {
      type: String,
      default: 'Выберите...',
    },
    disabled: {
      type: Boolean,
      default: false,
    },
    searchable: {
      type: Boolean,
      default: false,
    },
  },
  emits: ['update:modelValue'],
  data() {
    return {
      isOpen: false,
      searchQuery: '',
    };
  },
  computed: {
    selectedOption() {
      if (this.modelValue === null || this.modelValue === undefined) return null;
      return this.options.find((opt) => opt[this.valueKey] === this.modelValue) || null;
    },
    filteredOptions() {
      if (!this.searchable || !this.searchQuery) return this.options;
      const query = this.searchQuery.toLowerCase();
      return this.options.filter((opt) =>
        String(opt[this.labelKey]).toLowerCase().includes(query)
      );
    },
  },
  mounted() {
    document.addEventListener('click', this.handleClickOutside);
  },
  beforeUnmount() {
    document.removeEventListener('click', this.handleClickOutside);
  },
  methods: {
    toggle() {
      if (this.disabled) return;
      this.isOpen = !this.isOpen;
      if (this.isOpen) {
        this.searchQuery = '';
        this.$nextTick(() => {
          if (this.searchable && this.$refs.searchInput) {
            this.$refs.searchInput.focus();
          }
        });
      }
    },
    select(option) {
      this.$emit('update:modelValue', option[this.valueKey]);
      this.isOpen = false;
      this.searchQuery = '';
    },
    isSelected(option) {
      return option[this.valueKey] === this.modelValue;
    },
    handleClickOutside(e) {
      if (this.$refs.dropdown && !this.$refs.dropdown.contains(e.target)) {
        this.isOpen = false;
        this.searchQuery = '';
      }
    },
  },
};
</script>

<style scoped>
.base-dropdown {
  position: relative;
}

.base-dropdown__button {
  width: 100%;
  min-height: 30px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 0 15px;
  border: 1px solid #e6e6e6;
  background-color: #fff;
  border-radius: 50px;
  outline: none;
  cursor: pointer;
  transition: border-color 0.2s;
}

.base-dropdown__button:hover:not(:disabled) {
  border-color: #4F5BDF;
}

.base-dropdown__button--disabled {
  background-color: #f5f5f5;
  cursor: not-allowed;
  opacity: 0.6;
}

.base-dropdown__text {
  font-size: 14px;
  color: #000;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.base-dropdown__text--placeholder {
  color: #a2a2a2;
  font-weight: 400;
}

.base-dropdown__arrow {
  width: 10px;
  height: 10px;
  flex-shrink: 0;
  color: #666;
  transition: transform 0.2s;
}

.base-dropdown__arrow--open {
  transform: rotate(180deg);
}

.base-dropdown__menu {
  position: absolute;
  top: calc(100% + 5px);
  left: 0;
  width: 100%;
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 20px;
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.1);
  z-index: 1000;
  overflow: hidden;
}

.base-dropdown__search {
  padding: 8px 12px;
  border-bottom: 1px solid #e6e6e6;
}

.base-dropdown__search-input {
  width: 100%;
  padding: 6px 12px;
  border: 1px solid #e6e6e6;
  border-radius: var(--radius-md);
  font-size: 13px;
  outline: none;
  transition: border-color 0.2s;
  box-sizing: border-box;
}

.base-dropdown__search-input:focus {
  border-color: #4F5BDF;
}

.base-dropdown__options {
  max-height: 250px;
  overflow-y: auto;
}

.base-dropdown__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 15px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.base-dropdown__item:hover {
  background-color: #f5f5f5;
}

.base-dropdown__item--selected {
  background-color: #f0f1ff;
  color: #4F5BDF;
}

.base-dropdown__item--selected .base-dropdown__item-text {
  color: #4F5BDF;
}

.base-dropdown__item:first-child {
  border-radius: 10px 10px 0 0;
}

.base-dropdown__item:last-child {
  border-radius: 0 0 10px 10px;
}

.base-dropdown__item-text {
  font-size: 13px;
  color: #333;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.base-dropdown__empty {
  padding: 12px 15px;
  font-size: 13px;
  color: #a2a2a2;
  text-align: center;
}

/* Transition */
.dropdown-enter-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}
.dropdown-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
