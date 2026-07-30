import { describe, it, expect, vi, beforeEach } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';

// Текст согласия при первом входе (#1567): администраторская часть в настройках.

const getSettings = vi.fn();
vi.mock('@/api/settings', () => ({
  getSettings: (...a) => getSettings(...a),
  updateSetting: vi.fn(),
}));

const getDataProcessingMeta = vi.fn();
const fetchDataProcessingBlob = vi.fn();
vi.mock('@/api/dataProcessing', () => ({
  getDataProcessingMeta: (...a) => getDataProcessingMeta(...a),
  fetchDataProcessingBlob: (...a) => fetchDataProcessingBlob(...a),
  uploadDataProcessingDoc: vi.fn(),
  deleteDataProcessingDoc: vi.fn(),
  downloadDataProcessingDoc: vi.fn(),
}));

const getPDConsentSettings = vi.fn();
const savePDConsentText = vi.fn();
const setPDConsentRequired = vi.fn();
const requirePDConsentAgain = vi.fn();
vi.mock('@/api/pdConsent', () => ({
  getPDConsentSettings: (...a) => getPDConsentSettings(...a),
  savePDConsentText: (...a) => savePDConsentText(...a),
  setPDConsentRequired: (...a) => setPDConsentRequired(...a),
  requirePDConsentAgain: (...a) => requirePDConsentAgain(...a),
}));

const extractDocumentHtml = vi.fn();
vi.mock('@/utils/documentTextExtract', () => ({
  extractDocumentHtml: (...a) => extractDocumentHtml(...a),
  UnsupportedDocumentError: class extends Error {},
}));

import AdminSettings from '../AdminSettings.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';

const pdfMeta = { file_name: 'soglasie.pdf', ext: '.pdf', uploaded_at: '2026-07-01T10:00:00Z' };

const state = (over = {}) => ({ text: '', version: 1, required: false, ...over });

// Разметка секции лежит в слоте SkeletonTransition, а shallowMount слоты
// застабленных компонентов не рисует - подменяем его на прозрачную обёртку,
// чтобы проверять реальный DOM секции. Остальные дети остаются заглушками.
const renderSlot = { template: '<div><slot /></div>' };

async function openSection() {
  getSettings.mockResolvedValue([]);
  const wrapper = shallowMount(AdminSettings, {
    global: { stubs: { SkeletonTransition: renderSlot } },
  });
  await flushPromises();
  wrapper.vm.activeSection = 'data-processing';
  await flushPromises();
  return wrapper;
}

