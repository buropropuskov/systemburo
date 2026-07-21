<template>
  <div class="passage__grid">
    <div
      v-for="item in tables"
      :key="item.table.id"
      class="passage__item"
      :class="{
        'passage__item--active': isSelected(item.table.id) && item.table.status === 'active',
        'passage__item--attached': attachedIds.includes(item.table.id),
        'passage__item--inactive': item.table.status !== 'active'
      }"
      @click="toggle(item)"
      @mouseenter="showTooltip(item, $event)"
      @mouseleave="hideTooltip"
    >
      {{ item.table.display_name }}
    </div>
  </div>

  <!-- Tooltip для неактивных таблиц -->
  <div
    v-if="tooltip.visible"
    class="inactive-tooltip"
    :style="{ top: tooltip.y + 'px', left: tooltip.x + 'px' }"
  >
    <div class="inactive-tooltip-content">
      {{ tooltip.text }}
    </div>
  </div>
</template>

<script>
/**
 * Грид выбора целевых таблиц плитками (#1194). Извлечён 1:1 (markup+CSS) из блока
 * "Проезд"/"Места прохода" VehicleForm.vue/EmployeeForm.vue (#481: копировать байт-в-
 * байт, не "улучшать") - переиспользуется там же и в TableBulkTargetModal (групповой
 * перенос/добавление строк таблицы проходной).
 *
 * tables - список в формате { table: { id, display_name, status, status_comment,
 * table_type } } (как отдаёт VehicleForm/EmployeeForm после маппинга /system-tables).
 */
import { getViewportZoom } from '@/utils/viewportScale';

export default {
  name: 'TargetTablesGrid',
  props: {
    tables: {
      type: Array,
      required: true
    },
    modelValue: {
      type: Array,
      default: () => []
    },
    attachedIds: {
      type: Array,
      default: () => []
    },
    // true - можно выбрать несколько таблиц (как в форме заявки), false - выбор
    // одной таблицы снимает предыдущую.
    multiple: {
      type: Boolean,
      default: true
    }
  },
  emits: ['update:modelValue'],
  data() {
    return {
      tooltip: {
        visible: false,
        text: '',
        x: 0,
        y: 0
      }
    };
  },
  methods: {
    isSelected(id) {
      return this.modelValue.includes(id);
    },
    toggle(item) {
      if (item.table.status !== 'active') {
        return;
      }
      const id = item.table.id;
      const next = [...this.modelValue];
      const index = next.indexOf(id);
      if (index > -1) {
        next.splice(index, 1);
      } else if (this.multiple) {
        next.push(id);
      } else {
        next.length = 0;
        next.push(id);
      }
      this.$emit('update:modelValue', next);
    },
    showTooltip(item, event) {
      if (item.table.status !== 'active') {
        const tooltipText = item.table.status_comment
          ? `Недоступно: ${item.table.status_comment}`
          : 'Недоступно';

        this.tooltip.text = tooltipText;
        this.tooltip.visible = true;

        this.$nextTick(() => {
          // position:fixed тултип внутри зазумленного <html>: rect device-px ->
          // делим на zoom (inline left/top = layout-px). Отступ -10 не делим.
          const z = getViewportZoom();
          const rect = event.target.getBoundingClientRect();
          this.tooltip.x = (rect.left + rect.width / 2) / z;
          this.tooltip.y = rect.top / z - 10;
        });
      }
    },
    hideTooltip() {
      this.tooltip.visible = false;
    }
  }
};
</script>

<style scoped>
.passage__grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
    row-gap: 5px;
    max-width: 425px;
    margin-top: 5px;
}

.passage__item {
    height: 30px;
    background: #F2F2F2;
    color: #a2a2a2;
    border-radius: 50px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s;
    padding: 0 10px;
    text-align: center;
    border: 1px solid transparent;
    position: relative;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.passage__item:hover:not(.passage__item--active):not(.passage__item--inactive) {
    background: #e8e8e8;
}

.passage__item--active {
    background: #4F5BDF;
    color: #fff;
    border-color: #4F5BDF;
}

.passage__item--inactive {
    background: #ffe6e6;
    color: #ff6b6b;
    border-color: #ffcccc;
    cursor: not-allowed;
    opacity: 0.7;
}

.passage__item--attached {
    border-left: 3px solid #4F5BDF;
}

.inactive-tooltip {
    position: fixed;
    transform: translateX(-50%) translateY(-100%);
    z-index: 10000;
    pointer-events: none;
}

.inactive-tooltip-content {
    background: #333;
    color: white;
    padding: 8px 12px;
    border-radius: 8px;
    font-size: 12px;
    max-width: 300px;
    box-shadow: 0 2px 8px rgba(0,0,0,0.2);
}

.inactive-tooltip-content::before {
    content: '';
    position: absolute;
    top: 100%;
    left: 50%;
    transform: translateX(-50%);
    border: 5px solid transparent;
    border-top-color: #333;
}

@media (max-width: 768px) {
    /* Сетка перестраивалась только с 480 - на 481-768 оставались три колонки
       по 135px, куда название места не влезало. */
    .passage__grid {
        grid-template-columns: repeat(2, 1fr);
        max-width: 100%;
    }

    /* Названия обрезались многоточием - пускаем в две строки. */
    .passage__item {
        height: auto;
        min-height: 36px;
        padding: 6px 10px;
        white-space: normal;
        overflow: visible;
        text-overflow: clip;
        line-height: 1.25;
    }
}
</style>
