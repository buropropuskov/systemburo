<template>
  <div
    ref="trigger"
    class="row-remove-menu"
  >
    <!-- rt-pass__act: в карточке строки на телефоне кнопка становится такой же
         пилюлей с подписью, как «Удалить» рядом (responsive-tables.css, часть 3).
         Без подписи в подвале талона оставался бы значок в 16px - ровно то, за что
         владелец забраковал корзину в таблице людей. На десктопе подпись скрыта
         базовым правилом, там кнопка остаётся значком в узком столбце. -->
    <button
      ref="button"
      class="delete-btn rt-pass__act rt-pass__act--danger"
      title="Убрать"
      :disabled="disabled"
      data-testid="row-remove-trigger"
      @click="toggle"
    >
      <AppIcon
        name="trashcan"
        class="delete-icon rt-pass__act-icon"
      />
      <span class="rt-pass__act-label">Убрать</span>
    </button>

    <teleport to="body">
      <transition name="dropdown">
        <div
          v-if="isOpen"
          ref="menu"
          class="row-remove-menu__list"
          :style="menuStyle"
        >
          <button
            type="button"
            class="row-remove-menu__item"
            data-testid="row-remove-current"
            @click="choose('current')"
          >
            Убрать из этой таблицы
          </button>
          <button
            type="button"
            class="row-remove-menu__item row-remove-menu__item--danger"
            data-testid="row-remove-all"
            @click="choose('all')"
          >
            Убрать из всех таблиц
          </button>
        </div>
      </transition>
    </teleport>
  </div>
</template>

<script>
/**
 * Per-row подменю корзины (#1194 S5): показывается вместо обычной кнопки
 * удаления, когда сущность привязана к нескольким таблицам «Проезд»/«Проход»
 * (target_tables_count > 1) - выбор между снятием ТОЛЬКО с текущей таблицы и
 * глобальной деактивацией. Позиционирование - teleport + fixed-координаты по
 * getBoundingClientRect с флипом вверх/вниз, 1:1 с паттерном BaseDropdown
 * (тот же класс задачи: триггер живёт внутри rt-table с overflow:hidden).
 */
import { getViewportZoom } from '@/utils/viewportScale';
import AppIcon from '@/components/icons/AppIcon.vue';

export default {
  name: 'TableRowRemoveMenu',
  components: { AppIcon },
  props: {
    disabled: { type: Boolean, default: false },
  },
  emits: ['remove-current', 'remove-all'],
  data() {
    return {
      isOpen: false,
      menuStyle: {},
    };
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
        this.updateMenuPosition();
        this.addRepositionListeners();
        document.addEventListener('click', this.handleClickOutside, true);
      } else {
        this.teardownOpenState();
      }
    },
    choose(kind) {
      this.teardownOpenState();
      this.$emit(kind === 'current' ? 'remove-current' : 'remove-all');
    },
    teardownOpenState() {
      this.isOpen = false;
      this.removeRepositionListeners();
      document.removeEventListener('click', this.handleClickOutside, true);
    },
    // Capture-фаза (см. BaseDropdown/#1132): клик по кнопке-триггеру идёт через
    // @click.stop родительской .actions-col и не всплывает до document в
    // bubble-фазе - capture срабатывает раньше stopPropagation.
    handleClickOutside(e) {
      const inTrigger = this.$refs.trigger && this.$refs.trigger.contains(e.target);
      const inMenu = this.$refs.menu && this.$refs.menu.contains(e.target);
      if (!inTrigger && !inMenu) this.teardownOpenState();
    },
    updateMenuPosition() {
      // .row-remove-menu (wrapper) - display:contents, не генерирует бокс -
      // getBoundingClientRect дал бы нулевой rect; координаты берём с самой кнопки.
      const el = this.$refs.button;
      if (!el) return;
      // Меню телепортится в body внутри зазумленного <html> (масштаб под 1440):
      // rect - device-px, innerHeight/innerWidth - НЕзумленные; делим на zoom, чтобы
      // считать в layout-px (при zoom=1 без изменений). 1:1 с BaseDropdown.
      const z = getViewportZoom();
      const raw = el.getBoundingClientRect();
      const r = { top: raw.top / z, bottom: raw.bottom / z, right: raw.right / z };
      const vh = window.innerHeight / z;
      const vw = window.innerWidth / z;
      const gap = 5;
      const margin = 8;
      const menuWidth = 210;
      const estimatedHeight = 84;
      const spaceBelow = vh - r.bottom - gap - margin;
      const spaceAbove = r.top - gap - margin;
      const openUp = spaceBelow < estimatedHeight && spaceAbove > spaceBelow;
      // Прижимаем меню правым краем к триггеру (иконка у правого края строки),
      // клампим по ширине вьюпорта, чтобы не уехать за экран.
      const left = Math.min(Math.max(margin, r.right - menuWidth), vw - menuWidth - margin);
      this.menuStyle = {
        position: 'fixed',
        left: `${Math.round(left)}px`,
        width: `${menuWidth}px`,
        zIndex: 2000,
        ...(openUp
          ? { bottom: `${Math.round(vh - r.top + gap)}px`, top: 'auto' }
          : { top: `${Math.round(r.bottom + gap)}px`, bottom: 'auto' }),
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
.row-remove-menu {
  display: contents;
}

/* 1:1 с .delete-btn/.delete-icon CarsTable.vue/PeopleTable.vue - scoped-стили
   родителя не достают до этого компонента, копия обязательна (#481). */
.delete-btn {
  background: none;
  border: none;
  cursor: pointer;
  padding: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.delete-btn:hover:not(:disabled) {
  background-color: transparent;
}

.delete-btn:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.delete-icon {
  color: var(--text);
  width: 16px;
  height: 16px;
  opacity: 0.7;
  transition: opacity 0.2s ease;
}

.delete-btn:hover:not(:disabled) .delete-icon {
  opacity: 1;
}

.row-remove-menu__list {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
  box-shadow: 0 3px 10px var(--shadow-drop);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.row-remove-menu__item {
  background: none;
  border: none;
  text-align: left;
  padding: 10px 15px;
  font-size: 13px;
  color: var(--text);
  cursor: pointer;
  transition: background-color 0.2s;
}

.row-remove-menu__item:hover {
  background-color: var(--surface-2);
}

.row-remove-menu__item--danger {
  color: var(--danger-text);
}

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
