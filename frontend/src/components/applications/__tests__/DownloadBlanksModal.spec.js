import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import DownloadBlanksModal from '../DownloadBlanksModal.vue';
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';
import { useAuthStore } from '@/stores/auth';

// Окно скачивания бланков. Выбор источника (сохранённый файл против генерации заново)
// убран как непонятный заявителю: бланк всегда собирается заново. Осталось одно
// решение - идут ли в файл сведения документов, и оно закрыто парой прав.

const { apiRequest, downloadBlank, saveBlobAs, zipFile, zipGenerate } = vi.hoisted(() => ({
  apiRequest: vi.fn(),
  downloadBlank: vi.fn(),
  saveBlobAs: vi.fn(),
  zipFile: vi.fn(),
  zipGenerate: vi.fn(),
}));

vi.mock('@/api/client', () => ({ apiRequest }));
vi.mock('@/api/attachment-templates', () => ({ downloadBlank, saveBlobAs }));
vi.mock('jszip', () => ({
  default: class JSZipStub {
    constructor() {
      this.file = zipFile;
      this.generateAsync = zipGenerate;
    }
  },
}));

const APP_ID = 7;

const ATTACHMENTS = [
  { id: 1, attachment_name: 'Люди', attachment_type: 'people', has_template: true, archive_status: 'ok' },
  { id: 2, attachment_name: 'Машины', attachment_type: 'cars', has_template: true, archive_status: 'pending' },
  { id: 3, attachment_name: 'ТМЦ', attachment_type: 'items', has_template: true, archive_status: 'blocked' },
  { id: 4, attachment_name: 'Сбой', attachment_type: 'people', has_template: true, archive_status: 'failed' },
];

async function mountModal(attachments = ATTACHMENTS) {
  apiRequest.mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue(attachments) });
  const wrapper = mount(DownloadBlanksModal, {
    props: {
      show: false,
      applicationId: APP_ID,
      applicationInfo: { application_number: '20260801-601', organization_name: 'Ромашка' },
    },
    global: { stubs: { teleport: true } },
    attachTo: document.body,
  });
  // Модалка грузит вложения по открытию (watch show), а не на mount - открываем её
  // так же, как это делает деталь заявки.
  await wrapper.setProps({ show: true });
  await flushPromises();
  return wrapper;
}

const rowButtons = (wrapper) => wrapper.findAll('.dbm-item-download');
const documentsToggle = (wrapper) => wrapper.findComponent('[data-testid="blank-documents-toggle"]');
const rowToggle = (wrapper, id) => wrapper.findComponent(`[data-testid="blank-select-${id}"]`);

function buttonByText(wrapper, text) {
  const btn = wrapper.findAll('button').find((b) => b.text() === text);
  expect(btn, `кнопка «${text}» не найдена`).toBeTruthy();
  return btn;
}

// Выгрузка сведений документов открыта парой прав: detail.documents (видеть их на
// экране) и detail.documents.export (вынести файлом). Задаём их через сам стор, а не
// подменяем hasPermission - иначе конъюнкция в компоненте не проверялась бы вовсе.
// Инициатор заявки открывает выгрузку без всяких прав: паспорта участников он сам
// набирал в форме подачи. Подменяем не геттер, а разбор маркера доступа - иначе
// проверка «совпал ли идентификатор» в компоненте не выполнялась бы.
function loginAsInitiator(userId = 42) {
  const auth = useAuthStore();
  vi.spyOn(auth, 'userPayload', 'get').mockReturnValue({ user_id: userId, username: 'initiator' });
  return userId;
}

function grantDocuments(keys = ['detail.documents', 'detail.documents.export']) {
  const perms = usePermissionsStore();
  perms.mode = 'normal';
  perms.effective = Object.fromEntries(keys.map((key) => [key, { value: 'allow', source: 'test' }]));
}

let wrapper;

