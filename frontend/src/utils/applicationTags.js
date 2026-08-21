/**
 * Теги строки заявки в Центре: состав, приоритет и укладка в колонку.
 *
 * Колонка тегов узкая (около 90-170px в зависимости от простора и того, закреплено
 * ли нав-меню), а тегов у заявки бывает до восьми. Поэтому тег умеет показываться
 * тремя способами - полным текстом, иконкой с числом и голой иконкой, - а хвост,
 * которому места не осталось, сворачивается в счётчик "+N" с перечнем в подсказке.
 * Раскладку считаем от РЕАЛЬНОЙ ширины колонки (её замеряет потребитель), а не от
 * числа тегов: ширина зависит и от вьюпорта, и от закрепления навигации, и от того,
 * как flex поделил простор между колонками.
 */
import { blacklistFlagCount, blacklistFlagLabel, BLACKLIST_FLAG_TITLE } from './blacklistBadge';
import { pendingApprovalDays, pendingApprovalLabel, pendingApprovalShort } from './pendingApproval';

/** Badge size="sm": горизонтальные padding 8px + рамка 1px с каждой стороны. */
const TEXT_PADDING = 18;
/** Свёрнутый в кружок тег: padding 4px + иконка 13px + рамка. */
const ICON_TAG_WIDTH = 23;
/** Видимая иконка внутри тега с текстом: 13px + зазор 3px. */
const ICON_WIDTH = 16;
/** gap между тегами в строке. */
const TAG_GAP = 4;
/** Запас на округления и субпиксельные ширины: лучше свернуть лишний тег, чем наехать. */
const SAFETY_MARGIN = 2;

/**
 * Порядок = приоритет: что выше, то сворачивается последним и никогда не уходит под
 * "+N". ЧС первым - критичный флаг и якорь шага онбординга принимающего.
 */
const TAG_ORDER = ['chs', 'awaiting', 'questions', 'supplement', 'roof', 'parking', 'important', 'files'];

/**
 * Описание тега: как он выглядит в каждом из режимов и куда его можно сжимать.
 * minMode - предел свёртки: у ЧС и срока ожидания это 'count' (число нельзя терять,
 * без него тег перестаёт отвечать на вопрос "сколько"), у остальных - 'icon'.
 */
const TAG_DEFS = {
  chs: {
    variant: 'danger',
    minMode: 'count',
    iconInText: false,
    testid: () => 'ob-center-blacklist-tag',
    match: (app) => blacklistFlagCount(app) > 0,
    text: (app) => blacklistFlagLabel(app),
    count: (app) => String(blacklistFlagCount(app)),
    hint: () => BLACKLIST_FLAG_TITLE,
  },
  awaiting: {
    variant: 'warning',
    minMode: 'count',
    iconInText: true,
    match: (app) => pendingApprovalDays(app) !== null,
    text: (app) => pendingApprovalShort(pendingApprovalDays(app)),
    count: (app) => String(pendingApprovalDays(app)),
    hint: (app) => pendingApprovalLabel(pendingApprovalDays(app)),
  },
  questions: {
    variant: 'primary',
    minMode: 'icon',
    iconInText: false,
    testid: (app) => `center-questions-badge-${app.id}`,
    match: (app) => !!app.has_unseen_questions,
    text: () => 'Вопросы',
    hint: () => 'Есть новые вопросы или ответы',
  },
  supplement: {
    variant: 'info',
    minMode: 'icon',
    iconInText: false,
    testid: (app) => `center-supplement-badge-${app.id}`,
    match: (app) => !!app.has_open_supplement,
    text: () => 'Дополнение',
    hint: () => 'Идёт согласование дополнения к заявке',
  },
  roof: {
    variant: 'primary',
    minMode: 'icon',
    iconInText: false,
    match: (app) => !!app.has_roof_access,
    text: () => 'Крыша',
    hint: () => 'Доступ на крышу',
  },
  parking: {
    variant: 'warning',
    minMode: 'icon',
    iconInText: false,
    match: (app) => !!app.has_free_parking,
    text: () => 'Парковка',
    hint: () => 'Бесплатная парковка',
  },
  important: {
    variant: 'info',
    minMode: 'icon',
    iconInText: false,
    match: (app) => !!app.sender_is_important,
    text: () => 'Важный',
    hint: () => 'Важный пользователь',
  },
  files: {
    variant: 'neutral',
    minMode: 'icon',
    iconInText: false,
    textless: true,
    testid: (app) => `center-files-badge-${app.id}`,
    match: (app) => !!app.has_files,
    text: () => 'Файлы',
    hint: () => 'К заявке приложены файлы',
  },
};

/**
 * Теги заявки в порядке приоритета.
 * @param {object} application строка списка заявок
 * @returns {Array<{key: string, variant: string, text: string, countText: ?string,
 *   hint: string, testid: ?string, minMode: string, iconInText: boolean}>}
 */
