const {
  loginAsSuperAdmin,
  apiGet,
  apiPost,
  apiPut,
  apiDelete,
  e2eName,
  uniqSuffix,
  E2E_PREFIX,
} = require('./permissions');

const API_BASE = process.env.E2E_API_BASE_URL || '/api';

/** Пароль, который администратор задаёт работнику при заведении учётной записи. */
const INITIAL_PASSWORD = 'SuppE2E-pass-2718';

/**
 * Пароль, которым учётная запись пользуется дальше: работник задаёт его сам при
 * первом входе. Заданный администратором система считает временным и до смены
 * отвечает 403 на защищённые методы, поэтому фикстура проходит первый вход так
 * же, как живой человек.
 *
 * Значение обязано отличаться от начального: повторное использование прежнего
 * пароля система не примет.
 */
const PASSWORD = 'SuppE2E-own-3141';

/**
 * Версия пройденного тура заведомо выше любой реальной ONBOARDING_VERSION - иначе
 * автозапуск обучающего оверлея перехватывает клики (cmd/seed делает то же самое
 * для сид-юзеров, но созданные тестом учётки сид не видит).
 */
const ONBOARDING_DONE_VERSION = 1000;

/** Прогресс тура хранится по ключам, и непомеченный тур автозапустится (#1737). */
const ONBOARDING_TOURS = ['user', 'guard', 'approve', 'accept', 'admin'];

/**
 * Права, которых базовой роли не хватает для работы в Центре заявок. Ключ таблицы
 * КПП добавляется отдельно - он существует только после создания самой таблицы.
 *
 * `page.admin` в списке не по недосмотру: карточка заявки узнаёт принимающих из
 * `GET /application-approvers`, а он закрыт этим ключом и отвечает 403, который фронт
 * гасит молча (`silent403`). Без права список принимающих приезжает пустым, и кнопка
 * «Принять» не рисуется даже тому, кто в этой роли состоит.
 */
const CENTER_KEYS = [
  'page.center',
  'action.approve.application',
  'detail.open_application',
  'page.admin',
];

async function createUser(request, token, { username, organizationId, lastName }) {
  await apiPost(request, token, '/users', {
    username,
    password: INITIAL_PASSWORD,
    organization_id: organizationId,
    type_id: 1,
    last_name: lastName,
    first_name: 'E2E',
  });
  const users = await apiGet(request, token, '/users/all');
  const created = users.find((u) => u.username === username);
  if (!created) throw new Error(`user ${username} not found after create`);
  await changeInitialPassword(request, username);
  return { id: created.id, username, password: PASSWORD, lastName };
}

/**
 * Первый вход работника: меняет временный пароль на свой. Без этого шага система
 * отвечает 403 с кодом PASSWORD_CHANGE_REQUIRED на первый же защищённый запрос -
 * начиная с пометки туров пройденными.
 */
async function changeInitialPassword(request, username) {
  const res = await request.post(`${API_BASE}/login`, {
    data: { username, password: INITIAL_PASSWORD },
  });
  if (!res.ok()) throw new Error(`login ${username} failed: ${res.status()}`);
  const token = (await res.json()).data.token;
  await apiPut(request, token, '/users/me/password', {
    current_password: INITIAL_PASSWORD,
    new_password: PASSWORD,
  });
}

/** Туры помечаются пройденными от лица самого пользователя - /onboarding/complete self-эндпоинт. */
async function markOnboardingDone(request, user) {
  const res = await request.post(`${API_BASE}/login`, {
    data: { username: user.username, password: user.password },
  });
  if (!res.ok()) throw new Error(`login ${user.username} failed: ${res.status()}`);
  const token = (await res.json()).data.token;
  for (const tour of ONBOARDING_TOURS) {
    await apiPost(request, token, '/onboarding/complete', {
      tour,
      version: ONBOARDING_DONE_VERSION,
    });
  }
}

/**
 * Готовит изолированный стенд под сценарий дополнения заявки (#1685).
 *
 * Изоляция важнее краткости: организация, таблица КПП и все три учётки свои, потому
 * что назначение согласующего идёт через `PUT /organizations/:id/users`, а он ЗАМЕНЯЕТ
 * список - сделай это на общей «Бюро пропусков», и соседняя спека того же шарда
 * потеряет своих ответственных. Общими остаются только шаблон вложения (глобальный
 * справочник) и гражданство, и то создаётся лишь когда справочник пуст.
 *
 * Роли развожу по разным учёткам намеренно: если принимающий окажется ещё и
 * ответственным, ActionBar схлопывает два шага в одну кнопку «Согласовать и принять»
 * и раздельные круги согласования проверить уже нечем.
 */
