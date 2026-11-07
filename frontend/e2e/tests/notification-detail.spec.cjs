const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth.cjs');
const { loginAsSuperAdmin, apiGet, apiPost, apiDelete } = require('../helpers/permissions.cjs');
const { NotificationsDropdown } = require('../pages/NotificationsDropdown.cjs');
const { NotificationDetailModal } = require('../pages/NotificationDetailModal.cjs');
const { ApplicationDetailModal } = require('../pages/ApplicationDetailModal.cjs');

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

async function badgeCount(dropdown) {
  if ((await dropdown.badge.count()) === 0) return 0;
  const text = (await dropdown.badge.innerText()).trim();
  return parseInt(text, 10) || 0;
}

test.describe('Модалка подробностей уведомления', () => {
  let createdIds;
  let applicationIds;
  let templateIds;

  test.beforeEach(() => {
    createdIds = [];
    applicationIds = [];
    templateIds = [];
  });

  test.afterEach(async ({ request }) => {
    const token = await loginAsSuperAdmin(request);
    for (const id of applicationIds) {
      await apiPost(request, token, `/applications/${id}/withdraw`).catch(() => {});
    }
    for (const id of createdIds) {
      await apiDelete(request, token, `/notifications/${id}`).catch(() => {});
    }
    for (const id of templateIds) {
      await apiDelete(request, token, `/attachments/${id}`).catch(() => {});
    }
  });

  // Manual payloads remain opaque: caller text/Data cannot establish provenance.
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
    expect(created.title).toBe('Есть обновление');
    expect(created.message).toBe('Есть обновление');
    expect(created.data).toBeNull();
    expect(created).not.toHaveProperty('projection_ref');
    return created;
  }

  // A real submission uses the seeded organization, its own API-created template
  // (the CI seed does not include demo templates), and a normal durable intent.
  async function createApplicationNotification(request) {
    const token = await loginAsSuperAdmin(request);
    const organizations = await apiGet(request, token, '/organizations');
    const organization = organizations.find(row => row.is_active !== false);
    expect(organization, 'seeded active organization').toBeTruthy();
    const template = await apiPost(request, token, '/attachments', {
      attachment_type: 'cars',
      name: `e2e_notif_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
      display_name: 'E2E notification vehicles',
      title: 'E2E notification vehicles',
    });
    templateIds.push(template.id);
    const from = new Date();
    from.setUTCDate(from.getUTCDate() + 1);
    const to = new Date(from);
    to.setUTCDate(to.getUTCDate() + 1);
    const { version, hash } = await apiGet(request, token, '/consents/gate');
    const application = await apiPost(request, token, '/applications/submit-complete-application', {
      organization_id: organization.id, organization: organization.name,
      responsible_person: 'E2E notification applicant', contact_phone: '+79001234567',
      data_approval: true,
      document_revision: { version, hash },
      message: 'Private application comment must not become a notification preview',
      attachments: [{
        attachment_type: 'cars', attachment_name: template.name,
        attachment_display_name: template.display_name || template.title || template.name,
        unique_attachment_id: template.id,
        entry_date_from: from.toISOString().slice(0, 10), entry_date_to: to.toISOString().slice(0, 10),
        entry_time_from: '08:00', entry_time_to: '18:00',
        data: { vehicles: [{ car_number: 'A' + String(Date.now() % 1000).padStart(3, '0') + 'AA777', car_brand: 'E2E' }] },
      }],
    });
    expect(application.application_id).toBeGreaterThan(0);
    applicationIds.push(application.application_id);
    let created;
    await expect.poll(async () => {
      const rows = await apiGet(request, token, '/notifications?limit=100&offset=0');
      created = rows.find(row => row.type === 'application_created'
        && JSON.parse(row.data || '{}').application_id === application.application_id);
      return Boolean(created);
    }, { timeout: 10000, message: 'committed submission materializes its typed notification' }).toBe(true);
    createdIds.push(created.id);
    expect(created).not.toHaveProperty('projection_ref');
    expect(JSON.parse(created.data).application_number).toBe(application.application_number);
    return { created, application };
  }

  async function openNotificationDetail(page, request, dropdown, modal, id) {
    const read = page.waitForResponse(response => response.request().method() === 'PUT'
      && new URL(response.url()).pathname.endsWith('/notifications/' + id + '/read'));
    await dropdown.itemById(id).locator('.notification-item__title').click();
    const response = await read;
    expect(response.ok()).toBe(true);
    await modal.waitForOpen();
    // The UI only uses PUT headers; its displayed text comes from the list GET.
    // Check persistence independently rather than waiting on browser Response.body
    // (the local Chromium trace never finishes that resource). The API spec
    // separately validates the PUT JSON contract using APIRequestContext.
    const token = await loginAsSuperAdmin(request);
    const rows = await apiGet(request, token, '/notifications');
    const persisted = rows.find(row => row.id === id);
    expect(persisted, `own notification ${id} remains persisted`).toBeTruthy();
    expect(persisted.is_read).toBe(true);
  }

  test('клик по manual карточке открывает generic текст и помечает уведомление прочитанным', async ({ page, request }) => {
    const title = uniqueTitle('Подробности');
    const message = 'Полный текст уведомления для проверки модалки подробностей.';
    const created = await createNotification(request, { title, message, type: 'info' });

    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    const modal = new NotificationDetailModal(page);

    await dropdown.open();
    await expect(dropdown.itemById(created.id)).toHaveClass(/notification-item--unread/);

    await openNotificationDetail(page, request, dropdown, modal, created.id);
    await modal.waitForOpen();
    await expect(modal.title).toHaveText('Есть обновление');
    await expect(modal.message).toHaveText('Есть обновление');

    // Карточка остаётся в DOM под модалкой - класс непрочитанного должен уже спасть.
    await expect(dropdown.itemById(created.id)).not.toHaveClass(/notification-item--unread/);
  });

  test('модалка показывает номер и отправителя реальной поданной заявки', async ({ page, request }) => {
    const { created, application } = await createApplicationNotification(request);
    const applicationNumber = application.application_number;

    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    const modal = new NotificationDetailModal(page);

    await dropdown.open();
    await openNotificationDetail(page, request, dropdown, modal, created.id);
    await modal.waitForOpen();

    await expect(modal.fields).toBeVisible();
    await expect(modal.fieldValue('Заявка')).toHaveText(applicationNumber);
    const data = JSON.parse(created.data);
    expect(data.sender_name).toBeTruthy();
    await expect(modal.fieldValue('Отправитель')).toHaveText(data.sender_name);
    await expect(modal.message).toContainText(applicationNumber);
    await expect(modal.message).not.toContainText('Private application comment');
  });

  test('кнопка действия ведёт в доступную реальную заявку', async ({ page, request }) => {
    const { created, application } = await createApplicationNotification(request);
    const applicationId = application.application_id;

    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    const modal = new NotificationDetailModal(page);

    await dropdown.open();
    await openNotificationDetail(page, request, dropdown, modal, created.id);
    await modal.waitForOpen();

    await expect(modal.actionButton).toBeVisible();
    const detailsLoaded = page.waitForResponse(response => response.request().method() === 'GET'
      && new URL(response.url()).pathname.endsWith(`/applications/${applicationId}/details`));
    await modal.actionButton.click();

    expect((await detailsLoaded).ok()).toBe(true);
    const applicationModal = new ApplicationDetailModal(page);
    await expect(applicationModal.root).toBeVisible();
    await expect(applicationModal.title).toContainText(application.application_number);
    await expect(modal.overlay).toBeHidden();
    // Center consumes the exact-ID deep link after locating the application;
    // ?open is transient and must not reopen the card on a later page refresh.
    await expect(page).toHaveURL(/\/center$/);
  });

  test('manual data не создаёт ссылку на заявку или подробные поля', async ({ page, request }) => {
    const title = uniqueTitle('Без заявки');
    const message = 'Пароль вашей учётной записи был изменён.';
    const created = await createNotification(request, { title, message, type: 'password_changed',
      data: { application_id: 123456, application_number: 'Private fabricated number',
        projection_ref: { v: 1, type: 'password_changed', application_id: 123456 } },
    });

    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    const modal = new NotificationDetailModal(page);

    await dropdown.open();
    await openNotificationDetail(page, request, dropdown, modal, created.id);
    await modal.waitForOpen();

    await expect(modal.message).toHaveText('Есть обновление');
    await expect(modal.actionButton).toHaveCount(0);
    await expect(modal.fields).toHaveCount(0);
  });

  test('"Вернуть в непрочитанные" возвращает карточку в непрочитанные и поднимает счётчик в шапке', async ({ page, request }) => {
    const title = uniqueTitle('Непрочитанное');
    const created = await createNotification(request, { title, message: 'Проверка возврата в непрочитанные.', type: 'info' });

    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    const modal = new NotificationDetailModal(page);

    await dropdown.open();
    // Панель открывается раньше, чем резолвится fetchNotifications - дожидаемся
    // появления самой карточки: unreadCount пересчитывается из того же массива,
    // так что видимая карточка = гарантия, что счётчик уже актуален.
    await expect(dropdown.itemById(created.id)).toBeVisible();
    const before = await badgeCount(dropdown);
    expect(before).toBeGreaterThan(0);

    await openNotificationDetail(page, request, dropdown, modal, created.id);
    await modal.waitForOpen();
    const afterRead = await badgeCount(dropdown);
    expect(afterRead).toBe(before - 1);

    await modal.unreadButton.click();
    await modal.waitForClosed();

    await expect(dropdown.itemById(created.id)).toHaveClass(/notification-item--unread/);
    const afterUnread = await badgeCount(dropdown);
    expect(afterUnread).toBe(before);
  });

  test('модалка закрывается по Escape', async ({ page, request }) => {
    const title = uniqueTitle('Escape');
    const created = await createNotification(request, { title, message: 'Закрытие по Escape.', type: 'info' });

    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    const modal = new NotificationDetailModal(page);

    await dropdown.open();
    await openNotificationDetail(page, request, dropdown, modal, created.id);
    await modal.waitForOpen();

    await modal.closeByEscape();
    await expect(modal.dialog).toBeHidden();
  });

  test('модалка закрывается по клику в затемнение', async ({ page, request }) => {
    const title = uniqueTitle('Оверлей');
    const created = await createNotification(request, { title, message: 'Закрытие по клику в затемнение.', type: 'info' });

    await loginAsSuperAdminUI(page);
    await page.goto('/news');

    const dropdown = new NotificationsDropdown(page);
    const modal = new NotificationDetailModal(page);

    await dropdown.open();
    await openNotificationDetail(page, request, dropdown, modal, created.id);
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
    await openNotificationDetail(page, request, dropdown, modal, created.id);
    await modal.waitForOpen();

    const deleted = page.waitForResponse(response => response.request().method() === 'DELETE'
      && new URL(response.url()).pathname.endsWith('/notifications/' + created.id));
    await modal.deleteButton.click();
    expect((await deleted).ok()).toBe(true);
    await modal.waitForClosed();
    await expect(dropdown.itemById(created.id)).toHaveCount(0);

    const token = await loginAsSuperAdmin(request);
    const remaining = await apiGet(request, token, '/notifications');
    expect(remaining.some(row => row.id === created.id)).toBe(false);

    // Удалено самой модалкой - afterEach не должен пытаться удалить повторно.
    createdIds = createdIds.filter((id) => id !== created.id);
  });
});
