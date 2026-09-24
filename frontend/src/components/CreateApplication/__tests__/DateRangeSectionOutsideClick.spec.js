import { describe, it, expect, afterEach } from 'vitest';
import { mount, enableAutoUnmount } from '@vue/test-utils';
import { defineComponent, h } from 'vue';
import DateRangeSection from '../DateRangeSection.vue';

// Внутри BaseModal (окно правки срока заявки #2575, ручное добавление) корень окна
// гасит всплытие клика (@click.stop). Слушатель на document в фазе всплытия его не
// видел, и календарь оставался открытым поверх поля причины - кликнуть было некуда.
enableAutoUnmount(afterEach);

const InsideStoppedClick = defineComponent({
  render: () => h('div', { onClick: (e) => e.stopPropagation() }, [
    h(DateRangeSection),
    h('textarea', { class: 'reason' }),
  ]),
});

describe('DateRangeSection - закрытие календаря кликом мимо', () => {
  it('закрывается, даже когда предок гасит всплытие клика', async () => {
    const wrapper = mount(InsideStoppedClick, { attachTo: document.body });
    const section = wrapper.findComponent(DateRangeSection);
    section.vm.showStartDatepicker = true;
    section.vm.showQuickMenu = true;

    await wrapper.find('.reason').trigger('click');

    expect(section.vm.showStartDatepicker).toBe(false);
    expect(section.vm.showQuickMenu).toBe(false);
  });

  it('тап на телефоне: палец отпущен на выехавшем затемнении, клик достаётся body - лист остаётся', async () => {
    const wrapper = mount(DateRangeSection, { attachTo: document.body });
    wrapper.vm.showStartDatepicker = true;

    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }));

    expect(wrapper.vm.showStartDatepicker).toBe(true);
  });

  it('клик по полю даты календарь не закрывает', async () => {
    const wrapper = mount(DateRangeSection, { attachTo: document.body });
    wrapper.vm.showStartDatepicker = true;

    await wrapper.find('.datepicker-wrapper input').trigger('click');

    expect(wrapper.vm.showStartDatepicker).toBe(true);
  });

  it('после размонтирования слушатель снят: окно, открытое десять раз, не копит обработчики', async () => {
    const wrapper = mount(DateRangeSection, { attachTo: document.body });
    const vm = wrapper.vm;
    let calls = 0;
    vm.closeDatepicker = () => { calls += 1; };
    wrapper.unmount();

    const outside = document.createElement('div');
    document.body.appendChild(outside);
    outside.click();
    outside.remove();

    expect(calls).toBe(0);
  });
});
