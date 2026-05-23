<template>
  <div class="custom-fields">
    <div
      v-for="field in fields"
      :key="field.id"
      class="custom-fields__item"
    >
      <label class="input__label">{{ field.label }}</label>
      <input
        type="text"
        class="input"
        :placeholder="field.placeholder"
        :value="modelValue[field.id] || ''"
        @input="$emit('update:modelValue', { ...modelValue, [field.id]: $event.target.value })"
      >
    </div>
  </div>
</template>

<script>
export default {
    name: 'CustomFieldsSection',
    props: {
        fields: { type: Array, default: () => [] },
        modelValue: { type: Object, default: () => ({}) },
    },
    emits: ['update:modelValue'],
};
</script>

<style scoped>
.custom-fields {
    display: flex;
    gap: 30px;
    padding: 15px;
    border-bottom: 1px solid #e6e6e6;
}

.custom-fields__item {
    width: 260px;
    display: flex;
    flex-direction: column;
    gap: 5px;
}

.input__label {
    font-size: 13px;
    color: #a2a2a2;
}

.input {
    width: 100%;
    height: 40px;
    border: 1px solid #e6e6e6;
    outline: none;
    background: #FFF;
    border-radius: 10px;
    padding: 5px 10px;
    font-family: inherit;
    font-size: 13px;
    transition: border-color 0.2s ease;
}

.input:focus {
    border-color: #4F5BDF;
}
</style>
