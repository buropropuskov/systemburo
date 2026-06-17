import { Extension, Mark, mergeAttributes } from '@tiptap/core';
import { VueNodeViewRenderer } from '@tiptap/vue-3';
import Heading from '@tiptap/extension-heading';
import Image from '@tiptap/extension-image';
import ResizableImage from './ResizableImage.vue';

/**
 * Класс-ориентированные марки TextConstructor.
 *
 * Существующий контент (инструкции к таблицам, новости, объявления) сохранён в БД как HTML
 * с CSS-классами: `<span class="red-text">`, `<span class="font-size-14">`,
 * `<span class="font-weight-600">`, `<h1 class="heading-h1">`, `<img class="constructor-image">`.
 * Потребители рендерят его через v-html с :deep(.red-text)-стилями. Чтобы переход на Tiptap
 * не ломал уже сохранённый контент (round-trip), эти марки парсят и рендерят те же классы 1:1.
 */

export const COLOR_CLASSES = ['black-text', 'red-text', 'green-text', 'blue-text'];
export const FONT_SIZE_CLASSES = [
  'font-size-10',
  'font-size-12',
  'font-size-14',
  'font-size-16',
  'font-size-18',
  'font-size-20',
];
export const FONT_WEIGHT_CLASSES = [
  'font-weight-300',
  'font-weight-400',
  'font-weight-500',
  'font-weight-600',
  'font-weight-900',
];
export const TEXT_ALIGN_TYPES = ['paragraph', 'heading'];
export const TEXT_ALIGNMENTS = ['left', 'center', 'right'];
export const IMAGE_ALIGNMENTS = ['left', 'center', 'right'];

/**
 * Фабрика марки, которая хранит один класс из набора на `<span>` и round-trip'ит его.
 * @param {object} cfg
 * @param {string} cfg.name имя марки
 * @param {string} cfg.attr имя атрибута, где лежит класс
 * @param {string[]} cfg.classes допустимый набор классов
 * @param {string} cfg.setCommand имя команды установки
 * @param {string} cfg.unsetCommand имя команды снятия
 */
function createClassMark({ name, attr, classes, setCommand, unsetCommand }) {
  return Mark.create({
    name,

    addAttributes() {
      return {
        [attr]: {
          default: null,
          parseHTML: (element) => classes.find((cls) => element.classList.contains(cls)) || null,
          renderHTML: (attributes) => (attributes[attr] ? { class: attributes[attr] } : {}),
        },
      };
    },

    parseHTML() {
      return classes.map((cls) => ({ tag: `span.${cls}` }));
    },

    renderHTML({ HTMLAttributes }) {
      return ['span', mergeAttributes(HTMLAttributes), 0];
    },

    addCommands() {
      return {
        [setCommand]: (value) => ({ commands }) => commands.setMark(name, { [attr]: value }),
        [unsetCommand]: () => ({ commands }) => commands.unsetMark(name),
      };
    },
  });
}

export const ColorClass = createClassMark({
  name: 'colorClass',
  attr: 'colorClass',
  classes: COLOR_CLASSES,
  setCommand: 'setColorClass',
  unsetCommand: 'unsetColorClass',
});

export const FontSizeClass = createClassMark({
  name: 'fontSizeClass',
  attr: 'fontSizeClass',
  classes: FONT_SIZE_CLASSES,
  setCommand: 'setFontSizeClass',
  unsetCommand: 'unsetFontSizeClass',
});

export const FontWeightClass = createClassMark({
  name: 'fontWeightClass',
  attr: 'fontWeightClass',
  classes: FONT_WEIGHT_CLASSES,
  setCommand: 'setFontWeightClass',
  unsetCommand: 'unsetFontWeightClass',
});

/**
 * Heading, который рендерит `<hN class="heading-hN">` (как старый редактор) и парсит обратно.
 * Только уровни 1 и 2 - как в тулбаре.
 */
export const ClassHeading = Heading.extend({
  addOptions() {
    return {
      ...this.parent?.(),
      levels: [1, 2],
    };
  },

  renderHTML({ node, HTMLAttributes }) {
    const level = this.options.levels.includes(node.attrs.level)
      ? node.attrs.level
      : this.options.levels[0];
    return [`h${level}`, mergeAttributes(HTMLAttributes, { class: `heading-h${level}` }), 0];
  },
});

/**
 * Парсит размер картинки (`width`/`height`) из HTML-атрибута, а если его нет - из inline-style
 * `<dim>:Npx`. Возвращает целое число px > 0 либо null. Хранение и сериализация идут в HTML-атрибут
 * (он проходит whitelist DOMPurify); inline-style понимаем только ради уже сохранённого чужого контента.
 * @param {HTMLElement} element
 * @param {'width'|'height'} attr
 * @returns {number|null}
 */
