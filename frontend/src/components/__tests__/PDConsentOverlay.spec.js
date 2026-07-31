import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { mount, flushPromises } from '@vue/test-utils';

vi.mock('@/api/pdConsent', () => ({
  getConsentGate: vi.fn(),
  acceptConsent: vi.fn(),
}));
vi.mock('@/api/dataProcessing', () => ({
  downloadDataProcessingDoc: vi.fn().mockResolvedValue(undefined),
}));

const notify = vi.fn();
vi.mock('@/stores/deletions', () => ({
  useDeletionsStore: () => ({ notify }),
}));

import { acceptConsent } from '@/api/pdConsent';
import { downloadDataProcessingDoc } from '@/api/dataProcessing';
import { usePDConsentStore } from '@/stores/pdConsent';
import { resetBodyScrollLock } from '@/utils/bodyScrollLock';
import PDConsentOverlay from '../PDConsentOverlay.vue';

// jsdom не реализует наблюдателей. Держим колбэк IntersectionObserver под рукой:
// компонент детектит конец документа только им (арифметика scrollTop врёт под
// корневым zoom), поэтому «доскроллил» в тесте = вызов этого колбэка.
let ioCallbacks = [];
let ioDisconnects = 0;

class IntersectionObserverStub {
  constructor(cb) {
    ioCallbacks.push(cb);
  }
  observe() {}
  unobserve() {}
  disconnect() {
    ioDisconnects += 1;
  }
}

class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

function scrollToEnd() {
  ioCallbacks.at(-1)([{ isIntersecting: true }]);
}

function mountOverlay({ active = true } = {}) {
  return mount(PDConsentOverlay, {
    props: { active },
    global: { stubs: { teleport: true, transition: false } },
  });
}

function primeStore({ text = '<p>Текст согласия</p>', document: doc = null, version = 2, versionAt = '' } = {}) {
  const store = usePDConsentStore();
  store.resolved = true;
  store.required = true;
  store.version = version;
  store.versionAt = versionAt;
  store.html = text;
  store.docMeta = doc;
  return store;
}

