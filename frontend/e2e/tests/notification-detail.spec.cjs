const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth.cjs');
const { loginAsSuperAdmin, apiPost, apiDelete } = require('../helpers/permissions.cjs');
const { NotificationsDropdown } = require('../pages/NotificationsDropdown.cjs');
const { NotificationDetailModal } = require('../pages/NotificationDetailModal.cjs');

// Клик по карточке уведомления больше не ведёт сразу в заявку - он раскрывает
// подробности в NotificationDetailModal (#1748 S6), переход к заявке делает кнопка
// действия внутри неё. Сценарий "клик по уведомлению -> сразу Центр заявок" в проекте
// раньше отдельным e2e не покрывался (notifications.spec.cjs проверяет только
// открытие/закрытие панели) - переписывать нечего, весь путь через модалку новый.

function userIdFromToken(token) {
  const payload = JSON.parse(Buffer.from(token.split('.')[1], 'base64').toString());
  return payload.user_id;
}

function uniqueTitle(label) {
  return `${label} ${Date.now()}_${Math.random().toString(36).slice(2, 6)}`;
}

// Заведомо положительный псевдо-уникальный id заявки для data - реальная заявка
// не нужна (проверяем только текст поля и URL перехода, не содержимое Центра).
function fakeApplicationId() {
  return 100000 + (Date.now() % 900000);
}

async function badgeCount(dropdown) {
  if ((await dropdown.badge.count()) === 0) return 0;
  const text = (await dropdown.badge.innerText()).trim();
  return parseInt(text, 10) || 0;
}

