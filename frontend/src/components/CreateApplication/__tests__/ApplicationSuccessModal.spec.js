import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { useDeletionsStore } from '@/stores/deletions';

import ApplicationSuccessModal from '../ApplicationSuccessModal.vue';

function mountModal(props = {}) {
  return mount(ApplicationSuccessModal, {
    props: { show: true, applicationNumber: 'A-1024', ...props },
    global: { stubs: { teleport: true } },
    attachTo: document.body,
  });
}

describe('ApplicationSuccessModal — копирование номера (эталон notify)', () => {
  let notify;

  beforeEach(() => {
    setActivePinia(createPinia());
    notify = vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {});
  });

  it('успешное копирование -> notify(success) с номером, без useToast', async () => {
    const writeText = vi.fn().mockResolvedValue();
    Object.assign(navigator, { clipboard: { writeText } });

    const wrapper = mountModal();
    await wrapper.find('.number--copyable').trigger('click');
    await Promise.resolve();

    expect(writeText).toHaveBeenCalledWith('A-1024');
    expect(notify).toHaveBeenCalledTimes(1);
    const arg = notify.mock.calls[0][0];
    expect(arg.type).toBe('success');
    expect(arg.bold).toBe('A-1024');
    expect(`${arg.prefix}${arg.bold}${arg.suffix}`).toContain('A-1024');
    expect(`${arg.prefix}${arg.suffix}`).toContain('скопирован');
  });

  it('сбой копирования -> notify(error)', async () => {
    Object.assign(navigator, { clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) } });
    // execCommand-фолбэк тоже роняем, чтобы попасть в catch
    document.execCommand = vi.fn(() => { throw new Error('no exec'); });

    const wrapper = mountModal();
    await wrapper.find('.number--copyable').trigger('click');
    await Promise.resolve();
    await Promise.resolve();

    expect(notify).toHaveBeenCalledTimes(1);
    expect(notify.mock.calls[0][0].type).toBe('error');
    expect(notify.mock.calls[0][0].prefix).toContain('Не удалось');
  });

  it('пустой номер -> ничего не копирует и не нотифаит', async () => {
    const writeText = vi.fn().mockResolvedValue();
    Object.assign(navigator, { clipboard: { writeText } });

    const wrapper = mountModal({ applicationNumber: '' });
    await wrapper.find('.number--copyable').trigger('click');
    await Promise.resolve();

    expect(writeText).not.toHaveBeenCalled();
    expect(notify).not.toHaveBeenCalled();
  });
});
