/**
 * Единое ядро "мощного" клиентского поиска: из запроса строит варианты,
 * устойчивые к неправильной раскладке клавиатуры (RU<->EN QWERTY), фонетическому
 * транслиту (кириллица<->латиница), регистру и пробелам (номера слитно/раздельно).
 *
 * Зеркалит логику серверного поиска заявок (normalize.SwitchLayout + варианты),
 * чтобы справочники/таблицы, фильтруемые на клиенте, искали так же "умно".
 */

/** EN-клавиша -> RU-символ на той же физической клавише (ЙЦУКЕН <-> QWERTY). */
const EN_TO_RU_LAYOUT = {
    q: 'й', w: 'ц', e: 'у', r: 'к', t: 'е', y: 'н', u: 'г', i: 'ш', o: 'щ', p: 'з',
    '[': 'х', ']': 'ъ', a: 'ф', s: 'ы', d: 'в', f: 'а', g: 'п', h: 'р', j: 'о',
    k: 'л', l: 'д', ';': 'ж', "'": 'э', z: 'я', x: 'ч', c: 'с', v: 'м', b: 'и',
    n: 'т', m: 'ь', ',': 'б', '.': 'ю', '`': 'ё',
};

/** RU-символ -> EN-клавиша (обратное отображение EN_TO_RU_LAYOUT). */
const RU_TO_EN_LAYOUT = Object.fromEntries(
    Object.entries(EN_TO_RU_LAYOUT).map(([en, ru]) => [ru, en]),
);

/** Фонетический транслит кириллица -> латиница. */
const CYR_TO_LAT = {
    а: 'a', б: 'b', в: 'v', г: 'g', д: 'd', е: 'e', ё: 'e', ж: 'zh', з: 'z', и: 'i',
    й: 'y', к: 'k', л: 'l', м: 'm', н: 'n', о: 'o', п: 'p', р: 'r', с: 's', т: 't',
    у: 'u', ф: 'f', х: 'h', ц: 'ts', ч: 'ch', ш: 'sh', щ: 'sch', ъ: '', ы: 'y',
    ь: '', э: 'e', ю: 'yu', я: 'ya',
};

/** Фонетический транслит латиница -> кириллица. */
const LAT_TO_CYR = {
    a: 'а', b: 'б', c: 'ц', d: 'д', e: 'е', f: 'ф', g: 'г', h: 'х', i: 'и', j: 'й',
    k: 'к', l: 'л', m: 'м', n: 'н', o: 'о', p: 'п', q: 'к', r: 'р', s: 'с', t: 'т',
    u: 'у', v: 'в', w: 'в', x: 'кс', y: 'ы', z: 'з',
};

/** Посимвольно применяет карту к строке (символы без записи в карте остаются как есть). */
function mapChars(text, map) {
    let out = '';
    for (const char of text) out += map[char] ?? char;
    return out;
}

/**
 * Строит набор вариантов поискового запроса.
 * @param {string} query - сырой пользовательский ввод
 * @returns {string[]} уникальные непустые варианты в нижнем регистре
 */
export function buildSearchVariants(query) {
    const base = (query ?? '').toString().toLowerCase().trim();
    if (!base) return [];

    const variants = new Set([base]);
    variants.add(mapChars(base, EN_TO_RU_LAYOUT));
    variants.add(mapChars(base, RU_TO_EN_LAYOUT));
    variants.add(mapChars(base, CYR_TO_LAT));
    variants.add(mapChars(base, LAT_TO_CYR));

    // Версии без пробелов - для номеров и слитного написания.
    for (const variant of [...variants]) {
        if (/\s/.test(variant)) variants.add(variant.replace(/\s+/g, ''));
    }

    variants.delete('');
    return [...variants];
}

/**
 * Проверяет, совпадает ли haystack хотя бы с одним вариантом запроса.
 * Сравнивает и как есть, и без пробелов (номера слитно/раздельно).
 * @param {string} haystack - искомый текст (склейка полей сущности)
 * @param {string[]} variants - результат buildSearchVariants
 * @returns {boolean}
 */
export function matchesSearch(haystack, variants) {
    if (!variants || variants.length === 0) return true;
    const hay = (haystack ?? '').toString().toLowerCase();
    const hayNoSpace = hay.replace(/\s+/g, '');
    return variants.some((v) => hay.includes(v) || hayNoSpace.includes(v));
}
