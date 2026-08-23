import { driver } from 'driver.js';
import { waitForElement, ensureInView } from '@/components/onboarding/targetWaits';
import 'driver.js/dist/driver.css';
import { sanitizeHtml } from '@/utils/sanitize.js';
import { useOnboardingStore } from '@/stores/onboarding';
import { getDemo } from '@/components/onboarding/onboardingDemo';
import { groupStepsBySection } from '@/components/onboarding/stepsFlow';
import { createPopoverZoomFix } from '@/components/onboarding/popoverZoom';
import { STAGE_PADDING, STAGE_RADIUS, applyStageShape, raiseActiveHighlight } from '@/components/onboarding/stageShape';

/**
 * Мобильный брейкпоинт тура (#1097 S11) - совпадает с media-запросами
 * NavMenu/TheHeader/CreateApplication (`max-width: 768px`, ВКЛЮЧИТЕЛЬНО), на
 * которых рельс сворачивается в drawer, вторичные иконки шапки - в overflow-меню,
 * а форма/таблицы - в одну колонку. Порог `<= 768` (не `< 768`): на ровно 768px
 * (iPad-портрет) CSS уже мобильный - reveal обязан срабатывать там же, иначе тур
 * подсветит переехавшую пустоту (класс бага «768 vs 767.98» из S8/S9). Модульная
 * функция (не часть фабрики useOnboarding) - её зовут и reveal, и createDriver.
 *
 * @returns {boolean}
 */
