<script setup>
import { watch, onBeforeUnmount } from 'vue';
import { useRoute } from 'vue-router';
import { useOnboardingStore } from '@/stores/onboarding';
import { useUiStore } from '@/stores/ui';
import { useOnboarding } from '@/composables/useOnboarding';
import { collectSegment } from '@/components/onboarding/onboardingSteps';

const store = useOnboardingStore();
const ui = useUiStore();
const route = useRoute();
const { waitForElement, createDriver } = useOnboarding();

let driverObj = null;
let waitController = null;
// Прежнее состояние рельса до того как тур его развернул - чтобы вернуть как было.
let railSaved = null;

/**
 * Рельс нужен развёрнутым на nav-шаге И на шаге прямо перед ним: разворачиваем
 * заранее, чтобы к моменту подсветки рельс уже доехал до полной ширины и driver
 * померил рамку спотлайта корректно (иначе ловим элемент в середине анимации).
 */
function railNeeded(globalIndex) {
  const cur = store.steps[globalIndex];
  const next = store.steps[globalIndex + 1];
  return Boolean(cur?.expandRail || next?.expandRail);
}

/**
 * Развернуть/вернуть рельс под текущую позицию. Разворот - ОВЕРЛЕЙНЫЙ
 * (tourForceExpand), без сдвига контента: reflow от пина дёргал бы спотлайт
 * соседних шагов. sidebarHidden снимаем, иначе скрытый рельс не покажется.
 */
function applyRail(globalIndex) {
  if (railNeeded(globalIndex)) {
    if (!railSaved) {
      railSaved = { force: ui.tourForceExpand, hidden: ui.sidebarHidden };
    }
    ui.tourForceExpand = true;
    ui.sidebarHidden = false;
  } else {
    restoreRail();
  }
}

function restoreRail() {
  if (railSaved) {
    ui.tourForceExpand = railSaved.force;
    ui.sidebarHidden = railSaved.hidden;
    railSaved = null;
  }
}

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
    onIndexChange: (globalIndex) => {
      store.setIndex(globalIndex);
      applyRail(globalIndex);
    },
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
  restoreRail();
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
