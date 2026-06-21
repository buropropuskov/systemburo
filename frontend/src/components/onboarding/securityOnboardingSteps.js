/**
 * Шаги онбординг-тура для сотрудника охраны (auth.isSecurity). Отдельный сценарий:
 * охранник не подаёт заявки и не ведёт свои справочники, а смотрит согласованные
 * вложения по своим местам и отмечает въезд/выезд. Поэтому тур ведёт по разделам
 * «Доступные мне» и «Таблицы», а не по оформлению заявки.
 *
 * Структура и поля шага - те же, что у applicant-тура (см. onboardingSteps.js):
 * движок группирует подряд идущие шаги с общим `route` в сегмент driver.js, смена
 * `route` = cross-page граница. Версия тура и флаг завершения общие с основным
 * сценарием - аргументация в stores/onboarding.js, где ветвится `steps`.
 * collectSegment живёт в onboardingSteps.js и работает с любым массивом шагов.
 *
 * @type {Array<{ id: string, route: string, element: string|null, title: string, description: string, expandRail?: boolean, optional?: boolean, side?: string, align?: string }>}
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
    id: 'sec-header-feedback',
    side: 'bottom',
    route: '/news',
    element: '[data-testid="header-button-feedback"]',
    title: 'Сообщить о проблеме',
    description: 'Заметили ошибку или есть предложение - напишите нам прямо отсюда, не покидая систему.',
  },
  {
    id: 'sec-header-time',
    side: 'bottom',
    route: '/news',
    element: '[data-testid="ob-header-time"]',
    title: 'Дата и время',
    description: 'Текущие дата и время системы - удобно сверяться при проверке пропусков.',
  },
  {
    id: 'sec-header-notifications',
    side: 'bottom',
    route: '/news',
    element: '[data-testid="ob-header-notifications"]',
    title: 'Уведомления',
    description: 'Колокольчик показывает количество непрочитанных уведомлений системы. Нажмите, чтобы открыть список.',
  },
  {
    id: 'sec-nav-rail',
    route: '/news',
    element: '[data-testid="ob-nav-rail"]',
    title: 'Навигация',
    description:
      'Боковое меню - главный способ перемещаться по системе. Отсюда вы попадаете в нужные разделы, а внизу - кнопка «Выйти».',
    expandRail: true,
  },
  {
    id: 'sec-nav-accessible',
    route: '/news',
    element: '[data-testid="nav-link-accessible-attachments"]',
    title: 'Доступные мне',
    description:
      'Раздел «Доступные мне» - согласованные вложения заявок по местам, которые вы охраняете. Здесь вы смотрите оформленные пропуска и открываете их бланки. Сейчас перейдём туда.',
    expandRail: true,
  },
  {
    id: 'sec-nav-tables',
    route: '/news',
    element: '[data-testid="nav-link-tables"]',
    title: 'Таблицы',
    description:
      'Раздел «Таблицы» - журналы по вашим местам. Внутри открываются таблицы со списками машин и людей.',
    expandRail: true,
  },
  // ── Сегмент /accessible-attachments: страница «Доступные мне» ──
  {
    id: 'sec-aa-intro',
    route: '/accessible-attachments',
    element: null,
    title: 'Раздел «Доступные мне»',
    description:
      'Мы открыли раздел «Доступные мне». Здесь собраны согласованные вложения заявок по местам, которые вы охраняете.',
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
      'Каждая карточка - это вложение: тип, организация, статус и срок действия. Нажмите на карточку, чтобы справа открылись детали со списком машин или людей.',
  },
  {
    id: 'sec-aa-detail',
    side: 'left',
    align: 'start',
    optional: true,
    route: '/accessible-attachments',
    element: '[data-testid="aa-detail"]',
    title: 'Детали вложения',
    description:
      'Справа - подробности выбранного вложения: данные заявки и её содержимое. Отсюда же открывается прикреплённый бланк.',
  },
  {
    id: 'sec-aa-preview',
    side: 'left',
    align: 'start',
    optional: true,
    route: '/accessible-attachments',
    element: '[data-testid="aa-preview-blank"]',
    title: 'Посмотреть файл',
    description:
      'Кнопка «Посмотреть файл» открывает заполненный бланк вложения прямо в системе - скачивать его для просмотра не нужно.',
  },
];
