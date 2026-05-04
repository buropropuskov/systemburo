import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount } from '@vue/test-utils';
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

  it('disabled пропс блокирует toolbar-кнопки и textarea', () => {
    const wrapper = mountConstructor({ disabled: true, modelValue: 'hi' });
    const textarea = wrapper.find('.constructor-textarea');
    expect(textarea.attributes('disabled')).toBeDefined();
    const buttons = wrapper.findAll('.toolbar-btn');
    // image, format, lists, headings, colors, break, undo
    const disabledOnes = buttons.filter(b => b.attributes('disabled') !== undefined);
    expect(disabledOnes.length).toBeGreaterThan(5);
  });

  it('preview-кнопка disabled когда modelValue пустой', () => {
    const wrapper = mountConstructor({ modelValue: '' });
    const previewBtn = wrapper.find('.preview-btn');
    expect(previewBtn.attributes('disabled')).toBeDefined();
  });

  it('preview-кнопка активна когда есть контент', () => {
    const wrapper = mountConstructor({ modelValue: 'Hello' });
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

  it('валидное изображение вставляет <img src="data:..."> через update:modelValue', async () => {
    const wrapper = mountConstructor();
    const input = wrapper.find('input.image-input');

    const file = new File(['fakebytes'], 'pic.png', { type: 'image/png' });
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true });
    await input.trigger('change');
    // ждём цепочку microtask -> reader.onload -> handleImageSelected await
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
    await wrapper.vm.$nextTick();

    const emitted = wrapper.emitted('update:modelValue');
    expect(emitted).toBeTruthy();
    const lastValue = emitted[emitted.length - 1][0];
    expect(lastValue).toContain('<img src="data:image/png;base64,');
    expect(lastValue).toContain('alt="pic.png"');
    expect(lastValue).toContain('class="constructor-image"');
  });

  it('alt атрибут изображения санитизируется от опасных символов', async () => {
    const wrapper = mountConstructor();
    const input = wrapper.find('input.image-input');

    const file = new File(['x'], '<script>"name".png', { type: 'image/png' });
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true });
    await input.trigger('change');
    // ждём цепочку microtask -> reader.onload -> handleImageSelected await
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
    await wrapper.vm.$nextTick();

    const emitted = wrapper.emitted('update:modelValue');
    const lastValue = emitted[emitted.length - 1][0];
    expect(lastValue).not.toContain('<script>');
    expect(lastValue).not.toContain('"name"');
  });

  it('форматирование italic оборачивает выделенный текст', async () => {
    const wrapper = mountConstructor({ modelValue: 'hello world' });
    const textarea = wrapper.find('.constructor-textarea');
    textarea.element.setSelectionRange(0, 5);
    await wrapper.findAll('.toolbar-btn')[0].trigger('click'); // italic

    const emitted = wrapper.emitted('update:modelValue');
    expect(emitted).toBeTruthy();
    expect(emitted[emitted.length - 1][0]).toContain('<em>hello</em>');
  });

  it('undo возвращает предыдущее значение', async () => {
    const wrapper = mountConstructor({ modelValue: 'a' });
    const textarea = wrapper.find('.constructor-textarea');
    await textarea.setValue('ab');
    await textarea.setValue('abc');

    await wrapper.vm.undo();
    const emitted = wrapper.emitted('update:modelValue');
    const last = emitted[emitted.length - 1][0];
    expect(last).toBe('ab');
  });

  it('кнопки форматирования имеют data-tooltip', () => {
    const wrapper = mountConstructor();
    const tooltips = wrapper.findAll('.toolbar-btn')
      .map(b => b.attributes('data-tooltip'))
      .filter(Boolean);
    expect(tooltips.length).toBeGreaterThan(5);
    expect(tooltips).toContain('Курсив');
    expect(tooltips).toContain('Заголовок h1');
    expect(tooltips).toContain('Вставить изображение');
  });
});
