import { driver } from 'driver.js';
import 'driver.js/dist/driver.css';
import { sanitizeHtml } from '@/utils/sanitize.js';
import { useOnboardingStore } from '@/stores/onboarding';
import { getDemo } from '@/components/onboarding/onboardingDemo';

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
      const isReady = (el) =>
        el && (el.offsetParent !== null || el.getBoundingClientRect().width > 0);

      if (signal?.aborted) {
        resolve(null);
        return;
      }

      const immediate = document.querySelector(selector);
      if (isReady(immediate)) {
        resolve(immediate);
        return;
      }

      const start = Date.now();
      const cleanup = () => {
        clearInterval(intervalId);
        signal?.removeEventListener('abort', onAbort);
      };
      const onAbort = () => {
        cleanup();
        resolve(null);
      };
      const intervalId = setInterval(() => {
        const el = document.querySelector(selector);
        if (isReady(el)) {
          cleanup();
          resolve(el);
          return;
        }
        if (Date.now() - start >= timeout) {
          cleanup();
          resolve(null);
        }
      }, 100);
      signal?.addEventListener('abort', onAbort);
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
    skip.textContent = 'Пропустить';
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
   * @param {{ startIndex?: number, onIndexChange?: (globalIndex: number) => void, onDestroyed?: () => void, onBoundaryNext?: () => void, onCtaClick?: () => void, onCloseRequest?: () => void }} [options]
   * @returns {import('driver.js').Driver}
   */
  function createDriver(stepsForSegment, { startIndex = 0, onIndexChange, onDestroyed, onBoundaryNext, onCtaClick, onCloseRequest, onBeforeStep } = {}) {
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
        // выделенный элемент (напр. элементы шапки вверху -> поповер вниз).
        if (s.side) popover.side = s.side;
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
        } else {
          // Шаг может требовать подготовки DOM перед переходом (напр. добавить
          // демо-вложение, чтобы форма отрисовалась) - даём хосту шанс довести
          // цель до готовности ДО moveNext, иначе driver подсветит пустоту.
          if (onBeforeStep) await onBeforeStep(startIndex + localIndex + 1);
          driverObj.moveNext();
        }
      },
      onPopoverRender(popover) {
        const localIndex = driverObj.getActiveIndex() ?? 0;
        const globalIndex = startIndex + localIndex + 1;
        const total = store.totalSteps;
        const step = stepsForSegment[localIndex];
        // Подсказка "Далее" сквозная - берём следующий шаг по глобальному
        // индексу, чтобы на границе сегмента показать имя шага со след. страницы.
        const nextStep = store.steps[globalIndex];
        const nextTitle = nextStep ? nextStep.title : '';

        // Финал: галочка перед заголовком + CTA после описания.
        if (step?.celebrate) {
          popover.wrapper.insertBefore(buildCelebrate(), popover.title);
        }
        if (step?.cta) {
          const cta = buildCtaButton(step.cta, () => onCtaClick?.());
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
  };
}
