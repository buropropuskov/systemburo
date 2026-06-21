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

/**
 * Выбрать route фактовой таблицы для шага отметки въезда/выезда из списка
 * системных таблиц (тот же `/system-tables`, что NavMenu показывает в дропдауне
 * «Таблицы»). Берём первую активную фактовую таблицу типа `cars`: кнопки
 * «Въезд»/«Выезд» в FactTable есть только у машин (people-фактовая таблица их
 * не имеет). Форма элемента - `{ table: {...} }` (как отдаёт GetAll) либо плоская.
 *
 * @param {Array<object>} systemTables ответ GET /system-tables
 * @returns {string|null} `/table/<name>` или null, если подходящей таблицы нет
 */
export function resolveFactTableRoute(systemTables) {
  if (!Array.isArray(systemTables)) return null;
  for (const item of systemTables) {
    const t = item?.table || item;
    if (t && t.is_active && t.show_fact_table && t.table_type === 'cars' && t.name) {
      return `/table/${t.name}`;
    }
  }
  return null;
}

/**
 * Сегмент «Таблицы и отметка въезда/выезда» с ДИНАМИЧЕСКИМ route: целевую
 * фактовую таблицу резолвим в рантайме (resolveFactTableRoute) и подставляем
 * сюда, потому что у разных охранников разные доступные таблицы. Сегмент
 * добавляется в хвост `steps` стора только когда route резолвится - индексы
 * ранних шагов он не сдвигает.
 *
 * Первый шаг помечен `optionalSegment`: если у охранника пока нет доступа к
 * `/table/:name` (роут-гард редиректит), хост штатно завершает тур на границе
 * сегмента, а не висит на недостижимой странице. Когда доступ выдан -
 * навигация проходит, и шаги подсвечивают реальную таблицу. Шаги строки/кнопок
 * `optional`: на пустой фактовой таблице (строк нет) они пропускаются.
 *
 * @param {string|null} route `/table/<name>` целевой фактовой таблицы
 * @returns {Array<object>} шаги сегмента (пустой массив при отсутствии route)
 */
export function buildSecurityFactSteps(route) {
  if (!route) return [];
  return [
    {
      id: 'sec-fact-intro',
      route,
      element: null,
      optionalSegment: true,
      title: 'Отметка въезда и выезда',
      description:
        'Открыли таблицу «Автомобили по факту»: сюда попадает транспорт по согласованным заявкам. Здесь охрана отмечает въезд и выезд машин.',
    },
    {
      id: 'sec-fact-row',
      route,
      element: '[data-testid="ob-fact-row"]',
      optional: true,
      side: 'bottom',
      align: 'start',
      title: 'Строка транспорта',
      description:
        'Каждая строка - автомобиль по согласованной заявке: номер, организация, время и срок действия пропуска.',
    },
    {
      id: 'sec-fact-entry',
      route,
      element: '[data-testid="ob-fact-entry"]',
      optional: true,
      side: 'bottom',
      align: 'start',
      title: 'Отметка въезда',
      description: 'Кнопкой «Въезд» отмечаете, что автомобиль заехал на территорию.',
    },
    {
      id: 'sec-fact-exit',
      route,
      element: '[data-testid="ob-fact-exit"]',
      optional: true,
      side: 'bottom',
      align: 'start',
      title: 'Отметка выезда',
      description:
        'После отметки въезда станет активной кнопка «Выезд» - нажмите её, когда автомобиль покинет территорию.',
    },
  ];
}

/** Route раздела «Доступные мне» - всегда достижим охранником (canViewAccessibleAttachments). */
export const ACCESSIBLE_ATTACHMENTS_ROUTE = '/accessible-attachments';

/**
 * Финальный шаг security-тура: празднование (`celebrate`) и CTA в раздел
 * «Доступные мне» (`ctaRoute`), а НЕ на оформление заявки - охранник пропуска
 * не подаёт. Шаг строится отдельно (не лежит в базовом массиве), чтобы быть
 * ПОСЛЕДНИМ независимо от того, добавлен ли динамический сегмент фактовой
 * таблицы.
 *
 * Финал всегда на `/accessible-attachments` - единственной странице тура,
 * гарантированно достижимой охранником. Сегмент фактовой таблицы опционален и
 * может быть недостижим (роут-гард `/table/:name`): если бы финал жил на его
 * route, охранник без доступа к таблицам не дошёл бы до празднования. На
 * достижимой странице финал показывается всегда - либо в одном сегменте с
 * шагами «Доступные мне» (когда фактовой таблицы нет), либо после перехода с
 * фактовой таблицы / перепрыгивания недостижимого сегмента (см. OnboardingTour).
 *
 * @returns {object} финальный шаг
 */
export function buildSecurityFinalStep() {
  return {
    id: 'sec-finish',
    route: ACCESSIBLE_ATTACHMENTS_ROUTE,
    element: null,
    celebrate: true,
    cta: 'Перейти к «Доступным мне»',
    ctaRoute: ACCESSIBLE_ATTACHMENTS_ROUTE,
    title: 'Готово!',
    description:
      'Вы освоились: смотрите согласованные вложения по своим местам в разделе «Доступные мне» и отмечаете въезд и выезд машин в таблицах. Можно открыть «Доступные мне» прямо сейчас - там уже ждут оформленные пропуска.',
  };
}
