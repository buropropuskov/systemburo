import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest';

/**
 * Фейковый driver.js: настоящий инстанс в jsdom бесполезен (нулевые рамки,
 * анимации), а проверяем мы СВОЮ логику - какой конфиг шагов уходит в driver и
 * как он меняется на «Далее»/«Назад». Конфиг и вызовы перемещения складываем в
 * общее состояние.
 */
const mocks = vi.hoisted(() => ({
  state: { config: null, activeIndex: 0, moves: [] },
}));

vi.mock('driver.js', () => ({
  driver: (config) => {
    mocks.state.config = config;
    return {
      getActiveIndex: () => mocks.state.activeIndex,
      moveNext: () => mocks.state.moves.push('next'),
      movePrevious: () => mocks.state.moves.push('prev'),
      moveTo: (i) => mocks.state.moves.push(`to:${i}`),
      drive: (i) => { mocks.state.activeIndex = i; },
      destroy: () => {},
    };
  },
}));

// createDriver читает из стора только набор шагов (нумерация и подпись «Далее»).
const storeState = vi.hoisted(() => ({ steps: [] }));
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => storeState }));

import {
  isMobileViewport,
  showsDemo,
  isSkippableStep,
  useOnboarding,
  STEP_DEMO_FALLBACK,
} from '../useOnboarding';

/**
 * isMobileViewport - брейкпоинт (#1097 S11), по которому OnboardingTour решает,
 * раскрывать ли переехавшие цели (drawer NavMenu / overflow-меню шапки) перед
 * подсветкой шага, и createDriver - принудительно класть поповер снизу для
 * side:'left'/'right' шагов (см. onboardingSteps.js JSDoc `reveal`).
 */
describe('isMobileViewport', () => {
  const originalWidth = window.innerWidth;

  afterEach(() => {
    window.innerWidth = originalWidth;
  });

  it('false на десктопной ширине (>=769)', () => {
    window.innerWidth = 1440;
    expect(isMobileViewport()).toBe(false);
    window.innerWidth = 769;
    expect(isMobileViewport()).toBe(false);
  });

  it('true на мобильной ширине (<=768, порог включителен как CSS max-width:768px)', () => {
    // 768 - iPad-портрет: CSS уже мобильный (max-width:768px), reveal обязан сработать.
    window.innerWidth = 768;
    expect(isMobileViewport()).toBe(true);
    window.innerWidth = 767;
    expect(isMobileViewport()).toBe(true);
    window.innerWidth = 390;
    expect(isMobileViewport()).toBe(true);
  });
});

/**
 * Демо-скриншот и пропуск шага - одна развилка (#1736): у нового пользователя
 * система пуста, и шаг «при наличии» либо выпадает молча, либо остаётся и
 * показывает вместо подсветки картинку. Что именно - решают эти два предиката.
 */
describe('showsDemo / isSkippableStep', () => {
  it('скриншот - только у шага без цели: рядом с подсвеченным элементом он дубль', () => {
    expect(showsDemo({ demo: 'applications', element: null })).toBe(true);
    expect(showsDemo({ demo: 'applications', element: '[data-testid="x"]' })).toBe(false);
  });

  it('без ключа demo скриншота нет ни при какой цели', () => {
    expect(showsDemo({ element: null })).toBe(false);
    expect(showsDemo({})).toBe(false);
  });

  it('пропускаем опциональный шаг без скриншота, со скриншотом - никогда', () => {
    expect(isSkippableStep({ optional: true })).toBe(true);
    expect(isSkippableStep({ optional: true, demo: 'applications' })).toBe(false);
    expect(isSkippableStep({ demo: 'applications' })).toBe(false);
    expect(isSkippableStep({})).toBe(false);
  });
});

