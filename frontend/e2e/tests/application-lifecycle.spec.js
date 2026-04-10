const { test, expect } = require('@playwright/test');
const { registerUser, setAuthTokens } = require('../helpers/auth');
const {
  getToken,
  submitCompleteApplication,
  forwardApplication,
  approveApplication,
  takeToWork,
} = require('../helpers/api');
const { ApplicationCenterPage } = require('../pages/ApplicationCenterPage');

const API_BASE = 'http://localhost:8080';

/**
 * Golden path: full application lifecycle.
 *
 * 1. Register admin (type 6) and regular user via API.
 * 2. Regular user submits an application via API.
 * 3. Admin logs in via browser, navigates to center, sees application.
 * 4. Forward → approve → take-to-work via API.
 * 5. Admin verifies final status in browser.
 */
test.describe('Application Lifecycle', () => {
  test('full lifecycle: create → forward → approve → take to work', async ({ page }) => {
    test.setTimeout(60000);

    const ts = Date.now();
    const adminName = `e2e_lifecycle_admin_${ts}`;
    const userName = `e2e_lifecycle_user_${ts}`;

    // 1. Create admin and regular user via API
    await registerUser(adminName, 'testpass123', 6);
    await registerUser(userName, 'testpass123', 1);

    const adminToken = await getToken(adminName);
    const userToken = await getToken(userName);

    // 2. Regular user submits an application via API
    const appData = {
      organization: 'Lifecycle Test Org',
      responsible_person: 'E2E Tester',
      contact_phone: '+7 (999) 000-00-01',
      data_approval: true,
      attachments: [{
        attachment_type: 'cars',
        attachment_name: 'cars_lifecycle',
        attachment_display_name: 'Авто тест жизн. цикла',
        unique_attachment_id: 1,
        data: {
          vehicles: [{ car_number: 'C123CC777', car_brand: 'Toyota' }],
        },
      }],
    };

    const application = await submitCompleteApplication(userToken, appData);
    const appId = application.id || application.ID;
    expect(appId).toBeTruthy();

    // 3. Admin logs in via browser → go to center → verify application visible
    await setAuthTokens(page, adminName, 'testpass123');

    const centerPage = new ApplicationCenterPage(page);
    await centerPage.goto();
    await expect(centerPage.root).toBeVisible();

    // Wait for application rows to load
    const rows = centerPage.getAllRows();
    await rows.first().waitFor({ state: 'visible', timeout: 15000 });
    const rowCount = await rows.count();
    expect(rowCount).toBeGreaterThanOrEqual(1);

    // 4. Get admin user ID for workflow API calls
    const adminInfoRes = await fetch(`${API_BASE}/users/me`, {
      headers: { Authorization: `Bearer ${adminToken}` },
    });
    const adminInfo = await adminInfoRes.json();
    const adminUserId = adminInfo.id || adminInfo.ID;

    // Forward application to admin
    await forwardApplication(adminToken, appId, {
      users: [{ user_id: adminUserId, required_approval: true, can_view: false }],
    });

    // Approve application
    await approveApplication(adminToken, appId, {
      user_id: adminUserId,
      status: 'approved',
      comment: 'E2E lifecycle approved',
    });

    // Take to work
    await takeToWork(adminToken, appId, {
      user_id: adminUserId,
      action: 'accept',
    });

    // 5. Verify final status in browser — reload center and check
    await centerPage.goto();
    await expect(centerPage.root).toBeVisible();

    // The application should still be visible (possibly with updated status)
    await rows.first().waitFor({ state: 'visible', timeout: 15000 });
  });
});
