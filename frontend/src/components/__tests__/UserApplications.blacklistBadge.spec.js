import { describe, it, expect } from 'vitest';

import UserApplications from '../UserApplications.vue';

// Презентационная логика сводного бейджа "N похожи на ЧС" в строке "Моих заявок" (#481, срез 6c).
// Зеркало Центра заявок: то же поле blacklist_flags_count из GET /applications/user.
// Рендер бейджа (v-if по полю) проверяется визуально на staging - здесь покрываем чистую логику.
const m = UserApplications.methods;
const ctx = { blacklistFlagCount: m.blacklistFlagCount };

const count = app => m.blacklistFlagCount.call(ctx, app);
const label = app => m.blacklistFlagLabel.call(ctx, app);

describe('UserApplications - сводный бейдж ЧС', () => {
  it('blacklistFlagCount читает поле и приводит к числу', () => {
    expect(count({ blacklist_flags_count: 4 })).toBe(4);
    expect(count({ blacklist_flags_count: '1' })).toBe(1);
  });

  it('blacklistFlagCount устойчив к отсутствию/null/нулю', () => {
    expect(count({})).toBe(0);
    expect(count({ blacklist_flags_count: null })).toBe(0);
    expect(count({ blacklist_flags_count: 0 })).toBe(0);
    expect(count(undefined)).toBe(0);
  });

  it('blacklistFlagLabel склоняет "похожа/похожи" по числу', () => {
    expect(label({ blacklist_flags_count: 1 })).toBe('1 похожа на ЧС');
    expect(label({ blacklist_flags_count: 3 })).toBe('3 похожи на ЧС');
  });

  it('blacklistFlagTitle - подсказка про чёрный список', () => {
    expect(m.blacklistFlagTitle()).toContain('чёрн');
  });
});
