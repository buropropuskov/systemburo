import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import DirIcon from '../DirIcon.vue';

const pathOf = (direction) => mount(DirIcon, { props: { direction } }).find('path').attributes('d');

describe('DirIcon', () => {
  it('рисует стрелку с древком, а не шеврон-галочку', () => {
    // Древко = вертикальный (V) или горизонтальный (H) сегмент. У старого шеврона
    // его не было (только ломаная-галочка) — владелец просил нормальную стрелку.
    expect(pathOf('up')).toContain('V');
    expect(pathOf('down')).toContain('V');
    expect(pathOf('flat')).toContain('H');
    expect(pathOf('up')).not.toBe('M3 14l5-6 5 6');
    expect(pathOf('down')).not.toBe('M3 6l5 6 5-6');
  });

  it('разные направления дают разные пути, неизвестное падает на flat', () => {
    expect(pathOf('up')).not.toBe(pathOf('down'));
    expect(pathOf('whatever')).toBe(pathOf('flat'));
  });
});
