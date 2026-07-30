import { describe, it, expect, vi, beforeEach } from 'vitest';
import JSZip from 'jszip';
import { extractDocumentHtml, UnsupportedDocumentError } from '../documentTextExtract';

// pdf.js тянет воркер через ?worker-конструктор - в jsdom его нет, мок делает спек герметичным.
vi.mock('pdfjs-dist/build/pdf.worker.min.mjs?worker', () => ({ default: class {} }));

const getDocument = vi.fn();
vi.mock('pdfjs-dist', () => ({
  GlobalWorkerOptions: {},
  getDocument: (...args) => getDocument(...args),
}));

/** Страница PDF из готовых строк: pdf.js помечает конец строки флагом hasEOL. */
function pdfPage(lines) {
  const items = [];
  for (const line of lines) {
    items.push({ str: line, hasEOL: true });
  }
  return { getTextContent: async () => ({ items }), cleanup: vi.fn() };
}

function stubPdf(pages) {
  getDocument.mockReturnValue({
    promise: Promise.resolve({
      numPages: pages.length,
      getPage: async (n) => pdfPage(pages[n - 1]),
      destroy: vi.fn(),
    }),
  });
}

/** Минимальный DOCX: только word/document.xml, этого хватает для извлечения. */
async function buildDocx(paragraphsXml) {
  const zip = new JSZip();
  zip.file(
    'word/document.xml',
    '<?xml version="1.0" encoding="UTF-8"?>'
      + '<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">'
      + `<w:body>${paragraphsXml}</w:body></w:document>`,
  );
  return zip.generateAsync({ type: 'blob' });
}

const wp = (text) => `<w:p><w:r><w:t>${text}</w:t></w:r></w:p>`;
const wpList = (text) => `<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/></w:numPr></w:pPr><w:r><w:t>${text}</w:t></w:r></w:p>`;

function blobOf(content) {
  return new Blob([content], { type: 'application/octet-stream' });
}

beforeEach(() => {
  getDocument.mockReset();
});

describe('extractDocumentHtml - PDF', () => {
  it('склеивает строки в абзацы, разделяя их пустой строкой', async () => {
    stubPdf([['Согласие на обработку', 'персональных данных', '', 'Я подтверждаю согласие.']]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).toBe('<p>Согласие на обработку персональных данных</p><p>Я подтверждаю согласие.</p>');
  });

  it('нумерованный пункт открывает новый абзац без пустой строки', async () => {
    stubPdf([['1. Первый пункт', '2. Второй пункт']]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).toBe('<p>1. Первый пункт</p><p>2. Второй пункт</p>');
  });

  it('выкидывает номера страниц и повторяющиеся колонтитулы', async () => {
    stubPdf([
      ['Политика обработки данных', 'Текст первой страницы', '1'],
      ['Политика обработки данных', 'Текст второй страницы', '2'],
      ['Политика обработки данных', 'Текст третьей страницы', '3'],
    ]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).not.toContain('Политика обработки данных');
    expect(html).not.toMatch(/<p>\d+<\/p>/);
    expect(html).toBe('<p>Текст первой страницы</p><p>Текст второй страницы</p><p>Текст третьей страницы</p>');
  });

  it('на двух страницах повтор не считает колонтитулом', async () => {
    stubPdf([['Общие положения', 'Один'], ['Общие положения', 'Два']]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).toContain('Общие положения');
  });

  it('экранирует угловые скобки из текста документа', async () => {
    stubPdf([['Оператор <ООО "Бюро"> & партнёры']]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).toBe('<p>Оператор &lt;ООО "Бюро"&gt; &amp; партнёры</p>');
  });

  it('документ без текстового слоя даёт пустую строку, а не мусор', async () => {
    stubPdf([[]]);

    expect(await extractDocumentHtml(blobOf('%PDF'), '.pdf')).toBe('');
  });

  it('битый или запароленный PDF объясняет причину по-русски', async () => {
    getDocument.mockReturnValue({ promise: Promise.reject(new Error('PasswordException')) });

    await expect(extractDocumentHtml(blobOf('broken'), '.pdf'))
      .rejects.toThrow(UnsupportedDocumentError);
    await expect(extractDocumentHtml(blobOf('broken'), '.pdf'))
      .rejects.toThrow(/Не удалось прочитать PDF/);
  });
});

describe('extractDocumentHtml - DOCX', () => {
  it('переносит абзацы, пустые пропускает', async () => {
    const blob = await buildDocx(`${wp('Первый абзац')}<w:p/>${wp('Второй абзац')}`);

    const html = await extractDocumentHtml(blob, '.docx');

    expect(html).toBe('<p>Первый абзац</p><p>Второй абзац</p>');
  });

  it('подряд идущие пункты с нумерацией группирует в список', async () => {
    const blob = await buildDocx(`${wp('Вводный абзац')}${wpList('Пункт один')}${wpList('Пункт два')}${wp('Заключение')}`);

    const html = await extractDocumentHtml(blob, '.docx');

    expect(html).toBe('<p>Вводный абзац</p><ul><li>Пункт один</li><li>Пункт два</li></ul><p>Заключение</p>');
  });

  it('табуляции и переносы внутри абзаца сводит к пробелам', async () => {
    const blob = await buildDocx(
      '<w:p><w:r><w:t>Оператор</w:t><w:tab/><w:t>Бюро</w:t><w:br/><w:t>пропусков</w:t></w:r></w:p>',
    );

    const html = await extractDocumentHtml(blob, '.docx');

    expect(html).toBe('<p>Оператор Бюро пропусков</p>');
  });

  it('архив без word/document.xml отвергает с понятной ошибкой', async () => {
    const zip = new JSZip();
    zip.file('hello.txt', 'not a docx');
    const blob = await zip.generateAsync({ type: 'blob' });

    await expect(extractDocumentHtml(blob, '.docx')).rejects.toThrow(UnsupportedDocumentError);
  });

  it('не-архив отвергает с понятной ошибкой', async () => {
    await expect(extractDocumentHtml(blobOf('это просто текст'), '.docx'))
      .rejects.toThrow(UnsupportedDocumentError);
  });
});

describe('extractDocumentHtml - неподдержанные форматы', () => {
  it('легаси .doc объясняет, что делать', async () => {
    await expect(extractDocumentHtml(blobOf('doc'), '.doc')).rejects.toThrow(/DOCX или PDF/);
  });

  it('прочие расширения отвергает', async () => {
    await expect(extractDocumentHtml(blobOf('x'), '.rtf')).rejects.toThrow(UnsupportedDocumentError);
  });

  it('без документа не падает необъяснимо', async () => {
    await expect(extractDocumentHtml(null, '.pdf')).rejects.toThrow(UnsupportedDocumentError);
  });
});
