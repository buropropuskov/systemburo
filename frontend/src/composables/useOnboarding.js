import { createStepNavigation } from '@/components/onboarding/stepNavigation';
import { driver } from 'driver.js';
import { createStepTransition } from '@/components/onboarding/stepTransition';
import { waitForElement, ensureInView } from '@/components/onboarding/targetWaits';
import { findTourTarget } from '@/components/onboarding/targetResolver';
import 'driver.js/dist/driver.css';
import { sanitizeHtml } from '@/utils/sanitize.js';
import { useOnboardingStore } from '@/stores/onboarding';
import { getDemo } from '@/components/onboarding/onboardingDemo';
import { buildProgressBlock } from '@/components/onboarding/popoverProgress';
import { isChapterEnd } from '@/components/onboarding/stepsFlow';
import { STAGE_PADDING, STAGE_RADIUS, applyStageShape, raiseActiveHighlight } from '@/components/onboarding/stageShape';

/**
 * Телефонная раскладка: форма и таблицы в одну колонку, поповер тура обязан
 * ложиться вниз, а не сбоку. Порог тот же, что у CSS (`max-width: 767.98px`),
 * и 768 в него НЕ входит: это портретный iPad, там раскладка планшетная.
 * За «меню сейчас в drawer» отвечает отдельный признак - `isDrawerViewport`
 * в `constants/breakpoints.js`: drawer держится до 1366, и путать эти два
 * условия нельзя, иначе тур на планшете подсветит пустоту вместо пункта меню.
 *
 * @returns {boolean}
 */
export function isMobileViewport() {
  return typeof window !== 'undefined' && window.innerWidth < 768;
}

/**
 * Ответ `onBeforeStep`, когда цели шага на экране нет, но у шага есть демо-скриншот:
 * шаг не пропускаем, а показываем центр-модалом с картинкой вместо подсветки.
 */
export const STEP_DEMO_FALLBACK = 'demo-fallback';

/**
 * Показывать ли на шаге демо-скриншот. Картинка заменяет ЖИВОЙ экран, поэтому
 * рисуется только у шага без цели (движок уже снял `element` - подсвечивать
 * нечего). Рядом с подсвеченным реальным элементом она была бы дублем.
 *
 * @param {{ demo?: string, element?: string|null }} step
 * @returns {boolean}
 */
export function showsDemo(step) {
  return Boolean(step?.demo) && !step?.element;
}

/**
 * Может ли шаг вовсе исчезнуть из тура. Опциональный шаг С демо-скриншотом не
 * исчезает никогда: без цели он показывается центр-модалом с картинкой.
 *
 * Нумерацию по этому признаку НЕ считаем: пометка `optional` говорит «элемента
 * может не быть», а не «его нет». На карточке заявки optional весь сегмент, и
 * счётчик замирал на девять шагов подряд («Шаг 16 из 32» девять раз). Считаем по
 * фактически пройденному маршруту - см. countShownSteps.
 *
 * @param {{ optional?: boolean, demo?: string }} step
 * @returns {boolean}
 */
export function isSkippableStep(step) {
  return Boolean(step?.optional) && !step?.demo;
}

/**
 * Нумерация «Шаг N из M». Номер растёт на каждом ПОКАЗАННОМ шаге - выброшенные
 * дырок в нём не оставляют. А знаменатель берём как есть, на весь набор, и по
 * ходу тура не трогаем.
 *
 * Раньше из знаменателя вычитались выброшенные шаги, и человек видел, как число
 * тает на глазах: «из 57» превращалось в «из 55», потом в «из 48». Состав набора
 * решается до старта (права, данные - см. gatingData.js), а редкие шаги вроде
 * «организация на проверке» зависят от конкретной заявки, и предсказать их
 * нельзя. Пусть лучше тур закончится на «Шаг 11 из 13» - это читается как «два
 * шага не понадобились», - чем знаменатель будет меняться под рукой.
 *
 * @param {Array<object>} steps все шаги тура
 * @param {number} currentIndex глобальный индекс текущего шага
 * @param {Array<number>|Set<number>} skipped индексы выброшенных шагов
 * @returns {{ index: number, total: number }}
 */
