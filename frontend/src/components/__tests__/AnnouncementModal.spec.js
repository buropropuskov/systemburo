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
 * Анимацию открытия jsdom не воспроизводит (нет layout и transition), поэтому замок
 * сверяет CSS объявления с ЭТАЛОНОМ - окнами обзора (карточка новости, «Режимы работы»,
 * «Руководство» в NewsAndReview). Открытие уже расходилось дважды: сначала перевод
 * цветов на токены поставил в старт конечный цвет подложки (анимировать нечего), потом
 * объявление фейдило одну подложку и открывалось иначе, чем соседние окна.
 */
describe('AnnouncementModal - открытие как у остальных окон обзора (#1415)', () => {
  const css = readFileSync(resolve(__dirname, '../AnnouncementModal.vue'), 'utf8');
  const etalon = readFileSync(resolve(__dirname, '../../views/NewsAndReview.vue'), 'utf8');

  const rule = (src, head) => {
    const m = src.match(new RegExp(`${head}\\s*\\{([\\s\\S]*?)\\}`));
    return m && m[1].replace(/\/\*[\s\S]*?\*\//g, '').replace(/\s+/g, ' ').trim();
  };
  const enterActive = (src) => rule(src, '\\.modal-fade-enter-active,\\s*\\n\\.modal-fade-leave-active');
  const enterFrom = (src) => rule(src, '\\.modal-fade-enter-from,\\s*\\n\\.modal-fade-leave-to');
  const contentFrom = (src) => rule(src, '\\.modal-fade-enter-from \\.modal-content,\\s*\\n\\.modal-fade-leave-to \\.modal-content');

  it('переход и стартовое состояние совпадают с эталоном', () => {
    expect(enterActive(css)).toBe(enterActive(etalon));
    expect(enterFrom(css)).toBe(enterFrom(etalon));
  });

  it('фейдится весь оверлей по opacity, а не одна подложка', () => {
    // background-color в старте = фейд одной подложки: у соседних окон так не делают,
    // и открытие ощущалось иначе.
    expect(enterFrom(css)).toContain('opacity: 0');
    expect(enterFrom(css), 'подложку отдельно не фейдим').not.toContain('background-color');
  });

  it('лист приходит из scale, как у эталона', () => {
    expect(contentFrom(css)).toBe(contentFrom(etalon));
    expect(contentFrom(css)).toMatch(/scale\(0\.9\)/);
  });
});
