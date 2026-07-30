import { describe, it, expect, vi, beforeEach } from 'vitest';
import JSZip from 'jszip';
import ExcelJS from 'exceljs';
import { extractDocumentHtml, UnsupportedDocumentError } from '../documentTextExtract';

// pdf.js тянет воркер через ?worker-конструктор - в jsdom его нет, мок делает спек герметичным.
vi.mock('pdfjs-dist/build/pdf.worker.min.mjs?worker', () => ({ default: class {} }));

const getDocument = vi.fn();
vi.mock('pdfjs-dist', () => ({
  GlobalWorkerOptions: {},
  getDocument: (...args) => getDocument(...args),
}));

const LINE_HEIGHT = 14;
const NORMAL_LEAD = 16;

/** Левое поле и правый край текстовой колонки страницы. */
const COLUMN_LEFT = 50;
const COLUMN_RIGHT = 500;

/**
 * Страница PDF из строк. Строка - либо текст, либо объект со свойствами:
 * lead (расстояние до предыдущей строки, отбивка между абзацами больше типовой),
 * x/width (геометрия: по умолчанию строка занимает колонку целиком, как в
 * выключенном по формату документе), chunks (куски для проверки восстановления
 * пробелов по зазору).
 */
function pdfPage(lines) {
  const items = [];
  let y = 700;
  lines.forEach((line, index) => {
    const spec = typeof line === 'string' ? { text: line } : line;
    if (index > 0) y -= spec.lead ?? NORMAL_LEAD;
    const x = spec.x ?? COLUMN_LEFT;
    const width = spec.width ?? COLUMN_RIGHT - x;
    const chunks = spec.chunks || [{ str: spec.text, x, width }];
    chunks.forEach((chunk, chunkIndex) => {
      items.push({
        str: chunk.str,
        hasEOL: chunkIndex === chunks.length - 1,
        width: chunk.width,
        height: LINE_HEIGHT,
        transform: [1, 0, 0, 1, chunk.x, y],
      });
    });
  });
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
async function buildDocx(paragraphsXml, extraFiles = {}) {
  const zip = new JSZip();
  zip.file(
    'word/document.xml',
    '<?xml version="1.0" encoding="UTF-8"?>'
      + '<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">'
      + `<w:body>${paragraphsXml}</w:body></w:document>`,
  );
  for (const [name, content] of Object.entries(extraFiles)) zip.file(name, content);
  return zip.generateAsync({ type: 'blob' });
}

/** numbering.xml с одним списком: numId=1, формат задаётся параметром. */
function buildNumbering(format) {
  return '<?xml version="1.0" encoding="UTF-8"?>'
    + '<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">'
    + `<w:abstractNum w:abstractNumId="7"><w:lvl w:ilvl="0"><w:numFmt w:val="${format}"/></w:lvl></w:abstractNum>`
    + '<w:num w:numId="1"><w:abstractNumId w:val="7"/></w:num></w:numbering>';
}

const wp = (text) => `<w:p><w:r><w:t>${text}</w:t></w:r></w:p>`;
const wpList = (text) => '<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr>'
  + `<w:r><w:t>${text}</w:t></w:r></w:p>`;

async function buildXlsx(sheets) {
  const workbook = new ExcelJS.Workbook();
  for (const [name, rows] of Object.entries(sheets)) {
    const sheet = workbook.addWorksheet(name);
    rows.forEach((row) => sheet.addRow(row));
  }
  const buffer = await workbook.xlsx.writeBuffer();
  return new Blob([buffer]);
}

function blobOf(content) {
  return new Blob([content], { type: 'application/octet-stream' });
}

beforeEach(() => {
  getDocument.mockReset();
});

describe('extractDocumentHtml - PDF', () => {
  it('строки одного абзаца склеивает, увеличенная отбивка начинает новый', async () => {
    stubPdf([[
      'Согласие на обработку',
      'персональных данных.',
      { text: 'Я подтверждаю согласие.', lead: 30 },
    ]]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).toBe('<p>Согласие на обработку персональных данных.</p><p>Я подтверждаю согласие.</p>');
  });

  it('многоуровневый номер пункта открывает абзац без отбивки', async () => {
    stubPdf([[
      '1.1. В настоящей Политике используются термины:',
      '1.1.1. «Администрация сайта» - уполномоченные',
      'сотрудники, действующие от имени компании.',
      '1.1.2. «Персональные данные» - любая информация.',
    ]]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).toBe(
      '<p>1.1. В настоящей Политике используются термины:</p>'
      + '<p>1.1.1. «Администрация сайта» - уполномоченные сотрудники, действующие от имени компании.</p>'
      + '<p>1.1.2. «Персональные данные» - любая информация.</p>',
    );
  });

  it('строку прописными по центру выделяет заголовком второго уровня', async () => {
    stubPdf([[
      { text: 'ПОЛИТИКА КОНФИДЕНЦИАЛЬНОСТИ', x: 160, width: 230 },
      { text: 'г. Москва «02» марта 2022 г.', lead: 30 },
    ]]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).toBe('<h2>ПОЛИТИКА КОНФИДЕНЦИАЛЬНОСТИ</h2><p>г. Москва «02» марта 2022 г.</p>');
  });

  it('заголовок в две строки собирает в один', async () => {
    stubPdf([[
      { text: '4. СПОСОБЫ И СРОКИ ОБРАБОТКИ ПЕРСОНАЛЬНОЙ', x: 120, width: 260 },
      { text: 'ИНФОРМАЦИИ', x: 220, width: 60 },
      { text: '4.1. Обработка данных ведётся без ограничения срока.', lead: 32 },
    ]]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).toBe(
      '<h2>4. СПОСОБЫ И СРОКИ ОБРАБОТКИ ПЕРСОНАЛЬНОЙ ИНФОРМАЦИИ</h2>'
      + '<p>4.1. Обработка данных ведётся без ограничения срока.</p>',
    );
  });

  it('перечисление прописными во всю ширину строки заголовком не считает', async () => {
    // Иначе капслок внутри предложения («перечень категорий данных») разрывал бы
    // абзац натрое, вставляя лже-заголовок посередине.
    stubPdf([[
      'Оператор обрабатывает следующие данные:',
      'ФИО ПАСПОРТНЫЕ ДАННЫЕ АДРЕС РЕГИСТРАЦИИ',
      'а также сведения о месте работы субъекта.',
    ]]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).not.toContain('<h2>');
    expect(html).toBe(
      '<p>Оператор обрабатывает следующие данные: ФИО ПАСПОРТНЫЕ ДАННЫЕ АДРЕС РЕГИСТРАЦИИ'
      + ' а также сведения о месте работы субъекта.</p>',
    );
  });

  it('маркированные строки собирает в список, маркер убирает', async () => {
    stubPdf([[
      'Мы собираем:',
      { text: '• IP адрес;', width: 90 },
      { text: '• информация из cookies;', width: 150 },
      'Отключение cookies допустимо.',
    ]]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).toBe(
      '<p>Мы собираем:</p><ul><li>IP адрес;</li><li>информация из cookies;</li></ul>'
      + '<p>Отключение cookies допустимо.</p>',
    );
  });

  it('абзац, оборванный концом страницы, продолжается на следующей', async () => {
    stubPdf([
      ['1.1.7. «IP-адрес» - уникальный сетевой адрес узла в сети,'],
      ['построенной по протоколу IP.'],
    ]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).toBe('<p>1.1.7. «IP-адрес» - уникальный сетевой адрес узла в сети, построенной по протоколу IP.</p>');
  });

  it('законченное на странице предложение новую страницу не тянет', async () => {
    stubPdf([['Первый абзац закончился здесь.'], ['Второй абзац начался тут.']]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).toBe('<p>Первый абзац закончился здесь.</p><p>Второй абзац начался тут.</p>');
  });

  it('перенос по слогам сшивает без дефиса, а дефис в слове сохраняет', async () => {
    stubPdf([
      ['Данные подлежат надёжному хра-', 'нению и нераспространению.'],
      [{ text: 'Уникальный IP-', lead: 0 }, 'адрес узла в сети.'],
    ]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).toContain('надёжному хранению и нераспространению.');
    expect(html).toContain('Уникальный IP-адрес узла в сети.');
  });

  it('восстанавливает пробел между словами, если его нет в текстовом слое', async () => {
    stubPdf([[{
      chunks: [
        { str: 'используются', x: 50, width: 80 },
        { str: 'следующие', x: 134, width: 60 },
      ],
    }]]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).toBe('<p>используются следующие</p>');
  });

  it('не вставляет пробел там, где его нет и в разметке', async () => {
    stubPdf([[{
      chunks: [
        { str: 'Поли', x: 50, width: 25 },
        { str: 'тика', x: 75, width: 25 },
      ],
    }]]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).toBe('<p>Политика</p>');
  });

  it('выкидывает номера страниц и повторяющиеся колонтитулы', async () => {
    stubPdf([
      ['Политика обработки данных', 'Текст первой страницы.', '1'],
      ['Политика обработки данных', 'Текст второй страницы.', '2'],
      ['Политика обработки данных', 'Текст третьей страницы.', '3'],
    ]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).not.toContain('Политика обработки данных');
    expect(html).not.toMatch(/<p>\d+<\/p>/);
    expect(html).toBe('<p>Текст первой страницы.</p><p>Текст второй страницы.</p><p>Текст третьей страницы.</p>');
  });

  it('на двух страницах повтор не считает колонтитулом', async () => {
    stubPdf([['Общие положения.', 'Один.'], ['Общие положения.', 'Два.']]);

    const html = await extractDocumentHtml(blobOf('%PDF'), '.pdf');

    expect(html).toContain('Общие положения.');
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

  it('маркированный список Word даёт ul', async () => {
    const blob = await buildDocx(
      `${wp('Вводный абзац')}${wpList('Пункт один')}${wpList('Пункт два')}${wp('Заключение')}`,
      { 'word/numbering.xml': buildNumbering('bullet') },
    );

    const html = await extractDocumentHtml(blob, '.docx');

    expect(html).toBe('<p>Вводный абзац</p><ul><li>Пункт один</li><li>Пункт два</li></ul><p>Заключение</p>');
  });

  it('нумерованный список Word даёт ol, а не маркеры', async () => {
    const blob = await buildDocx(
      `${wpList('Первое условие')}${wpList('Второе условие')}`,
      { 'word/numbering.xml': buildNumbering('decimal') },
    );

    const html = await extractDocumentHtml(blob, '.docx');

    expect(html).toBe('<ol><li>Первое условие</li><li>Второе условие</li></ol>');
  });

  it('пункт со своим номером в тексте не получает второй номер от ol', async () => {
    const blob = await buildDocx(
      `${wpList('1.1. Первое условие')}${wpList('1.2. Второе условие')}`,
      { 'word/numbering.xml': buildNumbering('decimal') },
    );

    const html = await extractDocumentHtml(blob, '.docx');

    expect(html).toBe('<ul><li>1.1. Первое условие</li><li>1.2. Второе условие</li></ul>');
  });

  it('стиль заголовка Word превращает абзац в h2', async () => {
    const blob = await buildDocx(
      '<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Общие положения</w:t></w:r></w:p>'
      + wp('Текст раздела'),
    );

    const html = await extractDocumentHtml(blob, '.docx');

    expect(html).toBe('<h2>Общие положения</h2><p>Текст раздела</p>');
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

describe('extractDocumentHtml - XLSX', () => {
  it('строку листа переносит абзацем, ячейки склеивает', async () => {
    const blob = await buildXlsx({ Лист1: [['1.1.', 'Оператор обрабатывает данные.'], ['1.2.', 'Срок хранения - 5 лет.']] });

    const html = await extractDocumentHtml(blob, '.xlsx');

    expect(html).toBe('<p>1.1. Оператор обрабатывает данные.</p><p>1.2. Срок хранения - 5 лет.</p>');
  });

  it('пустые строки и ячейки пропускает', async () => {
    const blob = await buildXlsx({ Лист1: [['Первая строка.'], [], ['', '  '], ['Вторая строка.']] });

    const html = await extractDocumentHtml(blob, '.xlsx');

    expect(html).toBe('<p>Первая строка.</p><p>Вторая строка.</p>');
  });

  it('несколько листов разделяет заголовком с именем листа', async () => {
    const blob = await buildXlsx({ Согласие: [['Текст согласия.']], Приложение: [['Перечень данных.']] });

    const html = await extractDocumentHtml(blob, '.xlsx');

    expect(html).toBe('<h2>Согласие</h2><p>Текст согласия.</p><h2>Приложение</h2><p>Перечень данных.</p>');
  });

  it('единственный лист имя не показывает', async () => {
    const blob = await buildXlsx({ Лист1: [['Текст согласия.']] });

    const html = await extractDocumentHtml(blob, '.xlsx');

    expect(html).toBe('<p>Текст согласия.</p>');
  });

  it('маркированную строку превращает в пункт списка', async () => {
    const blob = await buildXlsx({ Лист1: [['Мы собираем:'], ['• IP адрес;'], ['• cookies;']] });

    const html = await extractDocumentHtml(blob, '.xlsx');

    expect(html).toBe('<p>Мы собираем:</p><ul><li>IP адрес;</li><li>cookies;</li></ul>');
  });

  it('числа и текст с разметкой приводит к строке', async () => {
    const workbook = new ExcelJS.Workbook();
    const sheet = workbook.addWorksheet('Лист1');
    sheet.addRow(['Срок хранения, лет', 5]);
    sheet.addRow([{ richText: [{ text: 'Согласие ' }, { text: 'обязательно' }] }]);
    const blob = new Blob([await workbook.xlsx.writeBuffer()]);

    const html = await extractDocumentHtml(blob, '.xlsx');

    expect(html).toBe('<p>Срок хранения, лет 5</p><p>Согласие обязательно</p>');
  });

  it('не-книгу отвергает с понятной ошибкой', async () => {
    await expect(extractDocumentHtml(blobOf('это просто текст'), '.xlsx'))
      .rejects.toThrow(UnsupportedDocumentError);
  });
});

describe('extractDocumentHtml - неподдержанные форматы', () => {
  it('легаси .doc объясняет, что делать', async () => {
    await expect(extractDocumentHtml(blobOf('doc'), '.doc')).rejects.toThrow(/DOCX, XLSX или PDF/);
  });

  it('прочие расширения отвергает', async () => {
    await expect(extractDocumentHtml(blobOf('x'), '.rtf')).rejects.toThrow(UnsupportedDocumentError);
  });

  it('без документа не падает необъяснимо', async () => {
    await expect(extractDocumentHtml(null, '.pdf')).rejects.toThrow(UnsupportedDocumentError);
  });
});
