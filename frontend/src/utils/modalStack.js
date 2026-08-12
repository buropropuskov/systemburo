/**
 * Стопка открытых окон - чтобы Escape закрывал только верхнее.
 *
 * Каждое окно вешает свой обработчик `keydown` на document и на Escape эмитит close.
 * Пока окно одно, это работает; открытых окон два - и один Escape закрывает оба разом
 * (карточка участника поверх списка получателей схлопывалась вместе со списком).
 *
 * Учёт по владельцам, как в `bodyScrollLock`: ключ - сам инстанс компонента. Верхним
 * считается окно с наибольшим слоем, а при равных слоях - открытое последним.
 */
const stack = new Map();
let sequence = 0;

/**
 * @param {object} owner уникальный ключ владельца - обычно инстанс компонента (`this`)
 * @param {boolean} open открыто ли окно
 * @param {number} [zIndex] слой окна; при равных слоях верхним будет открытое последним
 */
export function setModalOpen(owner, open, zIndex = 0) {
  if (!owner) return;
  if (open) {
    // Повторный вызов от того же окна (watch срабатывает дважды) порядок не двигает:
    // иначе окно-родитель перепрыгнуло бы вперёд открытого поверх него.
    if (!stack.has(owner)) stack.set(owner, { zIndex, order: ++sequence });
    else stack.get(owner).zIndex = zIndex;
    return;
  }
  stack.delete(owner);
}

/** Снять окно со стопки - вызывать в `beforeUnmount`. */
export function releaseModal(owner) {
  stack.delete(owner);
}

/**
 * @param {object} owner
 * @returns {boolean} это окно верхнее в стопке (либо стопка о нём не знает - тогда
 *   поведение прежнее: закрываем, а не игнорируем нажатие)
 */
export function isTopModal(owner) {
  const self = stack.get(owner);
  if (!self) return true;
  for (const [other, info] of stack) {
    if (other === owner) continue;
    if (info.zIndex > self.zIndex) return false;
    if (info.zIndex === self.zIndex && info.order > self.order) return false;
  }
  return true;
}

/**
 * Escape, на который слой уже ответил.
 *
 * Одной стопки мало: слушатели висят на document у каждого слоя, вызываются в порядке
 * подписки, и слой, обработавший нажатие, снимается со стопки не мгновенно - следующий
 * слушатель в том же нажатии успевает счесть себя верхним. Так один Escape закрывал и
 * список получателей, и панель заявки под ним (поймано руками на стенде, в jsdom
 * порядок другой и тест этого не видел). Пометка на самом событии от порядка не зависит.
 */
const handledEscapes = new WeakSet();

/** @param {KeyboardEvent} event @returns {boolean} на это нажатие уже ответил другой слой */
export function isEscapeHandled(event) {
  return !!event && handledEscapes.has(event);
}

/** Отметить, что нажатие обработано этим слоем. @param {KeyboardEvent} event */
export function markEscapeHandled(event) {
  if (event) handledEscapes.add(event);
}

/** Только для тестов: сбросить состояние между кейсами. */
export function resetModalStack() {
  stack.clear();
  sequence = 0;
}
