import { describe, it, expect } from 'vitest';

import ApplicationsCenter from '../ApplicationsCenter.vue';

// Презентационная логика сводного бейджа "N похожи на ЧС" в строке Центра заявок (#481, срез 6c).
// Поле blacklist_flags_count приходит из GET /applications (агрегат непереопределённых флагов).
// Рендер бейджа (v-if по полю) проверяется визуально на staging - здесь покрываем чистую логику.
const m = ApplicationsCenter.methods;
const ctx = { blacklistFlagCount: m.blacklistFlagCount };

const count = app => m.blacklistFlagCount.call(ctx, app);
const label = app => m.blacklistFlagLabel.call(ctx, app);

describe('ApplicationsCenter - сводный бейдж ЧС', () => {
  it('blacklistFlagCount читает поле и приводит к числу', () => {
    expect(count({ blacklist_flags_count: 2 })).toBe(2);
    expect(count({ blacklist_flags_count: '3' })).toBe(3);
  });

  it('blacklistFlagCount устойчив к отсутствию/null/нулю', () => {
    expect(count({})).toBe(0);
    expect(count({ blacklist_flags_count: null })).toBe(0);
    expect(count({ blacklist_flags_count: 0 })).toBe(0);
    expect(count(undefined)).toBe(0);
  });

  it('blacklistFlagLabel склоняет "похожа/похожи" по числу', () => {
    expect(label({ blacklist_flags_count: 1 })).toBe('1 похожа на ЧС');
    expect(label({ blacklist_flags_count: 2 })).toBe('2 похожи на ЧС');
    expect(label({ blacklist_flags_count: 5 })).toBe('5 похожи на ЧС');
  });

  it('blacklistFlagTitle - подсказка про чёрный список', () => {
    expect(m.blacklistFlagTitle()).toContain('чёрн');
  });
});
