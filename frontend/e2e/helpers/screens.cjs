/**
 * Реестр экранов проекта для инструментов раскладки.
 *
 * Заведён для эпика адаптивности (#2473): «прошерстить каждую вкладку» невозможно
 * повторяемо, пока список экранов живёт в голове. Здесь он один на всех, и новый
 * экран получает проверки добавлением строки, а не правкой инструмента.
 *
 * `card` - селектор строки/карточки, внутри которой ищем взаимные наложения. В
 * компонентах управления это общий маркер `.rt-row` (responsive-tables.css), в
 * пользовательских списках - собственный класс строки.
 *
 * `open` - клики после загрузки, раскрывающие второе состояние экрана (деталь
 * мастер-детейла, модалка). Без них аудит видит только пустой список и молчит про
 * окна, а половина претензий пользователя как раз про окна.
 */

// Имя таблицы для /table/:tableName. На стенде существует `auto_blank`; для другого
// окружения подменяется переменной окружения, чтобы аудит не падал на 404.
const TABLE_NAME = process.env.AUDIT_TABLE || 'auto_blank';

/** @typedef {{slug:string,name:string,path:string,area:'user'|'admin',card?:string,open?:string[]}} Screen */

/** @type {Screen[]} */
const USER_SCREENS = [
  { slug: 'news', name: 'Обзор и новости', path: '/news', area: 'user', card: '.news-item' },
  { slug: 'cabinet', name: 'Личный кабинет', path: '/personal-cabinet', area: 'user', card: '.application-item' },
  { slug: 'center', name: 'Центр заявок', path: '/center', area: 'user', card: '.application-item' },
  { slug: 'create', name: 'Подача заявки', path: '/new-application', area: 'user' },
  { slug: 'accessible', name: 'Доступные мне', path: '/accessible-attachments', area: 'user', card: '[data-testid="aa-card"]' },
  { slug: 'table', name: 'Таблица', path: `/table/${TABLE_NAME}`, area: 'user', card: '.rt-row' },
  { slug: 'cars', name: 'Мои автомобили', path: '/carsview', area: 'user', card: '.car-row' },
  { slug: 'employees', name: 'Мои сотрудники', path: '/employeesview', area: 'user', card: '.employee-row' },
  { slug: 'analytics', name: 'Аналитика', path: '/analytics', area: 'user', card: '.metric' },
  { slug: 'notif-settings', name: 'Настройки уведомлений', path: '/notification-settings', area: 'user' },
];

/** @type {Screen[]} */
const ADMIN_SCREENS = [
  { slug: 'users', name: 'Пользователи', path: '/admin/users', area: 'admin', card: '.rt-row', open: ['[data-testid="users-row"]'] },
  { slug: 'roles', name: 'Роли', path: '/admin/roles', area: 'admin', card: '.rt-row' },
  { slug: 'permission-groups', name: 'Группы прав', path: '/admin/permission-groups', area: 'admin', card: '.rt-row' },
  { slug: 'organizations', name: 'Организации', path: '/admin/organizations', area: 'admin', card: '.rt-row' },
  { slug: 'companies', name: 'Компании', path: '/admin/companies', area: 'admin', card: '.rt-row' },
  { slug: 'unload-places', name: 'Места разгрузки', path: '/admin/unload-places', area: 'admin', card: '.rt-row' },
  { slug: 'number-formats', name: 'Форматы номеров', path: '/admin/number-formats', area: 'admin', card: '.rt-row' },
  { slug: 'citizenship', name: 'Гражданство', path: '/admin/citizenship', area: 'admin', card: '.rt-row' },
  { slug: 'marks', name: 'Марки', path: '/admin/marks', area: 'admin', card: '.rt-row' },
  { slug: 'attachment-types', name: 'Типы вложений', path: '/admin/attachment-types', area: 'admin', card: '.rt-row' },
  { slug: 'user-types', name: 'Типы пользователей', path: '/admin/user-types', area: 'admin', card: '.rt-row' },
  { slug: 'approvers', name: 'Принимающие', path: '/admin/approvers', area: 'admin', card: '.rt-row' },
  { slug: 'documents', name: 'Документы', path: '/admin/documents', area: 'admin', card: '.rt-row' },
  { slug: 'admin-news', name: 'Новости и объявления', path: '/admin/news', area: 'admin', card: '.manage-item' },
  { slug: 'guide', name: 'Руководство', path: '/admin/guide', area: 'admin', card: '.rt-row' },
  { slug: 'file-archive', name: 'Файловый архив', path: '/admin/file-archive', area: 'admin', card: '.rt-row' },
  { slug: 'blacklist', name: 'Чёрный список', path: '/admin/blacklist', area: 'admin', card: '.rt-row' },
  { slug: 'pd-subject', name: 'Субъект персональных данных', path: '/admin/pd-subject', area: 'admin' },
  { slug: 'pd-audit', name: 'Аудит персональных данных', path: '/admin/pd-audit', area: 'admin', card: '.rt-row' },
  { slug: 'access-denials', name: 'Отказы доступа', path: '/admin/access-denials', area: 'admin', card: '.rt-row' },
  { slug: 'data-processing', name: 'Обработка персональных данных', path: '/admin/data-processing', area: 'admin' },
  { slug: 'admin-settings', name: 'Настройки системы', path: '/admin/settings', area: 'admin' },
  { slug: 'feedback', name: 'Обратная связь', path: '/admin/feedback', area: 'admin', card: '.rt-row' },
  { slug: 'requests', name: 'Запросы', path: '/admin/requests', area: 'admin', card: '.rt-row' },
  { slug: 'system-control', name: 'Системный контроль', path: '/admin/system-control', area: 'admin' },
  { slug: 'table-constructor', name: 'Таблицы системы', path: '/table-constructor', area: 'admin', card: '.rt-row' },
];

const SCREENS = [...USER_SCREENS, ...ADMIN_SCREENS];

/**
 * Отбор по переменной окружения `AUDIT_SCREENS`: список slug или `admin`/`user`
 * через запятую. Нужен срезам эпика - каждый гоняет свои экраны за секунды вместо
 * полного обхода на четверть часа.
 */
function selectScreens(filter = process.env.AUDIT_SCREENS) {
  if (!filter) return SCREENS;
  const wanted = filter.split(',').map((s) => s.trim()).filter(Boolean);
  return SCREENS.filter((s) => wanted.includes(s.slug) || wanted.includes(s.area));
}

module.exports = { SCREENS, USER_SCREENS, ADMIN_SCREENS, selectScreens, TABLE_NAME };
