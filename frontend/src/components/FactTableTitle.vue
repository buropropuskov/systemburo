<template>
  <div class="card-header__title">
    <h3 class="card-title">
      {{ tableType === 'cars' ? 'Автомобили' : 'Люди' }} <span class="highlight-text">по факту</span>
    </h3>
  </div>
</template>

<script setup>
defineProps({ tableType: { type: String, required: true } });
</script>

<style scoped>
.card-title {
  margin: 0;
  color: var(--text);
  font-weight: 600;
  font-size: 1.1em;
}

.highlight-text {
  color: var(--accent-text);
}

@media (max-width: 899.98px) {
  .card-title {
    font-size: 18px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
}
</style>
