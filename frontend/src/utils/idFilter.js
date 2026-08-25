/**
 * Набор id для клиентской фильтрации по мультивыбору (#1398).
 *
 * Пустой выбор возвращается как `null`, а не как пустой Set: вызывающая сторона
 * должна отличать «фильтр выключен» от «выбрано, но ничего не совпало» - иначе
 * страница без выбранных значений покажет ноль строк.
 *
 * Ключи строковые: дропдаун отдаёт id справочника как есть, а строки таблиц
 * приходят другим запросом, и совпадение типов не гарантировано - раньше это
 * держалось на нестрогом `==` в каждом предикате.
 *
 * @param {Array<number|string>|null|undefined} ids значения из мультивыбора
 * @returns {Set<string>|null} набор ключей либо null, если фильтровать не нужно
 */
export function idFilterSet(ids) {
  if (!Array.isArray(ids) || ids.length === 0) return null;
  const keys = new Set();
  ids.forEach((id) => {
    if (id === null || id === undefined || id === '') return;
    keys.add(String(id));
  });
  return keys.size ? keys : null;
}

export default idFilterSet;
