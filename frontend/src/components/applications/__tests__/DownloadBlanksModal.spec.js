import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

import DownloadBlanksModal from '../DownloadBlanksModal.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { usePermissionsStore } from '@/stores/permissions';

// Выбор источника скачивания бланков (#1615, C6): «Сохранённый файл» отдаёт готовый
// файл файлового архива, «Сформировать заново» - прежняя генерация на лету. Разница
// между ними видна только по тому, куда уходит запрос, поэтому мокаются api-модули,
// а не сеть.

const { apiRequest, downloadBlank, downloadApplicationArchive, saveBlobAs, zipFile, zipGenerate } = vi.hoisted(() => ({
  apiRequest: vi.fn(),
  downloadBlank: vi.fn(),
  downloadApplicationArchive: vi.fn(),
  saveBlobAs: vi.fn(),
  zipFile: vi.fn(),
  zipGenerate: vi.fn(),
}));

vi.mock('@/api/client', () => ({ apiRequest }));
vi.mock('@/api/attachment-templates', () => ({ downloadBlank, downloadApplicationArchive, saveBlobAs }));
vi.mock('jszip', () => ({
  default: class JSZipStub {
    constructor() {
      this.file = zipFile;
      this.generateAsync = zipGenerate;
    }
  },
}));

const APP_ID = 7;

// По вложению на каждый статус реестра: у списка ровно один сохранённый файл, а
// остальные строки показывают, что архив ещё (или уже никогда) не готов.
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
  });
  // Модалка грузит вложения по открытию (watch show), а не на mount - открываем её
  // так же, как это делает деталь заявки.
  await wrapper.setProps({ show: true });
  await flushPromises();
  return wrapper;
}

const rowButtons = (wrapper) => wrapper.findAll('.dbm-item-download');
// Групп вкладок в модалке две (наполнение и источник), поэтому адресуемся по testid,
// а не по порядку компонентов: порядок в разметке уже менялся.
const sourceTabs = (wrapper) => wrapper.findComponent('[data-testid="blank-source-tabs"]');
const badges = (wrapper) => wrapper.findAll('.dbm-archive-badge');

function buttonByText(wrapper, text) {
  const btn = wrapper.findAll('button').find((b) => b.text() === text);
  expect(btn, `кнопка «${text}» не найдена`).toBeTruthy();
  return btn;
}

async function switchSource(wrapper, key) {
  await wrapper.find(`[data-testid="filter-tab-${key}"]`).trigger('click');
  await flushPromises();
}

// Выгрузка документов открыта парой прав: detail.documents (видеть их на экране) и
// detail.documents.export (вынести файлом). Тесты ниже задают их через сам стор, а не
// подменяют hasPermission - иначе конъюнкция в компоненте не проверялась бы вовсе.
function grantDocuments(keys = ['detail.documents', 'detail.documents.export']) {
  const perms = usePermissionsStore();
  perms.mode = 'normal';
  perms.effective = Object.fromEntries(keys.map((key) => [key, { value: 'allow', source: 'test' }]));
}

// Режим «С паспортными данными»: вкладки наполнения бланка видны только тем, у кого
// пара прав есть, поэтому переключение живёт отдельным хелпером.
async function switchDocuments(wrapper, key) {
  await wrapper.find(`[data-testid="filter-tab-${key}"]`).trigger('click');
  await flushPromises();
}

// Открытая модалка в режиме с документами - исходное состояние тестов про источник:
// «Сохранённый файл» существует только в нём.
async function mountWithDocuments(attachments = ATTACHMENTS) {
  grantDocuments();
  const wrapper = await mountModal(attachments);
  await switchDocuments(wrapper, 'with');
  return wrapper;
}

let wrapper;

