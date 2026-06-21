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
});
