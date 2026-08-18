<script>
import { appIcons } from './appIcons.js';
import { createIconComponent } from './iconComponent.js';

/**
 * Значок действия или поля внутри экрана: рисуется из реестра appIcons обводкой
 * currentColor, поэтому цвет наследуется от текста и темизация не требует
 * отдельных правил. Размер по умолчанию 20px - типичный значок в поле и кнопке;
 * где нужен другой, его задаёт CSS класса или проп size.
 */
export default createIconComponent({
  name: 'AppIcon',
  icons: appIcons,
  defaultSize: 20,
});
</script>
