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
 * не трогает. Ответчик выбирает чтение, локальный no-op или контролируемый отказ;
 * перехватчик не ходит в сеть и ограничивает ответ временем своей установки.
 * Центр допускает только точную отметку прочтения и отказывает прочим действиям;
 * прежний ответчик тура заявителя отдельно не переработан.
 *
 * Живёт ровно столько, сколько тур: он снимает перехватчик, и следующий запрос
 * идёт на живой бэкенд.
 */

let interceptor = null;
let generation = 0;

/**
 * @param {((path: string, method: string) => object|Response|null) | null} fn отдаёт тело
 *   ответа для пути и метода или null, если подменять не нужно
 */
export function setReadInterceptor(fn) {
  generation++;
  interceptor = typeof fn === 'function' ? fn : null;
}

/**
 * @param {string} path путь запроса
 * @param {{ method?: string }} options
 * @returns {Response|null} готовый ответ вместо похода в сеть
 */
export function interceptRead(path, options = {}) {
  if (!interceptor) return null;
  const lease = generation;
  const method = (options.method || 'GET').toUpperCase();
  const body = interceptor(path, method);
  if (!body) return null;
  const response = body instanceof Response ? body : new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
  for (const method of ['json', 'text', 'blob', 'arrayBuffer']) {
    const read = response[method].bind(response);
    response[method] = async () => {
      if (lease !== generation) throw new DOMException('Learning data expired', 'AbortError');
      const value = await read();
      if (lease !== generation) throw new DOMException('Learning data expired', 'AbortError');
      return value;
    };
  }
  return response;
}
