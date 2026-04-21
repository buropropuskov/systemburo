import { describe, it, expect } from 'vitest';
import {
  formatPhoneNumberImmediately,
  formatPhoneNumber,
  clearPhoneFormat,
} from '../usePhoneFormat';

describe('formatPhoneNumberImmediately', () => {
  it.each([
    ['9161234567', '+7 (916) 123 45-67', '9161234567'],
    ['89161234567', '+7 (916) 123 45-67', '89161234567'],
    ['+79161234567', '+7 (916) 123 45-67', '79161234567'],
    ['+7 (916) 123-45-67', '+7 (916) 123 45-67', '79161234567'],
  ])('formats "%s" to "%s"', (input, expectedFormatted, expectedRaw) => {
    const result = formatPhoneNumberImmediately(input);
    expect(result.formatted).toBe(expectedFormatted);
    expect(result.raw).toBe(expectedRaw);
  });

  it.each([
    ['', ''],
    [null, ''],
    [undefined, ''],
  ])('returns empty raw for falsy input: %s', (input, expectedRaw) => {
    const result = formatPhoneNumberImmediately(input);
    expect(result.raw).toBe(expectedRaw);
    expect(result.formatted).toBe(input);
  });

  it('strips non-digit characters', () => {
    const result = formatPhoneNumberImmediately('abc+7(916)def123-45-67');
    expect(result.raw).toBe('79161234567');
    expect(result.formatted).toBe('+7 (916) 123 45-67');
  });

  it('returns raw digits without formatting for incomplete number', () => {
    const result = formatPhoneNumberImmediately('916');
    expect(result.raw).toBe('916');
    expect(result.formatted).toBe('916');
  });

  it('does not format 11-digit number not starting with 7 or 8', () => {
    const result = formatPhoneNumberImmediately('19161234567');
    expect(result.raw).toBe('19161234567');
    expect(result.formatted).toBe('19161234567');
  });
});

describe('formatPhoneNumber', () => {
  it.each([
    ['', ''],
    [null, ''],
    [undefined, ''],
  ])('returns empty raw for falsy input: %s', (input, expectedRaw) => {
    const result = formatPhoneNumber(input);
    expect(result.raw).toBe(expectedRaw);
    expect(result.formatted).toBe(input);
  });

  it('formats complete 10-digit number with 7 prefix', () => {
    const result = formatPhoneNumber('9161234567');
    expect(result.formatted).toBe('+7 (916) 123 45-67');
  });

  it('replaces 8-prefix with 7 for 11-digit number', () => {
    const result = formatPhoneNumber('89161234567');
    expect(result.formatted).toBe('+7 (916) 123 45-67');
  });

  it('returns raw digits for partial input', () => {
    const result = formatPhoneNumber('916123');
    expect(result.raw).toBe('916123');
    expect(result.formatted).toBe('916123');
  });

  it('formats +7 prefixed number', () => {
    const result = formatPhoneNumber('+79161234567');
    expect(result.formatted).toBe('+7 (916) 123 45-67');
    expect(result.raw).toBe('79161234567');
  });
});

describe('clearPhoneFormat', () => {
  it('returns the raw number as-is', () => {
    expect(clearPhoneFormat('79161234567')).toBe('79161234567');
  });

  it('returns empty string for falsy input', () => {
    expect(clearPhoneFormat('')).toBe('');
    expect(clearPhoneFormat(null)).toBe('');
    expect(clearPhoneFormat(undefined)).toBe('');
  });
});
