/**
 * Примерная заявка для тура принимающего (и согласующего).
 *
 * Обучение обязано быть одинаковым для всех, а Центр у каждого свой: у одного
 * первой лежит заявка с товарами, у другого - отозванная, у третьего вовсе пусто.
 * Шаги про места разгрузки, проезд, дополнение и разбор наименования при этом
 * просто выпадали: показывать их было не на чем (#2580).
 *
 * Поэтому на время тура Центр показывает одну примерную заявку, собранную так,
 * чтобы в ней нашлось всё, о чём рассказывают шаги: две машины с местами и
 * проездом, совпадение с чёрным списком, дополнение на решении, организация «на
 * проверке» и настроенный бланк.
 *
 * Данные выдуманы здесь и никуда не уходят: подмена работает только на чтение и
 * только пока идёт тур (см. readInterceptor.js). Организация и отправитель названы
 * узнаваемо-условно - «Пример» в наименовании не даёт принять карточку за живую
 * заявку, по которой надо принимать решение.
 */

/** Идентификаторы заведомо вне диапазона живых записей - чтобы не спутать в логах. */
export const DEMO_CENTER_APPLICATION_ID = 999000002;
export const DEMO_CENTER_ATTACHMENT_ID = 999000102;
/** Второе вложение примера - люди: у них вместо проезда места прохода. */
export const DEMO_CENTER_PEOPLE_ID = 999000103;

const DAY = 24 * 60 * 60 * 1000;
const HOUR = 60 * 60 * 1000;

/**
 * @param {{ now?: number }} [ctx]
 * @returns {object} заявка в форме строки списка Центра и карточки
 */
export function buildDemoCenterApplication(ctx = {}) {
  const now = ctx.now || Date.now();
  const sent = new Date(now - DAY).toISOString();
  const stamp = sent.slice(0, 10).replace(/-/g, '');
  return {
    id: DEMO_CENTER_APPLICATION_ID,
    application_number: `№ ${stamp}/001`,
    // Согласование пройдено: колонка «Подтверждение» в списке иначе выглядит
    // пустой серой полосой, а кнопка приёма ждала бы чужого решения.
    confirmation: 'Согласовано',
    confirmation_datetime: new Date(now - 3 * HOUR).toISOString(),
    sending_datetime: sent,
    reading_datetime: null,
    organization_id: 999000,
    organization_name: 'Пример: ООО «Ремонтная бригада»',
    company_id: 0,
    company_name: '',
    // Наименование, заведённое подачей: даёт плашку разбора в карточке.
    organization_moderation_status: 'pending',
    company_moderation_status: 'approved',
    sender_user_id: 0,
    sender_full_name: 'Пример: Сидоров Олег Петрович',
    sender_name: 'Пример',
    sender_is_important: false,
    message: 'Пример заявки: привезут оборудование для монтажа вентиляции, две машины на два дня.',
    status: 'Непрочитано',
    responsible_user_id: null,
    responsible_full_name: '',
    responsible_name: '',
    responsible_comment: null,
    data_approval: true,
    has_blank_template: true,
    is_read: false,
    // Совпадение с чёрным списком: по нему виден тег в строке списка.
    blacklist_flags_count: 1,
    has_roof_access: true,
    has_free_parking: false,
    has_unseen_questions: false,
    has_files: false,
    has_status_update: false,
    // Дополнение, ждущее решения: по нему в карточке появляется блок раундов.
    has_open_supplement: true,
    supplements_count: 1,
    is_demo: true,
  };
}

/** Вложение примерной заявки: машины - у них есть и места разгрузки, и проезд. */
export function buildDemoCenterAttachments(now = Date.now()) {
  const from = new Date(now).toISOString().slice(0, 10);
  const to = new Date(now + DAY).toISOString().slice(0, 10);
  return [
    {
      id: DEMO_CENTER_ATTACHMENT_ID,
      attachment_type: 'cars',
      attachment_name: 'auto_blank',
      attachment_display_name: 'Автозаявка',
      entry_date_from: from,
      entry_date_to: to,
      entry_time_from: '08:00:00',
      entry_time_to: '20:00:00',
      roof_access: true,
      free_parking: false,
      created_at: null,
      unique_attachment_id: DEMO_CENTER_ATTACHMENT_ID,
      unique_attachment_display_name: 'Автозаявка',
      unique_attachment_title: 'АВТОЗАЯВКИ',
      has_template: true,
      archive_status: 'active',
    },
    {
      id: DEMO_CENTER_PEOPLE_ID,
      attachment_type: 'people',
      attachment_name: 'zayavka_work',
      attachment_display_name: 'Заявка на работы',
      entry_date_from: from,
      entry_date_to: to,
      entry_time_from: '09:00:00',
      entry_time_to: '18:00:00',
      roof_access: false,
      free_parking: false,
      created_at: null,
      unique_attachment_id: DEMO_CENTER_PEOPLE_ID,
      unique_attachment_display_name: 'Заявка на работы',
      unique_attachment_title: 'ЗАЯВКИ НА РАБОТЫ',
      has_template: true,
      archive_status: 'active',
    },
  ];
}

