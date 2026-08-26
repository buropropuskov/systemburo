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


/**
 * Расстояние Дамерау-Левенштейна с ранним выходом: как только минимум по
 * строке превысил лимит, считать дальше незачем. Перестановка соседних
 * символов стоит одну правку - «935» против «395» это типичная опечатка,
 * а не два независимых промаха.
 */
function editDistance(a, b, limit) {
    if (Math.abs(a.length - b.length) > limit) return limit + 1;

    const rows = [Array.from({ length: b.length + 1 }, (_, i) => i)];
    for (let i = 1; i <= a.length; i++) {
        const cur = [i];
        let rowMin = i;
        const prev = rows[i - 1];
        const prev2 = rows[i - 2];
        for (let j = 1; j <= b.length; j++) {
            const cost = a[i - 1] === b[j - 1] ? 0 : 1;
            let value = Math.min(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + cost);
            if (i > 1 && j > 1 && a[i - 1] === b[j - 2] && a[i - 2] === b[j - 1]) {
                value = Math.min(value, prev2[j - 2] + 1);
            }
            cur[j] = value;
            if (value < rowMin) rowMin = value;
        }
        if (rowMin > limit) return limit + 1;
        rows.push(cur);
    }
    return rows[a.length][b.length];
}

/** Только цифры - тогда для номера имеет смысл сравнивать состав, а не порядок. */
function isDigits(text) {
    return /^\d+$/.test(text);
}

/**
 * Цифры те же, порядок другой: «359» и «935». Номер запоминают набором цифр,
 * порядок путают, поэтому для чисто числового запроса перестановка считается
 * совпадением. К буквам не применяем - там перестановка всех символов это
 * обычно другое слово, а не опечатка.
 */
function sameDigitsInAnyOrder(candidate, needle) {
    if (candidate.length !== needle.length) return false;
    if (!isDigits(candidate) || !isDigits(needle)) return false;
    return [...candidate].sort().join('') === [...needle].sort().join('');
}

/**
 * Порог близости для фрагмента запроса.
 *
 * Метрика та же, что у серверной проверки обхода ЧС по номеру машины
 * (`vehicleBlacklistService.FindSimilar`): `1 - levenshtein / max(len)`. Порог
 * там 0.7 и сравниваются номера целиком, а здесь пользователь вводит фрагмент:
 * на трёх символах одна опечатка даёт ровно 0.667, поэтому берём чуть мягче,
 * иначе «942» не нашло бы «952» - ровно тот случай, с которого фича началась.
 */
const FRAGMENT_SIMILARITY_THRESHOLD = 0.65;

/** Минимальная длина фрагмента: на двух символах «похожим» окажется что угодно. */
const MIN_FUZZY_LENGTH = 3;

/** Сколько правок укладывается в порог для фрагмента такой длины. */
function allowedTypos(length) {
    if (length < MIN_FUZZY_LENGTH) return 0;
    return Math.floor(length * (1 - FRAGMENT_SIMILARITY_THRESHOLD));
}

/**
 * Поиск, устойчивый к опечатке: «942» находит «У 952 ЕУ 935», «мерсдес» -
 * «Мерседес». Сначала обычное вхождение, нечёткое сравнение подключается,
 * только когда точного совпадения не нашлось.
 *
 * Сравниваем со словами и со склейками соседних слов, а не с произвольными
 * кусками строки. Склейки нужны потому, что номер пишут и раздельно, и слитно:
 * запрос «у953» должен находить «У 952 ЕУ 935», хотя «у» и «952» - разные
 * слова. Произвольные окна не годятся: на «У 465 КУ 423» нашёлся бы кусок
 * «у42» в одной замене от «942», и запрос вытаскивал бы полтаблицы.
 *
 * Метрика та же, что у серверной проверки обхода ЧС по номеру
 * (`vehicleBlacklistService.FindSimilar`), сравнение пословное - как
 * `strict_word_similarity` в поиске заявок. Сверх этого: перестановка соседних
 * символов считается одной правкой, а числовой фрагмент совпадает с теми же
 * цифрами в любом порядке («359» находит «935»).
 *
 * @param {string} haystack - искомый текст (склейка полей сущности)
 * @param {string[]} variants - результат buildSearchVariants
 * @returns {boolean}
 */
export function matchesSearchFuzzy(haystack, variants) {
    if (matchesSearch(haystack, variants)) return true;
    if (!variants || variants.length === 0) return false;

    const words = (haystack ?? '').toString().toLowerCase().split(/[^\p{L}\p{N}]+/u).filter(Boolean);
    if (!words.length) return false;

    return variants.some((variant) => {
        const needle = variant.replace(/\s+/g, '');
        const limit = allowedTypos(needle.length);
        if (!limit) return false;

        for (let start = 0; start < words.length; start++) {
            let candidate = '';
            for (let end = start; end < words.length; end++) {
                candidate += words[end];
                if (candidate.length > needle.length + limit) break;
                if (Math.abs(candidate.length - needle.length) > limit) continue;
                if (sameDigitsInAnyOrder(candidate, needle)) return true;
                if (editDistance(candidate, needle, limit) <= limit) return true;
            }
        }
        return false;
    });
}
