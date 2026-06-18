import { describe, it, expect } from 'vitest';
import { getDemo } from '../onboardingDemo';
import { onboardingSteps } from '../onboardingSteps';

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

  it('каждый step.demo в конфиге резолвится в существующий скриншот', () => {
    for (const step of onboardingSteps.filter((s) => s.demo)) {
      expect(getDemo(step.demo)).toBeTruthy();
    }
  });
});
