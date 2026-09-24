import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import DocumentPreviewModal from '../DocumentPreviewModal.vue';

const документ = {
  id: 7,
  title: 'Автозаявка',
  file_name: 'Автозаявка.xlsx',
  file_ext: '.xlsx',
  file_size: 430848,
  published_at: '2026-06-20T09:07:21Z',
  description: 'Заполнить и подписать',
  comment: 'Оба листа, подпись руководителя, нести в бюро',
};

function смонтировать(props = {}) {
  return mount(DocumentPreviewModal, {
    props: { show: true, doc: документ, ...props },
    global: { stubs: { teleport: true, BaseModal: { template: '<div><slot /><slot name="actions" /></div>' } } },
  });
}

describe('окно документа', () => {
  it('показывает файл и описание от бюро', () => {
    const w = смонтировать();
    const текст = w.text();
    expect(текст).toContain('Автозаявка.xlsx');
    expect(текст).toContain('Оба листа, подпись руководителя, нести в бюро');
  });

  it('без описания от бюро показывает короткую подпись из списка', () => {
    const w = смонтировать({ doc: { ...документ, comment: null } });
    expect(w.find('[data-testid="document-comment"]').text()).toBe('Заполнить и подписать');
  });

  it('когда описания нет совсем, говорит об этом прямо, а не оставляет пустоту', () => {
    const w = смонтировать({ doc: { ...документ, comment: null, description: null } });
    expect(w.find('[data-testid="document-comment"]').exists()).toBe(false);
    expect(w.find('[data-testid="document-comment-empty"]').text()).toBe('Описания нет');
  });

  it('сам файл не показывает - ни картинкой, ни встроенным просмотром', () => {
    const w = смонтировать();
    expect(w.find('iframe').exists()).toBe(false);
    expect(w.find('embed').exists()).toBe(false);
    expect(w.find('canvas').exists()).toBe(false);
  });

  it('кнопка скачивания отдаёт документ наружу', async () => {
    const w = смонтировать();
    await w.find('[data-testid="document-download"]').trigger('click');
    expect(w.emitted('download')[0][0]).toEqual(документ);
  });

  it('во время скачивания кнопка заблокирована', () => {
    const w = смонтировать({ downloading: true });
    const кнопка = w.find('[data-testid="document-download"]');
    expect(кнопка.attributes('disabled')).toBeDefined();
    expect(кнопка.text()).toContain('Скачивание');
  });

  it('размер файла человекочитаемый', () => {
    expect(смонтировать().text()).toContain('421 КБ');
    expect(смонтировать({ doc: { ...документ, file_size: 3 * 1024 * 1024 } }).text()).toContain('3.0 МБ');
  });
});
