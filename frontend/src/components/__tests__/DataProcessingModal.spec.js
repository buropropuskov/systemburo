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

// PdfDocumentViewer (в цепочке импорта) статически тянет ?worker-конструктор воркера pdf.js;
// мок делает спек герметичным - иначе Vite резолвит реальный ассет из node_modules.
vi.mock('pdfjs-dist/build/pdf.worker.min.mjs?worker', () => ({ default: class {} }));

// Окно берёт текст согласия из стора гейта (#1567); в спеке сеть не нужна.
const getConsentGate = vi.fn();
vi.mock('@/api/pdConsent', () => ({
  getConsentGate: (...a) => getConsentGate(...a),
  acceptConsent: vi.fn(),
}));

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
    setActivePinia(createPinia());
    getConsentGate.mockReset().mockResolvedValue({ required: false, version: 1, text: '', document: null });
    getDataProcessingMeta.mockReset();
    fetchDataProcessingBlob.mockReset();
    downloadDataProcessingDoc.mockReset();
  });

  // #1567: когда администратор задал текст согласия, окно показывает именно его -
  // это та редакция, которую пользователь подтверждает при входе.
  it('показывает текст согласия, когда он задан, и не читает файл', async () => {
    getConsentGate.mockResolvedValue({
      required: false, version: 2, text: '<p>Пункт согласия</p>', document: null,
    });
    getDataProcessingMeta.mockResolvedValue(null);
    // Загрузку запускает watch(show): монтируем закрытым и открываем, как родитель.
    const wrapper = mount(DataProcessingModal, { props: { show: false }, global: { stubs } });
    await wrapper.setProps({ show: true });
    await flushPromises();

    expect(wrapper.find('.dp-modal__text').text()).toContain('Пункт согласия');
    expect(fetchDataProcessingBlob).not.toHaveBeenCalled();
    expect(wrapper.text()).not.toContain('Документ ещё не загружен');
  });

  it('текст рендерится через sanitizeHtml: скрипт и onerror вырезаны', async () => {
    getConsentGate.mockResolvedValue({
      required: false,
      version: 2,
      text: '<p>Текст</p><script>window.__pwnModal = 1;</script><img src="x" onerror="window.__pwnModal = 2">',
      document: null,
    });
    getDataProcessingMeta.mockResolvedValue(null);
    // Загрузку запускает watch(show): монтируем закрытым и открываем, как родитель.
    const wrapper = mount(DataProcessingModal, { props: { show: false }, global: { stubs } });
    await wrapper.setProps({ show: true });
    await flushPromises();

    const html = wrapper.find('.dp-modal__text').html();
    expect(html).toContain('Текст');
    expect(html).not.toContain('<script');
    expect(html).not.toContain('onerror');
    expect(window.__pwnModal).toBeUndefined();
  });

  it('текст показывается и когда файл тоже загружен - PDF уступает тексту', async () => {
    getConsentGate.mockResolvedValue({
      required: false, version: 2, text: '<p>Редакция 2</p>', document: null,
    });
    getDataProcessingMeta.mockResolvedValue({
      file_name: 'soglasie.pdf', mime_type: 'application/pdf', ext: '.pdf', uploaded_at: '',
    });
    // Загрузку запускает watch(show): монтируем закрытым и открываем, как родитель.
    const wrapper = mount(DataProcessingModal, { props: { show: false }, global: { stubs } });
    await wrapper.setProps({ show: true });
    await flushPromises();

    expect(wrapper.find('.dp-modal__text').exists()).toBe(true);
    expect(wrapper.findComponent({ name: 'PdfDocumentViewer' }).exists()).toBe(false);
    expect(fetchDataProcessingBlob).not.toHaveBeenCalled();
  });

  // Ревью поймало: при заданном тексте сбой чтения файла поднимал флаг «загружено»
  // и «грузим раз на сеанс» навсегда запоминал неудачу - повторное открытие уже не
  // пробовало прочитать документ.
  it('сбой чтения файла при заданном тексте показывает текст и не блокирует повтор', async () => {
    getConsentGate.mockResolvedValue({
      required: false, version: 2, text: '<p>Редакция 2</p>', document: null,
    });
    getDataProcessingMeta.mockRejectedValueOnce(new Error('503'));
    const wrapper = mountClosed();

    await wrapper.setProps({ show: true });
    await flushPromises();
    expect(wrapper.find('.dp-modal__text').exists()).toBe(true);
    expect(wrapper.text()).not.toContain('Не удалось загрузить документ');

    getDataProcessingMeta.mockResolvedValue({
      file_name: 'soglasie.pdf', mime_type: 'application/pdf', ext: '.pdf',
    });
    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    await flushPromises();

    expect(getDataProcessingMeta).toHaveBeenCalledTimes(2);
    expect(wrapper.find('.dp-modal__btn--primary').exists()).toBe(true);
  });

  // "<p></p>" - это очищенный редактором документ, а не текст согласия: показывать
  // надо файл, а не пустой лист.
  it('визуально пустой HTML текстом не считается - остаётся файловый путь', async () => {
    getConsentGate.mockResolvedValue({
      required: false, version: 2, text: '<p></p>', document: null,
    });
    getDataProcessingMeta.mockResolvedValue({
      file_name: 'soglasie.pdf', mime_type: 'application/pdf', ext: '.pdf',
    });
    fetchDataProcessingBlob.mockResolvedValue(new Blob(['%PDF']));
    const wrapper = mountClosed();

    await wrapper.setProps({ show: true });
    await flushPromises();

    expect(wrapper.find('.dp-modal__text').exists()).toBe(false);
    expect(wrapper.find('.pdf-stub').exists()).toBe(true);
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
