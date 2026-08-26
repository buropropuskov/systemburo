import { mount } from '@vue/test-utils';
import { describe, it, expect } from 'vitest';
import Pager from '../Pager.vue';

function mountPager(props = {}, slots = {}) {
  return mount(Pager, {
    props: {
      page: 1, totalPages: 3, total: 51, ...props,
    },
    slots,
  });
}

describe('ui/Pager', () => {
  it('показывает всего, номер страницы и шлёт соседние страницы наверх', async () => {
    const wrapper = mountPager({ page: 2 });

    expect(wrapper.find('.pager__total').text()).toBe('Всего: 51');
    expect(wrapper.find('.pager__page').text()).toBe('2 / 3');

    const [back, forward] = wrapper.findAll('.pager__btn');
    await back.trigger('click');
    await forward.trigger('click');

    expect(wrapper.emitted('update:page')).toEqual([[1], [3]]);
  });

  it('на краях списка кнопка в тупик выключена', () => {
    const first = mountPager({ page: 1 });
    expect(first.findAll('.pager__btn')[0].attributes('disabled')).toBeDefined();
    expect(first.findAll('.pager__btn')[1].attributes('disabled')).toBeUndefined();

    const last = mountPager({ page: 3 });
    expect(last.findAll('.pager__btn')[0].attributes('disabled')).toBeUndefined();
    expect(last.findAll('.pager__btn')[1].attributes('disabled')).toBeDefined();
  });

  it('во время загрузки обе кнопки заблокированы', () => {
    const wrapper = mountPager({ page: 2, loading: true });
    const disabled = wrapper.findAll('.pager__btn').map(b => b.attributes('disabled'));
    expect(disabled.every(d => d !== undefined)).toBe(true);
  });

  it('total принимает уже отформатированную строку, а префикс идёт перед номером', () => {
    const wrapper = mountPager({ total: '1 234', pagePrefix: 'Стр. ' });

    expect(wrapper.find('.pager__total').text()).toBe('Всего: 1 234');
    expect(wrapper.find('.pager__page').text()).toBe('Стр. 1 / 3');
  });

  it('слот lead рендерится перед счётчиком', () => {
    const wrapper = mountPager({}, { lead: '<span class="live">в реальном времени</span>' });

    expect(wrapper.find('.live').exists()).toBe(true);
    expect(wrapper.text().indexOf('в реальном времени')).toBeLessThan(wrapper.text().indexOf('Всего'));
  });
});
