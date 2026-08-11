/**
 * Обводка элементов на снимке экрана.
 *
 * Рисуется отдельным слоем поверх страницы, а не правкой стилей самого элемента:
 * правка border/outline у элемента двигает соседей и меняет то, что снимается,
 * слой же ничего не двигает и снимается ровно поверх нетронутой вёрстки.
 */

export const LAYER_ID = '__doc_shot_layer__';

/** Отступ обводки наружу от края элемента, CSS-пиксели. */
export const OUTLINE_OFFSET = 6;

/** Толщина линии обводки, CSS-пиксели. */
export const OUTLINE_WIDTH = 3;

/**
 * Оранжево-красный выбран намеренно: фирменный цвет системы - синий #4F5BDF,
 * и синяя обводка сливалась бы с активными элементами интерфейса.
 */
export const OUTLINE_COLOR = '#E8590C';

/** Диаметр кружка-выноски, CSS-пиксели. */
export const BADGE_SIZE = 34;

/**
 * Рисует обводки и выноски. Возвращает прямоугольники обводок и список
 * предупреждений - вызывающий решает, ронять ли на них прогон.
 *
 * @param {import('playwright').Page} page
 * @param {Array<{selector: string, badge?: number, nth?: number}>} targets
 * @returns {Promise<{boxes: Array<{left:number,top:number,width:number,height:number}>, warnings: string[]}>}
 */
export async function drawOutlines(page, targets) {
  return page.evaluate(
    ({ targets, LAYER_ID, OUTLINE_OFFSET, OUTLINE_WIDTH, OUTLINE_COLOR }) => {
      document.getElementById(LAYER_ID)?.remove();

      const layer = document.createElement('div');
      layer.id = LAYER_ID;
      Object.assign(layer.style, {
        position: 'fixed',
        left: '0',
        top: '0',
        width: '100%',
        height: '100%',
        zIndex: '2147483647',
        pointerEvents: 'none',
      });
      document.body.appendChild(layer);

      /*
       * Радиус угла в computed style бывает "15px", "50%" и парой значений
       * "50% 25%" (горизонтальная и вертикальная полуоси). Проценты считаются
       * от ширины для горизонтальной полуоси и от высоты для вертикальной,
       * поэтому без разворота в пиксели круглая кнопка обводится эллипсом.
       */
      const resolveRadius = (raw, width, height) => {
        const parts = String(raw || '0px').trim().split(/\s+/);
        const toPx = (value, base) =>
          value.endsWith('%') ? (parseFloat(value) / 100) * base : parseFloat(value) || 0;
        const x = toPx(parts[0], width);
        const y = toPx(parts[1] ?? parts[0], height);
        return [x, y];
      };

      const boxes = [];
      const warnings = [];

      for (const target of targets) {
        const nodes = document.querySelectorAll(target.selector);
        const element = nodes[target.nth ?? 0];
        if (!element) {
          warnings.push(`цель не найдена: ${target.selector}`);
          continue;
        }

        const rect = element.getBoundingClientRect();
        if (rect.width === 0 || rect.height === 0) {
          warnings.push(`цель нулевого размера: ${target.selector}`);
          continue;
        }

        const style = getComputedStyle(element);
        const corners = [
          'borderTopLeftRadius',
          'borderTopRightRadius',
          'borderBottomRightRadius',
          'borderBottomLeftRadius',
        ].map((prop) => resolveRadius(style[prop], rect.width, rect.height));

        /*
         * Эквидистанта скруглённого прямоугольника: наружная кривая отстоит от
         * внутренней на постоянное расстояние, значит каждая полуось растёт
         * ровно на величину отступа. У прямого угла (радиус 0) наружный угол
         * скругляется на величину отступа - это и есть верный контур, а не
         * «скругление на глаз».
         */
        const grown = corners.map(([x, y]) => [x + OUTLINE_OFFSET, y + OUTLINE_OFFSET]);

        const box = {
          left: rect.left - OUTLINE_OFFSET - OUTLINE_WIDTH,
          top: rect.top - OUTLINE_OFFSET - OUTLINE_WIDTH,
          width: rect.width + 2 * (OUTLINE_OFFSET + OUTLINE_WIDTH),
          height: rect.height + 2 * (OUTLINE_OFFSET + OUTLINE_WIDTH),
        };

        const outline = document.createElement('div');
        Object.assign(outline.style, {
          position: 'fixed',
          left: `${box.left}px`,
          top: `${box.top}px`,
          width: `${rect.width + 2 * OUTLINE_OFFSET}px`,
          height: `${rect.height + 2 * OUTLINE_OFFSET}px`,
          border: `${OUTLINE_WIDTH}px solid ${OUTLINE_COLOR}`,
          borderRadius: grown
            .map(([x]) => `${x}px`)
            .join(' ')
            .concat(' / ', grown.map(([, y]) => `${y}px`).join(' ')),
          boxSizing: 'content-box',
          boxShadow: `0 0 0 3px rgba(232, 89, 12, 0.18)`,
        });
        layer.appendChild(outline);

        boxes.push({ ...box, badge: target.badge ?? null });
      }

      return { boxes, warnings };
    },
    { targets, LAYER_ID, OUTLINE_OFFSET, OUTLINE_WIDTH, OUTLINE_COLOR },
  );
}

