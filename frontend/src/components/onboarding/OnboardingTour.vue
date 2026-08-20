<script setup>
import { watch, onMounted, onBeforeUnmount } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useOnboardingStore } from '@/stores/onboarding';
import { useUiStore } from '@/stores/ui';
import { useOnboarding, STEP_DEMO_FALLBACK } from '@/composables/useOnboarding';
import { collectSegment, indexAfterRoute } from '@/components/onboarding/stepsFlow';
import { applyReveal, restoreReveal } from '@/components/onboarding/reveal';
import { createStepWatchers } from '@/components/onboarding/stepWatchers';

const store = useOnboardingStore();
const ui = useUiStore();
const route = useRoute();
const router = useRouter();
const { waitForElement, ensureInView, createDriver, prefersReducedMotion } = useOnboarding();

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
// Цель, которую надо СНАЧАЛА раскрыть (карточка заявки, панель поиска), ждёт ещё
// дольше: сперва приходит список своим запросом, и только потом владелец узла
// открывает карточку. На staging этой цепочки не хватало четырёх секунд - сегмент
// карточки выбрасывался целиком.
const REVEAL_TARGET_TIMEOUT = 9000;

/**
 * Сколько ждать цель шага, которому нужно раскрытие узла.
 *
 * @param {{ reveal?: { open?: string } }} step
 * @returns {number}
 */
function targetTimeoutFor(step) {
  return step?.reveal?.open ? REVEAL_TARGET_TIMEOUT : FIRST_TARGET_TIMEOUT;
}

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
// Наблюдение за DOM на время шага: ведёт шаг вперёд по действию человека и
// удерживает подсветку на пересозданной цели (stepWatchers.js).
const watchers = createStepWatchers({
  getDriver: () => driverObj,
  getGen: () => driverGen,
  getStep: (index) => store.steps[index],
  getIndex: () => store.currentIndex,
});
// Границы поднятого сейчас сегмента (глобальные индексы, включительно). Судить о
// принадлежности шага сегменту по одному только route нельзя: тур возвращается на
// ту же страницу несколькими сегментами - у охранника «Доступные мне» идут и до
// таблицы поста, и после неё, финалом.
let segmentRange = null;

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
  const next = store.steps[globalIndex]?.demoAttachment || null;
  const changed = store.demoAttachmentType !== next;
  store.setDemoAttachment(next);
  return changed;
}

/**
 * Что ждём перед показом шага. По умолчанию - сам подсвечиваемый элемент, но шаг
 * может задать `waitFor` отдельно: у формы заявки один и тот же узел обслуживает
 * оба бланка, и ждать «форму вообще» мало - иначе тур подсвечивает её, пока в ней
 * ещё прежний бланк (шаг «Сотрудники» показывал форму автомобилей).
 *
 * @param {{ element?: string|null, waitFor?: string }} step
 * @returns {string}
 */
function waitSelectorOf(step) {
  return step?.waitFor || step?.element;
}

/**
 * Пересчитать подсветку после того, как сменившийся бланк перерисовал форму.
 * Ждём НОВЫЙ узел по тому же селектору и зовём driver.refresh(): без этого
 * подсветка остаётся на удалённом элементе, то есть пропадает.
 *
 * @param {number} globalIndex шаг, чью цель ждём
 * @param {number} gen поколение driver-инстанса на момент запроса
 */
