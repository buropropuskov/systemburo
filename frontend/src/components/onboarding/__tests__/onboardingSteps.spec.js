import { describe, it, expect, afterEach } from 'vitest';
import { onboardingSteps, ONBOARDING_VERSION } from '../onboardingSteps';
import { collectSegment, indexAfterRoute } from '../stepsFlow';

const byId = (id) => onboardingSteps.find((s) => s.id === id);
const idx = (id) => onboardingSteps.findIndex((s) => s.id === id);

/** Шаги, живущие внутри карточки заявки (модалка в кабинете). */
const DETAIL_STEP_IDS = [
  'detail-opened',
  'detail-status', 'detail-status-section', 'detail-questions', 'detail-actions-intro',
  'detail-supplement', 'detail-duplicate', 'detail-revoke',
];

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
    for (const id of ['header-feedback', 'header-notifications', 'header-submit', 'nav-rail', 'nav-group-data']) {
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
  });

  it('контент страницы идёт до шапки, шапка - до навигации (тур не мечется по экрану)', () => {
    const ids = onboardingSteps.filter((s) => s.route === '/news' && s.id !== 'finish').map((s) => s.id);
    const last = (prefix) => ids.reduce((acc, id, i) => (id.startsWith(prefix) ? i : acc), -1);
    const first = (prefix) => ids.findIndex((id) => id.startsWith(prefix));
    expect(ids.indexOf('work-modes')).toBeLessThan(first('header-'));
    expect(last('header-')).toBeLessThan(first('nav-'));
  });

  it('шаги шапки идут слева направо: feedback -> notifications -> submit', () => {
    const ids = onboardingSteps.map((s) => s.id);
    const order = ['header-feedback', 'header-notifications', 'header-submit'].map((id) => ids.indexOf(id));
    const sorted = [...order].sort((a, b) => a - b);
    expect(order).toEqual(sorted);
  });
});

describe('reveal (#1097 S11 - переехавшие на <768 цели)', () => {
  it('единственное значение оси mobile - nav (header-overflow вымер вместе с меню "..." в W3)', () => {
    for (const s of onboardingSteps) {
      if (s.reveal?.mobile !== undefined) {
        expect(s.reveal.mobile).toBe('nav');
      }
    }
  });

  it('nav-rail и nav-group-data просят раскрытие drawer (nav)', () => {
    const byId = (id) => onboardingSteps.find((s) => s.id === id);
    expect(byId('nav-rail').reveal).toEqual({ mobile: 'nav' });
    expect(byId('nav-group-data').reveal).toEqual({ mobile: 'nav' });
  });

  it('feedback переехал в drawer (nav); колокольчик - в самой шапке', () => {
    const byId = (id) => onboardingSteps.find((s) => s.id === id);
    // #1097 W3.3: «Сообщить о проблеме» вынесено из "⋯" в бургер-drawer.
    expect(byId('header-feedback').reveal).toEqual({ mobile: 'nav' });
    // #1097 W3.2: колокольчик вынесен из "⋯" в саму шапку - reveal не нужен.
    expect(byId('header-notifications').reveal).toBeUndefined();
  });

  it('header-submit остаётся видимой иконкой на мобилке - без reveal', () => {
    expect(onboardingSteps.find((s) => s.id === 'header-submit').reveal).toBeUndefined();
  });
});

describe('requires (шаги, зависящие от прав)', () => {
  it('permission-зависимые цели шапки помечены своим правом', () => {
    const byId = (id) => onboardingSteps.find((s) => s.id === id);
    expect(byId('header-feedback').requires).toBe('header.report_problem');
    expect(byId('header-submit').requires).toBe('header.create_application');
  });

  it('requires не путается с optional - у этих шагов элемент есть всегда, когда право есть', () => {
    const byId = (id) => onboardingSteps.find((s) => s.id === id);
    expect(byId('header-feedback').optional).toBeUndefined();
    expect(byId('header-submit').optional).toBeUndefined();
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
    expect(seg[seg.length - 1].id).toBe('nav-theme');
  });
});

