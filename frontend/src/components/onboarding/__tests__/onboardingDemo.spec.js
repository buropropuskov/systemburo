import { describe, it, expect } from 'vitest';
import { getDemo } from '../onboardingDemo';
import { allTourSteps } from '../tours';

describe('onboardingDemo', () => {
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
