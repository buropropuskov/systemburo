<script setup>
import { computed } from 'vue';
import { useOnboardingStore } from '@/stores/onboarding';

const store = useOnboardingStore();
const canShow = computed(() => store.canShowTour);

function startTour() {
  store.start({ manual: true });
}
</script>

<template>
  <button
    v-if="canShow"
    type="button"
    class="lk-button lk-button--secondary ob-start-button"
    data-testid="ob-start-button"
    @click="startTour"
  >
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
    >
      <circle
        cx="12"
        cy="12"
        r="9"
        stroke="currentColor"
        stroke-width="1.8"
      />
      <path
        d="M9.5 9.2a2.6 2.6 0 1 1 3.6 2.4c-.7.3-1.1.7-1.1 1.5v.4"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linecap="round"
      />
      <circle
        cx="12"
        cy="16.6"
        r="1"
        fill="currentColor"
      />
    </svg>
    <span>Обучение</span>
  </button>
</template>

<style scoped>
.ob-start-button {
  height: 25px;
  padding: 0 14px;
  font-size: 13px;
}
</style>
