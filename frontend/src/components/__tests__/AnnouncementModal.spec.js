import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
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

/*
 * Анимацию открытия jsdom не воспроизводит (нет layout и transition), поэтому
 * замок читает сам CSS компонента. Оба правила уже терялись: перевод цветов на
 * токены поставил в enter-from конечный цвет подложки (анимировать стало нечего),
 * а transition оверлея до листа не достаёт - лист прыгал за один кадр.
 */
describe('AnnouncementModal - анимация открытия (#1415)', () => {
  const css = readFileSync(resolve(__dirname, '../AnnouncementModal.vue'), 'utf8');

  it('стартовое состояние подложки прозрачное, а не конечный цвет', () => {
    const enterFrom = css.match(/\.modal-fade-enter-from,\s*\n\.modal-fade-leave-to\s*\{([\s\S]*?)\}/);
    expect(enterFrom, 'нет правила стартового состояния перехода').not.toBeNull();
    const bg = enterFrom[1].match(/background-color:\s*([^;]+);/);
    expect(bg, 'старт должен задавать цвет подложки').not.toBeNull();
    expect(bg[1].trim()).toBe('transparent');
    expect(bg[1], 'var(--overlay) - это КОНЕЧНЫЙ цвет, фейда не будет').not.toContain('--overlay');
  });

  it('у листа собственный transition: с оверлея он не наследуется', () => {
    const contentTransition = css.match(
      /\.modal-fade-enter-active\s+\.modal-content,\s*\n\.modal-fade-leave-active\s+\.modal-content\s*\{([\s\S]*?)\}/,
    );
    expect(contentTransition, 'лист остался без своего transition - fade+scale за один кадр').not.toBeNull();
    expect(contentTransition[1]).toMatch(/transition:[^;]*opacity/);
    expect(contentTransition[1]).toMatch(/transition:[^;]*transform/);
  });
});
