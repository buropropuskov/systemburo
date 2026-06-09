<script>
import { h } from 'vue';
import { navIcons } from './navIcons.js';

/**
 * Обёртка line-иконки навигации (#510). Рендерит SVG из реестра navIcons по
 * имени, обводкой currentColor - цвет наследуется от текста пункта (active/hover
 * меняют цвет без правки иконки). Размер задаётся пропом (по умолчанию 18px).
 *
 * Внутренняя разметка иконок - статические доверенные константы из navIcons.js
 * (не пользовательский ввод), поэтому innerHTML здесь безопасен.
 */
export default {
  name: 'NavIcon',
  props: {
    name: {
      type: String,
      required: true,
    },
    size: {
      type: [Number, String],
      default: 18,
    },
  },
  render() {
    const markup = navIcons[this.name] || '';
    return h('svg', {
      width: this.size,
      height: this.size,
      viewBox: '0 0 24 24',
      fill: 'none',
      stroke: 'currentColor',
      'stroke-width': 1.7,
      'stroke-linecap': 'round',
      'stroke-linejoin': 'round',
      'aria-hidden': 'true',
      focusable: 'false',
      innerHTML: markup,
    });
  },
};
</script>
