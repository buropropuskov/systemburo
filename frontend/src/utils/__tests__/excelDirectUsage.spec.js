import { describe, it, expect } from 'vitest';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';

/**
 * Храповик на прямое обращение к ExcelJS (#2418).
 *
 * Стиль листа был скопирован в двадцати шести файлах, и правка вида требовала обойти их
 * все - на деле не обходили, и выгрузки расходились. Теперь лист рисует общий
 * `utils/excelSheet.js`, а этот список отслеживает, кто ещё не переведён: новый файл с
 * собственной книгой добавить незаметно нельзя, а перевод обязан вычеркнуть строку.
 *
 * Список нужен строгим равенством, а не порогом: порог разрешил бы «перевёл один, завёл
 * другой» и не показывал бы, что именно осталось.
 */
const SRC = join(process.cwd(), 'src');

/** Файлы, которым своя книга нужна по делу. */
const ALLOWED = [
  // Рисует лист - это и есть общая реализация.
  'utils/excelSheet.js',
  // Читают чужой файл, а не собирают свой: просмотрщик вложения и разбор текста.
  'components/admin/XlsxViewer.vue',
  'utils/documentTextExtract.js',
  // Две служебные таблицы без оформления: список ошибок разбора бланка и перечень тех,
  // кто не подтвердил согласие. У них нет ни чередования строк, ни рамок, ни подписи
  // «кто сформировал» - это рабочие списки на один взгляд, а не отчёты наружу. Общий лист
  // нарисовал бы им чередование и рамки, то есть изменил бы файл, который человек уже
  // привык видеть, - поэтому они остаются со своей книгой осознанно.
  'components/CreateApplication/BlankImportResult.vue',
  'components/admin/DataProcessingSettings.vue',
];

/**
 * Ещё не переведённые на общий лист. Список пуст: #2418 закрыт, и теперь проверка работает
 * как запрет - своя книга допустима только у перечисленных выше файлов.
 */
const PENDING = [];

function sourceFiles(dir) {
  const found = [];
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      if (entry === '__tests__') continue;
      found.push(...sourceFiles(full));
      continue;
    }
    if (/\.(vue|js)$/.test(entry) && !/\.spec\.js$/.test(entry)) found.push(full);
  }
  return found;
}

describe('прямое обращение к ExcelJS (#2418)', () => {
  const withOwnWorkbook = sourceFiles(SRC)
    .filter(file => readFileSync(file, 'utf8').includes('new ExcelJS.Workbook()'))
    .map(file => relative(SRC, file).split(/[\\/]/).join('/'))
    .sort();

  it('свою книгу собирают только перечисленные файлы', () => {
    expect(withOwnWorkbook).toEqual([...ALLOWED, ...PENDING].sort());
  });

  it('перевод на общий лист обязан вычеркнуть файл из списка', () => {
    const stale = PENDING.filter(file => !withOwnWorkbook.includes(file));
    expect(stale).toEqual([]);
  });
});
