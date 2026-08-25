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

// Ширина бейджей почты и телефона считается клонированием узла с УДАЛЕНИЕМ
// `.icon` (см. measureBadgeWidths): часть глифов теперь приходит компонентом
// NavIcon, и стоит ему перестать пробрасывать класс - замер молча начнёт
// считать бейдж вместе с иконкой, а надпись «Скопировано!» будет обрезаться.
describe('UserProfileHeader — глифы бейджей', () => {
  it('все иконки несут класс .icon, по которому идёт замер ширины', () => {
    const markup = sfc.slice(0, sfc.indexOf('</template>'));
    const glyphs = markup.match(/<(svg|NavIcon)\b[^>]*/g) || [];

    expect(glyphs.length).toBeGreaterThanOrEqual(6);
    glyphs.forEach((tag) => expect(tag, `глиф без класса: ${tag}`).toMatch(/class="icon"/));
  });

  it('рисует глифы обводкой, а не заливкой силуэта', () => {
    const icon = ruleBody('.icon');

    expect(icon).not.toBeNull();
    expect(icon).toMatch(/fill:\s*none/);
    expect(icon).toMatch(/stroke:\s*currentColor/);
  });
});

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
