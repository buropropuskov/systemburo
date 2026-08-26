const { test, expect } = require('@playwright/test');
const { loginAsUI } = require('../helpers/auth');
const { createSupplementFixture, destroySupplementFixture } = require('../helpers/supplement');
const { CreateApplicationPage } = require('../pages/CreateApplicationPage');
const { ApplicationCenterPage } = require('../pages/ApplicationCenterPage');
const { CabinetPage } = require('../pages/CabinetPage');
const { ApplicationDetailModal } = require('../pages/ApplicationDetailModal');
const { SupplementModal } = require('../pages/SupplementModal');
const { PeopleTablePage } = require('../pages/PeopleTablePage');

/**
 * Полный цикл дополнения поданной заявки (#1685).
 *
 * Смысл фичи в том, что добавка к уже принятой заявке идёт отдельным кругом
 * согласования и принятия, НЕ отзывая пропуска, выданные по первому кругу. Проверяется
 * это по таблице проходной: между подачей дополнения и его принятием первая строка
 * обязана остаться допущенной, а вторая - не появиться.
 *
 * Сценарий линейный: каждый шаг опирается на состояние предыдущего, поэтому serial.
 * Роли разведены по трём созданным учёткам, каждый тест входит своей - контекст
 * Playwright у теста всё равно свой.
 */

// Сценарий гоняет живой стек через браузер: подача заявки заполняет форму целиком,
// а решения по кругам ждут перезагрузки карточки. Дефолтных 30 секунд не хватает.
const STEP_TIMEOUT = 120_000;

const APPLICANT_PHONE = '9990000001';
const SUPPLEMENT_COMMENT = 'E2E: подрядчик прислал монтажника сверх списка';

/** дд.мм.гггг - формат полей срока действия. */
function formatDate(date) {
  const dd = String(date.getDate()).padStart(2, '0');
  const mm = String(date.getMonth() + 1).padStart(2, '0');
  return `${dd}.${mm}.${date.getFullYear()}`;
}

/**
 * Окно пропуска - сегодня и завтра. Таблица поста показывает только действующие
 * сегодня пропуска, а прогон длится около полутора минут и может перешагнуть полночь:
 * на однодневной заявке это дало бы красный при исправной фиче.
 */
function passWindow() {
  const from = new Date();
  const to = new Date();
  to.setDate(from.getDate() + 1);
  return { from: formatDate(from), to: formatDate(to) };
}

let fixture = null;
let applicationNumber = '';
let firstEmployee = null;
let secondEmployee = null;

function employeeFor(role) {
  const suffix = `${process.pid}${Math.floor(Math.random() * 10000)}`;
  return {
    lastName: `${role}${suffix}`,
    // Паспорт уникален не для красоты: таблица проходной схлопывает строки по хешу
    // паспорта, и на двух одинаковых сотрудник дополнения вытеснил бы первого.
    passport: `4501 ${suffix.slice(-6).padStart(6, '0')}`,
  };
}

