import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

// Раздел сведений о субъекте (#2356). Тест держит две вещи, которые ловились только
// глазами на стенде и обе выглядели как «ничего не произошло»:
// собранные сведения появляются НИЖЕ списка, за краем экрана, и содержимое модалки
// обязано лежать в блоке со своими отступами - у base-modal__body их нет.

const findSubjectCandidates = vi.fn();
const fetchSubjectReport = vi.fn();
const fetchSubjectDisclosures = vi.fn();
vi.mock('@/api/pdSubject', () => ({
  findSubjectCandidates: (...a) => findSubjectCandidates(...a),
  fetchSubjectReport: (...a) => fetchSubjectReport(...a),
  fetchSubjectDisclosures: (...a) => fetchSubjectDisclosures(...a),
  exportSubjectReport: vi.fn(),
}));
const notify = vi.hoisted(() => vi.fn());
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify }) }));
vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ hasPermission: () => true }),
}));

import PdSubjectView from '../PdSubjectView.vue';
import PdSubjectExportModal from '@/components/admin/PdSubjectExportModal.vue';

const REPORT = {
  origin: 'по записи реестра 8',
  basis: 'Законный интерес оператора, п. 7 ч. 1 ст. 6 152-ФЗ',
  total: 3,
  sections: [
    { title: 'Сведения', headers: ['ФИО'], rows: [['Иванов Иван']] },
    // Раздел без строк приходит с rows: null - Go отдаёт пустой срез как null.
    { title: 'Проходы', headers: ['Дата'], rows: null },
  ],
};

function mountView() {
  return mount(PdSubjectView, {
    global: { stubs: { AdminPageShell: { template: '<div><slot /></div>' }, RefreshButton: true } },
  });
}

describe('PdSubjectView', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    findSubjectCandidates.mockResolvedValue([
      { source: 'реестр', id: 8, full_name: 'Иванов Иван', has_document: true },
    ]);
    fetchSubjectReport.mockResolvedValue(REPORT);
    fetchSubjectDisclosures.mockResolvedValue([]);
  });

  it('прокручивает к собранным сведениям: иначе они появляются за краем экрана', async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;

    const wrapper = mountView();
    await wrapper.find('[data-testid="pds-fio"]').setValue('Иванов Иван');
    await wrapper.find('form').trigger('submit');
    await flushPromises();

    await wrapper.find('[data-testid="pds-collect"]').trigger('click');
    await flushPromises();

    expect(scrollIntoView).toHaveBeenCalled();
  });

  it('раздел без строк не роняет экран', async () => {
    const wrapper = mountView();
    await wrapper.find('[data-testid="pds-fio"]').setValue('Иванов Иван');
    await wrapper.find('form').trigger('submit');
    await flushPromises();
    await wrapper.find('[data-testid="pds-collect"]').trigger('click');
    await flushPromises();

    // rows: null у «Проходов» - если бы шаблон читал rows.length напрямую, рендер
    // упал бы целиком и человека выбросило бы со страницы.
    expect(wrapper.text()).toContain('Проходы');
    expect(wrapper.text()).toContain('Записей нет');
  });
});

describe('PdSubjectExportModal', () => {
  it('содержимое лежит в блоке со своими отступами', () => {
    const wrapper = mount(PdSubjectExportModal, {
      props: { show: true },
      global: { stubs: { BaseModal: { template: '<div><slot /><slot name="actions" /></div>' } } },
    });

    // base-modal__body идёт без padding: отступы несёт содержимое. Без обёртки поля
    // упирались в края окна и начинались левее заголовка.
    const form = wrapper.find('.pdse__form');
    expect(form.exists()).toBe(true);
    expect(form.find('[data-testid="pdse-recipient"]').exists()).toBe(true);
    expect(form.find('[data-testid="pdse-request"]').exists()).toBe(true);
  });

  it('кнопка выгрузки гаснет без получателя и реквизитов запроса', async () => {
    const wrapper = mount(PdSubjectExportModal, {
      props: { show: true },
      global: { stubs: { BaseModal: { template: '<div><slot /><slot name="actions" /></div>' } } },
    });

    const submit = wrapper.find('[data-testid="pdse-submit"]');
    expect(submit.attributes('disabled')).toBeDefined();

    await wrapper.find('[data-testid="pdse-recipient"]').setValue('УМВД');
    await wrapper.find('[data-testid="pdse-request"]').setValue('исх. 1');
    expect(submit.attributes('disabled')).toBeUndefined();
  });
});
