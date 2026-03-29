import { describe, it, expect } from 'vitest';
import { decodeToken, isTokenValid, getUserType, getTokenExpiry } from '../token';

function createMockJWT(payload, expiresInSeconds = 3600) {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const body = btoa(JSON.stringify({
    ...payload,
    exp: Math.floor(Date.now() / 1000) + expiresInSeconds,
  }));
  return `${header}.${body}.fake-signature`;
}

describe('decodeToken', () => {
  it('decodes a valid JWT payload', () => {
    const token = createMockJWT({ username: 'admin', type_id: 6 });
    const payload = decodeToken(token);
    expect(payload.username).toBe('admin');
    expect(payload.type_id).toBe(6);
  });

  it('returns null for invalid string', () => {
    expect(decodeToken('not-a-jwt')).toBeNull();
  });

  it('returns null for null input', () => {
    expect(decodeToken(null)).toBeNull();
  });

  it('returns null for undefined input', () => {
    expect(decodeToken(undefined)).toBeNull();
  });
});

describe('isTokenValid', () => {
  it('returns true for a non-expired token', () => {
    const token = createMockJWT({ username: 'user' }, 3600);
    expect(isTokenValid(token)).toBe(true);
  });

  it('returns false for an expired token', () => {
    const token = createMockJWT({ username: 'user' }, -100);
    expect(isTokenValid(token)).toBe(false);
  });

  it('returns false for null', () => {
    expect(isTokenValid(null)).toBe(false);
  });

  it('returns false for empty string', () => {
    expect(isTokenValid('')).toBe(false);
  });

  it('returns false for malformed token', () => {
    expect(isTokenValid('abc.def.ghi')).toBe(false);
  });
});

describe('getUserType', () => {
  it('extracts type_id from token', () => {
    const token = createMockJWT({ type_id: 3 });
    expect(getUserType(token)).toBe(3);
  });

  it('returns null when type_id is missing', () => {
    const token = createMockJWT({ username: 'user' });
    expect(getUserType(token)).toBeNull();
  });

  it('returns null for invalid token', () => {
    expect(getUserType('invalid')).toBeNull();
  });
});

describe('getTokenExpiry', () => {
  it('returns exp timestamp from token', () => {
    const token = createMockJWT({}, 7200);
    const expiry = getTokenExpiry(token);
    const expectedMin = Math.floor(Date.now() / 1000) + 7199;
    expect(expiry).toBeGreaterThanOrEqual(expectedMin);
  });

  it('returns 0 for invalid token', () => {
    expect(getTokenExpiry('bad-token')).toBe(0);
  });
});
