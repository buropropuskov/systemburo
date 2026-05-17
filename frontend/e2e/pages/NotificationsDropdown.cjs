class NotificationsDropdown {
  constructor(page) {
    this.page = page;
    this.bell = page.locator('.user__notifications').first();
    this.badge = this.bell.locator('.notification-badge, span').first();
    this.panel = page.locator('.notifications').first();
    this.title = this.panel.locator('.notifications__title');
    this.unreadCount = this.panel.locator('.notifications__unread-count');
    this.emptyText = this.panel.locator('.notifications__empty-text');
    this.clearAllButton = this.panel.locator('.notifications__clear-btn');
    this.items = this.panel.locator('.notification-item, .notifications__item');
  }

  async open() {
    await this.bell.click();
    await this.panel.waitFor({ state: 'visible' });
  }

  async close() {
    if (await this.panel.isVisible().catch(() => false)) {
      await this.bell.click();
      await this.panel.waitFor({ state: 'hidden' }).catch(() => {});
    }
  }

  async isOpen() {
    return this.panel.isVisible().catch(() => false);
  }
}

module.exports = { NotificationsDropdown };
