import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const setBureauNote = vi.fn();
vi.mock('@/api/applications', () => ({
  setBureauNote: (...args) => setBureauNote(...args),
}));

import ApplicationBureauNote from '../ApplicationBureauNote.vue';

const NOTE = { text: 'Ждём паспорт водителя', author_name: 'Иванов Иван', updated_at: '2026-08-25T10:00:00Z' };

function mountNote(note = null) {
  return mount(ApplicationBureauNote, { props: { applicationId: 7, note } });
}

describe('ApplicationBureauNote', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    setBureauNote.mockReset();
  });

  it('без заметки предлагает её добавить, текста и метаданных не показывает', () => {
    const wrapper = mountNote();
    expect(wrapper.find('[data-testid="bureau-note-add"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="bureau-note-text"]').exists()).toBe(false);
  });

  // Заметка живёт строкой в потоке карточки, а не отдельным блоком: автору и
  // времени в строке места нет, они ушли в подсказку. Информация не потеряна -
  // проверяем её там, где она теперь показывается.
  it('показывает текст заметки, а автора и время держит в подсказке', () => {
    const wrapper = mountNote(NOTE);
    const text = wrapper.find('[data-testid="bureau-note-text"]');

    expect(text.text()).toBe(NOTE.text);

    const title = text.attributes('title');
    expect(title).toContain(NOTE.text);
    expect(title).toContain('Иванов Иван');
    expect(title).toContain('25.08.2026');
  });

  // Длинная заметка не должна растить карточку заявки.
  it('текст заметки сжимается в одну строку', () => {
    const wrapper = mountNote(NOTE);
    const classes = wrapper.find('[data-testid="bureau-note-text"]').classes();
    expect(classes).toContain('bureau-note__text');
  });

  it('сохранение шлёт новый текст и отдаёт ответ наверх', async () => {
    const saved = { ...NOTE, text: 'Заявитель дозагрузит доверенность' };
    setBureauNote.mockResolvedValue(saved);

    const wrapper = mountNote(NOTE);
    await wrapper.find('[data-testid="bureau-note-edit"]').trigger('click');
    await wrapper.find('[data-testid="bureau-note-input"]').setValue('Заявитель дозагрузит доверенность');
    await wrapper.find('[data-testid="bureau-note-save"]').trigger('click');
    await new Promise(r => setTimeout(r, 0));

    expect(setBureauNote).toHaveBeenCalledWith(7, 'Заявитель дозагрузит доверенность');
    expect(wrapper.emitted('update')[0]).toEqual([saved]);
  });

  // Очистка идёт тем же методом с пустым текстом - отдельного удаления на бэке нет.
  it('очистка шлёт пустой текст и отдаёт наверх null', async () => {
    setBureauNote.mockResolvedValue(null);

    const wrapper = mountNote(NOTE);
    await wrapper.find('[data-testid="bureau-note-clear"]').trigger('click');
    await new Promise(r => setTimeout(r, 0));

    expect(setBureauNote).toHaveBeenCalledWith(7, '');
    expect(wrapper.emitted('update')[0]).toEqual([null]);
  });

  // Повторное нажатие без правки переписало бы автора и время у неизменившегося текста.
  it('сохранение неизменённого текста заблокировано', async () => {
    const wrapper = mountNote(NOTE);
    await wrapper.find('[data-testid="bureau-note-edit"]').trigger('click');
    expect(wrapper.find('[data-testid="bureau-note-save"]').attributes('disabled')).toBeDefined();
  });

  it('отказ бэка не роняет блок и не сообщает об успехе', async () => {
    setBureauNote.mockRejectedValue(new Error('Заметку бюро ведут только принимающие'));

    const wrapper = mountNote(NOTE);
    await wrapper.find('[data-testid="bureau-note-clear"]').trigger('click');
    await new Promise(r => setTimeout(r, 0));

    expect(wrapper.emitted('update')).toBeUndefined();
    expect(wrapper.find('[data-testid="bureau-note-text"]').text()).toBe(NOTE.text);
  });
});
