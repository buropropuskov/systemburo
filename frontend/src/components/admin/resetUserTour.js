import { resetOnboardingForUser } from '@/api/onboarding';
import { TOURS } from '@/components/onboarding/tours';
import { useUiStore } from '@/stores/ui';
import { useDeletionsStore } from '@/stores/deletions';

/**
 * Сброс прохождения обучения пользователю. Пустой ключ = все туры сразу.
 *
 * @param {object} user
 * @param {string} tourKey ключ тура из реестра либо '' для сброса всех
 */
export async function resetUserTour(user, tourKey) {
  if (tourKey === null || tourKey === undefined) return;
  const tourTitle = TOURS.find((t) => t.key === tourKey)?.title || '';
  const scope = tourTitle ? `тур «${tourTitle}»` : 'все туры';
  const ok = await useUiStore().confirm({
    title: 'Сбросить обучение?',
    message: `Пользователю «${user.username}» будет сброшен(ы) ${scope}: обучение запустится у него снова при следующем входе.`,
    confirmText: 'Сбросить',
    cancelText: 'Отмена',
    danger: false,
  });
  if (!ok) return;
  try {
    await resetOnboardingForUser(user.username, tourKey || undefined);
    useDeletionsStore().notify({
      prefix: tourTitle ? `Тур «${tourTitle}» сброшен для ` : 'Все туры сброшены для ',
      bold: user.username,
      suffix: ' — обучение запустится снова при входе',
    });
  } catch (error) {
    useDeletionsStore().notify({ prefix: 'Не удалось сбросить обучение: ', bold: error?.message || 'ошибка', type: 'error' });
  }
}