export function buildApplicationTags(application) {
  if (!application) return [];
  return TAG_ORDER.filter((key) => TAG_DEFS[key].match(application)).map((key) => {
    const def = TAG_DEFS[key];
    return {
      key,
      variant: def.variant,
      text: def.text(application),
      countText: def.count ? def.count(application) : null,
      hint: def.hint(application),
      testid: def.testid ? def.testid(application) : null,
      minMode: def.minMode,
      iconInText: !!def.iconInText,
      textless: !!def.textless,
    };
  });
}

const widthCache = new Map();
let measureContext;

/**
 * Ширина строки в шрифте бейджа. В браузере меряем канвасом - посимвольная оценка
 * промахивается на широких буквах ("Крыша" на 4px уже, чем считает эвристика), а
 * промах в меньшую сторону = наезд на соседнюю колонку. В jsdom и прочих средах без
 * канваса остаётся эвристика: там раскладка всё равно идёт по дефолтной ветке.
 */
function measureText(text) {
  if (!text) return 0;
  if (widthCache.has(text)) return widthCache.get(text);

  let width = null;
  if (measureContext === undefined) {
    measureContext = null;
    try {
      const ctx = document.createElement('canvas').getContext('2d');
      if (ctx) {
        ctx.font = '500 11px Montserrat, sans-serif';
        measureContext = ctx;
      }
    } catch {
      // Среда без канваса (jsdom в юнит-тестах) - меряем эвристикой ниже. Ловить
      // тут нечего: раскладка от этого не ломается, она лишь чуть осторожнее.
    }
  }
  if (measureContext) {
    width = measureContext.measureText(text).width;
  }
  if (width === null || !Number.isFinite(width) || width <= 0) {
    width = estimateTextWidth(text);
  }
  widthCache.set(text, width);
  return width;
}

/** Запасная оценка ширины текста: буквы шире цифр и пунктуации. */
export function estimateTextWidth(text) {
  let width = 0;
  for (const char of String(text)) {
    if (/[A-ZА-ЯЁ]/.test(char)) width += 8.6;
    else if (/[a-zа-яё]/.test(char)) width += 7.4;
    else width += 6;
  }
  return width;
}

/**
 * Ширина тега в конкретном режиме показа.
 * @param {object} tag
 * @param {'text'|'count'|'icon'} mode
 * @returns {number}
 */
export function tagWidth(tag, mode) {
  if (mode === 'icon') return ICON_TAG_WIDTH;
  if (mode === 'count') return TEXT_PADDING + ICON_WIDTH + measureText(tag.countText || '');
  return TEXT_PADDING + (tag.iconInText ? ICON_WIDTH : 0) + measureText(tag.text);
}

/** Ширина счётчика скрытых тегов ("+3"). */
function counterWidth(hiddenCount) {
  return TEXT_PADDING + measureText(`+${hiddenCount}`);
}

function rowWidth(entries, hiddenCount) {
  let width = entries.reduce((sum, e) => sum + tagWidth(e.tag, e.mode), 0);
  const items = entries.length + (hiddenCount > 0 ? 1 : 0);
  if (items > 1) width += TAG_GAP * (items - 1);
  if (hiddenCount > 0) width += counterWidth(hiddenCount);
  return width;
}

/** Режим, в котором тег показывается по умолчанию (когда места вдоволь). */
function defaultMode(tag) {
  return tag.textless ? 'icon' : 'text';
}

/**
 * Раскладка тегов по доступной ширине колонки.
 *
 * Сначала все теги ужимаются до предела (иконка или иконка с числом) и хвост, который
 * всё равно не влезает, уходит в счётчик. Затем оставшиеся по одному разворачиваются
 * обратно в текст, пока хватает места, - первым тот, что выше по приоритету. Так при
 * тесной колонке строка остаётся иконочной, а при просторной сама возвращает подписи.
 *
 * @param {Array} tags теги в порядке приоритета (buildApplicationTags)
 * @param {number} availableWidth ширина колонки в px; 0 или отсутствие = места вдоволь
 *   (мобильная карточка, ещё не выполненный замер) - показываем всё полным текстом
 * @returns {{visible: Array<{tag: object, mode: string}>, hidden: Array}}
 */
export function layoutApplicationTags(tags, availableWidth) {
  if (!tags || !tags.length) return { visible: [], hidden: [] };
  if (!availableWidth || availableWidth <= 0) {
    return { visible: tags.map((tag) => ({ tag, mode: defaultMode(tag) })), hidden: [] };
  }

  const budget = availableWidth - SAFETY_MARGIN;

  let visibleCount = tags.length;
  while (visibleCount > 1) {
    const entries = tags.slice(0, visibleCount).map((tag) => ({ tag, mode: tag.minMode }));
    if (rowWidth(entries, tags.length - visibleCount) <= budget) break;
    visibleCount -= 1;
  }

  const visible = tags.slice(0, visibleCount).map((tag) => ({ tag, mode: tag.minMode }));
  const hidden = tags.slice(visibleCount);

  for (const entry of visible) {
    const target = defaultMode(entry.tag);
    if (entry.mode === target) continue;
    const previous = entry.mode;
    entry.mode = target;
    if (rowWidth(visible, hidden.length) > budget) entry.mode = previous;
  }

  return { visible, hidden };
}