describe('DownloadBlanksModal - выбор вложений и скачивание', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {});
    apiRequest.mockReset();
    downloadBlank.mockReset();
    saveBlobAs.mockReset();
    zipFile.mockReset();
    zipGenerate.mockReset();

    downloadBlank.mockResolvedValue({ blob: new Blob(['x']), filename: 'blank.xlsx' });
    zipGenerate.mockResolvedValue(new Blob(['zip']));
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
  });

  it('вложения выбираются тумблерами, а не флажками', async () => {
    wrapper = await mountModal();

    expect(wrapper.find('.dbm-checkbox').exists()).toBe(false);
    expect(wrapper.findAllComponents(ToggleSwitch).length).toBeGreaterThanOrEqual(ATTACHMENTS.length);

    await rowToggle(wrapper, 2).vm.$emit('update:modelValue', true);
    expect(wrapper.vm.selectedIds).toEqual([2]);
    expect(buttonByText(wrapper, 'Скачать (1)').exists()).toBe(true);

    await rowToggle(wrapper, 2).vm.$emit('update:modelValue', false);
    expect(wrapper.vm.selectedIds).toEqual([]);
  });

  it('выбора источника больше нет, бланк всегда собирается заново', async () => {
    wrapper = await mountModal();

    expect(wrapper.find('[data-testid="blank-source-tabs"]').exists()).toBe(false);
    expect(wrapper.text()).not.toContain('Сохранённый файл');
    expect(wrapper.text()).not.toContain('Сформировать заново');

    await rowButtons(wrapper)[0].trigger('click');
    await flushPromises();
    // Ни source, ни признака архива в запросе: у окна остался один источник.
    expect(downloadBlank).toHaveBeenCalledWith(APP_ID, 1, { withDocuments: false });
  });

  it('состояние выгрузки в архив в окне заявителя не показывается', async () => {
    wrapper = await mountModal();

    for (const label of ['В архиве', 'В очереди', 'Нет места', 'Ошибка']) {
      expect(wrapper.text()).not.toContain(label);
    }
  });

  it('«Скачать все» собирает ZIP в браузере', async () => {
    wrapper = await mountModal();

    await buttonByText(wrapper, 'Скачать все').trigger('click');
    await flushPromises();

    expect(downloadBlank).toHaveBeenCalledTimes(ATTACHMENTS.length);
    expect(zipFile).toHaveBeenCalledTimes(ATTACHMENTS.length);
    expect(saveBlobAs).toHaveBeenCalledWith(expect.any(Blob), expect.stringMatching(/^20260801-601.*\.zip$/));
  });
});

