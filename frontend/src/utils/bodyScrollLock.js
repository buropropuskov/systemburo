/**
 * Блокировка прокрутки фона под открытым окном.
 *
 * Окна на странице живут стопкой (карточка Т/С -> место разгрузки -> таблица), и каждое
 * ставило `document.body.style.overflow` само. Пока окно одно, это работает; как только
 * рядом размонтируется сосед со своим `beforeUnmount -> overflow = ''`, блокировка
 * снимается под всё ещё открытым окном, и фон начинает прокручиваться (#1097 S4).
 *
 * Учёт по владельцам, а не счётчиком: повторный `lock` от того же окна (watch срабатывает
 * дважды) не разъезжается с одним `release`.
 *
 * `overflow: hidden` ставится и на `<body>`, и на `<html>` (#1097 w8): фактический
 * скроллящийся элемент документа - `document.scrollingElement`, и им оказывается
 * `<html>`, а не `<body>` (замерено на живой странице - Центр заявок со списком выше
 * вьюпорта). Пропагация overflow тела на viewport, на которую полагался старый код,
 * в браузере не сработала - `window.scrollTo`/тач-скролл фона продолжали двигать
 * `documentElement.scrollTop` при одном лишь `body.style.overflow='hidden'`. Лочим
 * оба узла - тогда блокировка держит независимо от того, кто у документа реально
 * скроллится.
 */
const owners = new Set();

function apply() {
  if (typeof document === 'undefined') return;
  const value = owners.size ? 'hidden' : '';
  document.body.style.overflow = value;
  document.documentElement.style.overflow = value;
}

/**
 * @param {object} owner уникальный ключ владельца - обычно сам инстанс компонента (`this`)
 * @param {boolean} locked нужна ли блокировка этому владельцу
 */
export function setBodyScrollLock(owner, locked) {
  if (!owner) return;
  if (locked) owners.add(owner);
  else owners.delete(owner);
  apply();
}

/** Снять блокировку владельца - вызывать в `beforeUnmount`. */
export function releaseBodyScrollLock(owner) {
  setBodyScrollLock(owner, false);
}

/** Только для тестов: сбросить состояние между кейсами. */
export function resetBodyScrollLock() {
  owners.clear();
  apply();
}
