/**
 * Где человек остановился в туре.
 *
 * Обучение длинное - у заявителя под шестьдесят шагов, это минут семь подряд.
 * Раньше любой перерыв стоил всего пройденного: перезагрузил страницу на сороковом
 * шаге - и тур начинался с первого. Теперь позиция переживает и перезагрузку, и
 * закрытие вкладки, и человек продолжает с той же главы.
 *
 * Храним в localStorage и отдельно по пользователю: за одним компьютером в бюро
 * работают посменно, и чужая позиция сбивала бы с толку. Запись протухает через
 * две недели - вернувшись через месяц, обучение честнее начать заново.
 */

const PREFIX = 'ob:progress';
const TTL_MS = 14 * 24 * 60 * 60 * 1000;

const keyOf = (userId, tour) => `${PREFIX}:${userId || 'anon'}:${tour}`;

/**
 * @param {string|number|null} userId
 * @param {string} tour ключ тура
 * @returns {{ index: number, interrupted: boolean }} шаг, с которого продолжать
 *   (0 - сначала), и был ли тур на экране в момент ухода со страницы
 */
export function readProgress(userId, tour) {
  const empty = { index: 0, interrupted: false };
  try {
    const raw = localStorage.getItem(keyOf(userId, tour));
    if (!raw) return empty;
    const { index, at, onScreen } = JSON.parse(raw);
    if (!Number.isInteger(index) || index <= 0) return empty;
    if (!at || Date.now() - at > TTL_MS) {
      localStorage.removeItem(keyOf(userId, tour));
      return empty;
    }
    return { index, interrupted: Boolean(onScreen) };
  } catch {
    return empty;
  }
}

/**
 * @param {string|number|null} userId
 * @param {string} tour
 * @param {number} index шаг, на котором человек сейчас
 * @param {boolean} [onScreen] тур сейчас открыт. Отличает случайную перезагрузку
 *   от осознанной паузы: после F5 обучение поднимается само, после «Продолжить
 *   позже» - ждёт, пока человек сам вернётся через меню.
 */
export function saveProgress(userId, tour, index, onScreen = false) {
  try {
    if (!Number.isInteger(index) || index <= 0) {
      clearProgress(userId, tour);
      return;
    }
    localStorage.setItem(keyOf(userId, tour), JSON.stringify({ index, at: Date.now(), onScreen }));
  } catch {
    // приватный режим браузера или переполненное хранилище - тур просто начнётся
    // сначала, ломать из-за этого обучение незачем
  }
}

/**
 * @param {string|number|null} userId
 * @param {string} tour
 */
export function clearProgress(userId, tour) {
  try {
    localStorage.removeItem(keyOf(userId, tour));
  } catch {
    // см. saveProgress
  }
}

/**
 * Обвязка над хранилищем для стора: подставляет текущего пользователя и прячет
 * форму записи. Стору остаются четыре понятных вызова.
 *
 * @param {() => string|number|null} getUserId
 */
export function createProgressTracker(getUserId) {
  return {
    /** @returns {number} шаг, с которого продолжать */
    resumeIndex: (tour) => readProgress(getUserId(), tour).index,
    /** @returns {boolean} есть ли что продолжать - по этому меню зовёт «Продолжить» */
    has: (tour) => readProgress(getUserId(), tour).index > 0,
    /** @returns {boolean} тур остался на экране при уходе со страницы */
    interrupted: (tour) => readProgress(getUserId(), tour).interrupted,
    save: (tour, index, onScreen) => saveProgress(getUserId(), tour, index, onScreen),
    clear: (tour) => clearProgress(getUserId(), tour),
  };
}
