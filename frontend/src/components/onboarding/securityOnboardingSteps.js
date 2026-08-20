/**
 * Версия тура охраны. Своя, независимая от остальных туров: подъём здесь
 * переигрывает только этот сценарий (см. hasCompleted в stores/onboarding.js).
 *
 * 2 - сквозной поиск, объяснение скрытого ФИО, пропуск по факту и отчёт по
 *     проходам; сегмент отметки перестал требовать таблицу машин.
 */
export const SECURITY_ONBOARDING_VERSION = 3;

/**
 * Шаги онбординг-тура для сотрудника охраны (тур `guard` в реестре tours.js).
 * Отдельный сценарий: охранник не подаёт заявки и не ведёт свои справочники, а
 * смотрит согласованные вложения по своим местам и отмечает въезд/выезд. Поэтому
 * тур ведёт по разделам «Доступные мне» и «Таблицы», а не по оформлению заявки.
 *
 * Структура и поля шага - общие для всех туров, полный список в JSDoc
 * `onboardingSteps.js` (включая `reveal` и `requires`). Движок группирует подряд
 * идущие шаги с общим `route` в сегмент driver.js, смена `route` = cross-page
 * граница. collectSegment живёт в onboardingSteps.js и работает с любым массивом.
 *
 * @type {Array<{ id: string, route: string, element: string|null, title: string, description: string, demo?: string, requires?: string, expandRail?: boolean, optional?: boolean, side?: string, align?: string, reveal?: { mobile?: 'nav', open?: string } }>}
 */
