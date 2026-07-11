import { driver } from 'driver.js';
import 'driver.js/dist/driver.css';
import { sanitizeHtml } from '@/utils/sanitize.js';
import { useOnboardingStore } from '@/stores/onboarding';
import { getDemo } from '@/components/onboarding/onboardingDemo';

/**
 * Мобильный брейкпоинт тура (#1097 S11) - совпадает с media-запросами
 * NavMenu/TheHeader/CreateApplication (`max-width: 768px`, ВКЛЮЧИТЕЛЬНО), на
 * которых рельс сворачивается в drawer, вторичные иконки шапки - в overflow-меню,
 * а форма/таблицы - в одну колонку. Порог `<= 768` (не `< 768`): на ровно 768px
 * (iPad-портрет) CSS уже мобильный - reveal обязан срабатывать там же, иначе тур
 * подсветит переехавшую пустоту (класс бага «768 vs 767.98» из S8/S9). Модульная
 * функция (не часть фабрики useOnboarding) - её зовут и mobileReveal, и createDriver.
 *
 * @returns {boolean}
 */
export function isMobileViewport() {
  return typeof window !== 'undefined' && window.innerWidth <= 768;
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
   * Дождаться появления и видимости элемента в DOM. Резолвит элементом или
   * `null` по таймауту - тур никогда не падает из-за отсутствующей цели.
   * `signal` позволяет хосту отменить ожидание (teardown/logout) и не оставить
   * висящий интервал.
   *
   * @param {string} selector
   * @param {number} [timeout]
   * @param {AbortSignal} [signal]
   * @returns {Promise<Element|null>}
   */
  function waitForElement(selector, timeout = 2500, signal) {
    return new Promise((resolve) => {
      const isVisible = (el) =>
        el && (el.offsetParent !== null || el.getBoundingClientRect().width > 0);
      // Готовность по СТАБИЛЬНОЙ высоте: элемент часто появляется пустым (скелетон/
      // ещё не пришли данные) и дорастает. Если резолвить по факту появления,
      // driver подсветит пустую рамку, а данные приедут уже под оверлеем. Поэтому
      // ждём, пока высота перестанет меняться между опросами.
      const measure = (el) => {
        if (!isVisible(el)) return null;
        const h = el.getBoundingClientRect().height;
        return h > 0 ? h : null;
      };

      if (signal?.aborted) {
        resolve(null);
        return;
      }

      const start = Date.now();
      let prevEl = null;
      let prevHeight = null;
      const cleanup = () => {
        clearInterval(intervalId);
        signal?.removeEventListener('abort', onAbort);
      };
      const onAbort = () => {
        cleanup();
        resolve(null);
      };
      const tick = () => {
        const el = document.querySelector(selector);
        const h = measure(el);
        if (h !== null && el === prevEl && h === prevHeight) {
          cleanup();
          resolve(el);
          return;
        }
        prevEl = el;
        prevHeight = h;
        if (Date.now() - start >= timeout) {
          cleanup();
          // По таймауту отдаём элемент, если он хотя бы виден (пусть driver
          // подсветит как есть), иначе null - цель так и не появилась.
          resolve(isVisible(el) ? el : null);
        }
      };
      const intervalId = setInterval(tick, 120);
      signal?.addEventListener('abort', onAbort);
      tick();
    });
  }

  /**
   * Тело поповера: текст шага + опциональный демо-скриншот (`step.demo`).
   * Весь HTML прогоняется через sanitizeHtml (src/alt/caption - наши статичные
   * значения, санитайз страхует от случайной разметки в тексте).
   *
   * @param {{ description: string, demo?: string }} step
   * @returns {string}
   */
  function buildPopoverHtml(step) {
    let html = step.description || '';
    const demo = step.demo ? getDemo(step.demo) : null;
    if (demo) {
      const caption = demo.caption
        ? `<figcaption class="ob-popover__demo-caption">${demo.caption}</figcaption>`
        : '';
      html += `<figure class="ob-popover__demo"><img class="ob-popover__demo-img" src="${demo.src}" alt="${demo.alt}" />${caption}</figure>`;
    }
    return sanitizeHtml(html);
  }

  function buildProgressBlock(globalIndex, total, nextTitle) {
    const block = document.createElement('div');
    block.className = 'ob-popover__progress';

    const label = document.createElement('div');
    label.className = 'ob-popover__step-label';
    label.textContent = `Шаг ${globalIndex} из ${total}`;

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

  function buildCtaButton(text, onClick) {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'ob-popover__cta';
    btn.textContent = text;
    btn.addEventListener('click', onClick);
    return btn;
  }

  /**
   * Сконфигурировать driver-инстанс для одного сегмента (подряд идущих шагов
   * с общим route).
   *
   * @param {Array<object>} stepsForSegment
   * @param {{ startIndex?: number, onIndexChange?: (globalIndex: number) => void, onDestroyed?: () => void, onBoundaryNext?: () => void, onBoundaryPrev?: (segmentStartGlobal: number) => void, onCtaClick?: (ctaRoute?: string) => void, onCloseRequest?: () => void }} [options]
   * @returns {import('driver.js').Driver}
   */
  function createDriver(stepsForSegment, { startIndex = 0, onIndexChange, onDestroyed, onBoundaryNext, onBoundaryPrev, onCtaClick, onCloseRequest, onBeforeStep } = {}) {
    const store = useOnboardingStore();
    const lastLocal = stepsForSegment.length - 1;

    const driverObj = driver({
      showProgress: false,
      animate: !prefersReducedMotion(),
      allowClose: true,
      // Чётче выделение: затемнение фона плотнее, спотлайт плотно по элементу,
      // скругление 30px. popoverOffset больше - карточка не наезжает на элемент.
      overlayOpacity: 0.78,
      stagePadding: 5,
      stageRadius: 30,
      popoverOffset: 16,
      popoverClass: 'ob-popover',
      nextBtnText: 'Далее',
      prevBtnText: 'Назад',
      doneBtnText: 'Готово',
      steps: stepsForSegment.map((s, li) => {
        const popover = {
          title: s.title,
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
        // Шаги с демо-скриншотом шире - чтобы реальная таблица читалась.
        if (s.demo) popover.popoverClass = 'ob-popover ob-popover--wide';
        // Последний шаг сегмента, но впереди есть шаги на другой странице -
        // ТОЛЬКО подпись кнопки: "Далее" вместо "Готово", чтобы юзер видел, что
        // тур продолжится. Само поведение перехода диктует onNextClick ниже.
        if (li === lastLocal && store.steps[startIndex + li + 1]) {
          popover.nextBtnText = 'Далее';
        }
        return { element: s.element || undefined, popover };
      }),
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
        // "при наличии") пропускаем к следующему.
        let target = localIndex + 1;
        while (target <= lastLocal) {
          const ready = onBeforeStep ? await onBeforeStep(startIndex + target) : true;
          if (ready !== false) break;
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
        } else {
          driverObj.movePrevious();
        }
      },
      onPopoverRender(popover) {
        const localIndex = driverObj.getActiveIndex() ?? 0;
        const currentGlobal = startIndex + localIndex;
        const step = stepsForSegment[localIndex];
        // Нумерация без опциональных шагов: доп.поля «при наличии» появляются не
        // всегда, поэтому в счёт «Шаг N из M» их не берём - иначе при пропуске
        // получается дырка в номерах (22 -> 24).
        const total = store.steps.filter((s) => !s.optional).length;
        const globalIndex = store.steps.slice(0, currentGlobal + 1).filter((s) => !s.optional).length;
        // Подсказка "Далее" сквозная (вкл. шаг со след. страницы). Пропускаем
        // опциональные шаги, элемента которых сейчас нет в DOM (будут скипнуты) -
        // иначе хинт обещает шаг, на который тур не перейдёт.
        let nextIdx = currentGlobal + 1;
        let nextStep = store.steps[nextIdx];
        while (nextStep && nextStep.optional && nextStep.element && !document.querySelector(nextStep.element)) {
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
        if (step?.cta) {
          const cta = buildCtaButton(step.cta, () => onCtaClick?.(step.ctaRoute));
          popover.description.insertAdjacentElement('afterend', cta);
        }

        // Прогресс сверху футера.
        popover.footer.insertBefore(
          buildProgressBlock(globalIndex, total, nextTitle),
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
      },
      // Esc / клик по оверлею / крестик идут сюда (g(true)) ДО гейта на
      // __activeStep - в отличие от onDestroyed это надёжно срабатывает, даже
      // если шаг закрыт во время entry-анимации. Отдаём закрытие хосту; он
      // остановит тур (-> teardown снимет overlay и пометит авто-тур).
      onDestroyStarted() {
        if (onCloseRequest) onCloseRequest();
        else driverObj.destroy();
      },
      onHighlighted() {
        const localIndex = driverObj.getActiveIndex() ?? 0;
        onIndexChange?.(startIndex + localIndex);
      },
      onDestroyed() {
        onDestroyed?.();
      },
    });

    return driverObj;
  }

  return {
    prefersReducedMotion,
    waitForElement,
    buildPopoverHtml,
    createDriver,
    isMobileViewport,
  };
}
