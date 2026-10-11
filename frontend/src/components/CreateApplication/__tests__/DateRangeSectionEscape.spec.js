import { mount } from '@vue/test-utils';
import { describe, it, expect, afterEach } from 'vitest';
import DateRangeSection from '../DateRangeSection.vue';
import BaseModal from '@/components/ui/BaseModal.vue';
import { resetModalStack } from '@/utils/modalStack';

const opened = [];
afterEach(() => {
  opened.splice(0).reverse().forEach(wrapper => wrapper.unmount());
  resetModalStack();
});

describe('DateRangeSection: Escape внутри модального окна', () => {
  for (const parentFirst of [true, false]) {
    it(`закрывает только календарь независимо от порядка слушателей (${parentFirst})`, async () => {
      let modal;
      let dates;
      const openModal = () => {
        modal = mount(BaseModal, { props: { show: true, title: 'Окно' }, global: { stubs: { teleport: true } } });
        opened.push(modal);
      };
      const openDates = () => {
        dates = mount(DateRangeSection, { props: { singleMoment: true }, attachTo: document.body });
        opened.push(dates);
      };
      if (parentFirst) { openModal(); openDates(); } else { openDates(); openModal(); }
      for (const field of ['showStartDatepicker', 'showEndDatepicker', 'showSingleDatepicker']) {
        dates.vm[field] = true;
        dates.element.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
        await dates.vm.$nextTick();
        expect(dates.vm[field]).toBe(false);
        expect(modal.emitted('close')).toBeUndefined();
      }
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
      expect(modal.emitted('close')).toHaveLength(1);
    });
  }
});
