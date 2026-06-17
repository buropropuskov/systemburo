<template>
  <div class="gallery">
    <p class="gallery__lead">
      Готовые отчёты — нажмите карточку, чтобы заполнить конструктор и сразу увидеть результат.
      Период берётся из фильтра вверху, а любой отчёт можно доработать в конструкторе ниже.
    </p>
    <div class="gallery__grid">
      <button
        v-for="preset in availablePresets"
        :key="preset.id"
        type="button"
        class="gallery__card"
        :class="{ 'gallery__card--active': preset.id === activeId }"
        @click="$emit('apply', preset)"
      >
        <span class="gallery__card-title">{{ preset.title }}</span>
        <span class="gallery__card-desc">{{ preset.description }}</span>
        <span class="gallery__card-result">{{ preset.resultHint }}</span>
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import { REPORT_PRESETS, presetAvailable } from './reportPresets';

const props = defineProps({
  catalog: { type: Object, default: null },
  activeId: { type: String, default: '' },
});

defineEmits(['apply']);

const availablePresets = computed(() =>
  REPORT_PRESETS.filter((p) => presetAvailable(p, props.catalog)),
);
</script>

<style scoped>
.gallery {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.gallery__lead {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-muted);
}

.gallery__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 14px;
}

.gallery__card {
  display: flex;
  flex-direction: column;
  gap: 7px;
  padding: 16px 18px;
  text-align: left;
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  cursor: pointer;
  font-family: inherit;
  transition: transform 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease;
}

.gallery__card:hover {
  transform: translateY(-2px);
  border-color: var(--color-primary);
  box-shadow: 0 6px 18px rgba(79, 91, 223, 0.12);
}

.gallery__card--active {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 1px var(--color-primary) inset;
}

.gallery__card-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text);
}

.gallery__card-desc {
  font-size: 13px;
  line-height: 1.4;
  color: var(--color-text-muted);
}

.gallery__card-result {
  margin-top: auto;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-primary);
}
</style>