function parseImageDimension(element, attr) {
  const raw = element.getAttribute(attr);
  if (raw && /^\d+(?:\.\d+)?$/.test(raw)) {
    const value = Math.round(parseFloat(raw));
    return value > 0 ? value : null;
  }
  const styleValue = element.style?.[attr] || '';
  const match = styleValue.match(/^(\d+(?:\.\d+)?)px$/);
  if (!match) return null;
  const value = Math.round(parseFloat(match[1]));
  return value > 0 ? value : null;
}

/**
 * Image с классом `constructor-image` и поддержкой data:URL (картинки вставляются в base64).
 *
 * Размер настраивается перетаскиванием маркеров (NodeView `ResizableImage.vue`): 8 маркеров
 * (4 угла + 4 стороны), свободный ресайз ширины и высоты независимо, Shift = пропорционально.
 * Ширина и высота хранятся в HTML-атрибутах `width`/`height` (число px) и round-trip'ят через них.
 * Эти атрибуты проходят санитайзер DOMPurify по умолчанию (см. utils/sanitize.js + sanitize.spec.js),
 * поэтому размер переживает сохранение/перезагрузку. Без заданных размеров картинка рендерится в
 * натуральном размере и ограничена `max-width:100%` у потребителей (не растягивается на максимум).
 */
export const ConstructorImage = Image.extend({
  addOptions() {
    return {
      ...this.parent?.(),
      inline: false,
      allowBase64: true,
      HTMLAttributes: { class: 'constructor-image' },
    };
  },

  addAttributes() {
    return {
      ...this.parent?.(),
      width: {
        default: null,
        parseHTML: (element) => parseImageDimension(element, 'width'),
        renderHTML: (attributes) => (attributes.width > 0 ? { width: attributes.width } : {}),
      },
      height: {
        default: null,
        parseHTML: (element) => parseImageDimension(element, 'height'),
        renderHTML: (attributes) => (attributes.height > 0 ? { height: attributes.height } : {}),
      },
      align: {
        default: null,
        parseHTML: (element) =>
          IMAGE_ALIGNMENTS.find((a) => element.classList.contains(`img-align-${a}`)) || null,
        renderHTML: (attributes) =>
          attributes.align ? { class: `img-align-${attributes.align}` } : {},
      },
    };
  },

  addNodeView() {
    return VueNodeViewRenderer(ResizableImage);
  },

  addCommands() {
    return {
      ...this.parent?.(),
      setImageAlign:
        (align) =>
        ({ commands }) => {
          if (align !== null && !IMAGE_ALIGNMENTS.includes(align)) return false;
          return commands.updateAttributes(this.name, { align });
        },
    };
  },
});

/**
 * Выравнивание абзацев и заголовков через CSS-класс `text-align-left|center|right`,
 * а НЕ inline-style. Консистентно с остальными марками TextConstructor (цвет/размер/жирность
 * тоже class-based) и не завязывается на санитизацию значений inline-style. Глобальный
 * атрибут вешается на paragraph/heading; потребители рендерят его через `:deep(.text-align-*)`.
 * parseHTML дополнительно понимает inline `style="text-align:..."` - на случай уже сохранённого
 * чужого контента, но сериализует всегда в класс.
 */
export const TextAlignClass = Extension.create({
  name: 'textAlignClass',

  addOptions() {
    return {
      types: TEXT_ALIGN_TYPES,
      alignments: TEXT_ALIGNMENTS,
    };
  },

  addGlobalAttributes() {
    return [
      {
        types: this.options.types,
        attributes: {
          textAlign: {
            default: null,
            parseHTML: (element) => {
              const cls = this.options.alignments.find((a) =>
                element.classList.contains(`text-align-${a}`)
              );
              if (cls) return cls;
              const styleAlign = element.style?.textAlign;
              return this.options.alignments.includes(styleAlign) ? styleAlign : null;
            },
            renderHTML: (attributes) =>
              attributes.textAlign ? { class: `text-align-${attributes.textAlign}` } : {},
          },
        },
      },
    ];
  },

  addCommands() {
    return {
      setTextAlignClass:
        (alignment) =>
        ({ commands }) => {
          if (!this.options.alignments.includes(alignment)) return false;
          return this.options.types.every((type) =>
            commands.updateAttributes(type, { textAlign: alignment })
          );
        },
      unsetTextAlignClass:
        () =>
        ({ commands }) =>
          this.options.types.every((type) => commands.resetAttributes(type, 'textAlign')),
    };
  },
});
