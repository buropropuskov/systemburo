/**
 * Форматирование телефонного номера в формат +7 (XXX) XXX-XX-XX
 *
 * @param {string} phone - Номер телефона (любой формат)
 * @returns {{ raw: string, formatted: string }}
 */
export function formatPhoneNumberImmediately(phone) {
    if (!phone) return { raw: '', formatted: phone };

    const raw = phone.replace(/\D/g, '');
    let formattedNumber = raw;

    if (formattedNumber.length === 11 && formattedNumber.startsWith('8')) {
        formattedNumber = '7' + formattedNumber.substring(1);
    }

    if (formattedNumber.length === 10) {
        formattedNumber = '7' + formattedNumber;
    }

    if (formattedNumber.length === 11 && formattedNumber.startsWith('7')) {
        formattedNumber = formattedNumber.replace(
            /(\d{1})(\d{3})(\d{3})(\d{2})(\d{2})/,
            '+$1 ($2) $3 $4-$5'
        );
    }

    return { raw, formatted: formattedNumber };
}

/**
 * Форматирование введённого телефонного номера
 *
 * @param {string} value - Текущее значение поля телефона
 * @returns {{ raw: string, formatted: string }}
 */
export function formatPhoneNumber(value) {
    if (!value) return { raw: '', formatted: value };

    const raw = value.replace(/\D/g, '');
    let formattedNumber = raw;

    if (formattedNumber.length === 11 && formattedNumber.startsWith('8')) {
        formattedNumber = '7' + formattedNumber.substring(1);
    }

    if (formattedNumber.length === 10) {
        formattedNumber = '7' + formattedNumber;
    }

    if (formattedNumber.length === 11 && formattedNumber.startsWith('7')) {
        formattedNumber = formattedNumber.replace(
            /(\d{1})(\d{3})(\d{3})(\d{2})(\d{2})/,
            '+$1 ($2) $3 $4-$5'
        );
    }

    return { raw, formatted: formattedNumber };
}

/**
 * Снимает форматирование телефона, возвращает сырые цифры
 *
 * @param {string} rawPhoneNumber - Сырой номер без форматирования
 * @returns {string}
 */
export function clearPhoneFormat(rawPhoneNumber) {
    return rawPhoneNumber || '';
}
