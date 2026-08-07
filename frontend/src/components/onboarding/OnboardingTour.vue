<script setup>
import { watch, onMounted, onBeforeUnmount } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useOnboardingStore } from '@/stores/onboarding';
import { useUiStore } from '@/stores/ui';
import { useOnboarding, STEP_DEMO_FALLBACK } from '@/composables/useOnboarding';
import { collectSegment, indexAfterRoute } from '@/components/onboarding/onboardingSteps';
import { applyReveal, restoreReveal } from '@/components/onboarding/reveal';

const store = useOnboardingStore();
const ui = useUiStore();
const route = useRoute();
const router = useRouter();
const { waitForElement, createDriver, prefersReducedMotion } = useOnboarding();

/**
 * Плавное закрытие тура: driver.js делает только fade-IN, а на destroy убирает
 * overlay и поповер мгновенно (рывок). Навешиваем класс затухания на оба
 * элемента и удаляем DOM уже после анимации. Только для ЗАВЕРШЕНИЯ тура
 * (финал/Esc/крестик/пропуск) - не для переходов между страницами.
 *
 * @param {import('driver.js').Driver} driverInstance
 */
function fadeAndDestroy(driverInstance) {
  const els = [
    document.querySelector('.driver-overlay'),
    document.querySelector('.driver-popover'),
  ].filter(Boolean);
  if (!els.length || prefersReducedMotion()) {
    driverInstance.destroy();
    return;
  }
  els.forEach((el) => el.classList.add('ob-fade-out'));
  setTimeout(() => driverInstance.destroy(), 240);
}

// Первую цель сегмента ждём дольше при cross-page: после router.push страница
// монтируется и грузит данные (скелетоны), цель появляется не сразу.
const FIRST_TARGET_TIMEOUT = 4000;

let driverObj = null;
let waitController = null;
// Поколение активного driver-инстанса: отложенный onDestroyed предыдущего
// сегмента (анимация ухода ~0.4s) не должен трогать уже поднятый следующий.
let driverGen = 0;
// Тур дошёл до финального шага (кнопка «Готово» или CTA), а не был брошен на
// середине. Сбрасывается при старте каждого тура - см. watch(isActive).
let reachedFinal = false;
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

/**
 * Демо-вложение оформления заявки: шаги «Бланк Автомобили/Сотрудники» просят
 * BlankSelector добавить вложение нужного типа, чтобы справа отрисовалась
 * реальная форма. Шаги без demoAttachment - убираем демо (null).
 */
function applyDemoAttachment(globalIndex) {
  store.setDemoAttachment(store.steps[globalIndex]?.demoAttachment || null);
}

/**
 * Перед переходом на шаг готовим DOM: ставим демо-вложение и дожидаемся
 * появления целевого элемента - иначе driver подсветит пустоту, если данные
 * ещё грузятся (баг: шаг всплывал раньше, чем элемент попадал в DOM).
 *
 * Опциональный шаг (`optional`, напр. доп.поля «при наличии»): если элемента
 * нет за короткий таймаут - возвращаем false, и onNextClick пропускает шаг.
 *
 * Шаг с демо-скриншотом (`demo`) не пропускаем никогда: у нового пользователя
 * система пуста (ни заявок, ни вложений), и молчаливый пропуск отнимал бы у него
 * ровно то, ради чего тур и заведён. Вместо подсветки показываем скриншот - об
 * этом и говорит STEP_DEMO_FALLBACK.
 *
 * @param {number} globalIndex
 * @returns {Promise<boolean|string>} false = пропустить шаг, STEP_DEMO_FALLBACK =
 *   показать без подсветки со скриншотом, true = вести шаг как обычно
 */
async function prepareStep(globalIndex) {
  const step = store.steps[globalIndex];
  applyDemoAttachment(globalIndex);
  await applyReveal(store.steps, globalIndex);
  if (!step?.element) return true;
  // Опциональный шаг ждём коротко: к этому моменту форма и field-config уже
  // отрисованы на предыдущем шаге, так что отсутствие элемента (доп.полей нет)
  // определяется быстро - не держим пользователя на «Далее». Обязательный шаг
  // ждём дольше (данным/демо-форме нужно время появиться).
  const timeout = step.optional ? 700 : FIRST_TARGET_TIMEOUT;
  const el = await waitForElement(step.element, timeout);
  if (el) return true;
  if (step.demo) return STEP_DEMO_FALLBACK;
  return step.optional ? false : true;
}

