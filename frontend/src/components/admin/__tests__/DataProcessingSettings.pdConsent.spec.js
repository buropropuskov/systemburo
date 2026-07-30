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
const uploadDataProcessingDoc = vi.fn();
vi.mock('@/api/dataProcessing', () => ({
  getDataProcessingMeta: (...a) => getDataProcessingMeta(...a),
  uploadDataProcessingDoc: (...a) => uploadDataProcessingDoc(...a),
  fetchDataProcessingBlob: vi.fn(),
  deleteDataProcessingDoc: vi.fn(),
  downloadDataProcessingDoc: vi.fn(),
}));

const getPDConsentSettings = vi.fn();
const savePDConsentText = vi.fn();
const setPDConsentRequired = vi.fn();
const requirePDConsentAgain = vi.fn();
const getPDConsentCollection = vi.fn();
vi.mock('@/api/pdConsent', () => ({
  getPDConsentSettings: (...a) => getPDConsentSettings(...a),
  savePDConsentText: (...a) => savePDConsentText(...a),
  setPDConsentRequired: (...a) => setPDConsentRequired(...a),
  requirePDConsentAgain: (...a) => requirePDConsentAgain(...a),
  getPDConsentCollection: (...a) => getPDConsentCollection(...a),
}));

const extractDocumentHtml = vi.fn();
vi.mock('@/utils/documentTextExtract', () => ({
  extractDocumentHtml: (...a) => extractDocumentHtml(...a),
  UnsupportedDocumentError: class extends Error {},
}));

import DataProcessingSettings from '../DataProcessingSettings.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';

const pdfMeta = { file_name: 'soglasie.pdf', ext: '.pdf', uploaded_at: '2026-07-01T10:00:00Z' };

const state = (over = {}) => ({ text: '', version: 1, required: false, ...over });

/** Событие выбора файла, как его отдаёт <input type="file">. */
function fileEvent(name = 'soglasie.pdf', type = 'application/pdf') {
  const file = new File(['%PDF'], name, { type });
  return { file, event: { target: { files: [file], value: name } } };
}

// Разметка секции лежит в слоте SkeletonTransition, а shallowMount слоты
// застабленных компонентов не рисует - подменяем его на прозрачную обёртку,
// чтобы проверять реальный DOM секции. Остальные дети остаются заглушками.
async function openSection() {
  // Раздел стал отдельной страницей (#1567): компонент грузит данные сам на
  // монтировании, выбирать секцию больше не надо.
  const wrapper = shallowMount(DataProcessingSettings, {
    global: { stubs: { TextConstructor: true, RefreshButton: true } },
  });
  await flushPromises();
  return wrapper;
}