export const securityOnboardingSteps = [
  // ── Сегмент /news: знакомство, шапка, навигация охранника ──
  {
    id: 'sec-start',
    route: '/news',
    element: null,
    title: 'Добро пожаловать!',
    description:
      'Коротко покажем, что доступно вам как сотруднику охраны. Это займёт минуту. Двигайтесь кнопкой «Далее» или стрелками, выйти можно в любой момент клавишей Esc.',
  },
  {
    // Колокольчик - в самой шапке (не в drawer), поэтому идёт ПЕРЕД feedback:
    // так его шаг держит drawer закрытым, а два drawer-шага (feedback + nav-rail)
    // остаются смежными nav-группой. Иначе колокольчик, зажатый между двумя nav,
    // унаследовал бы 'nav' через lookahead-hold и drawer открылся бы поверх шапки.
    id: 'sec-header-notifications',
    side: 'bottom',
    route: '/news',
    element: '[data-testid="ob-header-notifications"]',
    title: 'Уведомления',
    description: 'Колокольчик показывает количество непрочитанных уведомлений системы. Сейчас откроем список.',
    advanceWhen: '[data-testid="ob-notifications-panel"]',
  },
  {
    id: 'sec-header-notifications-panel',
    side: 'bottom',
    align: 'end',
    route: '/news',
    element: '[data-testid="ob-notifications-panel"]',
    title: 'Список уведомлений',
    description:
      'Свежие события идут сверху, непрочитанные выделены. Нужное открывается прямо отсюда, а шестерёнка ведёт к настройке уведомлений.',
    optional: true,
    reveal: { open: 'notifications' },
  },
  {
    // Панель поиска выезжает справа во всю высоту окна и лежит выше подсветки
    // (z-index 15000 против 10000 у driver.js), поэтому сама кнопка на время шага
    // оказывается под ней. Поповер кладём снизу от кнопки - он рисуется поверх
    // панели, и человек видит именно то, что описано: открытое поле поиска.
    id: 'sec-header-search',
    side: 'bottom',
    align: 'end',
    route: '/news',
    element: '[data-testid="header-button-search"]',
    title: 'Поиск по системе',
    description:
      'Эта кнопка открывает поиск сразу по всей системе. С клавиатуры - Ctrl+K.',
    // Раскрытие панели тут не просим намеренно: она выезжает поверх шапки и
    // закрывает саму кнопку, о которой идёт речь, - подсветки не видно вовсе.
    // Панель разбираем следующим шагом, как в туре заявителя.
  },
  {
    id: 'sec-header-search-panel',
    side: 'left',
    route: '/news',
    element: '[data-testid="global-search-panel"]',
    title: 'Что находит поиск',
    description:
      'Машина находится и по государственному номеру, и по марке, человек - по фамилии. Опечатка и латинская раскладка поиску не мешают, а в выдачу попадает только то, что вам открыто.',
    optional: true,
    reveal: { open: 'search-panel' },
  },
  {
    id: 'sec-header-feedback',
    side: 'bottom',
    route: '/news',
    element: '[data-testid="header-button-feedback"]',
    title: 'Сообщить о проблеме',
    description: 'Заметили ошибку или есть предложение - напишите нам прямо отсюда, не покидая систему.',
    requires: 'header.report_problem',
    // На мобилке кнопка переехала из "⋯" в бургер-drawer (W3.3) - раскрываем drawer.
    reveal: { mobile: 'nav' },
  },
  {
    id: 'sec-nav-rail',
    route: '/news',
    element: '[data-testid="ob-nav-rail"]',
    title: 'Навигация',
    description:
      'Боковое меню - главный способ перемещаться по системе. Отсюда вы попадаете в нужные разделы, а внизу - кнопка «Выйти».',
    expandRail: true,
    reveal: { mobile: 'nav' },
  },
  {
    id: 'sec-nav-tables',
    route: '/news',
    element: '[data-testid="nav-link-tables"]',
    title: 'Таблицы',
    description:
      'Раздел «Таблицы» - рабочие списки постов. У каждого поста своя таблица: в одних машины, в других люди; какие из них вам открыты, видно в самом разделе.',
    // Пункт меню появляется, только когда работнику доступна хотя бы одна
    // таблица поста (NavMenu: tablesItemVisible). Тур в меню открывается и по
    // праву «Доступные мне», поэтому у такого работника раздела нет вовсе -
    // без пометки шаг ждал цель четыре секунды и показывал окно про раздел,
    // которого у человека не существует.
    optional: true,
    expandRail: true,
    reveal: { mobile: 'nav' },
  },
  {
    id: 'sec-nav-accessible',
    route: '/news',
    element: '[data-testid="nav-link-accessible-attachments"]',
    title: 'Доступные мне',
    description:
      'Раздел «Доступные мне» - согласованные бланки заявок, по которым вы пропускаете машины и людей. Здесь их видно целиком и можно открыть сам бланк. Сейчас туда и перейдём.',
    expandRail: true,
    reveal: { mobile: 'nav' },
  },
  // ── Сегмент /accessible-attachments: страница «Доступные мне» ──
  {
    id: 'sec-aa-intro',
    route: '/accessible-attachments',
    // Подсвечиваем сам список: центр-модалка без цели читалась как «тур завис».
    element: '[data-testid="aa-list"]',
    optional: true,
    scrollTo: 'start',
    title: 'Раздел «Доступные мне»',
    description:
      'Мы открыли раздел «Доступные мне». Здесь собраны согласованные бланки заявок, по которым вы пропускаете машины и людей.',
  },
  {
    id: 'sec-aa-filters',
    side: 'bottom',
    align: 'start',
    route: '/accessible-attachments',
    element: '[data-testid="aa-filters"]',
    title: 'Поиск и фильтры',
    description:
      'Сверху - поиск по заявке, машине, ФИО или месту разгрузки и фильтры: тип вложения, организация и компания. Кнопки «Завершённые» и «Ночь» (окно въезда 22:00-06:00) сужают список, «Сбросить» очищает всё.',
  },
  {
    id: 'sec-aa-card',
    side: 'right',
    align: 'start',
    optional: true,
    route: '/accessible-attachments',
    element: '[data-testid="aa-card"]',
    title: 'Карточка вложения',
    description:
      'Каждая карточка - это вложение заявки: тип, организация, статус и срок действия пропуска. Нажмите на карточку - справа откроются подробности. Сейчас откроем первую.',
    advanceWhen: '[data-testid="aa-detail"]',
  },
  {
    id: 'sec-aa-detail',
    side: 'left',
    align: 'start',
    optional: true,
    route: '/accessible-attachments',
    // Детали открываются только по выбору карточки: без раскрытия целей этих
    // шагов на экране нет, и разбор вложения выпадал целиком.
    reveal: { open: 'first-attachment' },
    element: '[data-testid="aa-detail"]',
    title: 'Детали вложения',
    description:
      'Справа - подробности выбранного вложения: данные заявки и её содержимое. Отсюда же открывается прикреплённый бланк.',
  },
  {
    id: 'sec-aa-elements',
    side: 'left',
    align: 'start',
    optional: true,
    route: '/accessible-attachments',
    reveal: { open: 'first-attachment' },
    element: '[data-testid="attachment-elements"]',
    title: 'Кого пропускать',
    description:
      'Состав вложения: машины с номерами и марками или люди с ФИО и должностями - именно их вы и пропускаете. Рядом есть поиск, чтобы быстро найти нужного в длинной заявке.',
  },
  {
    id: 'sec-aa-preview',
    side: 'left',
    align: 'start',
    optional: true,
    route: '/accessible-attachments',
    // Детали открываются только по выбору карточки: без раскрытия целей этих
    // шагов на экране нет, и разбор вложения выпадал целиком.
    reveal: { open: 'first-attachment' },
    element: '[data-testid="aa-preview-blank"]',
    title: 'Посмотреть файл',
    description:
      'Кнопка «Посмотреть файл» открывает заполненный бланк прямо в системе - скачивать его не нужно. Сейчас откроем.',
    advanceWhen: '[data-testid="ob-blank-preview"]',
  },
  {
    id: 'sec-aa-blank',
    side: 'top',
    align: 'center',
    optional: true,
    route: '/accessible-attachments',
    // Тур открывает бланк сам: рассказывать про кнопку и не показывать файл -
    // половина объяснения.
    reveal: { open: 'attachment-blank' },
    element: '[data-testid="ob-blank-preview"]',
    title: 'Бланк заявки',
    description:
      'Так выглядит заполненный бланк - то же, что подписано на бумаге. Закрывается крестиком или клавишей Esc.',
  },
  {
    // Переход в таблицу поста анонсируем отдельным шагом: прежде тур уходил туда
    // молча, и человек не понимал, куда попал и как вернуться (замечание
    // владельца 20.08). Пункт меню помечен optional - он есть не у каждого
    // охранника (нужна хотя бы одна доступная таблица поста).
    id: 'sec-nav-tables-open',
    route: '/accessible-attachments',
    element: '[data-testid="nav-link-tables"]',
    optional: true,
    expandRail: true,
    reveal: { mobile: 'nav' },
    title: 'Идём в таблицу поста',
    description:
      'Бланки посмотрели - теперь сама работа. Она идёт в разделе «Таблицы»: открываем таблицу вашего поста, из неё же потом переключаются на другие посты.',
  },
];
