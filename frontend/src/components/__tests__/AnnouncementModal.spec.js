import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mount } from '@vue/test-utils';
import AnnouncementModal from '../AnnouncementModal.vue';

const announcement = {
  title: 'Тест',
  description: 'Описание',
  created_at: '2026-07-12T15:00:00Z',
  is_important: false,
};

function mountM(props = {}) {
  return mount(AnnouncementModal, {
    props: { show: true, announcement, ...props },
    attachTo: document.body,
  });
}

describe('AnnouncementModal - bottom-sheet close-пути (#1097 W3)', () => {
  beforeEach(() => { document.body.innerHTML = ''; });
  afterEach(() => { document.body.innerHTML = ''; });

  it('рендерит контент и ползунок bottom-sheet', () => {
    const wrapper = mountM();
    expect(document.body.querySelector('.announcement-modal')).not.toBeNull();
    expect(document.body.querySelector('.sheet-handle')).not.toBeNull();
    expect(document.body.textContent).toContain('Тест');
    wrapper.unmount();
  });

  it('кнопка Закрыть эмитит update:show=false и close', async () => {
    const wrapper = mountM();
    document.body.querySelector('.close-modal-btn').dispatchEvent(new Event('click', { bubbles: true }));
    expect(wrapper.emitted('update:show')[0]).toEqual([false]);
    expect(wrapper.emitted('close')).toBeTruthy();
    wrapper.unmount();
  });

  it('Escape закрывает когда show', () => {
    const wrapper = mountM();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(wrapper.emitted('close')).toBeTruthy();
    wrapper.unmount();
  });

  it('клик по overlay (@click.self) закрывает', () => {
    const wrapper = mountM();
    const overlay = document.body.querySelector('.modal-overlay');
    // dispatchEvent на оверлей ставит event.target=overlay -> @click.self срабатывает.
    overlay.dispatchEvent(new Event('click', { bubbles: true }));
    expect(wrapper.emitted('close')).toBeTruthy();
    wrapper.unmount();
  });
});
