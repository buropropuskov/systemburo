import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

// Бейдж согласия ниже контактных (22px против 28px), и держится это только на
// специфичности: одиночный .consent-badge стоит в файле выше .detail-badge, чья
// shorthand `padding` его перебивает. jsdom scoped-CSS не считает, поэтому
// каскад стережём чтением SFC - иначе правку молча съест соседнее правило.
const sfc = fs.readFileSync(
  path.resolve(__dirname, '../UserProfileHeader.vue'),
  'utf8',
);

const ruleBody = (selector) => {
  const at = sfc.indexOf(`\n${selector} {`);
  return at === -1 ? null : sfc.slice(at, sfc.indexOf('}', at));
};

describe('UserProfileHeader — поля бейджа согласия', () => {
  it('сжатые поля объявлены селектором сильнее .detail-badge', () => {
    const compact = ruleBody('.detail-badge.consent-badge');

    expect(compact).not.toBeNull();
    expect(compact).toMatch(/padding-top:\s*2px/);
    expect(compact).toMatch(/padding-bottom:\s*2px/);
  });

  it('одиночный .consent-badge полей не задаёт - его перебьёт .detail-badge ниже', () => {
    const plain = ruleBody('.consent-badge');
    const detailAt = sfc.indexOf('\n.detail-badge {');
    const plainAt = sfc.indexOf('\n.consent-badge {');

    expect(plain).not.toMatch(/padding/);
    // Если порядок когда-нибудь изменится, проверка выше перестанет быть нужной,
    // но пока .detail-badge ниже - одиночному селектору полей не доверять.
    expect(plainAt).toBeLessThan(detailAt);
  });
});
