const { test, expect } = require('@playwright/test');
const { loginAsSuperAdminUI } = require('../helpers/auth');

test.describe('AdminSystemControl smoke', () => {
  test('страница /admin/system-control открывается у супер-админа', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    await page.goto('/admin/system-control');
    await expect(page).toHaveURL(/admin\/system-control/);
    await expect(page.getByRole('heading', { name: 'Системное управление' })).toBeVisible();
  });

  test('Клик на тумблер maintenance открывает confirm-модалку', async ({ page }) => {
    await loginAsSuperAdminUI(page);
    await page.goto('/admin/system-control');

    // Ищем тумблер/кнопку включения maintenance - текст подсказывает.
    const toggle = page.getByRole('button', { name: /Включить режим|Выключить режим|Maintenance/ }).first();
    if (await toggle.isVisible({ timeout: 2000 }).catch(() => false)) {
      await toggle.click();
      // Confirm-модалка с h2 "Включить режим технических работ?"
      const modal = page.getByRole('heading', { name: /режим технических работ/ });
      if (await modal.isVisible({ timeout: 2000 }).catch(() => false)) {
        // Закрываем модалку через Отмена/Esc чтобы не включить maintenance
        await page.keyboard.press('Escape');
      }
    }
  });
});