test.describe.serial('Дополнение поданной заявки (#1685)', () => {
  test.beforeAll(async ({ request }) => {
    fixture = await createSupplementFixture(request);
    firstEmployee = employeeFor('Первый');
    secondEmployee = employeeFor('Второй');
  });

  test.afterAll(async ({ request }) => {
    await destroySupplementFixture(request, fixture);
  });

  test('заявитель подаёт заявку с одним сотрудником', async ({ page }) => {
    test.setTimeout(STEP_TIMEOUT);
    const createPage = new CreateApplicationPage(page);

    await loginAsUI(page, fixture.applicant.username, fixture.applicant.password);
    await createPage.goto();
    await createPage.expectLoaded();

    await createPage.addAttachment(fixture.attachmentTitle);
    await createPage.phoneInput.fill(APPLICANT_PHONE);
    const { from, to } = passWindow();
    await createPage.setDateRange(from, to);
    await createPage.setTimeRange('00:00', '23:59');

    await createPage.employeeForm.addEmployee({
      lastName: firstEmployee.lastName,
      passport: firstEmployee.passport,
      passageTable: fixture.tableDisplayName,
    });

    // Согласующий подставляется из организации автора, вручную его не выбирают.
    await expect(createPage.recipientChips.filter({ hasText: 'Согласующий' })).toHaveCount(1);

    applicationNumber = await createPage.submitAndGetNumber();
    expect(applicationNumber).toMatch(/\d/);
  });

  test('согласующий согласовывает заявку', async ({ page }) => {
    test.setTimeout(STEP_TIMEOUT);
    const center = new ApplicationCenterPage(page);
    const detail = new ApplicationDetailModal(page);

    await loginAsUI(page, fixture.approver.username, fixture.approver.password);
    await center.openApplication(applicationNumber);
    await detail.expectOpen();
    await expect(detail.title).toContainText(applicationNumber);

    await detail.approveApplication();
    await expect(detail.voteBadge.first()).toContainText('Вы согласовали');
  });

  test('принимающий принимает заявку в работу', async ({ page }) => {
    test.setTimeout(STEP_TIMEOUT);
    const center = new ApplicationCenterPage(page);
    const detail = new ApplicationDetailModal(page);

    await loginAsUI(page, fixture.acceptor.username, fixture.acceptor.password);
    await center.openApplication(applicationNumber);
    await detail.expectOpen();

    await detail.takeToWork();
    await expect(detail.inWorkBadge).toContainText('В работе');
  });

  test('сотрудник из заявки допущен на проходную', async ({ page }) => {
    test.setTimeout(STEP_TIMEOUT);
    const kpp = new PeopleTablePage(page);

    await loginAsUI(page, fixture.acceptor.username, fixture.acceptor.password);
    await kpp.goto(fixture.tableName);

    await kpp.expectAdmitted([firstEmployee.lastName]);
  });

  test('заявитель добавляет второго сотрудника через «Дополнить»', async ({ page }) => {
    test.setTimeout(STEP_TIMEOUT);
    const cabinet = new CabinetPage(page);
    const detail = new ApplicationDetailModal(page);
    const supplement = new SupplementModal(page);

    await loginAsUI(page, fixture.applicant.username, fixture.applicant.password);
    await cabinet.openApplication(applicationNumber);
    await detail.expectOpen();

    await expect(detail.supplementButton).toBeVisible();
    await detail.supplementButton.click();
    await supplement.expectOpen();
    // Срок действия принадлежит вложению и в дополнении не редактируется.
    await expect(supplement.period).not.toHaveValue('');

    await supplement.selectAttachment(fixture.attachmentTitle);
    await supplement.employeeForm.addEmployee({
      lastName: secondEmployee.lastName,
      passport: secondEmployee.passport,
      passageTable: fixture.tableDisplayName,
    });
    await supplement.submit(SUPPLEMENT_COMMENT);

    await expect(detail.supplementRoundBadge).toContainText('Дополнение №1 на согласовании');
    await expect(detail.supplementPanel).toContainText(SUPPLEMENT_COMMENT);
  });

  test('в составе вложения новая строка помечена, а прежняя нет', async ({ page }) => {
    test.setTimeout(STEP_TIMEOUT);
    const cabinet = new CabinetPage(page);
    const detail = new ApplicationDetailModal(page);

    await loginAsUI(page, fixture.applicant.username, fixture.applicant.password);
    await cabinet.openApplication(applicationNumber);
    await detail.expectOpen();
    await detail.openAttachment();

    await expect(detail.elementRows).toHaveCount(2);
    await expect(detail.elementRowBadge(secondEmployee.lastName)).toContainText('На согласовании');
    // У прежней строки метки нет вовсе - она допущена первым кругом.
    await expect(detail.elementRowBadge(firstEmployee.lastName)).toHaveCount(0);
  });

  test('бейдж дополнения встал рядом со статусом заявки, не подменив его', async ({ page }) => {
    test.setTimeout(STEP_TIMEOUT);
    const center = new ApplicationCenterPage(page);
    const detail = new ApplicationDetailModal(page);

    await loginAsUI(page, fixture.acceptor.username, fixture.acceptor.password);
    await center.openApplication(applicationNumber);
    await detail.expectOpen();
    await detail.expectActionsReady();

    await expect(detail.supplementRoundBadge).toContainText('Дополнение №1 на согласовании');
    // Статус самой заявки не трогают: от него зависит допуск уже выданных пропусков.
    await expect(detail.inWorkBadge).toContainText('В работе');
  });

  test('пока дополнение не принято, на проходной только прежний сотрудник', async ({ page }) => {
    test.setTimeout(STEP_TIMEOUT);
    const kpp = new PeopleTablePage(page);

    await loginAsUI(page, fixture.acceptor.username, fixture.acceptor.password);
    await kpp.goto(fixture.tableName);

    // Ядро требования: добавка ждёт своего круга, а выданный пропуск не отзывается.
    await kpp.expectAdmitted([firstEmployee.lastName]);
    await expect(kpp.row(secondEmployee.lastName)).toHaveCount(0);
  });

  test('согласующий согласовывает дополнение', async ({ page }) => {
    test.setTimeout(STEP_TIMEOUT);
    const center = new ApplicationCenterPage(page);
    const detail = new ApplicationDetailModal(page);

    await loginAsUI(page, fixture.approver.username, fixture.approver.password);
    await center.openApplication(applicationNumber);
    await detail.expectOpen();

    // Подпись с номером раунда из панели убрали (#1097): номер несёт бейдж в шапке
    // заявки, а здесь проверяем то, ради чего тест и написан - согласующий видит панель
    // решения по дополнению и может голосовать.
    await expect(detail.supplementActions).toBeVisible();
    await detail.approveSupplement();
    await expect(detail.supplementMyVote).toContainText('вы согласовали');
  });

  test('принимающий принимает дополнение', async ({ page }) => {
    test.setTimeout(STEP_TIMEOUT);
    const center = new ApplicationCenterPage(page);
    const detail = new ApplicationDetailModal(page);

    await loginAsUI(page, fixture.acceptor.username, fixture.acceptor.password);
    await center.openApplication(applicationNumber);
    await detail.expectOpen();

    await expect(detail.supplementRoundBadge).toContainText('Дополнение №1 ждёт принятия');
    await detail.acceptSupplement();
    await expect(detail.inWorkBadge).toContainText('В работе');
  });

  test('после принятия дополнения на проходной оба сотрудника', async ({ page }) => {
    test.setTimeout(STEP_TIMEOUT);
    const kpp = new PeopleTablePage(page);

    await loginAsUI(page, fixture.acceptor.username, fixture.acceptor.password);
    await kpp.goto(fixture.tableName);

    await kpp.expectAdmitted([firstEmployee.lastName, secondEmployee.lastName]);
  });
});