/**
 * Сотрудники примерной заявки: у людей вместо проезда места прохода. Первый
 * назначен, второй нет - на нём объясняется доназначение.
 */
export function buildDemoCenterEmployees() {
  return [
    {
      id: 0,
      last_name: 'Гаврилов', first_name: 'Пётр', middle_name: 'Игнатьевич',
      position: 'Монтажник', citizenship_id: 1, citizenship_name: 'Российская Федерация',
      passport_series_number: '4515 882014', patent_number: null, other_permission: null,
      entry_date_to: null, pass_time: null,
      organization: null, organization_id: null, company: null, company_id: null,
      target_tables: [{ id: 0, name: 'post_72', display_name: 'ПОСТ №72', source: 'application' }],
      is_blacklisted: false,
    },
    {
      id: 0,
      last_name: 'Дёмин', first_name: 'Артём', middle_name: 'Сергеевич',
      position: 'Электрик', citizenship_id: 1, citizenship_name: 'Российская Федерация',
      passport_series_number: '4517 331902', patent_number: null, other_permission: null,
      entry_date_to: null, pass_time: null,
      organization: null, organization_id: null, company: null, company_id: null,
      target_tables: [],
      is_blacklisted: false,
    },
  ];
}

/**
 * Согласующие примера: без них блок согласования пуст, и шаг про него нечем
 * наполнить - владелец увидел пустую рамку и справедливо спросил, где данные.
 */
export function buildDemoCenterResponsibleUsers(now = Date.now()) {
  const created = new Date(now - DAY).toISOString();
  return [
    {
      id: 0, username: 'demo_head', last_name: 'Волков', first_name: 'Владимир',
      middle_name: 'Ильич', position: 'Начальник участка', is_primary: true,
      required_approval: true, approval_status: 'approved',
      approval_comment: 'Работы согласованы, въезд с сопровождением.',
      approval_datetime: new Date(now - 5 * HOUR).toISOString(),
      created_at: created, reminder_count: 0,
    },
    {
      id: 0, username: 'demo_safety', last_name: 'Соколова', first_name: 'Анна',
      middle_name: 'Владимировна', position: 'Служба безопасности', is_primary: false,
      required_approval: true, approval_status: 'approved', approval_comment: null,
      approval_datetime: new Date(now - 3 * HOUR).toISOString(),
      created_at: created, reminder_count: 0,
    },
  ];
}

/**
 * Машины примерной заявки. Первая уже назначена - на ней видно, как выглядят
 * места и проезд; вторая пустая, и на ней объясняется доназначение.
 */
export function buildDemoCenterCars() {
  return [
    {
      id: 0,
      car_number: 'В207МК797',
      car_brand: 'BMW X5',
      unload_place: null,
      entry_date_from: null, entry_time_from: null, entry_date_to: null, entry_time_to: null,
      organization: null, organization_id: null, company: null, company_id: null,
      unload_places: [{ id: 0, name: 'Дебаркадер №1', description: null }],
      target_tables: [{ id: 0, name: 'kpp_4', display_name: 'КПП №4', source: 'application' }],
      is_blacklisted: false,
    },
    {
      id: 0,
      car_number: 'Е412ТР750',
      car_brand: 'Мерседес',
      unload_place: null,
      entry_date_from: null, entry_time_from: null, entry_date_to: null, entry_time_to: null,
      organization: null, organization_id: null, company: null, company_id: null,
      unload_places: [],
      target_tables: [],
      // Похоже на чёрный список: по этой отметке в туре объясняют тег в списке.
      blacklist_similar: { reason: 'Похожий номер в чёрном списке', matched: 'Е412ТР790' },
      is_blacklisted: false,
    },
  ];
}

/** Раунд дополнения: по нему в карточке рисуется блок с решением. */
export function buildDemoCenterSupplements(now = Date.now()) {
  return [
    {
      id: 0,
      application_id: DEMO_CENTER_APPLICATION_ID,
      round: 1,
      status: 'pending',
      created_at: new Date(now - 2 * HOUR).toISOString(),
      author_full_name: 'Пример: Сидоров Олег Петрович',
      comment: 'Добавили третью машину - подвезут кабель во второй день.',
      elements_count: 1,
    },
  ];
}

/**
 * Файлы, приложенные к примерной заявке: шаг про них рассказывал вслепую -
 * у примера файлов не было, и шаг молча выпадал из тура (#2622). Скачивание на
 * примере не работает: файла за этими именами на сервере нет, и шаг нажимать не
 * просит.
 */
export function buildDemoCenterFiles() {
  return [
    {
      id: 1,
      application_id: DEMO_CENTER_APPLICATION_ID,
      file_name: 'Договор подряда 14-2026.pdf',
      file_size: 384_512,
      mime_type: 'application/pdf',
    },
    {
      id: 2,
      application_id: DEMO_CENTER_APPLICATION_ID,
      file_name: 'Список оборудования.xlsx',
      file_size: 28_160,
      mime_type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    },
  ];
}
