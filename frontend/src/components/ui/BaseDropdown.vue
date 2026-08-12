<template>
  <div
    ref="dropdown"
    class="base-dropdown"
  >
    <!-- Слот триггера: по умолчанию - штатная кнопка-поле. Заменяется, когда
         открывающий элемент уже существует в своём виде (кнопка «Обучение»
         в шапке «Обзора»), чтобы не заводить ради него второй дропдаун. -->
    <slot
      name="trigger"
      :toggle="toggle"
      :is-open="isOpen"
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
    </slot>

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
          <!-- Строка сброса присутствует всё время, пока меню открыто: если
               показывать её только при непустом выборе, первый же клик по пункту
               вставляет её в поток и сдвигает список вниз прямо под курсором. -->
          <button
            v-if="multiple"
            type="button"
            class="base-dropdown__clear"
            :class="{ 'base-dropdown__clear--disabled': selectedValues.length === 0 }"
            :disabled="selectedValues.length === 0"
            data-testid="base-dropdown-clear"
            @click="clearSelection"
          >
            {{ selectedValues.length > 0 ? `Сбросить выбор (${selectedValues.length})` : 'Ничего не выбрано' }}
          </button>
          <div
            ref="options"
            class="base-dropdown__options"
            :style="optionsStyle"
          >
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
import { wholeItemsHeight, measureChromeHeight } from '@/utils/dropdownMetrics';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';