export function isMobileViewport() {
  return typeof window !== 'undefined' && window.innerWidth <= 768;
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
   * Список шагов тура с переходом по клику. Тур длинный (у заявителя за сорок
   * шагов), и без него вернуться к нужному месту можно было только прокликав
   * всё заново.
   *
   * @param {number} currentGlobal глобальный индекс текущего шага
   * @param {(index: number) => void} onJump
   * @returns {HTMLElement}
   */
  function buildStepList(currentGlobal, onJump) {
    const store = useOnboardingStore();
    const list = document.createElement('div');
    list.className = 'ob-popover__steps';
    list.setAttribute('data-testid', 'ob-step-list');

    groupStepsBySection(store.steps, store.skippedIndexes).forEach((group) => {
      const head = document.createElement('div');
      head.className = 'ob-popover__steps-group';
      head.textContent = group.title;
      list.appendChild(head);

      group.items.forEach((item) => {
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'ob-popover__steps-item';
        if (item.index === currentGlobal) btn.classList.add('is-current');
        if (item.index < currentGlobal) btn.classList.add('is-passed');
        btn.textContent = item.title;
        btn.addEventListener('click', () => onJump(item.index));
        list.appendChild(btn);
      });
    });
    return list;
  }

  function buildProgressBlock(globalIndex, total, nextTitle, currentGlobal, onJump) {
    const block = document.createElement('div');
    block.className = 'ob-popover__progress';

    const label = document.createElement('button');
    label.type = 'button';
    label.className = 'ob-popover__step-label';
    label.setAttribute('data-testid', 'ob-step-counter');
    label.textContent = `Шаг ${globalIndex} из ${total}`;
    if (onJump) {
      const list = buildStepList(currentGlobal, onJump);
      label.addEventListener('click', () => {
        const opening = !block.classList.contains('is-open');
        if (opening) {
          // Список - слой поверх карточки, а не её часть: иначе раскрытие
          // растит поповер, и driver не переставляет его - нижние пункты
          // оказываются за краем экрана. Сторону выбираем по свободному месту.
          const rect = block.getBoundingClientRect();
          const below = window.innerHeight - rect.bottom;
          const up = below < Math.min(rect.top, 260);
          block.classList.toggle('ob-popover__progress--up', up);
          list.style.maxHeight = `${Math.max(140, Math.min(260, (up ? rect.top : below) - 16))}px`;
          // Текущий шаг сразу в поле зрения - иначе в длинном туре список
          // открывается на первом разделе, где искать нечего.
          requestAnimationFrame(() => {
            list.querySelector('.is-current')?.scrollIntoView({ block: 'center' });
          });
        }
        block.classList.toggle('is-open', opening);
      });
      block.appendChild(list);
      label.title = 'Показать список шагов';
    } else {
      label.disabled = true;
    }

    const bar = document.createElement('div');
    bar.className = 'ob-popover__bar';
    const fill = document.createElement('div');
    fill.className = 'ob-popover__bar-fill';
    // Заполнение через scaleX (анимируем transform, не width - правило проекта).
    fill.style.transform = `scaleX(${total ? globalIndex / total : 0})`;
    bar.appendChild(fill);

    block.appendChild(label);
    block.appendChild(bar);

    if (nextTitle) {
      const hint = document.createElement('div');
      hint.className = 'ob-popover__next-hint';
      hint.textContent = `Далее: ${nextTitle}`;
      block.appendChild(hint);
    }

    return block;
  }

  function buildSkipButton(onSkip) {
    const skip = document.createElement('button');
    skip.type = 'button';
    skip.className = 'ob-popover__skip';
    skip.textContent = 'Пропустить обучение';
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
  function createDriver(stepsForSegment, { startIndex = 0, fallbackIndex = -1, onIndexChange, onDestroyed, onBoundaryNext, onBoundaryPrev, onCloseRequest, onBeforeStep, onJumpTo } = {}) {
    const store = useOnboardingStore();
    const lastLocal = stepsForSegment.length - 1;

    const zoomFix = createPopoverZoomFix();

    /**
     * Снять класс подсветки со всего, что не является текущей целью.
     *
     * driver.js помечает элемент классом сразу, а снимает со старого только когда
     * его внутреннее `__activeElement` доедет - при переприцеливании (obRetarget)
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
        || (selector ? document.querySelector(selector) : null);
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
      return { element: s.element || undefined, popover };
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
      async onNextClick() {
        const localIndex = driverObj.getActiveIndex() ?? 0;
        if (localIndex >= lastLocal && onBoundaryNext) {
          // Передаём РЕАЛЬНЫЙ глобальный индекс driver'а - store.currentIndex может
          // отставать (onHighlighted последнего шага мог не успеть его обновить).
          onBoundaryNext(startIndex + localIndex);
          return;
        }
        // Готовим целевой шаг ДО перехода: onBeforeStep ставит демо-вложение и
        // ДОЖИДАЕТСЯ появления элемента (иначе driver подсветит пустоту, если
        // данные ещё грузятся). Опциональный шаг без элемента (напр. доп.поля
        // "при наличии") пропускаем к следующему; шаг со скриншотом вместо
        // пропуска показываем без подсветки (STEP_DEMO_FALLBACK).
        let target = localIndex + 1;
        while (target <= lastLocal) {
          const ready = onBeforeStep ? await onBeforeStep(startIndex + target) : true;
          if (ready !== false) {
            // Вид шага пересобираем на каждом проходе: цель могла как пропасть,
            // так и появиться (данные подъехали, «Назад» и снова «Далее»).
            setStepMode(target, ready === STEP_DEMO_FALLBACK);
            break;
          }
          // Шаг выброшен - запоминаем, чтобы «Шаг N из M» считал по реальному
          // маршруту, а не по конфигу.
          store.markSkipped(startIndex + target);
          target += 1;
        }
        if (target > lastLocal) {
          // Все оставшиеся шаги опциональны и отсутствуют - это граница сегмента.
          if (onBoundaryNext) onBoundaryNext(startIndex + lastLocal);
          return;
        }
        if (target === localIndex + 1) driverObj.moveNext();
        else driverObj.moveTo(target);
      },
      // "Назад": внутри сегмента - обычный movePrevious; на ПЕРВОМ шаге сегмента
      // (есть предыдущий шаг на другой странице) отдаём хосту, чтобы он вернулся
      // на ту страницу и показал последний шаг предыдущего сегмента.
      onPrevClick() {
        const localIndex = driverObj.getActiveIndex() ?? 0;
        if (localIndex <= 0 && startIndex > 0 && onBoundaryPrev) {
          onBoundaryPrev(startIndex);
          return;
        }
        // «Назад» через onBeforeStep не проходит (тот гейтит только «Далее»),
        // поэтому те же решения принимаем здесь.
        //
        // Во-первых, пропускаем назад шаги, которые «Далее» выбросило: без этого
        // возврат показывал шаг, которого человек вперёд не видел вовсе (в туре
        // охраны - «Пропуск по факту» на пустом блоке ручного ввода).
        //
        // Во-вторых, вид шага со скриншотом зависит от наличия цели прямо сейчас -
        // пересобираем его сами, иначе шаг вернулся бы в том виде, в каком его
        // собрали на старте сегмента. Страницу мы уже видели, поэтому проверка
        // синхронная: ждать отрисовки нечего, и «Назад» не подвисает.
        // Пропускаем ровно те шаги, которые тур выбросил по дороге вперёд: их
        // индексы копит стор. Судить по текущему DOM нельзя - экран с тех пор
        // изменился (карточка открыта, панель раскрыта), и «Назад» то показывал
        // шаг, которого человек не видел, то перепрыгивал через увиденный.
        const skipped = new Set(store.skippedIndexes);
        let target = localIndex - 1;
        while (target > 0 && skipped.has(startIndex + target)) target -= 1;
        const prev = stepsForSegment[target];
        if (prev?.demo) {
          setStepMode(target, !prev.element || !document.querySelector(prev.element));
        }
        // Шаг, которому нужно раскрыть узел или сменить бланк, готовим так же,
        // как на «Далее»: ставим сигнал и ДОЖИДАЕМСЯ цели. Иначе возврат на шаг
        // с окном показывал его пустым - окно открывалось лишь через несколько
        // секунд, уже после того, как шаг нарисовался («нажал Назад, десять
        // секунд ничего, потом бланк без бланка»).
        const needsPrepare = Boolean(prev?.reveal?.open || prev?.demoAttachment);
        const go = () => {
          if (target === localIndex - 1) driverObj.movePrevious();
          else driverObj.moveTo(target);
        };
        if (needsPrepare && onBeforeStep) {
          Promise.resolve(onBeforeStep(startIndex + target)).then(go);
          return;
        }
        go();
      },
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
          && !document.querySelector(nextStep.element)
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
          buildProgressBlock(globalIndex, total, nextTitle, currentGlobal, onJumpTo),
          popover.footer.firstChild,
        );
        // "Пропустить" - первым в ряду кнопок (CSS прижимает Назад/Далее вправо).
        // На финале не показываем: там уже есть "Готово" и CTA.
        if (!step?.celebrate) {
          popover.footerButtons.insertBefore(
            buildSkipButton(() => (onCloseRequest ? onCloseRequest() : driverObj.destroy())),
            popover.footerButtons.firstChild,
          );
        }

        // Компенсация корневого zoom для позиционирования поповера - едина для ВСЕХ
        // шагов (и anchored, и центро-модальных): см. popoverZoom.js.
        zoomFix.attach(popover.wrapper);
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
        dropStaleHighlights();
        raiseActiveHighlight(driverObj);
        const localIndex = driverObj.getActiveIndex() ?? 0;
        onIndexChange?.(startIndex + localIndex);
      },
      onDestroyed() {
        zoomFix.detach();
        // Свой класс подъёма driver не знает - снимаем сами, иначе элемент
        // останется висеть над остальной страницей после закрытия тура.
        document.querySelectorAll('.ob-highlighted').forEach((el) => el.classList.remove('ob-highlighted'));
        onDestroyed?.();
      },
    });

    /**
     * Переприцелить шаг на заново отрисованный узел и перерисовать его.
     *
     * Нужно после смены демо-бланка: форма пересоздаётся, driver продолжает
     * держаться за прежнюю цель, и подсветка пропадает (шаг «Форма Автомобили»
     * при возврате «Назад» с «Сотрудников»). setStepMode пересобирает конфиг
     * шага, moveTo заставляет driver перечитать его.
     *
     * @param {number} globalIndex глобальный индекс шага
     */
    driverObj.obRetarget = (globalIndex) => driverObj.obGoTo(globalIndex, false);

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
