/**
 * Копирование текста в буфер обмена.
 *
 * navigator.clipboard живёт только в защищённом контексте: по http (стенд без TLS,
 * заход по IP) его в объекте navigator просто нет, и без запасного пути копирование
 * там молча не срабатывает. Запасной путь - скрытое textarea с execCommand: команда
 * устарела, но остаётся единственной, что работает без https.
 *
 * Текст уведомления остаётся за вызывающим - формулировки в списке заявок, в ленте
 * журнала и в модалке отправки разные.
 *
 * @param {string|number} value что копируем
 * @returns {Promise<boolean>} удалось ли скопировать
 */
export async function copyText(value) {
  if (value === null || value === undefined || value === '') return false;
  const text = String(value);

  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return true;
    }

    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.setAttribute('readonly', '');
    textarea.style.position = 'absolute';
    textarea.style.left = '-9999px';
    document.body.appendChild(textarea);
    textarea.select();
    const copied = document.execCommand('copy');
    document.body.removeChild(textarea);
    return copied;
  } catch {
    return false;
  }
}