describe('indexAfterRoute', () => {
  const steps = [
    { id: 'a', route: '/accessible-attachments' },
    { id: 'fact-intro', route: '/table/kpp_1' },
    { id: 'fact-row', route: '/table/kpp_1' },
    { id: 'fact-exit', route: '/table/kpp_1' },
    { id: 'finish', route: '/accessible-attachments' },
  ];

  it('перепрыгивает непрерывный блок одного route к следующему шагу', () => {
    // с fact-intro (1) пропускаем весь блок /table/kpp_1 -> финал (4)
    expect(indexAfterRoute(steps, 1, '/table/kpp_1')).toBe(4);
    expect(steps[indexAfterRoute(steps, 1, '/table/kpp_1')].id).toBe('finish');
  });

  it('-1 если за недостижимым блоком шагов не осталось', () => {
    const tail = [
      { id: 'a', route: '/accessible-attachments' },
      { id: 'fact-intro', route: '/table/kpp_1' },
      { id: 'fact-exit', route: '/table/kpp_1' },
    ];
    expect(indexAfterRoute(tail, 1, '/table/kpp_1')).toBe(-1);
  });

  it('возвращает сам fromIndex, если его route уже другой (блок пуст)', () => {
    expect(indexAfterRoute(steps, 4, '/table/kpp_1')).toBe(4);
  });
});

