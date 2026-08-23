import { describe, it, expect } from 'vitest';
import { tourChapters, chapterOf, isChapterEnd } from '../stepsFlow';
import { onboardingSteps } from '../onboardingSteps';

/**
 * Тур заявителя идёт почти шестьдесят шагов подряд - это минут семь. Главы дают
 * ориентир «где я» и место, где прерваться не жалко.
 */
describe('главы тура', () => {
  const steps = [
    { route: '/news', title: 'а' },
    { route: '/news', title: 'б' },
    { route: '/personal-cabinet', title: 'в' },
    { route: '/personal-cabinet', title: 'г' },
    { route: '/carsview', title: 'д' },
  ];

  it('глава - подряд идущие шаги одной страницы', () => {
    const chapters = tourChapters(steps);
    expect(chapters).toHaveLength(3);
    expect(chapters[0]).toMatchObject({ start: 0, end: 1 });
    expect(chapters[1]).toMatchObject({ start: 2, end: 3 });
    expect(chapters[2]).toMatchObject({ start: 4, end: 4 });
  });

  it('шаг знает свою главу и её номер', () => {
    expect(chapterOf(steps, 3)).toMatchObject({ number: 2, total: 3 });
    expect(chapterOf(steps, 0)).toMatchObject({ number: 1, total: 3 });
  });

  it('у главы человеческое имя из навигации системы', () => {
    expect(chapterOf(steps, 0).title).toBeTruthy();
    expect(chapterOf(steps, 0).title).not.toBe('/news');
  });

  it('прерваться предлагаем на последнем шаге главы', () => {
    expect(isChapterEnd(steps, 1)).toBe(true);
    expect(isChapterEnd(steps, 0)).toBe(false);
    expect(isChapterEnd(steps, 3)).toBe(true);
  });

  it('на последней главе паузу не предлагаем - там финал, а не перерыв', () => {
    expect(isChapterEnd(steps, 4)).toBe(false);
  });

  it('настоящий тур заявителя делится на несколько глав разумного размера', () => {
    const chapters = tourChapters(onboardingSteps);
    expect(chapters.length).toBeGreaterThanOrEqual(4);
    expect(chapters.every((c) => c.title && c.end >= c.start)).toBe(true);
    // Глава на весь тур смысла не имеет - это то же, что её отсутствие
    expect(Math.max(...chapters.map((c) => c.end - c.start + 1))).toBeLessThan(onboardingSteps.length);
  });
});