describe('PDConsentOverlay (#1567)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    ioCallbacks = [];
    ioDisconnects = 0;
    notify.mockReset();
    acceptConsent.mockReset();
    downloadDataProcessingDoc.mockReset().mockResolvedValue(undefined);
    resetBodyScrollLock();
    vi.stubGlobal('IntersectionObserver', IntersectionObserverStub);
    vi.stubGlobal('ResizeObserver', ResizeObserverStub);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    resetBodyScrollLock();
  });

  it('без прокрутки до конца галочка и кнопка мертвы', async () => {
    primeStore();
    const wrapper = mountOverlay();
    await flushPromises();

    expect(wrapper.get('[data-testid="pdc-agree"]').attributes('disabled')).toBeDefined();
    expect(wrapper.get('[data-testid="pdc-accept"]').attributes('disabled')).toBeDefined();
    expect(wrapper.text()).toContain('Прокрутите документ до конца');
  });

  it('прокрутка до конца без галочки кнопку не оживляет', async () => {
    primeStore();
    const wrapper = mountOverlay();
    await flushPromises();

    scrollToEnd();
    await flushPromises();

    expect(wrapper.get('[data-testid="pdc-agree"]').attributes('disabled')).toBeUndefined();
    expect(wrapper.get('[data-testid="pdc-accept"]').attributes('disabled')).toBeDefined();
    expect(wrapper.text()).toContain('Отметьте согласие');
  });

  it('прокрутка до конца плюс галочка активируют кнопку, клик записывает согласие', async () => {
    primeStore();
    acceptConsent.mockResolvedValue({ required: false, version: 2, text: '<p>Текст согласия</p>' });
    const wrapper = mountOverlay();
    await flushPromises();

    scrollToEnd();
    await flushPromises();
    await wrapper.get('[data-testid="pdc-agree"]').setValue(true);
    await flushPromises();

    const accept = wrapper.get('[data-testid="pdc-accept"]');
    expect(accept.attributes('disabled')).toBeUndefined();

    await accept.trigger('click');
    await flushPromises();

    expect(acceptConsent).toHaveBeenCalledTimes(1);
    expect(usePDConsentStore().required).toBe(false);
  });

  it('ошибка подтверждения показывает уведомление и не запирает кнопку', async () => {
    primeStore();
    acceptConsent.mockRejectedValue(new Error('Пользователь не найден'));
    const wrapper = mountOverlay();
    await flushPromises();

    scrollToEnd();
    await flushPromises();
    await wrapper.get('[data-testid="pdc-agree"]').setValue(true);
    await wrapper.get('[data-testid="pdc-accept"]').trigger('click');
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'error', bold: 'Пользователь не найден' }),
    );
    expect(wrapper.get('[data-testid="pdc-accept"]').attributes('disabled')).toBeUndefined();
  });

  it('текст рендерится через sanitizeHtml: скрипт и onerror вырезаны', async () => {
    primeStore({
      text: '<p>Пункт 1</p><script>window.__pwn = 1;</script><img src="x" onerror="window.__pwn = 2">',
    });
    const wrapper = mountOverlay();
    await flushPromises();

    const html = wrapper.get('.pdc-doc').html();
    expect(html).toContain('Пункт 1');
    expect(html).not.toContain('<script');
    expect(html).not.toContain('onerror');
    expect(window.__pwn).toBeUndefined();
  });

  it('«Выйти» доступна всегда - окно без выхода это тупик', async () => {
    primeStore();
    const wrapper = mountOverlay();
    await flushPromises();

    const logout = wrapper.get('[data-testid="pdc-logout"]');
    expect(logout.attributes('disabled')).toBeUndefined();
    await logout.trigger('click');
    expect(wrapper.emitted('logout')).toHaveLength(1);
  });

  it('«Скачать документ» есть при загруженном документе и качает его', async () => {
    primeStore({ document: { stored_name: 'doc.pdf', file_name: 'Согласие.pdf' } });
    const wrapper = mountOverlay();
    await flushPromises();

    await wrapper.get('[data-testid="pdc-download"]').trigger('click');
    await flushPromises();

    expect(downloadDataProcessingDoc).toHaveBeenCalledWith('Согласие.pdf');
  });

  it('без документа кнопки скачивания нет', async () => {
    primeStore({ document: null });
    const wrapper = mountOverlay();
    await flushPromises();

    expect(wrapper.find('[data-testid="pdc-download"]').exists()).toBe(false);
  });

  it('пустой текст не даёт согласиться и сообщает об этом', async () => {
    primeStore({ text: '' });
    const wrapper = mountOverlay();
    await flushPromises();

    scrollToEnd();
    await flushPromises();

    expect(wrapper.text()).toContain('Текст согласия недоступен');
    expect(wrapper.get('[data-testid="pdc-agree"]').attributes('disabled')).toBeDefined();
    expect(wrapper.get('[data-testid="pdc-accept"]').attributes('disabled')).toBeDefined();
  });

  it('окно не закрывается кликом по затемнению - гейт не смахивается', async () => {
    primeStore();
    const wrapper = mountOverlay();
    await flushPromises();

    await wrapper.get('.pdc-overlay').trigger('click');

    expect(wrapper.find('.pdc-modal').exists()).toBe(true);
    expect(wrapper.emitted('logout')).toBeUndefined();
  });

  it('блокирует прокрутку фона, пока показано, и отпускает при скрытии', async () => {
    primeStore();
    const wrapper = mountOverlay();
    await flushPromises();
    expect(document.body.style.overflow).toBe('hidden');

    await wrapper.setProps({ active: false });
    await flushPromises();
    expect(document.body.style.overflow).toBe('');
  });

  it('скрытие отпускает наблюдателя и сбрасывает прогресс чтения', async () => {
    primeStore();
    const wrapper = mountOverlay();
    await flushPromises();
    scrollToEnd();
    await flushPromises();
    await wrapper.get('[data-testid="pdc-agree"]').setValue(true);
    await flushPromises();

    await wrapper.setProps({ active: false });
    await flushPromises();
    expect(ioDisconnects).toBe(1);

    // Повторный показ (подняли редакцию) обязан снова требовать прочтения.
    await wrapper.setProps({ active: true });
    await flushPromises();
    expect(wrapper.get('[data-testid="pdc-agree"]').attributes('disabled')).toBeDefined();
    expect(wrapper.get('[data-testid="pdc-accept"]').attributes('disabled')).toBeDefined();
  });

  it('смена редакции при открытом окне требует прочитать текст заново', async () => {
    const store = primeStore();
    const wrapper = mountOverlay();
    await flushPromises();
    scrollToEnd();
    await flushPromises();
    await wrapper.get('[data-testid="pdc-agree"]').setValue(true);
    await flushPromises();
    expect(wrapper.get('[data-testid="pdc-accept"]').attributes('disabled')).toBeUndefined();

    store.version = 3;
    store.html = '<p>Новая редакция</p>';
    await flushPromises();

    expect(wrapper.get('[data-testid="pdc-agree"]').attributes('disabled')).toBeDefined();
    expect(wrapper.get('[data-testid="pdc-accept"]').attributes('disabled')).toBeDefined();
  });

  // Номер редакции сам по себе человеку ничего не говорит - дата говорит, с какого
  // числа действует то, что ему показывают.
  it('редакция помечена датой появления, а без даты остаётся один номер', async () => {
    primeStore({ version: 7, versionAt: '2026-07-30T12:00:00Z' });
    const withDate = mountOverlay();
    await flushPromises();
    expect(withDate.text()).toContain('Редакция 7 от 30.07.2026');

    withDate.unmount();
    setActivePinia(createPinia());
    primeStore({ version: 7, versionAt: '' });
    const noDate = mountOverlay();
    await flushPromises();
    expect(noDate.text()).toContain('Редакция 7');
    expect(noDate.text()).not.toContain('от ');
  });

  // Подзаголовок убран по просьбе: он повторял то, что и так видно по мёртвой кнопке.
  it('в шапке нет прежнего подзаголовка про недоступность работы', async () => {
    primeStore();
    const wrapper = mountOverlay();
    await flushPromises();

    expect(wrapper.text()).not.toContain('работа в системе недоступна');
  });

  it('рядом с текстом есть пояснение своими словами', async () => {
    primeStore();
    const wrapper = mountOverlay();
    await flushPromises();

    const aside = wrapper.get('.pdc-modal__aside').text();
    expect(aside).toContain('данные вашей учётной записи');
    // Про отзыв согласия сказано с последствием, а не обещанием «в любой момент»:
    // отозвав его, человек снова упирается в это же окно.
    expect(aside).toContain('Отозвать согласие можно в личном кабинете');
    expect(aside).toContain('снова покажет это окно');
  });

  it('пока active=false, окно не рисуется', async () => {
    primeStore();
    const wrapper = mountOverlay({ active: false });
    await flushPromises();

    expect(wrapper.find('.pdc-overlay').exists()).toBe(false);
    expect(document.body.style.overflow).toBe('');
  });
});
