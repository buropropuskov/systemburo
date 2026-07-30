/**
 * Извлечение текста из загруженного документа согласия (PDF/DOCX/XLSX) в HTML для
 * редактора (#1567). Текст переносится сразу при загрузке файла, поэтому разметка
 * должна получаться читаемой без ручной доводки: абзацы разделены, нумерованные
 * пункты стоят каждый со своей строки, маркированные списки собраны в <ul>,
 * заголовки размечены.
 *
 * Результат всё равно стоит вычитать: извлечение не воспроизводит таблицы, теряет
 * автонумерацию Word и не всегда угадывает перенос по слогам (в «веб-браузер»
 * дефис на переносе неотличим от дефиса в самом слове).
 *
 * Новых зависимостей не нужно: pdfjs-dist, jszip и exceljs уже в бандле.
 */

/** Формат, для которого извлечения нет (например легаси .doc). */
export class UnsupportedDocumentError extends Error {
  constructor(message) {
    super(message);
    this.name = 'UnsupportedDocumentError';
  }
}

/** Пункт с многоуровневым номером: "1." / "1.1." / "2.3.4)" - основа юридического текста. */
const NUMBERED_CLAUSE = /^\d+(?:\.\d+)*[.)]\s/;

/** Пункт с буквенным номером: "а)" / "b)". */
const LETTER_CLAUSE = /^[a-zа-яё][)]\s/i;

/** Маркер списка в начале строки; захватывается вместе с отступом, чтобы срезать его. */
const BULLET_MARKER = /^[•▪◦‣·*]\s*/;

/** Тире как маркер списка: обязателен пробел после, иначе это часть слова. */
const DASH_MARKER = /^[-–—]\s+/;

/** Строка целиком номер страницы. */
const PAGE_NUMBER_ONLY = /^\s*[-–—]?\s*\d+\s*[-–—]?\s*$/;

