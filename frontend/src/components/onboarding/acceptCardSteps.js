/**
 * Шаги тура принимающего по карточке заявки: от шапки до бланков.
 *
 * Вынесены из `acceptOnboardingSteps` - тот перерос порог `lint:size`, когда
 * карточку разобрали по частям (#2610). Делятся по смыслу: сперва общее о заявке
 * (сообщение, кто подал, статус, согласование, история, обсуждение), затем состав
 * вложений, затем решение и связи с людьми.
 *
 * Все шаги `optional` и открывают карточку через `reveal.open: 'first-application'`:
 * на пустом Центре сегмент пропускается целиком. Шаги, чья цель приезжает вместе с
 * данными, объявляют `dataReady` - без него неприменимый шаг держал тур по 2,5 с
 * (#2590).
 *
 * @type {Array<object>}
 */
export const acceptCardSteps = [
  {
    id: 'acc-detail-card',
    route: '/center',
    element: '[data-testid="ob-detail-card"]',
    optional: true,
    title: 'Карточка заявки',
    description:
      'Это вся заявка на одном экране. Сверху шапка с номером и действиями, слева вложения с составом, справа основная информация, согласование и дополнения. Разберём по порядку.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-header',
    route: '/center',
    element: '[data-testid="ob-detail-header"]',
    optional: true,
    side: 'bottom',
    align: 'start',
    title: 'Шапка карточки',
    description:
      'Номер заявки и дата подачи - по ним заявку ищут и на них ссылаются в переписке. Здесь же кнопки действий. Закрыть карточку можно крестиком справа или клавишей Esc.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-message',
    route: '/center',
    element: '[data-testid="ob-detail-message"]',
    optional: true,
    scrollTo: 'center',
    title: 'Сообщение к заявке',
    description:
      'Заявитель пишет здесь, зачем едут: что везут, к кому и с какой целью. Это первое, что стоит прочитать - по нему понятно, ту ли заявку вы открыли и хватает ли в ней данных.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-basic',
    route: '/center',
    element: '[data-testid="ob-detail-basic"]',
    optional: true,
    scrollTo: 'center',
    title: 'Основная информация',
    description:
      'Организация или отдел, компания и отправитель. По ним видно, от кого заявка и кому звонить, если что-то не сходится. Отметка «Важный» у отправителя значит, что его заявки просили не задерживать.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-status-section',
    route: '/center',
    // Без подсветки: блок «Статус заявки» появляется в карточке только после
    // решения (принята, отказана, завершена, отозвана), а разбираем мы заявку,
    // которая решения ещё ждёт - подсвечивать нечего (#2622).
    element: null,
    optional: true,
    title: 'Статус заявки',
    description:
      'Судьба заявки: «Непрочитано», «В обработке», «В работе», «Завершено», «Отказано», «Отозвана». Это не то же, что согласование: согласование - про решение людей, статус - про то, где заявка в работе бюро. Как только заявку примут или откажут, в карточке появится блок «Статус заявки» с именем того, кто решил, и временем.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-status',
    route: '/center',
    element: '[data-testid="ob-detail-status"]',
    optional: true,
    scrollTo: 'center',
    title: 'Согласование и согласующие',
    description:
      'Здесь видно, кто согласовывал заявку и чем это кончилось: имя, должность, решение и время. Голосуют согласующие - отдельная роль; вам важен итог, потому что принять в работу можно только согласованную заявку.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-history',
    route: '/center',
    element: '[data-testid="ob-detail-history"]',
    optional: true,
    scrollTo: 'center',
    title: 'История заявки',
    description:
      'Журнал изменений: кто и когда менял состав, сроки, статус и решения. Сюда смотрят, когда заявка выглядит не так, как её помнят, - видно и правку срока, и отзыв решения.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-questions',
    route: '/center',
    element: '[data-testid="application-questions"]',
    optional: true,
    scrollTo: 'center',
    title: 'Обсуждение заявки',
    description:
      'Если чего-то не хватает - спросите прямо в заявке. Заявитель получит уведомление и ответит здесь же, переписка останется в карточке. Это короче, чем отказывать и ждать повторную подачу.',
    reveal: { open: 'first-application' },
  },

  // ── Состав заявки ──
  {
    id: 'acc-detail-attachments-list',
    route: '/center',
    element: '[data-testid="ob-detail-attachments-list"]',
    optional: true,
    waitsData: true,
    dataReady: '[data-testid="ob-detail-card"]',
    title: 'Вложения заявки',
    description:
      'Вложение - это бланк: машины, сотрудники или товар. В одной заявке их бывает несколько, у каждого свой срок и своё содержимое. Переключаетесь между ними прямо здесь.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-attachments',
    route: '/center',
    dataReady: '[data-testid="ob-detail-card"]',
    element: '[data-testid="attachment-elements"]',
    optional: true,
    waitsData: true,
    title: 'Состав вложения',
    description:
      'Строки вложения: у машин номер и марка, у людей фамилия и должность. Это то, что реально поедет и пройдёт, - состав стоит сверить с сообщением заявителя.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-search',
    route: '/center',
    dataReady: '[data-testid="attachment-elements"]',
    element: '[data-testid="attachment-elements-search"]',
    optional: true,
    waitsData: true,
    title: 'Поиск по составу',
    description:
      'Когда строк десятки, поиск оставляет нужные - по номеру машины или фамилии. Счётчик рядом показывает, сколько строк во вложении всего.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-places',
    route: '/center',
    dataReady: '[data-testid="attachment-elements"]',
    element: '[data-testid="attachment-chip"]',
    optional: true,
    waitsData: true,
    title: 'Места разгрузки',
    description:
      'Место разгрузки - куда машина едет внутри территории: дебаркадер, ворота, площадка. Их может быть несколько на одну машину, и охрана на месте видит именно то, что здесь назначено.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-tables',
    route: '/center',
    dataReady: '[data-testid="attachment-elements"]',
    element: '[data-testid="attachment-chip-table"]',
    optional: true,
    waitsData: true,
    title: 'Проезд',
    description:
      'Проезд - через какой пост машина въезжает на территорию: КПП №4, ПОСТ №72. Это другой признак, чем разгрузка: разгрузка про то, куда ехать внутри, проезд - про то, где пускают. От него зависит, в чьей таблице появится запись: охранник поста видит только свой.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-assign',
    route: '/center',
    dataReady: '[data-testid="attachment-elements"]',
    element: '[data-testid="attachment-assign-open"]',
    optional: true,
    waitsData: true,
    title: 'Назначить по одной строке',
    description:
      'Кнопка с плюсом в строке открывает выбор: отмечаете нужные места и посты, и запись попадает в их таблицы. Заявитель мог оставить выбор пустым или указать не то - доназначение ваша работа, и сделать его можно и после приёма в работу.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-assign-all',
    route: '/center',
    dataReady: '[data-testid="attachment-elements"]',
    element: '[data-testid="attachment-assign-all-places"]',
    optional: true,
    waitsData: true,
    title: 'Назначить всем сразу',
    description:
      '«Назначить всем» ставит выбор сразу всем строкам вложения - выручает, когда в заявке два десятка машин на один дебаркадер. Места и проезд раздаются отдельными кнопками, так что можно назначить общий пост, а разгрузку оставить построчной. На телефоне это одна кнопка «Назначить всем…», которая открывает выбор листом.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-people',
    route: '/center',
    // Без подсветки: список вложений уже подсвечен своим шагом, а второй рамкой на
    // том же узле замок справедливо ругается - шаги сегмента смотрят в разные точки.
    element: null,
    optional: true,
    title: 'Второе вложение: люди',
    description:
      'В этой заявке есть и бланк на людей. У сотрудников вместо марки - должность и гражданство, а вместо проезда - места прохода: через какой пост человек проходит пешком. Остальное так же: строки, поиск, назначение по одной и всем сразу.',
    reveal: { open: 'first-application' },
  },

  // ── Решение по заявке ──
  {
    id: 'acc-detail-blacklist-override',
    route: '/center',
    dataReady: '[data-testid="ob-detail-card"]',
    element: '[data-testid="blacklist-override-btn"]',
    optional: true,
    waitsData: true,
    scrollTo: 'center',
    title: 'Подтвердить пропуск похожего на ЧС',
    description:
      'У строки, совпавшей с чёрным списком, стоит кнопка подтверждения: вы смотрите, тот это человек или однофамилец, и разрешаете пропуск по своей ответственности. Пока хоть одно совпадение не разобрано, согласование заявке не даётся, и принять её нельзя. Подтверждение попадает в историю с вашим именем.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-note',
    route: '/center',
    dataReady: '[data-testid="ob-detail-card"]',
    element: '[data-testid="bureau-note"]',
    optional: true,
    waitsData: true,
    scrollTo: 'center',
    title: 'Заметка бюро',
    description:
      'Строка для своих: заметку видят только сотрудники бюро. Ни заявитель, ни согласующие, ни охрана на посту её не читают - в бланк и в выгрузки она тоже не попадает. Держат в ней то, что пригодится следующему: «звонить перед въездом», «пропуск только с сопровождением».',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-take',
    route: '/center',
    // Якорь - весь ряд действий, а не кнопка: пока заявка ждёт согласующих,
    // кнопки нет вовсе (вместо неё надпись ожидания), и шаг про главное действие
    // роли молча пропал бы именно на таких заявках.
    element: '[data-testid="ob-detail-actions"]',
    optional: true,
    side: 'bottom',
    align: 'end',
    title: 'Согласовать и принять',
    description:
      'Главная кнопка роли берёт заявку в работу: вы становитесь ответственным, и заявитель видит ваше имя. Подпись зависит от заявки - когда согласующих у неё нет, кнопка называется «Согласовать и принять» и закрывает оба шага разом; когда они есть, на ней написано «Принять», и она оживает только после их решения.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-reject',
    route: '/center',
    element: '[data-testid="app-detail-button-reject"]',
    optional: true,
    side: 'bottom',
    align: 'end',
    title: 'Отказать по заявке',
    description:
      'Соседняя кнопка закрывает заявку отказом. Комментарий обязателен и попадёт в карточку - заявитель его увидит и поймёт, что исправить. Если дело в мелочи, быстрее спросить в обсуждении, чем отказывать.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-supplement',
    route: '/center',
    dataReady: '[data-testid="ob-detail-card"]',
    // Якорь - блок раундов, а не кнопка решения: она появляется в единственном
    // состоянии раунда (согласован, ждёт вашего слова), а блок стоит у любой
    // заявки с дополнением - на нём и видно, из-за чего решение принимается.
    element: '[data-testid="supplement-panel"]',
    optional: true,
    waitsData: true,
    scrollTo: 'center',
    title: 'Дополнение к поданной заявке',
    description:
      'Заявитель может дописать в поданную заявку машины или людей. Добавка идёт отдельным кругом: в списке появляется тег «Дополнение», в карточке - блок с его составом, а вам кнопки «Принять дополнение» и «Отказать». Уже выданные пропуска при этом действуют.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-org-moderation',
    route: '/center',
    dataReady: '[data-testid="ob-detail-card"]',
    element: '[data-testid="ob-org-moderation"]',
    optional: true,
    waitsData: true,
    scrollTo: 'center',
    requires: 'application.organization.moderate',
    title: 'Организация «на проверке»',
    description:
      'Организацию, которой нет в справочнике, заявитель вписывает сам - она помечается «На проверке». Разбираете вы: принять как есть, исправить наименование или привязать к существующей записи. Похожую запись система предложит сама.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-participants',
    route: '/center',
    element: '[data-testid="app-detail-button-participants"]',
    optional: true,
    side: 'bottom',
    align: 'end',
    title: 'Кто ещё видит заявку',
    description:
      'Кнопка получателей показывает всех, кому заявка доступна: заявитель, согласующие, принимающие и те, кому её переслали. Оттуда же открывается карточка участника - с должностью и контактами, чтобы не искать человека по справочникам.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-forward',
    route: '/center',
    element: '[data-testid="app-detail-button-forward"]',
    optional: true,
    side: 'bottom',
    align: 'end',
    title: 'Переслать коллеге',
    description:
      'Пересылка открывает заявку другому человеку: на просмотр, на согласование или ответственным вместо себя. Так поступают, когда заявка не ваша по участку или вы уходите и передаёте её смене.',
    reveal: { open: 'first-application' },
  },
  {
    id: 'acc-detail-files',
    route: '/center',
    dataReady: '[data-testid="ob-detail-card"]',
    element: '[data-testid="application-files"]',
    optional: true,
    waitsData: true,
    scrollTo: 'center',
    title: 'Файлы к заявке',
    description:
      'Заявитель прикладывает к заявке сканы и письма - договор, гарантийное, список оборудования. Файлы видны в карточке и открываются прямо отсюда; после подачи состав файлов не меняется. В примерной заявке они показаны для вида, открыть их не получится.',
    reveal: { open: 'first-application' },
  },
];
