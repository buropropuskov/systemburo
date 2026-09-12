/**
 * Замеры раскладки, общие для мобилки, планшета и десктопа.
 *
 * Дополняют `mobileInvariants.cjs` (тот заточен под тач-жест и мобильный экран) тем,
 * что нужно при обходе всех ширин сразу: сводка по странице и поиск строк, которые
 * шире своего контейнера.
 *
 * Все функции исполняются в браузере через `page.evaluate`, поэтому не имеют внешних
 * зависимостей и замыканий.
 */

/** Сводка по документу: ширина прокрутки против вьюпорта и корневой zoom. */
function pageMetrics() {
  const de = document.documentElement;
  return {
    doc: de.scrollWidth,
    vw: window.innerWidth,
    scrollable: de.scrollHeight - de.clientHeight > 40,
    zoom: parseFloat(getComputedStyle(de).zoom) || 1,
  };
}

/**
 * Строки/карточки шире своего контейнера.
 *
 * Отдельная проверка от `horizontalOverflow`, потому что переполнение внутри блока с
 * `overflow: hidden` (а `.rt-table` именно такой) не утекает на документ: страница
 * горизонтально не прокручивается, гейт зелёный, а содержимое строки обрезано. Так
 * уже ловили таблицу с неразрывным словом - 560px при вьюпорте 390.
 */
function oversizedRows(rowSelector) {
  const out = [];
  for (const row of document.querySelectorAll(rowSelector)) {
    const parent = row.parentElement;
    if (!parent) continue;
    const rw = row.scrollWidth;
    const pw = parent.clientWidth;
    if (pw > 0 && rw > pw + 1) {
      out.push({
        cls: (row.className || '').toString().slice(0, 40),
        row: Math.round(rw),
        parent: Math.round(pw),
      });
    }
  }
  return out.slice(0, 5);
}

/**
 * Текст, обрезанный по ширине без многоточия: содержимое шире своей ячейки, а
 * `text-overflow` не задан - значение просто пропадает из виду. На узком экране это
 * читается как «данные потерялись», а не как «не поместилось».
 */
function clippedText(root) {
  const out = [];
  const scope = root ? document.querySelectorAll(root) : [document.body];
  for (const box of scope) {
    for (const el of box.querySelectorAll('*')) {
      if (el.children.length) continue;
      const text = (el.textContent || '').trim();
      if (text.length < 4) continue;
      const cs = getComputedStyle(el);
      if (cs.overflow === 'visible' || cs.textOverflow === 'ellipsis') continue;
      // Подпись, спрятанная clip-приёмом ради скринридера (`.rt-btn-label`,
      // `.action-btn__label`), - не обрезанный текст, а намеренно невидимый узел.
      if (el.clientWidth <= 2 || cs.clip === 'rect(0px, 0px, 0px, 0px)') continue;
      if (el.scrollWidth > el.clientWidth + 2 && cs.whiteSpace === 'nowrap') {
        out.push({ cls: (el.className || '').toString().slice(0, 30), text: text.slice(0, 24) });
      }
    }
  }
  return out.slice(0, 5);
}

module.exports = { pageMetrics, oversizedRows, clippedText };
