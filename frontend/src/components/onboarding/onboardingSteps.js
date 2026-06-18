/**
 * Версия тура. Записывается в localStorage-флаг завершения и сверяется при
 * чтении: повышение версии (новые шаги) заставит тур считаться непройденным
 * и переиграться при авто-запуске. Поднимать при добавлении/изменении шагов.
 */
export const ONBOARDING_VERSION = 1;

/**
 * Плоский упорядоченный массив шагов тура. Движок группирует ПОДРЯД идущие
 * шаги с одинаковым `route` в «сегмент» драйвера; смена `route` = граница
 * сегмента (cross-page переход достраивается отдельным срезом).
 *
 * Поля шага:
 * - `id`         уникальный строковый ключ;
 * - `route`      путь страницы, на которой шаг живёт;
 * - `element`    CSS-селектор цели или `null` (центр-модал без подсветки);
 * - `title`      заголовок поповера;
 * - `description` HTML-тело поповера (прогоняется через sanitizeHtml);
 * - `demo`       необязательный ключ скриншота (используется будущими срезами);
 * - `expandRail` если true - хост разворачивает свёрнутый рельс навигации на
 *                время шага и возвращает прежнее состояние при выходе.
 *
 * @type {Array<{ id: string, route: string, element: string|null, title: string, description: string, demo?: string, expandRail?: boolean }>}
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
      'Здесь публикуются новости системы и важные объявления. Заглядывайте сюда, чтобы быть в курсе изменений.',
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
    id: 'documents',
    route: '/news',
    element: '[data-testid="ob-documents"]',
    title: 'Документы',
    description: 'Полезные документы и шаблоны, доступные для скачивания.',
  },
  {
    id: 'header-broadcast',
    route: '/news',
    element: '[data-testid="ob-header-broadcast"]',
    title: 'Объявления',
    description: 'Здесь появляется кнопка важного объявления, когда администрация публикует что-то срочное. Нажмите, чтобы прочитать.',
  },
  {
    id: 'header-time',
    route: '/news',
    element: '[data-testid="ob-header-time"]',
    title: 'Дата и время',
    description: 'Текущие дата и время системы - удобно сверяться при оформлении заявок.',
  },
  {
    id: 'header-notifications',
    route: '/news',
    element: '[data-testid="ob-header-notifications"]',
    title: 'Уведомления',
    description: 'Колокольчик показывает количество непрочитанных уведомлений: смена статуса заявки, ответы согласующих и другие события. Нажмите, чтобы открыть список.',
  },
  {
    id: 'header-feedback',
    route: '/news',
    element: '[data-testid="header-button-feedback"]',
    title: 'Сообщить о проблеме',
    description: 'Заметили ошибку или есть предложение - напишите нам прямо отсюда, не покидая систему.',
  },
  {
    id: 'header-submit',
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
    description: 'Боковое меню - главный способ перемещаться по системе. Наведите курсор, чтобы развернуть его с подписями разделов.',
    expandRail: true,
  },
  {
    id: 'nav-group-data',
    route: '/news',
    element: '[data-testid="ob-nav-group-data"]',
    title: 'Управление данными',
    description: 'Раздел «Управление данными»: таблицы системы, ваши сотрудники и автомобили. Отсюда вы ведёте справочники, по которым оформляются пропуска.',
    expandRail: true,
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
