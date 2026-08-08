import { describe, it, expect } from 'vitest';
import {
  isIOS, isStandalone, needsIosHomeScreenInstall, isIosSafari, iosNeedsSafari,
} from '../webPushPlatform';

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

/**
 * Проверка на живом iPhone (#974): зашли через Chrome, а система показала инструкцию
 * для Safari - выполнить её оттуда нельзя. На iOS все браузеры работают на WebKit, но
 * push доступен только сайту, установленному из Safari.
 */
const IPHONE_CHROME_UA = 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/124.0.6367.111 Mobile/15E148 Safari/604.1';
const IPHONE_FIREFOX_UA = 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/125.0 Mobile/15E148 Safari/605.1.15';
const IPHONE_YANDEX_UA = 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 YaBrowser/24.4.3 Mobile/15E148 Safari/604.1';
const IPHONE_EDGE_UA = 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) EdgiOS/124.0 Mobile/15E148 Safari/605.1.15';

describe('isIosSafari', () => {
  it('настоящий Safari на iPhone', () => {
    expect(isIosSafari(nav({ userAgent: IPHONE_UA }))).toBe(true);
  });

  it.each([
    ['Chrome', IPHONE_CHROME_UA],
    ['Firefox', IPHONE_FIREFOX_UA],
    ['Яндекс.Браузер', IPHONE_YANDEX_UA],
    ['Edge', IPHONE_EDGE_UA],
  ])('%s на iPhone за Safari не выдаёт (в строке у всех есть слово Safari)', (_, ua) => {
    expect(isIosSafari(nav({ userAgent: ua }))).toBe(false);
  });

  it('не-iOS вообще не рассматривается', () => {
    expect(isIosSafari(nav({ userAgent: ANDROID_UA }))).toBe(false);
  });
});

describe('какую подсказку показать на iOS', () => {
  it('сторонний браузер - сначала перейти в Safari, а не ставить на «Домой»', () => {
    const n = nav({ userAgent: IPHONE_CHROME_UA });
    expect(iosNeedsSafari(n, win())).toBe(true);
    expect(needsIosHomeScreenInstall(n, win())).toBe(false);
  });

  it('Safari без установки - подсказка про экран «Домой»', () => {
    const n = nav({ userAgent: IPHONE_UA });
    expect(needsIosHomeScreenInstall(n, win())).toBe(true);
    expect(iosNeedsSafari(n, win())).toBe(false);
  });

  it('уже установлено - ни одной подсказки, показывается кнопка включения', () => {
    const n = nav({ userAgent: IPHONE_UA });
    const w = win({ standalone: true });
    expect(needsIosHomeScreenInstall(n, w)).toBe(false);
    expect(iosNeedsSafari(n, w)).toBe(false);
  });

  it('Android никакой iOS-подсказки не получает', () => {
    const n = nav({ userAgent: ANDROID_UA });
    expect(needsIosHomeScreenInstall(n, win())).toBe(false);
    expect(iosNeedsSafari(n, win())).toBe(false);
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
