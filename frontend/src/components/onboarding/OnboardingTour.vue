<script setup>
import { watch, onMounted, onBeforeUnmount } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useOnboardingStore } from '@/stores/onboarding';
import { useUiStore } from '@/stores/ui';
import { useOnboarding, STEP_DEMO_FALLBACK } from '@/composables/useOnboarding';
import { collectSegment } from '@/components/onboarding/stepsFlow';
import { applyReveal, restoreReveal } from '@/components/onboarding/reveal';
import { createStepPreparation, waitSelectorOf } from '@/components/onboarding/stepPreparation';
import { createStepSetReconciliation } from '@/components/onboarding/stepSetReconciliation';
import { createTourGeometryBinding } from '@/components/onboarding/tourGeometryBinding';
import { createTourLayerGuard } from '@/components/onboarding/tourLayerGuard';
import { createTourRecoveryHost } from '@/components/onboarding/tourRecoveryHost';
import { createRailControl } from '@/components/onboarding/railControl';
import { createKeyboardNav } from '@/components/onboarding/keyboardNav';
import { createStepTransition } from '@/components/onboarding/stepTransition';
import { previousStepIndex } from '@/components/onboarding/stepNavigation';
import { createStepWatchers } from '@/components/onboarding/stepWatchers';
import { fadeAndDestroy } from '@/components/onboarding/tourFade';
import { createAutostart } from '@/components/onboarding/tourAutostart';

const store = useOnboardingStore();
const ui = useUiStore();
const route = useRoute();
const router = useRouter();
const { waitForElement, ensureInView, createDriver, prefersReducedMotion } = useOnboarding();

let driverObj = null;
let geometryBinding = null;
// Поколение активного driver-инстанса: отложенный onDestroyed предыдущего
// сегмента (анимация ухода ~0.4s) не должен трогать уже поднятый следующий.
let driverGen = 0;
let routeRequest = 0;
// Тур дошёл до финального шага (кнопка «Готово» или CTA), а не был брошен на
// середине. Сбрасывается при старте каждого тура - см. watch(isActive).
let reachedFinal = false;
let technicalStop = false;
// Наблюдение за DOM на время шага: ведёт шаг вперёд по действию человека и
// удерживает подсветку на пересозданной цели (stepWatchers.js).
const watchers = createStepWatchers({
  getDriver: () => (transitions.isBusy() ? null : driverObj),
  getGen: () => driverGen,
  getStep: (index) => store.steps[index],
  getIndex: () => store.currentIndex, getRevealOpen: () => store.revealOpen,
});
// Границы поднятого сейчас сегмента (глобальные индексы, включительно). Судить о
// принадлежности шага сегменту по одному только route нельзя: тур возвращается на
// ту же страницу несколькими сегментами - у охранника «Доступные мне» идут и до
// таблицы поста, и после неё, финалом.
let segmentRange = null;
const rail = createRailControl(ui, (index) => store.steps[index]);

// Стрелки и Esc ведёт хост: driver теряет нажатие на переезде подсветки и между
// страницами, где инстанса нет вовсе (см. keyboardNav.js).
const keys = createKeyboardNav({
  isActive: () => store.isActive && !layerGuard.isBlocked(),
  next: () => driverObj?.obNext(), prev: () => driverObj?.obPrev(),
  close: () => recovery.hasFailure() ? recovery.stop() : store.stop(),
});
const busyButtons = new Map();
const transitions = createStepTransition({
  isActive: () => store.isActive && !layerGuard.isBlocked(),
  onError: (error, operation) => recovery.fail(error, operation),
  onBusyChange: (busy) => {
    if (busy) {
      document.querySelectorAll('.driver-popover-next-btn, .driver-popover-prev-btn').forEach((button) => {
        if (!busyButtons.has(button)) busyButtons.set(button, button.disabled);
        button.disabled = true;
      });
    } else {
      busyButtons.forEach((disabled, button) => { if (button.isConnected) button.disabled = disabled; });
      busyButtons.clear();
    }
    if (busy) geometryBinding?.stop();
    keys.setBusy(busy);
    if (!busy && !transitions.isBusy() && !layerGuard.isBlocked() && !recovery.isShowing()) {
      driverObj?.obSyncHighlighted?.();
      geometryBinding?.commit(store.currentIndex);
    }
  },
});

