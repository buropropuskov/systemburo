<template>
  <div class="post-document-notice">
    <strong>Сверить оригинал документа</strong>
    <span>Проверьте документ перед отметкой входа.</span>
  </div>
</template>

<style scoped>
.post-document-notice {
    display: grid;
    gap: 4px;
    margin-bottom: 12px;
    padding: 10px 13px;
    border: 1px solid color-mix(in srgb, var(--warning) 45%, var(--border));
    border-radius: var(--radius-md);
    background: color-mix(in srgb, var(--warning) 8%, var(--surface));
}
</style>
