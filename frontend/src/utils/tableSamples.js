/**
 * Примеры данных для превью системных таблиц (#345).
 * Используется в SystemTableColumnsTab и CarsTable/PeopleTable в preview-режиме.
 */

const SAMPLE_VALUES = {
  car_number: ['А 123 ВЕ 77', 'М 456 ОК 99', 'О 789 ЕМ 50', 'Х 012 ВН 77', 'Т 345 КН 99',
    'Е 678 МН 50', 'У 901 ОТ 77', 'К 234 РС 99', 'В 567 ТУ 50', 'С 890 ХР 77'],
  car_brand: ['Тойота', 'Лада', 'Газель', 'Камаз', 'Вольво', 'МАН', 'Мерседес', 'Рено', 'Скания', 'ДАФ'],
  organization: ['ООО Альфа', 'ООО Бета', 'ЗАО Гамма', 'ИП Дельта', 'ООО Эпсилон',
    'АО Дзета', 'ООО Эта', 'ИП Тета', 'ООО Йота', 'ЗАО Каппа'],
  company: ['Альфа-Сервис', 'Бета-Логистик', 'Гамма-Транс', 'Дельта-Карго', 'Эпсилон-Экспресс',
    'Дзета-Авто', 'Эта-Логист', 'Тета-Линия', 'Йота-Доставка', 'Каппа-Транс'],
  application_id: ['20260530/00148', '20260530/00149', '20260530/00150', '20260530/00151',
    '20260530/00152', '20260531/00001', '20260531/00002', '20260531/00003', '20260531/00004', '20260531/00005'],
  unload_place: ['Дебаркадер №1', 'Дебаркадер №2', 'Склад А', 'Склад Б', 'Площадка №3',
    'Зона разгрузки', 'Пандус', 'Склад В', 'Площадка №7', 'Дебаркадер №5'],
  valid_until: ['31.05.2026', '01.06.2026', '15.06.2026', '30.06.2026', '07.06.2026',
    '14.06.2026', '21.06.2026', '28.06.2026', '05.07.2026', '12.07.2026'],
  time_range: ['08:00 - 23:59', '09:00 - 18:00', '06:00 - 22:00', '10:00 - 16:00', '08:00 - 20:00',
    '07:00 - 19:00', '00:00 - 23:59', '08:00 - 17:00', '09:00 - 21:00', '06:00 - 14:00'],
  last_name: ['Иванов', 'Петров', 'Сидоров', 'Кузнецов', 'Смирнов',
    'Попов', 'Лебедев', 'Соколов', 'Морозов', 'Волков'],
  first_name: ['Иван', 'Пётр', 'Александр', 'Сергей', 'Михаил',
    'Андрей', 'Дмитрий', 'Николай', 'Алексей', 'Владимир'],
  middle_name: ['Иванович', 'Петрович', 'Сергеевич', 'Александрович', 'Михайлович',
    'Андреевич', 'Дмитриевич', 'Николаевич', 'Алексеевич', 'Владимирович'],
  position: ['Грузчик', 'Водитель', 'Экспедитор', 'Кладовщик', 'Менеджер',
    'Оператор', 'Инженер', 'Логист', 'Контролёр', 'Бригадир'],
  citizenship_name: ['Россия', 'Россия', 'Беларусь', 'Казахстан', 'Россия',
    'Узбекистан', 'Россия', 'Армения', 'Россия', 'Таджикистан'],
  pass_time: ['10:00 - 15:00', '09:00 - 18:00', '08:00 - 17:00', '07:00 - 14:00', '11:00 - 19:00',
    '06:00 - 12:00', '13:00 - 21:00', '08:00 - 16:00', '10:00 - 18:00', '14:00 - 22:00'],
};

/**
 * Генерирует N строк примерных данных для таблицы заданного типа.
 * @param {'cars'|'people'} tableType
 * @param {number} count
 * @returns {Array<object>} массив строк в формате, ожидаемом CarsTable/PeopleTable
 */
export function generateSampleRows(tableType, count = 10) {
  const rows = [];
  for (let i = 0; i < count; i++) {
    const pick = (key) => {
      const values = SAMPLE_VALUES[key];
      return values ? values[i % values.length] : null;
    };
    if (tableType === 'cars') {
      const [tf, tt] = pick('time_range').split(' - ');
      rows.push({
        id: -1 - i,
        car_number: pick('car_number'),
        car_brand: pick('car_brand'),
        organization_id: i + 1,
        organization_name: pick('organization'),
        company: pick('company'),
        company_id: i + 1,
        unload_place: pick('unload_place'),
        unload_place_ids: [],
        entry_date_to: '2026-06-15',
        entry_time_from: tf,
        entry_time_to: tt,
        status: 'В работе',
        entry_checked: false,
        exit_checked: false,
        applicationId: i + 100,
        applicationNumber: pick('application_id'),
        territory_status: 0,
      });
    } else if (tableType === 'people') {
      const [tf, tt] = pick('pass_time').split(' - ');
      rows.push({
        id: -1 - i,
        last_name: pick('last_name'),
        first_name: pick('first_name'),
        middle_name: pick('middle_name'),
        organization_id: i + 1,
        organization_name: pick('organization'),
        company: pick('company'),
        company_id: i + 1,
        position: pick('position'),
        citizenshipName: pick('citizenship_name'),
        entry_date_to: '2026-06-15',
        pass_time: `${tf} - ${tt}`,
        status: 'Активен',
        applicationId: i + 100,
        applicationNumber: pick('application_id'),
        entry_checked: false,
        exit_checked: false,
        territory_status: 0,
      });
    }
  }
  return rows;
}
