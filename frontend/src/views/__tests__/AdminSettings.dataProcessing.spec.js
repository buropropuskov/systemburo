import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

const getSettings = vi.fn();
const updateSetting = vi.fn();
vi.mock('@/api/settings', () => ({
  getSettings: (...a) => getSettings(...a),
  updateSetting: (...a) => updateSetting(...a),
}));

const getDataProcessingMeta = vi.fn();
const uploadDataProcessingDoc = vi.fn();
const deleteDataProcessingDoc = vi.fn();
const downloadDataProcessingDoc = vi.fn();
const fetchDataProcessingBlob = vi.fn();
vi.mock('@/api/dataProcessing', () => ({
  getDataProcessingMeta: (...a) => getDataProcessingMeta(...a),
  uploadDataProcessingDoc: (...a) => uploadDataProcessingDoc(...a),
  deleteDataProcessingDoc: (...a) => deleteDataProcessingDoc(...a),
  downloadDataProcessingDoc: (...a) => downloadDataProcessingDoc(...a),
  fetchDataProcessingBlob: (...a) => fetchDataProcessingBlob(...a),
}));

// Секция настроек грузит вместе с документом и текст согласия (#1567) - мокаем,
// чтобы спека оставалась герметичной и не уходила в сеть.
const getPDConsentSettings = vi.fn();
vi.mock('@/api/pdConsent', () => ({
  getPDConsentSettings: (...a) => getPDConsentSettings(...a),
  savePDConsentText: vi.fn(),
  setPDConsentRequired: vi.fn(),
  requirePDConsentAgain: vi.fn(),
}));

import AdminSettings from '../AdminSettings.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';

async function mountView() {
  getSettings.mockResolvedValue([]);
  const wrapper = shallowMount(AdminSettings);
  await flushPromises();
  return wrapper;
}

const pdfMeta = { file_name: 'soglasie.pdf', mime_type: 'application/pdf', ext: '.pdf', uploaded_at: '2026-06-21T10:00:00Z' };

describe('AdminSettings - обработка данных', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    [getSettings, updateSetting, getDataProcessingMeta, uploadDataProcessingDoc,
      deleteDataProcessingDoc, downloadDataProcessingDoc, fetchDataProcessingBlob,
      getPDConsentSettings].forEach((m) => m.mockReset());
    getPDConsentSettings.mockResolvedValue({ text: '', version: 1, required: false });
  });

  it('подгружает документ при первом открытии секции (lazy)', async () => {
    getDataProcessingMeta.mockResolvedValue(pdfMeta);
    const wrapper = await mountView();

    expect(getDataProcessingMeta).not.toHaveBeenCalled();
    wrapper.vm.activeSection = 'data-processing';
    await flushPromises();

    expect(getDataProcessingMeta).toHaveBeenCalledTimes(1);
    expect(wrapper.vm.dpMeta.file_name).toBe('soglasie.pdf');
  });

  it('загрузка файла зовёт API и уведомляет об успехе', async () => {
    getDataProcessingMeta.mockResolvedValue(null);
    uploadDataProcessingDoc.mockResolvedValue(pdfMeta);
    const wrapper = await mountView();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    const file = new File(['%PDF'], 'soglasie.pdf', { type: 'application/pdf' });
    await wrapper.vm.onDpFileChange({ target: { files: [file], value: '' } });

    expect(uploadDataProcessingDoc).toHaveBeenCalledWith(file);
    expect(wrapper.vm.dpMeta.file_name).toBe('soglasie.pdf');
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'soglasie.pdf' }));
  });

  it('удаление спрашивает подтверждение и очищает документ', async () => {
    getDataProcessingMeta.mockResolvedValue(pdfMeta);
    deleteDataProcessingDoc.mockResolvedValue();
    const wrapper = await mountView();
    wrapper.vm.dpMeta = { ...pdfMeta };
    const confirmSpy = vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);

    await wrapper.vm.deleteDp();

    expect(confirmSpy).toHaveBeenCalled();
    expect(deleteDataProcessingDoc).toHaveBeenCalledTimes(1);
    expect(wrapper.vm.dpMeta).toBeNull();
  });

  it('отказ в подтверждении не удаляет документ', async () => {
    getDataProcessingMeta.mockResolvedValue(pdfMeta);
    const wrapper = await mountView();
    wrapper.vm.dpMeta = { ...pdfMeta };
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(false);

    await wrapper.vm.deleteDp();

    expect(deleteDataProcessingDoc).not.toHaveBeenCalled();
    expect(wrapper.vm.dpMeta).not.toBeNull();
  });
});
