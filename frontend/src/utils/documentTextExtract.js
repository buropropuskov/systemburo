/**
 * Извлечение текста из загруженного документа согласия (PDF/DOCX) в HTML для
 * редактора (#1567). Результат обязателен к вычитке администратором: извлечение
 * не воспроизводит таблицы, путает переносы по слогам и не знает про
 * автонумерацию Word. Поэтому это шаг импорта в правимое поле, а не источник
 * показа.
 *
 * Новых зависимостей не нужно: pdfjs-dist и jszip уже в бандле.
 */

/** Формат, для которого извлечения нет (например легаси .doc). */
export class UnsupportedDocumentError extends Error {
  constructor(message) {
    super(message);
    this.name = 'UnsupportedDocumentError';
  }
}

/** Строка выглядит как начало пункта списка: "1." / "2)" / "-" / "•". */
const LIST_ITEM_START = /^\s*(\d+[.)]|[-•–—*])\s+/;

/** Строка целиком номер страницы. */
const PAGE_NUMBER_ONLY = /^\s*[-–—]?\s*\d+\s*[-–—]?\s*$/;

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
 * Собирает строки страницы PDF из элементов текстового слоя. Разбиение по
 * hasEOL, который pdf.js выставляет на последнем элементе визуальной строки.
 * @param {Array<{str: string, hasEOL: boolean}>} items
 * @returns {string[]}
 */
function itemsToLines(items) {
  const lines = [];
  let current = '';
  for (const item of items) {
    current += item.str || '';
    if (item.hasEOL) {
      lines.push(current.trim());
      current = '';
    }
  }
  if (current.trim()) lines.push(current.trim());
  return lines;
}

/**
 * Находит колонтитулы: первая/последняя строка страницы, повторяющаяся на
 * большинстве страниц. Разовые совпадения не режем - на двух-трёх страницах
 * повтор может быть просто одинаковым началом абзаца.
 * @param {string[][]} pages
 * @returns {Set<string>}
 */
function detectRunningHeads(pages) {
  const heads = new Set();
  if (pages.length < 3) return heads;
  const counts = new Map();
  for (const lines of pages) {
    const candidates = new Set();
    if (lines.length) candidates.add(lines[0]);
    if (lines.length > 1) candidates.add(lines[lines.length - 1]);
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
 * Склеивает строки в абзацы: пустая строка разделяет абзацы, начало пункта
 * списка тоже открывает новый абзац (в юридическом тексте нумерованные пункты -
 * основная структура).
 * @param {string[]} lines
 * @returns {string[]}
 */
function linesToParagraphs(lines) {
  const paragraphs = [];
  let current = [];
  const flush = () => {
    if (current.length) {
      paragraphs.push(current.join(' '));
      current = [];
    }
  };
  for (const line of lines) {
    if (!line) {
      flush();
      continue;
    }
    if (LIST_ITEM_START.test(line)) flush();
    current.push(line);
  }
  flush();
  return paragraphs;
}

/**
 * Извлекает абзацы из PDF через текстовый слой pdf.js.
 * @param {Blob} blob
 * @returns {Promise<string[]>}
 */
async function extractPdfParagraphs(blob) {
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
    const lines = [];
    for (const pageLines of pages) {
      for (const line of pageLines) {
        if (runningHeads.has(line) || PAGE_NUMBER_ONLY.test(line)) continue;
        lines.push(line);
      }
      // Граница страницы разрывает абзац: продолжение через разрыв встречается
      // реже, чем новый абзац, и склеенный через страницы текст читать хуже.
      lines.push('');
    }
    return linesToParagraphs(lines);
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
 * Извлекает абзацы из DOCX: распаковываем word/document.xml и читаем w:p в
 * порядке документа (текст ячеек таблиц попадёт отдельными абзацами - таблицы
 * извлечение не воспроизводит).
 * @param {Blob} blob
 * @returns {Promise<Array<{text: string, listItem: boolean}>>}
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
  const blocks = [];
  for (const paragraph of Array.from(doc.getElementsByTagNameNS(WORD_NS, 'p'))) {
    const text = docxParagraphText(paragraph);
    if (!text) continue;
    blocks.push({
      text,
      listItem: paragraph.getElementsByTagNameNS(WORD_NS, 'numPr').length > 0,
    });
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
 * Собирает HTML из блоков: подряд идущие пункты списка группируются в ul.
 * Автонумерация Word в текст не входит (её рисует Word), поэтому у таких
 * пунктов номера теряются - это одна из причин вычитывать результат.
 * @param {Array<{text: string, listItem: boolean}>} blocks
 * @returns {string}
 */
function blocksToHtml(blocks) {
  const html = [];
  let inList = false;
  for (const block of blocks) {
    const text = escapeHtml(block.text);
    if (block.listItem) {
      if (!inList) {
        html.push('<ul>');
        inList = true;
      }
      html.push(`<li>${text}</li>`);
      continue;
    }
    if (inList) {
      html.push('</ul>');
      inList = false;
    }
    html.push(`<p>${text}</p>`);
  }
  if (inList) html.push('</ul>');
  return html.join('');
}

/**
 * Извлекает текст документа согласия в HTML для редактора.
 *
 * @param {Blob} blob содержимое документа
 * @param {string} ext расширение из метаданных документа (".pdf", ".docx", ".doc")
 * @returns {Promise<string>} HTML из абзацев и списков; пустая строка, если текста нет
 * @throws {UnsupportedDocumentError} для форматов без извлечения (.doc) и битых файлов
 */
export async function extractDocumentHtml(blob, ext) {
  if (!blob) throw new UnsupportedDocumentError('Документ не загружен.');
  const format = String(ext || '').toLowerCase().replace(/^\./, '');

  if (format === 'pdf') {
    const paragraphs = await extractPdfParagraphs(blob);
    return blocksToHtml(paragraphs.map((text) => ({ text, listItem: false })));
  }
  if (format === 'docx') {
    return blocksToHtml(await extractDocxBlocks(blob));
  }
  if (format === 'doc') {
    throw new UnsupportedDocumentError(
      'Из старого формата .doc текст не извлекается. Сохраните документ как DOCX или PDF либо вставьте текст вручную.',
    );
  }
  throw new UnsupportedDocumentError(`Извлечение текста из «${format || 'без расширения'}» не поддерживается.`);
}
