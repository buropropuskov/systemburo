/**
 * Перехват ЧТЕНИЯ на время демонстрации.
 *
 * Онбординг-тур должен быть один для всех: человеку, у которого ещё нет ни одной
 * заявки, шаги про её карточку показывают не пустоту и не картинку, а настоящий
 * интерфейс с примерными данными. Подменять их в компонентах нельзя - пришлось бы
 * тащить «а если демо» через список, карточку и все её панели. Поэтому подмена
 * живёт на входе в сеть: интерфейс рисует то, что «пришло с сервера», и ничего не
 * знает о демонстрации.
 *
 * Перехватчик решает сам, отвечать ли за путь и метод. Записи по чужим адресам он
 * не трогает, а по своим - гасит успехом: примерная заявка живёт только на экране,
 * и отметка о прочтении для неё уходила бы на сервер, где такой заявки нет (403 и
 * тост «недостаточно прав» посреди обучения).
 *
 * Живёт ровно столько, сколько тур: он снимает перехватчик, и следующий запрос
 * идёт на живой бэкенд.
 */

let interceptor = null;

/**
 * @param {((path: string, method: string) => object|null) | null} fn отдаёт тело
 *   ответа для пути и метода или null, если подменять не нужно
 */
export function setReadInterceptor(fn) {
  interceptor = typeof fn === 'function' ? fn : null;
}

/**
 * @param {string} path путь запроса
 * @param {{ method?: string }} options
 * @returns {Response|null} готовый ответ вместо похода в сеть
 */
export function interceptRead(path, options = {}) {
  if (!interceptor) return null;
  const method = (options.method || 'GET').toUpperCase();
  const body = interceptor(path, method);
  if (!body) return null;
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}
