import { describe, it, expect, vi, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import ScrollTopButton from '../ScrollTopButton.vue';

// Ω.1 (#1097 волна 3): Яндекс-браузер (мобильный) сам рисует стрелку-вверх при
// скролле - наша кнопка дублировала её. Прячем ТОЛЬКО в Яндексе (UA YaBrowser);
// у Chrome/Safari/Firefox встроенной стрелки нет - там кнопку показываем.

function setUA(ua) {
  Object.defineProperty(window.navigator, 'userAgent', { value: ua, configurable: true });
}

const REAL_UA = window.navigator.userAgent;

describe('ScrollTopButton — гейт Яндекс-браузера (#1097)', () => {
  afterEach(() => {
    setUA(REAL_UA);
    vi.restoreAllMocks();
  });

  it('в МОБИЛЬНОМ Яндексе: кнопка скрыта и scroll-слушатель не навешивается', () => {
    setUA('Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 YaBrowser/24.7.1 Mobile Safari/537.36');
    const addSpy = vi.spyOn(window, 'addEventListener');

    const wrapper = mount(ScrollTopButton);

    expect(wrapper.vm.isMobileYandex).toBe(true);
    expect(addSpy.mock.calls.some((c) => c[0] === 'scroll')).toBe(false);
    expect(wrapper.find('.scroll-top-btn').isVisible()).toBe(false);
  });

  it('в ДЕСКТОПНОМ Яндексе (YaBrowser без Mobile): кнопка остаётся - десктоп стрелку не рисует', () => {
    setUA('Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126.0 YaBrowser/24.7.0.0 Yowser/2.5 Safari/537.36');
    const addSpy = vi.spyOn(window, 'addEventListener');
    vi.spyOn(window, 'scrollY', 'get').mockReturnValue(200);

    const wrapper = mount(ScrollTopButton);

    expect(wrapper.vm.isMobileYandex).toBe(false);
    expect(addSpy.mock.calls.some((c) => c[0] === 'scroll')).toBe(true);
    wrapper.vm.handleScroll();
    expect(wrapper.vm.visible).toBe(true);
  });

  it('в обычном браузере: слушатель вешается, кнопка появляется после скролла >150', () => {
    setUA('Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 Chrome/126.0 Mobile Safari/537.36');
    const addSpy = vi.spyOn(window, 'addEventListener');
    vi.spyOn(window, 'scrollY', 'get').mockReturnValue(200);

    const wrapper = mount(ScrollTopButton);

    expect(wrapper.vm.isMobileYandex).toBe(false);
    expect(addSpy.mock.calls.some((c) => c[0] === 'scroll')).toBe(true);
    expect(wrapper.vm.visible).toBe(false);

    wrapper.vm.handleScroll();
    expect(wrapper.vm.visible).toBe(true);
  });
});
