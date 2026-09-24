/**
 * Срок заявки в двух видах: как его хранит сервер (ГГГГ-ММ-ДД, ЧЧ:ММ:СС) и как его
 * редактирует DateRangeSection формы подачи (ДД.ММ.ГГГГ, ЧЧ:ММ, признак «один день»).
 * Нужен окну правки срока принимающим (#2575): оно переиспользует тот же блок дат.
 */

/** Статусы, в которых принимающий может сдвинуть срок - зеркало серверного белого списка. */
export const DATES_EDITABLE_STATUSES = ['Непрочитано', 'В обработке'];

/** Итоги согласования, после которых срок уже обещан охране и заявителю. */
const FINAL_CONFIRMATIONS = ['Согласовано', 'Не согласовано'];

/**
 * Можно ли сейчас менять срок заявки. Сервер проверяет то же самое под блокировкой;
 * здесь - чтобы не показывать кнопку, которая заведомо получит отказ.
 * @param {{status?: string, confirmation?: string|null}|null} application
 * @returns {boolean}
 */
export function canEditApplicationDates(application) {
  if (!application) return false;
  if (!DATES_EDITABLE_STATUSES.includes(application.status)) return false;
  return !FINAL_CONFIRMATIONS.includes(application.confirmation);
}

function isoToRu(iso) {
  if (!iso) return '';
  const [year, month, day] = String(iso).slice(0, 10).split('-');
  return year && month && day ? `${day}.${month}.${year}` : '';
}

function ruToIso(ru) {
  if (!ru) return '';
  const [day, month, year] = String(ru).split('.');
  return year && month && day ? `${year}-${month}-${day}` : '';
}

function clock(value) {
  return value ? String(value).slice(0, 5) : '';
}

/**
 * Состояние DateRangeSection из окна вложения.
 * @param {{entry_date_from?: string, entry_date_to?: string, entry_time_from?: string, entry_time_to?: string}|null} attachment
 */
export function periodFormFromAttachment(attachment) {
  const from = isoToRu(attachment?.entry_date_from);
  const to = isoToRu(attachment?.entry_date_to);
  const isOneDay = !!from && from === to;
  return {
    isOneDay,
    startDate: isOneDay ? '' : from,
    endDate: isOneDay ? '' : to,
    singleDate: isOneDay ? from : '',
    startTime: clock(attachment?.entry_time_from),
    endTime: clock(attachment?.entry_time_to),
  };
}

/**
 * Тело запроса PUT /applications/:id/dates из состояния DateRangeSection.
 * @returns {{entry_date_from: string, entry_date_to: string, entry_time_from: string, entry_time_to: string}}
 */
export function periodPayloadFromForm(form) {
  const from = form.isOneDay ? form.singleDate : form.startDate;
  const to = form.isOneDay ? form.singleDate : form.endDate;
  return {
    entry_date_from: ruToIso(from),
    entry_date_to: ruToIso(to),
    entry_time_from: form.startTime ? `${form.startTime}:00` : '',
    entry_time_to: form.endTime ? `${form.endTime}:00` : '',
  };
}

/**
 * Ошибки окна в тех ключах, что понимает DateRangeSection. Пусто - можно сохранять.
 * Правила те же, что у сервера: всё заполнено, конец позже начала.
 */
export function periodFormErrors(form) {
  const errors = {};
  const payload = periodPayloadFromForm(form);
  if (form.isOneDay) {
    if (!payload.entry_date_from) errors.singleDate = 'Укажите дату';
  } else {
    if (!payload.entry_date_from) errors.startDate = 'Укажите дату начала';
    if (!payload.entry_date_to) errors.endDate = 'Укажите дату окончания';
  }
  if (!/^\d{2}:\d{2}$/.test(form.startTime || '')) errors.startTime = 'Укажите время начала';
  if (!/^\d{2}:\d{2}$/.test(form.endTime || '')) errors.endTime = 'Укажите время окончания';
  if (Object.keys(errors).length) return errors;

  const start = `${payload.entry_date_from}T${payload.entry_time_from}`;
  const end = `${payload.entry_date_to}T${payload.entry_time_to}`;
  if (payload.entry_date_to < payload.entry_date_from) {
    errors.endDate = 'Дата окончания не может быть раньше даты начала';
  } else if (end <= start) {
    errors.endTime = 'Время окончания должно быть позже времени начала';
  }
  return errors;
}

/** Окно для человека: «01.10.2026 09:00 - 03.10.2026 18:00». */
export function formatPeriod(attachment) {
  const side = (date, time) => [isoToRu(date), clock(time)].filter(Boolean).join(' ');
  return `${side(attachment.entry_date_from, attachment.entry_time_from)} - ${side(attachment.entry_date_to, attachment.entry_time_to)}`;
}
