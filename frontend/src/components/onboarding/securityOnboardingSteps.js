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
    description: 'Колокольчик показывает количество непрочитанных уведомлений системы. Нажмите, чтобы открыть список.',
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
      'Раздел «Таблицы» - журналы по вашим местам. Внутри открываются таблицы со списками машин и людей.',
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
      'Раздел «Доступные мне» - согласованные вложения заявок по местам, которые вы охраняете. Здесь вы смотрите оформленные пропуска и открываете их бланки. Сейчас туда и перейдём.',
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
];

/**
 * Выбрать route фактовой таблицы для сегмента отметки въезда/выезда из списка
 * системных таблиц (тот же `/system-tables`, что NavMenu показывает в дропдауне
 * «Таблицы»). Форма элемента - `{ table: {...} }` (как отдаёт GetAll) либо плоская.
 *
 * Подходит ЛЮБАЯ активная фактовая таблица, а не только машинная: на посту, где
 * заведена одна таблица людей, прежний отбор по `table_type === 'cars'` не находил
 * ничего, и весь сегмент отметки молча пропадал из тура. Машины остаются в
 * приоритете - это основной сценарий поста, и кнопки «Въезд»/«Выезд» есть только у
 * них; тексты шагов сегмента написаны нейтрально к типу таблицы, а шаги самих
 * кнопок помечены `optional` и на таблице людей пропускаются.
 *
 * `canView` отсеивает таблицы, которые пользователю не открыть: роут
 * `/table/:tableName` гейтится правом `table.<name>.view`, и без отсева сегмент мог
 * указать на чужую таблицу, с которой роут-гард сразу уводит. Без предиката отбор
 * идёт только по составу таблиц - как раньше.
 *
 * @param {Array<object>} systemTables ответ GET /system-tables
 * @param {(name: string) => boolean} [canView] доступна ли таблица пользователю
 * @returns {string|null} `/table/<name>` или null, если подходящей таблицы нет
 */
export function resolveFactTableRoute(systemTables, canView) {
  if (!Array.isArray(systemTables)) return null;
  let otherType = null;
  for (const item of systemTables) {
    const t = item?.table || item;
    if (!t || !t.is_active || !t.show_fact_table || !t.name) continue;
    if (typeof canView === 'function' && !canView(t.name)) continue;
    if (t.table_type === 'cars') return `/table/${t.name}`;
    if (!otherType) otherType = `/table/${t.name}`;
  }
  return otherType;
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
 * `optional`: на пустой фактовой таблице (строк нет) они пропускаются, а кнопок
 * «Въезд»/«Выезд» нет вовсе, когда таблица про людей.
 *
 * Шаг отчёта тоже `optional`, а не `requires`: кнопка гейтится правом
 * `table.<имя таблицы>.report`, то есть ключ у каждой таблицы свой и статическим
 * `requires` не выражается.
 *
 * @param {string|null} route `/table/<name>` целевой фактовой таблицы
 * @returns {Array<object>} шаги сегмента (пустой массив при отсутствии route)
 */
export function buildSecurityFactSteps(route) {
  if (!route) return [];
  return [
    {
      id: 'sec-table-instruction',
      route,
      element: '[data-testid="ob-table-instruction"]',
      optional: true,
      // Первый шаг сегмента несёт признак недостижимости: если роут таблицы
      // закрыт гардом, хост перепрыгивает весь сегмент к финалу.
      optionalSegment: true,
      side: 'bottom',
      align: 'start',
      title: 'Инструкция поста',
      description:
        'Бюро пропусков пишет здесь порядок работы именно вашего поста: что проверять, кого пропускать, куда звонить. Если сомневаетесь - смотрите сюда.',
    },
    {
      id: 'sec-pass-intro',
      route,
      // Основная работа поста - таблица «по заявке»: в ней машины и люди из
      // согласованных заявок. Тур раньше показывал только блок «по факту»
      // (ручной ввод) и молчал про главную таблицу.
      element: '[data-testid="cars-table"]',
      optional: true,
      scrollTo: 'start',
      title: 'Таблица поста',
      description:
        'Список «по заявке» - те, кому пропуск уже согласован и кто должен приехать на ваш пост. Здесь вы и отмечаете проезд.',
    },
    {
      id: 'sec-pass-row',
      route,
      element: '[data-testid="ob-pass-row"]',
      optional: true,
      side: 'bottom',
      align: 'start',
      title: 'Строка списка',
      description:
        'Строка - одна запись из заявки: номер и марка машины или ФИО человека, рядом организация, номер заявки и срок действия пропуска. Нажатие на строку раскрывает подробности.',
    },
    {
      id: 'sec-pass-entry',
      route,
      element: '[data-testid="ob-pass-entry"]',
      optional: true,
      side: 'bottom',
      align: 'start',
      title: 'Отметка въезда',
      description:
        'Кнопкой «Въезд» отмечаете, что машина заехала. После отметки кнопка становится зелёной и больше не нажимается - повторно въезд не поставить.',
    },
    {
      id: 'sec-pass-exit',
      route,
      element: '[data-testid="ob-pass-exit"]',
      optional: true,
      side: 'bottom',
      align: 'start',
      title: 'Отметка выезда',
      description:
        'После въезда становится активной кнопка «Выезд» - нажмите её, когда машина покинет территорию.',
    },
    {
      id: 'sec-on-territory',
      route,
      element: '[data-testid="ob-on-territory"]',
      optional: true,
      side: 'bottom',
      align: 'end',
      title: 'Сколько машин на территории',
      description:
        'Счётчик показывает, сколько машин заехало и ещё не выехало. Он меняется сам, как только вы отмечаете въезд или выезд.',
    },
    {
      id: 'sec-fact-intro',
      route,
      element: '[data-testid="fact-table"]',
      optional: true,
      scrollTo: 'start',
      title: 'Приехали без заявки',
      description:
        'Список «по факту» - те, кого в заявке не было: машину пропустили по месту, и запись осталась здесь. При таком пропуске система спрашивает формат и номер Т/С, марку можно не указывать.',
    },
    {
      id: 'sec-fact-report',
      route,
      element: '[data-testid="pass-report-button"]',
      optional: true,
      side: 'bottom',
      align: 'end',
      title: 'Отчёт по проходам',
      description: 'Кнопка «Отчёт» подводит итог по проходам. Сейчас откроем его.',
      advanceWhen: '[data-testid="ob-pass-report"]',
    },
    {
      id: 'sec-fact-report-window',
      route,
      element: '[data-testid="ob-pass-report"]',
      optional: true,
      side: 'left',
      // Тур открывает отчёт сам - иначе рассказ о нём остаётся словами.
      reveal: { open: 'pass-report' },
      title: 'Что в отчёте',
      description:
        'Сколько машин заехало и выехало за сутки: они считаются с 21:30 предыдущего дня до 21:30 текущего, поэтому сразу после 21:30 счётчики пустые - начались новые сутки. Если отмечал не один человек, видно, кто сколько отметил, а «Показать прошлые дни» открывает прошедшие сутки.',
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
      'Вы освоились: смотрите согласованные пропуска в разделе «Доступные мне», отмечаете въезд и выезд в таблице поста и в любой момент можете свериться с отчётом по проходам. Можно открыть «Доступные мне» прямо сейчас.',
  };
}
