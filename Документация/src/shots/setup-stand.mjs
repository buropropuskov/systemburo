/**
 * Донастройка съёмочного стенда поверх наливки `server fake`.
 *
 * Наливка заводит организации, работников, заявки, чёрные списки, таблицы
 * постов и отметки проходов, но двух вещей не делает: не выдаёт охраннику прав
 * на таблицы поста (проходы в наливке отмечает администратор партии) и не
 * заполняет инструкцию поста. Без первого охранник не увидит ни одной таблицы,
 * без второго снимок инструкции нечего показывать.
 *
 * Всё делается методами программного интерфейса, а не правкой базы: так стенд
 * проходит те же проверки, что и работа руками.
 *
 * Запуск: node Документация/src/shots/setup-stand.mjs [--api=http://localhost:8095/api]
 */

import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const HERE = path.dirname(fileURLToPath(import.meta.url));

/**
 * Права, которые получает охранник на каждую таблицу поста. Полный набор
 * глаголов шире (история, версии, корзина, удаление), но эти четыре - работа
 * администратора, а не поста, и в руководстве охранника не описываются.
 */
const GUARD_TABLE_VERBS = ['view', 'entry', 'exit', 'detail', 'report', 'export'];

const POST_INSTRUCTION =
  'Пропуск по действующей заявке. Сверьте государственный регистрационный знак ' +
  'с записью в таблице и срок действия пропуска. Отметьте въезд сразу после ' +
  'проезда машины, выезд - после того, как машина покинула территорию. ' +
  'Машину, которой нет в таблице, на территорию не пропускать: обратитесь в бюро ' +
  'пропусков по телефону 2-15.';

/**
 * Содержимое страницы «Обзор и новости». Демонстрационный набор из `cmd/seed`
 * заводит записи с названиями «Демо-новость» и «Важное демо-объявление» - на
 * снимке в документе для заказчика такое выглядит недоделкой, поэтому текст
 * заменяется правдоподобным. Организации и фамилии остаются вымышленными.
 */
const NEWS = [
  {
    title: 'Изменение режима работы бюро пропусков',
    description:
      'С 1 сентября бюро пропусков принимает заявки с 08:00 до 18:00 без перерыва. ' +
      'Заявки, поданные после 18:00, рассматриваются на следующий рабочий день.',
  },
  {
    title: 'Запуск электронной подачи заявок',
    description:
      'Заявки на проход людей, проезд транспорта и внос материальных ценностей ' +
      'принимаются в системе. Подавать их на бумаге больше не требуется.',
  },
];

const ANNOUNCEMENT = {
  title: 'Ограничение проезда к дебаркадеру №2',
  description:
    'С 12 по 16 августа проезд к дебаркадеру №2 закрыт из-за ремонта покрытия. ' +
    'Разгрузка на это время переносится на площадку «Склад №1».',
  is_important: true,
};

/**
 * Текст согласия на обработку персональных данных. Не образец для заказчика:
 * настоящий текст готовит юрист и загружает администратор. Здесь он нужен
 * ровно затем, чтобы окно согласия было чем наполнить на снимке.
 */
const CONSENT_TEXT = `<p>Настоящим я даю согласие на обработку моих персональных данных,
указанных в заявке на пропуск, а также персональных данных лиц, сведения о которых я
вношу в заявку, действуя с их ведома и согласия.</p>
<p>Обработка включает сбор, запись, систематизацию, хранение, уточнение, использование,
передачу сотрудникам службы охраны в объёме, необходимом для организации пропускного
режима, блокирование, удаление и уничтожение.</p>
<p>Согласие действует на срок оформления и действия пропуска, а также на срок хранения
сведений о проходе, установленный на предприятии. Согласие может быть отозвано
письменным обращением в бюро пропусков.</p>`;

const DOCUMENTS = [
  { title: 'Правила пропускного режима', description: 'Порядок прохода и проезда на территорию' },
  { title: 'Образец заявки на ввоз', description: 'Заполненный пример для материальных ценностей' },
  { title: 'Памятка о согласовании', description: 'Сроки рассмотрения и порядок обжалования отказа' },
];