const recovery = createTourRecoveryHost({
  store, keys, watchers, rail, getGeometry: () => geometryBinding,
  getDriver: () => driverObj, setDriver: value => { driverObj = value; },
  advanceGeneration: () => ++driverGen, getGeneration: () => driverGen,
  getRoute: () => route.path, cancelTransitions: () => transitions.cancel(),
  startSegment: () => startSegment(), pushRoute: target => pushTourRoute(target),
  reducedMotion: prefersReducedMotion, isBlocked: () => layerGuard.isBlocked(),
  stop: () => { technicalStop = true; store.stop(); },
});

const stepSet = createStepSetReconciliation({ store, recovery,
  isBlocked: () => layerGuard.isBlocked(),
  // Invalidate the old Driver before cancel's busy=false callback can sync it.
  cancel: () => { routeRequest++; driverGen++; transitions.cancel(); },
  cancelPendingRoute: () => router.push(route.path),
});

const layerGuard = createTourLayerGuard((blocked) => {
  if (!store.isActive) return;
  if (blocked) {
    keys.detach();
    transitions.cancel();
    geometryBinding?.stop();
    driverGen += 1;
    const previous = driverObj;
    driverObj = null;
    // Public destroy removes Driver's independent Tab trap. Its focus restore
    // must not steal focus from the question that has just taken ownership.
    layerGuard.preserveGlobalFocus(() => previous?.destroy());
  } else {
    keys.attach();
    if (stepSet.resumeDeferred()) return;
    // Preparation may already have changed the demo/reveal before cancellation.
    // Rebuild this same step so its UI and anchor share one current context.
    if (recovery.hasFailure()) recovery.show();
    else if (!store.pendingSegment) restartSegmentAt(store.currentIndex);
  }
}, { getRevealOpen: () => store.revealOpen });

geometryBinding = createTourGeometryBinding({
  store, getDriver: () => driverObj, getGeneration: () => driverGen,
  isBusy: () => transitions.isBusy() || layerGuard.isBlocked() || recovery.isShowing(),
  runTransition: transitions.run, waitForElement, ensureInView,
  onError: error => recovery.fail(Object.assign(error, { stepId: store.currentStep?.id })),
});

/**
 * Демо-вложение оформления заявки: шаги «Бланк Автомобили/Сотрудники» просят
 * BlankSelector добавить вложение нужного типа, чтобы справа отрисовалась
 * реальная форма. Шаги без demoAttachment - убираем демо (null).
 */
function applyDemoAttachment(globalIndex) {
  const next = store.steps[globalIndex]?.demoAttachment || null;
  const changed = store.demoAttachmentType !== next;
  store.setDemoAttachment(next);
  return changed;
}

// Общая подготовка шага; контракт и ожидания описаны в stepPreparation.
const prepareStep = createStepPreparation({
  store, watchers, applyDemoAttachment,
  beforePrepare: index => { recovery.prepare(store.steps[index]?.id); geometryBinding.stop(); },
  waitSelectorOf, waitForElement, ensureInView, demoFallback: STEP_DEMO_FALLBACK,
  prepareTarget: geometryBinding.prepare,
});

const startSegment = () => transitions.run(startSegmentInner, { replace: true,
  intent: { tourKey: store.activeTourKey, stepId: store.steps[store.currentIndex]?.id, direction: 'segment' } });

