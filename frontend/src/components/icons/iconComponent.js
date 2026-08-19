import { h } from 'vue';

/**
 * Собирает компонент-обёртку над реестром line-иконок: рендерит SVG по имени
 * глифа обводкой currentColor, размер берёт из пропа. Единая фабрика на оба
 * реестра (navIcons и appIcons) - иначе обёртки расходятся в мелочах вроде
 * stroke-width, и наборы перестают выглядеть одним семейством.
 *
 * Внутренняя разметка глифов - статические константы реестра (не пользовательский
 * ввод), поэтому innerHTML здесь безопасен.
 *
 * @param {{name: string, icons: Record<string, string>, defaultSize: number}} options
 * @returns {import('vue').Component}
 */
export function createIconComponent({ name, icons, defaultSize }) {
  return {
    name,
    props: {
      name: {
        type: String,
        required: true,
        // Имя вне реестра рисует пустой svg: разметка, стили и тесты при этом
        // верны, а значка на экране нет. Молча это переживает и опечатка, и
        // старый контракт пропа (BlacklistTabBase принимал путь к файлу).
        validator: (value) => {
          const known = Object.hasOwn(icons, value);
          if (!known) {
            console.warn(`${name}: глифа "${value}" нет в реестре - значок не нарисуется`);
          }
          return known;
        },
      },
      size: {
        type: [Number, String],
        default: defaultSize,
      },
    },
    render() {
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
        innerHTML: icons[this.name] || '',
      });
    },
  };
}