async function createSupplementFixture(request) {
  const token = await loginAsSuperAdmin(request);
  const sfx = uniqSuffix();

  const org = await apiPost(request, token, '/organizations', {
    name: e2eName('supp_org'),
    type: 'Организация',
  });

  const tableName = `${E2E_PREFIX}supp_tbl_${sfx}`;
  const tableDisplayName = `КПП дополнения ${sfx}`;
  const table = await apiPost(request, token, '/system-tables', {
    name: tableName,
    display_name: tableDisplayName,
    table_type: 'people',
  });

  const attachmentTitle = `Сотрудники дополнения ${sfx}`;
  const attachment = await apiPost(request, token, '/attachments', {
    attachment_type: 'people',
    name: `${E2E_PREFIX}supp_att_${sfx}`,
    display_name: attachmentTitle,
    title: attachmentTitle,
  });

  // Гражданство обязательно в форме сотрудника и подставляется само (is_default либо
  // первое в списке). Своё заводим только на пустом справочнике - в живой БД чужой
  // is_default перебивать нечем и незачем.
  const citizenships = await apiGet(request, token, '/citizenships');
  const citizenship = citizenships.length
    ? null
    : await apiPost(request, token, '/citizenships', { name: 'Россия', is_default: true });

  const group = await apiPost(request, token, '/permission-groups', {
    name: e2eName('supp_grp'),
    keys: [...CENTER_KEYS, `table.${tableName}.view`],
  });

  const applicant = await createUser(request, token, {
    username: `${E2E_PREFIX}sapl_${sfx}`, organizationId: org.id, lastName: 'Заявитель',
  });
  const approver = await createUser(request, token, {
    username: `${E2E_PREFIX}sapr_${sfx}`, organizationId: org.id, lastName: 'Согласующий',
  });
  const acceptor = await createUser(request, token, {
    username: `${E2E_PREFIX}sacc_${sfx}`, organizationId: org.id, lastName: 'Принимающий',
  });

  for (const user of [approver, acceptor]) {
    await apiPost(request, token, `/users/${user.id}/permission-groups/${group.id}`);
  }
  for (const user of [applicant, approver, acceptor]) {
    await markOnboardingDone(request, user);
  }

  // Согласующий по заявке - это required_approval в организации автора: заявка,
  // поданная из формы, уходит ему сама, без ручного forward.
  await apiPut(request, token, `/organizations/${org.id}/users`, {
    users: [{ username: approver.username, is_primary: true, required_approval: true }],
  });

  // Принимающий - глобальная роль application_approvers, правами она не выдаётся.
  // Создание отвечает одним сообщением, без id, а строка переживает удаление самого
  // пользователя - поэтому id вычитываем списком, иначе убрать её в teardown нечем.
  await apiPost(request, token, '/application-approvers', { user_id: acceptor.id });
  const approverRows = await apiGet(request, token, '/application-approvers');
  const approverRow = approverRows.find((row) => row.user_id === acceptor.id);

  return {
    orgId: org.id,
    tableId: table.id,
    tableName,
    tableDisplayName,
    attachmentId: attachment.id,
    attachmentTitle,
    citizenshipId: citizenship ? citizenship.id : null,
    groupId: group.id,
    approverRowId: approverRow ? approverRow.id : null,
    applicant,
    approver,
    acceptor,
  };
}

/**
 * Убирает за собой всё, что создала фикстура. Best-effort: заявка держит FK на
 * пользователей и вложение, часть удалений вернёт отказ - валить на этом уже
 * прошедший сценарий незачем.
 */
async function destroySupplementFixture(request, fixture) {
  if (!fixture) return;
  const token = await loginAsSuperAdmin(request).catch(() => null);
  if (!token) return;

  const drop = (path) => apiDelete(request, token, path).catch(() => {});

  if (fixture.approverRowId) await drop(`/application-approvers/${fixture.approverRowId}`);
  for (const user of [fixture.applicant, fixture.approver, fixture.acceptor]) {
    if (user) await drop(`/users/${user.username}`);
  }
  if (fixture.groupId) await drop(`/permission-groups/${fixture.groupId}`);
  if (fixture.attachmentId) await drop(`/attachments/${fixture.attachmentId}`);
  if (fixture.tableId) await drop(`/system-tables/${fixture.tableId}`);
  if (fixture.citizenshipId) await drop(`/citizenships/${fixture.citizenshipId}`);
  if (fixture.orgId) await drop(`/organizations/${fixture.orgId}`);
}

module.exports = { createSupplementFixture, destroySupplementFixture, PASSWORD };