describe('Обработка данных - текст согласия при первом входе', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    [getSettings, getDataProcessingMeta, uploadDataProcessingDoc, getPDConsentSettings,
      savePDConsentText, setPDConsentRequired, requirePDConsentAgain,
      extractDocumentHtml, getPDConsentCollection].forEach((m) => m.mockReset());
    uploadDataProcessingDoc.mockResolvedValue(pdfMeta);
    getPDConsentCollection.mockResolvedValue({
      version: 1, total: 0, accepted: 0, pending: 0, pending_users: [],
    });
    getDataProcessingMeta.mockResolvedValue(pdfMeta);
    getPDConsentSettings.mockResolvedValue(state());
  });

  // Раздел стал отдельной страницей: настройки грузятся на открытии.
  it('грузит настройки согласия при открытии страницы', async () => {
    getPDConsentSettings.mockResolvedValue(state({ text: '<p>Текст</p>', version: 3, required: true }));

    const wrapper = await openSection();

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

  it('загрузка документа сразу переносит текст в редактор, без отдельной команды', async () => {
    extractDocumentHtml.mockResolvedValue('<p>Извлечённый текст</p>');
    const wrapper = await openSection();
    const { file, event } = fileEvent();

    await wrapper.vm.onDpFileChange(event);
    await flushPromises();

    // Читаем сам выбранный файл, а не скачиваем документ обратно с сервера.
    expect(extractDocumentHtml).toHaveBeenCalledWith(file, '.pdf');
    expect(wrapper.vm.pdcText).toBe('<p>Извлечённый текст</p>');
    // Перенос сам не сохраняет: администратор сперва вычитывает результат.
    expect(savePDConsentText).not.toHaveBeenCalled();
  });

  it('кнопки извлечения в разделе больше нет', async () => {
    const wrapper = await openSection();

    expect(wrapper.find('[data-testid="pdc-extract"]').exists()).toBe(false);
  });

  it('инпут файла принимает и xlsx', async () => {
    const wrapper = await openSection();

    const accept = wrapper.find('input[type="file"]').attributes('accept');
    expect(accept).toContain('.xlsx');
    expect(accept).toContain('.pdf');
  });

  it('замену непустого текста спрашивает, отказ оставляет прежний', async () => {
    getPDConsentSettings.mockResolvedValue(state({ text: '<p>Было</p>' }));
    extractDocumentHtml.mockResolvedValue('<p>Новое</p>');
    const wrapper = await openSection();
    const confirmSpy = vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(false);

    await wrapper.vm.onDpFileChange(fileEvent().event);
    await flushPromises();

    expect(confirmSpy).toHaveBeenCalled();
    expect(extractDocumentHtml).not.toHaveBeenCalled();
    expect(wrapper.vm.pdcText).toBe('<p>Было</p>');
  });

  it('пустой текст заменяет без вопросов', async () => {
    extractDocumentHtml.mockResolvedValue('<p>Новое</p>');
    const wrapper = await openSection();
    const confirmSpy = vi.spyOn(useUiStore(), 'confirm');

    await wrapper.vm.onDpFileChange(fileEvent().event);
    await flushPromises();

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(wrapper.vm.pdcText).toBe('<p>Новое</p>');
  });

  it('перенос не обгоняет загрузку настроек и не затирается их ответом', async () => {
    // Настройки и загрузка документа летят параллельно: ответ настроек с пустым
    // текстом не должен стереть только что перенесённый.
    let releaseSettings;
    getPDConsentSettings.mockReturnValue(new Promise((resolve) => { releaseSettings = resolve; }));
    extractDocumentHtml.mockResolvedValue('<p>Перенесённый</p>');
    const wrapper = shallowMount(DataProcessingSettings, {
      global: { stubs: { TextConstructor: true, RefreshButton: true } },
    });

    const upload = wrapper.vm.onDpFileChange(fileEvent().event);
    releaseSettings(state({ text: '' }));
    await upload;
    await flushPromises();

    expect(wrapper.vm.pdcText).toBe('<p>Перенесённый</p>');
  });

  it('неизвлекаемый документ предупреждает, но загрузку не отменяет', async () => {
    extractDocumentHtml.mockRejectedValue(new Error('Из старого формата .doc текст не извлекается'));
    getPDConsentSettings.mockResolvedValue(state({ text: '<p>Было</p>' }));
    const wrapper = await openSection();
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await wrapper.vm.onDpFileChange(fileEvent('soglasie.doc').event);
    await flushPromises();

    expect(uploadDataProcessingDoc).toHaveBeenCalled();
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      prefix: 'Из старого формата .doc текст не извлекается',
      type: 'warning',
    }));
    expect(wrapper.vm.pdcText).toBe('<p>Было</p>');
  });

  it('документ без текстового слоя предупреждает, а не молчит', async () => {
    extractDocumentHtml.mockResolvedValue('');
    getPDConsentSettings.mockResolvedValue(state({ text: '<p>Было</p>' }));
    const wrapper = await openSection();
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await wrapper.vm.onDpFileChange(fileEvent().event);
    await flushPromises();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'warning' }));
    expect(wrapper.vm.pdcText).toBe('<p>Было</p>');
  });

  it('пока идёт перенос, повторная загрузка и правка текста закрыты', async () => {
    // Диалог подтверждения в приложении один: второй вызов оставил бы первый
    // висеть без ответа, а правка в редакторе была бы затёрта переносом.
    getPDConsentSettings.mockResolvedValue(state({ text: '<p>Было</p>' }));
    let releaseConfirm;
    vi.spyOn(useUiStore(), 'confirm')
      .mockReturnValue(new Promise((resolve) => { releaseConfirm = resolve; }));
    const wrapper = await openSection();

    const running = wrapper.vm.onDpFileChange(fileEvent().event);
    await flushPromises();

    expect(wrapper.vm.dpBusy).toBe(true);
    expect(wrapper.findComponent({ name: 'TextConstructor' }).props('disabled')).toBe(true);

    releaseConfirm(false);
    await running;
    expect(wrapper.vm.dpBusy).toBe(false);
  });

  it('провал загрузки документа текст не трогает', async () => {
    uploadDataProcessingDoc.mockRejectedValue(new Error('Файл слишком большой'));
    const wrapper = await openSection();

    await wrapper.vm.onDpFileChange(fileEvent().event);
    await flushPromises();

    expect(extractDocumentHtml).not.toHaveBeenCalled();
    expect(wrapper.vm.pdcText).toBe('');
  });

  it('сохранение текста применяет состояние с сервера', async () => {
    getPDConsentSettings.mockResolvedValue(state({ text: '<p>Сохранено</p>' }));
    savePDConsentText.mockResolvedValue(state({ text: '<p>Сохранено</p>', version: 2, required: true }));
    const wrapper = await openSection();

    await wrapper.vm.savePdcText();

    // Текст не менялся - вопроса про новую редакцию нет и редакцию не двигаем.
    expect(savePDConsentText).toHaveBeenCalledWith('<p>Сохранено</p>', false);
    expect(wrapper.vm.pdcVersion).toBe(2);
    expect(wrapper.vm.pdcRequired).toBe(true);
  });

  describe('смена текста и повторное согласие', () => {
    it('изменённый текст спрашивает про повторное согласие и поднимает редакцию', async () => {
      getPDConsentSettings.mockResolvedValue(state({ text: '<p>Было</p>', version: 4, required: true }));
      savePDConsentText.mockResolvedValue(state({ text: '<p>Стало</p>', version: 5, required: true }));
      const wrapper = await openSection();
      const confirmSpy = vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);
      wrapper.vm.pdcText = '<p>Стало</p>';

      await wrapper.vm.savePdcText();

      expect(confirmSpy).toHaveBeenCalledWith(expect.objectContaining({
        title: 'Текст изменён',
        confirmText: 'Запросить заново',
      }));
      // Редакцию поднимает тот же запрос: отдельный вызов мог бы не дойти и
      // оставить новый текст со старой редакцией.
      expect(savePDConsentText).toHaveBeenCalledWith('<p>Стало</p>', true);
      expect(requirePDConsentAgain).not.toHaveBeenCalled();
      expect(wrapper.vm.pdcVersion).toBe(5);
    });

    it('отказ от повторного согласия текст всё равно сохраняет', async () => {
      getPDConsentSettings.mockResolvedValue(state({ text: '<p>Было</p>', version: 4 }));
      savePDConsentText.mockResolvedValue(state({ text: '<p>Стало</p>', version: 4 }));
      const wrapper = await openSection();
      vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(false);
      wrapper.vm.pdcText = '<p>Стало</p>';

      await wrapper.vm.savePdcText();

      expect(savePDConsentText).toHaveBeenCalledWith('<p>Стало</p>', false);
      expect(wrapper.vm.pdcVersion).toBe(4);
    });

    it('без правки текста вопроса нет', async () => {
      getPDConsentSettings.mockResolvedValue(state({ text: '<p>Было</p>' }));
      savePDConsentText.mockResolvedValue(state({ text: '<p>Было</p>' }));
      const wrapper = await openSection();
      const confirmSpy = vi.spyOn(useUiStore(), 'confirm');

      await wrapper.vm.savePdcText();

      expect(confirmSpy).not.toHaveBeenCalled();
      expect(savePDConsentText).toHaveBeenCalledWith('<p>Было</p>', false);
    });

    it('правка только разметки тоже считается сменой редакции', async () => {
      getPDConsentSettings.mockResolvedValue(state({ text: '<p>Согласие</p>' }));
      savePDConsentText.mockResolvedValue(state({ text: '<p><strong>Согласие</strong></p>' }));
      const wrapper = await openSection();
      vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);
      wrapper.vm.pdcText = '<p><strong>Согласие</strong></p>';

      await wrapper.vm.savePdcText();

      expect(savePDConsentText).toHaveBeenCalledWith('<p><strong>Согласие</strong></p>', true);
    });

    it('при выключенном запросе вопрос честно говорит, что окно поднимется позже', async () => {
      getPDConsentSettings.mockResolvedValue(state({ text: '<p>Было</p>', required: false }));
      savePDConsentText.mockResolvedValue(state({ text: '<p>Стало</p>' }));
      const wrapper = await openSection();
      const confirmSpy = vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);
      wrapper.vm.pdcText = '<p>Стало</p>';

      await wrapper.vm.savePdcText();

      expect(confirmSpy).toHaveBeenCalledWith(expect.objectContaining({
        message: expect.stringContaining('запрос согласия выключен'),
      }));
    });

    it('после сохранения повторная правка снова спрашивает', async () => {
      getPDConsentSettings.mockResolvedValue(state({ text: '<p>Было</p>' }));
      savePDConsentText.mockResolvedValue(state({ text: '<p>Стало</p>', version: 2 }));
      const wrapper = await openSection();
      const confirmSpy = vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);
      wrapper.vm.pdcText = '<p>Стало</p>';
      await wrapper.vm.savePdcText();

      // Сохранённый текст обновился - повторное сохранение без правки молчит.
      expect(wrapper.vm.pdcTextChanged).toBe(false);
      await wrapper.vm.savePdcText();
      expect(confirmSpy).toHaveBeenCalledTimes(1);

      wrapper.vm.pdcText = '<p>Снова другое</p>';
      expect(wrapper.vm.pdcTextChanged).toBe(true);
    });

    it('перенос текста из документа делает его изменённым', async () => {
      getPDConsentSettings.mockResolvedValue(state({ text: '<p>Было</p>' }));
      extractDocumentHtml.mockResolvedValue('<p>Из документа</p>');
      const wrapper = await openSection();
      vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true);

      await wrapper.vm.onDpFileChange(fileEvent().event);
      await flushPromises();

      expect(wrapper.vm.pdcTextChanged).toBe(true);
    });
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

  it('без загруженного документа раздел предлагает его загрузить', async () => {
    getDataProcessingMeta.mockResolvedValue(null);
    const wrapper = await openSection();

    expect(wrapper.find('.dp-upload input[type="file"]').exists()).toBe(true);
  });
});