test.describe('Модалка подробностей уведомления', () => {
  let createdIds;

  test.beforeEach(() => {
    createdIds = [];
  });

  test.afterEach(async ({ request }) => {
    if (createdIds.length === 0) return;
    const token = await loginAsSuperAdmin(request);
    for (const id of createdIds) {
      await apiDelete(request, token, `/notifications/${id}`).catch(() => {});
    }
  });

  // Уведомление создаём напрямую через admin-эндпоинт (по образцу notifications-api.spec.cjs) -
  // так тест сам управляет заголовком/текстом/data, не завися от того, какое реальное
  // событие (новость, заявка...) сейчас доступно для триггера.
  async function createNotification(request, { title, message, type, data }) {
    const token = await loginAsSuperAdmin(request);
    const userId = userIdFromToken(token);
    const created = await apiPost(request, token, '/notifications', {
      user_id: userId,
      title,
      message,
      type: type || 'info',
      data: data ? JSON.stringify(data) : undefined,
    });
    createdIds.push(created.id);
    return created;
  }

  test('клик по карточке открывает модалку с полным текстом и помечает уведомление прочитанным', async ({ page, request }) => {
    const title = uniqueTitle('Подробности');
    const message = 'Полный текст уведомления для проверки модалки подробностей.';
    await createNotification(request, { title, message, type: 'info' });

    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    const modal = new NotificationDetailModal(page);

    await dropdown.open();
    await expect(dropdown.item(title)).toHaveClass(/notification-item--unread/);

    await dropdown.openDetail(title);
    await modal.waitForOpen();
    await expect(modal.title).toHaveText(title);
    await expect(modal.message).toHaveText(message);

    // Карточка остаётся в DOM под модалкой - класс непрочитанного должен уже спасть.
    await expect(dropdown.item(title)).not.toHaveClass(/notification-item--unread/);
  });

  test('модалка показывает поля из data - номер заявки', async ({ page, request }) => {
    const title = uniqueTitle('С полями');
    const applicationNumber = 'E2E-TEST-000123';
    await createNotification(request, {
      title,
      message: 'Заявка ожидает согласования.',
      type: 'application_status_changed',
      data: { application_id: fakeApplicationId(), application_number: applicationNumber },
    });

    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    const modal = new NotificationDetailModal(page);

    await dropdown.open();
    await dropdown.openDetail(title);
    await modal.waitForOpen();

    await expect(modal.fields).toBeVisible();
    await expect(modal.fieldValue('Заявка')).toHaveText(applicationNumber);
  });

  test('кнопка действия ведёт в заявку по application_id из data', async ({ page, request }) => {
    const title = uniqueTitle('С переходом');
    const applicationId = fakeApplicationId();
    await createNotification(request, {
      title,
      message: 'Заявка передана на согласование.',
      type: 'application_status_changed',
      data: { application_id: applicationId, application_number: 'E2E-TEST-000456' },
    });

    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    const modal = new NotificationDetailModal(page);

    await dropdown.open();
    await dropdown.openDetail(title);
    await modal.waitForOpen();

    await expect(modal.actionButton).toBeVisible();
    await modal.actionButton.click();

    await expect(page).toHaveURL(new RegExp(`/center\\?open=${applicationId}$`));
  });

  test('уведомление без заявки открывает модалку без кнопки действия', async ({ page, request }) => {
    const title = uniqueTitle('Без заявки');
    const message = 'Пароль вашей учётной записи был изменён.';
    await createNotification(request, { title, message, type: 'password_changed' });

    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    const modal = new NotificationDetailModal(page);

    await dropdown.open();
    await dropdown.openDetail(title);
    await modal.waitForOpen();

    await expect(modal.message).toHaveText(message);
    await expect(modal.actionButton).toHaveCount(0);
  });

  test('"Вернуть в непрочитанные" возвращает карточку в непрочитанные и поднимает счётчик в шапке', async ({ page, request }) => {
    const title = uniqueTitle('Непрочитанное');
    await createNotification(request, { title, message: 'Проверка возврата в непрочитанные.', type: 'info' });

    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    const modal = new NotificationDetailModal(page);

    await dropdown.open();
    // Панель открывается раньше, чем резолвится fetchNotifications - дожидаемся
    // появления самой карточки: unreadCount пересчитывается из того же массива,
    // так что видимая карточка = гарантия, что счётчик уже актуален.
    await expect(dropdown.item(title)).toBeVisible();
    const before = await badgeCount(dropdown);
    expect(before).toBeGreaterThan(0);

    await dropdown.openDetail(title);
    await modal.waitForOpen();
    const afterRead = await badgeCount(dropdown);
    expect(afterRead).toBe(before - 1);

    await modal.unreadButton.click();
    await modal.waitForClosed();

    await expect(dropdown.item(title)).toHaveClass(/notification-item--unread/);
    const afterUnread = await badgeCount(dropdown);
    expect(afterUnread).toBe(before);
  });

  test('модалка закрывается по Escape', async ({ page, request }) => {
    const title = uniqueTitle('Escape');
    await createNotification(request, { title, message: 'Закрытие по Escape.', type: 'info' });

    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    const modal = new NotificationDetailModal(page);

    await dropdown.open();
    await dropdown.openDetail(title);
    await modal.waitForOpen();

    await modal.closeByEscape();
    await expect(modal.dialog).toBeHidden();
  });

  test('модалка закрывается по клику в затемнение', async ({ page, request }) => {
    const title = uniqueTitle('Оверлей');
    await createNotification(request, { title, message: 'Закрытие по клику в затемнение.', type: 'info' });

    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    const modal = new NotificationDetailModal(page);

    await dropdown.open();
    await dropdown.openDetail(title);
    await modal.waitForOpen();

    await modal.closeByOverlay();
    await expect(modal.dialog).toBeHidden();
  });

  test('"Удалить" в модалке удаляет уведомление из списка', async ({ page, request }) => {
    const title = uniqueTitle('Удаление');
    const created = await createNotification(request, { title, message: 'Удаление из модалки.', type: 'info' });

    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    const modal = new NotificationDetailModal(page);

    await dropdown.open();
    await dropdown.openDetail(title);
    await modal.waitForOpen();

    await modal.deleteButton.click();
    await modal.waitForClosed();
    await expect(dropdown.item(title)).toHaveCount(0);

    // Удалено самой модалкой - afterEach не должен пытаться удалить повторно.
    createdIds = createdIds.filter((id) => id !== created.id);
  });
});