async function refreshHighlightFor(globalIndex, gen) {
  const step = store.steps[globalIndex];
  if (!step?.element) return;
  const el = await waitForElement(waitSelectorOf(step), FIRST_TARGET_TIMEOUT);
  // Тур мог уйти дальше или перезапуститься, пока форма перерисовывалась.
  if (!el || !driverObj || gen !== driverGen || store.currentIndex !== globalIndex) return;
  // refresh здесь бесполезен: он пересчитывает рамку по цели, которую driver
  // запомнил при создании сегмента. Нужен именно пересбор конфига шага - его
  // и делает obRetarget.
  driverObj.obRetarget(globalIndex);
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
  // Тур уже двинулся - наблюдение прошлого шага снимаем сразу. Иначе оно
  // срабатывало на узле, который открывает СЛЕДУЮЩИЙ шаг (карточку заявки), и
  // тур перескакивал через него.
  watchers.stopAll();
  const step = store.steps[globalIndex];
  const attachmentChanged = applyDemoAttachment(globalIndex);
  const revealed = await applyReveal(store.steps, globalIndex);
  if (!step?.element) return true;
  // Опциональный шаг ждём коротко: к этому моменту форма и field-config уже
  // отрисованы на предыдущем шаге, так что отсутствие элемента (доп.полей нет)
  // определяется быстро - не держим пользователя на «Далее». Обязательный шаг
  // ждём дольше (данным/демо-форме нужно время появиться).
  // Полный таймаут ждём, только когда на этом шаге ЧТО-ТО раскрывали или меняли
  // бланк: узел въезжает анимацией и за 700 мс не поспевает (#1771). Когда узел
  // давно открыт - ждать нечего, отсутствие цели значит «её тут нет». Разница
  // видна на карточке заявки: там подряд идут необязательные кнопки, и по 4 с на
  // каждую превращались в «нажал Далее, а ничего не происходит».
  const needsLongWait = revealed || attachmentChanged || !step.optional;
  const timeout = needsLongWait ? targetTimeoutFor(step) : 700;
  const el = await waitForElement(waitSelectorOf(step), timeout);
  if (el) {
    // Подсвечиваем ровно то, что человек видит: длинная форма заявки остаётся
    // прокрученной от прошлого шага, и цель могла уехать за край экрана.
    await ensureInView(step.element ? document.querySelector(step.element) : el, step.scrollTo);
    return true;
  }
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
  segmentRange = { start: segmentStartIndex, end: segmentStartIndex + segmentSteps.length - 1 };
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
    // Первый шаг новой страницы ждём по верхней границе: страница монтируется,
    // грузит данные и только потом дорастает до конечного размера. На таблице
    // поста четырёх секунд не хватало - шаг вырождался в окно по центру, хотя
    // таблица появлялась мгновением позже.
    const el = await waitForElement(waitSelectorOf(targetStep), REVEAL_TARGET_TIMEOUT, waitController.signal);
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
      watchers.watchStep(globalIndex);
      // Backstop: синхронизируем демо-вложение с подсвеченным шагом (важно для
      // навигации «Назад» - prepareStep отрабатывает только на «Далее»).
      const attachmentChanged = applyDemoAttachment(globalIndex);
      // Смена бланка пересоздаёт форму: driver.js держит ссылку на прежний узел,
      // и после «Назад» с «Сотрудников» на «Автомобили» подсветка пропадала -
      // форма правильная, а рамки нет. Дожидаемся нового узла и просим driver
      // пересчитать подсветку по тому же селектору.
      if (attachmentChanged) refreshHighlightFor(globalIndex, driverGen);
      // Держит раскрытый узел открытым, пока «Назад» ходит внутри группы шагов с
      // одинаковым reveal (prepareStep этот путь не покрывает - он гейтит только
      // «Далее»). Эксклюзивность внутри applyReveal закрывает чужой узел.
      // Не await - фоновый прогрев.
      applyReveal(store.steps, globalIndex);
    },
    onBeforeStep: prepareStep,
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
  if (step.route !== route.path) {
    retreatToSegment(globalIndex, step.route);
    return;
  }
  // Шаг на этой же странице, но в другом сегменте: driver знает только шаги
  // поднятого сегмента, и obGoTo для такого индекса молча ничего не делал - в
  // туре заявителя прыжок на финал из списка шагов не срабатывал вовсе.
  // Навигации тут не будет (страница та же), поэтому сегмент поднимаем сами.
  if (!isInActiveSegment(globalIndex)) {
    restartSegmentAt(globalIndex);
    return;
  }
  const gen = driverGen;
  applyRail(globalIndex);
  const ready = await prepareStep(globalIndex);
  if (!driverObj || gen !== driverGen) return;
  driverObj.obGoTo(globalIndex, ready === false || ready === STEP_DEMO_FALLBACK);
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
  store.setIndex(globalIndex);
  restoreRail();
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
  watchers.stopAll();
  driverObj = null;
  // Переход между страницами: тур продолжается, не останавливаем и рельс не трогаем.
  if (store.pendingSegment) return;
  restoreRail();
  restoreReveal();
  markIfAuto();
  store.stop();
}

function teardown() {
  watchers.stopAll();
  segmentRange = null;
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
    // Фоновые подсказки на время тура придерживаем: они всплывают поверх поповера
    // и сбивают с шага. Ошибки сквозь паузу проходят - см. deletions.notify.
    ui.tourActive = active;
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
