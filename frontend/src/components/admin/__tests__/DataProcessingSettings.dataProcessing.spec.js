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
  getPDConsentCollection: vi.fn().mockResolvedValue({
    active: false, version: 1, total: 0, accepted: 0, pending: 0, pending_users: [], truncated: false,
  }),
}));

// Загрузка документа сразу переносит из него текст (#1567 S10) - извлечение мокаем,
// иначе спека тянула бы настоящий pdf.js с воркером. Сам перенос проверяет
// DataProcessingSettings.pdConsent.spec.js.
const extractDocumentHtml = vi.fn();
vi.mock('@/utils/documentTextExtract', () => ({
  extractDocumentHtml: (...a) => extractDocumentHtml(...a),
  UnsupportedDocumentError: class extends Error {},
}));

import DataProcessingSettings from '../DataProcessingSettings.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';

async function mountView() {
  getSettings.mockResolvedValue([]);
  const wrapper = shallowMount(DataProcessingSettings);
  await flushPromises();
  return wrapper;
}

const pdfMeta = { file_name: 'soglasie.pdf', mime_type: 'application/pdf', ext: '.pdf', uploaded_at: '2026-06-21T10:00:00Z' };

describe('Обработка данных - обработка данных', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    [getSettings, updateSetting, getDataProcessingMeta, uploadDataProcessingDoc,
      deleteDataProcessingDoc, downloadDataProcessingDoc, fetchDataProcessingBlob,
      getPDConsentSettings, extractDocumentHtml].forEach((m) => m.mockReset());
    getPDConsentSettings.mockResolvedValue({ text: '', version: 1, required: false });
    extractDocumentHtml.mockResolvedValue('<p>Текст из документа</p>');
  });

  // Раздел стал отдельной страницей: данные грузятся на открытии, а не по выбору
  // секции - ленивой загрузки внутри настроек больше нет.
  it('грузит документ при открытии страницы', async () => {
    getDataProcessingMeta.mockResolvedValue(pdfMeta);
    const wrapper = await mountView();

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
