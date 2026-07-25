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
        :class="{ 'base-dropdown__text--placeholder': isEmptySelection }"
      >
        <template v-if="multiple">{{ multipleText }}</template>
        <slot
          v-else-if="selectedOption"
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

    <teleport
      to="body"
      :disabled="!teleport"
    >
      <transition name="dropdown">
        <div
          v-if="isOpen"
          ref="menu"
          class="base-dropdown__menu"
          :style="teleport ? menuStyle : null"
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
          <button
            v-if="multiple && selectedValues.length > 0"
            type="button"
            class="base-dropdown__clear"
            data-testid="base-dropdown-clear"
            @click="clearSelection"
          >
            Сбросить выбор
          </button>
          <div class="base-dropdown__options">
            <div
              v-for="option in filteredOptions"
              :key="option[valueKey]"
              class="base-dropdown__item"
              :class="{
                'base-dropdown__item--selected': isSelected(option),
                'base-dropdown__item--multi': multiple,
              }"
              @click="select(option)"
            >
              <span
                v-if="multiple"
                class="base-dropdown__check"
                :class="{ 'base-dropdown__check--on': isSelected(option) }"
                aria-hidden="true"
              />
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
    </teleport>
  </div>
</template>

<script>
import { getViewportZoom } from '@/utils/viewportScale';

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
    // Множественный выбор (#1398): modelValue - массив значений, выбор пункта тоглит
    // его и НЕ закрывает меню. Одиночная ветка (multiple=false) не меняется.
    multiple: {
      type: Boolean,
      default: false,
    },
    // Подпись кнопки при нескольких выбранных: "<summaryLabel>: N". По умолчанию
    // берётся placeholder - отдельный проп нужен, когда placeholder длинный
    // ("Все организации" -> summaryLabel "Организация").
    summaryLabel: {
      type: String,
      default: '',
    },
    // Рендерить меню в <Teleport to="body"> с position:fixed - чтобы его не
    // обрезал overflow предка (напр. прокручиваемое тело модалки).
    teleport: {
      type: Boolean,
      default: false,
    },
    // z-index телепортнутого меню. Дефолт 2000 - выше тела обычных модалок (~1000),
    // ниже глобальных диалогов (20000+). Поднять, если родитель-модалка имеет высокий
    // overlay z-index (напр. WorkModesModal 10000), иначе меню уходит ЗА неё.
    menuZIndex: {
      type: Number,
      default: 2000,
    },
  },
  emits: ['update:modelValue'],
  data() {
    return {
      isOpen: false,
      searchQuery: '',
      menuStyle: {},
    };
  },
  computed: {
    selectedOption() {
      if (this.modelValue === null || this.modelValue === undefined) return null;
      return this.options.find((opt) => opt[this.valueKey] === this.modelValue) || null;
    },
    // Множественный выбор (#1398). modelValue может прийти null/скаляром (родитель ещё
    // не инициализировал массив) - приводим к массиву, чтобы шаблон не падал.
    selectedValues() {
      return Array.isArray(this.modelValue) ? this.modelValue : [];
    },
    isEmptySelection() {
      return this.multiple ? this.selectedValues.length === 0 : !this.selectedOption;
    },
    // Пусто - placeholder; один выбранный - его имя (полезнее счётчика "1");
    // несколько - "<summaryLabel>: N". Значение без своей опции (справочник ещё
    // грузится) не должно схлопывать подпись в пустоту - показываем счётчик.
    multipleText() {
      const count = this.selectedValues.length;
      if (count === 0) return this.placeholder;
      const label = this.summaryLabel || this.placeholder;
      if (count === 1) {
        const only = this.options.find((opt) => opt[this.valueKey] === this.selectedValues[0]);
        return only ? String(only[this.labelKey]) : `${label}: 1`;
      }
      return `${label}: ${count}`;
    },
    filteredOptions() {
      if (!this.searchable || !this.searchQuery) return this.options;
      const query = this.searchQuery.toLowerCase();
      return this.options.filter((opt) =>
        String(opt[this.labelKey]).toLowerCase().includes(query)
      );
    },
  },
  watch: {
    isOpen(open) {
      if (!this.teleport) return;
      if (open) {
        this.updateMenuPosition();
        this.addRepositionListeners();
      } else {
        this.removeRepositionListeners();
      }
    },
  },
  mounted() {
    // Capture-фаза (#1132): внутри BaseModal на `.base-modal` висит @click.stop,
    // и bubble-листнер document не срабатывает -> дропдаун не закрывался по клику
    // вне себя, и открытие одного не гасило другой (два меню разом). Capture идёт
    // до stopPropagation. Логика containment та же -> вне модалки поведение не меняется.
    document.addEventListener('click', this.handleClickOutside, true);
  },
  beforeUnmount() {
    document.removeEventListener('click', this.handleClickOutside, true);
    this.removeRepositionListeners();
  },
  methods: {
    toggle() {
      if (this.disabled) return;
      this.isOpen = !this.isOpen;
      if (this.isOpen) {
        // Поле поиска НЕ фокусируем: на мобилке автофокус выбрасывает клавиатуру
        // поверх списка, и выбрать значение мышью/пальцем сразу нельзя.
        this.searchQuery = '';
      }
    },
    select(option) {
      const value = option[this.valueKey];
      if (this.multiple) {
        // Меню оставляем открытым: смысл мультивыбора - отметить несколько подряд.
        // Эмитим новый массив, не мутируя проп.
        const next = this.selectedValues.includes(value)
          ? this.selectedValues.filter((v) => v !== value)
          : [...this.selectedValues, value];
        this.$emit('update:modelValue', next);
        return;
      }
      this.$emit('update:modelValue', value);
      this.isOpen = false;
      this.searchQuery = '';
    },
    clearSelection() {
      this.$emit('update:modelValue', []);
    },
    isSelected(option) {
      if (this.multiple) return this.selectedValues.includes(option[this.valueKey]);
      return option[this.valueKey] === this.modelValue;
    },
    handleClickOutside(e) {
      const inTrigger = this.$refs.dropdown && this.$refs.dropdown.contains(e.target);
      const inMenu = this.$refs.menu && this.$refs.menu.contains(e.target);
      if (!inTrigger && !inMenu) {
        this.isOpen = false;
        this.searchQuery = '';
      }
    },
    updateMenuPosition() {
      const el = this.$refs.dropdown;
      if (!el) return;
      // Меню телепортится в body ВНУТРИ зазумленного <html> (масштаб под 1440 на
      // мониторах >1440): его inline top/left трактуются в зазумленных CSS-px и
      // домножаются на zoom. getBoundingClientRect отдаёт device-px, innerHeight -
      // НЕзумленную высоту; делим оба на zoom, чтобы считать в layout-px (при
      // zoom=1 - деление на 1, поведение не меняется). Иначе меню улетает по X/Y
      // в правый нижний угол тем дальше, чем правее триггер.
      const z = getViewportZoom();
      const raw = el.getBoundingClientRect();
      const r = {
        left: raw.left / z,
        top: raw.top / z,
        bottom: raw.bottom / z,
        width: raw.width / z,
      };
      const vh = window.innerHeight / z;
      const gap = 5;
      const margin = 8; // не впритык к краю экрана
      const spaceBelow = vh - r.bottom - gap - margin;
      const spaceAbove = r.top - gap - margin;
      // По умолчанию вниз; если снизу мало места, а сверху больше - открываем ВВЕРХ
      // (иначе высокое меню на низком вьюпорте/мобильном bottom-sheet уезжает за экран).
      const openUp = spaceBelow < 200 && spaceAbove > spaceBelow;
      const avail = Math.max(120, Math.floor(openUp ? spaceAbove : spaceBelow));
      this.menuStyle = {
        position: 'fixed',
        left: `${Math.round(r.left)}px`,
        width: `${Math.round(r.width)}px`,
        // клампим по доступному пространству выбранной стороны, чтобы меню не выходило за экран
        maxHeight: `${Math.min(320, avail)}px`,
        // top:'auto' в ветке флипа обязателен - иначе базовый CSS
        // .base-dropdown__menu{top:calc(100%+5px)} не сбрасывается и конфликтует с bottom.
        ...(openUp
          ? { bottom: `${Math.round(vh - r.top + gap)}px`, top: 'auto' }
          : { top: `${Math.round(r.bottom + gap)}px`, bottom: 'auto' }),
        // выше тела модалки, но НИЖЕ глобальных блокирующих диалогов
        // (ConfirmDialog 20000, SessionExpiredModal 25000) - см. [[z-index лестница]].
        // Настраивается пропом menuZIndex для модалок с высоким overlay z-index.
        zIndex: this.menuZIndex,
      };
    },
    addRepositionListeners() {
      window.addEventListener('scroll', this.updateMenuPosition, true);
      window.addEventListener('resize', this.updateMenuPosition);
    },
    removeRepositionListeners() {
      window.removeEventListener('scroll', this.updateMenuPosition, true);
      window.removeEventListener('resize', this.updateMenuPosition);
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
  /* flex-column, чтобы inline maxHeight (teleport-режим) ужимал прокручиваемый
     список опций, а не обрезал его через overflow:hidden. */
  display: flex;
  flex-direction: column;
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
  flex: 1 1 auto;
  min-height: 0;
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

/* Множественный выбор (#1398): чекбокс слева, подпись прижата к нему -
   space-between одиночного режима развёл бы их по краям пункта. */
.base-dropdown__item--multi {
  justify-content: flex-start;
  gap: 8px;
}

.base-dropdown__check {
  flex-shrink: 0;
  width: 15px;
  height: 15px;
  border: 1px solid #cfcfcf;
  border-radius: 4px;
  background: #fff;
  position: relative;
  transition: background-color 0.15s ease, border-color 0.15s ease;
}

.base-dropdown__check--on {
  background: #4F5BDF;
  border-color: #4F5BDF;
}

.base-dropdown__check--on::after {
  content: '';
  position: absolute;
  left: 4px;
  top: 1px;
  width: 4px;
  height: 8px;
  border: solid #fff;
  border-width: 0 2px 2px 0;
  transform: rotate(45deg);
}

.base-dropdown__clear {
  width: 100%;
  padding: 8px 15px;
  border: none;
  border-bottom: 1px solid #e6e6e6;
  background: #fff;
  color: #4F5BDF;
  font-size: 12px;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
}

.base-dropdown__clear:hover {
  background: #f5f5f5;
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