export function countShownSteps(steps, currentIndex, skipped) {
  const dropped = skipped instanceof Set ? skipped : new Set(skipped || []);
  let index = 0;
  for (let i = 0; i <= currentIndex && i < steps.length; i += 1) {
    if (!dropped.has(i)) index += 1;
  }
  return { index, total: steps.length };
}

/**
 * Тонкая обёртка над driver.js: ожидание целевого элемента, сборка тела
 * поповера и конфигурация инстанса с кастомным прогресс-блоком в футере.
 */
export function useOnboarding() {
  /**
   * @returns {boolean} пользователь просит уменьшить анимации.
   */
  function prefersReducedMotion() {
    try {
      return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    } catch {
      return false;
    }
  }


  /**
   * Тело поповера: текст шага + демо-скриншот, если шаг показывается без живой
   * цели (см. showsDemo). Весь HTML прогоняется через sanitizeHtml (src/alt/caption -
   * наши статичные значения, санитайз страхует от случайной разметки в тексте).
   *
   * @param {{ description: string, demo?: string, element?: string|null }} step
   * @returns {string}
   */
  /**
   * Подставить в текст шага живые значения с экрана: `{имя}` заменяется на текст
   * узла из `step.dynamic.имя`. Нужно там, где название задаёт не система, а Бюро:
   * шаг про смену бланка говорил «выбран другой бланк», и понять, какой именно,
   * было нельзя - имена бланков у каждой организации свои.
   *
   * @param {string} text
   * @param {Record<string, string>|undefined} dynamic карта «имя -> селектор»
   * @returns {string}
   */
  function fillDynamic(text, dynamic) {
    if (!text || !dynamic) return text;
    return Object.entries(dynamic).reduce((acc, [key, selector]) => {
      const value = document.querySelector(selector)?.textContent?.trim();
      // Узла нет - оставляем текст как есть, но без плейсхолдера: лучше общая
      // фраза, чем фигурные скобки в лицо пользователю.
      return acc.replaceAll(`{${key}}`, value || 'выбранный');
    }, text);
  }

  function buildPopoverHtml(step) {
    let html = fillDynamic(step.description || '', step.dynamic);
    const demo = showsDemo(step) ? getDemo(step.demo) : null;
    if (demo) {
      const caption = demo.caption
        ? `<figcaption class="ob-popover__demo-caption">${demo.caption}</figcaption>`
        : '';
      html += `<figure class="ob-popover__demo"><img class="ob-popover__demo-img" src="${demo.src}" alt="${demo.alt}" />${caption}</figure>`;
    }
    return sanitizeHtml(html);
  }


  /**
   * Кнопка выхода. На последнем шаге главы зовём её иначе: там обучение прерывать
   * не жалко - глава дочитана, и вернуться можно ровно сюда же. В середине главы
   * это по-прежнему «пропустить», чтобы человек не бросал раздел на середине.
   */
  function buildSkipButton(onSkip, atChapterEnd) {
    const skip = document.createElement('button');
    skip.type = 'button';
    skip.className = 'ob-popover__skip';
    skip.textContent = atChapterEnd ? 'Продолжить позже' : 'Пропустить обучение';
    skip.setAttribute('data-testid', atChapterEnd ? 'ob-pause' : 'ob-skip');
    skip.addEventListener('click', onSkip);
    return skip;
  }

  /** Сдержанное празднование финала: галочка в круге (scale-in через CSS). */
  function buildCelebrate() {
    const wrap = document.createElement('div');
    wrap.className = 'ob-popover__celebrate';
    const circle = document.createElement('div');
    circle.className = 'ob-popover__check';
    // Статичная иконка-константа, пользовательских данных нет - innerHTML безопасен.
    circle.innerHTML = '<svg width="26" height="26" viewBox="0 0 24 24" fill="none"><path d="M5 12.5l4.5 4.5L19 7" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"/></svg>';
    wrap.appendChild(circle);
    return wrap;
  }

  /**
   * Сконфигурировать driver-инстанс для одного сегмента (подряд идущих шагов
   * с общим route).
   *
   * @param {Array<object>} stepsForSegment
   * @param {{ startIndex?: number, fallbackIndex?: number, onIndexChange?: (globalIndex: number) => void, onDestroyed?: () => void, onBoundaryNext?: () => void, onBoundaryPrev?: (segmentStartGlobal: number) => void, onCloseRequest?: () => void, onJumpTo?: (globalIndex: number) => void }} [options]
   * @returns {import('driver.js').Driver}
   */
  function createDriver(stepsForSegment, { startIndex = 0, fallbackIndex = -1, onIndexChange, onDestroyed, onBoundaryNext, onBoundaryPrev, onCloseRequest, onBeforeStep, onJumpTo, runTransition } = {}) {
    const store = useOnboardingStore();
    const lastLocal = stepsForSegment.length - 1;
    let alive = true;
    const tourKey = store.activeTourKey;
    const transitions = createStepTransition({ isActive: () => alive });
    const navigate = (work, direction) => (runTransition || transitions.run)(async (operation) => {
      if (!alive || !operation.isCurrent()) return;
      const sourceIndex = driverObj.getActiveIndex() ?? 0;
      const sourceId = stepsForSegment[sourceIndex]?.id;
      await work({
        signal: operation.signal,
        isCurrent: () => alive && operation.isCurrent() && store.activeTourKey === tourKey
          && store.steps[startIndex + sourceIndex]?.id === sourceId
          && driverObj.getActiveIndex() === sourceIndex,
      });
    }, { intent: { tourKey, stepId: stepsForSegment[driverObj.getActiveIndex() ?? 0]?.id, direction } });


    /**
     * Снять класс подсветки со всего, что не является текущей целью.
     *
     * driver.js помечает элемент классом сразу, а снимает со старого только когда
     * его внутреннее `__activeElement` доедет - при перепривязке
     * и быстрых переходах этого не происходит, и над затемнением остаются торчать
     * прежние цели, пока driver доводит свою анимацию.
     */
    function dropStaleHighlights(activeEl) {
      const localIndex = driverObj?.getActiveIndex?.() ?? 0;
      const selector = stepsForSegment[localIndex]?.element;
      // Активную цель берём из хука, от driver и по селектору шага: на первом шаге
      // сегмента внутреннее состояние driver ещё не обновлено, и чистка по нему
      // сняла бы класс с только что подсвеченного элемента.
      const active = activeEl
        || driverObj?.getActiveElement?.()
        || (selector ? findTourTarget(selector) : null);
      if (!active) return;
      document.querySelectorAll('.driver-active-element, .ob-highlighted').forEach((el) => {
        if (el === active) return;
        el.classList.remove('driver-active-element', 'driver-no-interaction', 'ob-highlighted');
      });
    }

    /**
     * Шаг в формате driver.js. Пересобирается, когда шаг переключается между
     * подсветкой цели и видом без неё (setStepMode), поэтому сборка одна на оба
     * случая - иначе два вида разъехались бы по оформлению.
     *
     * @param {object} s шаг тура в том виде, в котором показываем (element уже снят, если цели нет)
     * @param {number} li локальный индекс внутри сегмента
     * @returns {import('driver.js').DriveStep}
     */
    function buildDriverStep(s, li) {
      const popover = {
        title: fillDynamic(s.title, s.dynamic),
        description: buildPopoverHtml(s),
      };
      // Сторона/выравнивание поповера от шага - чтобы карточка не наезжала на
      // выделенный элемент (напр. элементы шапки вверху -> поповер вниз). На
      // мобилке (<768) раскладка стековая (одна колонка) - side:'left'/'right'
      // (рассчитан на десктопный split селектор/форма или карточка/детали)
      // толкал бы поповер за край узкого экрана; принудительно кладём вниз (#1097).
      const narrow = isMobileViewport();
      if (s.side) {
        popover.side = narrow && (s.side === 'left' || s.side === 'right') ? 'bottom' : s.side;
      }
      if (s.align) popover.align = s.align;
      // Шаги с демо-скриншотом шире - чтобы реальная таблица читалась. Ширину даём
      // только когда картинка реально рисуется: без неё пустая широкая карточка.
      if (showsDemo(s)) popover.popoverClass = 'ob-popover ob-popover--wide';
      // Последний шаг сегмента, но впереди есть шаги на другой странице -
      // ТОЛЬКО подпись кнопки: "Далее" вместо "Готово", чтобы юзер видел, что
      // тур продолжится. Само поведение перехода диктует onNextClick ниже.
      if (li === lastLocal && store.steps[startIndex + li + 1]) {
        popover.nextBtnText = 'Далее';
      }
      return { element: s.element ? () => findTourTarget(s.element) : undefined, popover };
    }
    const driverSteps = stepsForSegment.map((s, li) => buildDriverStep(s, li));

    /**
     * Переключить шаг между подсветкой реальной цели и показом без неё (центр-модал,
     * со скриншотом у шагов с `demo`). driver читает config.steps на каждом переходе,
     * поэтому правка на месте действует и вперёд, и назад, и на повторном проходе.
     *
     * @param {number} localIndex
     * @param {boolean} withoutTarget цели на экране нет
     */
    function setStepMode(localIndex, withoutTarget) {
      const source = stepsForSegment[localIndex];
      if (!source) return;
      driverSteps[localIndex] = buildDriverStep(
        withoutTarget ? { ...source, element: null } : source,
        localIndex,
      );
    }

    // Цель первого показываемого шага не появилась (хост ждал её до таймаута) -
    // поднимаем сегмент сразу в виде без подсветки.
    if (fallbackIndex >= 0) setStepMode(fallbackIndex, true);

    const driverObj = driver({
      showProgress: false,
      // Родная анимация driver.js: вырез плавно едет от прежней цели к новой.
      // Два её побочных эффекта закрыты снаружи - подсветка со старой цели
      // снимается в onHighlightStarted (иначе накапливалась и над затемнением
      // торчали сразу три элемента), а новая цель поднимается над затемнением
      // только по завершении перехода (иначе протыкала его раньше выреза, и на
      // форме заявки это читалось как «сначала светятся инпуты»).
      animate: !prefersReducedMotion(),
      allowClose: true,
      // Клавиши обрабатывает хост (OnboardingTour), а не driver. Свой обработчик
      // библиотеки снимается на время переезда подсветки и подъёма следующего
      // сегмента: нажатие в эти 300-400 мс уходило в никуда, и человек видел
      // «тур завис» - жал ещё раз и только тогда двигался дальше.
      allowKeyboardControl: false,
      // Клик мимо окна шага обучение не обрывает. driver по умолчанию читает его
      // как выход, и промах мышью стоил человеку всего тура разом: авто-тур
      // после закрытия больше не всплывает. Хук-заглушка уводит driver с ветки
      // "close" (см. его обработчик оверлея), не трогая allowClose - на нём
      // держится выход по Esc, обещанный в тексте первого шага.
      overlayClickBehavior: () => {},
      // Чётче выделение: затемнение фона плотнее, скругление 30px.
      // popoverOffset больше - карточка не наезжает на элемент.
      overlayOpacity: 0.78,
      // Зазор и скругление - общие; на цели во весь экран их снимает applyStageShape.
      stagePadding: STAGE_PADDING,
      stageRadius: STAGE_RADIUS,
      popoverOffset: 16,
      popoverClass: 'ob-popover',
      nextBtnText: 'Далее',
      prevBtnText: 'Назад',
      doneBtnText: 'Готово',
      steps: driverSteps,
      // Перехватываем "Далее" (кнопка И стрелка вправо идут сюда): на границе
      // сегмента отдаём решение хосту (перейти на след. страницу / завершить),
      // иначе обычный moveNext.
      ...createStepNavigation({
        driverObj: { getActiveIndex: () => driverObj.getActiveIndex(),
          moveNext: () => driverObj.moveNext(), movePrevious: () => driverObj.movePrevious(),
          moveTo: (index) => driverObj.moveTo(index) },
        store, stepsForSegment, startIndex, lastLocal, navigate,
        onBeforeStep, onBoundaryNext, onBoundaryPrev, setStepMode, demoFallback: STEP_DEMO_FALLBACK,
      }),
      onPopoverRender(popover) {
        // Первый показ поповера в сегменте - без анимации переезда: иначе он
        // приезжал бы из левого верхнего угла, куда его ставит driver.js до
        // первого позиционирования. Дальше внутри сегмента переезды плавные.
        popover.wrapper.classList.add('ob-popover--instant');
        requestAnimationFrame(() => {
          requestAnimationFrame(() => popover.wrapper.classList.remove('ob-popover--instant'));
        });
        const localIndex = driverObj.getActiveIndex() ?? 0;
        const currentGlobal = startIndex + localIndex;
        const step = stepsForSegment[localIndex];
        // Номер считаем по пройденному маршруту: из счёта выпадают только шаги,
        // которые тур реально выбросил в этом прохождении (см. countShownSteps).
        const { index: globalIndex, total } = countShownSteps(store.steps, currentGlobal, store.skippedIndexes);
        // Подсказка "Далее" сквозная (вкл. шаг со след. страницы). Пропускаем
        // шаги, элемента которых сейчас нет в DOM и которые движок выбросит -
        // иначе хинт обещает шаг, на который тур не перейдёт.
        //
        // Судим только о шагах ЭТОЙ страницы: цели соседней страницы в текущем
        // DOM отсутствуют всегда, и цикл пробегал весь хвост тура - на последнем
        // шаге раздела «Доступные мне» подсказка обещала «Готово!», хотя дальше
        // шёл целый сегмент таблицы поста.
        const currentRoute = store.steps[currentGlobal]?.route;
        let nextIdx = currentGlobal + 1;
        let nextStep = store.steps[nextIdx];
        while (
          nextStep
          && nextStep.route === currentRoute
          && isSkippableStep(nextStep)
          && nextStep.element
          // Цель, которая появляется по действию (раскрытие окна, смена бланка),
          // сейчас отсутствует законно - шаг всё равно будет показан. Иначе
          // подсказка перепрыгивала его и обещала «Готово!» посреди тура.
          && !nextStep.reveal?.open
          && !nextStep.demoAttachment
          && !findTourTarget(nextStep.element)
        ) {
          nextIdx += 1;
          nextStep = store.steps[nextIdx];
        }
        const nextTitle = nextStep ? nextStep.title : '';

        // На первом шаге сегмента driver дизейблит "Назад"; но если впереди (вернее
        // позади) есть шаг на другой странице - раскрываем кнопку, чтобы вернуться
        // по границе (cross-page back). Само поведение - в onPrevClick.
        if (localIndex === 0 && startIndex > 0 && popover.previousButton) {
          popover.previousButton.disabled = false;
          popover.previousButton.classList.remove('driver-popover-btn-disabled');
        }

        // Финал: галочка перед заголовком + CTA после описания.
        if (step?.celebrate) {
          popover.wrapper.insertBefore(buildCelebrate(), popover.title);
        }
        // Прогресс сверху футера.
        popover.footer.insertBefore(
          buildProgressBlock({
            steps: store.steps,
            skipped: store.skippedIndexes,
            globalIndex,
            total,
            nextTitle,
            currentGlobal,
            onJump: onJumpTo,
          }),
          popover.footer.firstChild,
        );
        // "Пропустить" - первым в ряду кнопок (CSS прижимает Назад/Далее вправо).
        // На финале не показываем: там уже есть "Готово" и CTA.
        if (!step?.celebrate) {
          popover.footerButtons.insertBefore(
            buildSkipButton(
              () => (onCloseRequest ? onCloseRequest() : driverObj.destroy()),
              isChapterEnd(store.steps, currentGlobal),
            ),
            popover.footerButtons.firstChild,
          );
        }

        // Компенсация корневого zoom для позиционирования поповера - едина для ВСЕХ
        // шагов (и anchored, и центро-модальных): см. popoverZoom.js.
      },
      // Esc / клик по оверлею / крестик идут сюда (g(true)) ДО гейта на
      // __activeStep - в отличие от onDestroyed это надёжно срабатывает, даже
      // если шаг закрыт во время entry-анимации. Отдаём закрытие хосту; он
      // остановит тур (-> teardown снимет overlay и пометит авто-тур).
      onDestroyStarted() {
        if (onCloseRequest) onCloseRequest();
        else driverObj.destroy();
      },
      // Начало перехода: снимаем подсветку с прежней цели сразу. driver.js делает
      // это только в конце своей анимации, и при быстрых «Далее» пометки
      // накапливались - на разделе «Автомобили» светились три элемента разом.
      onHighlightStarted(element) {
        applyStageShape(driverObj, element);
        dropStaleHighlights(element);
      },
      onHighlighted() {
        if (!alive) return;
        dropStaleHighlights();
        raiseActiveHighlight(driverObj);
        const localIndex = driverObj.getActiveIndex() ?? 0;
        onIndexChange?.(startIndex + localIndex);
      },
      onDestroyed() {
        alive = false;
        transitions.cancel();
        // Свой класс подъёма driver не знает - снимаем сами, иначе элемент
        // останется висеть над остальной страницей после закрытия тура.
        document.querySelectorAll('.ob-highlighted').forEach((el) => el.classList.remove('ob-highlighted'));
        onDestroyed?.();
      },
    });

    // Инвалидируем подготовку сразу: fade может удалить DOM позже, но ждать
    // его окончания для отмены действий закрытого тура нельзя.
    driverObj.obCancel = () => {
      alive = false;
      transitions.cancel();
    };
    const destroy = driverObj.destroy.bind(driverObj);
    driverObj.destroy = () => {
      driverObj.obCancel();
      destroy();
    };

    driverObj.obGetGlobalIndex = () => startIndex + (driverObj.getActiveIndex() ?? 0);
    driverObj.obSyncHighlighted = () => {
      if (alive && store.activeTourKey === tourKey && driverObj.getConfig?.()?.animate === false) {
        driverObj.getConfig().onHighlighted?.();
      }
    };

    /**
     * Шаг вперёд ровно тем же путём, что и кнопка «Далее»: с подготовкой
     * следующего шага и обработкой границы сегмента. Нужен хосту, когда шаг
     * должен продвинуться сам (человек выполнил действие, о котором шаг просил).
     */
    driverObj.obNext = () => driverObj.getConfig().onNextClick?.();

    /** Шаг назад тем же путём, что кнопка «Назад» - для своей обработки стрелок. */
    driverObj.obPrev = () => driverObj.getConfig().onPrevClick?.();

    /**
     * Перейти на шаг сегмента по глобальному индексу, собрав его заново. Ходом
     * пользуются переприцеливание после смены бланка и прыжок из списка шагов.
     *
     * @param {number} globalIndex глобальный индекс шага
     * @param {boolean} [withoutTarget] показать шаг без подсветки (цели нет)
     */
    driverObj.obGoTo = (globalIndex, withoutTarget = false) => {
      if (!alive) return;
      const localIndex = globalIndex - startIndex;
      if (localIndex < 0 || localIndex >= stepsForSegment.length) return;
      setStepMode(localIndex, withoutTarget);
      driverObj.moveTo(localIndex);
    };

    return driverObj;
  }

  return {
    prefersReducedMotion,
    fillDynamic,
    waitForElement,
    ensureInView,
    buildPopoverHtml,
    createDriver,
    isMobileViewport,
  };
}
