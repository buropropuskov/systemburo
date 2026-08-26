/**
 * POM модалки подробностей уведомления (NotificationDetailModal.vue, #1748 S6).
 * Самостоятельное окно (role="dialog", Teleport в body поверх NotificationsDropdown),
 * а не часть панели уведомлений - отдельный POM, не натягиваем один на оба view
 * (методология проекта: у разных окон разные контейнеры - разные POM).
 *
 * В компоненте нет data-testid: локаторы держатся на getByRole (кнопки с текстом/
 * aria-label фиксированы в разметке) и на CSS, скоуплённом от dialog, для
 * заголовка/текста/полей, у которых устойчивой роли нет.
 */
class NotificationDetailModal {
  constructor(page) {
    this.page = page;
    this.dialog = page.getByRole('dialog');
    this.overlay = page.locator('.notif-detail-overlay');
    this.title = this.dialog.locator('.notif-detail-dialog__title');
    this.message = this.dialog.locator('.notif-detail-dialog__message');
    this.fields = this.dialog.locator('.notif-detail-dialog__fields');
    this.closeButton = this.dialog.getByRole('button', { name: 'Закрыть' });
    this.actionButton = this.dialog.getByRole('button', { name: 'Открыть заявку' });
    this.unreadButton = this.dialog.getByRole('button', { name: 'В непрочитанные' });
    this.deleteButton = this.dialog.getByRole('button', { name: 'Удалить' });
  }

  async waitForOpen() {
    await this.dialog.waitFor({ state: 'visible' });
  }

  async waitForClosed() {
    await this.dialog.waitFor({ state: 'hidden' });
  }

  // Значение конкретного поля из dl/dt/dd (например "Заявка" -> номер заявки).
  // dt и dd - соседние узлы одного уровня, поэтому берём dd сразу за нужным dt.
  fieldValue(label) {
    return this.fields.locator('dt', { hasText: label }).locator('xpath=following-sibling::dd[1]');
  }

  async close() {
    await this.closeButton.click();
    await this.waitForClosed();
  }

  async closeByEscape() {
    await this.page.keyboard.press('Escape');
    await this.waitForClosed();
  }

  // Клик в затемнение вне диалога (не в сам диалог - у него @mousedown.stop).
  // Верхний левый угол оверлея гарантированно вне центрированного диалога
  // (max-width 480px) при обычных тестовых вьюпортах.
  async closeByOverlay() {
    await this.overlay.click({ position: { x: 10, y: 10 } });
    await this.waitForClosed();
  }
}

module.exports = { NotificationDetailModal };