/** Конец предложения: по нему решаем, продолжается ли абзац через разрыв страницы. */
const SENTENCE_END = /[.!?;:][»"')\]]?$/;

/** Доля прописных среди букв, начиная с которой строка считается заголовком. */
const HEADING_UPPERCASE_RATIO = 0.6;

/** Длиннее этого заголовком не считаем - это уже абзац капслоком. */
const HEADING_MAX_LENGTH = 100;

/** Во сколько раз разрыв между строками должен превышать типовой, чтобы начать абзац. */
const PARAGRAPH_GAP_FACTOR = 1.25;

/** Насколько строка должна не дотянуть до правого края, чтобы считаться последней в абзаце. */
const SHORT_LINE_RATIO = 0.12;

const WORD_NS = 'http://schemas.openxmlformats.org/wordprocessingml/2006/main';

let pdfjsPromise = null;

/**
 * Лениво поднимает pdf.js с воркером. Порт воркера глобальный на весь pdf.js,
 * поэтому не перезаписываем уже выставленный (его мог поднять инлайн-просмотрщик
 * документа) - иначе прежний воркер остался бы висеть.
 * @returns {Promise<object>}
 */
function loadPdfjs() {
  if (!pdfjsPromise) {
    pdfjsPromise = (async () => {
      const lib = await import('pdfjs-dist');
      if (!lib.GlobalWorkerOptions.workerPort) {
        const { default: PdfWorker } = await import('pdfjs-dist/build/pdf.worker.min.mjs?worker');
        lib.GlobalWorkerOptions.workerPort = new PdfWorker();
      }
      return lib;
    })().catch((err) => {
      // Не кэшируем провал: следующая попытка должна пробовать снова.
      pdfjsPromise = null;
      throw err;
    });
  }
  return pdfjsPromise;
}

/**
 * Строка начинает новый блок: нумерованный или маркированный пункт.
 * @param {string} text
 * @returns {boolean}
 */
function startsBlock(text) {
  return NUMBERED_CLAUSE.test(text)
    || LETTER_CLAUSE.test(text)
    || BULLET_MARKER.test(text)
    || DASH_MARKER.test(text);
}

/**
 * Собирает строки страницы PDF с геометрией: по ней дальше считаются разрывы
 * между абзацами. Разбиение по hasEOL, который pdf.js выставляет на последнем
 * элементе визуальной строки.
 *
 * Пробел между соседними элементами восстанавливаем по горизонтальному зазору:
 * часть PDF не кладёт пробелы в текстовый слой вовсе, и без этого слова слипаются.
 * @param {Array<{str: string, hasEOL: boolean, width: number, height: number, transform: number[]}>} items
 * @returns {Array<{text: string, x: number, endX: number, y: number, height: number}>}
 */
function itemsToLines(items) {
  const lines = [];
  let text = '';
  let y = 0;
  let height = 0;
  let startX = 0;
  let prevEnd = null;

  for (const item of items) {
    const chunk = item.str || '';
    const x = item.transform?.[4];
    const width = item.width || 0;
    const itemHeight = item.height || 0;

    if (!text) {
      y = item.transform?.[5] ?? 0;
      height = itemHeight;
      startX = x ?? 0;
    } else if (
      chunk
      && prevEnd !== null
      && typeof x === 'number'
      && !/\s$/.test(text)
      && !/^\s/.test(chunk)
      // Порог в долях кегля: у мелкого шрифта межсловный зазор тоже мельче.
      && x - prevEnd > 0.2 * Math.max(itemHeight, height, 1)
    ) {
      text += ' ';
    }

    text += chunk;
    height = Math.max(height, itemHeight);
    if (typeof x === 'number') prevEnd = x + width;

    if (item.hasEOL) {
      const trimmed = text.trim();
      if (trimmed) lines.push({ text: trimmed, x: startX, endX: prevEnd ?? startX, y, height });
      text = '';
      prevEnd = null;
    }
  }
  const tail = text.trim();
  if (tail) lines.push({ text: tail, x: startX, endX: prevEnd ?? startX, y, height });
  return lines;
}

/**
 * Находит колонтитулы: первая/последняя строка страницы, повторяющаяся на
 * большинстве страниц. Разовые совпадения не режем - на двух-трёх страницах
 * повтор может быть просто одинаковым началом абзаца.
 * @param {Array<Array<{text: string}>>} pages
 * @returns {Set<string>}
 */
function detectRunningHeads(pages) {
  const heads = new Set();
  if (pages.length < 3) return heads;
  const counts = new Map();
  for (const lines of pages) {
    const candidates = new Set();
    if (lines.length) candidates.add(lines[0].text);
    if (lines.length > 1) candidates.add(lines[lines.length - 1].text);
    for (const line of candidates) {
      if (!line) continue;
      counts.set(line, (counts.get(line) || 0) + 1);
    }
  }
  const threshold = Math.max(2, Math.ceil(pages.length / 2));
  for (const [line, count] of counts) {
    if (count >= threshold) heads.add(line);
  }
  return heads;
}

/**
 * Типовой межстрочный интервал документа: по нему отличаем перевод строки внутри
 * абзаца от отбивки между абзацами. Берём 40-й процентиль, а не медиану - разрывы
 * между абзацами тоже попадают в выборку и тянут середину вверх, а нужна именно
 * нижняя, «внутриабзацная» гроздь значений.
 * @param {Array<Array<{y: number}>>} pages
 * @returns {number} 0, если геометрии нет (тогда разрывы по ней не считаем)
 */
function typicalLineGap(pages) {
  const gaps = [];
  for (const lines of pages) {
    for (let i = 1; i < lines.length; i += 1) {
      const gap = lines[i - 1].y - lines[i].y;
      if (gap > 0) gaps.push(gap);
    }
  }
  if (!gaps.length) return 0;
  gaps.sort((a, b) => a - b);
  return gaps[Math.floor(gaps.length * 0.4)];
}

/**
 * Склеивает строки одного блока в текст. Строка, оборванная дефисом, всегда
 * сшивается со следующей без пробела; сам дефис убираем только если перед ним
 * строчная буква - в «IP-адрес» и «ГОСТ-Р» дефис авторский и должен остаться.
 * @param {string[]} lines
 * @returns {string}
 */
function joinLines(lines) {
  let text = '';
  for (const line of lines) {
    if (!text) {
      text = line;
      continue;
    }
    if (/[-­]$/.test(text) && /^[\p{L}\d]/u.test(line)) {
      text = /[a-zа-яё][-­]$/.test(text) && /^[a-zа-яё]/.test(line)
        ? text.slice(0, -1) + line
        : text + line;
    } else {
      text += ` ${line}`;
    }
  }
  return text;
}

/**
 * Строка выглядит заголовком: прописные преобладают, длина небольшая. Номер
 * раздела («2. ОБЩИЕ ПОЛОЖЕНИЯ») в подсчёт долей не входит - считаем по буквам.
 * @param {string} text
 * @returns {boolean}
 */
function looksLikeHeading(text) {
  if (text.length > HEADING_MAX_LENGTH) return false;
  const letters = text.match(/\p{L}/gu);
  if (!letters || letters.length < 3) return false;
  const upper = letters.filter((ch) => ch === ch.toUpperCase() && ch !== ch.toLowerCase());
  return upper.length / letters.length >= HEADING_UPPERCASE_RATIO;
}

/**
 * Ширина, начиная с которой строка считается недотянувшей до правого края.
 * Считается по странице: у разных страниц бывают разные поля.
 * @param {Array<{x: number, endX: number}>} lines
 * @returns {number} 0, если ширины страницы не видно
 */
function shortLineEdge(lines) {
  if (!lines.length) return 0;
  const maxEnd = Math.max(...lines.map((line) => line.endX));
  const minStart = Math.min(...lines.map((line) => line.x));
  const width = maxEnd - minStart;
  if (width <= 0) return 0;
  return maxEnd - width * SHORT_LINE_RATIO;
}

/**
 * Собирает блоки из строк PDF: где кончается абзац, решают отбивка между
 * строками, номер пункта в начале строки, недотянувшая до края последняя строка
 * и разрыв страницы.
 * @param {Array<Array<{text: string, x: number, endX: number, y: number, height: number}>>} pages
 * @returns {Array<{type: string, text: string}>}
 */
function pdfLinesToBlocks(pages) {
  const gapThreshold = typicalLineGap(pages) * PARAGRAPH_GAP_FACTOR;
  const blocks = [];
  let current = [];

  const flush = () => {
    if (!current.length) return;
    const heading = current.every((line) => line.heading);
    const raw = joinLines(current.map((line) => line.text));
    current = [];
    const bullet = BULLET_MARKER.test(raw) || DASH_MARKER.test(raw);
    if (bullet) {
      const text = raw.replace(BULLET_MARKER, '').replace(DASH_MARKER, '').trim();
      if (text) blocks.push({ type: 'bullet', text });
      return;
    }
    blocks.push({ type: heading ? 'heading' : 'paragraph', text: raw });
  };

  pages.forEach((lines, pageIndex) => {
    const edge = shortLineEdge(lines);
    // Заголовок - строка прописными, НЕ дотянувшая до правого края. Без проверки
    // ширины за заголовок принималось бы и перечисление капсом внутри абзаца
    // («ФИО ПАСПОРТНЫЕ ДАННЫЕ АДРЕС»), разрывая предложение натрое.
    const headings = lines.map((line) => looksLikeHeading(line.text) && (edge <= 0 || line.endX < edge));
    lines.forEach((line, lineIndex) => {
      const first = pageIndex === 0 && lineIndex === 0;
      let breakHere = false;
      if (!first && current.length) {
        const prev = lines[lineIndex - 1];
        // Заголовок стоит особняком: он и закрывает предыдущий блок, и не даёт
        // приклеить к себе следующую строку. Две подряд заголовочные строки -
        // это один заголовок в две строки, их не разделяем (разделит отбивка,
        // если заголовка на самом деле два).
        const heading = headings[lineIndex] !== current[current.length - 1].heading;
        // Строка, оборвавшаяся заметно раньше правого края на законченной мысли,
        // закрывает абзац: продолжение заполнило бы строку до конца.
        const prevEndsBlock = prev
          && edge > 0
          && prev.endX < edge
          && SENTENCE_END.test(prev.text);
        if (lineIndex === 0) {
          // Через разрыв страницы абзац продолжается, только если предыдущая
          // страница оборвалась на середине предложения.
          breakHere = heading
            || startsBlock(line.text)
            || SENTENCE_END.test(current[current.length - 1].text);
        } else {
          const gap = prev.y - line.y;
          breakHere = heading
            || startsBlock(line.text)
            || prevEndsBlock
            || (gapThreshold > 0 && gap > gapThreshold);
        }
      }
      if (breakHere) flush();
      current.push({ text: line.text, heading: headings[lineIndex] });
    });
  });
  flush();
  return blocks;
}

/**
 * Извлекает блоки из PDF через текстовый слой pdf.js.
 * @param {Blob} blob
 * @returns {Promise<Array<{type: string, text: string}>>}
 */
async function extractPdfBlocks(blob) {
  let lib;
  let pdf;
  try {
    lib = await loadPdfjs();
    const data = new Uint8Array(await blob.arrayBuffer());
    pdf = await lib.getDocument({ data }).promise;
  } catch (err) {
    // Сообщения pdf.js техничные и на английском - администратору нужен внятный
    // текст, как в ветке DOCX. Типовые причины: битый файл и пароль на открытие.
    throw new UnsupportedDocumentError(
      `Не удалось прочитать PDF: ${err?.message || 'файл повреждён или защищён паролем'}.`,
    );
  }
  try {
    const pages = [];
    for (let i = 1; i <= pdf.numPages; i += 1) {
      const page = await pdf.getPage(i);
      const content = await page.getTextContent();
      pages.push(itemsToLines(content.items || []));
      page.cleanup?.();
    }
    const runningHeads = detectRunningHeads(pages);
    const cleaned = pages.map((lines) => lines.filter(
      (line) => !runningHeads.has(line.text) && !PAGE_NUMBER_ONLY.test(line.text),
    ));
    return pdfLinesToBlocks(cleaned.filter((lines) => lines.length));
  } finally {
    pdf.destroy?.();
  }
}

/**
 * Текст одного абзаца DOCX: w:t собираем как есть, w:tab и w:br дают пробел.
 * @param {Element} paragraph
 * @returns {string}
 */
function docxParagraphText(paragraph) {
  let text = '';
  const walk = (node) => {
    for (const child of Array.from(node.childNodes)) {
      if (child.nodeType !== 1) continue;
      const name = child.localName;
      if (name === 't') {
        text += child.textContent || '';
      } else if (name === 'tab' || name === 'br' || name === 'cr') {
        text += ' ';
      } else {
        walk(child);
      }
    }
  };
  walk(paragraph);
  return text.replace(/\s+/g, ' ').trim();
}

/**
 * Читает word/numbering.xml и возвращает numId -> нумерованный ли список.
 * Без этого нумерованные пункты Word приезжали маркерами: сама нумерация в текст
 * не входит (её рисует Word), а тип списка лежит только здесь.
 * @param {object} zip открытый JSZip
 * @returns {Promise<Map<string, boolean>>}
 */
async function docxNumberingKinds(zip) {
  const kinds = new Map();
  const entry = zip.file('word/numbering.xml');
  if (!entry) return kinds;
  let doc;
  try {
    doc = new DOMParser().parseFromString(await entry.async('string'), 'application/xml');
  } catch {
    return kinds;
  }
  if (doc.getElementsByTagName('parsererror').length) return kinds;

  const attr = (el, name) => el.getAttributeNS(WORD_NS, name) || el.getAttribute(`w:${name}`);
  const orderedByAbstract = new Map();
  for (const abstract of Array.from(doc.getElementsByTagNameNS(WORD_NS, 'abstractNum'))) {
    const id = attr(abstract, 'abstractNumId');
    const level = Array.from(abstract.getElementsByTagNameNS(WORD_NS, 'lvl'))
      .find((lvl) => attr(lvl, 'ilvl') === '0');
    const fmt = level?.getElementsByTagNameNS(WORD_NS, 'numFmt')[0];
    orderedByAbstract.set(id, fmt ? attr(fmt, 'val') !== 'bullet' : true);
  }
  for (const num of Array.from(doc.getElementsByTagNameNS(WORD_NS, 'num'))) {
    const ref = num.getElementsByTagNameNS(WORD_NS, 'abstractNumId')[0];
    const abstractId = ref ? attr(ref, 'val') : null;
    kinds.set(attr(num, 'numId'), orderedByAbstract.get(abstractId) ?? true);
  }
  return kinds;
}

/**
 * Извлекает блоки из DOCX: распаковываем word/document.xml и читаем w:p в
 * порядке документа (текст ячеек таблиц попадёт отдельными абзацами - таблицы
 * извлечение не воспроизводит).
 * @param {Blob} blob
 * @returns {Promise<Array<{type: string, text: string}>>}
 */
async function extractDocxBlocks(blob) {
  const { default: JSZip } = await import('jszip');
  let zip;
  try {
    zip = await JSZip.loadAsync(blob);
  } catch {
    throw new UnsupportedDocumentError('Файл не читается как DOCX. Проверьте, что документ не повреждён.');
  }
  const entry = zip.file('word/document.xml');
  if (!entry) {
    throw new UnsupportedDocumentError('В файле нет word/document.xml - это не DOCX.');
  }
  const xml = await entry.async('string');
  const doc = new DOMParser().parseFromString(xml, 'application/xml');
  if (doc.getElementsByTagName('parsererror').length) {
    throw new UnsupportedDocumentError('Не удалось разобрать содержимое DOCX.');
  }
  const numbering = await docxNumberingKinds(zip);
  const blocks = [];
  for (const paragraph of Array.from(doc.getElementsByTagNameNS(WORD_NS, 'p'))) {
    const text = docxParagraphText(paragraph);
    if (!text) continue;
    const numPr = paragraph.getElementsByTagNameNS(WORD_NS, 'numPr')[0];
    if (numPr) {
      const numId = numPr.getElementsByTagNameNS(WORD_NS, 'numId')[0];
      const id = numId ? (numId.getAttributeNS(WORD_NS, 'val') || numId.getAttribute('w:val')) : null;
      // Пункт уже несёт номер в тексте - второй номер от <ol> был бы лишним.
      const ordered = NUMBERED_CLAUSE.test(text) ? false : (numbering.get(id) ?? false);
      blocks.push({ type: ordered ? 'ordered' : 'bullet', text: text.replace(BULLET_MARKER, '').trim() });
      continue;
    }
    const style = paragraph.getElementsByTagNameNS(WORD_NS, 'pStyle')[0];
    const styleName = style ? (style.getAttributeNS(WORD_NS, 'val') || style.getAttribute('w:val') || '') : '';
    if (/^heading/i.test(styleName) || /^Заголовок/i.test(styleName) || looksLikeHeading(text)) {
      blocks.push({ type: 'heading', text });
      continue;
    }
    blocks.push({ type: 'paragraph', text });
  }
  return blocks;
}

/**
 * Текст ячейки XLSX. Значение приходит числом, датой, формулой или rich-text -
 * у каждого свой способ добраться до строки.
 * @param {*} value
 * @returns {string}
 */
function xlsxCellText(value) {
  if (value === null || value === undefined) return '';
  if (value instanceof Date) return value.toLocaleDateString('ru-RU');
  if (typeof value === 'object') {
    if (Array.isArray(value.richText)) {
      return value.richText.map((part) => part.text || '').join('');
    }
    if (value.text !== undefined) return String(value.text);
    if (value.result !== undefined) return String(value.result);
    if (value.hyperlink) return String(value.hyperlink);
    return '';
  }
  return String(value);
}

/**
 * Извлекает блоки из XLSX: строка листа становится абзацем, непустые ячейки в
 * ней склеиваются пробелом. Табличную сетку не воспроизводим - редактор текста
 * согласия таблиц не держит, а текст сохранить важнее.
 * @param {Blob} blob
 * @returns {Promise<Array<{type: string, text: string}>>}
 */
async function extractXlsxBlocks(blob) {
  const { default: ExcelJS } = await import('exceljs');
  const workbook = new ExcelJS.Workbook();
  try {
    await workbook.xlsx.load(await blob.arrayBuffer());
  } catch {
    throw new UnsupportedDocumentError('Файл не читается как XLSX. Проверьте, что документ не повреждён.');
  }

  const sheets = [];
  workbook.eachSheet((sheet) => {
    const rows = [];
    sheet.eachRow((row) => {
      const cells = [];
      row.eachCell({ includeEmpty: false }, (cell) => {
        const text = xlsxCellText(cell.value).trim();
        if (text) cells.push(text);
      });
      const text = cells.join(' ').replace(/\s+/g, ' ').trim();
      if (text) rows.push(text);
    });
    if (rows.length) sheets.push({ name: sheet.name, rows });
  });

  const blocks = [];
  for (const sheet of sheets) {
    // Имя листа даём, только когда листов несколько: одинокий «Лист1» - шум.
    if (sheets.length > 1 && sheet.name) blocks.push({ type: 'heading', text: sheet.name });
    for (const text of sheet.rows) {
      if (BULLET_MARKER.test(text) || DASH_MARKER.test(text)) {
        const clean = text.replace(BULLET_MARKER, '').replace(DASH_MARKER, '').trim();
        if (clean) blocks.push({ type: 'bullet', text: clean });
        continue;
      }
      blocks.push({ type: looksLikeHeading(text) ? 'heading' : 'paragraph', text });
    }
  }
  return blocks;
}

/**
 * Экранирует текст для вставки в HTML.
 * @param {string} value
 * @returns {string}
 */
function escapeHtml(value) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

/**
 * Собирает HTML из блоков: подряд идущие пункты одного вида группируются в один
 * список. Заголовок даём вторым уровнем - редактор текста согласия знает только
 * h1 и h2, третий уровень он бы выбросил.
 * @param {Array<{type: string, text: string}>} blocks
 * @returns {string}
 */
function blocksToHtml(blocks) {
  const html = [];
  let listTag = null;
  const closeList = () => {
    if (listTag) {
      html.push(`</${listTag}>`);
      listTag = null;
    }
  };

  for (const block of blocks) {
    const text = escapeHtml(block.text);
    if (block.type === 'bullet' || block.type === 'ordered') {
      const tag = block.type === 'ordered' ? 'ol' : 'ul';
      if (listTag !== tag) {
        closeList();
        html.push(`<${tag}>`);
        listTag = tag;
      }
      html.push(`<li>${text}</li>`);
      continue;
    }
    closeList();
    html.push(block.type === 'heading' ? `<h2>${text}</h2>` : `<p>${text}</p>`);
  }
  closeList();
  return html.join('');
}

/**
 * Извлекает текст документа согласия в HTML для редактора.
 *
 * @param {Blob} blob содержимое документа
 * @param {string} ext расширение из метаданных документа (".pdf", ".docx", ".xlsx", ".doc")
 * @returns {Promise<string>} HTML из абзацев, заголовков и списков; пустая строка, если текста нет
 * @throws {UnsupportedDocumentError} для форматов без извлечения (.doc) и битых файлов
 */
export async function extractDocumentHtml(blob, ext) {
  if (!blob) throw new UnsupportedDocumentError('Документ не загружен.');
  const format = String(ext || '').toLowerCase().replace(/^\./, '');

  if (format === 'pdf') return blocksToHtml(await extractPdfBlocks(blob));
  if (format === 'docx') return blocksToHtml(await extractDocxBlocks(blob));
  if (format === 'xlsx') return blocksToHtml(await extractXlsxBlocks(blob));
  if (format === 'doc') {
    throw new UnsupportedDocumentError(
      'Из старого формата .doc текст не извлекается. Сохраните документ как DOCX, XLSX или PDF либо вставьте текст вручную.',
    );
  }
  throw new UnsupportedDocumentError(`Извлечение текста из «${format || 'без расширения'}» не поддерживается.`);
}
