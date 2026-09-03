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

import { createRequire } from 'node:module';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const HERE = path.dirname(fileURLToPath(import.meta.url));

/*
 * Работа с электронными таблицами уже есть в зависимостях веб-части, поэтому
 * пакет резолвится от неё явно - тем же приёмом, что и playwright в lib/session.
 */
const REPO_ROOT = path.resolve(HERE, '..', '..', '..');
const requireFromFrontend = createRequire(path.join(REPO_ROOT, 'frontend', 'package.json'));

/**
 * Права, которые получает охранник на каждую таблицу поста. Полный набор
 * глаголов шире (история, версии, корзина, удаление), но это работа
 * администратора, а не поста, и в руководстве охранника не описывается.
 *
 * Выгрузки в наборе нет намеренно: на постах заказчика её не выдают, а кадр с
 * кнопкой «Экспорт» обещал бы работнику возможность, которой у него не будет.
 */
const GUARD_TABLE_VERBS = ['view', 'entry', 'exit', 'detail', 'report'];

/**
 * Остальные глаголы прав таблицы. Они не просто не выдаются, а гасятся явным
 * запретом: обновление прав меняет только присланные ключи, и разрешение,
 * выданное прошлым прогоном, иначе остаётся навсегда - так на кадрах ещё долго
 * висела кнопка «Экспорт» после того, как её убрали из набора.
 */
const GUARD_TABLE_VERBS_DENIED = ['history', 'versions', 'export', 'trash', 'delete'];

/**
 * Права, которые у охранника снимаются персонально поверх базовой роли.
 * Паспорт и патент посту не показывают: карточка человека открывает их по
 * detail.documents, и на стенде это право приходит из базовой роли
 * «Пользователь». Без запрета кадр карточки показывал бы раздел «Документы».
 */
const GUARD_DENIED_PERMISSIONS = ['detail.documents'];

/**
 * Посты стенда: системное имя таблицы наливки -> название на экране. Наливка
 * заводит четыре таблицы с описательными названиями («Центральный КПП»), а на
 * проходной посты называют коротко и номером - именно так они выглядят у
 * заказчика, и именно так их читает работник в списке.
 */
const POST_NAMES = {
  'kpp-cargo': 'КПП №4',
  'kpp-central': 'ПОСТ №72',
  'checkpoint-main': 'ПОСТ СЕВЕР',
  'checkpoint-service': 'ПОСТ №27',
};

/** Пятый пост: в наливке таблиц людей две, а в списке их должно быть три. */
const EXTRA_POST = { name: 'post-21', display_name: 'ПОСТ №21', table_type: 'people' };

const POST_INSTRUCTION =
  'Пропуск по действующей заявке. Сверьте государственный регистрационный знак ' +
  'с записью в таблице и срок действия пропуска. Отметьте въезд сразу после ' +
  'проезда машины, выезд - после того, как машина покинула территорию. ' +
  'Машину, которой нет в таблице, на территорию не пропускать: обратитесь в бюро ' +
  'пропусков.';

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

/**
 * Обращения работников в бюро пропусков. Первое закрыто ответом, остальные
 * ждут разбора: так в кадре видны обе вкладки раздела и поле ответа.
 */
const FEEDBACK = [
  {
    role: 'user',
    message:
      'При подаче заявки не нашёл в списке место разгрузки «Склад №3». Раньше оно было, ' +
      'сейчас его нет, а машина приходит именно туда.',
    answer:
      'Место разгрузки «Склад №3» убрано в архив на время ремонта. До 20 сентября указывайте ' +
      '«Склад №1», проезд к нему открыт с той же стороны.',
  },
  {
    role: 'approver',
    message:
      'Письма о новых заявках приходят с задержкой в несколько часов, а заявки срочные. ' +
      'Можно ли настроить, чтобы уведомление приходило сразу?',
  },
  {
    role: 'acceptor',
    message:
      'В выгрузке бланка по заявке не заполняется столбец с должностью, хотя в заявке ' +
      'должность указана. Проверьте, пожалуйста, привязку поля к ячейке.',
  },
];

