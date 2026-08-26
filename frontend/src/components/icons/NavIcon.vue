<script>
import { navIcons } from './navIcons.js';
import { createIconComponent } from './iconComponent.js';

/**
 * Обёртка line-иконки навигации (#510). Рендерит SVG из реестра navIcons по
 * имени, обводкой currentColor - цвет наследуется от текста пункта (active/hover
 * меняют цвет без правки иконки). Размер задаётся пропом (по умолчанию 18px).
 */
export default createIconComponent({
  name: 'NavIcon',
  icons: navIcons,
  defaultSize: 18,
});
</script>