/**
 * Собирает простейший PDF из одной страницы с заголовком.
 *
 * Файлы нужны только для того, чтобы блок «Документы» на снимке не был пустым:
 * заказчик читает про кнопку «Скачать», а пустой блок ничему не учит. Тащить
 * ради этого генератор PDF незачем, а положить готовые файлы в репозиторий -
 * значит хранить двоичные вложения, которые никто не откроет. Смещения таблицы
 * ссылок считаются по факту, поэтому файл валиден и открывается просмотрщиком.
 */
function makePdf(title) {
  const escape = (text) => text.replace(/([\\()])/g, '\\$1');
  const stream = `BT /F1 18 Tf 72 720 Td (${escape(title)}) Tj ET`;
  const objects = [
    '<< /Type /Catalog /Pages 2 0 R >>',
    '<< /Type /Pages /Kids [3 0 R] /Count 1 >>',
    '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] ' +
      '/Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>',
    '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>',
    `<< /Length ${stream.length} >>\nstream\n${stream}\nendstream`,
  ];

  let body = '%PDF-1.4\n';
  const offsets = [];
  objects.forEach((object, index) => {
    offsets.push(body.length);
    body += `${index + 1} 0 obj\n${object}\nendobj\n`;
  });

  const xrefAt = body.length;
  let xref = `xref\n0 ${objects.length + 1}\n0000000000 65535 f \n`;
  for (const offset of offsets) {
    xref += `${String(offset).padStart(10, '0')} 00000 n \n`;
  }
  const trailer =
    `trailer\n<< /Size ${objects.length + 1} /Root 1 0 R >>\nstartxref\n${xrefAt}\n%%EOF\n`;

  return Buffer.from(body + xref + trailer, 'latin1');
}

function arg(name, fallback) {
  const found = process.argv.find((value) => value.startsWith(`--${name}=`));
  return found ? found.slice(name.length + 3) : fallback;
}

