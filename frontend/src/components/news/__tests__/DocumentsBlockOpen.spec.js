import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const listPublicDocuments = vi.fn();
const downloadDocument = vi.fn();
vi.mock('@/api/documents', () => ({
  listPublicDocuments: (...a) => listPublicDocuments(...a),
  downloadDocument: (...a) => downloadDocument(...a),
}));

import DocumentsBlock from '../DocumentsBlock.vue';

const документ = {
  id: 7, title: 'Автозаявка', file_name: 'Автозаявка.xlsx', file_ext: '.xlsx',
  file_size: 430848, published_at: '2026-06-20T09:07:21Z', comment: 'Оба листа',
};

describe('документы на обзоре', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    listPublicDocuments.mockResolvedValue([{ id: 1, name: 'Бланки', documents: [документ] }]);
    downloadDocument.mockResolvedValue();
  });

  it('клик по документу открывает окно, а не качает файл', async () => {
    const w = mount(DocumentsBlock, { global: { stubs: { teleport: true } } });
    await flushPromises();

    await w.find('[data-testid="document-open"]').trigger('click');
    await flushPromises();

    expect(downloadDocument).not.toHaveBeenCalled();
    expect(w.findComponent({ name: 'DocumentPreviewModal' }).props('show')).toBe(true);
    expect(w.findComponent({ name: 'DocumentPreviewModal' }).props('doc')).toMatchObject({ id: 7 });
  });

  it('скачивание идёт из окна', async () => {
    const w = mount(DocumentsBlock, { global: { stubs: { teleport: true } } });
    await flushPromises();
    await w.find('[data-testid="document-open"]').trigger('click');
    await flushPromises();

    w.findComponent({ name: 'DocumentPreviewModal' }).vm.$emit('download', документ);
    await flushPromises();

    expect(downloadDocument).toHaveBeenCalledWith(7, 'Автозаявка.xlsx');
  });
});
