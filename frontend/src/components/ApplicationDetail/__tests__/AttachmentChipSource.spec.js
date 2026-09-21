import { describe, it, expect } from 'vitest';
import { passageSourceLabel } from '@/constants/passageSource';

/**
 * Подсказка к чипу поста в таблице вложения (#2551, замечание со стенда): в ячейке
 * места на подпись нет, поэтому источник уходит в подсказку. Считаем её тем же
 * способом, что и компонент, - иначе тест проверял бы собственную копию правила.
 */
function подсказка(items, names) {
  return items.map((item, index) => {
    const источник = passageSourceLabel(item.source);
    return источник ? `${names[index]} - ${источник}` : names[index];
  }).join(', ');
}

describe('подсказка к постам в карточке заявки', () => {
  it('называет источник у каждого поста', () => {
    const items = [
      { display_name: 'КПП №4', source: 'application' },
      { display_name: 'ПОСТ №72 (АВТО)', source: 'approver' },
    ];
    expect(подсказка(items, items.map((i) => i.display_name)))
      .toBe('КПП №4 - из заявки, ПОСТ №72 (АВТО) - назначил принимающий');
  });

  it('без источника оставляет одно название', () => {
    const items = [{ display_name: 'Проверка' }];
    expect(подсказка(items, ['Проверка'])).toBe('Проверка');
  });
});
