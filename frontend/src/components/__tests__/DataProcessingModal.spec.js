import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

const getDataProcessingMeta = vi.fn();
const fetchDataProcessingBlob = vi.fn();
const downloadDataProcessingDoc = vi.fn();

vi.mock('@/api/dataProcessing', () => ({
  getDataProcessingMeta: (...a) => getDataProcessingMeta(...a),
  fetchDataProcessingBlob: (...a) => fetchDataProcessingBlob(...a),
  downloadDataProcessingDoc: (...a) => downloadDataProcessingDoc(...a),
}));

// PdfDocumentViewer (в цепочке импорта) статически тянет ?url-ассет воркера pdf.js;
// мок делает спек герметичным - иначе Vite резолвит реальный ассет из node_modules.
vi.mock('pdfjs-dist/build/pdf.worker.min.mjs?url', () => ({ default: 'worker-url' }));

import DataProcessingModal from '../DataProcessingModal.vue';

// BaseModal телепортится в body - подменяем простым враппером, чтобы читать содержимое
// через wrapper и не гоняться за teleport; PdfDocumentViewer подменяем заглушкой.
const stubs = {
  BaseModal: {
    name: 'BaseModal',
    props: ['show'],
    template: '<div v-if="show" class="base-modal-stub"><slot /><slot name="actions" /></div>',
  },
  PdfDocumentViewer: {
    name: 'PdfDocumentViewer',
    props: ['blob'],
    template: '<div class="pdf-stub" />',
  },
};

function mountClosed() {
  return mount(DataProcessingModal, { props: { show: false }, global: { stubs } });
}

describe('DataProcessingModal', () => {
  beforeEach(() => {
    getDataProcessingMeta.mockReset();
    fetchDataProcessingBlob.mockReset();
    downloadDataProcessingDoc.mockReset();
  });

  it('на открытии грузит документ и показывает PDF во просмотрщике', async () => {
    getDataProcessingMeta.mockResolvedValue({
      file_name: 'soglasie.pdf', mime_type: 'application/pdf', ext: '.pdf',
    });
    fetchDataProcessingBlob.mockResolvedValue(new Blob(['%PDF']));
    const wrapper = mountClosed();

    await wrapper.setProps({ show: true });
    await flushPromises();

    expect(getDataProcessingMeta).toHaveBeenCalledTimes(1);
    expect(fetchDataProcessingBlob).toHaveBeenCalledTimes(1);
    expect(wrapper.find('.pdf-stub').exists()).toBe(true);
  });

  it('для DOCX показывает скачивание, без просмотрщика и без загрузки blob', async () => {
    getDataProcessingMeta.mockResolvedValue({
      file_name: 'soglasie.docx',
      mime_type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
      ext: '.docx',
    });
    const wrapper = mountClosed();

    await wrapper.setProps({ show: true });
    await flushPromises();

    expect(fetchDataProcessingBlob).not.toHaveBeenCalled();
    expect(wrapper.find('.pdf-stub').exists()).toBe(false);
    expect(wrapper.text()).toContain('soglasie.docx');
  });

  it('кнопка «Скачать» скачивает документ с именем файла', async () => {
    getDataProcessingMeta.mockResolvedValue({
      file_name: 'soglasie.pdf', mime_type: 'application/pdf', ext: '.pdf',
    });
    fetchDataProcessingBlob.mockResolvedValue(new Blob(['%PDF']));
    downloadDataProcessingDoc.mockResolvedValue();
    const wrapper = mountClosed();

    await wrapper.setProps({ show: true });
    await flushPromises();
    await wrapper.find('.dp-modal__btn--primary').trigger('click');
    await flushPromises();

    expect(downloadDataProcessingDoc).toHaveBeenCalledWith('soglasie.pdf');
  });

  it('при ошибке загрузки показывает сообщение и кнопку повтора', async () => {
    getDataProcessingMeta.mockRejectedValue(new Error('boom'));
    const wrapper = mountClosed();

    await wrapper.setProps({ show: true });
    await flushPromises();

    expect(wrapper.text()).toContain('Не удалось загрузить документ');
    expect(wrapper.find('.dp-modal__btn--ghost').exists()).toBe(true);
  });

  it('повторное открытие не перезапрашивает документ', async () => {
    getDataProcessingMeta.mockResolvedValue({
      file_name: 'soglasie.pdf', mime_type: 'application/pdf', ext: '.pdf',
    });
    fetchDataProcessingBlob.mockResolvedValue(new Blob(['%PDF']));
    const wrapper = mountClosed();

    await wrapper.setProps({ show: true });
    await flushPromises();
    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    await flushPromises();

    expect(getDataProcessingMeta).toHaveBeenCalledTimes(1);
  });
});
