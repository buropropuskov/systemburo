<template>
  <button class="action-btn" :class="[direction + '-btn', { active: checked, revertable }]" :title="revertable ? 'Отменить последнюю отметку ' + movement : undefined">
    {{ revertable ? 'Отмена' : label }}
  </button>
</template>
<script>
export default {
  computed: { movement() { return { 'Вход': 'входа', 'Выход': 'выхода', 'Въезд': 'въезда', 'Выезд': 'выезда' }[this.label]; } },
  name: 'PassageMarkButton',
  props: { direction: { type: String, required: true }, checked: Boolean, revertable: Boolean, label: { type: String, required: true } },
};
</script>
