import { describe, it, expect } from 'vitest';
import { onboardingSteps, ONBOARDING_VERSION, collectSegment } from '../onboardingSteps';

describe('onboardingSteps', () => {
  it('версия тура - целое число >= 1', () => {
    expect(Number.isInteger(ONBOARDING_VERSION)).toBe(true);
    expect(ONBOARDING_VERSION).toBeGreaterThanOrEqual(1);
  });

  it('каждый шаг имеет непустые id, title и строковый description', () => {
    for (const step of onboardingSteps) {
      expect(typeof step.id).toBe('string');
      expect(step.id.length).toBeGreaterThan(0);
      expect(typeof step.title).toBe('string');
      expect(step.title.length).toBeGreaterThan(0);
      expect(typeof step.description).toBe('string');
      expect(step.description.length).toBeGreaterThan(0);
    }
  });

  it('element - либо строка-селектор, либо null', () => {
    for (const step of onboardingSteps) {
      const ok = step.element === null || (typeof step.element === 'string' && step.element.length > 0);
      expect(ok).toBe(true);
    }
  });

  it('route у каждого шага - непустая строка', () => {
    for (const step of onboardingSteps) {
      expect(typeof step.route).toBe('string');
      expect(step.route.length).toBeGreaterThan(0);
    }
  });

  it('нет дублей id', () => {
    const ids = onboardingSteps.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('есть хотя бы один центр-модал (element null) для старта', () => {
    const hasCenterModal = onboardingSteps.some((s) => s.element === null);
    expect(hasCenterModal).toBe(true);
  });

  it('expandRail только у nav-шагов и только true', () => {
    const railSteps = onboardingSteps.filter((s) => s.expandRail);
    expect(railSteps.length).toBeGreaterThan(0);
    for (const s of railSteps) {
      expect(s.expandRail).toBe(true);
      expect(s.id.startsWith('nav-')).toBe(true);
    }
  });

  it('есть шаги шапки и навигации с целевыми селекторами', () => {
    const ids = onboardingSteps.map((s) => s.id);
    for (const id of ['header-feedback', 'header-time', 'header-notifications', 'header-submit', 'nav-rail', 'nav-group-data']) {
      expect(ids).toContain(id);
    }
    // у каждого шага шапки/навигации - строковый селектор цели (не центр-модал)
    for (const s of onboardingSteps.filter((x) => x.id.startsWith('header-') || x.id.startsWith('nav-'))) {
      expect(typeof s.element).toBe('string');
    }
  });

  it('announcement идёт ПЕРЕД documents в сегменте news', () => {
    const ids = onboardingSteps.map((s) => s.id);
    expect(ids).toContain('announcement');
    expect(ids.indexOf('announcement')).toBeLessThan(ids.indexOf('documents'));
    expect(ids).not.toContain('header-broadcast');
  });

  it('шаги шапки идут слева направо: feedback -> time -> notifications -> submit', () => {
    const ids = onboardingSteps.map((s) => s.id);
    const order = ['header-feedback', 'header-time', 'header-notifications', 'header-submit'].map((id) => ids.indexOf(id));
    const sorted = [...order].sort((a, b) => a - b);
    expect(order).toEqual(sorted);
  });
});

describe('collectSegment', () => {
  const steps = [
    { id: 'a', route: '/news' },
    { id: 'b', route: '/news' },
    { id: 'c', route: '/personal-cabinet' },
    { id: 'd', route: '/personal-cabinet' },
    { id: 'e', route: '/carsview' },
  ];

  it('берёт подряд идущие шаги с одним route начиная с индекса', () => {
    expect(collectSegment(steps, 0, '/news').map((s) => s.id)).toEqual(['a', 'b']);
  });

  it('останавливается на границе сегмента (другой route)', () => {
    expect(collectSegment(steps, 2, '/personal-cabinet').map((s) => s.id)).toEqual(['c', 'd']);
  });

  it('пустой массив если route стартового шага не совпадает с активным', () => {
    expect(collectSegment(steps, 0, '/carsview')).toEqual([]);
  });

  it('одиночный сегмент в конце массива', () => {
    expect(collectSegment(steps, 4, '/carsview').map((s) => s.id)).toEqual(['e']);
  });

  it('реальная конфигурация: первый сегмент news = contiguous блок с нулевого индекса', () => {
    const seg = collectSegment(onboardingSteps, 0, '/news');
    // финальный шаг тоже /news, но он в конце (отдельный сегмент) - сегмент с 0
    // обрывается на первом не-/news шаге, а не собирает все /news скопом.
    const firstNonNews = onboardingSteps.findIndex((s) => s.route !== '/news');
    expect(seg.length).toBe(firstNonNews);
    expect(seg[0].id).toBe('start');
    expect(seg[seg.length - 1].id).toBe('nav-group-data');
  });
});

describe('cross-page конфигурация (cabinet)', () => {
  it('есть шаги личного кабинета на /personal-cabinet', () => {
    const ids = onboardingSteps.map((s) => s.id);
    for (const id of ['cabinet-profile', 'cabinet-notifications', 'cabinet-applications']) {
      expect(ids).toContain(id);
    }
    for (const s of onboardingSteps.filter((x) => x.id.startsWith('cabinet-'))) {
      expect(s.route).toBe('/personal-cabinet');
      expect(typeof s.element).toBe('string');
    }
  });

  it('сегмент cabinet отделён от news границей route (есть >1 сегмента)', () => {
    const firstCabinet = onboardingSteps.findIndex((s) => s.route === '/personal-cabinet');
    expect(firstCabinet).toBeGreaterThan(0);
    // предыдущий шаг - другой страницы (настоящая граница сегмента)
    expect(onboardingSteps[firstCabinet - 1].route).not.toBe('/personal-cabinet');
    const cabinetSeg = collectSegment(onboardingSteps, firstCabinet, '/personal-cabinet');
    expect(cabinetSeg.map((s) => s.id)).toEqual(['cabinet-profile', 'cabinet-notifications', 'cabinet-applications']);
  });

  it('шаг заявок несёт демо-скриншот applications', () => {
    const apps = onboardingSteps.find((s) => s.id === 'cabinet-applications');
    expect(apps.demo).toBe('applications');
  });
});

describe('cross-page конфигурация (cars / employees)', () => {
  const segs = [
    { route: '/carsview', ids: ['cars-filters', 'cars-add', 'cars-table'], demoStep: 'cars-table', demoKey: 'cars' },
    { route: '/employeesview', ids: ['employees-filters', 'employees-add', 'employees-table'], demoStep: 'employees-table', demoKey: 'employees' },
  ];

  for (const seg of segs) {
    it(`сегмент ${seg.route} собирается из своих шагов и отделён границей`, () => {
      const first = onboardingSteps.findIndex((s) => s.route === seg.route);
      expect(first).toBeGreaterThan(0);
      expect(onboardingSteps[first - 1].route).not.toBe(seg.route);
      expect(collectSegment(onboardingSteps, first, seg.route).map((s) => s.id)).toEqual(seg.ids);
    });

    it(`шаг ${seg.demoStep} несёт демо-скриншот ${seg.demoKey}`, () => {
      expect(onboardingSteps.find((s) => s.id === seg.demoStep).demo).toBe(seg.demoKey);
    });
  }

  it('сегменты идут в порядке cabinet -> cars -> employees', () => {
    const idx = (id) => onboardingSteps.findIndex((s) => s.id === id);
    expect(idx('cabinet-applications')).toBeLessThan(idx('cars-filters'));
    expect(idx('cars-table')).toBeLessThan(idx('employees-filters'));
  });
});

describe('cross-page конфигурация (создание заявки)', () => {
  it('сегмент /new-application отделён границей и идёт после сотрудников', () => {
    const first = onboardingSteps.findIndex((s) => s.route === '/new-application');
    expect(first).toBeGreaterThan(0);
    expect(onboardingSteps[first - 1].route).not.toBe('/new-application');
    expect(collectSegment(onboardingSteps, first, '/new-application').map((s) => s.id))
      .toEqual([
        'createapp-selector',
        'createapp-orginfo',
        'createapp-custom',
        'createapp-dates',
        'createapp-car-form',
        'createapp-people-form',
      ]);
  });

  it('гранулярные шаги формы держат демо-вложение и не несут скриншотов', () => {
    const formSteps = ['createapp-orginfo', 'createapp-custom', 'createapp-dates', 'createapp-car-form']
      .map((id) => onboardingSteps.find((s) => s.id === id));
    // секции org/доп.поля/даты/форма ТС держат бланк «Автомобили» отрисованным
    formSteps.forEach((s) => expect(s.demoAttachment).toBe('cars'));
    // шаг формы сотрудника переключает бланк
    expect(onboardingSteps.find((s) => s.id === 'createapp-people-form').demoAttachment).toBe('people');
    // скриншотов в шагах формы больше нет - выделяем реальные секции
    onboardingSteps
      .filter((s) => s.route === '/new-application')
      .forEach((s) => expect(s.demo).toBeUndefined());
  });

  it('шаг доп.полей опционален (может отсутствовать в форме)', () => {
    expect(onboardingSteps.find((s) => s.id === 'createapp-custom').optional).toBe(true);
  });

  it('идёт после сотрудников', () => {
    const idx = (id) => onboardingSteps.findIndex((s) => s.id === id);
    expect(idx('employees-table')).toBeLessThan(idx('createapp-selector'));
  });
});

describe('финальный шаг', () => {
  const finish = onboardingSteps[onboardingSteps.length - 1];

  it('финал - последний шаг, на Обзоре, подсвечивает кнопку Обучение', () => {
    expect(finish.id).toBe('finish');
    expect(finish.route).toBe('/news');
    expect(finish.element).toBe('[data-testid="ob-start-button"]');
  });

  it('несёт празднование и CTA на оформление заявки', () => {
    expect(finish.celebrate).toBe(true);
    expect(typeof finish.cta).toBe('string');
    expect(finish.cta.length).toBeGreaterThan(0);
  });

  it('возвращает тур на /news (граница сегмента после createApp)', () => {
    const prev = onboardingSteps[onboardingSteps.length - 2];
    expect(prev.route).toBe('/new-application');
    expect(finish.route).not.toBe(prev.route);
  });

  it('celebrate/cta есть только у финального шага', () => {
    const withCelebrate = onboardingSteps.filter((s) => s.celebrate);
    const withCta = onboardingSteps.filter((s) => s.cta);
    expect(withCelebrate).toHaveLength(1);
    expect(withCta).toHaveLength(1);
    expect(withCelebrate[0].id).toBe('finish');
  });
});
