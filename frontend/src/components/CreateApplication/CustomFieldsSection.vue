<template>
  <div class="custom-fields">
    <div
      v-for="field in fields"
      :key="field.id"
      class="custom-fields__item"
    >
      <label class="input__label">{{ field.label }}<span
        v-if="field.is_required"
        class="required"
      > *</span></label>
      <input
        type="text"
        :class="['input', { 'input--error': field.is_required && submitted && !modelValue[field.id]?.trim() }]"
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
        // Триггер подсветки ошибок обязательных полей (is_required=true).
        // Дефолт false - поведение без переданного пропса не меняется.
        submitted: { type: Boolean, default: false },
    },
    emits: ['update:modelValue'],
};
</script>

<style scoped>
.custom-fields {
    display: flex;
    gap: 30px;
    padding: 15px;
    border-bottom: 1px solid var(--border);
}

.custom-fields__item {
    width: 260px;
    display: flex;
    flex-direction: column;
    gap: 5px;
}

.input__label {
    font-size: 13px;
    color: var(--text-muted);
}

.required {
    color: var(--danger-text);
}

.input {
    width: 100%;
    height: 40px;
    border: 1px solid var(--border);
    outline: none;
    background: var(--surface);
    border-radius: 15px;
    padding: 5px 10px;
    font-family: inherit;
    font-size: 13px;
    transition: border-color 0.2s ease;
}

.input:focus {
    border-color: var(--accent);
}

.input--error {
    border-color: var(--danger);
}

/* Фикс-ширные (260px) поля в ряд не влезают на узком - стекаем в колонку. */
@media (max-width: 768px) {
    .custom-fields {
        flex-direction: column;
        gap: 16px;
        padding: 12px;
    }

    .custom-fields__item {
        width: 100%;
    }
}
</style>
