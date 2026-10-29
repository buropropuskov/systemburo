import { describe, it, expect } from 'vitest';
import { getDemo } from '../onboardingDemo';
import { allTourSteps } from '../tours';
import { buildPopoverHtml } from '../popoverContent';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

describe('onboardingDemo', () => {
  it('reserves the real PNG aspect ratio before loading every demo image', () => {
    const files={factTable:'fact-table',applications:'applications',cars:'cars',employees:'employees',centerList:'center-list',approvers:'approvers',applicationDetail:'application-detail',vote:'detail-actions'};
    for(const [key,file] of Object.entries(files)) {
      const demo=getDemo(key),bytes=readFileSync(resolve(__dirname, '../../../assets/onboarding', `demo-${file}.png`));
      expect([demo.width,demo.height]).toEqual([bytes.readUInt32BE(16),bytes.readUInt32BE(20)]);
      const root=document.createElement('div');root.innerHTML=buildPopoverHtml({demo:key});
      expect(root.querySelector('img')?.getAttribute('width')).toBe(String(demo.width));
      expect(root.querySelector('img')?.getAttribute('height')).toBe(String(demo.height));
    }
  });
  it('getDemo возвращает {src, alt} для известного ключа', () => {
    const demo = getDemo('applications');
    expect(demo).toBeTruthy();
    expect(typeof demo.src).toBe('string');
    expect(demo.src.length).toBeGreaterThan(0);
    expect(typeof demo.alt).toBe('string');
  });

  it('getDemo возвращает null для неизвестного ключа', () => {
    expect(getDemo('nope')).toBeNull();
  });

  it('каждый step.demo во ВСЕХ турах резолвится в существующий скриншот', () => {
    // Скриншотом закрывают шаг, у которого на пустой системе нет цели, - опечатка
    // в ключе означала бы поповер без картинки ровно там, где показывать нечего.
    for (const step of allTourSteps().filter((s) => s.demo)) {
      expect(getDemo(step.demo), step.id).toBeTruthy();
    }
  });
});
