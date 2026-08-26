/**
 * @param {string} value
 * @param {string|null} allowedLetters
 * @returns {string}
 */
export function filterCyrillicLetters(value, allowedLetters) {
    if (allowedLetters) {
        const allowedChars = allowedLetters.split('')
        return value.split('').filter(char => allowedChars.includes(char)).join('')
    }
    return value.replace(/[^АВЕКМНОРСТУХ]/g, '')
}

/**
 * @param {string} value
 * @param {string|null} allowedLetters
 * @returns {string}
 */
export function filterLatinLetters(value, allowedLetters) {
    if (allowedLetters) {
        const allowedChars = allowedLetters.split('')
        return value.split('').filter(char => allowedChars.includes(char)).join('')
    }
    return value.replace(/[^A-Z]/g, '')
}

/**
 * @param {string} value
 * @param {string|null} allowedLetters
 * @returns {string}
 */
export function filterBothLetters(value, allowedLetters) {
    if (allowedLetters) {
        const allowedChars = allowedLetters.split('')
        return value.split('').filter(char => allowedChars.includes(char)).join('')
    }
    return value.replace(/[^A-ZА-Я]/g, '')
}

/**
 * @param {string} value
 * @param {string|null} allowedLetters
 * @returns {string}
 */
export function filterMixedCyrillic(value, allowedLetters) {
    const numericPart = value.replace(/\D/g, '')
    const letterPart = filterCyrillicLetters(value.replace(/[0-9]/g, ''), allowedLetters)
    return numericPart + letterPart
}

/**
 * @param {string} value
 * @param {string|null} allowedLetters
 * @returns {string}
 */
export function filterMixedLatin(value, allowedLetters) {
    const numericPart = value.replace(/\D/g, '')
    const letterPart = filterLatinLetters(value.replace(/[0-9]/g, ''), allowedLetters)
    return numericPart + letterPart
}

/**
 * @param {string} value
 * @param {string|null} allowedLetters
 * @returns {string}
 */
export function filterMixedBoth(value, allowedLetters) {
    const numericPart = value.replace(/\D/g, '')
    const letterPart = filterBothLetters(value.replace(/[0-9]/g, ''), allowedLetters)
    return numericPart + letterPart
}

/**
 * @param {object} cell - ячейка формата номера
 * @param {string} value
 * @returns {string}
 */
export function validatePartValue(value, cell) {
    let result = value.toUpperCase()

    if (cell.cell_type === 'numbers') {
        result = result.replace(/\D/g, '')
    } else if (cell.cell_type === 'letters') {
        if (cell.alphabet_type === 'cyrillic') {
            result = filterCyrillicLetters(result, cell.allowed_letters)
        } else if (cell.alphabet_type === 'latin') {
            result = filterLatinLetters(result, cell.allowed_letters)
        } else if (cell.alphabet_type === 'both') {
            result = filterBothLetters(result, cell.allowed_letters)
        }
    } else if (cell.cell_type === 'mixed') {
        if (cell.alphabet_type === 'cyrillic') {
            result = filterMixedCyrillic(result, cell.allowed_letters)
        } else if (cell.alphabet_type === 'latin') {
            result = filterMixedLatin(result, cell.allowed_letters)
        } else if (cell.alphabet_type === 'both') {
            result = filterMixedBoth(result, cell.allowed_letters)
        }
    }

    if (result.length > cell.max_length) {
        result = result.slice(0, cell.max_length)
    }

    return result
}

/**
 * @param {string} value
 * @param {object} cell - ячейка формата номера
 * @returns {string}
 */
export function formatPartValue(value, cell) {
    if (cell.cell_type === 'numbers' && cell.padding_side && value) {
        const targetLength = cell.max_length
        if (value.length < targetLength) {
            const paddingChar = cell.padding_char || '0'
            if (cell.padding_side === 'left') {
                return value.padStart(targetLength, paddingChar)
            }
            return value.padEnd(targetLength, paddingChar)
        }
    }
    return value
}

/**
 * @param {object|null} selectedFormat
 * @returns {string[]}
 */
export function initializeNumberParts(selectedFormat) {
    if (selectedFormat) {
        return new Array(selectedFormat.cells.length).fill('')
    }
    return []
}

/**
 * Раскладывает строку номера на ячейки формата бэктрекингом: для каждой ячейки перебирает
 * длины от max_length к min_length и проверяет содержимое через validatePartValue (та же
 * проверка, что и на ручном вводе) - подходит только строка, которая целиком, без остатка,
 * укладывается в ячейки формата.
 * @param {string} value - без пробелов, остаток строки для текущей ячейки
 * @param {object[]} cells
 * @param {number} index
 * @returns {string[]|null}
 */
function splitByCells(value, cells, index = 0) {
    if (index === cells.length) {
        return value.length === 0 ? [] : null
    }
    const cell = cells[index]
    const max = cell.max_length || value.length
    const min = cell.min_length || max
    for (let len = Math.min(max, value.length); len >= min; len -= 1) {
        const candidate = value.slice(0, len)
        if (validatePartValue(candidate, cell) !== candidate) continue
        const rest = splitByCells(value.slice(len), cells, index + 1)
        if (rest) return [candidate, ...rest]
    }
    return null
}

/**
 * Подбирает формат номера, под ячейки которого раскладывается строка (U3: у машины из
 * импорта бланка formatId не сохранён - формат неизвестен). Перебирает активные форматы
 * (дефолтный - первым), для каждого пробует разложить строку по ячейкам splitByCells.
 * @param {string} rawNumber
 * @param {{format: object, cells: object[]}[]} formats
 * @returns {{format: object, parts: string[]}|null}
 */
export function matchNumberToFormat(rawNumber, formats) {
    const compact = (rawNumber || '').toString().toUpperCase().replace(/\s+/g, '')
    if (!compact || !Array.isArray(formats) || formats.length === 0) return null

    const ordered = [...formats].sort((a, b) => (b.format.is_default ? 1 : 0) - (a.format.is_default ? 1 : 0))
    for (const fmt of ordered) {
        if (!fmt.cells || fmt.cells.length === 0) continue
        const parts = splitByCells(compact, fmt.cells)
        if (parts) return { format: fmt, parts }
    }
    return null
}

/**
 * Собирает номер обратно с пробелами по формату - для ВЫВОДА в списках/деталях, а не
 * для ввода. У машин, заведённых импортом бланка, гос. номер хранится слитно (formatId
 * не сохраняется), и печатался одним словом ("K321HT777") вместо принятого в системе
 * вида с пробелами ("В 746 КУ 964"), как его собирает форма ручного ввода
 * (VehicleForm numberParts.join(' ')).
 * @param {string} rawNumber - номер как он есть (с пробелами, слитно или в любом регистре)
 * @param {{format: object, cells: object[]}[]} formats
 * @returns {string} номер с пробелами по формату; не подошёл ни под один формат -
 *   исходная строка без изменений (без порчи)
 */
export function formatNumberForDisplay(rawNumber, formats) {
    if (!rawNumber) return rawNumber
    const matched = matchNumberToFormat(rawNumber, formats)
    if (!matched) return rawNumber
    return matched.parts.join(' ')
}

export function useNumberFormat() {
    return {
        filterCyrillicLetters,
        filterLatinLetters,
        filterBothLetters,
        filterMixedCyrillic,
        filterMixedLatin,
        filterMixedBoth,
        validatePartValue,
        formatPartValue,
        initializeNumberParts,
        matchNumberToFormat,
        formatNumberForDisplay
    }
}
