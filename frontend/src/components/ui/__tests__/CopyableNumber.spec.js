import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import CopyableNumber from '../CopyableNumber.vue';
import { useDeletionsStore } from '@/stores/deletions';

let writeText;

beforeEach(() => {
  setActivePinia(createPinia());
  writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
});

describe('CopyableNumber', () => {
  it('клик копирует значение', async () => {
    const wrapper = mount(CopyableNumber, { props: { value: '№ 20260815/001' } });
    await wrapper.trigger('click');

    expect(writeText).toHaveBeenCalledWith('№ 20260815/001');
  });

  it('доступен с клавиатуры: роль кнопки, фокус и Enter', async () => {
    const wrapper = mount(CopyableNumber, { props: { value: 42 } });

    expect(wrapper.attributes('role')).toBe('button');
    expect(wrapper.attributes('tabindex')).toBe('0');
    await wrapper.trigger('keydown.enter');
    expect(writeText).toHaveBeenCalledWith('42');
  });

  it('подсказка по умолчанию говорит, что будет по нажатию', () => {
    const wrapper = mount(CopyableNumber, { props: { value: '1' } });
    expect(wrapper.attributes('data-tooltip')).toBe('Копировать');
  });

  it('успех подтверждается уведомлением со значением', async () => {
    const wrapper = mount(CopyableNumber, { props: { value: '№ 7' } });
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await wrapper.trigger('click');
    await Promise.resolve();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: '№ 7', type: 'success' }));
  });

  it('отказ буфера показывает ошибку, а не молчит', async () => {
    writeText.mockRejectedValue(new Error('denied'));
    const wrapper = mount(CopyableNumber, { props: { value: '№ 7' } });
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await wrapper.trigger('click');
    await Promise.resolve();
    await Promise.resolve();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }));
  });
});