/**
 * Ставит кружки-выноски. Вызывается ПОСЛЕ вычисления кадра: выноска обязана
 * целиком лежать внутри кадра, иначе получится срезанный краем кружок - ровно
 * то, чем испорчены руководства прошлого поколения.
 *
 * @param {import('playwright').Page} page
 * @param {Array<{left:number,top:number,width:number,height:number,badge:number|null}>} boxes
 * @param {{x:number,y:number,width:number,height:number}} clip
 * @returns {Promise<string[]>} предупреждения
 */
export async function drawBadges(page, boxes, clip) {
  return page.evaluate(
    ({ boxes, clip, LAYER_ID, OUTLINE_COLOR, BADGE_SIZE }) => {
      const layer = document.getElementById(LAYER_ID);
      if (!layer) return ['слой обводки не найден'];

      const warnings = [];
      const half = BADGE_SIZE / 2;
      const gap = 4;
      const placedBadges = [];

      const insideClip = (cx, cy) =>
        cx - half >= clip.x + gap &&
        cy - half >= clip.y + gap &&
        cx + half <= clip.x + clip.width - gap &&
        cy + half <= clip.y + clip.height - gap;

      const overlapsRect = (cx, cy, rect) =>
        cx + half > rect.left - gap &&
        cx - half < rect.left + rect.width + gap &&
        cy + half > rect.top - gap &&
        cy - half < rect.top + rect.height + gap;

      /*
       * Кружок обязан лечь мимо всех обведённых элементов, а не только мимо
       * своего: в ряду соседних кнопок место «слева от третьей» - это ровно
       * вторая кнопка. Мимо уже поставленных кружков - тоже, иначе два номера
       * слипаются в один нечитаемый ком.
       */
      const free = (cx, cy) =>
        insideClip(cx, cy) &&
        !boxes.some((other) => overlapsRect(cx, cy, other)) &&
        !placedBadges.some(([px, py]) => Math.hypot(px - cx, py - cy) < BADGE_SIZE + gap);

      for (const box of boxes) {
        if (box.badge === null) continue;

        const midX = box.left + box.width / 2;
        const midY = box.top + box.height / 2;
        const outer = half + gap;

        // Сверху и снизу - первыми: в интерфейсе элементы чаще стоят рядами,
        // и свободное место оказывается над рядом или под ним, а не между
        // соседями.
        const candidates = [
          [midX, box.top - outer],
          [midX, box.top + box.height + outer],
          [box.left - outer, midY],
          [box.left + box.width + outer, midY],
          [box.left - outer, box.top - outer],
          [box.left + box.width + outer, box.top - outer],
          [box.left - outer, box.top + box.height + outer],
          [box.left + box.width + outer, box.top + box.height + outer],
        ];

        let placed = candidates.find(([cx, cy]) => free(cx, cy));
        if (!placed) {
          /*
           * Внутрь элемента кружок не ставим: он перекроет содержимое, ради
           * которого кадр и снимается. Кадру не хватает воздуха - лечится
           * увеличением pad в манифесте либо разбиением на два кадра.
           */
          warnings.push(`выноска ${box.badge}: некуда поставить, не задев соседей - увеличьте pad`);
          placed = [
            Math.min(Math.max(midX, clip.x + outer), clip.x + clip.width - outer),
            Math.min(Math.max(box.top - outer, clip.y + outer), clip.y + clip.height - outer),
          ];
        }

        placedBadges.push(placed);
        const [cx, cy] = placed;
        const badge = document.createElement('div');
        badge.textContent = String(box.badge);
        Object.assign(badge.style, {
          position: 'fixed',
          left: `${cx - half}px`,
          top: `${cy - half}px`,
          width: `${BADGE_SIZE}px`,
          height: `${BADGE_SIZE}px`,
          borderRadius: '50%',
          background: OUTLINE_COLOR,
          color: '#FFFFFF',
          font: `700 ${Math.round(BADGE_SIZE * 0.55)}px/1 "Arial", sans-serif`,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          boxShadow: '0 2px 6px rgba(0, 0, 0, 0.28)',
        });
        layer.appendChild(badge);
      }

      return warnings;
    },
    { boxes, clip, LAYER_ID, OUTLINE_COLOR, BADGE_SIZE },
  );
}

/** Снимает слой обводки, чтобы страница вернулась в исходный вид. */
export async function clearOutlines(page) {
  await page.evaluate((id) => document.getElementById(id)?.remove(), LAYER_ID);
}
