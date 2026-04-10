import { describe, it, expect, beforeEach } from 'vitest';
import { useDropdownState } from '../useDropdownState';
import { mount, config } from '@vue/test-utils';
import { defineComponent } from 'vue';

function createWrapperComponent(containerSelector) {
  return defineComponent({
    setup() {
      return useDropdownState(containerSelector);
    },
    template: '<div></div>',
  });
}

describe('useDropdownState', () => {
  let wrapper;

  beforeEach(() => {
    wrapper = mount(createWrapperComponent('.dropdown'));
  });

  it('starts closed', () => {
    expect(wrapper.vm.isOpen).toBe(false);
  });

  describe('toggle', () => {
    it('opens when closed', () => {
      wrapper.vm.toggle();
      expect(wrapper.vm.isOpen).toBe(true);
    });

    it('closes when open', () => {
      wrapper.vm.toggle();
      wrapper.vm.toggle();
      expect(wrapper.vm.isOpen).toBe(false);
    });
  });

  describe('close', () => {
    it('sets isOpen to false', () => {
      wrapper.vm.toggle();
      expect(wrapper.vm.isOpen).toBe(true);
      wrapper.vm.close();
      expect(wrapper.vm.isOpen).toBe(false);
    });

    it('is idempotent when already closed', () => {
      wrapper.vm.close();
      expect(wrapper.vm.isOpen).toBe(false);
    });
  });
});
