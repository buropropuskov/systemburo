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

// PdfDocumentViewer (мобильная ветка) статически тянет ?url-ассет воркера pdf.js;
// мок делает спек герметичным - иначе Vite резолвит реальный ассет из node_modules.
vi.mock('pdfjs-dist/build/pdf.worker.min.mjs?url', () => ({ default: 'worker-url' }));

import DataProcessingView from '../DataProcessingView.vue';

describe('DataProcessingView', () => {
  beforeEach(() => {
    getDataProcessingMeta.mockReset();
    fetchDataProcessingBlob.mockReset();
    downloadDataProcessingDoc.mockReset();
    global.URL.createObjectURL = vi.fn(() => 'blob:mock-url');
    global.URL.revokeObjectURL = vi.fn();
  });

  it('показывает пустое состояние, если документ не загружен', async () => {
    getDataProcessingMeta.mockResolvedValue(null);
    const wrapper = mount(DataProcessingView);
    await flushPromises();

    expect(wrapper.text()).toContain('Документ ещё не загружен');
    expect(wrapper.find('.dp-pdf').exists()).toBe(false);
    expect(fetchDataProcessingBlob).not.toHaveBeenCalled();
  });

  it('встраивает PDF через object URL', async () => {
    getDataProcessingMeta.mockResolvedValue({
      file_name: 'soglasie.pdf', mime_type: 'application/pdf', ext: '.pdf', uploaded_at: '',
    });
    fetchDataProcessingBlob.mockResolvedValue(new Blob(['%PDF'], { type: 'application/pdf' }));
    const wrapper = mount(DataProcessingView);
    await flushPromises();

    const embed = wrapper.find('.dp-pdf');
    expect(embed.exists()).toBe(true);
    expect(embed.attributes('src')).toBe('blob:mock-url');
    expect(fetchDataProcessingBlob).toHaveBeenCalledTimes(1);
  });

  it('для DOCX показывает только скачивание, без встраивания', async () => {
    getDataProcessingMeta.mockResolvedValue({
      file_name: 'soglasie.docx',
      mime_type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
      ext: '.docx',
      uploaded_at: '',
    });
    const wrapper = mount(DataProcessingView);
    await flushPromises();

    expect(wrapper.find('.dp-pdf').exists()).toBe(false);
    expect(wrapper.text()).toContain('soglasie.docx');
    expect(wrapper.text()).toContain('Скачайте документ');
    expect(fetchDataProcessingBlob).not.toHaveBeenCalled();
  });

  it('кнопка «Скачать» вызывает загрузку с именем файла', async () => {
    getDataProcessingMeta.mockResolvedValue({
      file_name: 'soglasie.pdf', mime_type: 'application/pdf', ext: '.pdf', uploaded_at: '',
    });
    fetchDataProcessingBlob.mockResolvedValue(new Blob(['%PDF']));
    downloadDataProcessingDoc.mockResolvedValue();
    const wrapper = mount(DataProcessingView);
    await flushPromises();

    await wrapper.find('.dp-button--primary').trigger('click');
    expect(downloadDataProcessingDoc).toHaveBeenCalledWith('soglasie.pdf');
  });

  it('на мобилке рендерит PDF через pdf.js-просмотрщик, а не <embed>', async () => {
    vi.stubGlobal('matchMedia', vi.fn(() => ({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })));
    getDataProcessingMeta.mockResolvedValue({
      file_name: 'soglasie.pdf', mime_type: 'application/pdf', ext: '.pdf', uploaded_at: '',
    });
    fetchDataProcessingBlob.mockResolvedValue(new Blob(['%PDF'], { type: 'application/pdf' }));
    const wrapper = mount(DataProcessingView, {
      global: {
        stubs: {
          PdfDocumentViewer: {
            name: 'PdfDocumentViewer', props: ['blob'], template: '<div class="pdf-stub" />',
          },
        },
      },
    });
    await flushPromises();

    expect(wrapper.find('.dp-pdf').exists()).toBe(false);
    expect(wrapper.find('.pdf-stub').exists()).toBe(true);
    vi.unstubAllGlobals();
  });
});
