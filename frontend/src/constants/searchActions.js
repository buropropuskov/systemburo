/**
 * Быстрые действия для сквозного поиска.
 *
 * Человек ищет не только записи и разделы, но и то, что хочет сделать: набирает
 * «подать», «отправить» или «заявка» и ждёт, что система предложит подать заявку, а не
 * заставит вспоминать, где эта кнопка. Раздел и действие — разные вещи: «Центр заявок»
 * это место, «Подать заявку» это намерение, и по слову «подать» нужно второе.
 *
 * `keywords` — слова, которыми действие называют на практике, включая обиходные и
 * ошибочные. Совпадение по ним равнозначно совпадению по названию: их и добавляют ради
 * тех формулировок, до которых человек доходит раньше, чем до официальной.
 *
 * `permission` — право, без которого действие не показывается. Предлагать то, что
 * упрётся в отказ, хуже, чем не предлагать вовсе.
 */

/**
 * @typedef {object} SearchAction
 * @property {string} key устойчивый идентификатор
 * @property {string} label название, как показываем
 * @property {string} hint пояснение под названием
 * @property {string[]} keywords слова, по которым действие находится
 * @property {string} icon имя иконки из набора навигации
 * @property {string} [permission] право; без него действие скрыто
 * @property {{path: string, query?: object}} to куда ведёт
 */

/** @type {SearchAction[]} */
export const SEARCH_ACTIONS = [
  {
    key: 'new-application',
    label: 'Подать заявку',
    hint: 'Оформление и отправка новой заявки',
    keywords: ['подать', 'отправить', 'заявка', 'заявку', 'оформить', 'новая заявка', 'создать заявку', 'пропуск'],
    icon: 'new-application',
    permission: 'page.new_application',
    to: { path: '/new-application' },
  },
  {
    key: 'add-employee',
    label: 'Добавить сотрудника',
    hint: 'Реестр сотрудников',
    keywords: ['добавить сотрудника', 'новый сотрудник', 'человек', 'работник', 'персонал', 'сотрудник'],
    icon: 'employees',
    permission: 'entity.employees.write',
    to: { path: '/employeesview' },
  },
  {
    key: 'add-car',
    label: 'Добавить автомобиль',
    hint: 'Реестр автомобилей',
    keywords: ['добавить машину', 'новая машина', 'автомобиль', 'транспорт', 'машина', 'авто'],
    icon: 'cars',
    permission: 'entity.cars.write',
    to: { path: '/carsview' },
  },
  {
    key: 'report-problem',
    label: 'Сообщить о проблеме',
    hint: 'Обращение в поддержку',
    keywords: ['проблема', 'ошибка', 'не работает', 'поддержка', 'баг', 'сообщить', 'обращение', 'жалоба'],
    icon: 'feedback',
    permission: 'header.report_problem',
    to: { path: '/personal-cabinet', query: { feedback: '1' } },
  },
  {
    key: 'my-applications',
    label: 'Мои заявки',
    hint: 'Личный кабинет: поданные заявки и их состояние',
    keywords: ['мои заявки', 'мои', 'статус заявки', 'кабинет', 'история заявок'],
    icon: 'cabinet',
    permission: 'page.personal_cabinet',
    to: { path: '/personal-cabinet' },
  },
  {
    key: 'documents',
    label: 'Открыть документы',
    hint: 'Бланки, правила и инструкции',
    keywords: ['документ', 'бланк', 'правила', 'инструкция', 'шаблон', 'скачать'],
    icon: 'documents',
    permission: 'page.news',
    to: { path: '/news' },
  },
];
