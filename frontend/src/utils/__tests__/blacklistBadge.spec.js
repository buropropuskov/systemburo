import { describe, it, expect } from 'vitest';

import { blacklistFlagCount, blacklistFlagLabel, BLACKLIST_FLAG_TITLE } from '../blacklistBadge';

describe('blacklistBadge', () => {
  it('count читает поле и приводит к числу', () => {
    expect(blacklistFlagCount({ blacklist_flags_count: 2 })).toBe(2);
    expect(blacklistFlagCount({ blacklist_flags_count: '3' })).toBe(3);
  });

  it('count устойчив к отсутствию/null/нулю/мусору', () => {
    expect(blacklistFlagCount({})).toBe(0);
    expect(blacklistFlagCount({ blacklist_flags_count: null })).toBe(0);
    expect(blacklistFlagCount({ blacklist_flags_count: 0 })).toBe(0);
    expect(blacklistFlagCount(undefined)).toBe(0);
    expect(blacklistFlagCount({ blacklist_flags_count: 'abc' })).toBe(0);
  });

  it('label склоняет форму-1 на 1/21/31', () => {
    expect(blacklistFlagLabel({ blacklist_flags_count: 1 })).toBe('1 похоже на ЧС');
    expect(blacklistFlagLabel({ blacklist_flags_count: 21 })).toBe('21 похоже на ЧС');
    expect(blacklistFlagLabel({ blacklist_flags_count: 31 })).toBe('31 похоже на ЧС');
  });

  it('label склоняет множественную форму на 2-20/11', () => {
    expect(blacklistFlagLabel({ blacklist_flags_count: 2 })).toBe('2 похожи на ЧС');
    expect(blacklistFlagLabel({ blacklist_flags_count: 5 })).toBe('5 похожи на ЧС');
    expect(blacklistFlagLabel({ blacklist_flags_count: 11 })).toBe('11 похожи на ЧС');
    expect(blacklistFlagLabel({ blacklist_flags_count: 12 })).toBe('12 похожи на ЧС');
  });

  it('title - подсказка про чёрный список', () => {
    expect(BLACKLIST_FLAG_TITLE).toContain('чёрн');
  });
});