describe('buildPopoverHtml', () => {
  const { buildPopoverHtml } = useOnboarding();

  it('шаг без цели со скриншотом: картинка и подпись в теле поповера', () => {
    const html = buildPopoverHtml({ description: 'Текст шага', demo: 'applications', element: null });
    expect(html).toContain('Текст шага');
    expect(html).toContain('ob-popover__demo-img');
    expect(html).toContain('ob-popover__demo-caption');
  });

  it('та же цель на месте - скриншота нет, человек смотрит на живой экран', () => {
    const html = buildPopoverHtml({
      description: 'Текст шага',
      demo: 'applications',
      element: '[data-testid="ob-applications"]',
    });
    expect(html).toContain('Текст шага');
    expect(html).not.toContain('<img');
  });

  it('неизвестный ключ скриншота не роняет сборку тела', () => {
    const html = buildPopoverHtml({ description: 'Текст', demo: 'нет-такого', element: null });
    expect(html).toContain('Текст');
    expect(html).not.toContain('<img');
  });
});

/**
 * Поведение движка на пустой системе (#1736). Шаг помечают `optional`, когда его
 * цели может не быть на экране; со скриншотом такой шаг вместо молчаливого
 * пропуска остаётся и показывает картинку.
 */
describe('createDriver - шаг без цели', () => {
  const { createDriver } = useOnboarding();

  const steps = [
    { id: 'intro', element: '[data-testid="ob-intro"]', title: 'Начало', description: 'Вступление' },
    {
      id: 'empty-demo',
      element: '[data-testid="ob-list"]',
      title: 'Ваши заявки',
      description: 'Список заявок',
      optional: true,
      demo: 'applications',
      side: 'left',
    },
    { id: 'empty-plain', element: '[data-testid="ob-extra"]', title: 'Доп. поля', description: 'Поля', optional: true },
    { id: 'outro', element: '[data-testid="ob-outro"]', title: 'Финал', description: 'Конец' },
  ];

  /** Ответы onBeforeStep по локальному индексу шага. */
  function drive(answers) {
    storeState.steps = steps;
    return createDriver(steps, {
      onBeforeStep: (globalIndex) => Promise.resolve(
        Object.prototype.hasOwnProperty.call(answers, globalIndex) ? answers[globalIndex] : true,
      ),
      onBoundaryNext: () => mocks.state.moves.push('boundary'),
    });
  }

  beforeEach(() => {
    mocks.state.config = null;
    mocks.state.activeIndex = 0;
    mocks.state.moves = [];
    storeState.steps = steps;
    document.body.innerHTML = '';
  });

  it('цели нет, но есть скриншот: шаг остаётся, подсветка снята, картинка в поповере', async () => {
    drive({ 1: STEP_DEMO_FALLBACK });

    await mocks.state.config.onNextClick();

    // Именно переход на следующий шаг, а не прыжок через него.
    expect(mocks.state.moves).toEqual(['next']);
    const shown = mocks.state.config.steps[1];
    expect(shown.element).toBeUndefined();
    expect(shown.popover.description).toContain('ob-popover__demo-img');
    // Со скриншотом карточка шире - иначе таблица на картинке нечитаема.
    expect(shown.popover.popoverClass).toContain('ob-popover--wide');
  });

  it('цель на месте: подсветка реального элемента и никакого скриншота', async () => {
    drive({});

    await mocks.state.config.onNextClick();

    expect(mocks.state.moves).toEqual(['next']);
    const shown = mocks.state.config.steps[1];
    expect(shown.element).toBe('[data-testid="ob-list"]');
    expect(shown.popover.description).not.toContain('<img');
    expect(shown.popover.popoverClass).toBeUndefined();
  });

  it('опциональный шаг без скриншота при отсутствии цели выбрасывается (замок на прежнее поведение)', async () => {
    drive({ 2: false });
    mocks.state.activeIndex = 1;

    await mocks.state.config.onNextClick();

    // Через шаг 2 - сразу к 3, шаг 2 человек не видит.
    expect(mocks.state.moves).toEqual(['to:3']);
  });

  it('данные подъехали - шаг возвращается к подсветке, а не остаётся с картинкой', async () => {
    const answers = { 1: STEP_DEMO_FALLBACK };
    drive(answers);
    await mocks.state.config.onNextClick();
    expect(mocks.state.config.steps[1].element).toBeUndefined();

    // «Назад», а потом снова «Далее» - к этому моменту заявка у человека уже есть.
    answers[1] = true;
    mocks.state.activeIndex = 0;
    await mocks.state.config.onNextClick();

    expect(mocks.state.config.steps[1].element).toBe('[data-testid="ob-list"]');
    expect(mocks.state.config.steps[1].popover.description).not.toContain('<img');
  });

  it('«Назад» на шаг со скриншотом при пустом экране возвращает его с картинкой', () => {
    drive({});
    mocks.state.activeIndex = 2;

    mocks.state.config.onPrevClick();

    expect(mocks.state.moves).toEqual(['prev']);
    expect(mocks.state.config.steps[1].element).toBeUndefined();
    expect(mocks.state.config.steps[1].popover.description).toContain('ob-popover__demo-img');
  });
});

