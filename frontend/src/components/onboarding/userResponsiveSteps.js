import { onboardingSteps } from './onboardingSteps';

/** The phone opens downloads from the card, not from the hidden list action. */
export function buildUserSteps(context = {}) {
  if (!context.isPhoneViewport) return onboardingSteps;
  return onboardingSteps.map(step => step.id !== 'cabinet-download' ? step : {
    ...step,
    element: null,
    description: 'На телефоне кнопка скачивания находится в шапке карточки заявки. Она открывает окно «Актуальные бланки»: заполненные бланки можно взять по одному или все разом. Сначала откройте заявку — дальше покажем её карточку и действия в шапке.',
  });
}