async function startSegmentInner(operation) {
  if (!await stepSet.ensure(operation)) return;
  const myGen = ++driverGen;
  // Продолжение может начинаться с другой страницы. Индекс сохраняем: это
  // не «следующий сегмент», который advanceSegment искал бы от текущего шага.
  const savedStep = store.steps[store.currentIndex];
  if (savedStep?.route && savedStep.route !== route.path) {
    return retreatToSegment(store.currentIndex, savedStep.route);
  }
  // Берём сегмент, СОДЕРЖАЩИЙ текущий шаг: при «Назад» между страницами мы на последнем шаге прошлой.
  let segmentStartIndex = store.currentIndex;
  while (segmentStartIndex > 0 && store.steps[segmentStartIndex - 1].route === route.path) {
    segmentStartIndex -= 1;
  }
  const segmentSteps = collectSegment(store.steps, segmentStartIndex, route.path);
  if (!segmentSteps.length) {
    // Шаг продолжения на другой странице - идём туда, а не гасим тур (#2590).
    const target = store.steps[store.currentIndex]?.route;
    if (target && target !== route.path) advanceToSegment(target); else store.stop();
    return;
  }
  segmentRange = { start: segmentStartIndex, end: segmentStartIndex + segmentSteps.length - 1 };
  const localTarget = store.currentIndex - segmentStartIndex;

  const targetStep = segmentSteps[localTarget];
  if (!targetStep) { store.stop(); return; }
  rail.apply(store.currentIndex);
  const ready = await prepareStep(store.currentIndex, operation, { initial: true });
  if (!operation.isCurrent() || myGen !== driverGen) return;
  const targetMissing = ready === false || ready === STEP_DEMO_FALLBACK;

  driverObj = createDriver(segmentSteps, {
    startIndex: segmentStartIndex,
    fallbackIndex: targetMissing ? localTarget : -1,
    onIndexChange: (globalIndex) => {
      if (!store.isActive || myGen !== driverGen) return;
      // Завершение анимации прежнего шага не владеет уже начатой подготовкой.
      if (transitions.isBusy()) return;
      recovery.commit(store.steps[globalIndex]?.id);
      if (!layerGuard.isBlocked()) keys.attach();
      store.setIndex(globalIndex);
      rail.apply(globalIndex);
      watchers.watchStep(globalIndex);
      geometryBinding.commit(globalIndex);
      // Backstop: синхронизируем демо-вложение с подсвеченным шагом (важно для
      // навигации «Назад» - prepareStep отрабатывает только на «Далее»).
      const attachmentChanged = applyDemoAttachment(globalIndex);
      // Смена бланка пересоздаёт форму: driver.js держит ссылку на прежний узел,
      // и после «Назад» с «Сотрудников» на «Автомобили» подсветка пропадала -
      // форма правильная, а рамки нет. Дожидаемся нового узла и просим driver
      // пересчитать подсветку по тому же селектору.
      if (attachmentChanged) geometryBinding.refresh(globalIndex);
      // Прогрев раскрытия для «Назад» внутри группы шагов; сигнал, поставленный
      // prepareStep для СЛЕДУЮЩЕГО шага, не затираем - см. OnboardingTour.revealRace.spec.
      if (store.revealOpen === store.steps[globalIndex + 1]?.reveal?.open) return;
      applyReveal(store.steps, globalIndex);
    },
    onBeforeStep: prepareStep,
    runTransition: transitions.run,
    onBoundaryNext: handleBoundaryNext,
    onBoundaryPrev: handleBoundaryPrev,
    onJumpTo: jumpToStep,
    // Esc/оверлей/крестик/Пропустить -> просто останавливаем тур. teardown
    // (через watch isActive) снимет overlay и пометит авто-тур пройденным -
    // надёжно, даже если шаг закрыли во время entry-анимации (когда driver
    // не зовёт onDestroyed).
    onCloseRequest: () => store.stop(),
    onDestroyed: () => handleDestroyed(myGen),
  });
  driverObj.drive(localTarget);
}

/**
 * Прыжок на произвольный шаг из списка в поповере. Шаг на этой же странице
 * готовим как обычно (демо-вложение, раскрытие узла, ожидание цели) и просим
 * driver перейти; шаг на другой странице отдаём той же дорогой, что и переход по
 * границе сегмента - через навигацию с флагом ожидания.
 *
 * @param {number} globalIndex
 */
async function jumpToStep(globalIndex) {
  const step = store.steps[globalIndex];
  if (!step || globalIndex === store.currentIndex) return;
  return transitions.run(async (operation) => {
    if (step.route !== route.path) return retreatToSegment(globalIndex, step.route);
    // Один route может содержать несколько раздельных сегментов тура.
    if (!isInActiveSegment(globalIndex)) return restartSegmentAt(globalIndex);
    const gen = driverGen;
    rail.apply(globalIndex);
    const ready = await prepareStep(globalIndex, operation);
    if (!operation.isCurrent() || !driverObj || gen !== driverGen) return;
    driverObj.obGoTo(globalIndex, !store.steps[globalIndex]?.element || ready === false || ready === STEP_DEMO_FALLBACK);
  }, { replace: true, intent: { tourKey: store.activeTourKey, stepId: step.id, direction: 'jump' } });
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
    return advanceToSegment(next.route);
  } else if (driverObj) {
    driverObj.moveNext();
  }
}

function advanceToSegment(targetRoute) {
  geometryBinding.stop();
  store.advanceSegment();
  rail.restore();
  restoreReveal();
  // Старый driver уничтожаем; его отложенный onDestroyed обезврежен driverGen.
  if (driverObj) {
    driverObj.destroy();
    driverObj = null;
  }
  // Навигация может быть отменена глобальным beforeEach (dirty-confirm и т.п.).
  // Тогда afterEach не сработает - не оставляем тур висеть в pendingSegment.
  return pushTourRoute(targetRoute);
}

