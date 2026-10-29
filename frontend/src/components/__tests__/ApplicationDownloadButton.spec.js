import { describe, it, expect } from 'vitest';
import { defineComponent } from 'vue';
import { mount } from '@vue/test-utils';
import ApplicationDownloadButton, { openApplicationDownload } from '../applications/ApplicationDownloadButton.vue';

describe('ApplicationDownloadButton', () => {
  it('keeps the download name and onboarding anchor on the native button', () => {
    const wrapper = mount(ApplicationDownloadButton, { attrs: { 'data-testid': 'ob-application-download' } });
    const button = wrapper.get('button[data-testid="ob-application-download"]');
    expect(button.attributes('title')).toBe('Скачать');
    expect(button.text()).toBe('Скачать');
    expect(button.find('svg').exists()).toBe(true);
    wrapper.unmount();
  });

  it('opens the chosen application without triggering the containing row', async () => {
    const application = { id: 42, application_number: '20261030/001' };
    const Host = defineComponent({
      components: { ApplicationDownloadButton },
      data: () => ({ selected: false, showDownloadModal: false, downloadAppId: null, downloadAppInfo: null, application }),
      methods: { download() { openApplicationDownload(this, this.application); } },
      template: '<div @click="selected = true"><ApplicationDownloadButton @click.stop="download" /></div>',
    });
    const wrapper = mount(Host);
    await wrapper.get('button').trigger('click');
    expect(wrapper.vm.selected).toBe(false);
    expect(wrapper.vm.showDownloadModal).toBe(true);
    expect(wrapper.vm.downloadAppId).toBe(42);
    expect(wrapper.vm.downloadAppInfo).toBe(wrapper.vm.application);
    expect(application).toEqual({ id: 42, application_number: '20261030/001' });
    wrapper.unmount();
  });
});