describe('DownloadBlanksModal - гейт сведений документов', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {});
    apiRequest.mockReset();
    downloadBlank.mockReset();
    saveBlobAs.mockReset();
    downloadBlank.mockResolvedValue({ blob: new Blob(['x']), filename: 'blank.xlsx' });
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
  });

  it('инициатор заявки видит переключатель без всяких прав', async () => {
    const userId = loginAsInitiator();
    apiRequest.mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue(ATTACHMENTS) });
    wrapper = mount(DownloadBlanksModal, {
      props: {
        show: false,
        applicationId: APP_ID,
        applicationInfo: { application_number: '20260801-601', sender_user_id: userId },
      },
      global: { stubs: { teleport: true } },
      attachTo: document.body,
    });
    await wrapper.setProps({ show: true });
    await flushPromises();

    expect(wrapper.vm.canExportDocuments).toBe(false);
    expect(wrapper.vm.isInitiator).toBe(true);
    expect(documentsToggle(wrapper).exists()).toBe(true);
    expect(wrapper.find('[data-testid="blank-documents-note"]').exists()).toBe(false);

    await documentsToggle(wrapper).vm.$emit('update:modelValue', true);
    await flushPromises();
    await rowButtons(wrapper)[0].trigger('click');
    await flushPromises();
    expect(downloadBlank).toHaveBeenLastCalledWith(APP_ID, 1, { withDocuments: true });
  });

  it('участнику чужой заявки переключателя нет', async () => {
    // Заявку подал кто-то другой: доступ к ней есть, а вводил документы не он.
    loginAsInitiator(7);
    apiRequest.mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue(ATTACHMENTS) });
    wrapper = mount(DownloadBlanksModal, {
      props: {
        show: false,
        applicationId: APP_ID,
        applicationInfo: { application_number: '20260801-601', sender_user_id: 99 },
      },
      global: { stubs: { teleport: true } },
      attachTo: document.body,
    });
    await wrapper.setProps({ show: true });
    await flushPromises();

    expect(wrapper.vm.isInitiator).toBe(false);
    expect(documentsToggle(wrapper).exists()).toBe(false);
  });

  it('без прав и не инициатору тумблера нет, а вместо него строка о прочерках', async () => {
    wrapper = await mountModal();

    expect(documentsToggle(wrapper).exists()).toBe(false);
    expect(wrapper.find('[data-testid="blank-documents-note"]').text()).toContain('прочерком');
  });

  it('одного detail.documents мало: права работают только парой', async () => {
    grantDocuments(['detail.documents']);
    wrapper = await mountModal();

    expect(wrapper.vm.canExportDocuments).toBe(false);
    expect(documentsToggle(wrapper).exists()).toBe(false);
  });

  it('с правами тумблер выключен по умолчанию и включает документы', async () => {
    grantDocuments();
    wrapper = await mountModal();

    const toggle = documentsToggle(wrapper);
    expect(toggle.props('modelValue')).toBe(false);

    await rowButtons(wrapper)[0].trigger('click');
    await flushPromises();
    expect(downloadBlank).toHaveBeenLastCalledWith(APP_ID, 1, { withDocuments: false });

    await toggle.vm.$emit('update:modelValue', true);
    await flushPromises();
    await rowButtons(wrapper)[0].trigger('click');
    await flushPromises();
    expect(downloadBlank).toHaveBeenLastCalledWith(APP_ID, 1, { withDocuments: true });
  });

  it('повторное открытие возвращает тумблер в выключенное положение', async () => {
    grantDocuments();
    wrapper = await mountModal();
    await documentsToggle(wrapper).vm.$emit('update:modelValue', true);
    expect(wrapper.vm.withDocuments).toBe(true);

    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    await flushPromises();

    expect(wrapper.vm.withDocuments).toBe(false);
  });
});

describe('DownloadBlanksModal - закрытие', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {});
    apiRequest.mockReset();
    downloadBlank.mockReset();
    downloadBlank.mockResolvedValue({ blob: new Blob(['x']), filename: 'blank.xlsx' });
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
  });

  it('Escape закрывает окно', async () => {
    wrapper = await mountModal();

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await flushPromises();
    expect(wrapper.emitted('close')).toBeTruthy();
  });

  it('после закрытия Escape больше не стреляет', async () => {
    // Обработчик висит на документе: не снятый при закрытии, он копился бы с каждым
    // открытием и закрывал бы соседние окна.
    wrapper = await mountModal();
    await wrapper.setProps({ show: false });

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await flushPromises();
    expect(wrapper.emitted('close')).toBeFalsy();
  });

  it('кнопка «Закрыть» и крестик просят закрыть окно', async () => {
    wrapper = await mountModal();

    await buttonByText(wrapper, 'Закрыть').trigger('click');
    expect(wrapper.emitted('close')).toHaveLength(1);

    await wrapper.find('.dbm-close').trigger('click');
    expect(wrapper.emitted('close')).toHaveLength(2);
  });

  it('клик по подложке закрывает, протяжка изнутри - нет', async () => {
    wrapper = await mountModal();
    const overlay = wrapper.find('.dbm-overlay');

    await overlay.trigger('mousedown');
    await overlay.trigger('mouseup');
    expect(wrapper.emitted('close')).toHaveLength(1);

    // Выделение текста мышью, начатое внутри окна и отпущенное на фоне.
    await wrapper.find('.dbm-modal').trigger('mousedown');
    await overlay.trigger('mouseup');
    expect(wrapper.emitted('close')).toHaveLength(1);
  });
});
