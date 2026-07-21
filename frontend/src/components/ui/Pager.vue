<template>
  <div class="pager">
    <slot name="lead" />
    <span class="pager__total">Всего: {{ total }}</span>
    <button
      type="button"
      class="lk-button lk-button--ghost pager__btn"
      :disabled="page <= 1 || loading"
      @click="$emit('update:page', page - 1)"
    >
      Назад
    </button>
    <span class="pager__page">{{ pagePrefix }}{{ page }} / {{ totalPages }}</span>
    <button
      type="button"
      class="lk-button lk-button--ghost pager__btn"
      :disabled="page >= totalPages || loading"
      @click="$emit('update:page', page + 1)"
    >
      Вперёд
    </button>
  </div>
</template>

<script>
/**
 * Постраничная навигация под таблицей: «Всего: N» + Назад/Вперёд + номер страницы.
 * Позиционирование (justify-content, отступы, цвет) задаёт потребитель классом
 * на самом компоненте - здесь только раскладка ряда и типографика чисел.
 * Цвет и размеры .pager__total/.pager__btn намеренно не заданы: потребители
 * правят их через :deep(), и своё правило тут вступило бы с ними в спор
 * специфичности, который решается порядком загрузки чанков.
 */
export default {
  name: 'UiPager',
  props: {
    page: {
      type: Number,
      required: true,
    },
    totalPages: {
      type: Number,
      required: true,
    },
    // Строкой - когда потребитель форматирует число сам (разряды, единицы).
    total: {
      type: [Number, String],
      required: true,
    },
    loading: {
      type: Boolean,
      default: false,
    },
    // Например «Стр. » перед номером страницы.
    pagePrefix: {
      type: String,
      default: '',
    },
  },
  emits: ['update:page'],
};
</script>

<style scoped>
.pager {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
}

.pager__page {
  font-variant-numeric: tabular-nums;
}
</style>
