<script setup>
import { watch, onBeforeUnmount } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useOnboardingStore } from '@/stores/onboarding';
import { useUiStore } from '@/stores/ui';
import { useOnboarding } from '@/composables/useOnboarding';
import { collectSegment } from '@/components/onboarding/onboardingSteps';

const store = useOnboardingStore();
const ui = useUiStore();
const route = useRoute();
const router = useRouter();
const { waitForElement, createDriver } = useOnboarding();

// Первую цель сегмента ждём дольше при cross-page: после router.push страница
// монтируется и грузит данные (скелетоны), цель появляется не сразу.
const FIRST_TARGET_TIMEOUT = 4000;

let driverObj = null;
let waitController = null;
// Поколение активного driver-инстанса: отложенный onDestroyed предыдущего
// сегмента (анимация ухода ~0.4s) не должен трогать уже поднятый следующий.
let driverGen = 0;
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
  const myGen = ++driverGen;
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
    const el = await waitForElement(firstStep.element, FIRST_TARGET_TIMEOUT, waitController.signal);
    waitController = null;
    // Тур могли остановить или перезапустить (Esc/logout/новый сегмент) пока
    // ждали элемент - не поднимаем driver-зомби поверх неактивного/чужого тура.
    if (!store.isActive || myGen !== driverGen) return;
    if (!el) segmentSteps[0] = { ...firstStep, element: null };
  }

  driverObj = createDriver(segmentSteps, {
    startIndex: segmentStartIndex,
    onIndexChange: (globalIndex) => {
      store.setIndex(globalIndex);
      applyRail(globalIndex);
    },
    onBoundaryNext: handleBoundaryNext,
    onDestroyed: () => handleDestroyed(myGen),
  });
  driverObj.drive(0);
}

/**
 * Пользователь нажал "Далее" на последнем шаге сегмента: если следующий шаг -
 * на другой странице, переходим туда (тур продолжится); если шагов больше нет,
 * завершаем тур.
 */
function handleBoundaryNext() {
  const next = store.steps[store.currentIndex + 1];
  if (!next) {
    finishTour();
  } else if (next.route !== route.path) {
    advanceToSegment(next.route);
  } else if (driverObj) {
    driverObj.moveNext();
  }
}

function advanceToSegment(targetRoute) {
  store.advanceSegment();
  restoreRail();
  // Старый driver уничтожаем; его отложенный onDestroyed обезврежен driverGen.
  if (driverObj) {
    driverObj.destroy();
    driverObj = null;
  }
  router.push(targetRoute);
}

function finishTour() {
  // destroy() синхронно зовёт onDestroyed -> handleDestroyed (pendingSegment=false)
  // -> restoreRail + store.stop. Без инстанса завершаем напрямую.
  if (driverObj) {
    driverObj.destroy();
  } else {
    restoreRail();
    store.stop();
  }
}

function handleDestroyed(gen) {
  // Игнорируем callback от инстанса, который уже сменён следующим сегментом.
  if (gen !== driverGen) return;
  driverObj = null;
  // Переход между страницами: тур продолжается, не останавливаем и рельс не трогаем.
  if (store.pendingSegment) return;
  restoreRail();
  store.stop();
}

function teardown() {
  driverGen += 1;
  if (waitController) {
    waitController.abort();
    waitController = null;
  }
  if (driverObj) {
    driverObj.destroy();
    driverObj = null;
  }
  store.clearPending();
  restoreRail();
}

watch(
  () => store.isActive,
  (active) => {
    if (active) startSegment();
    else teardown();
  },
);

// Подхват следующего сегмента после cross-page навигации: ждём, пока роутер
// приведёт нас на страницу первого шага следующего сегмента.
const removeAfterEach = router.afterEach((to) => {
  if (!store.isActive || !store.pendingSegment) return;
  // clearPending до startSegment страхует от повторного resume (redirect-цепочка);
  // logout посреди перехода успел бы сбросить pending через teardown - тогда сюда не войдём.
  if (store.currentStep?.route === to.path) {
    store.clearPending();
    startSegment();
  } else {
    // Навигация увела не туда, куда вёл тур - не держим силой.
    store.clearPending();
    store.stop();
  }
});

onBeforeUnmount(() => {
  removeAfterEach();
  teardown();
});
</script>

<template>
  <div
    style="display: none"
    data-testid="ob-host"
  />
</template>