/**
 * Нумерация «Шаг N из M» и подсказка «Далее: ...». Шаг, который движок может
 * выбросить, в счёт не идёт - иначе в номерах дырка; шаг со скриншотом не
 * выбрасывается никогда и потому считается.
 */
describe('createDriver - прогресс и подсказка следующего шага', () => {
  const { createDriver } = useOnboarding();

  const steps = [
    { id: 'a', element: null, title: 'Первый', description: 'A' },
    { id: 'b', element: '[data-testid="ob-list"]', title: 'Ваши заявки', description: 'B', optional: true, demo: 'applications' },
    { id: 'c', element: '[data-testid="ob-extra"]', title: 'Доп. поля', description: 'C', optional: true },
    { id: 'd', element: null, title: 'Финал', description: 'D' },
  ];

  /** Минимальный PopoverDOM - ровно те узлы, которые трогает onPopoverRender. */
  function fakePopover() {
    const wrapper = document.createElement('div');
    const title = document.createElement('div');
    const description = document.createElement('div');
    const footer = document.createElement('div');
    const footerButtons = document.createElement('div');
    const previousButton = document.createElement('button');
    footer.appendChild(footerButtons);
    wrapper.append(title, description, footer);
    return { wrapper, title, description, footer, footerButtons, previousButton };
  }

  function render(localIndex) {
    storeState.steps = steps;
    mocks.state.activeIndex = localIndex;
    createDriver(steps, {});
    const popover = fakePopover();
    mocks.state.config.onPopoverRender(popover);
    return popover;
  }

  beforeEach(() => {
    mocks.state.config = null;
    mocks.state.activeIndex = 0;
    mocks.state.moves = [];
    document.body.innerHTML = '';
  });

  it('шаг со скриншотом посчитан, пропускаемый - нет', () => {
    // Из четырёх шагов в счёт идут три: «доп. поля» выпадут на пустой системе.
    const popover = render(1);
    expect(popover.wrapper.querySelector('.ob-popover__step-label').textContent).toBe('Шаг 2 из 3');
  });

  it('номер последнего шага сходится с итогом', () => {
    const popover = render(3);
    expect(popover.wrapper.querySelector('.ob-popover__step-label').textContent).toBe('Шаг 3 из 3');
  });

  it('подсказка перескакивает пропускаемый шаг без цели', () => {
    const popover = render(1);
    expect(popover.wrapper.querySelector('.ob-popover__next-hint').textContent).toBe('Далее: Финал');
  });

  it('шаг со скриншотом в подсказке не перескакивается - тур на него перейдёт', () => {
    const popover = render(0);
    expect(popover.wrapper.querySelector('.ob-popover__next-hint').textContent).toBe('Далее: Ваши заявки');
  });
});
