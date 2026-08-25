const { expect } = require('@playwright/test');

/**
 * POM карточки заявки (ApplicationDetail.vue). Отдельного роута у неё нет - это
 * модалка, которую монтируют Центр заявок (mode=center) и личный кабинет (mode=user).
 * Разметка одна и та же, поэтому POM один; различается лишь набор доступных действий:
 * кнопки голосования и статус заявки рисуются только в центре, «Дополнить» - только
 * в кабинете. Открывают карточку ApplicationCenterPage/CabinetPage.
 */
class ApplicationDetailModal {
  constructor(page) {
    this.page = page;
    this.root = page.locator('.application-detail');
    this.title = page.locator('.detail-title');
    this.closeButton = page.locator('.close-detail-btn');

    // Действия по самой заявке (только Центр).
    this.approveButton = page.getByTestId('app-detail-button-approve');
    this.takeToWorkButton = page.getByTestId('app-detail-button-take-to-work');
    this.voteBadge = page.locator('.vote-status-badge');
    this.inWorkBadge = page.locator('.status-in-work-badge');
    this.approvedBadge = page.locator('.status-approved-badge');

    // Дополнение (#1685).
    this.supplementButton = page.getByTestId('app-detail-button-supplement');
    this.supplementRoundBadge = page.getByTestId('app-detail-supplement-round-badge');
    this.supplementPanel = page.getByTestId('supplement-panel');
    this.supplementActions = page.getByTestId('supplement-actions');
    this.supplementActionsLabel = this.supplementActions.locator('.supplement-actions__label');
    this.supplementApproveButton = page.getByTestId('supplement-button-approve');
    this.supplementAcceptButton = page.getByTestId('supplement-button-accept');
    this.supplementMyVote = page.getByTestId('supplement-my-vote');
    // Решение по раунду подтверждается ConfirmationModal - у неё свои кнопки.
    this.confirmButton = page.getByTestId('confirmation-confirm');

    // Состав вложения.
    this.attachmentItems = page.locator('.attachment-item');
    this.elementRows = page.getByTestId('attachment-element-row');
  }

  async expectOpen() {
    await expect(this.root).toBeVisible();
  }

  /** Кнопки действий появляются после догрузки ролей и голосов - до этого висит лоадер. */
  async expectActionsReady() {
    await expect(this.page.locator('.actions-ready-loader')).toHaveCount(0);
  }

  async openAttachment(index = 0) {
    await this.attachmentItems.nth(index).click();
    await expect(this.elementRows.first()).toBeVisible();
  }

  elementRow(keyText) {
    return this.elementRows.filter({ hasText: keyText });
  }

  /** Бейдж метки дополнения на строке состава («На согласовании», «Доп. №N»). */
  elementRowBadge(keyText) {
    return this.elementRow(keyText).getByTestId('attachment-supplement-badge');
  }

  async approveApplication() {
    await expect(this.approveButton).toBeEnabled();
    await this.approveButton.click();
    await expect(this.voteBadge.first()).toBeVisible();
  }

  async takeToWork() {
    await expect(this.takeToWorkButton).toBeEnabled();
    await this.takeToWorkButton.click();
    await expect(this.inWorkBadge).toBeVisible();
  }

  async approveSupplement() {
    await expect(this.supplementApproveButton).toBeVisible();
    await this.supplementApproveButton.click();
    await this.confirmButton.click();
    await expect(this.supplementMyVote).toBeVisible();
  }

  async acceptSupplement() {
    await expect(this.supplementAcceptButton).toBeVisible();
    await this.supplementAcceptButton.click();
    await this.confirmButton.click();
    // Раунд закрылся - второй бейдж в шапке снимается.
    await expect(this.supplementRoundBadge).toBeHidden();
  }
}

module.exports = { ApplicationDetailModal };