// Минимум пунктов в списке: даже в тесном месте должно быть видно, что он прокручивается.
const MIN_VISIBLE_OPTIONS = 2;
// Запас под список в обычном (не телепортнутом) режиме - прежняя max-height из стилей.
const DEFAULT_OPTIONS_SPACE = 250;

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
    // Поля опции, по которым идёт поиск. Пусто - только labelKey. Список нужен там,
    // где значимый текст лежит вне подписи: у мест разгрузки код площадки живёт в
    // description, и поиск по нему работал до переезда на этот дропдаун.
    searchKeys: {
      type: Array,
      default: () => [],
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
    // Минимальная ширина телепортнутого меню. По умолчанию меню повторяет ширину
    // триггера; когда триггер узкий, а пункты содержательные (название + описание),
    // ширину задаём отдельно - и тогда же прижимаем меню к правому краю экрана,
    // чтобы уширение не вынесло его за вьюпорт.
    menuMinWidth: {
      type: Number,
      default: 0,
    },
    // Потолок высоты телепортнутого меню. 320 хватает справочникам с одной строкой
    // в пункте; списку, где у пункта ещё и описание, - нет, и он уезжает в скролл
    // при пяти элементах. Всё равно клампится по вьюпорту, так что поднять безопасно.
    menuMaxHeight: {
      type: Number,
      default: 320,
    },
  },
  emits: ['update:modelValue'],
  data() {
    return {
      isOpen: false,
      searchQuery: '',
      menuStyle: {},
      // Высота списка опций, кратная высоте пункта: иначе список обрывается на
      // половине строки и непонятно, есть ли под ней ещё варианты.
      optionsMaxHeight: null,
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
    searchFields() {
      return this.searchKeys.length ? this.searchKeys : [this.labelKey];
    },
    // Поиск через общий util (#1157), а не плоское вхождение подстроки: иначе
    // забытая раскладка ("hjvfirf") и транслит ("romashka") не находят "Ромашка".
    // Плоское совпадение остаётся подмножеством - base всегда входит в варианты.
    filteredOptions() {
      if (!this.searchable || !this.searchQuery) return this.options;
      const variants = buildSearchVariants(this.searchQuery);
      if (!variants.length) return this.options;
      return this.options.filter((opt) => matchesSearch(this.optionHaystack(opt), variants));
    },
    optionsStyle() {
      return this.optionsMaxHeight ? { maxHeight: `${this.optionsMaxHeight}px` } : null;
    },
  },
  watch: {
    isOpen(open) {
      // Слушатель Escape живёт ровно столько, сколько открыто меню: висеть постоянно
      // ему незачем, дропдаунов на странице бывает десяток.
      if (open) {
        document.addEventListener('keydown', this.handleEscape, true);
      } else {
        document.removeEventListener('keydown', this.handleEscape, true);
      }
      if (!this.teleport) return;
      if (open) {
        this.updateMenuPosition();
        this.addRepositionListeners();
      } else {
        this.removeRepositionListeners();
      }
    },
    // Поиск меняет длину списка: пока пунктов не было («Ничего не найдено»),
    // высоту пункта измерить нечем - пересчитываем, когда они появились снова.
    'filteredOptions.length': function filteredOptionsLength() {
      if (this.isOpen) this.$nextTick(this.updateOptionsHeight);
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
    document.removeEventListener('keydown', this.handleEscape, true);
    this.removeRepositionListeners();
  },
  methods: {
    optionHaystack(option) {
      return this.searchFields
        .map((key) => option[key])
        .filter((value) => value !== null && value !== undefined && value !== '')
        .join(' ');
    },
    toggle() {
      if (this.disabled) return;
      this.isOpen = !this.isOpen;
      if (this.isOpen) {
        // Поле поиска НЕ фокусируем: на мобилке автофокус выбрасывает клавиатуру
        // поверх списка, и выбрать значение мышью/пальцем сразу нельзя.
        this.searchQuery = '';
        this.$nextTick(this.updateOptionsHeight);
      } else {
        this.optionsMaxHeight = null;
      }
    },

    /**
     * Ограничивает список целым числом пунктов, чтобы он не обрывался на середине строки.
     *
     * Считает доступное место из ограничений меню, ничего предварительно не сбрасывая:
     * пересчёт зовётся в том числе на каждое событие прокрутки, и сброс высоты
     * разворачивал бы список во весь рост с последующим схлопыванием - меню мигало
     * и прыгало под курсором.
     */
    updateOptionsHeight() {
      if (!this.isOpen) return;
      const box = this.$refs.options;
      if (!box) return;
      const items = Array.from(box.querySelectorAll('.base-dropdown__item'));
      const menuLimit = this.teleport
        ? parseFloat(this.menuStyle.maxHeight) || DEFAULT_OPTIONS_SPACE
        : DEFAULT_OPTIONS_SPACE;
      const available = Math.max(0, menuLimit - measureChromeHeight(this.$refs.menu, box));
      this.optionsMaxHeight = wholeItemsHeight(box, items, available, MIN_VISIBLE_OPTIONS);
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

    /**
     * Escape закрывает открытое меню - как и любое другое окно проекта. Событие
     * дальше не идёт: без этого то же нажатие закроет ещё и модалку под списком,
     * и человек потеряет форму вместо того, чтобы свернуть список.
     */
    handleEscape(e) {
      if (e.key !== 'Escape') return;
      this.isOpen = false;
      this.searchQuery = '';
      e.stopPropagation();
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
      const vw = window.innerWidth / z;
      const gap = 5;
      const margin = 8; // не впритык к краю экрана
      const spaceBelow = vh - r.bottom - gap - margin;
      const spaceAbove = r.top - gap - margin;
      // По умолчанию вниз; если снизу мало места, а сверху больше - открываем ВВЕРХ
      // (иначе высокое меню на низком вьюпорте/мобильном bottom-sheet уезжает за экран).
      const openUp = spaceBelow < 200 && spaceAbove > spaceBelow;
      const avail = Math.max(120, Math.floor(openUp ? spaceAbove : spaceBelow));
      const width = Math.max(Math.round(r.width), this.menuMinWidth);
      // Меню шире триггера уехало бы за правый край - сдвигаем влево ровно
      // настолько, чтобы уместиться. При menuMinWidth=0 ширина равна триггеру,
      // тот всегда на экране, и сдвиг не срабатывает.
      const left = Math.min(Math.round(r.left), Math.max(margin, Math.round(vw - width - margin)));
      this.menuStyle = {
        position: 'fixed',
        left: `${left}px`,
        width: `${width}px`,
        // клампим по доступному пространству выбранной стороны, чтобы меню не выходило за экран
        maxHeight: `${Math.min(this.menuMaxHeight, avail)}px`,
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
      // Доступное место изменилось (скролл/ресайз сдвинули триггер) - пересчитываем,
      // сколько целых пунктов теперь помещается.
      if (this.isOpen) this.$nextTick(this.updateOptionsHeight);
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
  border: 1px solid var(--border);
  background-color: var(--surface);
  border-radius: 50px;
  outline: none;
  cursor: pointer;
  transition: border-color 0.2s;
}

.base-dropdown__button:hover:not(:disabled) {
  border-color: var(--accent);
}

.base-dropdown__button--disabled {
  background-color: var(--surface-2);
  cursor: not-allowed;
  opacity: 0.6;
}

.base-dropdown__text {
  font-size: 14px;
  color: var(--text);
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.base-dropdown__text--placeholder {
  color: var(--text-muted);
  font-weight: 400;
}

.base-dropdown__arrow {
  width: 10px;
  height: 10px;
  flex-shrink: 0;
  color: var(--text-muted);
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
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 20px;
  box-shadow: 0 3px 10px var(--shadow-drop);
  z-index: 1000;
  overflow: hidden;
  /* flex-column, чтобы inline maxHeight (teleport-режим) ужимал прокручиваемый
     список опций, а не обрезал его через overflow:hidden. */
  display: flex;
  flex-direction: column;
}

.base-dropdown__search {
  padding: 8px 12px;
  border-bottom: 1px solid var(--border);
}

.base-dropdown__search-input {
  width: 100%;
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  color: var(--text);
  font-size: 13px;
  outline: none;
  transition: border-color 0.2s;
  box-sizing: border-box;
}

.base-dropdown__search-input:focus {
  border-color: var(--accent);
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
  background-color: var(--row-hover);
}

.base-dropdown__item--selected {
  background-color: var(--accent-tint);
  color: var(--accent-text);
}

.base-dropdown__item--selected .base-dropdown__item-text {
  color: var(--accent-text);
}

.base-dropdown__item:first-child {
  border-radius: 10px 10px 0 0;
}

.base-dropdown__item:last-child {
  border-radius: 0 0 10px 10px;
}

.base-dropdown__item-text {
  font-size: 13px;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.base-dropdown__empty {
  padding: 12px 15px;
  font-size: 13px;
  color: var(--text-muted);
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
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--surface);
  position: relative;
  transition: background-color 0.15s ease, border-color 0.15s ease;
}

.base-dropdown__check--on {
  background: var(--accent);
  border-color: var(--accent);
}

.base-dropdown__check--on::after {
  content: '';
  position: absolute;
  left: 4px;
  top: 1px;
  width: 4px;
  height: 8px;
  border: solid var(--accent-contrast);
  border-width: 0 2px 2px 0;
  transform: rotate(45deg);
}

.base-dropdown__clear {
  width: 100%;
  padding: 8px 15px;
  border: none;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  color: var(--accent-text);
  font-size: 12px;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
}

.base-dropdown__clear:hover:not(:disabled) {
  background: var(--row-hover);
}

/* Пустой выбор: строка остаётся на месте (высота зарезервирована), но гаснет
   и не кликается - иначе её появление дёргало бы список при первом выборе. */
.base-dropdown__clear--disabled {
  color: var(--text-muted);
  cursor: default;
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