function pushTourRoute(targetRoute) {
  if (!stepSet.isKnown()) return stepSet.failUnknown();
  const request = ++routeRequest;
  const gen = driverGen;
  const failed = () => {
    if (request !== routeRequest || gen !== driverGen || !store.isActive || !store.pendingSegment) return;
    return recovery.fail({ stepId: store.currentStep?.id, reason: 'route-unavailable' });
  };
  // Vue Router возвращает NavigationFailure как результат, а не только reject.
  return router.push(targetRoute).then((failure) => { if (failure) return failed(); }, failed);
}

/**
 * Пользователь нажал "Назад" на ПЕРВОМ шаге сегмента: предыдущий шаг - на другой
 * странице, возвращаемся туда и показываем последний шаг того сегмента.
 *
 * @param {number} segmentStartGlobal глобальный индекс первого шага текущего сегмента
 */
function handleBoundaryPrev(segmentStartGlobal) {
  const prevIdx = previousStepIndex(segmentStartGlobal, store.skippedIndexes);
  const prevStep = store.steps[prevIdx];
  if (!prevStep) return;
  if (prevStep.route !== route.path) {
    return retreatToSegment(prevIdx, prevStep.route);
  } else return restartSegmentAt(prevIdx);
}

/**
 * Лежит ли шаг в поднятом сейчас сегменте driver.js.
 *
 * @param {number} globalIndex
 * @returns {boolean}
 */
function isInActiveSegment(globalIndex) {
  return Boolean(segmentRange) && globalIndex >= segmentRange.start && globalIndex <= segmentRange.end;
}

/**
 * Поднять сегмент заново вокруг шага на ТЕКУЩЕЙ странице. Дорога для прыжка в
 * соседний сегмент того же route: `retreatToSegment` там не годится - он ждёт
 * навигации, а `router.push` на текущий путь её не делает и тур бы остановился.
 *
 * @param {number} globalIndex глобальный индекс целевого шага
 */
function restartSegmentAt(globalIndex) {
  geometryBinding.stop();
  store.setIndex(globalIndex);
  rail.restore();
  restoreReveal();
  // Поколение двигаем ДО destroy: onDestroyed прежнего инстанса иначе примет это
  // за конец обучения и остановит тур (флага ожидания навигации здесь нет -
  // страница та же). Тот же приём, что в teardown.
  driverGen += 1;
  if (driverObj) {
    driverObj.destroy();
    driverObj = null;
  }
  startSegment();
}

function retreatToSegment(targetIndex, targetRoute) {
  geometryBinding.stop();
  store.retreatSegment(targetIndex);
  rail.restore();
  restoreReveal();
  // Старый driver уничтожаем; его отложенный onDestroyed обезврежен driverGen.
  if (driverObj) {
    driverObj.destroy();
    driverObj = null;
  }
  return pushTourRoute(targetRoute);
}

function finishTour() {
  if (!stepSet.isKnown()) return stepSet.failUnknown();
  driverObj?.obCancel?.();
  // Дошли до финала. Кнопка «Готово» и крестик приводят в один и тот же destroy,
  // поэтому исход помечаем флагом ДО него - иначе handleDestroyed не отличит
  // досмотренный тур от брошенного и «Пройден» не выставится никогда.
  reachedFinal = true;
  // Затухаем, затем destroy(): он синхронно зовёт onDestroyed -> handleDestroyed
  // (pendingSegment=false) -> markCompleted (если авто) + stop. driverObj обнуляем
  // сразу, чтобы teardown по stop не дёрнул второй destroy. Без инстанса - напрямую.
  if (driverObj) {
    const d = driverObj;
    driverObj = null;
    fadeAndDestroy(d, prefersReducedMotion());
  } else {
    rail.restore();
    restoreReveal();
    markIfAuto();
    store.stop();
  }
}

/**
 * Отметка о туре. Авто-тур помечаем при любом закрытии - иначе он всплывал бы
 * при каждом входе; ручной запуск отметку ставит только дойдя до финала.
 *
 * @param {boolean} [finished] тур доведён до финального шага. Пропуск и Esc
 *   гасят автозапуск, но «Пройден» в меню не дают - человек тура не видел.
 */
function markIfAuto(finished = reachedFinal) {
  if (technicalStop || recovery.hasFailure()) return;
  if (!store.isManual || finished) store.markCompleted(finished);
}

