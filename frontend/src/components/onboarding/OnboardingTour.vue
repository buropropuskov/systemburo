<script setup>
import { watch, onMounted, onBeforeUnmount } from 'vue';
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
 * Рельс держим развёрнутым на nav-шаге И на шаге ПЕРЕД ним: разворачиваем
 * заранее (overlay через tourForceExpand, без сдвига контента), чтобы к моменту
 * подсветки рельс уже доехал до полной ширины и driver померил рамку сразу
 * верно - без ре-замера и без моргания спотлайта.
 */
function railNeeded(globalIndex) {
  const cur = store.steps[globalIndex];
  const next = store.steps[globalIndex + 1];
  return Boolean(cur?.expandRail || next?.expandRail);
}

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
    onCtaClick: finishAndCreateApp,
    // Esc/оверлей/крестик/Пропустить -> просто останавливаем тур. teardown
    // (через watch isActive) снимет overlay и пометит авто-тур пройденным -
    // надёжно, даже если шаг закрыли во время entry-анимации (когда driver
    // не зовёт onDestroyed).
    onCloseRequest: () => store.stop(),
    onDestroyed: () => handleDestroyed(myGen),
  });
  driverObj.drive(0);
}

/** CTA финала: завершаем тур и ведём на оформление первой заявки. */
function finishAndCreateApp() {
  finishTour();
  router.push('/new-application').catch(() => {});
}

/**
 * Пользователь нажал "Далее" на последнем шаге сегмента: если следующий шаг -
 * на другой странице, переходим туда (тур продолжится); если шагов больше нет,
 * завершаем тур.
 */
function handleBoundaryNext(activeGlobalIndex) {
  // Берём индекс от driver'а (точная позиция); store.currentIndex может отставать,
  // если onHighlighted последнего шага сегмента ещё не обновил его.
  const idx = typeof activeGlobalIndex === 'number' ? activeGlobalIndex : store.currentIndex;
  if (idx !== store.currentIndex) store.setIndex(idx);
  const next = store.steps[idx + 1];
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
  // Навигация может быть отменена глобальным beforeEach (dirty-confirm и т.п.).
  // Тогда afterEach не сработает - не оставляем тур висеть в pendingSegment.
  router.push(targetRoute).catch(() => {
    if (store.pendingSegment) {
      store.clearPending();
      store.stop();
    }
  });
}

function finishTour() {
  // destroy() синхронно зовёт onDestroyed -> handleDestroyed (pendingSegment=false)
  // -> markCompleted (если авто) + stop. Без инстанса завершаем напрямую.
  if (driverObj) {
    driverObj.destroy();
  } else {
    restoreRail();
    markIfAuto();
    store.stop();
  }
}

/**
 * Авто-тур (первый вход) помечаем пройденным даже при выходе/пропуске, чтобы
 * он не запускался снова. Ручной запуск (кнопка «Обучение») флаг не трогает.
 */
function markIfAuto() {
  if (!store.isManual) store.markCompleted();
}

function handleDestroyed(gen) {
  // Игнорируем callback от инстанса, который уже сменён следующим сегментом.
  if (gen !== driverGen) return;
  driverObj = null;
  // Переход между страницами: тур продолжается, не останавливаем и рельс не трогаем.
  if (store.pendingSegment) return;
  restoreRail();
  markIfAuto();
  store.stop();
}

function teardown() {
  // Тур ещё жив (logout/unmount во время прохождения) - авто-тур помечаем
  // пройденным здесь: ниже driverGen++ обезвредит отложенный onDestroyed, и тот
  // до markIfAuto уже не дойдёт. Так автозапуск действительно "один раз".
  if (driverObj) markIfAuto();
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

/**
 * Автозапуск один раз для любого первого входа: на «Обзоре», если юзер
 * авторизован и тур ещё не пройден. Статус per-user тянется с бэкенда
 * (loadStatus) - на ошибке сети не автозапускаем (statusLoaded остаётся false).
 * Повторно не сработает: completedVersion ставится при любом завершении
 * авто-тура (см. markIfAuto), а на бэкенде - per-user, сброс только админом.
 */
async function maybeAutostart() {
  if (store.isActive) return;
  if (route.path !== '/news') return;
  if (!store.canShowTour) return;
  if (!store.statusLoaded) await store.loadStatus();
  // Перепроверяем после await: статус мог не загрузиться, юзер мог уйти/стартовать.
  if (!store.statusLoaded || store.isActive || route.path !== '/news') return;
  if (store.hasCompleted()) return;
  store.start({ manual: false });
}

watch(() => route.path, maybeAutostart);

onMounted(maybeAutostart);

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
