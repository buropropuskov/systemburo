<script setup>
const checked = defineModel({ type: Boolean, required: true });
</script>

<template>
  <label class="document-at-post-choice">
    <input v-model="checked" type="checkbox">
    <span>Документ на посту</span>
  </label>
</template>

<style scoped>
.document-at-post-choice {
    display: flex;
    align-items: center;
    gap: 6px;
    min-height: 34px;
    font-size: 12px;
    white-space: nowrap;
    cursor: pointer;
}
.document-at-post-choice input {
    flex: none;
    accent-color: var(--accent);
}
</style>
