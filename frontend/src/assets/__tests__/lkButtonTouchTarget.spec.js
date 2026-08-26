import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect } from 'vitest';

const FORMS_CSS = readFileSync(resolve(__dirname, '../forms.css'), 'utf8');
const VIEWS = ['EmployeeView', 'CarsView'].map((name) => [
  name,
  readFileSync(resolve(__dirname, `../../views/${name}.vue`), 'utf8'),
]);

/**
 * Высота базовой пилюли задаётся отступами, а не минимумом.
 *
 * Минимум здесь уже стоял: гейт мобильных инвариантов поймал кнопку «Повторить» на 34px
 * при норме 36, и правило дописали в базу. Но десятки кнопок объявляют СВОЮ высоту в
 * 25-34px, а `min-height` сильнее `height` - раздулось всё разом, от «Обучения» в обзоре
 * до компактных кнопок в карточках. Норму добираем точечно там, где она нужна.
 */
describe('forms.css — высота базовой кнопки', () => {
  it('.lk-button не задаёт минимальную высоту', () => {
    const rule = FORMS_CSS.match(/\.lk-button\s*\{([^}]*)\}/);

    expect(rule, 'правило .lk-button не найдено').not.toBeNull();
    expect(rule[1], `минимум на базовой пилюле раздувает кнопки со своей высотой: ${rule[1]}`)
      .not.toMatch(/min-height:\s*[1-9]/);
  });

  it.each(VIEWS)('%s: кнопка повтора в блоке ошибки добирает норму тач-таргета', (_name, sfc) => {
    const rule = sfc.match(/\.list-error-state \.lk-button\s*\{([^}]*)\}/);

    expect(rule, 'правило для кнопки повтора не найдено').not.toBeNull();
    const minHeight = rule[1].match(/min-height:\s*(\d+)px/);
    expect(minHeight, `min-height не задан: ${rule[1]}`).not.toBeNull();
    expect(Number(minHeight[1])).toBeGreaterThanOrEqual(36);
  });
});
