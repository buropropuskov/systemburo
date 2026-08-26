/**
 * Примерная заявка для онбординга.
 *
 * Обучение одинаково для всех: человек, который ещё не подавал заявок, проходит
 * те же шаги про карточку, что и остальные, - просто данные в ней примерные.
 * Раньше такие шаги просто выпадали из тура, и новичок не узнавал ни про статус,
 * ни про вопросы, ни про «отозвать» - то есть ровно про то, что ему предстоит.
 *
 * Данные выдуманы здесь и никуда не отправляются: подмена работает только на
 * чтение и только пока идёт тур (см. readInterceptor.js). Организацию и отправителя
 * называем нейтрально («Ваша организация», «Вы»): выдуманное название компании в
 * своей же карточке читалось бы как чужая заявка, а не как пример.
 */

/** Идентификатор заведомо вне диапазона живых заявок - чтобы не спутать в логах. */
export const DEMO_APPLICATION_ID = 999000001;

/** Вложение примерной заявки: по нему грузится состав. */
export const DEMO_ATTACHMENT_ID = 999000101;

const DAY = 24 * 60 * 60 * 1000;

/**
 * @param {{ organization?: string, company?: string, fullName?: string, now?: number }} ctx
 * @returns {object} заявка в форме ответа `/applications/user`
 */
export function buildDemoApplication(ctx = {}) {
  const now = ctx.now || Date.now();
  const sentDate = new Date(now - 2 * DAY);
  const sent = sentDate.toISOString();
  // Номер в системе собирается из даты подачи - у примера он такой же, иначе
  // шапка карточки выглядит собранной из разных заявок.
  const stamp = sent.slice(0, 10).replace(/-/g, '');
  return {
    id: DEMO_APPLICATION_ID,
    application_number: `№ ${stamp}/007`,
    confirmation: 'Согласование',
    sending_datetime: sent,
    reading_datetime: sent,
    confirmation_datetime: null,
    organization_id: 0,
    organization_name: ctx.organization || 'Ваша организация',
    company_id: 0,
    company_name: ctx.company || 'Ваша компания',
    organization_moderation_status: 'approved',
    company_moderation_status: 'approved',
    sender_user_id: 0,
    sender_full_name: ctx.fullName || 'Вы',
    sender_name: ctx.fullName || 'Вы',
    sender_is_important: false,
    message: 'Пример заявки: привозим оборудование для монтажа, две машины и трое рабочих.',
    status: 'В обработке',
    responsible_user_id: null,
    responsible_full_name: '',
    responsible_name: '',
    responsible_comment: null,
    data_approval: true,
    has_blank_template: true,
    is_read: true,
    blacklist_flags_count: 0,
    has_roof_access: false,
    has_free_parking: false,
    has_unseen_questions: false,
    has_files: false,
    has_status_update: false,
    has_open_supplement: false,
    supplements_count: 0,
    // Метка для интерфейса: карточка и строка списка могут показать, что это пример.
    is_demo: true,
  };
}

/** Согласующие примерной заявки - показывают блок статуса и список ответственных. */
export function buildDemoResponsibleUsers(now = Date.now()) {
  const created = new Date(now - 2 * DAY).toISOString();
  return [
    {
      id: 0, username: 'demo_approver', last_name: 'Петров', first_name: 'Игорь',
      middle_name: 'Сергеевич', position: 'Начальник участка', is_primary: true,
      required_approval: true, approval_status: 'approved', approval_comment: null,
      approval_datetime: new Date(now - DAY).toISOString(), created_at: created, reminder_count: 0,
    },
    {
      id: 0, username: 'demo_reviewer', last_name: 'Соколова', first_name: 'Анна',
      middle_name: 'Владимировна', position: 'Служба безопасности', is_primary: false,
      required_approval: true, approval_status: 'pending', approval_comment: null,
      approval_datetime: null, created_at: created, reminder_count: 0,
    },
  ];
}

/** Состав примерной заявки: одно вложение с товарами. */
export function buildDemoAttachments(now = Date.now()) {
  const day = new Date(now + DAY).toISOString().slice(0, 10);
  return [
    {
      id: DEMO_ATTACHMENT_ID,
      attachment_type: 'items',
      attachment_name: 'zayavka_na_vvoz',
      attachment_display_name: 'Заявка на ввоз',
      entry_date_from: day,
      entry_date_to: day,
      entry_time_from: '09:00:00',
      entry_time_to: '18:00:00',
      roof_access: false,
      free_parking: false,
      created_at: null,
      unique_attachment_id: DEMO_ATTACHMENT_ID,
      unique_attachment_display_name: 'Заявка на ввоз',
      unique_attachment_title: 'Заявка на ввоз',
      has_template: true,
      archive_status: 'active',
    },
  ];
}

/** Товары примерной заявки - их показывает шаг про состав. */
export function buildDemoItems() {
  return [
    { id: 0, name: 'Стеллаж металлический', count: 4, order_index: 0 },
    { id: 0, name: 'Ящик с инструментом', count: 2, order_index: 1 },
    { id: 0, name: 'Кабель силовой, бухта', count: 3, order_index: 2 },
  ];
}
