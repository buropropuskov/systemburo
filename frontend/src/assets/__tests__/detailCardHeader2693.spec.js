import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
const read = file => readFileSync(new URL(file, import.meta.url), 'utf8');
describe('shared detail card header #2693', () => {
  it('uses 60px in both layers and the agreed desktop padding', () => {
    const shared = read('../detail-card-header.css');
    const component = read('../../components/CreateApplication/DetailCardHeader.vue');
    for (const css of [shared, component]) {
      expect(css).toMatch(/min-height:\s*60px/);
      expect(css).toMatch(/height:\s*60px/);
      expect(css).not.toMatch(/height:\s*50px/);
    }
    expect(shared).toContain('padding: 8px 22px');
    expect(component).toContain('padding: 6px 10px');
  });
});