function handleDestroyed(gen) {
  // Игнорируем callback от инстанса, который уже сменён следующим сегментом.
  if (gen !== driverGen) return;
  watchers.stopAll();
  driverObj = null;
  // Переход между страницами: тур продолжается, не останавливаем и рельс не трогаем.
  if (store.pendingSegment) return;
  rail.restore();
  restoreReveal();
  markIfAuto();
  store.stop(reachedFinal);
}

function teardown() {
  routeRequest += 1;
  keys.detach();
  driverObj?.obCancel?.();
  transitions.cancel();
  geometryBinding.stop();

  watchers.stopAll();
  segmentRange = null;
  // Тур ещё жив (logout/unmount во время прохождения) - авто-тур помечаем
  // пройденным здесь: ниже driverGen++ обезвредит отложенный onDestroyed, и тот
  // до markIfAuto уже не дойдёт. Так автозапуск действительно "один раз".
  if (driverObj) markIfAuto();
  driverGen += 1;
  if (driverObj) {
    // Затухаем перед удалением DOM; driverGen уже увеличен - отложенный
    // onDestroyed обезврежен (gen-гард в handleDestroyed).
    const d = driverObj;
    driverObj = null;
    fadeAndDestroy(d, prefersReducedMotion());
  }
  store.clearPending();
  rail.restore();
  restoreReveal();
  // Тур окончен/прерван - снять демо-вложение (BlankSelector уберёт его из формы).
  store.setDemoAttachment(null);
}

watch(
  () => store.isActive,
  (active) => {
    // Фоновые подсказки на время тура придерживаем: они всплывают поверх поповера
    // и сбивают с шага. Ошибки сквозь паузу проходят - см. deletions.notify.
    ui.tourActive = active;
    if (active) {
      technicalStop = false; recovery.reset();
      layerGuard.activate();
      // Каждый запуск начинается «недосмотренным»: иначе повторный тур унаследовал
      // бы отметку о финале предыдущего и закрытие на первом шаге зачлось бы.
      reachedFinal = false;
      if (!layerGuard.isBlocked()) { keys.attach(); startSegment(); }
    } else { layerGuard.deactivate(); keys.detach(); teardown(); recovery.reset(); }
  },
  { flush: 'sync' },
);

// Подхват следующего сегмента после cross-page навигации: ждём, пока роутер
// приведёт нас на страницу первого шага следующего сегмента.
const removeAfterEach = router.afterEach((to, from, failure) => {
  if (failure || !store.isActive) return;
  if (['Error500', 'Maintenance', 'Forbidden', 'NotFound', 'NotFoundCatchAll'].includes(to.name)) {
    technicalStop = true;
    recovery.interrupt();
    return;
  }
  if (!store.pendingSegment) {
    // Человек ушёл со страницы сам - пунктом меню, ссылкой, кнопкой «Назад»
    // браузера. Шаги остаются от прежней страницы, и тур висит поверх чужого
    // экрана, подсвечивая то, чего здесь нет. Продолжать негде - завершаем.
    if (store.currentStep?.route !== to.path) store.stop();
    return;
  }
  // clearPending до startSegment страхует от повторного resume (redirect-цепочка);
  // logout посреди перехода успел бы сбросить pending через teardown - тогда сюда не войдём.
  if (store.currentStep?.route === to.path) {
    store.clearPending();
    startSegment();
  } else {
    // A redirect is not a viewed segment and cannot produce a successful finale.
    recovery.fail({ stepId: store.currentStep?.id, reason: 'route-unavailable' });
  }
});

/**
 * Автозапуск один раз для любого первого входа: на «Обзоре», если юзер авторизован
 * и профильный тур ещё не пройден. Запускается РОВНО ОДИН тур - самый приоритетный
 * из доступных и непройденных (pickAutostartTour); остальные доступные человек
 * берёт вручную из меню «Обучение», иначе первый вход превратился бы в очередь из
 * пяти туров подряд.
 *
 * Статус per-user/per-tour тянется с бэкенда (loadStatus) - на ошибке сети не
 * автозапускаем (statusLoaded остаётся false). Повторно не сработает: версия тура
 * ставится при любом завершении авто-тура (см. markIfAuto), а на бэкенде статус
 * per-user, сброс только админом.
 */
const maybeAutostart = createAutostart(store, () => route.path);

watch(() => route.path, maybeAutostart);

onMounted(maybeAutostart);

onBeforeUnmount(() => {
  stepSet.dispose();
  removeAfterEach();
  teardown();
  recovery.reset();
  geometryBinding.dispose();
  layerGuard.dispose();
});
</script>

<template>
  <div
    style="display: none"
    data-testid="ob-host"
  />
</template>
