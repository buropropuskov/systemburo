/**
 * Инварианты мобильной раскладки: пять правил, которые обязан соблюдать КАЖДЫЙ экран.
 *
 * Заведены после трёх волн правок #1097, где одни и те же дефекты возвращались, потому
 * что проверялись не тем инструментом и не там:
 *
 *  - прокрутку я мерил `window.scrollTo`, а он двигает документ МИМО жеста: палец
 *    достаётся ближайшему предку с `overflow: auto`, и на «Мои сотрудники» их было три
 *    подряд. Программная прокрутка показывала «работает», настоящий драг - `0 -> 0`;
 *  - внутренние области гасились по ИМЕНАМ классов, поэтому `.employees-body` и
 *    `.cars-body`, названные иначе, остались нетронутыми и дефект вернулся;
 *  - наложения бейджей вылезали только на длинных значениях, которых на стенде нет;
 *  - юнит-тесты раскладку не считают вовсе (jsdom), а их 4863 - и они молчали.
 *
 * Поэтому проверки идут в тач-контексте, реальным жестом и сразу по списку экранов:
 * новый экран добавляется одной строкой и получает все пять правил.
 */

/** Свой горизонтальный жест или своя прокрутка - элемент помечается атрибутом. */
const OWN_SCROLL_ATTR = 'data-scroll-own';

/**
 * Настоящий тач-драг через CDP. Playwright `mouse.wheel` и `window.scrollTo` для
 * мобильной прокрутки не годятся: первый - не тач, второй идёт мимо жеста.
 */
async function touchDrag(cdp, { x = 195, from = 700, to = 220, steps = 12 } = {}) {
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x, y: from }] });
  for (let i = 1; i <= steps; i += 1) {
    const y = from + ((to - from) * i) / steps;
    await cdp.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x, y }] });
    await new Promise((r) => setTimeout(r, 16));
  }
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
  await new Promise((r) => setTimeout(r, 600));
}

/**
 * Снимок позиций прокрутки: страница и все области под точкой касания.
 *
 * Проверять «нет ни одного предка с overflow: auto» оказалось слишком грубо - такой
 * контейнер, если он не переполнен и не зажат по высоте, жест не забирает, и гейт
 * ругался на здоровые экраны. Поэтому смотрим не на объявления, а на факт: кто
 * сдвинулся после настоящего драга. Забрал жест внутренний блок вместо страницы -
 * это и есть дефект, который владелец описывает как «скроллю экран, а он пытается
 * прокрутить список и стоит на месте».
 */
function scrollSnapshot(ownAttr) {
  const point = document.elementFromPoint(Math.round(innerWidth / 2), Math.round(innerHeight * 0.6));
  const inner = [];
  for (let el = point; el && el !== document.documentElement; el = el.parentElement) {
    if (ownAttr && el.closest(`[${ownAttr}]`)) break; // область заявила прокрутку своей
    const cs = getComputedStyle(el);
    if (cs.overflowY === 'auto' || cs.overflowY === 'scroll') {
      inner.push({ cls: (el.className || '').toString().slice(0, 40) || el.tagName, top: el.scrollTop });
    }
  }
  return { page: window.scrollY, inner };
}

/** Узлы, вылезающие за правый край страницы (внутренние ленты не в счёт). */
function horizontalOverflow() {
  const vw = innerWidth;
  const bad = [];
  const walk = (el) => {
    const r = el.getBoundingClientRect();
    if (r.width > 0 && r.right > vw + 0.5) bad.push(el);
    for (const c of el.children) walk(c);
  };
  walk(document.body);
  const deepest = bad.filter((el) => ![...el.children].some((c) => bad.includes(c)));
  return deepest
    .filter((el) => {
      for (let p = el.parentElement; p; p = p.parentElement) {
        const ox = getComputedStyle(p).overflowX;
        if (ox === 'auto' || ox === 'scroll') return false;
      }
      return true;
    })
    .slice(0, 6)
    .map((el) => ({
      cls: (el.className || '').toString().slice(0, 40) || el.tagName,
      right: Math.round(el.getBoundingClientRect().right),
      text: (el.innerText || '').replace(/\s+/g, ' ').slice(0, 30),
    }));
}

/**
 * Наложения внутри карточек: соседние по вертикали блоки не должны пересекаться.
 * Именно так вылезали бейджи «Доп №N», «Неактивен» и «На согласовании» - поверх
 * текста, когда тот переносился на вторую строку.
 */
function overlaps(cardSelector) {
  const found = [];
  for (const card of document.querySelectorAll(cardSelector)) {
    const kids = [...card.children].filter((c) => {
      const r = c.getBoundingClientRect();
      return r.width > 0 && r.height > 0 && getComputedStyle(c).position !== 'absolute';
    });
    for (let i = 0; i < kids.length; i += 1) {
      for (let j = i + 1; j < kids.length; j += 1) {
        const a = kids[i].getBoundingClientRect();
        const b = kids[j].getBoundingClientRect();
        const vertical = a.bottom > b.top + 1 && b.bottom > a.top + 1;
        const horizontal = a.right > b.left + 1 && b.right > a.left + 1;
        if (vertical && horizontal) {
          found.push({
            a: (kids[i].className || '').toString().slice(0, 24),
            b: (kids[j].className || '').toString().slice(0, 24),
            text: (kids[i].innerText || '').replace(/\s+/g, ' ').slice(0, 24),
          });
        }
      }
    }
  }
  return found.slice(0, 6);
}

/**
 * Кнопки и ссылки мельче нормы тач-таргета.
 *
 * Норма здесь 36, а не абстрактные 44: эталон §18 фиксирует 36x36 для компактных
 * контролов проекта (`.rt-btn-compact`, «Обновить», кнопки шапки приложения), и
 * гейт, ругающийся на принятую норму, отключат в первый же день. Ловим то, что
 * реально мельче: иконки на 24 и подобное.
 */
function smallTargets(min) {
  const out = [];
  for (const el of document.querySelectorAll('button, a[href], [role="button"]')) {
    const r = el.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) continue;
    const cs = getComputedStyle(el);
    if (cs.visibility === 'hidden') continue;
    // Элемент в середине анимации появления или ухода уже/ещё не в своём размере.
    if (/-(enter|leave)-(from|to|active)\b/.test(el.className || '')) continue;
    if (cs.opacity !== '' && parseFloat(cs.opacity) < 0.9) continue;
    // Невидимое расширение зоны через ::before - принятый в проекте приём: кнопка
    // выглядит компактной пилюлей, а палец попадает. Учитываем его.
    const before = getComputedStyle(el, '::before');
    const inset = parseFloat(before.top) || 0;
    const effective = r.height + Math.abs(inset) * 2;
    if (effective < min - 0.5) {
      out.push({
        cls: (el.className || '').toString().slice(0, 34),
        h: Math.round(r.height),
        effective: Math.round(effective),
        text: (el.innerText || '').replace(/\s+/g, ' ').slice(0, 20),
      });
    }
  }
  return out.slice(0, 8);
}

module.exports = { touchDrag, scrollSnapshot, horizontalOverflow, overlaps, smallTargets, OWN_SCROLL_ATTR };
