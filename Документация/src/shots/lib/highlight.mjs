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

        /*
         * Поле ввода часто прозрачно и без скругления: заливка и радиус лежат на
         * обёртке со значком и отступами. Линия по самому input выходит
         * прямоугольником поперёк круглого поля и режет значок - замечание
         * владельца по странице входа (31.08.2026). Разово это не лечится:
         * кандидаты рассыпаны по всем манифестам, поэтому предупреждаем на съёмке.
         */
        const parent = element.parentElement;
        if (parent) {
            const parentStyle = getComputedStyle(parent);
            const parentRect = parent.getBoundingClientRect();
            const invisible = (cs) =>
              (cs.backgroundColor === 'rgba(0, 0, 0, 0)' || cs.backgroundColor === 'transparent') &&
              parseFloat(cs.borderTopLeftRadius) === 0;
            /*
             * Ругаемся только на обёртку, плотно облегающую цель: у поля ввода
             * это рамка со значком, и разница площадей невелика. Прозрачная
             * секция внутри большой карточки - обычное дело, её обводят
             * намеренно, и предупреждение там только зашумляет прогон.
             */
            const area = rect.width * rect.height;
            const parentArea = parentRect.width * parentRect.height;
            const snug = parentArea > 0 && parentArea < area * 1.6;
            // Правило про поля ввода: у секции или строки списка своя причина
            // быть прозрачной, и предупреждение там только шумит.
            // Только само поле ввода: блок с чекбоксом и пояснением обводят
            // целиком намеренно, и линия по нему верна.
            const isField = element.matches('input, textarea, select');
            if (isField && snug && invisible(style) && !invisible(parentStyle)) {
              warnings.push(
                `${target.selector}: обводится элемент без заливки и скругления, ` +
                'а они есть у его обёртки - линия ляжет не по видимому полю',
              );
            }
        }

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
         *
         * Область, прижатая к краю окна (боковое меню, верхняя строка), наружную
         * линию за край не вмещает, и на снимке она выходит обрезанной с одной
         * стороны. Для таких целей линия уводится внутрь: полуоси на ту же
         * величину уменьшаются, контур остаётся эквидистантой.
         */
        const inward = target.inset === true ? -1 : 1;
        const offset = OUTLINE_OFFSET * inward;
        const grown = corners.map(([x, y]) => [
          Math.max(0, x + offset),
          Math.max(0, y + offset),
        ]);

        const box = {
          left: rect.left - offset - OUTLINE_WIDTH,
          top: rect.top - offset - OUTLINE_WIDTH,
          width: rect.width + 2 * (offset + OUTLINE_WIDTH),
          height: rect.height + 2 * (offset + OUTLINE_WIDTH),
        };

        const outline = document.createElement('div');
        Object.assign(outline.style, {
          position: 'fixed',
          left: `${box.left}px`,
          top: `${box.top}px`,
          width: `${rect.width + 2 * offset}px`,
          height: `${rect.height + 2 * offset}px`,
          border: `${OUTLINE_WIDTH}px solid ${OUTLINE_COLOR}`,
          borderRadius: grown
            .map(([x]) => `${x}px`)
            .join(' ')
            .concat(' / ', grown.map(([, y]) => `${y}px`).join(' ')),
          boxSizing: 'content-box',
          boxShadow: `0 0 0 3px rgba(232, 89, 12, 0.18)`,
        });
        layer.appendChild(outline);

        boxes.push({
          ...box,
          badge: target.badge ?? null,
          badgeInside: target.badgeInside === true,
          // Прямоугольник самого элемента: при посадке номера внутрь он не
          // считается занятым местом. Поле ввода целиком числится занятым как
          // элемент ввода, и без этого исключения номер внутрь него не встаёт
          // вовсе - даже в пустую правую половину.
          own: { left: rect.left, top: rect.top, width: rect.width, height: rect.height },
        });
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

      /*
       * Занято ли место - выясняется попаданием в точку, а не перебором
       * прямоугольников. Перебор считает занятым и то, что лежит под наложенной
       * панелью: кнопка шапки продолжает занимать свои координаты, хотя поверх
       * неё выехало окно поиска, и свободного места «не находилось» там, где
       * оно на самом деле есть. Попадание в точку возвращает только верхний
       * слой, поэтому перекрытое им не мешает.
       *
       * Слой обводки прозрачен для попаданий, так что сам себя не заслоняет.
       */
      const busyAt = (x, y, own) => {
        const element = document.elementFromPoint(x, y);
        if (!element) return false;
        if (own) {
          const rect = element.getBoundingClientRect();
          const same =
            Math.abs(rect.left - own.left) < 1 &&
            Math.abs(rect.top - own.top) < 1 &&
            Math.abs(rect.width - own.width) < 1 &&
            Math.abs(rect.height - own.height) < 1;
          if (same) return false;
        }
        /*
         * Текст ищется в собственных узлах элемента, а не в его потомках:
         * подпись поля лежит в теге, внутри которого есть ещё и звёздочка
         * обязательности, - по признаку «нет потомков» такая подпись читалась
         * как свободное место, и кружок садился прямо на слово.
         */
        const bearsText = Array.from(element.childNodes).some(
          (node) => node.nodeType === 3 && (node.textContent || '').trim().length > 0,
        );
        const bearsPicture = ['IMG', 'INPUT', 'TEXTAREA', 'SELECT'].includes(element.tagName) ||
          element.namespaceURI === 'http://www.w3.org/2000/svg';
        return bearsText || bearsPicture;
      };

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

      // Круг прощупывается центром и восемью точками по окружности: одной точки
      // мало, кружок задевал бы соседний текст краем.
      const RING = Array.from({ length: 8 }, (_, index) => {
        const angle = (index * Math.PI) / 4;
        return [Math.cos(angle), Math.sin(angle)];
      });

      const overlapsContent = (cx, cy, own) =>
        busyAt(cx, cy, own) ||
        RING.some(([dx, dy]) => busyAt(cx + dx * (half + gap), cy + dy * (half + gap), own));

      /*
       * Кружок обязан лечь мимо всех обведённых элементов, а не только мимо
       * своего: в ряду соседних кнопок место «слева от третьей» - это ровно
       * вторая кнопка. Мимо уже поставленных кружков - тоже, иначе два номера
       * слипаются в один нечитаемый ком.
       */
      const noBadgeNearby = (cx, cy) =>
        !placedBadges.some(([px, py]) => Math.hypot(px - cx, py - cy) < BADGE_SIZE + gap);

      const free = (cx, cy) =>
        insideClip(cx, cy) &&
        !boxes.some((other) => overlapsRect(cx, cy, other)) &&
        !overlapsContent(cx, cy) &&
        noBadgeNearby(cx, cy);

      for (const box of boxes) {
        if (box.badge === null) continue;

        const midX = box.left + box.width / 2;
        const midY = box.top + box.height / 2;
        const outer = half + gap;

        /*
         * Обзорный кадр показывает области экрана целиком, и они занимают его
         * без остатка: снаружи такой области места нет ни с одной стороны.
         * Кружок внутри крупной области ничего не заслоняет - в отличие от
         * кружка внутри кнопки, - поэтому для них место ищется по внутренним
         * углам. Признак задаётся в манифесте явно, сам снимальщик не решает.
         */
        const candidates = box.badgeInside
          ? [
              /*
               * Кружок садится ровно на угол обводки: половина ложится наружу,
               * в промежуток между блоками, половина - на внутренний отступ
               * блока. Целиком внутри он наезжал бы на заголовок, целиком
               * снаружи - на соседний блок.
               */
              [box.left, box.top],
              [box.left + box.width, box.top],
              [box.left, box.top + box.height],
              [box.left + box.width, box.top + box.height],
              /*
               * Углов и середины мало для вытянутой области: у верхней строки
               * все четыре угла заняты значками, а свободный промежуток лежит
               * между приветствием и часами. Поэтому область прощупывается ещё
               * и вдоль осей - справа налево.
               *
               * Порядок именно такой из-за полей ввода: набранный в поле текст
               * не отдельный элемент, попаданием в точку его не видно, и слева
               * кружок сел бы прямо на него. Текст прижат влево всегда, значит
               * справа безопаснее.
               */
              ...[0.85, 0.7, 0.5, 0.3, 0.15].flatMap((part) => [
                [box.left + box.width * part, midY],
                [midX, box.top + box.height * part],
              ]),
            ]
          : [
              // Сверху и снизу - первыми: в интерфейсе элементы чаще стоят
              // рядами, и свободное место оказывается над рядом или под ним,
              // а не между соседями.
              [midX, box.top - outer],
              [midX, box.top + box.height + outer],
              [box.left - outer, midY],
              [box.left + box.width + outer, midY],
              [box.left - outer, box.top - outer],
              [box.left + box.width + outer, box.top - outer],
              [box.left - outer, box.top + box.height + outer],
              [box.left + box.width + outer, box.top + box.height + outer],
            ];

        // У кружка на углу блока свои правила: он намеренно ложится поверх
        // собственной обводки, поэтому пересечение с рамками не проверяется -
        // но текст он закрывать не должен так же, как и всякий другой.
        const fits = box.badgeInside
          ? (cx, cy) => insideClip(cx, cy) && !overlapsContent(cx, cy, box.own) && noBadgeNearby(cx, cy)
          : free;

        let placed = candidates.find(([cx, cy]) => fits(cx, cy));
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
