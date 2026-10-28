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
 *
 * Вместе с позицией держим версию набора шагов. Тур переписывают: шаги меняются
 * местами, появляются новые - и старый индекс указывает уже не туда. Хуже того,
 * если шаг той версии жил на другой странице, продолжение с него роняло тур сразу
 * после запуска: сегмента для чужого пути нет, и обучение гасло молча (#2584).
 */

import { TOURS } from '@/components/onboarding/tours';

const PREFIX = 'ob:progress';
const TTL_MS = 14 * 24 * 60 * 60 * 1000;

const keyOf = (userId, tour) => `${PREFIX}:${userId || 'anon'}:${tour}`;

/** @param {string} tour @returns {number|null} версия набора шагов тура */
const versionOf = (tour) => TOURS.find((t) => t.key === tour)?.version ?? null;

/** Keep a semantic step across filtering; prefer its next remaining neighbour. */
export function resolveStepIndex(stepId, previousIds, steps, legacyIndex = 0) {
  if (!steps.length) return 0;
  const ids = steps.map(step => step.id);
  const exact = typeof stepId === 'string' ? ids.indexOf(stepId) : -1;
  if (exact >= 0) return exact;
  const previous = Array.isArray(previousIds) ? previousIds : [];
  const anchor = previous.indexOf(stepId);
  if (anchor >= 0) {
    for (let index = anchor + 1; index < previous.length; index += 1) {
      const next = ids.indexOf(previous[index]);
      if (next >= 0) return next;
    }
    for (let index = anchor - 1; index >= 0; index -= 1) {
      const next = ids.indexOf(previous[index]);
      if (next >= 0) return next;
    }
    return 0;
  }
  return Math.min(Math.max(0, Number.isInteger(legacyIndex) ? legacyIndex : 0), steps.length - 1);
}

/**
 * @param {string|number|null} userId
 * @param {string} tour ключ тура
 * @returns {{ index: number, interrupted: boolean }} шаг, с которого продолжать
 *   (0 - сначала), и был ли тур на экране в момент ухода со страницы
 */
export function readProgress(userId, tour, steps) {
  const empty = { index: 0, interrupted: false };
  try {
    const raw = localStorage.getItem(keyOf(userId, tour));
    if (!raw) return empty;
    const { index, at, onScreen, version, stepId, stepIds } = JSON.parse(raw);
    if (!Number.isInteger(index) || index <= 0) return empty;
    if (version !== versionOf(tour)) {
      localStorage.removeItem(keyOf(userId, tour));
      return empty;
    }
    if (!at || Date.now() - at > TTL_MS) {
      localStorage.removeItem(keyOf(userId, tour));
      return empty;
    }
    return { index: Array.isArray(steps) ? resolveStepIndex(stepId, stepIds, steps, index) : index,
      interrupted: Boolean(onScreen) };
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
export function saveProgress(userId, tour, index, onScreen = false, steps) {
  try {
    if (!Number.isInteger(index) || index <= 0) {
      clearProgress(userId, tour);
      return;
    }
    const identity = Array.isArray(steps) && typeof steps[index]?.id === 'string'
      ? { stepId: steps[index].id, stepIds: steps.map(step => step.id) } : {};
    localStorage.setItem(
      keyOf(userId, tour),
      JSON.stringify({ index, at: Date.now(), onScreen, version: versionOf(tour), ...identity }),
    );
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
    resumeIndex: (tour, steps) => readProgress(getUserId(), tour, steps).index,
    /** @returns {boolean} есть ли что продолжать - по этому меню зовёт «Продолжить» */
    has: (tour) => readProgress(getUserId(), tour).index > 0,
    /** @returns {boolean} тур остался на экране при уходе со страницы */
    interrupted: (tour) => readProgress(getUserId(), tour).interrupted,
    save: (tour, index, onScreen, steps) => saveProgress(getUserId(), tour, index, onScreen, steps),
    clear: (tour) => clearProgress(getUserId(), tour),
  };
}