describe('DownloadBlanksModal — источник скачивания (#1615 C6)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {});
    apiRequest.mockReset();
    downloadBlank.mockReset();
    downloadApplicationArchive.mockReset();
    saveBlobAs.mockReset();
    zipFile.mockReset();
    zipGenerate.mockReset();

    downloadBlank.mockResolvedValue({ blob: new Blob(['x']), filename: 'blank.xlsx' });
    downloadApplicationArchive.mockResolvedValue({ blob: new Blob(['zip']), filename: 'заявка.zip' });
    zipGenerate.mockResolvedValue(new Blob(['zip']));
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
  });

  it('источник по умолчанию - сохранённый файл, когда хоть один бланк уже в архиве', async () => {
    wrapper = await mountWithDocuments();

    expect(wrapper.vm.source).toBe('archive');
    expect(sourceTabs(wrapper).props('modelValue')).toBe('archive');
  });

  it('без единого сохранённого файла остаётся генерация заново', async () => {
    wrapper = await mountWithDocuments([
      { id: 1, attachment_name: 'Люди', has_template: true, archive_status: 'pending' },
      { id: 2, attachment_name: 'Сбой', has_template: true, archive_status: 'failed' },
      // Вложение без бланка выгружать нечем: его статус на выбор источника не влияет.
      { id: 3, attachment_name: 'Без бланка', has_template: false, archive_status: 'ok' },
    ]);

    expect(wrapper.vm.source).toBe('live');
    expect(sourceTabs(wrapper).props('modelValue')).toBe('live');
  });

  it('переключатель источника - общий FilterTabs, а не свои кнопки', async () => {
    wrapper = await mountWithDocuments();

    const tabs = sourceTabs(wrapper);
    expect(tabs.exists()).toBe(true);
    expect(tabs.props('tabs').map((t) => t.key)).toEqual(['archive', 'live']);

    await switchSource(wrapper, 'live');
    expect(wrapper.vm.source).toBe('live');
    expect(tabs.props('modelValue')).toBe('live');

    await switchSource(wrapper, 'archive');
    expect(wrapper.vm.source).toBe('archive');
    expect(tabs.props('modelValue')).toBe('archive');
  });

  it('бейдж состояния свой у каждого статуса, ожидание и нехватка места не сливаются', async () => {
    wrapper = await mountModal();

    const labels = badges(wrapper).map((b) => b.text());
    expect(labels).toEqual(['В архиве', 'В очереди', 'Нет места', 'Ошибка']);
    expect(labels[2]).not.toBe(labels[1]);
  });

  it('вложение без строки реестра бейджа не получает', async () => {
    wrapper = await mountModal([
      { id: 1, attachment_name: 'Люди', has_template: true, archive_status: 'ok' },
      { id: 2, attachment_name: 'Без архива', has_template: true },
      { id: 3, attachment_name: 'Пропущено', has_template: true, archive_status: 'skipped' },
    ]);

    expect(badges(wrapper)).toHaveLength(1);
    expect(badges(wrapper)[0].text()).toBe('В архиве');
  });

  it('на архивном источнике кнопка строки гаснет там, где сохранённого файла нет', async () => {
    wrapper = await mountWithDocuments();

    expect(wrapper.vm.unavailableInArchive).toEqual({ 1: false, 2: true, 3: true, 4: true });
    const disabled = rowButtons(wrapper).map((b) => b.attributes('disabled') !== undefined);
    expect(disabled).toEqual([false, true, true, true]);

    // Живая генерация умеет собрать бланк любому вложению - гасить нечего.
    await switchSource(wrapper, 'live');
    expect(rowButtons(wrapper).map((b) => b.attributes('disabled') !== undefined))
      .toEqual([false, false, false, false]);
  });

  it('скачивание строки уходит с выбранным источником', async () => {
    wrapper = await mountWithDocuments();

    await rowButtons(wrapper)[0].trigger('click');
    await flushPromises();
    expect(downloadBlank).toHaveBeenCalledWith(APP_ID, 1, { source: 'archive', withDocuments: true });

    await switchSource(wrapper, 'live');
    await rowButtons(wrapper)[1].trigger('click');
    await flushPromises();
    expect(downloadBlank).toHaveBeenLastCalledWith(APP_ID, 2, { source: 'live', withDocuments: true });
  });

  it('«Скачать все» на архивном источнике берёт готовый ZIP заявки с сервера', async () => {
    wrapper = await mountWithDocuments();

    await buttonByText(wrapper, 'Скачать все').trigger('click');
    await flushPromises();

    expect(downloadApplicationArchive).toHaveBeenCalledWith(APP_ID);
    expect(downloadBlank).not.toHaveBeenCalled();
    expect(zipGenerate).not.toHaveBeenCalled();
    expect(saveBlobAs).toHaveBeenCalledWith(expect.any(Blob), 'заявка.zip');
  });

  it('«Скачать все» на живом источнике собирает ZIP в браузере, как раньше', async () => {
    wrapper = await mountWithDocuments();
    await switchSource(wrapper, 'live');

    await buttonByText(wrapper, 'Скачать все').trigger('click');
    await flushPromises();

    expect(downloadApplicationArchive).not.toHaveBeenCalled();
    expect(downloadBlank).toHaveBeenCalledTimes(ATTACHMENTS.length);
    expect(downloadBlank).toHaveBeenCalledWith(APP_ID, 4, { source: 'live', withDocuments: true });
    expect(zipFile).toHaveBeenCalledTimes(ATTACHMENTS.length);
    expect(zipGenerate).toHaveBeenCalled();
    expect(saveBlobAs).toHaveBeenCalledWith(expect.any(Blob), expect.stringMatching(/^20260801-601.*\.zip$/));
  });
});

