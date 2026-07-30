import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

const getDataProcessingMeta = vi.fn();
const fetchDataProcessingBlob = vi.fn();
const downloadDataProcessingDoc = vi.fn();

vi.mock('@/api/dataProcessing', () => ({
  getDataProcessingMeta: (...a) => getDataProcessingMeta(...a),
  fetchDataProcessingBlob: (...a) => fetchDataProcessingBlob(...a),
  downloadDataProcessingDoc: (...a) => downloadDataProcessingDoc(...a),
}));

// PdfDocumentViewer (мобильная ветка) статически тянет ?worker-конструктор воркера pdf.js;
// мок делает спек герметичным - иначе Vite резолвит реальный ассет из node_modules.
vi.mock('pdfjs-dist/build/pdf.worker.min.mjs?worker', () => ({ default: class {} }));

// Страница берёт текст согласия из стора гейта (#1567); в спеке сеть не нужна.
const getConsentGate = vi.fn();
vi.mock('@/api/pdConsent', () => ({
  getConsentGate: (...a) => getConsentGate(...a),
  acceptConsent: vi.fn(),
}));

import DataProcessingView from '../DataProcessingView.vue';

describe('DataProcessingView', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    getConsentGate.mockReset().mockResolvedValue({ required: false, version: 1, text: '', document: null });
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

  // #1567: когда администратор задал текст согласия, страница показывает именно его -
  // это та редакция, которую пользователь подтверждает при входе.
  it('показывает текст согласия, когда он задан, и не читает файл', async () => {
    getConsentGate.mockResolvedValue({
      required: false, version: 2, text: '<p>Пункт согласия</p>', document: null,
    });
    getDataProcessingMeta.mockResolvedValue(null);
    const wrapper = mount(DataProcessingView);
    await flushPromises();

    expect(wrapper.find('.dp-text').text()).toContain('Пункт согласия');
    expect(fetchDataProcessingBlob).not.toHaveBeenCalled();
    expect(wrapper.find('.dp-pdf').exists()).toBe(false);
    expect(wrapper.text()).not.toContain('Документ ещё не загружен');
  });

  it('текст рендерится через sanitizeHtml: скрипт и onerror вырезаны', async () => {
    getConsentGate.mockResolvedValue({
      required: false,
      version: 2,
      text: '<p>Текст</p><script>window.__pwnView = 1;</script><img src="x" onerror="window.__pwnView = 2">',
      document: null,
    });
    getDataProcessingMeta.mockResolvedValue(null);
    const wrapper = mount(DataProcessingView);
    await flushPromises();

    const html = wrapper.find('.dp-text').html();
    expect(html).toContain('Текст');
    expect(html).not.toContain('<script');
    expect(html).not.toContain('onerror');
    expect(window.__pwnView).toBeUndefined();
  });

  it('текст показывается и когда файл тоже загружен - PDF уступает тексту', async () => {
    getConsentGate.mockResolvedValue({
      required: false, version: 2, text: '<p>Редакция 2</p>', document: null,
    });
    getDataProcessingMeta.mockResolvedValue({
      file_name: 'soglasie.pdf', mime_type: 'application/pdf', ext: '.pdf', uploaded_at: '',
    });
    const wrapper = mount(DataProcessingView);
    await flushPromises();

    expect(wrapper.find('.dp-text').exists()).toBe(true);
    expect(wrapper.find('.dp-pdf').exists()).toBe(false);
    expect(fetchDataProcessingBlob).not.toHaveBeenCalled();
    // Кнопка скачивания остаётся: файл рядом с текстом никуда не делся.
    expect(wrapper.find('.dp-button--primary').exists()).toBe(true);
  });

  it('сбой чтения файла при заданном тексте не превращает страницу в ошибку', async () => {
    getConsentGate.mockResolvedValue({
      required: false, version: 2, text: '<p>Редакция 2</p>', document: null,
    });
    getDataProcessingMeta.mockRejectedValue(new Error('503'));
    const wrapper = mount(DataProcessingView);
    await flushPromises();

    expect(wrapper.find('.dp-text').exists()).toBe(true);
    expect(wrapper.find('.dp-state--error').exists()).toBe(false);
  });

  // "<p></p>" - это очищенный редактором документ, а не текст согласия: показывать
  // надо файл, а не пустой лист.
  it('визуально пустой HTML текстом не считается - остаётся файловый путь', async () => {
    getConsentGate.mockResolvedValue({
      required: false, version: 2, text: '<p></p>', document: null,
    });
    getDataProcessingMeta.mockResolvedValue({
      file_name: 'soglasie.pdf', mime_type: 'application/pdf', ext: '.pdf', uploaded_at: '',
    });
    fetchDataProcessingBlob.mockResolvedValue(new Blob(['%PDF'], { type: 'application/pdf' }));
    const wrapper = mount(DataProcessingView);
    await flushPromises();

    expect(wrapper.find('.dp-text').exists()).toBe(false);
    expect(wrapper.find('.dp-pdf').exists()).toBe(true);
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
