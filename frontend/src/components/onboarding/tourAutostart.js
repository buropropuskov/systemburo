/**
 * Кому и когда обучение поднимается само.
 *
 * Два случая. Первый - первый вход: человек ещё не видел своего тура, и мы
 * показываем его на «Обзоре и новостях», где начинаются все сценарии. Второй -
 * обучение, прерванное перезагрузкой страницы: его никто не закрывал, поэтому
 * возвращаем с того же шага. Тур, закрытый руками («Продолжить позже», Esc,
 * «Пропустить»), сам не всплывает - за ним человек приходит через меню.
 *
 * @param {object} store стор онбординга
 * @param {() => string} getPath текущий путь роутера
 * @returns {() => Promise<void>}
 */
export function createAutostart(store, getPath) {
  return async function maybeAutostart() {
    if (store.isActive || getPath() !== '/news' || !store.canShowTour) return;
    // Права, тип пользователя и роль в согласовании гейтят туры и приезжают
    // своими запросами - без ожидания выбор шёл бы по неполному списку.
    const pending = [store.ensureGatingContext()];
    if (!store.statusLoaded) pending.push(store.loadStatus());
    await Promise.all(pending);
    // Перепроверяем после ожидания: статус мог не загрузиться, человек - уйти со
    // страницы или запустить тур сам, а гейт согласия - доехать ответом (#1567).
    if (!store.statusLoaded || store.isActive || getPath() !== '/news') return;
    if (!store.canShowTour) return;

    const cut = store.availableTours.find((t) => store.interruptedTour(t.key));
    if (cut) {
      store.start({ tour: cut.key, manual: true });
      return;
    }
    const tour = store.pickAutostartTour();
    if (tour) store.start({ tour: tour.key, manual: false });
  };
}
