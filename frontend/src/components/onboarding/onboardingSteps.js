/**
 * Версия тура. Записывается в localStorage-флаг завершения и сверяется при
 * чтении: повышение версии (новые шаги) заставит тур считаться непройденным
 * и переиграться при авто-запуске. Поднимать при добавлении/изменении шагов.
 */
export const ONBOARDING_VERSION = 1;

/**
 * Плоский упорядоченный массив шагов тура. Движок группирует ПОДРЯД идущие
 * шаги с одинаковым `route` в «сегмент» драйвера; смена `route` = граница
 * сегмента (cross-page переход).
 *
 * Поля шага:
 * - `id`         уникальный строковый ключ;
 * - `route`      путь страницы, на которой шаг живёт;
 * - `element`    CSS-селектор цели или `null` (центр-модал без подсветки);
 * - `title`      заголовок поповера;
 * - `description` HTML-тело поповера (прогоняется через sanitizeHtml);
 * - `demo`       необязательный ключ скриншота (мапа в onboardingDemo.js);
 * - `expandRail` если true - хост держит рельс навигации развёрнутым на время
 *                шага (и шага перед ним), возвращает прежнее состояние при выходе;
 * - `celebrate`  если true - в поповере рисуется галочка-празднование (финал);
 * - `cta`        текст финальной кнопки-CTA (ведёт на оформление заявки);
 * - `side`       сторона поповера от элемента (top/bottom/left/right) - чтобы
 *                карточка не наезжала на выделенный элемент;
 * - `align`      выравнивание поповера вдоль стороны (start/center/end);
 * - `mobileReveal` на <=768px цель шага переехала в свёрнутый узел интерфейса -
 *                хост (OnboardingTour) открывает его ПЕРЕД подсветкой:
 *                `'nav'` - бургер-drawer NavMenu (рельс+группы уезжают
 *                transform'ом за экран, но остаются в DOM "видимыми" для
 *                waitForElement - без открытия тур подсветил бы пустоту за
 *                краем). На >=769 (десктоп) поле не читается - там эти узлы
 *                всегда на месте. Механика раскрытия - в mobileReveal.js.
 *
 * @type {Array<{ id: string, route: string, element: string|null, title: string, description: string, demo?: string, expandRail?: boolean, celebrate?: boolean, cta?: string, side?: string, align?: string, mobileReveal?: 'nav' }>}
 */
