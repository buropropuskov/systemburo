import { describe, it, expect } from 'vitest';
import { isIOS, isStandalone, needsIosHomeScreenInstall } from '../webPushPlatform';

const IPHONE_UA = 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1';
const IPAD_OLD_UA = 'Mozilla/5.0 (iPad; CPU OS 13_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0 Mobile/15E148 Safari/604.1';
const ANDROID_UA = 'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Mobile Safari/537.36';
const DESKTOP_MAC_UA = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36';

function nav({ userAgent = '', platform = '', maxTouchPoints = 0 } = {}) {
  return { userAgent, platform, maxTouchPoints };
}

function win({ standalone = false, matches = false } = {}) {
  return {
    matchMedia: (q) => ({ media: q, matches }),
    navigator: { standalone },
  };
}

describe('isIOS', () => {
  it('распознаёт iPhone по User-Agent', () => {
    expect(isIOS(nav({ userAgent: IPHONE_UA }))).toBe(true);
  });

  it('распознаёт старый iPad (несёт "iPad" в UA)', () => {
    expect(isIOS(nav({ userAgent: IPAD_OLD_UA }))).toBe(true);
  });

  it('распознаёт iPadOS 13+ (UA как у Mac, но мультитач)', () => {
    expect(isIOS(nav({ userAgent: DESKTOP_MAC_UA, platform: 'MacIntel', maxTouchPoints: 5 }))).toBe(true);
  });

  it('не путает настоящий Mac (тот же UA/platform, но без тача) с iPad', () => {
    expect(isIOS(nav({ userAgent: DESKTOP_MAC_UA, platform: 'MacIntel', maxTouchPoints: 0 }))).toBe(false);
  });

  it('не считает Android мобильным iOS-устройством', () => {
    expect(isIOS(nav({ userAgent: ANDROID_UA }))).toBe(false);
  });
});

describe('isStandalone', () => {
  it('true при display-mode: standalone', () => {
    expect(isStandalone(win({ matches: true }))).toBe(true);
  });

  it('true при navigator.standalone (специфично для iOS Safari)', () => {
    expect(isStandalone(win({ standalone: true }))).toBe(true);
  });

  it('false в обычной вкладке браузера', () => {
    expect(isStandalone(win())).toBe(false);
  });
});

describe('needsIosHomeScreenInstall', () => {
  it('true на iPhone в обычной вкладке Safari', () => {
    expect(needsIosHomeScreenInstall(nav({ userAgent: IPHONE_UA }), win())).toBe(true);
  });

  it('false на iPhone, уже добавленном на экран Домой', () => {
    expect(needsIosHomeScreenInstall(nav({ userAgent: IPHONE_UA }), win({ standalone: true }))).toBe(false);
  });

  it('false на десктопе и Android независимо от standalone', () => {
    expect(needsIosHomeScreenInstall(nav({ userAgent: DESKTOP_MAC_UA, platform: 'MacIntel' }), win())).toBe(false);
    expect(needsIosHomeScreenInstall(nav({ userAgent: ANDROID_UA }), win())).toBe(false);
  });
});
