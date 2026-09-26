<template>
  <div
    ref="row"
    class="filter-tabs"
    :class="{ 'filter-tabs--collapsed': collapsed }"
  >
    <!-- Не поместились в строку - становимся списком. Ряд при этом остаётся в
         разметке (скрыт), иначе измерять было бы нечего и он бы не «разворачивался»
         обратно, когда места снова хватит. -->
    <BaseDropdown
      v-if="collapsed"
      class="filter-tabs__select"
      data-testid="filter-tabs-select"
      :model-value="modelValue"
      :options="dropdownOptions"
      label-key="label"
      value-key="value"
      @update:model-value="$emit('update:modelValue', $event)"
    />
    <button
      v-for="tab in visibleTabs"
      :key="tab.key"
      class="filter-tab"
      :class="{ 'filter-tab--active': modelValue === tab.key }"
      :data-testid="`filter-tab-${tab.key}`"
      @click="$emit('update:modelValue', tab.key)"
    >
      <span class="filter-tab__label">{{ tab.label }}</span>
      <Badge
        v-if="tab.count != null"
        :label="String(tab.count)"
        size="sm"
        variant="neutral"
      />
    </button>
  </div>
</template>

<script>
import Badge from '@/components/ui/Badge.vue';
import BaseDropdown from '@/components/ui/BaseDropdown.vue';

/**
 * Ряд пилюль-фильтров. Если в строку они не помещаются - сворачивается в
 * выпадающий список (правило владельца, тег #55678).
 *
 * Перенос на второй ряд запрещён намеренно: он крадёт высоту и рвёт шапку. Замер,
 * с которого правило появилось: четыре фильтра «Моих сотрудников» - 807px при
 * контейнере 728 на ширине 768.
 *
 * Меряем сумму собственных ширин пилюль, а не `scrollWidth` ряда: ряд переносит
 * содержимое, и переполнение в него не утекает - `scrollWidth` был бы равен
 * `clientWidth` и всегда говорил «помещается».
 */
export default {
  name: 'FilterTabs',
  components: { Badge, BaseDropdown },
  props: {
    tabs: {
      type: Array,
      required: true,
    },
    modelValue: {
      type: String,
      required: true,
    },
  },
  emits: ['update:modelValue'],
  data() {
    return { collapsed: false };
  },
  computed: {
    visibleTabs() {
      return this.tabs.filter((tab) => tab.visible !== false);
    },
    dropdownOptions() {
      return this.visibleTabs.map((tab) => ({ value: tab.key, label: tab.label }));
    },
  },
  watch: {
    visibleTabs() {
      this.$nextTick(this.measure);
    },
  },
  mounted() {
    // ResizeObserver ловит и смену ширины контейнера, не только окна: ряд стоит в
    // шапке, а та умеет перестраиваться сама. В jsdom его нет - там довольствуемся
    // событием окна, замер всё равно вызывается на монтировании.
    if (typeof ResizeObserver === 'function') {
      this.observer = new ResizeObserver(() => this.measure());
      this.observer.observe(this.$refs.row);
    } else if (typeof window !== 'undefined') {
      this.onResize = () => this.measure();
      window.addEventListener('resize', this.onResize);
    }
    this.$nextTick(this.measure);
  },
  beforeUnmount() {
    if (this.observer) this.observer.disconnect();
    if (this.onResize) window.removeEventListener('resize', this.onResize);
  },
  methods: {
    /** Помещается ли ряд в одну строку. Считаем по пилюлям, список в сумму не берём. */
    measure() {
      const row = this.$refs.row;
      if (!row) return;
      const pills = [...row.children].filter((el) => el.classList.contains('filter-tab'));
      if (!pills.length) return;
      const gap = parseFloat(getComputedStyle(row).columnGap) || 0;
      const need = pills.reduce((sum, el) => sum + el.offsetWidth, 0) + gap * (pills.length - 1);
      // Гистерезис в один зазор: без него на границе ряд и список мигают друг в
      // друга, потому что свёрнутый ряд освобождает место и тут же разворачивается.
      this.collapsed = this.collapsed ? need > row.clientWidth - gap : need > row.clientWidth;
    },
  },
};
</script>

<style scoped>
.filter-tabs {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

/* Свёрнутый ряд: пилюли остаются в разметке для замера, но не видны и не кликаются.
   `visibility` вместо `display: none` - у скрытого display ширины нет, и померить
   «поместились бы» стало бы нечем. */
.filter-tabs--collapsed .filter-tab {
  position: absolute;
  visibility: hidden;
  pointer-events: none;
}

.filter-tabs--collapsed {
  position: relative;
}

.filter-tabs__select {
  min-width: 220px;
}

.filter-tab {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 0 14px;
  height: 30px;
  border: 1px solid var(--border);
  background: var(--surface);
  border-radius: 50px;
  cursor: pointer;
  font-size: 14px;
  transition: all 0.2s;
  white-space: nowrap;
}

.filter-tab:hover {
  border-color: var(--accent);
}

.filter-tab--active {
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

/* Частичное улучшение тач-таргета: 30px до полноценных 44px (WCAG 2.5.5, эталон §18)
   не дотягивают, 36 - размер компактных контролов проекта. Полные 44 в плотном ряду
   вкладок распирают шапку, поэтому там, где они нужны, раздел добавляет их сам -
   так сделано в чёрном списке. Десктопные 30px оставляем: там попадают курсором. */
@media (max-width: 767.98px) {
  .filter-tab {
    height: 36px;
  }
}
</style>
