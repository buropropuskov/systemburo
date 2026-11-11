import { flushPromises, mount } from '@vue/test-utils';
import { expect, it, vi } from 'vitest';
import DocumentsBlock from './DocumentsBlock.vue';
import { downloadDocument } from '@/api/documents';

vi.mock('@/api/documents', () => ({
  listPublicDocuments: vi.fn().mockResolvedValue([{
    id: 1,
    name: 'Инструкции',
    documents: [{ id: 2, title: 'Памятка', file_name: 'guide.pdf', file_ext: 'pdf' }],
  }]),
  downloadDocument: vi.fn().mockResolvedValue(undefined),
}));

it('loads the document preview on first open and keeps it usable after reopening', async () => {
  const wrapper = mount(DocumentsBlock, {
    attachTo: document.body,
    global: { stubs: { teleport: true } },
  });
  try {
    await flushPromises();
    expect(wrapper.find('[data-testid="document-preview"]').exists()).toBe(false);
    await wrapper.get('[data-testid="document-open"]').trigger('click');
    await vi.dynamicImportSettled();
    expect(wrapper.get('[data-testid="document-preview"]').text()).toContain('guide.pdf');

    await wrapper.get('[aria-label="Закрыть"]').trigger('click');
    expect(wrapper.find('[data-testid="document-preview"]').exists()).toBe(false);
    await wrapper.get('[data-testid="document-open"]').trigger('click');
    await wrapper.get('[role="dialog"]').findAll('button')
      .find(button => button.text().includes('Скачать')).trigger('click');
    expect(downloadDocument).toHaveBeenCalledWith(2, 'guide.pdf');
  } finally {
    wrapper.unmount();
  }
});
