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

const DAY = 24 * 60 * 60 * 1000;

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
    // Согласующих у примера нет: тогда кнопка приёма называется «Согласовать и
    // принять» и доступна - именно её объясняет шаг про главное действие роли.
    confirmation: null,
    confirmation_datetime: null,
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
      created_at: new Date(now - 2 * 60 * 60 * 1000).toISOString(),
      author_full_name: 'Пример: Сидоров Олег Петрович',
      comment: 'Добавили третью машину - подвезут кабель во второй день.',
      elements_count: 1,
    },
  ];
}
