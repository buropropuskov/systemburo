
/**
 * Версия тура заявителя. Сверяется с пройденной версией, полученной с бэкенда:
 * повышение версии (новые шаги) заставит тур считаться непройденным и
 * переиграться при авто-запуске. Поднимать при добавлении/изменении шагов.
 * У каждого тура версия своя - подъём одной не сбрасывает остальные.
 */
export const ONBOARDING_VERSION = 3;

/**
 * Плоский упорядоченный массив шагов тура. Движок группирует ПОДРЯД идущие
 * шаги с одинаковым `route` в «сегмент» драйвера; смена `route` = граница
 * сегмента (cross-page переход).
 *
 * Поля шага (общие для ВСЕХ туров реестра tours.js):
 * - `id`         уникальный строковый ключ;
 * - `route`      путь страницы, на которой шаг живёт;
 * - `element`    CSS-селектор цели или `null` (центр-модал без подсветки);
 * - `title`      заголовок поповера;
 * - `description` HTML-тело поповера (прогоняется через sanitizeHtml);
 * - `demo`       ключ скриншота (мапа в onboardingDemo.js). Картинка ЗАМЕНЯЕТ живой
 *                экран, поэтому рисуется только когда цели шага на экране нет;
 *                рядом с подсвеченным элементом она была бы дублем;
 * - `requires`   ключ права: без него шаг молча выбрасывается из набора (стор
 *                фильтрует `steps`), не попадая ни в навигацию, ни в счётчик
 *                «Шаг N из M». Не путать с `optional`: `requires` - «этому
 *                пользователю такого элемента не положено», `optional` - «элемент
 *                может не отрисоваться» (данных нет);
 * - `waitFor`    селектор ОЖИДАНИЯ, если он строже подсвечиваемого: форма заявки
 *                живёт в одном узле для всех бланков, и «форма появилась» ещё не
 *                значит «перерисовалась под нужный бланк». По умолчанию ждём сам
 *                `element`;
 * - `optional`   элемента может не быть в DOM - шаг ждёт цель коротко и
 *                пропускается, если она не появилась (пропущенный шаг выпадает из
 *                счётчика «Шаг N из M»). Вместе с `demo` пропуска нет: у нового
 *                пользователя система пуста, и вместо подсветки шаг показывается
 *                центр-модалом со скриншотом - и считается наравне с обычными;
 * - `optionalSegment` весь сегмент может быть недостижим (роут-гард) - хост
 *                перепрыгивает его к следующему достижимому шагу;
 * - `expandRail` если true - хост держит рельс навигации развёрнутым на время
 *                шага (и шага перед ним), возвращает прежнее состояние при выходе;
 * - `celebrate`  если true - в поповере рисуется галочка-празднование (финал);
 * - `cta`        текст финальной кнопки-CTA;
 * - `ctaRoute`   куда ведёт CTA (по умолчанию - оформление заявки);
 * - `demoAttachment` тип демо-вложения ('cars'/'people'), которое BlankSelector
 *                добавит на время шага, чтобы показать реальную форму;
 * - `dynamic`    карта «имя -> селектор»: `{имя}` в заголовке и описании заменяется
 *                текстом узла. Для того, что называет не система, а Бюро (имена
 *                бланков);
 * - `advanceWhen` селектор: если во время шага этот узел появился, тур сам идёт
 *                дальше. Шаг-приглашение к действию («Откройте заявку») иначе
 *                оставался бы с подсветкой под открытым окном;
 * - `scrollTo`   куда подвести цель перед показом: 'end' прижимает её к низу
 *                экрана. Нужно высоким блокам (форма сотрудников), над которыми
 *                встаёт поповер: по центру он накрывал ровно то, что объясняет;
 * - `side`       сторона поповера от элемента (top/bottom/left/right) - чтобы
 *                карточка не наезжала на выделенный элемент;
 * - `align`      выравнивание поповера вдоль стороны (start/center/end);
 * - `reveal`     раскрытие свёрнутого узла ПЕРЕД подсветкой, две независимые оси:
 *                `{ mobile: 'nav' }` - на <=768px цель уехала в бургер-drawer
 *                NavMenu (на десктопе ось не читается - узлы всегда на месте);
 *                `{ open: 'admin-column'|'search-panel'|'first-application' }` -
 *                узел свёрнут на любой ширине и появляется только по действию
 *                пользователя. Механика - в reveal.js.
 *
 * @type {Array<{ id: string, route: string, element: string|null, waitFor?: string, dynamic?: Record<string, string>, advanceWhen?: string, scrollTo?: 'center'|'end'|'start', title: string, description: string, demo?: string, requires?: string, optional?: boolean, optionalSegment?: boolean, expandRail?: boolean, celebrate?: boolean, cta?: string, ctaRoute?: string, demoAttachment?: string, side?: string, align?: string, reveal?: { mobile?: 'nav', open?: 'admin-column'|'search-panel'|'first-application' } }>}
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
      'Объявления администрации. Здесь появляются обычные и важные объявления - так вы не пропустите срочную информацию.',
  },
  {
    id: 'documents',
    route: '/news',
    element: '[data-testid="ob-documents"]',
    title: 'Документы',
    description: 'Полезные документы и шаблоны, доступные для скачивания.',
  },
  {
    id: 'work-modes',
    route: '/news',
    element: '[data-testid="ob-work-modes"]',
    title: 'Режимы работы',
    description: 'Кнопка «Режимы работы» открывает расписание Бюро, мест разгрузки и мест прохода. Сейчас откроем.',
    advanceWhen: '[data-testid="work-modes-modal"]',
  },
  {
    id: 'work-modes-window',
    route: '/news',
    element: '[data-testid="work-modes-modal"]',
    title: 'Расписание',
    description:
      'По каждой позиции видно, открыта она сейчас, закрыта или неактивна, и какой график действует сегодня. Сверьтесь с ним до подачи заявки: если время пребывания не попадает в график места, форма об этом предупредит.',
    optional: true,
    // Тур открывает расписание сам - рассказ о кнопке без самого расписания
    // человеку ничего не даёт.
    reveal: { open: 'work-modes' },
  },
  {
    id: 'header-broadcast',
    side: 'bottom',
    route: '/news',
    element: '[data-testid="ob-header-broadcast"]',
    title: 'Объявление в шапке',
    description:
      'Пока объявление действует, оно висит в шапке подписью «Объявление» или «Важное объявление» и видно из любого раздела, а не только на «Обзоре».',
    // Пилюля рисуется только при активном объявлении - без него шага быть не должно.
    optional: true,
  },
  {
    id: 'header-feedback',
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
    id: 'header-notifications',
    side: 'bottom',
    route: '/news',
    element: '[data-testid="ob-header-notifications"]',
    title: 'Уведомления',
    description: 'Колокольчик показывает количество непрочитанных уведомлений: смена статуса заявки, ответы согласующих и другие события. Сейчас откроем список.',
    // Человек может открыть список прямо сейчас - тогда тур перейдёт к разбору
    // сам, а не оставит открытый список лежать под затемнением.
    advanceWhen: '[data-testid="ob-notifications-panel"]',
  },
  {
    id: 'header-notifications-panel',
    side: 'bottom',
    align: 'end',
    route: '/news',
    element: '[data-testid="ob-notifications-panel"]',
    title: 'Список уведомлений',
    description:
      'Свежие события идут сверху, непрочитанные выделены. Нужное открывается прямо отсюда, а шестерёнка ведёт к настройке: какие уведомления получать и куда.',
    optional: true,
    reveal: { open: 'notifications' },
  },
  {
    id: 'header-submit',
    side: 'bottom',
    route: '/news',
    element: '[data-testid="header-button-submit-app"]',
    title: 'Подать заявку',
    description: 'Быстрый переход к оформлению новой заявки на пропуск - кнопка доступна из любого раздела.',
    requires: 'header.create_application',
  },
  {
    id: 'header-search',
    side: 'bottom',
    align: 'end',
    route: '/news',
    element: '[data-testid="header-button-search"]',
    title: 'Поиск по системе',
    description:
      'Кнопка поиска открывает панель, где ищется всё сразу: человек, машина, заявка, раздел или действие. Сочетание Ctrl+K открывает её с клавиатуры, не отрывая рук.',
  },
  {
    // Отдельным шагом от кнопки: пока панель не раскрыта, рассказывать про её
    // содержимое не о чем, а раскрывать её сразу - значит спрятать саму кнопку,
    // которую человек и должен запомнить.
    id: 'header-search-panel',
    // Панель занимает правый край экрана, поэтому поповер уводим влево от неё:
    // снизу он ложился прямо на результаты поиска, о которых и рассказывает.
    side: 'left',
    align: 'start',
    route: '/news',
    element: '[data-testid="global-search-panel"]',
    title: 'Что показывает поиск',
    description:
      'Начните вводить - совпадения появятся сразу и будут разбиты по разделам. Поиск понимает опечатки, латиницу и неверную раскладку клавиатуры: «bdfyjd» найдёт Иванова. Панель можно закрепить раскрытой или свернуть в столбик.',
    optional: true,
    reveal: { open: 'search-panel' },
  },
  {
    id: 'nav-rail',
    route: '/news',
    element: '[data-testid="ob-nav-rail"]',
    title: 'Навигация',
    description: 'Боковое меню - главный способ перемещаться по системе. Отсюда вы попадаете в любой раздел, в свой Личный кабинет, а внизу - кнопка «Выйти».',
    expandRail: true,
    reveal: { mobile: 'nav' },
  },
  {
    id: 'nav-group-data',
    route: '/news',
    element: '[data-testid="ob-nav-group-data"]',
    title: 'Управление данными',
    description: 'Раздел «Управление данными»: ваши сотрудники и автомобили. Здесь вы ведёте справочники, по которым оформляются пропуска.',
    expandRail: true,
    reveal: { mobile: 'nav' },
  },
  {
    id: 'nav-theme',
    route: '/news',
    element: '[data-testid="nav-theme-toggle"]',
    title: 'Тёмная тема',
    description:
      'Переключатель «Тёмная тема» меняет оформление всей системы. Выбор сохраняется в вашем профиле, поэтому при следующем входе, в том числе с другого устройства, система откроется в той же теме.',
    expandRail: true,
    reveal: { mobile: 'nav' },
  },
  {
    id: 'cabinet-profile',
    route: '/personal-cabinet',
    element: '[data-testid="ob-profile"]',
    title: 'Личный кабинет',
    description: 'Мы перешли в ваш Личный кабинет. Вверху - ваши данные: организация, должность, контакты. Они автоматически подставляются в заявки.',
  },
  {
    id: 'cabinet-password',
    route: '/personal-cabinet',
    element: '[data-testid="cabinet-change-password"]',
    title: 'Смена пароля',
    description: 'Кнопка «Сменить пароль» в этом же ряду. Понадобится текущий пароль и новый, требования к нему система покажет прямо в окне. После смены нужно будет войти заново.',
  },
  {
    id: 'cabinet-applications',
    route: '/personal-cabinet',
    element: '[data-testid="ob-applications"]',
    title: 'Ваши заявки',
    description: 'Список поданных вами заявок: номер, дата, статус. Свежие изменения помечаются, чтобы не пропустить ответ по своей заявке.',
    demo: 'applications',
    side: 'bottom',
  },
  {
    // Поиск - часть работы со списком, поэтому идёт здесь, а не после разбора
    // карточки: иначе тур возвращает к списку, от которого уже ушёл.
    id: 'cabinet-search',
    route: '/personal-cabinet',
    element: '[data-testid="ob-cabinet-search"]',
    title: 'Поиск по своим заявкам',
    description: 'Когда заявок много, ищите по номеру, организации или содержимому. Рядом - сортировка по дате и статусу.',
    optional: true,
    side: 'bottom',
  },
  {
    id: 'cabinet-download',
    route: '/personal-cabinet',
    element: '[data-testid="ob-application-download"]',
    title: 'Скачать бланки',
    description:
      'Кнопка «Скачать» в строке заявки открывает окно «Скачивание бланков»: заполненные бланки можно взять по одному или все разом. Она есть только у заявок, для вложений которых бланки настроены.',
    // Кнопки нет у заявок без бланков; на телефоне её убрали из строки - там
    // скачивание живёт в самой карточке.
    optional: true,
    side: 'left',
  },
  {
    // Показываем строку до открытия карточки: иначе окно появляется само собой
    // и человек не понимает, откуда оно взялось.
    id: 'cabinet-application-row',
    route: '/personal-cabinet',
    element: '[data-testid="ob-application-row"]',
    title: 'Откройте заявку',
    description: 'Клик по строке открывает карточку заявки - всё о ней в одном окне. Нажмите на любую или просто идите дальше: следующим шагом откроем первую сами.',
    // Человек может нажать на строку прямо сейчас - тогда карточка появится
    // поверх подсвеченной строки, и тур обязан перейти к рассказу о ней сам,
    // иначе подсветка останется под открытым окном.
    advanceWhen: '[data-testid="ob-detail-card"]',
    optional: true,
    side: 'bottom',
  },
  {
    id: 'detail-opened',
    route: '/personal-cabinet',
    // Карточка целиком, а не её шапка: шаг знакомит с окном, и подсвеченная
    // полоска сверху читалась как «тур показывает заголовок».
    element: '[data-testid="ob-detail-card"]',
    title: 'Вот ваша заявка',
    description: 'Окно заявки: сверху - номер, дата подачи и действия над ней; ниже - состав заявки и её согласование. Закрывается крестиком справа или клавишей Esc.',
    // У нового пользователя заявок нет, открывать нечего - тогда шаг показывается
    // со скриншотом-примером вместо подсветки, а не выпадает молча вместе со всем
    // сегментом карточки.
    demo: 'applicationDetail',
    optional: true,
    side: 'bottom',
    reveal: { open: 'first-application' },
  },
  {
    id: 'detail-status',
    route: '/personal-cabinet',
    element: '[data-testid="ob-detail-status"]',
    title: 'Статус и согласующие',
    description:
      'Блок «Согласование заявки» показывает, на какой стадии заявка, а если у неё есть согласующие - список «Ответственные за согласование»: у каждого свой статус и комментарий. Помеченные «Обязательно» решают судьбу заявки: без их согласия принять её в работу нельзя.',
    optional: true,
    side: 'left',
    reveal: { open: 'first-application' },
  },
  {
    id: 'detail-status-section',
    route: '/personal-cabinet',
    element: '[data-testid="ob-detail-status-section"]',
    title: 'Статус заявки',
    description:
      'Когда заявку приняли в работу, отказали или завершили, здесь появляется блок «Статус заявки»: кто принял решение, когда и с каким комментарием. У заявки на согласовании его ещё нет.',
    optional: true,
    side: 'left',
    reveal: { open: 'first-application' },
  },
  {
    id: 'detail-questions',
    route: '/personal-cabinet',
    element: '[data-testid="application-questions"]',
    title: 'Вопросы к заявке',
    description:
      'Блок «Вопросы к заявке» разворачивается по заголовку. Кнопка «Задать вопрос» заводит новый вопрос, ответы собираются в тред под ним. Так уточняют детали, не звоня и не подавая заявку заново; непрочитанное помечается меткой «Новое».',
    optional: true,
    side: 'bottom',
    reveal: { open: 'first-application' },
  },
  {
    id: 'detail-actions-intro',
    route: '/personal-cabinet',
    element: '[data-testid="ob-detail-header"]',
    title: 'Что можно сделать с заявкой',
    description:
      'В шапке карточки собраны действия: дополнить, продублировать, скачать бланки, отозвать. Набор зависит от стадии заявки и ваших прав - часть кнопок появляется не всегда. Разберём их по очереди.',
    optional: true,
    side: 'bottom',
    align: 'start',
    reveal: { open: 'first-application' },
  },
  {
    id: 'detail-supplement',
    route: '/personal-cabinet',
    element: '[data-testid="app-detail-button-supplement"]',
    title: 'Дополнить поданную заявку',
    description:
      'Кнопка «Дополнить» добавляет машины, сотрудников или позиции в уже поданную заявку - вторую подавать не нужно. Если заявка уже в работе, добавка уйдёт на отдельный круг согласования, а выданные пропуска продолжат действовать.',
    optional: true,
    requires: 'action.supplement.application',
    side: 'bottom',
    reveal: { open: 'first-application' },
  },
  {
    id: 'detail-duplicate',
    route: '/personal-cabinet',
    element: '[data-testid="ob-detail-duplicate"]',
    title: 'Продублировать заявку',
    description:
      'Ездит один и тот же транспорт - не заполняйте всё заново. «Продублировать» создаёт копию заявки со всем содержимым, а из списка сразу выбирается срок для копии: на сегодня, на завтра, на неделю.',
    optional: true,
    side: 'bottom',
    reveal: { open: 'first-application' },
  },
  {
    id: 'detail-revoke',
    route: '/personal-cabinet',
    element: '[data-testid="ob-detail-revoke"]',
    title: 'Отозвать свою заявку',
    description:
      'Передумали или ошиблись - кнопка «Отозвать» снимает заявку с рассмотрения, система переспросит перед этим. Отозвать можно только свою заявку и только пока она не закрыта.',
    optional: true,
    side: 'bottom',
    reveal: { open: 'first-application' },
  },
  {
    id: 'cabinet-notifications',
    route: '/personal-cabinet',
    element: '[data-testid="cabinet-notifications"]',
    title: 'Ваши уведомления',
    description: 'Лента событий по вашим заявкам: согласование, одобрение, отказ. Так вы всегда в курсе, что происходит.',
  },
  {
    id: 'cabinet-notifications-settings',
    route: '/personal-cabinet',
    element: '[data-testid="cabinet-notifications-settings"]',
    title: 'Что вам будет приходить',
    description: 'Кнопка «Настроить» открывает страницу настройки уведомлений - сейчас заглянем туда и разберём её.',
  },
  {
    id: 'notif-push',
    route: '/notification-settings',
    element: '[data-testid="webpush-block"]',
    title: 'Уведомления на телефон',
    description:
      'Push-уведомления доходят, даже когда система закрыта: разрешите их в браузере на своём устройстве, и о смене статуса заявки узнаете сразу. Разрешение даётся на каждом устройстве отдельно.',
    optional: true,
    side: 'bottom',
  },
  {
    id: 'notif-categories',
    route: '/notification-settings',
    // Подсвечиваем ОДИН раздел, а не весь список: он длиннее экрана, и вырез
    // уходил за верхний край.
    element: '[data-testid="ob-notif-category"]',
    title: 'Что присылать',
    description:
      'Типы уведомлений собраны по разделам - вот один из них. Переключатель на разделе гасит или включает его целиком, а внутри можно оставить только нужное: например, ответы согласующих, но без напоминаний.',
    optional: true,
    side: 'bottom',
    align: 'start',
  },
  {
    id: 'notif-save',
    route: '/notification-settings',
    element: '[data-testid="notif-settings-save"]',
    title: 'Не забудьте сохранить',
    description: 'Изменения вступают в силу после «Сохранить». Кнопка становится активной, как только вы что-то переключили.',
    optional: true,
    side: 'bottom',
    align: 'end',
  },
  {
    // Замыкает сегмент кабинета непропускаемым шагом: все шаги карточки
    // необязательны, и без этого при пустом списке заявок хвост сегмента исчезал
    // целиком. Заодно проговаривает переход - тур уводит на другую страницу.
    id: 'cabinet-outro',
    route: '/notification-settings',
    element: null,
    title: 'Дальше - ваши данные',
    description: 'С заявками разобрались. Теперь посмотрим, откуда берутся машины и сотрудники, которых вы вносите в заявку: у них свои разделы.',
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
    // Центр-модал, а не подсветка того же списка бланков, что и у следующего
    // шага: два шага подряд на одном якоре выглядят так, будто «Далее» не
    // сработало - поповер не сдвигается ни на пиксель.
    element: null,
    title: 'Оформление заявки',
    description: 'Перешли в «Оформление и подачу заявки» - главное действие в системе. Дальше разберём экран по частям: слева выбор бланков, справа их форма, сверху общая шапка заявки.',
  },
  {
    id: 'createapp-blank-added',
    route: '/new-application',
    element: '[data-testid="ob-app-selector"]',
    title: 'Бланк добавлен',
    description: 'Для примера мы добавили сюда бланк для транспорта - обычно вы делаете это кнопкой «Добавить». Бланк - это раздел заявки: в него вносят одну или несколько машин. Названия бланков задаёт Бюро, а бланков в заявке может быть сразу несколько - например, машины и сотрудники.',
    demoAttachment: 'cars',
    side: 'right',
    align: 'start',
  },
  {
    id: 'createapp-orginfo',
    route: '/new-application',
    element: '[data-testid="ob-app-orginfo"]',
    title: 'Шапка заявки',
    description: 'Организация, компания (отдел), ответственное лицо и телефон. Эти данные общие для всей заявки, а не для отдельного бланка, и обычно подставляются из вашего профиля автоматически.',
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
    waitFor: '[data-testid="ob-app-formdata"][data-attachment-type="cars"]',
    title: 'Форма транспорта',
    description: 'Форма выбранного бланка: формат номера, номер и марка. Кнопка «Добавить» переносит машину в список справа - в один бланк их можно внести несколько.',
    demoAttachment: 'cars',
    side: 'left',
    align: 'start',
  },
  {
    id: 'createapp-car-existing',
    route: '/new-application',
    element: '[data-testid="ob-form-existing"]',
    title: 'Машина уже заводилась',
    description:
      'Если транспорт уже есть в вашем разделе «Автомобили», не заполняйте всё заново: «Добавить существующую(-ие)» откроет список ваших машин - отметьте нужные и они попадут в заявку сразу с номером и маркой.',
    demoAttachment: 'cars',
    optional: true,
    side: 'bottom',
    align: 'start',
  },
  {
    id: 'createapp-car-places',
    route: '/new-application',
    element: '[data-testid="ob-form-places"]',
    title: 'Места разгрузки',
    description:
      'Отметьте, где машина будет разгружаться: без этого охрана на посту не поймёт, куда её пропускать. Список - места вашей организации; у каждого свой график, и если время заявки в него не попадает, форма предупредит.',
    demoAttachment: 'cars',
    optional: true,
    side: 'top',
    align: 'center',
  },
  {
    id: 'createapp-blank-switch',
    route: '/new-application',
    // Подсвечиваем именно выбранную строку: на весь список смотреть бесполезно -
    // непонятно, на какой бланк переключились.
    element: '[data-testid="ob-blank-selected"]',
    // Ждём, пока выделение переедет на бланк людей: смена пересоздаёт форму, и
    // без этого шаг подсвечивал прежний выбор.
    waitFor: '[data-testid="ob-blank-list"][data-selected-type="people"] [data-testid="ob-blank-selected"]',
    // Заголовок без подстановки: он же попадает в список шагов, где имени бланка
    // на экране ещё нет и подставлять нечего.
    title: 'Сменили бланк',
    description: 'Теперь в заявке выбран бланк «{blank}» - форма справа сменилась под него. Прежний бланк мы из заявки убрали; если оставить оба, заполненное в каждом хранится отдельно и уходит одной заявкой.',
    // Имена бланков задаёт Бюро («Заявка на работы», «Ввоз»), поэтому берём их с
    // экрана: без этого шаг говорил «выбран другой бланк» и не называл какой.
    dynamic: { blank: '[data-testid="ob-blank-selected"] .attachment-name' },
    demoAttachment: 'people',
    optional: true,
    side: 'right',
    align: 'start',
  },
  {
    id: 'createapp-people-form',
    route: '/new-application',
    element: '[data-testid="ob-app-formdata"]',
    waitFor: '[data-testid="ob-app-formdata"][data-attachment-type="people"]',
    title: 'Форма сотрудников',
    description: 'Бланк для людей: гражданство, ФИО, должность и паспортные данные (иностранным гражданам добавятся патент и разрешение на работу). Кнопка «Добавить» переносит сотрудника в список справа.',
    // Форма высокая - прижимаем её к низу экрана, иначе поповер сверху
    // накрывает ровно то, о чём рассказывает.
    scrollTo: 'end',
    demoAttachment: 'people',
    // Форма занимает почти весь экран, и по бокам driver.js места не находит -
    // поповер выдавливало в угол (замер 21,0). Сверху место есть всегда.
    side: 'top',
    align: 'center',
  },
  {
    id: 'createapp-people-existing',
    route: '/new-application',
    element: '[data-testid="ob-form-existing"]',
    title: 'Сотрудник уже заводился',
    description:
      'Тех, кто уже есть в вашем разделе «Сотрудники», добавляйте кнопкой «Добавить существующего(-их)»: отметьте нужных в списке - паспортные данные подставятся сами.',
    demoAttachment: 'people',
    optional: true,
    side: 'bottom',
    align: 'start',
  },
  {
    id: 'createapp-people-places',
    route: '/new-application',
    element: '[data-testid="ob-form-places"]',
    title: 'Места прохода',
    description:
      'Отметьте посты, через которые человек будет проходить: заявка попадёт в таблицы именно этих постов, и охрана увидит его на своём экране. Не отметите - пропускать будет некому.',
    demoAttachment: 'people',
    optional: true,
    side: 'top',
    align: 'center',
    scrollTo: 'end',
  },
  {
    id: 'createapp-consent',
    route: '/new-application',
    element: '[data-testid="ob-app-consent"]',
    waitFor: '[data-testid="ob-app-formdata"][data-attachment-type="people"]',
    title: 'Согласие на обработку данных',
    description:
      'Без этой отметки заявка не отправится: вы подтверждаете согласие на обработку, хранение и передачу персональных данных, изложенных в заявке. Слово «согласие» - ссылка на полный текст, он открывается отдельной вкладкой.',
    // Согласие и кнопка отправки живут внутри формы вложения - без бланка форма не
    // отрисована, поэтому демо-вложение держим до конца сегмента.
    demoAttachment: 'people',
    side: 'left',
  },
  {
    id: 'createapp-submit',
    route: '/new-application',
    element: '[data-testid="create-app-button-submit"]',
    title: 'Отправка заявки',
    description:
      'Кнопка «Отправить заявку» отправляет все бланки заявки разом. Пока чего-то не хватает, она неактивна: наведите на неё, и подсказка перечислит, чего именно. После отправки заявка попадёт в «Список заявок» вашего Личного кабинета, там же виден её статус.',
    demoAttachment: 'people',
    side: 'left',
  },
  {
    id: 'finish',
    route: '/news',
    element: '[data-testid="ob-start-button"]',
    title: 'Готово, вы освоились!',
    description: 'Возвращаемся на «Обзор». Это всё основное. Запустить обучение заново можно в любой момент - кнопкой «Обучение» вот здесь. Удачной работы!',
    celebrate: true,
    cta: 'Подать первую заявку',
  },
];