async function startSegment() {
  const myGen = ++driverGen;
  // Берём сегмент, СОДЕРЖАЩИЙ текущий шаг, а не обязательно начинающийся с него:
  // при cross-page «Назад» мы попадаем на ПОСЛЕДНИЙ шаг предыдущей страницы, и
  // нужно поднять весь её сегмент, чтобы внутри него работала навигация туда-сюда.
  let segmentStartIndex = store.currentIndex;
  while (segmentStartIndex > 0 && store.steps[segmentStartIndex - 1].route === route.path) {
    segmentStartIndex -= 1;
  }
  const segmentSteps = collectSegment(store.steps, segmentStartIndex, route.path);
  if (!segmentSteps.length) {
    store.stop();
    return;
  }
  const localTarget = store.currentIndex - segmentStartIndex;

  // Демо-вложение и рельс целевого шага ставим ДО ожидания элемента: форма
  // заявки рисуется только при добавленном вложении, а рельс-шаг должен мерить
  // уже раскрытый рельс (актуально при «Назад» прямо на такой шаг).
  applyDemoAttachment(store.currentIndex);
  applyRail(store.currentIndex);
  await applyReveal(store.steps, store.currentIndex);

  // Целевой шаг: дожидаемся его элемента (устойчивого по размеру), иначе
  // деградируем в центр-модал, чтобы driver не падал на отсутствующей цели.
  // Сам массив шагов не трогаем - он же служит источником, из которого движок
  // пересобирает шаг, когда цель появляется или пропадает (setStepMode).
  const targetStep = segmentSteps[localTarget];
  let targetMissing = false;
  if (targetStep.element) {
    waitController = new AbortController();
    const el = await waitForElement(targetStep.element, FIRST_TARGET_TIMEOUT, waitController.signal);
    waitController = null;
    // Тур могли остановить или перезапустить (Esc/logout/новый сегмент) пока
    // ждали элемент - не поднимаем driver-зомби поверх неактивного/чужого тура.
    if (!store.isActive || myGen !== driverGen) return;
    targetMissing = !el;
  }

  driverObj = createDriver(segmentSteps, {
    startIndex: segmentStartIndex,
    fallbackIndex: targetMissing ? localTarget : -1,
    onIndexChange: (globalIndex) => {
      store.setIndex(globalIndex);
      applyRail(globalIndex);
      // Backstop: синхронизируем демо-вложение с подсвеченным шагом (важно для
      // навигации «Назад» - prepareStep отрабатывает только на «Далее»).
      applyDemoAttachment(globalIndex);
      // Держит раскрытый узел открытым, пока «Назад» ходит внутри группы шагов с
      // одинаковым reveal (prepareStep этот путь не покрывает - он гейтит только
      // «Далее»). Эксклюзивность внутри applyReveal закрывает чужой узел.
      // Не await - фоновый прогрев.
      applyReveal(store.steps, globalIndex);
    },
    onBeforeStep: prepareStep,
    onBoundaryNext: handleBoundaryNext,
    onBoundaryPrev: handleBoundaryPrev,
    onCtaClick: finishWithCta,
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
 * CTA финала: завершаем тур и ведём на целевой раздел шага. Applicant-финал не
 * задаёт ctaRoute -> дефолт «оформить заявку»; security-финал ведёт в «Доступные
 * мне» (ctaRoute), а не на подачу заявки.
 *
 * Навигацию пропускаем, если уже на целевом роуте: security-финал показывается
 * прямо на /accessible-attachments, и повторный push того же пути зря
 * перезапускал бы navigation guard (а под кратковременно «протухшим» токеном
 * guard может отбросить на /personal-cabinet). CTA тогда просто завершает тур.
 *
 * @param {string} [ctaRoute] route из ctaRoute финального шага
 */
function finishWithCta(ctaRoute) {
  finishTour();
  const target = ctaRoute || '/new-application';
  if (route.path !== target) router.push(target).catch(() => {});
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
  restoreReveal();
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

/**
 * Пользователь нажал "Назад" на ПЕРВОМ шаге сегмента: предыдущий шаг - на другой
 * странице, возвращаемся туда и показываем последний шаг того сегмента.
 *
 * @param {number} segmentStartGlobal глобальный индекс первого шага текущего сегмента
 */
function handleBoundaryPrev(segmentStartGlobal) {
  const prevIdx = segmentStartGlobal - 1;
  const prevStep = store.steps[prevIdx];
  if (!prevStep) return;
  if (prevStep.route !== route.path) {
    retreatToSegment(prevIdx, prevStep.route);
  } else if (driverObj) {
    driverObj.movePrevious();
  }
}

function retreatToSegment(targetIndex, targetRoute) {
  store.retreatSegment(targetIndex);
  restoreRail();
  restoreReveal();
  // Старый driver уничтожаем; его отложенный onDestroyed обезврежен driverGen.
  if (driverObj) {
    driverObj.destroy();
    driverObj = null;
  }
  router.push(targetRoute).catch(() => {
    if (store.pendingSegment) {
      store.clearPending();
      store.stop();
    }
  });
}

function finishTour() {
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
    fadeAndDestroy(d);
  } else {
    restoreRail();
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
  if (!store.isManual || finished) store.markCompleted(finished);
}

function handleDestroyed(gen) {
  // Игнорируем callback от инстанса, который уже сменён следующим сегментом.
  if (gen !== driverGen) return;
  driverObj = null;
  // Переход между страницами: тур продолжается, не останавливаем и рельс не трогаем.
  if (store.pendingSegment) return;
  restoreRail();
  restoreReveal();
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
    // Затухаем перед удалением DOM; driverGen уже увеличен - отложенный
    // onDestroyed обезврежен (gen-гард в handleDestroyed).
    const d = driverObj;
    driverObj = null;
    fadeAndDestroy(d);
  }
  store.clearPending();
  restoreRail();
  restoreReveal();
  // Тур окончен/прерван - снять демо-вложение (BlankSelector уберёт его из формы).
  store.setDemoAttachment(null);
}

watch(
  () => store.isActive,
  (active) => {
    if (active) {
      // Каждый запуск начинается «недосмотренным»: иначе повторный тур унаследовал
      // бы отметку о финале предыдущего и закрытие на первом шаге зачлось бы.
      reachedFinal = false;
      startSegment();
    } else teardown();
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
    // Сегмент с динамическим route (фактовая таблица) опционален: если у
    // охранника пока нет доступа к /table/:name и роут-гард редиректит, это не
    // обрыв тура. Вместо завершения перепрыгиваем весь недостижимый сегмент к
    // следующему достижимому шагу - финалу-празднованию на /accessible-attachments,
    // чтобы охранник всё равно увидел завершение. Если за сегментом шагов нет -
    // штатно завершаем (авто-тур помечаем пройденным). Когда доступ выдан, переход
    // проходит и мы попадаем в ветку выше.
    const missed = store.currentStep;
    if (missed?.optionalSegment) {
      const resumeIndex = indexAfterRoute(store.steps, store.currentIndex, missed.route);
      if (resumeIndex !== -1) {
        // route фиксируем до jumpToSegment - индекс смены сегмента читаем один раз.
        const resumeRoute = store.steps[resumeIndex].route;
        store.jumpToSegment(resumeIndex);
        router.push(resumeRoute).catch(() => {
          store.clearPending();
          markIfAuto();
          store.stop();
        });
        return;
      }
      markIfAuto();
    }
    store.clearPending();
    store.stop();
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
async function maybeAutostart() {
  if (store.isActive) return;
  if (route.path !== '/news') return;
  if (!store.canShowTour) return;
  // Права, тип пользователя и роль в согласовании гейтят туры и приезжают своими
  // запросами - без ожидания автозапуск выбрал бы тур из неполного списка доступных.
  const pending = [store.ensureGatingContext()];
  if (!store.statusLoaded) pending.push(store.loadStatus());
  await Promise.all(pending);
  // Перепроверяем после await: статус мог не загрузиться, юзер мог уйти/стартовать,
  // а гейт согласия - доехать ответом и закрыть показ тура (#1567).
  if (!store.statusLoaded || store.isActive || route.path !== '/news') return;
  if (!store.canShowTour) return;
  const tour = store.pickAutostartTour();
  if (!tour) return;
  store.start({ tour: tour.key, manual: false });
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
