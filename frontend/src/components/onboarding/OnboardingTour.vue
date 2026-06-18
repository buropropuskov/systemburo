<script setup>
import { watch, onBeforeUnmount } from 'vue';
import { useRoute } from 'vue-router';
import { useOnboardingStore } from '@/stores/onboarding';
import { useOnboarding } from '@/composables/useOnboarding';
import { collectSegment } from '@/components/onboarding/onboardingSteps';

const store = useOnboardingStore();
const route = useRoute();
const { waitForElement, createDriver } = useOnboarding();

let driverObj = null;
let waitController = null;

async function startSegment() {
  const segmentSteps = collectSegment(store.steps, store.currentIndex, route.path);
  if (!segmentSteps.length) {
    store.stop();
    return;
  }

  const segmentStartIndex = store.currentIndex;
  // Первый шаг с целью: дожидаемся её появления, иначе деградируем в центр-модал,
  // чтобы driver не падал на отсутствующем элементе.
  const firstStep = segmentSteps[0];
  if (firstStep.element) {
    waitController = new AbortController();
    const el = await waitForElement(firstStep.element, 2500, waitController.signal);
    waitController = null;
    // Тур могли остановить (Esc/logout/повторный старт) пока ждали элемент -
    // не поднимаем driver-зомби поверх неактивного тура.
    if (!store.isActive) return;
    if (!el) segmentSteps[0] = { ...firstStep, element: null };
  }

  driverObj = createDriver(segmentSteps, {
    startIndex: segmentStartIndex,
    onIndexChange: (globalIndex) => store.setIndex(globalIndex),
    onDestroyed: handleDestroyed,
  });
  driverObj.drive(0);
}

function handleDestroyed() {
  driverObj = null;
  // Срез 1: сегмент один (news), конец сегмента = конец тура.
  // Запись флага completed (для авто-запуска) - срез 6; ручной запуск флаг не трогает.
  store.stop();
}

function teardown() {
  if (waitController) {
    waitController.abort();
    waitController = null;
  }
  if (driverObj) {
    driverObj.destroy();
    driverObj = null;
  }
}

watch(
  () => store.isActive,
  (active) => {
    if (active) startSegment();
    else teardown();
  },
);

onBeforeUnmount(teardown);
</script>

<template>
  <div
    style="display: none"
    data-testid="ob-host"
  />
</template>
