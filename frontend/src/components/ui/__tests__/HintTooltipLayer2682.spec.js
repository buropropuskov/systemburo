import { describe, expect, it } from 'vitest';
import { mount } from '@vue/test-utils';
import HintTooltip from '../HintTooltip.vue';
describe('tooltip layer for high modal headers', () => {
  it.each([[undefined, '15000'], [20020, '20020']])('preserves the default and supports the explicit layer %s', async (zIndex, expected) => {
    const wrapper = mount(HintTooltip, { props: { text: 'Подсказка', ...(zIndex === undefined ? {} : { zIndex }) }, attachTo: document.body });
    await wrapper.get('.hint-tooltip').trigger('focus');
    expect(document.querySelector('[role="tooltip"]').style.zIndex).toBe(expected);
    wrapper.unmount();
  });
});
