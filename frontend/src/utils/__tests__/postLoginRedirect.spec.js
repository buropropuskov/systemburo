import { describe, it, expect } from 'vitest';
import { isSafeRedirectPath, buildLoginRedirect, resolveLoginRedirect } from '../postLoginRedirect';

describe('isSafeRedirectPath', () => {
  it('пропускает обычный внутренний путь', () => {
    expect(isSafeRedirectPath('/table/cars')).toBe(true);
    expect(isSafeRedirectPath('/personal-cabinet?tab=cars')).toBe(true);
  });

  it('отклоняет protocol-relative адрес', () => {
    expect(isSafeRedirectPath('//evil.example')).toBe(false);
  });

  it('отклоняет внешний адрес с протоколом', () => {
    expect(isSafeRedirectPath('https://evil.example/path')).toBe(false);
    expect(isSafeRedirectPath('/redirect?to=https://evil.example')).toBe(false);
  });

  it('отклоняет не-строку и пустое значение', () => {
    expect(isSafeRedirectPath(undefined)).toBe(false);
    expect(isSafeRedirectPath(null)).toBe(false);
    expect(isSafeRedirectPath('')).toBe(false);
    expect(isSafeRedirectPath(['/x'])).toBe(false);
  });
});

describe('buildLoginRedirect', () => {
  it('кладёт исходный адрес в query redirect на пути входа', () => {
    expect(buildLoginRedirect('/table/cars?filter=1')).toEqual({
      path: '/',
      query: { redirect: '/table/cars?filter=1' },
    });
  });
});

describe('resolveLoginRedirect', () => {
  it('возвращает адрес из query, если он безопасен', () => {
    expect(resolveLoginRedirect({ redirect: '/personal-cabinet' })).toBe('/personal-cabinet');
  });

  it('возвращает null без query.redirect', () => {
    expect(resolveLoginRedirect({})).toBeNull();
    expect(resolveLoginRedirect(undefined)).toBeNull();
  });

  it('возвращает null на небезопасный адрес', () => {
    expect(resolveLoginRedirect({ redirect: '//evil.example' })).toBeNull();
    expect(resolveLoginRedirect({ redirect: 'https://evil.example' })).toBeNull();
  });
});
