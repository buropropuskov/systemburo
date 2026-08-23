/**
 * Шаги личного кабинета про СВОЮ заявку: строка списка, карточка и действия над
 * ней. Вынесены из `onboardingSteps.js` отдельным сегментом, потому что живут по
 * общему условию - `needs: 'hasOwnApplication'`.
 *
 * Условие решается ДО старта тура (см. gatingData.js): у человека без заявок этих
 * целей на экране не появится, и раньше тур выяснял это на ходу - платил
 * ожиданием за каждый шаг и на глазах уменьшал знаменатель «Шаг N из M».
 * Исключение - «Вот ваша заявка»: у него есть демо-скриншот, поэтому шаг остаётся
 * и новичку, просто показывается картинкой.
 *
 * @type {Array<object>} формат шага - см. JSDoc в onboardingSteps.js
 */
export const cabinetApplicationSteps = [
  {
    id: 'cabinet-download',
    needs: 'hasOwnApplication',
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
    needs: 'hasOwnApplication',
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
    // Без своей заявки открывать нечего: раскрытие не сработает, и шаг девять
    // секунд ждал бы цель, которой не будет. С этим полем он сразу показывается
    // скриншотом-примером - человек всё равно узнаёт, как выглядит карточка.
    demoUnless: 'hasOwnApplication',
    optional: true,
    side: 'bottom',
    reveal: { open: 'first-application' },
  },
  {
    id: 'detail-status',
    needs: 'hasOwnApplication',
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
    needs: 'hasOwnApplication',
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
    needs: 'hasOwnApplication',
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
    needs: 'hasOwnApplication',
    route: '/personal-cabinet',
    element: '[data-testid="ob-detail-header"]',
    title: 'Что можно сделать с заявкой',
    description:
      'В шапке карточки собраны действия: дополнить, продублировать, скачать бланки, отозвать. Какие из них доступны, зависит от того, на какой стадии заявка. Разберём их по очереди.',
    optional: true,
    side: 'bottom',
    align: 'start',
    reveal: { open: 'first-application' },
  },
  {
    id: 'detail-supplement',
    needs: 'hasOwnApplication',
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
    needs: 'hasOwnApplication',
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
    needs: 'hasOwnApplication',
    route: '/personal-cabinet',
    element: '[data-testid="ob-detail-revoke"]',
    title: 'Отозвать свою заявку',
    description:
      'Передумали или ошиблись - кнопка «Отозвать» снимает заявку с рассмотрения, система переспросит перед этим. Отозвать можно только свою заявку и только пока она не закрыта.',
    optional: true,
    side: 'bottom',
    reveal: { open: 'first-application' },
  },
];
