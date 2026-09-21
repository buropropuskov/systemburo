import { describe, it, expect } from 'vitest';
import { passageChipHint as подсказка } from '@/constants/passageSource';

/**
 * Подсказка к чипу поста в таблице вложения: в ячейке места на подпись нет, поэтому
 * источник уходит в подсказку - её и проверяем, той же функцией, что зовёт компонент.
 */

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