/** Статусы, при которых заявка закрыта и решения согласующего уже не ждёт. */
const ARCHIVED_STATUSES = new Set(['Завершено', 'Не согласовано', 'Отказано', 'Отозвана']);

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
  let tables = unwrap(await api(apiBase, token, 'GET', '/system-tables')).map(
    (item) => item.table ?? item,
  );
  if (tables.length === 0) throw new Error('на стенде нет ни одной таблицы поста');

  tables = await renamePosts(apiBase, token, tables);
  const active = tables.filter((table) => table.is_active !== false);

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
  for (const table of active) {
    for (const verb of GUARD_TABLE_VERBS_DENIED) {
      permissions.push({ key: `table.${table.name}.${verb}`, value: 'deny' });
    }
  }
  for (const key of GUARD_DENIED_PERMISSIONS) {
    permissions.push({ key, value: 'deny' });
  }
  await api(apiBase, token, 'PUT', `/permissions/user/${guard.id}`, { permissions });
  console.log(
    `Охраннику ${guard.username} выдано разрешений: ${permissions.length} (таблиц: ${active.length})`,
  );

  // Таблица «по факту» и инструкция поста - на первой таблице машин.
  // Список «по факту» и инструкция - на «КПП №4»: кадры манифеста сняты именно с
  // него (/table/kpp-cargo), и «первая попавшаяся таблица машин» после
  // переименования могла бы оказаться другой.
  const carsTable =
    active.find((table) => table.name === 'kpp-cargo') ??
    active.find((table) => table.table_type === 'cars') ??
    active[0];
  await api(apiBase, token, 'PUT', `/system-tables/${carsTable.id}`, {
    show_fact_table: true,
    instruction: POST_INSTRUCTION,
  });
  console.log(`Таблица «${carsTable.display_name}»: включён список по факту, заполнена инструкция`);

  await bindGuardPlaces(apiBase, token, guard.username, active);
  await grantRolePermissions(apiBase, token, list, accounts);
  await tidyAttachmentKinds(apiBase, token);
  await acceptConsentForRecipients(apiBase, accounts);

  await fillOverview(apiBase, token);
  await fillBureauSchedule(apiBase, token);
  await enableConsent(apiBase, token);
  await seedFeedback(apiBase, token, accounts);
  await ensureBlankTemplate(apiBase, token);
  await ensurePendingApproval(apiBase, token, accounts);
  await ensureBlacklistFlag(apiBase, token, accounts);
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
 * Подтверждает согласие на обработку данных за тех, кто попадает в строку
 * получателей заявки.
 *
 * Работник без согласия виден в системе заглушкой вместо фамилии (#1567), и на
 * съёмочном стенде согласия нет ни у кого: наливка его не спрашивает. В кадре
 * экрана подачи из-за этого стоял бы логин вместо человека, а раздел про
 * получателей учил бы читателя по вырожденному случаю.
 *
 * Согласие даёт сам работник, поэтому оно и подтверждается входом под его
 * учётной записью, а не правкой базы от имени администратора. Пароль у всех
 * заведённых наливкой один и записан в accounts.json.
 */
async function acceptConsentForRecipients(apiBase, accounts) {
  // Список кандидатов зависит от того, кто спрашивает: это коллеги заявителя и
  // руководители. Значит и спрашивать надо от его имени, а не от администратора,
  // у которого коллеги свои.
  const applicant = unwrap(
    await api(apiBase, null, 'POST', '/login', {
      username: accounts.roles.user.username,
      password: accounts.password,
    }),
  );
  const candidates = unwrap(
    await api(apiBase, applicant.token, 'GET', '/users/recipient-candidates'),
  );

  /*
   * Строка получателей набирается из двух разных списков: кандидаты в читатели
   * приходят своим методом, а согласующие по умолчанию - перечнем работников
   * организации и компании. Расшифровать надо оба, иначе в кадре останется
   * логин там, где должна стоять фамилия согласующего.
   */
  const profile = unwrap(await api(apiBase, applicant.token, 'GET', '/user-data'));
  const colleagues = [];
  for (const [id, endpoint] of [
    [profile.organization_id, 'organizations'],
    [profile.company_id, 'companies'],
  ]) {
    if (!id) continue;
    const users = unwrap(await api(apiBase, applicant.token, 'GET', `/${endpoint}/${id}/users`));
    colleagues.push(...(Array.isArray(users) ? users : []));
  }

  const list = [...(Array.isArray(candidates) ? candidates : []), ...colleagues];
  let accepted = 0;

  for (const candidate of list) {
    // Скрытого работника видно по тому же признаку, что и на экране: фамилии нет.
    if (!candidate.pd_hidden && candidate.last_name) continue;
    const personal = await api(apiBase, null, 'POST', '/login', {
      username: candidate.username,
      password: accounts.password,
    }).catch(() => null);
    const personalToken = unwrap(personal)?.token;
    if (!personalToken) continue;
    await api(apiBase, personalToken, 'POST', '/consents/accept', {});
    accepted += 1;
  }

  console.log(`Согласие получателей: подтверждено за ${accepted} из ${list.length}`);
}

/**
 * Приводит виды вложений к одному набору из трёх.
 *
 * Наливка и демонстрационный набор заводят каждый свои виды, и на экране подачи
 * получается шесть колонок: «АВТОМОБИЛИ» рядом с «Автомобили», и так трижды.
 * Заказчик увидел бы на снимке недоделку, а описать порядок «выберите колонку»
 * по такому экрану нельзя. Оставляем по одному виду на тип - тот, которым
 * заполнены заявки стенда, - и называем его так, как назвал бы администратор.
 *
 * Скрытые виды удаляются мягко (is_active = false), поэтому вложения уже
 * поданных заявок на них по-прежнему ссылаются и в карточках заявок ничего не
 * пропадает.
 */
const ATTACHMENT_KINDS = {
  people: { name: 'people', display_name: 'Сотрудники', title: 'СОТРУДНИКИ' },
  cars: { name: 'cars', display_name: 'Автомобили', title: 'АВТОМОБИЛИ' },
  items: { name: 'items', display_name: 'Материальные ценности', title: 'ЦЕННОСТИ' },
};

async function tidyAttachmentKinds(apiBase, token) {
  const kinds = unwrap(await api(apiBase, token, 'GET', '/attachments/all')) ?? [];
  const byType = new Map();
  const extra = [];

  // Первым по каждому типу берём тот вид, что завела наливка: на него ссылаются
  // сотни вложений уже поданных заявок, и именно он попадает в кадры кабинета.
  for (const kind of [...kinds].sort((a, b) => a.id - b.id)) {
    if (byType.has(kind.attachment_type)) {
      if (kind.is_active !== false) extra.push(kind);
      continue;
    }
    byType.set(kind.attachment_type, kind);
  }

  for (const [type, kind] of byType) {
    const wanted = ATTACHMENT_KINDS[type];
    if (!wanted) continue;
    const same =
      kind.name === wanted.name &&
      kind.display_name === wanted.display_name &&
      kind.title === wanted.title &&
      kind.is_active !== false;
    if (same) continue;
    if (kind.is_active === false) {
      await api(apiBase, token, 'PUT', `/attachments/${kind.id}/restore`, {});
    }
    await api(apiBase, token, 'PUT', `/attachments/${kind.id}`, {
      attachment_type: type,
      ...wanted,
    });
  }

  for (const kind of extra) {
    await api(apiBase, token, 'DELETE', `/attachments/${kind.id}`);
  }

  console.log(
    `Виды вложений: оставлено ${byType.size}, скрыто дублей ${extra.length}`,
  );
}

/**
 * Приводит названия постов к тому виду, в котором их читает работник, и
 * дозаводит недостающий пост людей.
 *
 * Названия правятся по системному имени таблицы, а не по порядку в ответе:
 * порядок наливка не гарантирует, и переименование вслепую перевесило бы
 * названия между постами машин и людей.
 *
 * @returns {Promise<Array<object>>} список таблиц после правок
 */
async function renamePosts(apiBase, token, tables) {
  let renamed = 0;
  for (const table of tables) {
    const wanted = POST_NAMES[table.name];
    if (!wanted || table.display_name === wanted) continue;
    await api(apiBase, token, 'PUT', `/system-tables/${table.id}`, { display_name: wanted });
    table.display_name = wanted;
    renamed += 1;
  }

  let extra = tables.find((table) => table.name === EXTRA_POST.name);
  if (!extra) {
    await api(apiBase, token, 'POST', '/system-tables', EXTRA_POST);
    const fresh = unwrap(await api(apiBase, token, 'GET', '/system-tables')).map(
      (item) => item.table ?? item,
    );
    extra = fresh.find((table) => table.name === EXTRA_POST.name);
    if (!extra) throw new Error(`пост ${EXTRA_POST.name} не создался`);
    tables = fresh;
    console.log(`Заведён пост «${EXTRA_POST.display_name}»`);
  }

  console.log(`Постов переименовано: ${renamed}, всего постов: ${tables.length}`);
  return tables;
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

/**
 * Держит на стенде заявку, которая ждёт решения согласующего.
 *
 * Наливка распределяет заявки по стадиям на момент своего запуска, но время идёт:
 * срок вложений истекает, заявка уходит в «Завершено», и кадры руководства
 * согласующего начинают показывать плашку статуса вместо кнопок решения. Здесь
 * подаётся свежая заявка со сроком на месяц вперёд, где съёмочный согласующий
 * назначен обязательным. Проверка идёт по факту: пока у него есть хоть одна
 * живая заявка, ждущая голоса, ничего не подаётся.
 */
async function ensurePendingApproval(apiBase, token, accounts) {
  const users = unwrap(await api(apiBase, token, 'GET', '/users/all'));
  const list = Array.isArray(users) ? users : (users.users ?? users.items ?? []);
  const approver = list.find((user) => user.username === accounts.roles.approver.username);
  if (!approver) throw new Error(`согласующий ${accounts.roles.approver.username} не найден`);

  const session = unwrap(
    await api(apiBase, null, 'POST', '/login', {
      username: accounts.roles.approver.username,
      password: accounts.password,
    }),
  );
  const center = unwrap(await api(apiBase, session.token, 'GET', '/applications?filter_type=all'));
  const rows = Array.isArray(center) ? center : (center?.applications ?? center?.items ?? []);
  const waiting = rows.filter(
    (row) => row.confirmation === 'Согласование' && !ARCHIVED_STATUSES.has(row.status),
  );
  if (waiting.length > 0) {
    console.log(`Согласующий: заявок, ждущих решения, ${waiting.length} - подача не нужна`);
    return;
  }

  const applicant = unwrap(
    await api(apiBase, null, 'POST', '/login', {
      username: accounts.roles.user.username,
      password: accounts.password,
    }),
  );
  const profile = unwrap(await api(apiBase, applicant.token, 'GET', '/user-data'));
  // Виды вложений читаются администраторским маркером: заявителю их перечень
  // отдаётся только правом справочников, которого у него нет.
  const kinds = unwrap(await api(apiBase, token, 'GET', '/attachments/all')) ?? [];
  const people = kinds.find((kind) => kind.attachment_type === 'people' && kind.is_active !== false);
  if (!people) throw new Error('на стенде нет действующего вида вложения для работников');

  const citizenships = unwrap(await api(apiBase, applicant.token, 'GET', '/citizenships')) ?? [];
  const citizenship = citizenships.find((item) => item.is_default) ?? citizenships[0];
  const tables = (unwrap(await api(apiBase, applicant.token, 'GET', '/system-tables')) ?? [])
    .map((item) => item.table ?? item)
    .filter((table) => table.table_type === 'people' && table.is_active !== false);

  const from = new Date(profile.server_time ?? Date.parse('2026-08-12T00:00:00Z'));
  const to = new Date(from.getTime() + 30 * 24 * 60 * 60 * 1000);
  const day = (date) => date.toISOString().slice(0, 10);

  await api(apiBase, applicant.token, 'POST', '/applications/submit-complete-application', {
    message: 'Прошу оформить пропуск для проведения пусконаладочных работ на насосной станции.',
    organization: profile.organization ?? '',
    organization_id: profile.organization_id ?? null,
    responsible_person: `${profile.last_name ?? ''} ${profile.first_name ?? ''}`.trim() || profile.username,
    contact_phone: '+7 (999) 214-76-05',
    data_approval: true,
    required_users: [{ user_id: approver.id, required_approval: true }],
    attachments: [
      {
        attachment_type: 'people',
        attachment_name: `${people.name}_1`,
        attachment_display_name: `${people.display_name} №1`,
        unique_attachment_id: people.id,
        entry_date_from: day(from),
        entry_date_to: day(to),
        entry_time_from: '08:00',
        entry_time_to: '18:00',
        data: {
          employees: [
            {
              last_name: 'Александров',
              first_name: 'Леонид',
              middle_name: 'Леонидович',
              citizenship_id: citizenship?.id ?? 1,
              position: 'инженер-наладчик',
              passport_series_number: '4519 774310',
              target_tables: tables.slice(0, 1).map((table) => table.id),
            },
          ],
        },
      },
    ],
  });

  console.log('Согласующий: подана заявка, ждущая его решения');
}

/**
 * Фамилия, отличающаяся от заданной одной буквой. Меняется последняя гласная
 * «и» или «о» - результат читается как настоящая фамилия, а не как опечатка
 * с задвоенной буквой.
 */
function nearMissName(name) {
  const swaps = { и: 'е', о: 'а', е: 'и', а: 'о' };
  for (let i = name.length - 2; i > 1; i -= 1) {
    const replacement = swaps[name[i].toLowerCase()];
    if (replacement) return name.slice(0, i) + replacement + name.slice(i + 1);
  }
  return `${name.slice(0, -1)}н`;
}

/**
 * Держит заявку с неразобранной пометкой чёрного списка.
 *
 * Кадр окна «Всё равно пропустить?» снимается только на живой заявке с
 * непогашенной пометкой, а наливка такие заявки со временем доводит до решения.
 * Работник подаётся с фамилией, отличающейся от записи чёрного списка одной
 * буквой: точное совпадение система запретила бы, близкое - как раз и даёт
 * пометку, которую разбирает согласующий.
 */
async function ensureBlacklistFlag(apiBase, token, accounts) {
  // Спрашиваем от имени согласующего: администратор в этих заявках не участвует и
  // в его списке их нет, поэтому проверка по нему всегда считала бы, что заявок нет.
  const approverSession = unwrap(
    await api(apiBase, null, 'POST', '/login', {
      username: accounts.roles.approver.username,
      password: accounts.password,
    }),
  );
  const flagged = unwrap(await api(apiBase, approverSession.token, 'GET', '/applications?filter_type=all'));
  const rows = Array.isArray(flagged) ? flagged : (flagged?.applications ?? flagged?.items ?? []);
  const live = rows.filter(
    (row) => row.confirmation === 'Согласование' && !ARCHIVED_STATUSES.has(row.status) && row.blacklist_flags_count > 0,
  );
  if (live.length > 0) {
    console.log(`Чёрный список: заявок с неразобранной пометкой ${live.length} - подача не нужна`);
    return;
  }

  const blacklist = unwrap(await api(apiBase, token, 'GET', '/person-blacklist')) ?? [];
  const record = (Array.isArray(blacklist) ? blacklist : (blacklist.items ?? [])).find((item) => item.last_name);
  if (!record) {
    console.log('Чёрный список: записей о людях нет, заявка с пометкой не подаётся');
    return;
  }

  const users = unwrap(await api(apiBase, token, 'GET', '/users/all'));
  const list = Array.isArray(users) ? users : (users.users ?? users.items ?? []);
  const approver = list.find((user) => user.username === accounts.roles.approver.username);

  const applicant = unwrap(
    await api(apiBase, null, 'POST', '/login', {
      username: accounts.roles.user.username,
      password: accounts.password,
    }),
  );
  const profile = unwrap(await api(apiBase, applicant.token, 'GET', '/user-data'));
  const kinds = unwrap(await api(apiBase, token, 'GET', '/attachments/all')) ?? [];
  const people = kinds.find((kind) => kind.attachment_type === 'people' && kind.is_active !== false);
  const citizenships = unwrap(await api(apiBase, applicant.token, 'GET', '/citizenships')) ?? [];
  const citizenship = citizenships.find((item) => item.is_default) ?? citizenships[0];
  const tables = (unwrap(await api(apiBase, applicant.token, 'GET', '/system-tables')) ?? [])
    .map((item) => item.table ?? item)
    .filter((table) => table.table_type === 'people' && table.is_active !== false);

  // Похожая фамилия строится заменой одной гласной: «Сорокина» -> «Сорокена».
  // Совпадение перестаёт быть точным и становится близким, а сама фамилия
  // остаётся правдоподобной - её увидит заказчик на снимке.
  const nearName = nearMissName(record.last_name);
  const from = new Date(profile.server_time ?? Date.parse('2026-08-12T00:00:00Z'));
  const to = new Date(from.getTime() + 30 * 24 * 60 * 60 * 1000);
  const day = (date) => date.toISOString().slice(0, 10);

  await api(apiBase, applicant.token, 'POST', '/applications/submit-complete-application', {
    message: 'Прошу оформить пропуск подрядчику для замены осветительных приборов.',
    organization: profile.organization ?? '',
    organization_id: profile.organization_id ?? null,
    responsible_person: `${profile.last_name ?? ''} ${profile.first_name ?? ''}`.trim() || profile.username,
    contact_phone: '+7 (999) 214-76-05',
    data_approval: true,
    required_users: approver ? [{ user_id: approver.id, required_approval: true }] : undefined,
    attachments: [
      {
        attachment_type: 'people',
        attachment_name: `${people.name}_1`,
        attachment_display_name: `${people.display_name} №1`,
        unique_attachment_id: people.id,
        entry_date_from: day(from),
        entry_date_to: day(to),
        entry_time_from: '09:00',
        entry_time_to: '17:00',
        data: {
          employees: [
            {
              last_name: nearName,
              first_name: record.first_name,
              middle_name: record.middle_name,
              citizenship_id: citizenship?.id ?? 1,
              position: 'электромонтажник',
              passport_series_number: '4521 660418',
              target_tables: tables.slice(0, 1).map((table) => table.id),
            },
          ],
        },
      },
    ],
  });

  console.log(`Чёрный список: подана заявка с работником «${nearName}», похожим на запись «${record.last_name}»`);
}

/**
 * Собирает бланк пропуска на транспорт - образец печатной формы, которую бюро
 * выдаёт на посту.
 *
 * Без загруженного шаблона редактор бланка показывает одну строку «Загрузите
 * шаблон .xlsx», и рассказать по такому кадру про привязку полей к ячейкам
 * нельзя. Настоящий бланк заказчика сюда класть незачем: он у каждого свой, а
 * для рисунка нужна узнаваемая форма с шапкой и таблицей строк.
 */
async function makeBlank() {
  const ExcelJS = requireFromFrontend('exceljs');
  const book = new ExcelJS.Workbook();
  const sheet = book.addWorksheet('Пропуск');

  // Первая колонка держит номер строки в таблице, поэтому подписи шапки
  // занимают две колонки merge-ом: в узкой A они обрезались бы многоточием, и
  // на рисунке в руководстве бланк выглядел бы недоделанным.
  sheet.columns = [
    { width: 6 },
    { width: 26 },
    { width: 20 },
    { width: 18 },
    { width: 22 },
  ];

  sheet.mergeCells('A1:E1');
  sheet.getCell('A1').value = 'ЗАЯВКА НА ПРОПУСК ТРАНСПОРТНОГО СРЕДСТВА';
  sheet.getCell('A1').font = { bold: true, size: 14 };
  sheet.getCell('A1').alignment = { horizontal: 'center' };

  for (const [row, label] of [
    [3, 'Организация:'],
    [4, 'Ответственный:'],
    [5, 'Телефон:'],
    [6, 'Срок действия:'],
  ]) {
    sheet.mergeCells(`A${row}:B${row}`);
    sheet.getCell(`A${row}`).value = label;
    sheet.mergeCells(`C${row}:E${row}`);
  }

  const head = ['№', 'Марка', 'Гос. номер', 'Водитель', 'Место разгрузки'];
  head.forEach((title, index) => {
    const cell = sheet.getRow(8).getCell(index + 1);
    cell.value = title;
    cell.font = { bold: true };
    cell.alignment = { horizontal: 'center' };
  });

  for (let row = 8; row <= 14; row += 1) {
    for (let column = 1; column <= 5; column += 1) {
      sheet.getRow(row).getCell(column).border = {
        top: { style: 'thin' },
        left: { style: 'thin' },
        bottom: { style: 'thin' },
        right: { style: 'thin' },
      };
    }
  }

  sheet.mergeCells('A16:C16');
  sheet.getCell('A16').value = 'Начальник бюро пропусков ____________________';

  const buffer = await book.xlsx.writeBuffer();
  return Buffer.from(buffer);
}

/**
 * Загружает образец бланка в вид вложения «Автомобили» и привязывает поля к
 * ячейкам. Привязки набираются по подписям, а не по заранее выписанным путям:
 * состав полей задаётся системой и меняется вместе с ней.
 */
async function ensureBlankTemplate(apiBase, token) {
  const kinds = unwrap(await api(apiBase, token, 'GET', '/attachments/all')) ?? [];
  const cars = kinds.find((kind) => kind.attachment_type === 'cars' && kind.is_active !== false);
  if (!cars) throw new Error('на стенде нет действующего вида вложения для машин');

  // Пока шаблона нет, метод отвечает отказом «Шаблон не настроен» - это не сбой.
  const current = unwrap(
    await api(apiBase, token, 'GET', `/attachments/${cars.id}/template`).catch(() => null),
  );
  if (!current?.file_path) {
    const form = new FormData();
    form.append('file', new Blob([await makeBlank()]), 'Пропуск на транспорт.xlsx');
    form.append('list_start_row', '9');
    form.append('list_end_row', '14');
    const response = await fetch(`${apiBase}/attachments/${cars.id}/template`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
      body: form,
    });
    if (!response.ok) {
      throw new Error(`загрузка бланка -> ${response.status}: ${(await response.text()).slice(0, 300)}`);
    }
  }

  const loaded = unwrap(
    await api(apiBase, token, 'GET', `/attachments/${cars.id}/template`).catch(() => null),
  );
  if ((loaded?.mappings ?? []).length > 0) {
    console.log(`Бланк вида «${cars.display_name}»: уже настроен, привязок ${loaded.mappings.length}`);
    return;
  }

  // Поля приходят группами («Заявка», «Автомобиль (список)» и прочие), а
  // признак строки списка стоит у самого поля - от него зависит, берётся
  // значение один раз или по строке на машину.
  const groups = unwrap(await api(apiBase, token, 'GET', `/attachments/${cars.id}/template-fields`)) ?? [];
  const flat = groups.flatMap((group) => group.fields ?? []);
  const byPath = new Map(flat.map((field) => [field.path, field]));

  const wanted = [
    ['C3', 'application.organization'],
    ['C4', 'application.sender.full_name'],
    ['C5', 'application.contact_phone'],
    ['A9', 'car.row_number'],
    ['B9', 'car.mark_name'],
    ['C9', 'car.car_number'],
  ];
  const mappings = [];
  for (const [cell, fieldPath] of wanted) {
    const field = byPath.get(fieldPath);
    if (!field) throw new Error(`поле ${fieldPath} исчезло из состава полей бланка`);
    mappings.push({ cell_ref: cell, field_path: fieldPath, is_list_field: Boolean(field.is_list) });
  }
  await api(apiBase, token, 'PUT', `/attachments/${cars.id}/template/mappings`, { mappings });

  console.log(`Бланк вида «${cars.display_name}»: загружен, привязок ${mappings.length}`);
}

/**
 * Заводит обращения в разделе «Обратная связь».
 *
 * Наливка обращений не создаёт, и раздел на стенде пуст: снимать нечего, а
 * описывать разбор обращения по пустому экрану нельзя. Одно обращение
 * закрывается ответом - иначе вкладка «Решено» и поле ответа заявителю тоже
 * остались бы без кадра.
 *
 * Обращение пишет сам работник, поэтому оно и заводится входом под его учётной
 * записью: отправка от имени администратора положила бы в список не ту фамилию.
 */
async function seedFeedback(apiBase, token, accounts) {
  const existing = unwrap(await api(apiBase, token, 'GET', '/feedback/all')) ?? [];
  const known = new Set(existing.map((item) => item.message));
  let added = 0;

  for (const item of FEEDBACK) {
    if (known.has(item.message)) continue;
    const session = unwrap(
      await api(apiBase, null, 'POST', '/login', {
        username: accounts.roles[item.role].username,
        password: accounts.password,
      }),
    );
    const created = unwrap(await api(apiBase, session.token, 'POST', '/feedback', {
      message: item.message,
    }));
    if (item.answer) {
      await api(apiBase, token, 'PUT', `/feedback/${created.id}/status`, {
        status: 'Решено',
        comment: item.answer,
      });
    }
    added += 1;
  }

  console.log(`Обратная связь: было обращений ${existing.length}, добавлено ${added}`);
}

main().catch((error) => {
  console.error(`Донастройка стенда не удалась: ${error.message}`);
  process.exit(1);
});
