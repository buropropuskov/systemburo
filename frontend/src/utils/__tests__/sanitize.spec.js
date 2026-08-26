import { describe, it, expect } from 'vitest';
import { sanitizeHtml, stripHtml } from '../sanitize';

describe('sanitizeHtml', () => {
  it('пустой/falsy вход даёт пустую строку', () => {
    expect(sanitizeHtml('')).toBe('');
    expect(sanitizeHtml(null)).toBe('');
    expect(sanitizeHtml(undefined)).toBe('');
  });

  it('сохраняет атрибут width у картинки (механизм ресайза round-trip)', () => {
    const html = '<img class="constructor-image" src="data:image/png;base64,ZmFrZQ==" alt="x" width="320">';
    const out = sanitizeHtml(html);
    expect(out).toContain('width="320"');
    expect(out).toContain('class="constructor-image"');
    expect(out).toContain('data:image/png;base64,');
  });

  it('сохраняет атрибуты width и height у картинки (свободный ресайз round-trip)', () => {
    const html =
      '<img class="constructor-image" src="data:image/png;base64,ZmFrZQ==" alt="x" width="320" height="200">';
    const out = sanitizeHtml(html);
    expect(out).toContain('width="320"');
    expect(out).toContain('height="200"');
  });

  it('сохраняет класс выравнивания картинки img-align-* (round-trip обтекания)', () => {
    const html =
      '<img class="constructor-image img-align-right" src="data:image/png;base64,ZmFrZQ==" alt="x" width="320">';
    const out = sanitizeHtml(html);
    expect(out).toContain('img-align-right');
    expect(out).toContain('constructor-image');
    expect(out).toContain('width="320"');
  });

  it('сохраняет классы форматирования (цвета, размер, жирность)', () => {
    const html =
      '<span class="red-text">a</span>' +
      '<span class="font-size-18">b</span>' +
      '<span class="font-weight-600">c</span>';
    const out = sanitizeHtml(html);
    expect(out).toContain('red-text');
    expect(out).toContain('font-size-18');
    expect(out).toContain('font-weight-600');
  });

  it('сохраняет <strong> и классы выравнивания (bold + align round-trip)', () => {
    const html = '<p class="text-align-center"><strong>жирный по центру</strong></p>';
    const out = sanitizeHtml(html);
    expect(out).toContain('<strong>');
    expect(out).toContain('text-align-center');
  });

  it('вырезает скрипты и обработчики событий', () => {
    expect(sanitizeHtml('<script>alert(1)</script><p>ok</p>')).not.toContain('<script');
    expect(sanitizeHtml('<img src="x" onerror="alert(1)">')).not.toContain('onerror');
  });
});

describe('stripHtml', () => {
  it('пустой/falsy вход даёт пустую строку', () => {
    expect(stripHtml('')).toBe('');
    expect(stripHtml(null)).toBe('');
    expect(stripHtml(undefined)).toBe('');
  });

  it('вырезает теги rich-HTML из TextConstructor, оставляя только текст', () => {
    const html = '<h1 class="heading-h1"><strong>Проведение работ</strong></h1>';
    expect(stripHtml(html)).toBe('Проведение работ');
  });

  it('схлопывает переносы и лишние пробелы в один пробел (одна строка под ellipsis)', () => {
    const html = '<p>Первая строка</p>\n<p>вторая   строка</p>';
    expect(stripHtml(html)).toBe('Первая строка вторая строка');
  });

  it('декодирует HTML-сущности', () => {
    expect(stripHtml('<p>Иванов &amp; Партнёры &lt;груз&gt;</p>')).toBe('Иванов & Партнёры <груз>');
  });

  it('не исполняет скрипты и не тянет ресурсы (инертный DOMParser)', () => {
    // textContent <script> возвращает его тело как текст, но НЕ исполняет.
    const out = stripHtml('<div>ok</div><script>window.__pwned = 1</script>');
    expect(out).toContain('ok');
    expect(typeof window.__pwned).toBe('undefined');
  });
});
