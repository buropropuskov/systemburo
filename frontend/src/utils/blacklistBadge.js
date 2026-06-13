/**
 * Презентация сводного бейджа "N похожи на ЧС" в списках заявок (#481, срез 6c).
 * Поле blacklist_flags_count - агрегат непереопределённых флагов ЧС из GET /applications,
 * тот же предикат, что и гейт согласования. Используется в ApplicationsCenter и UserApplications.
 */

/** Число непереопределённых флагов ЧС у заявки (0 при отсутствии/невалидном поле). */
export function blacklistFlagCount(application) {
  return Number(application?.blacklist_flags_count) || 0;
}

/**
 * Метка бейджа со склонением: 1/21/31 - "похоже", 11/12/2/5 - "похожи"
 * (форма-1 при n % 10 === 1 и n % 100 !== 11, иначе множественная).
 */
export function blacklistFlagLabel(application) {
  const n = blacklistFlagCount(application);
  const singular = n % 10 === 1 && n % 100 !== 11;
  return `${n} ${singular ? 'похоже' : 'похожи'} на ЧС`;
}

/** Подсказка (title) бейджа - что делать с похожими элементами. */
export const BLACKLIST_FLAG_TITLE =
  'В заявке есть элементы, похожие на чёрный список. Подтвердите пропуск в деталях заявки.';
