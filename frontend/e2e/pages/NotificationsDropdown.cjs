class NotificationsDropdown {
  constructor(page) {
    this.page = page;
    // data-testid устойчивее, чем класс контейнера (TheHeader.vue - ob-header-notifications).
    this.bell = page.getByTestId('ob-header-notifications');
    // Счётчик непрочитанных в самой шапке (виден без открытия панели) - .notifications__badge,
    // рисуется только при unreadCount > 0.
    this.badge = this.bell.locator('.notifications__badge');
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

  // Карточка по заголовку - фильтр по тексту от уже скоуплённого от панели `items`,
  // не от page. Заголовок обычно уникален в тесте (генерируется с меткой времени),
  // поэтому hasText не цепляет чужие карточки.
  item(title) {
    return this.items.filter({ hasText: title });
  }

  // Клик по карточке (#1748 S6): раскрывает подробности в NotificationDetailModal
  // и помечает уведомление прочитанным - переход к заявке теперь делает кнопка
  // действия внутри модалки, не сам клик по карточке. Кликаем по заголовку внутри
  // карточки, а не по всей строке - на строке справа висит отдельная кнопка
  // удаления (`.notification-item__delete`, останавливает всплытие), задевать её
  // мимо цели нельзя.
  async openDetail(title) {
    await this.item(title).locator('.notification-item__title').click();
  }
}

module.exports = { NotificationsDropdown };