export const onboardingSteps = [
  {
    id: 'start',
    route: '/news',
    element: null,
    title: 'Добро пожаловать!',
    description:
      'Коротко проведём по основным возможностям системы бюро пропусков. Это займёт пару минут. Двигайтесь кнопкой «Далее» или стрелками, выйти можно в любой момент клавишей Esc.',
  },
  {
    id: 'news',
    route: '/news',
    element: '[data-testid="ob-news"]',
    title: 'Новости системы',
    description:
      'Здесь публикуются новости системы. Заглядывайте сюда, чтобы быть в курсе изменений и обновлений.',
  },
  {
    id: 'guide',
    route: '/news',
    element: '[data-testid="ob-guide"]',
    title: 'Руководство пользователя',
    description:
      'Пошаговая инструкция по работе с системой: как подать заявку и пользоваться разделами. Откройте, если что-то непонятно.',
  },
  {
    id: 'announcement',
    side: 'left',
    route: '/news',
    element: '[data-testid="ob-announcement"]',
    title: 'Объявления',
    description:
      'Объявления администрации. Здесь появляются обычные и важные объявления - так вы не пропустите срочную информацию. Нажмите, чтобы прочитать целиком.',
  },
  {
    id: 'documents',
    route: '/news',
    element: '[data-testid="ob-documents"]',
    title: 'Документы',
    description: 'Полезные документы и шаблоны, доступные для скачивания.',
  },
  {
    id: 'header-feedback',
    side: 'bottom',
    route: '/news',
    element: '[data-testid="header-button-feedback"]',
    title: 'Сообщить о проблеме',
    description: 'Заметили ошибку или есть предложение - напишите нам прямо отсюда, не покидая систему.',
    // На мобилке кнопка переехала из "⋯" в бургер-drawer (W3.3) - раскрываем drawer.
    mobileReveal: 'nav',
  },
  {
    id: 'header-notifications',
    side: 'bottom',
    route: '/news',
    element: '[data-testid="ob-header-notifications"]',
    title: 'Уведомления',
    description: 'Колокольчик показывает количество непрочитанных уведомлений: смена статуса заявки, ответы согласующих и другие события. Нажмите, чтобы открыть список.',
  },
  {
    id: 'header-submit',
    side: 'bottom',
    route: '/news',
    element: '[data-testid="header-button-submit-app"]',
    title: 'Подать заявку',
    description: 'Быстрый переход к оформлению новой заявки на пропуск - кнопка доступна из любого раздела.',
  },
  {
    id: 'nav-rail',
    route: '/news',
    element: '[data-testid="ob-nav-rail"]',
    title: 'Навигация',
    description: 'Боковое меню - главный способ перемещаться по системе. Отсюда вы попадаете в любой раздел, в свой Личный кабинет, а внизу - кнопка «Выйти».',
    expandRail: true,
    mobileReveal: 'nav',
  },
  {
    id: 'nav-group-data',
    route: '/news',
    element: '[data-testid="ob-nav-group-data"]',
    title: 'Управление данными',
    description: 'Раздел «Управление данными»: ваши сотрудники и автомобили. Здесь вы ведёте справочники, по которым оформляются пропуска.',
    expandRail: true,
    mobileReveal: 'nav',
  },
  {
    id: 'cabinet-profile',
    route: '/personal-cabinet',
    element: '[data-testid="ob-profile"]',
    title: 'Личный кабинет',
    description: 'Мы перешли в ваш Личный кабинет. Вверху - ваши данные: организация, должность, контакты. Они автоматически подставляются в заявки.',
  },
  {
    id: 'cabinet-notifications',
    route: '/personal-cabinet',
    element: '[data-testid="cabinet-notifications"]',
    title: 'Ваши уведомления',
    description: 'Лента событий по вашим заявкам: согласование, одобрение, отказ. Так вы всегда в курсе, что происходит.',
  },
  {
    id: 'cabinet-applications',
    route: '/personal-cabinet',
    element: '[data-testid="ob-applications"]',
    title: 'Ваши заявки',
    description: 'Все поданные заявки и их статусы собраны здесь. Можно отслеживать ход согласования и историю. Вот как этот список выглядит с данными.',
    demo: 'applications',
  },
  {
    id: 'cars-filters',
    route: '/carsview',
    element: '[data-testid="ob-cars-filters"]',
    title: 'Раздел «Автомобили»',
    description: 'Теперь мы в разделе «Автомобили». Фильтры сверху переключают список: ваши машины, машины организации или компании.',
  },
  {
    id: 'cars-add',
    route: '/carsview',
    element: '[data-testid="cars-view-add-button"]',
    title: 'Добавить автомобиль',
    description: 'Кнопка добавления нового автомобиля - укажите номер и марку, и его можно будет вписывать в заявки.',
  },
  {
    id: 'cars-table',
    route: '/carsview',
    element: '[data-testid="ob-cars-table"]',
    title: 'Список автомобилей',
    description: 'Здесь все ваши автомобили с номерами и статусами. Вот как таблица выглядит с данными.',
    demo: 'cars',
  },
  {
    id: 'employees-filters',
    route: '/employeesview',
    element: '[data-testid="ob-employees-filters"]',
    title: 'Раздел «Сотрудники»',
    description: 'Дальше - раздел «Сотрудники». Фильтры так же переключают список по вашей организации или компании.',
  },
  {
    id: 'employees-add',
    route: '/employeesview',
    element: '[data-testid="ob-employees-add-button"]',
    title: 'Добавить сотрудника',
    description: 'Добавьте сотрудника - ФИО и должность, чтобы оформлять на него пропуска.',
  },
  {
    id: 'employees-table',
    route: '/employeesview',
    element: '[data-testid="ob-employees-table"]',
    title: 'Список сотрудников',
    description: 'Все ваши сотрудники собраны в этой таблице. Вот как она выглядит с данными.',
    demo: 'employees',
  },
  {
    id: 'createapp-selector',
    route: '/new-application',
    element: '[data-testid="ob-app-selector"]',
    title: 'Оформление заявки',
    description: 'Мы перешли в «Оформление и подачу заявки» - главное действие в системе. Слева выбираете бланк. От выбранного бланка зависит, какие поля появятся в форме справа. В одну заявку можно добавить сразу несколько бланков.',
  },
  {
    id: 'createapp-orginfo',
    route: '/new-application',
    element: '[data-testid="ob-app-orginfo"]',
    title: 'Шапка заявки',
    description: 'Слева добавили бланк «Автомобили» - это раздел заявки, куда вы вносите один или несколько автомобилей. Справа открылась его форма. Сверху - шапка заявки: организация, компания (отдел), ответственное лицо и телефон. Эти данные общие для всей заявки и часто подставляются автоматически.',
    demoAttachment: 'cars',
    side: 'left',
    align: 'start',
  },
  {
    id: 'createapp-custom',
    route: '/new-application',
    element: '[data-testid="ob-app-custom"]',
    title: 'Дополнительные поля',
    description: 'Если для бланка настроены дополнительные поля, они появятся здесь - заполните их. Если их нет, этот блок не показывается.',
    demoAttachment: 'cars',
    optional: true,
    side: 'left',
    align: 'start',
  },
  {
    id: 'createapp-dates',
    route: '/new-application',
    element: '[data-testid="ob-app-dates"]',
    title: 'Сроки действия',
    description: 'Указываете срок действия пропуска: дату или диапазон дат и время пребывания. У каждого бланка в заявке свои даты.',
    demoAttachment: 'cars',
    side: 'left',
    align: 'start',
  },
  {
    id: 'createapp-car-form',
    route: '/new-application',
    element: '[data-testid="ob-app-formdata"]',
    title: 'Форма «Автомобили»',
    description: 'Сама форма транспорта: формат номера, номер и марка, места разгрузки. Кнопка «Добавить» переносит авто в «Список транспортных средств» справа - в одну заявку можно внести несколько машин.',
    demoAttachment: 'cars',
    side: 'left',
    align: 'start',
  },
  {
    id: 'createapp-people-form',
    route: '/new-application',
    element: '[data-testid="ob-app-formdata"]',
    title: 'Форма «Сотрудники»',
    description: 'Для бланка «Сотрудники» форма другая: гражданство, ФИО, должность и паспортные данные. Для иностранных граждан появятся поля патента и разрешения на работу. Кнопка «Добавить» переносит сотрудника в «Список сотрудников» справа.',
    demoAttachment: 'people',
    side: 'left',
    align: 'start',
  },
  {
    id: 'finish',
    // element: null - финал показывается по центру экрана, как приветственный шаг
    // (празднование, а не подсветка конкретного элемента).
    route: '/news',
    element: null,
    title: 'Готово, вы освоились!',
    description: 'Возвращаемся на «Обзор». Это всё основное. Запустить обучение заново можно в любой момент - кнопкой «Обучение» на этой странице. Удачной работы!',
    celebrate: true,
    cta: 'Подать первую заявку',
  },
];

