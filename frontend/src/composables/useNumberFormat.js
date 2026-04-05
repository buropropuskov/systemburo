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
        initializeNumberParts
    }
}
