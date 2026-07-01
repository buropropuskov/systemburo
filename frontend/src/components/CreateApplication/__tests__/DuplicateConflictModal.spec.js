import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import DuplicateConflictModal from '../DuplicateConflictModal.vue';

// #952: модалка конфликта дублирования - 3 кнопки с подсказками (title) и emit'ами.
function mountModal(show = true) {
  return mount(DuplicateConflictModal, {
    props: { show },
    global: { stubs: { teleport: true } },
  });
}

const btn = (w, text) => w.findAll('.dup-conflict-dialog__btn').find(b => b.text() === text);

describe('DuplicateConflictModal (#952)', () => {
  it('show=false -> не рендерится', () => {
    const w = mountModal(false);
    expect(w.find('.dup-conflict-dialog').exists()).toBe(false);
  });

  it('show=true -> три кнопки с текстом и подсказками (data-hint, стиль #333)', () => {
    const w = mountModal(true);
    expect(w.find('.dup-conflict-dialog').exists()).toBe(true);

    const replace = btn(w, 'Заменить');
    const merge = btn(w, 'Объединить');
    const cancel = btn(w, 'Отмена');
    expect(replace && merge && cancel).toBeTruthy();

    // Подсказки при наведении - системный стиль #333 через data-hint (::after content).
    expect(replace.attributes('data-hint')).toMatch(/Удалит текущие данные/);
    expect(merge.attributes('data-hint')).toMatch(/Добавит вложения/);
    expect(cancel.attributes('data-hint')).toMatch(/Оставит текущие данные/);
  });

  it('клики по кнопкам эмитят replace / merge / cancel', async () => {
    const w = mountModal(true);
    await btn(w, 'Заменить').trigger('click');
    await btn(w, 'Объединить').trigger('click');
    await btn(w, 'Отмена').trigger('click');
    expect(w.emitted('replace')).toHaveLength(1);
    expect(w.emitted('merge')).toHaveLength(1);
    expect(w.emitted('cancel')).toHaveLength(1);
  });

  it('клик по оверлею -> cancel', async () => {
    const w = mountModal(true);
    await w.find('.dup-conflict-overlay').trigger('click');
    expect(w.emitted('cancel')).toHaveLength(1);
  });
});