/**
 * Подряд идущие шаги начиная с `startIndex`, чей `route` совпадает с активной
 * страницей. Граница сегмента - первый шаг с другим route (cross-page переход).
 *
 * @param {Array<{ route: string }>} steps
 * @param {number} startIndex глобальный индекс первого шага сегмента
 * @param {string} routePath активный путь роутера
 * @returns {Array<object>}
 */
export function collectSegment(steps, startIndex, routePath) {
  const segment = [];
  for (let i = startIndex; i < steps.length; i += 1) {
    if (steps[i].route !== routePath) break;
    segment.push(steps[i]);
  }
  return segment;
}

/**
 * Индекс первого шага ПОСЛЕ непрерывного блока шагов с данным `route`, начиная с
 * `fromIndex`. Нужен, чтобы перепрыгнуть недостижимый optional-сегмент
 * (фактовая таблица, роут-гард которой редиректит охранника) к следующему шагу
 * тура - финалу-празднованию на достижимой странице. Возвращает -1, если за
 * блоком шагов не осталось.
 *
 * @param {Array<{ route: string }>} steps
 * @param {number} fromIndex индекс первого шага недостижимого блока
 * @param {string} route route недостижимого блока
 * @returns {number} индекс следующего шага за блоком или -1
 */
export function indexAfterRoute(steps, fromIndex, route) {
  let i = fromIndex;
  while (i < steps.length && steps[i].route === route) i += 1;
  return i < steps.length ? i : -1;
}
