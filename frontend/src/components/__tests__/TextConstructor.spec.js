import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import TextConstructor from '../TextConstructor.vue';

function mountConstructor(props = {}) {
  return mount(TextConstructor, {
    props: { modelValue: '', ...props },
    global: {
      stubs: { teleport: true, BaseModal: { template: '<div class="stub-modal"><slot/></div>' } },
    },
    attachTo: document.body,
  });
}

/**
 * Подменяет FileReader на синхронный fake на время теста.
 * Так избегаем флейков из-за реального асинхронного jsdom FileReader.
 */
class SyncFileReader {
  constructor() {
    this.result = null;
    this.error = null;
    this.onload = null;
    this.onerror = null;
  }
  readAsDataURL(file) {
    queueMicrotask(() => {
      this.result = `data:${file.type || 'image/png'};base64,ZmFrZQ==`;
      if (this.onload) this.onload();
    });
  }
}

describe('TextConstructor', () => {
  let originalFileReader;

  beforeEach(() => {
    originalFileReader = globalThis.FileReader;
    globalThis.FileReader = SyncFileReader;
  });

  afterEach(() => {
    globalThis.FileReader = originalFileReader;
  });

  it('рендерит тулбар с кнопкой изображения', () => {
    const wrapper = mountConstructor();
    const imageBtn = wrapper.find('.image-btn');
    expect(imageBtn.exists()).toBe(true);
    expect(imageBtn.attributes('data-tooltip')).toBe('Вставить изображение');
  });

  it('disabled пропс блокирует toolbar-кнопки', () => {
    const wrapper = mountConstructor({ disabled: true, modelValue: 'hi' });
    const buttons = wrapper.findAll('.toolbar-btn');
    const disabledOnes = buttons.filter((b) => b.attributes('disabled') !== undefined);
    // italic, underline, lists, headings, colors, image, break, undo - всё кроме preview
    expect(disabledOnes.length).toBeGreaterThan(5);
  });

  it('preview-кнопка disabled когда modelValue пустой', () => {
    const wrapper = mountConstructor({ modelValue: '' });
    const previewBtn = wrapper.find('.preview-btn');
    expect(previewBtn.attributes('disabled')).toBeDefined();
  });

  it('preview-кнопка активна когда есть контент', () => {
    const wrapper = mountConstructor({ modelValue: '<p>Hello</p>' });
    const previewBtn = wrapper.find('.preview-btn');
    expect(previewBtn.attributes('disabled')).toBeUndefined();
  });

  it('кнопка изображения вызывает file input click', async () => {
    const wrapper = mountConstructor();
    const input = wrapper.find('input.image-input');
    expect(input.exists()).toBe(true);
    const clickSpy = vi.fn();
    input.element.click = clickSpy;
    await wrapper.find('.image-btn').trigger('click');
    expect(clickSpy).toHaveBeenCalled();
  });

  it('отвергает файл с неподдерживаемым типом и показывает ошибку', async () => {
    const wrapper = mountConstructor();
    const input = wrapper.find('input.image-input');

    const file = new File(['x'], 'evil.exe', { type: 'application/x-msdownload' });
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true });
    await input.trigger('change');

    expect(wrapper.find('.constructor-error').exists()).toBe(true);
    expect(wrapper.find('.constructor-error').text()).toContain('формат');
    expect(wrapper.emitted('update:modelValue')).toBeUndefined();
  });

  it('отвергает изображение больше maxImageBytes', async () => {
    const wrapper = mountConstructor({ maxImageBytes: 10 });
    const input = wrapper.find('input.image-input');

    const file = new File([new Uint8Array(50)], 'big.png', { type: 'image/png' });
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true });
    await input.trigger('change');

    expect(wrapper.find('.constructor-error').exists()).toBe(true);
    expect(wrapper.find('.constructor-error').text()).toContain('большой');
    expect(wrapper.emitted('update:modelValue')).toBeUndefined();
  });

  it('валидное изображение вставляет <img> с data:URL и классом constructor-image', async () => {
    const wrapper = mountConstructor();
    await flushPromises();
    const input = wrapper.find('input.image-input');

    const file = new File(['fakebytes'], 'pic.png', { type: 'image/png' });
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true });
    await input.trigger('change');
    await flushPromises();
    await wrapper.vm.$nextTick();

    const emitted = wrapper.emitted('update:modelValue');
    expect(emitted).toBeTruthy();
    const lastValue = emitted[emitted.length - 1][0];
    expect(lastValue).toContain('data:image/png;base64,');
    expect(lastValue).toContain('alt="pic.png"');
    expect(lastValue).toContain('class="constructor-image"');
  });

  it('alt атрибут изображения санитизируется от опасных символов', async () => {
    const wrapper = mountConstructor();
    await flushPromises();
    const input = wrapper.find('input.image-input');

    const file = new File(['fakebytes'], 'a<b>"&\'.png', { type: 'image/png' });
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true });
    await input.trigger('change');
    await flushPromises();
    await wrapper.vm.$nextTick();

    const emitted = wrapper.emitted('update:modelValue');
    expect(emitted).toBeTruthy();
    const lastValue = emitted[emitted.length - 1][0];
    expect(lastValue).not.toContain('<b>');
    expect(lastValue).toContain('alt="ab.png"');
  });

  it('round-trip: сохранённый HTML с классами не теряет форматирование', async () => {
    const html =
      '<h1 class="heading-h1">Заголовок</h1>' +
      '<p><span class="red-text">красный</span> ' +
      '<span class="font-size-18">крупный</span> ' +
      '<span class="font-weight-600">жирный</span></p>' +
      '<ul><li>пункт</li></ul>' +
      '<img class="constructor-image" src="data:image/png;base64,ZmFrZQ==" alt="pic">';
    const wrapper = mountConstructor({ modelValue: html });
    await flushPromises();

    const out = wrapper.vm.editor.getHTML();
    expect(out).toContain('heading-h1');
    expect(out).toContain('red-text');
    expect(out).toContain('font-size-18');
    expect(out).toContain('font-weight-600');
    expect(out).toContain('<ul>');
    expect(out).toContain('<li>');
    expect(out).toContain('constructor-image');
    expect(out).toContain('data:image/png;base64,');
  });

  it('кнопки форматирования имеют data-tooltip', () => {
    const wrapper = mountConstructor();
    const italicBtn = wrapper.findAll('.toolbar-btn').find((b) => b.attributes('data-tooltip') === 'Курсив');
    expect(italicBtn).toBeTruthy();
  });
});