describe('cross-page конфигурация (cabinet)', () => {
  it('есть шаги личного кабинета на /personal-cabinet', () => {
    const ids = onboardingSteps.map((s) => s.id);
    for (const id of ['cabinet-profile', 'cabinet-notifications', 'cabinet-applications']) {
      expect(ids).toContain(id);
    }
    // cabinet-outro - связка перед сменой страницы: у неё цели нет намеренно,
    // она же замыкает сегмент непропускаемым шагом.
    for (const s of onboardingSteps.filter((x) => x.id.startsWith('cabinet-') && x.id !== 'cabinet-outro')) {
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
    expect(cabinetSeg.map((s) => s.id)).toEqual([
      'cabinet-profile',
      'cabinet-password',
      'cabinet-applications',
      'cabinet-search',
      'cabinet-download',
      'cabinet-application-row',
      'detail-opened',
      'detail-status',
      'detail-status-section',
      'detail-questions',
      'detail-actions-intro',
      'detail-supplement',
      'detail-duplicate',
      'detail-revoke',
      'cabinet-notifications',
      'cabinet-notifications-settings',
    ]);
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
        'createapp-blank-added',
        'createapp-orginfo',
        'createapp-custom',
        'createapp-dates',
        'createapp-car-form',
        'createapp-car-existing',
        'createapp-car-places',
        'createapp-blank-switch',
        'createapp-people-form',
        'createapp-people-existing',
        'createapp-people-places',
        'createapp-consent',
        'createapp-submit',
      ]);
  });

  it('тур доводит до отправки: согласие идёт перед кнопкой «Отправить заявку»', () => {
    const idx = (id) => onboardingSteps.findIndex((s) => s.id === id);
    expect(idx('createapp-consent')).toBeLessThan(idx('createapp-submit'));
    // Согласие и кнопка отправки живут внутри формы вложения: без демо-бланка
    // блок не отрисован и шаги подсветили бы пустоту.
    for (const id of ['createapp-consent', 'createapp-submit']) {
      expect(onboardingSteps[idx(id)].demoAttachment).toBe('people');
    }
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

  // Форма одна на все бланки: по её появлению не понять, перерисовалась ли она
  // под нужный бланк. Без строгого ожидания шаг «Сотрудники» подсвечивал форму
  // автомобилей, а шаг смены бланка - список со старым выделением.
  it('шаги, зависящие от бланка, ждут именно свой бланк', () => {
    const waitOf = (id) => onboardingSteps.find((s) => s.id === id).waitFor;
    expect(waitOf('createapp-car-form')).toContain('data-attachment-type="cars"');
    expect(waitOf('createapp-people-form')).toContain('data-attachment-type="people"');
    expect(waitOf('createapp-consent')).toContain('data-attachment-type="people"');
    expect(waitOf('createapp-blank-switch')).toContain('data-selected-type="people"');
  });

  // Места разгрузки и прохода определяют, куда пустят машину и человека, а
  // «Добавить существующих» снимает повторный ввод - обе вещи просил показать
  // владелец, и обе живут внутри формы бланка.
  it('форма каждого бланка рассказывает про существующих и про места', () => {
    for (const id of ['createapp-car-existing', 'createapp-car-places']) {
      expect(byId(id).demoAttachment, id).toBe('cars');
    }
    for (const id of ['createapp-people-existing', 'createapp-people-places']) {
      expect(byId(id).demoAttachment, id).toBe('people');
    }
    expect(byId('createapp-car-places').element).toBe('[data-testid="ob-form-places"]');
    expect(byId('createapp-people-existing').element).toBe('[data-testid="ob-form-existing"]');
  });

  // Форма сотрудников выше половины экрана: по центру поповер накрывал ровно то,
  // о чём рассказывает.
  it('высокие шаги формы прижимают цель к низу экрана', () => {
    expect(byId('createapp-people-form').scrollTo).toBe('end');
    expect(byId('createapp-people-places').scrollTo).toBe('end');
  });

  // Имена бланков задаёт Бюро, поэтому шаг обязан брать их с экрана: иначе он
  // говорит «выбран другой бланк», не называя какой.
  it('шаг смены бланка называет бланк по имени с экрана', () => {
    const step = byId('createapp-blank-switch');
    expect(step.dynamic).toEqual({ blank: '[data-testid="ob-blank-selected"] .attachment-name' });
    // Заголовок остаётся статичным: он же показывается в списке шагов, где
    // подставлять ещё нечего.
    expect(step.title).not.toContain('{');
    expect(step.description).toContain('{blank}');
  });

  it('шаг доп.полей опционален (может отсутствовать в форме)', () => {
    expect(onboardingSteps.find((s) => s.id === 'createapp-custom').optional).toBe(true);
  });

  it('идёт после сотрудников', () => {
    const idx = (id) => onboardingSteps.findIndex((s) => s.id === id);
    expect(idx('employees-table')).toBeLessThan(idx('createapp-selector'));
  });
});

describe('сегмент карточки заявки (#1740)', () => {
  const detailSteps = DETAIL_STEP_IDS.map(byId);

  it('все шаги карточки есть и живут на роуте кабинета (карточка - модалка, не роут)', () => {
    detailSteps.forEach((s, i) => {
      expect(s, DETAIL_STEP_IDS[i]).toBeDefined();
      expect(s.route).toBe('/personal-cabinet');
    });
  });

  it('каждый шаг карточки optional - у новичка заявок нет вовсе', () => {
    detailSteps.forEach((s) => expect(s.optional, s.id).toBe(true));
  });

  // Шаг знакомит с окном заявки целиком. Пока он смотрел на шапку, подсвечивалась
  // полоска с номером, и это читалось как «тур показывает заголовок».
  // Шаг просит нажать на строку. Если человек нажал, карточка появляется поверх
  // подсвеченной строки - тур обязан уйти вперёд сам.
  it('шаг «Откройте заявку» уходит вперёд, когда карточка открылась', () => {
    expect(byId('cabinet-application-row').advanceWhen).toBe('[data-testid="ob-detail-card"]');
  });

  it('шаг «Вот ваша заявка» подсвечивает карточку, а не её шапку', () => {
    expect(byId('detail-opened').element).toBe('[data-testid="ob-detail-card"]');
  });

  // У нового пользователя заявок нет: открывать нечего, и без скриншота шаг
  // выпадал молча вместе со всем рассказом про карточку.
  it('шаг «Вот ваша заявка» несёт скриншот-пример на случай пустого кабинета', () => {
    expect(byId('detail-opened').demo).toBe('applicationDetail');
  });

  // В карточке кнопка скачивания скрыта на десктопе (display:none до 768px),
  // поэтому шаг про бланки всегда пропускался. Он живёт в списке заявок.
  it('шаг про скачивание бланков смотрит на кнопку в строке списка', () => {
    expect(byId('detail-download')).toBeUndefined();
    const step = byId('cabinet-download');
    expect(step.element).toBe('[data-testid="ob-application-download"]');
    expect(step.reveal).toBeUndefined();
  });

  it('reveal стоит на КАЖДОМ шаге карточки, а не только на первом', () => {
    // Наследование раскрытия у resolveReveal работает лишь ВНУТРИ группы (нужны оба
    // соседа с тем же значением): на последнем шаге карточка иначе закрылась бы.
    detailSteps.forEach((s) => expect(s.reveal, s.id).toEqual({ open: 'first-application' }));
  });

  it('сегмент кабинета начинается и заканчивается непропускаемым шагом', () => {
    const first = onboardingSteps.findIndex((s) => s.route === '/personal-cabinet');
    const seg = collectSegment(onboardingSteps, first, '/personal-cabinet');
    // Первый шаг сегмента ждёт цель долго и в центр-модалку не деградирует, а
    // последний ловит «Назад» со страницы настройки уведомлений - оба обязаны
    // существовать всегда.
    expect(seg[0].optional).toBeUndefined();
    expect(seg[seg.length - 1].id).toBe('cabinet-notifications-settings');
    expect(seg[seg.length - 1].optional).toBeUndefined();
  });

  it('шаги карточки идут подряд и не разрывают сегмент чужим роутом', () => {
    const positions = DETAIL_STEP_IDS.map(idx);
    expect(positions).toEqual([...positions].sort((a, b) => a - b));
    expect(positions[positions.length - 1] - positions[0]).toBe(positions.length - 1);
  });

  describe('пропуск, когда заявок нет', () => {
    afterEach(() => {
      document.body.innerHTML = '';
    });

    /**
     * Кабинет без открытой карточки: список пуст, модалки в DOM нет. Ровно то
     * состояние, в котором тур видит новичок.
     */
    function renderEmptyCabinet() {
      document.body.innerHTML = `
        <div data-testid="cabinet-page">
          <div data-testid="ob-profile"></div>
          <div data-testid="cabinet-notifications"></div>
          <div data-testid="ob-applications"></div>
          <div data-testid="ob-cabinet-search"></div>
        </div>`;
    }

    it('ни одна цель карточки в DOM не находится - движок пропустит их по optional', () => {
      renderEmptyCabinet();
      detailSteps.forEach((s) => expect(document.querySelector(s.element), s.id).toBe(null));
    });

    it('шаги кабинета вне карточки при этом на месте (иначе проверка была бы зелёной впустую)', () => {
      renderEmptyCabinet();
      for (const id of ['cabinet-profile', 'cabinet-notifications', 'cabinet-applications', 'cabinet-search']) {
        expect(document.querySelector(byId(id).element), id).not.toBe(null);
      }
    });

    it('после пропуска всех шагов карточки следующая цель - поиск, на том же роуте', () => {
      renderEmptyCabinet();
      // Тот же обход, что у движка на «Далее»: optional без элемента - к следующему.
      let i = idx('cabinet-applications') + 1;
      while (i < onboardingSteps.length
        && onboardingSteps[i].optional
        && !document.querySelector(onboardingSteps[i].element)) i += 1;

      expect(onboardingSteps[i].id).toBe('cabinet-search');
      expect(onboardingSteps[i].route).toBe('/personal-cabinet');
    });

    it('когда карточка открыта, те же цели находятся (замок не тавтология)', () => {
      renderEmptyCabinet();
      document.body.insertAdjacentHTML('beforeend', `
        <div data-testid="ob-detail-status"></div>
        <div data-testid="ob-detail-status-section"></div>
        <div data-testid="application-questions"></div>
        <div data-testid="ob-detail-card"><div data-testid="ob-detail-header"></div></div>
        <div data-testid="ob-detail-duplicate"></div>
        <button data-testid="app-detail-button-supplement"></button>
        <button data-testid="app-detail-button-download"></button>
        <button data-testid="ob-detail-revoke"></button>`);
      detailSteps.forEach((s) => expect(document.querySelector(s.element), s.id).not.toBe(null));
    });
  });
});

describe('порядок сегментов (#1740: новые шаги не рвут cross-page навигацию)', () => {
  /** Непрерывные блоки шагов одного роута - то, что движок поднимает как сегмент. */
  const blocks = onboardingSteps.reduce((acc, s) => {
    if (!acc.length || acc[acc.length - 1].route !== s.route) acc.push({ route: s.route, ids: [s.id] });
    else acc[acc.length - 1].ids.push(s.id);
    return acc;
  }, []);

  it('роут не появляется дважды вразбивку - единственный возврат на /news это финал', () => {
    expect(blocks.map((b) => b.route)).toEqual([
      '/news',
      '/personal-cabinet',
      // Настройка уведомлений - отдельная страница; шаги про неё стоят в конце
      // кабинета, чтобы не разрывать его надвое.
      '/notification-settings',
      '/carsview',
      '/employeesview',
      '/new-application',
      '/news',
    ]);
    expect(blocks[blocks.length - 1].ids).toEqual(['finish']);
  });

  it('каждый переход между блоками - смена роута (движок делает cross-page переход)', () => {
    blocks.forEach((b, i) => {
      if (i > 0) expect(b.route).not.toBe(blocks[i - 1].route);
    });
  });
});

describe('шаги, добавленные срезом S4 (#1740)', () => {
  const NEW_IDS = [
    'work-modes',
    'header-broadcast',
    'header-search',
    'nav-theme',
    ...DETAIL_STEP_IDS,
    'cabinet-search',
    'createapp-consent',
    'createapp-submit',
  ];

  it('все новые шаги на месте и целятся по data-testid (их стережёт tourSelectors.spec)', () => {
    NEW_IDS.forEach((id) => {
      const step = byId(id);
      expect(step, id).toBeDefined();
      expect(step.element, id).toMatch(/^\[data-testid="[^"]+"\]$/);
    });
  });

  it('версия тура поднята - иначе прошедшие старый тур не увидят новые шаги', () => {
    expect(ONBOARDING_VERSION).toBeGreaterThanOrEqual(2);
  });

  it('объявление в шапке помечено optional - пилюли нет, пока нет объявления', () => {
    expect(byId('header-broadcast').optional).toBe(true);
  });

  // Поиск разбит на два шага (#1771): сперва кнопка - её и надо запомнить, -
  // и только потом раскрытая панель. Раскрытие висит на втором: открыв панель
  // на первом, мы спрятали бы под ней ту самую кнопку.
  it('шаг кнопки поиска панель не раскрывает', () => {
    expect(byId('header-search').reveal).toBeUndefined();
    expect(byId('header-search').element).toBe('[data-testid="header-button-search"]');
  });

  it('шаг панели поиска раскрывает её и уводит поповер влево от панели', () => {
    const step = byId('header-search-panel');
    expect(step.reveal).toEqual({ open: 'search-panel' });
    expect(step.element).toBe('[data-testid="global-search-panel"]');
    // Панель прижата к правому краю: поповер снизу лёг бы прямо на результаты.
    expect(step.side).toBe('left');
    expect(step.optional).toBe(true);
  });

  it('тумблер темы держит рельс раскрытым и раскрывает drawer на мобилке', () => {
    expect(byId('nav-theme').expandRail).toBe(true);
    expect(byId('nav-theme').reveal).toEqual({ mobile: 'nav' });
  });

  it('дополнение заявки гейтится правом, а не только наличием кнопки', () => {
    expect(byId('detail-supplement').requires).toBe('action.supplement.application');
  });

  it('остальные новые шаги прав не требуют - их элементы правами не закрыты', () => {
    NEW_IDS.filter((id) => id !== 'detail-supplement')
      .forEach((id) => expect(byId(id).requires, id).toBeUndefined());
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