// Бланк уносит из системы паспорт, патент и иное разрешение участников. Выбор
// наполнения открыт паре прав (detail.documents и detail.documents.export), а
// сохранённый файл существует только в режиме с документами: копия на диске собрана
// с ними, и обезличить её при отдаче нечем.
describe('DownloadBlanksModal - гейт документов участников', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {});
    apiRequest.mockReset();
    downloadBlank.mockReset();
    downloadApplicationArchive.mockReset();
    saveBlobAs.mockReset();
    downloadBlank.mockResolvedValue({ blob: new Blob(['x']), filename: 'blank.xlsx' });
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = null;
  });

  it('без прав выбора нет, а вместо него строка о прочерках', async () => {
    wrapper = await mountModal();

    expect(wrapper.find('[data-testid="blank-documents-tabs"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="blank-documents-note"]').text())
      .toContain('заменены прочерком');
  });

  it('одного detail.documents мало: права работают только парой', async () => {
    grantDocuments(['detail.documents']);
    wrapper = await mountModal();

    expect(wrapper.vm.canExportDocuments).toBe(false);
    expect(wrapper.find('[data-testid="blank-documents-tabs"]').exists()).toBe(false);
  });

  it('без прав сохранённый файл недоступен, а скачивание идёт без документов', async () => {
    wrapper = await mountModal();

    // Вкладка источника «Сохранённый файл» скрыта: сервер на неё ответил бы 403.
    // Одинокая вкладка не выбор: группа источника в закрытом режиме не рисуется вовсе.
    expect(sourceTabs(wrapper).exists()).toBe(false);
    expect(wrapper.vm.source).toBe('live');

    await rowButtons(wrapper)[0].trigger('click');
    await flushPromises();
    expect(downloadBlank).toHaveBeenCalledWith(APP_ID, 1, { source: 'live', withDocuments: false });
  });

  it('с правами умолчание закрытое, документы включаются вторым щелчком', async () => {
    grantDocuments();
    wrapper = await mountModal();

    const documentsTabs = wrapper.findComponent('[data-testid="blank-documents-tabs"]');
    expect(documentsTabs.props('tabs').map((t) => t.key)).toEqual(['without', 'with']);
    expect(documentsTabs.props('modelValue')).toBe('without');

    await rowButtons(wrapper)[0].trigger('click');
    await flushPromises();
    expect(downloadBlank).toHaveBeenLastCalledWith(APP_ID, 1, { source: 'live', withDocuments: false });

    await switchDocuments(wrapper, 'with');
    await rowButtons(wrapper)[0].trigger('click');
    await flushPromises();
    expect(downloadBlank).toHaveBeenLastCalledWith(APP_ID, 1, { source: 'archive', withDocuments: true });
  });

  it('возврат в закрытый режим уводит источник с сохранённого файла', async () => {
    wrapper = await mountWithDocuments();
    expect(wrapper.vm.source).toBe('archive');

    await switchDocuments(wrapper, 'without');
    expect(wrapper.vm.source).toBe('live');
    expect(sourceTabs(wrapper).exists()).toBe(false);
  });

  it('повторное открытие модалки возвращает закрытый режим', async () => {
    wrapper = await mountWithDocuments();
    expect(wrapper.vm.documentsMode).toBe('with');

    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    await flushPromises();

    expect(wrapper.vm.documentsMode).toBe('without');
    expect(wrapper.vm.source).toBe('live');
  });
});