describe('AdminSettings - текст согласия при первом входе', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    [getSettings, getDataProcessingMeta, fetchDataProcessingBlob, getPDConsentSettings,
      savePDConsentText, setPDConsentRequired, requirePDConsentAgain,
      extractDocumentHtml].forEach((m) => m.mockReset());
    getDataProcessingMeta.mockResolvedValue(pdfMeta);
    getPDConsentSettings.mockResolvedValue(state());
  });

  it('грузит настройки согласия при первом открытии секции', async () => {
    getPDConsentSettings.mockResolvedValue(state({ text: '<p>Текст</p>', version: 3, required: true }));

    const wrapper = shallowMount(AdminSettings);
    await flushPromises();
    expect(getPDConsentSettings).not.toHaveBeenCalled();

    wrapper.vm.activeSection = 'data-processing';
    await flushPromises();

    expect(getPDConsentSettings).toHaveBeenCalledTimes(1);
    expect(wrapper.vm.pdcText).toBe('<p>Текст</p>');
    expect(wrapper.vm.pdcVersion).toBe(3);
    expect(wrapper.vm.pdcRequired).toBe(true);
  });

  it('повторное открытие секции не перезапрашивает настройки', async () => {
    const wrapper = await openSection();
    wrapper.vm.activeSection = 'upload';
    await flushPromises();
    wrapper.vm.activeSection = 'data-processing';
    await flushPromises();

    expect(getPDConsentSettings).toHaveBeenCalledTimes(1);
  });

  it('извлечение переносит текст документа в редактор', async () => {
    fetchDataProcessingBlob.mockResolvedValue(new Blob(['%PDF']));
    extractDocumentHtml.mockResolvedValue('<p>Извлечённый текст</p>');
    const wrapper = await openSection();

    await wrapper.vm.extractPdcText();

    expect(extractDocumentHtml).toHaveBeenCalledWith(expect.any(Blob), '.pdf');
    expect(wrapper.vm.pdcText).toBe('<p>Извлечённый текст</p>');
    // Извлечение само не сохраняет: администратор сперва вычитывает результат.
    expect(savePDConsentText).not.toHaveBeenCalled();
  });

  it('ошибку извлечения показывает пользователю и текст не трогает', async () => {
    fetchDataProcessingBlob.mockResolvedValue(new Blob(['doc']));
    extractDocumentHtml.mockRejectedValue(new Error('Из старого формата .doc текст не извлекается'));
    const wrapper = await openSection();
    wrapper.vm.pdcText = '<p>Было</p>';
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await wrapper.vm.extractPdcText();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      prefix: 'Из старого формата .doc текст не извлекается',
      type: 'error',
    }));
    expect(wrapper.vm.pdcText).toBe('<p>Было</p>');
  });

  it('документ без текстового слоя предупреждает, а не молчит', async () => {
    fetchDataProcessingBlob.mockResolvedValue(new Blob(['%PDF']));
    extractDocumentHtml.mockResolvedValue('');
    const wrapper = await openSection();
    wrapper.vm.pdcText = '<p>Было</p>';
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await wrapper.vm.extractPdcText();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'warning' }));
    expect(wrapper.vm.pdcText).toBe('<p>Было</p>');
  });

  it('сохранение текста применяет состояние с сервера', async () => {
    savePDConsentText.mockResolvedValue(state({ text: '<p>Сохранено</p>', version: 2, required: true }));
    const wrapper = await openSection();
    wrapper.vm.pdcText = '<p>Сохранено</p>';

    await wrapper.vm.savePdcText();

    expect(savePDConsentText).toHaveBeenCalledWith('<p>Сохранено</p>');
    expect(wrapper.vm.pdcVersion).toBe(2);
    expect(wrapper.vm.pdcRequired).toBe(true);
  });

  it('ошибку сохранения показывает сообщением сервера', async () => {
    savePDConsentText.mockRejectedValue(new Error('Текст согласия больше 512 КБ'));
    const wrapper = await openSection();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await wrapper.vm.savePdcText();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      prefix: 'Текст согласия больше 512 КБ',
      type: 'error',
    }));
  });

  it('включение запроса согласия спрашивает подтверждение', async () => {
    setPDConsentRequired.mockResolvedValue(state({ text: '<p>Текст</p>', required: true }));
    const wrapper = await openSection();
    const confirmSpy = vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);

    await wrapper.vm.togglePdcRequired();

    expect(confirmSpy).toHaveBeenCalled();
    expect(setPDConsentRequired).toHaveBeenCalledWith(true);
    expect(wrapper.vm.pdcRequired).toBe(true);
  });

  it('отказ в подтверждении не включает запрос согласия', async () => {
    const wrapper = await openSection();
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(false);

    await wrapper.vm.togglePdcRequired();

    expect(setPDConsentRequired).not.toHaveBeenCalled();
    expect(wrapper.vm.pdcRequired).toBe(false);
  });

  it('выключение запроса согласия подтверждения не требует', async () => {
    getPDConsentSettings.mockResolvedValue(state({ text: '<p>Текст</p>', required: true }));
    setPDConsentRequired.mockResolvedValue(state({ text: '<p>Текст</p>', required: false }));
    const wrapper = await openSection();
    const confirmSpy = vi.spyOn(useUiStore(), 'confirm');

    await wrapper.vm.togglePdcRequired();

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(setPDConsentRequired).toHaveBeenCalledWith(false);
    expect(wrapper.vm.pdcRequired).toBe(false);
  });

  it('отказ сервера включить с пустым текстом оставляет тумблер выключенным', async () => {
    setPDConsentRequired.mockRejectedValue(new Error('Сначала задайте текст согласия'));
    const wrapper = await openSection();
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await wrapper.vm.togglePdcRequired();

    expect(wrapper.vm.pdcRequired).toBe(false);
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      prefix: 'Сначала задайте текст согласия',
      type: 'error',
    }));
  });

  it('требовать заново поднимает редакцию после подтверждения', async () => {
    requirePDConsentAgain.mockResolvedValue(state({ text: '<p>Текст</p>', version: 5 }));
    const wrapper = await openSection();
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);

    await wrapper.vm.requirePdcAgain();

    expect(requirePDConsentAgain).toHaveBeenCalledTimes(1);
    expect(wrapper.vm.pdcVersion).toBe(5);
  });

  it('отказ в подтверждении не поднимает редакцию', async () => {
    const wrapper = await openSection();
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(false);

    await wrapper.vm.requirePdcAgain();

    expect(requirePDConsentAgain).not.toHaveBeenCalled();
    expect(wrapper.vm.pdcVersion).toBe(1);
  });

  describe('признак пустого текста', () => {
    it('пустой абзац редактора считается пустым текстом', async () => {
      const wrapper = await openSection();

      for (const empty of ['', '<p></p>', '<p>   </p>', '<p>&nbsp;</p>']) {
        wrapper.vm.pdcText = empty;
        await flushPromises();
        expect(wrapper.vm.pdcHasText, `«${empty}» должно считаться пустым`).toBe(false);
      }
    });

    it('текст и картинка считаются содержимым', async () => {
      const wrapper = await openSection();

      wrapper.vm.pdcText = '<p>Согласие</p>';
      expect(wrapper.vm.pdcHasText).toBe(true);

      wrapper.vm.pdcText = '<p><img src="data:image/png;base64,iVBORw0="></p>';
      expect(wrapper.vm.pdcHasText).toBe(true);
    });
  });

  it('включённый запрос с пустым текстом показывает предупреждение', async () => {
    getPDConsentSettings.mockResolvedValue(state({ text: '', required: true }));
    const wrapper = await openSection();

    expect(wrapper.find('[data-testid="pdc-empty-warning"]').exists()).toBe(true);
  });

  // Подпись под тумблером ловится только рендером: статичное «пока выключено»
  // при включённом запросе противоречило состоянию.
  it('подпись под тумблером следует его состоянию', async () => {
    getPDConsentSettings.mockResolvedValue(state({ text: '<p>Согласие</p>', required: true }));
    const wrapper = await openSection();

    expect(wrapper.find('[data-testid="pdc-required-hint"]').text()).toContain('Включено');

    wrapper.vm.pdcRequired = false;
    await flushPromises();
    expect(wrapper.find('[data-testid="pdc-required-hint"]').text()).toContain('Выключено');
  });

  it('при заданном тексте предупреждения нет', async () => {
    getPDConsentSettings.mockResolvedValue(state({ text: '<p>Согласие</p>', required: true }));
    const wrapper = await openSection();

    expect(wrapper.find('[data-testid="pdc-empty-warning"]').exists()).toBe(false);
  });

  it('без загруженного документа кнопка извлечения недоступна', async () => {
    getDataProcessingMeta.mockResolvedValue(null);
    const wrapper = await openSection();

    expect(wrapper.find('[data-testid="pdc-extract"]').attributes('disabled')).toBeDefined();
  });
});