async function api(base, token, method, endpoint, body) {
  const response = await fetch(`${base}${endpoint}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  });
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`${method} ${endpoint} -> ${response.status}: ${text.slice(0, 300)}`);
  }
  return text ? JSON.parse(text) : null;
}

const unwrap = (body) => (body && typeof body === 'object' && 'data' in body ? body.data : body);

async function main() {
  const apiBase = arg('api', 'http://localhost:8095/api');
  const accounts = JSON.parse(await readFile(path.join(HERE, 'accounts.json'), 'utf8'));

  const login = unwrap(
    await api(apiBase, null, 'POST', '/login', {
      username: accounts.superAdmin.username,
      password: accounts.superAdmin.password,
    }),
  );
  const token = login.token;

  /*
   * Список таблиц приходит вложенным: каждый элемент - {table: {...}} поверх
   * общей обёртки ответа. Без разворота имя таблицы читается как undefined, и
   * ключи прав собираются вида table.undefined.view.
   */
  const tables = unwrap(await api(apiBase, token, 'GET', '/system-tables')).map(
    (item) => item.table ?? item,
  );
  const active = tables.filter((table) => table.is_active !== false);
  if (active.length === 0) throw new Error('на стенде нет ни одной таблицы поста');

  // Охранник: персональные разрешения на таблицы постов.
  const users = unwrap(await api(apiBase, token, 'GET', '/users/all'));
  const list = Array.isArray(users) ? users : (users.users ?? users.items ?? []);
  const guard = list.find((user) => user.username === accounts.roles.guard.username);
  if (!guard) throw new Error(`охранник ${accounts.roles.guard.username} не найден`);

  const permissions = [{ key: 'page.tables', value: 'allow' }];
  for (const table of active) {
    for (const verb of GUARD_TABLE_VERBS) {
      permissions.push({ key: `table.${table.name}.${verb}`, value: 'allow' });
    }
  }
  await api(apiBase, token, 'PUT', `/permissions/user/${guard.id}`, { permissions });
  console.log(
    `Охраннику ${guard.username} выдано разрешений: ${permissions.length} (таблиц: ${active.length})`,
  );

  // Таблица «по факту» и инструкция поста - на первой таблице машин.
  const carsTable = active.find((table) => table.table_type === 'cars') ?? active[0];
  await api(apiBase, token, 'PUT', `/system-tables/${carsTable.id}`, {
    show_fact_table: true,
    instruction: POST_INSTRUCTION,
  });
  console.log(`Таблица «${carsTable.display_name}»: включён список по факту, заполнена инструкция`);

  await bindGuardPlaces(apiBase, token, guard.username, active);
  await grantRolePermissions(apiBase, token, list, accounts);

  await fillOverview(apiBase, token);
  await fillBureauSchedule(apiBase, token);
  await enableConsent(apiBase, token);
}

/**
 * Выдаёт согласующему и принимающему права, без которых их сценарии не
 * открываются.
 *
 * Наливка расставляет признаки ролей в данных (обязательное согласование в
 * организации, запись в справочнике принимающих), но правами не занимается: в
 * работе их выдаёт администратор. Без «Центра заявок» согласующий упирается в
 * страницу отказа, и снимать в его руководстве нечего.
 */
async function grantRolePermissions(apiBase, token, users, accounts) {
  const grants = {
    approver: ['page.center', 'action.approve.application', 'action.forward.application'],
    acceptor: ['page.center', 'action.approve.application', 'center.archive', 'action.supplement.application'],
  };

  for (const [role, keys] of Object.entries(grants)) {
    const account = accounts.roles[role];
    const user = users.find((item) => item.username === account.username);
    if (!user) throw new Error(`${role} ${account.username} не найден`);
    await api(apiBase, token, 'PUT', `/permissions/user/${user.id}`, {
      permissions: keys.map((key) => ({ key, value: 'allow' })),
    });
    console.log(`${account.username} (${role}): выдано разрешений ${keys.length}`);
  }
}

/**
 * Привязывает охраннику места доступа: таблицы постов и места разгрузки.
 *
 * Права на таблицы открывают сами таблицы, но раздел «Доступные мне» устроен
 * иначе: он показывает вложения согласованных заявок, места которых
 * пересекаются с местами работника. Без привязки раздел пуст, и снимок в
 * руководстве охранника показывал бы пустой экран.
 */
async function bindGuardPlaces(apiBase, token, username, tables) {
  const tableIDs = tables.map((table) => table.id);
  await api(apiBase, token, 'PUT', `/users/${username}/tables`, { table_ids: tableIDs });

  const places = unwrap(await api(apiBase, token, 'GET', '/unload-places')) ?? [];
  const placeIDs = places
    .filter((place) => place.is_active !== false)
    .map((place) => place.id);
  await api(apiBase, token, 'PUT', `/users/${username}/unload-places`, {
    unload_place_ids: placeIDs,
  });

  console.log(
    `Охраннику ${username} привязано мест доступа: таблиц ${tableIDs.length}, мест разгрузки ${placeIDs.length}`,
  );
}

/**
 * Заполняет расписание бюро пропусков.
 *
 * Без него окно «Режимы работы» показывает выходной все семь дней и «Закрыто
 * сейчас» в любое время. Руководство призывает сверяться с этим окном перед
 * подачей заявки, и снимок, на котором бюро закрыто всегда, читателя только
 * запутает.
 */
async function fillBureauSchedule(apiBase, token) {
  const existing = unwrap(await api(apiBase, token, 'GET', '/bureau/time-slots')) ?? [];
  if (existing.length > 0) {
    console.log(`Расписание бюро: уже задано, промежутков ${existing.length}`);
    return;
  }
  // 0 - понедельник, 6 - воскресенье. Будни целиком, суббота короче,
  // воскресенье не заводим - в окне оно и будет выходным.
  const slots = [
    ...[0, 1, 2, 3, 4].map((day) => ({ day_of_week: day, open_time: '08:00', close_time: '18:00' })),
    { day_of_week: 5, open_time: '09:00', close_time: '14:00' },
  ];
  for (const slot of slots) {
    await api(apiBase, token, 'POST', '/bureau/time-slots', { ...slot, is_active: true });
  }
  console.log(`Расписание бюро: заведено промежутков ${slots.length}`);
}

/**
 * Включает запрос согласия на обработку персональных данных при входе.
 *
 * Окно согласия видит каждый работник при первом входе, значит оно обязано быть
 * в руководстве со снимком. На чистом стенде запрос выключен: включить его без
 * текста система не даёт, а текст никто не задавал.
 *
 * Съёмочные учётные записи подтверждают согласие в момент входа (см.
 * lib/session.mjs), иначе окно перекрывало бы все остальные кадры.
 */
async function enableConsent(apiBase, token) {
  const current = unwrap(await api(apiBase, token, 'GET', '/settings/pd-consent'));
  const sameText = (current?.text ?? '').trim() === CONSENT_TEXT.trim();

  if (!sameText) {
    /*
     * `require_again` поднимает редакцию, а вместе с ней проставляется дата, с
     * которой редакция действует. Без него дата остаётся пустой, и в окне
     * согласия видна одна «РЕДАКЦИЯ 1» - тогда как руководство обещает
     * читателю номер редакции и дату. Заодно повторный запуск донастройки
     * доносит до стенда правку текста: сравнение по тексту, а не по признаку
     * «запрос уже включён».
     */
    await api(apiBase, token, 'PUT', '/settings/pd-consent/text', {
      text: CONSENT_TEXT,
      require_again: true,
    });
  }
  /*
   * Дата редакции проставляется только подъёмом редакции. На стенде, где текст
   * задали один раз без подъёма, она осталась пустой, и в окне согласия видна
   * одна «РЕДАКЦИЯ 1» без даты.
   */
  if (sameText && !current?.version_at) {
    await api(apiBase, token, 'POST', '/settings/pd-consent/require-again', {});
  }
  if (!current?.required) {
    await api(apiBase, token, 'PUT', '/settings/pd-consent/required', { required: true });
  }
  console.log('Согласие: текст, дата редакции и запрос при входе настроены');
}

/** Наполняет страницу «Обзор и новости»: новости, объявление, документы. */
async function fillOverview(apiBase, token) {
  /*
   * Донастройка обязана быть повторяемой: каждая заведённая новость и каждый
   * документ рассылают уведомления всем работникам, и повторный прогон со
   * сносом-пересозданием засыпал бы съёмочную учётную запись дубликатами - в
   * кадре списка уведомлений это сразу видно. Поэтому сверяем по заголовку и
   * трогаем только то, чего нет.
   */
  const existingNews = unwrap(await api(apiBase, token, 'GET', '/news/all')) ?? [];
  const newsTitles = new Set(existingNews.map((item) => item.title));
  for (const item of existingNews) {
    if (!NEWS.some((wanted) => wanted.title === item.title)) {
      await api(apiBase, token, 'DELETE', `/news/${item.id}`);
    }
  }
  let addedNews = 0;
  for (const item of NEWS) {
    if (newsTitles.has(item.title)) continue;
    await api(apiBase, token, 'POST', '/news', { ...item, is_active: true });
    addedNews += 1;
  }
  console.log(`Новости: было ${existingNews.length}, добавлено ${addedNews}`);

  const existingAnnouncements = unwrap(await api(apiBase, token, 'GET', '/announcements/all')) ?? [];
  let announcement = existingAnnouncements.find((item) => item.title === ANNOUNCEMENT.title);
  for (const item of existingAnnouncements) {
    if (item.title !== ANNOUNCEMENT.title) {
      await api(apiBase, token, 'DELETE', `/announcements/${item.id}`);
    }
  }
  if (!announcement) {
    announcement = unwrap(await api(apiBase, token, 'POST', '/announcements', ANNOUNCEMENT));
  }
  if (announcement?.id) {
    await api(apiBase, token, 'POST', '/announcements/set-active', {
      announcement_id: announcement.id,
    });
  }
  console.log('Объявление: активно');

  const existingDocs = unwrap(await api(apiBase, token, 'GET', '/documents')) ?? [];
  const haveTitles = new Set(existingDocs.map((item) => item.title));
  for (const doc of DOCUMENTS) {
    if (haveTitles.has(doc.title)) continue;
    const form = new FormData();
    form.append('file', new Blob([makePdf(doc.title)], { type: 'application/pdf' }), `${doc.title}.pdf`);
    form.append('title', doc.title);
    form.append('description', doc.description);
    const response = await fetch(`${apiBase}/documents`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
      body: form,
    });
    if (!response.ok) {
      throw new Error(`загрузка документа «${doc.title}» -> ${response.status}: ${(await response.text()).slice(0, 200)}`);
    }
  }
  console.log(`Документы: всего ${DOCUMENTS.length}`);
}

main().catch((error) => {
  console.error(`Донастройка стенда не удалась: ${error.message}`);
  process.exit(1);
});
