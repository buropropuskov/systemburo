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
 * - `optional`   элемента может не быть в DOM - шаг ждёт цель коротко и
 *                пропускается, если она не появилась (тогда он не участвует и в
 *                счётчике «Шаг N из M»). Вместе с `demo` пропуска нет: у нового
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
 * @type {Array<{ id: string, route: string, element: string|null, title: string, description: string, demo?: string, requires?: string, optional?: boolean, optionalSegment?: boolean, expandRail?: boolean, celebrate?: boolean, cta?: string, ctaRoute?: string, demoAttachment?: string, side?: string, align?: string, reveal?: { mobile?: 'nav', open?: 'admin-column'|'search-panel'|'first-application' } }>}
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
    element: '[data-testid="ob-news-head"]',
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
    id: 'work-modes',
    route: '/news',
    element: '[data-testid="ob-work-modes"]',
    title: 'Режимы работы',
    description:
      'Кнопка «Режимы работы» открывает расписание: время работы Бюро, мест разгрузки и мест прохода. По каждой позиции видно, открыта она сейчас, закрыта или неактивна, и какой график действует сегодня. Сверьтесь с ним до подачи заявки: если время пребывания не попадает в график места, форма заявки об этом предупредит.',
  },
  {
    id: 'header-broadcast',
    side: 'bottom',
    route: '/news',
    element: '[data-testid="ob-header-broadcast"]',
    title: 'Объявление в шапке',
    description:
      'Пока объявление действует, оно висит в шапке подписью «Объявление» или «Важное объявление» и видно из любого раздела, а не только на «Обзоре». Нажмите, чтобы прочитать целиком.',
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
    description: 'Колокольчик показывает количество непрочитанных уведомлений: смена статуса заявки, ответы согласующих и другие события. Нажмите, чтобы открыть список.',
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
    description:
      'Кнопка «Настроить» открывает «Настройку уведомлений»: события собраны по разделам, у каждого свой переключатель. Лишнее можно выключить, чтобы лента не забивалась, - кроме обязательных, их отключить нельзя. Там же можно включить push-уведомления на телефон или компьютер: системным сообщением, даже когда сайт закрыт. После правки нажмите «Сохранить».',
    optional: true,
  },
  {
    id: 'cabinet-applications',
    route: '/personal-cabinet',
    // Шапка списка, а не всё полотно: блок занимает 58% экрана, и подсветка
    // целиком читается как вспышка вместо указания на элемент.
    element: '[data-testid="ob-applications-head"]',
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
    // Показываем строку до открытия карточки: иначе окно появляется само собой
    // и человек не понимает, откуда оно взялось.
    id: 'cabinet-application-row',
    route: '/personal-cabinet',
    element: '[data-testid="ob-application-row"]',
    title: 'Откройте заявку',
    description: 'Клик по строке открывает карточку заявки - всё о ней в одном окне. Сейчас откроем первую из списка.',
    optional: true,
    side: 'bottom',
  },
  {
    id: 'detail-opened',
    route: '/personal-cabinet',
    element: '[data-testid="ob-detail-header"]',
    title: 'Вот ваша заявка',
    description: 'Это карточка заявки. Сверху - номер, дата подачи и действия над заявкой; ниже - её состав и согласование. Закрывается крестиком справа или клавишей Esc.',
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
    id: 'detail-download',
    route: '/personal-cabinet',
    element: '[data-testid="app-detail-button-download"]',
    title: 'Скачать бланки',
    description:
      'Кнопка «Скачать» открывает окно «Скачивание бланков»: заполненные бланки заявки можно взять по одному или все сразу. Кнопка появляется только у заявок, для вложений которых бланки настроены.',
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
    // Замыкает сегмент кабинета непропускаемым шагом: все шаги карточки
    // необязательны, и без этого при пустом списке заявок хвост сегмента исчезал
    // целиком. Заодно проговаривает переход - тур уводит на другую страницу.
    id: 'cabinet-outro',
    route: '/personal-cabinet',
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
    element: '[data-testid="ob-cars-table-head"]',
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
    element: '[data-testid="ob-employees-table-head"]',
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
    description: 'Для примера мы добавили сюда бланк «Автомобили» - обычно вы делаете это кнопкой «Добавить». Бланк - это раздел заявки: в него вносят один или несколько автомобилей. Бланков в заявке может быть несколько, например машины и сотрудники сразу.',
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
    title: 'Форма «Автомобили»',
    description: 'Сама форма транспорта: формат номера, номер и марка, места разгрузки. Кнопка «Добавить» переносит авто в «Список транспортных средств» справа - в одну заявку можно внести несколько машин.',
    demoAttachment: 'cars',
    side: 'left',
    align: 'start',
  },
  {
    id: 'createapp-blank-switch',
    route: '/new-application',
    element: '[data-testid="ob-blank-list"]',
    title: 'Переключились на другой бланк',
    description: 'Выбрали слева бланк «Сотрудники» - подсвечен выбранный. Форма справа сейчас сменится под него, а заполненное по автомобилям сохранится в своём бланке.',
    demoAttachment: 'people',
    optional: true,
    side: 'right',
    align: 'start',
  },
  {
    id: 'createapp-people-form',
    route: '/new-application',
    element: '[data-testid="ob-app-formdata"]',
    title: 'Форма «Сотрудники»',
    description: 'Слева выбрали бланк «Сотрудники» - и форма сменилась: теперь это гражданство, ФИО, должность и паспортные данные. Для иностранных граждан добавятся поля патента и разрешения на работу. Кнопка «Добавить» переносит сотрудника в «Список сотрудников» справа. Заполненное по автомобилям при этом никуда не делось.',
    demoAttachment: 'people',
    // Форма занимает почти весь экран, и по бокам driver.js места не находит -
    // поповер выдавливало в угол (замер 21,0). Сверху место есть всегда.
    side: 'top',
    align: 'center',
  },
  {
    id: 'createapp-consent',
    route: '/new-application',
    element: '[data-testid="ob-app-consent"]',
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
