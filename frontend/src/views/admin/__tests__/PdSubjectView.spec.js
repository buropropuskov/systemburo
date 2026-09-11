import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount, flushPromises } from '@vue/test-utils';

// Раздел сведений о субъекте (#2356). Тест держит то, что ловилось только глазами на
// стенде: собранные сведения появляются ниже списка и за краем экрана; раздел без
// строк приходит с rows: null; однофамильцы без пометок неразличимы; поиск по
// документу точнее поиска по имени.

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
  basis: 'Обеспечение пропускного режима на объекте',
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

async function searchByName(wrapper, fio = 'Иванов Иван') {
  await wrapper.find('[data-testid="pds-fio"]').setValue(fio);
  await wrapper.find('form').trigger('submit');
  await flushPromises();
}

describe('PdSubjectView', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    findSubjectCandidates.mockResolvedValue([
      {
        full_name: 'Иванов Иван',
        registry_id: 8,
        employee_id: 0,
        registry_rows: 3,
        application_rows: 7,
        has_document: true,
      },
    ]);
    fetchSubjectReport.mockResolvedValue(REPORT);
    fetchSubjectDisclosures.mockResolvedValue([]);
  });

  it('прокручивает к собранным сведениям: иначе они появляются за краем экрана', async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;

    const wrapper = mountView();
    await searchByName(wrapper);
    await wrapper.find('[data-testid="pds-collect"]').trigger('click');
    await flushPromises();

    expect(scrollIntoView).toHaveBeenCalled();
  });

  it('раздел без строк не роняет экран', async () => {
    const wrapper = mountView();
    await searchByName(wrapper);
    await wrapper.find('[data-testid="pds-collect"]').trigger('click');
    await flushPromises();

    // rows: null у «Проходов» - если бы шаблон читал rows.length напрямую, рендер
    // упал бы целиком и человека выбросило бы со страницы.
    expect(wrapper.text()).toContain('Проходы');
    expect(wrapper.text()).toContain('Записей нет');
  });

  it('строки одного человека склеены: один пункт вместо десяти, счётчики рядом', async () => {
    const wrapper = mountView();
    await searchByName(wrapper);

    expect(wrapper.findAll('.pds__item')).toHaveLength(1);
    expect(wrapper.text()).toContain('в реестре: 3');
    expect(wrapper.text()).toContain('в заявках: 7');
  });

  it('человек без записи реестра собирается от строки заявки', async () => {
    findSubjectCandidates.mockResolvedValue([
      {
        full_name: 'Заявкин Пётр',
        registry_id: 0,
        employee_id: 42,
        registry_rows: 0,
        application_rows: 2,
        has_document: true,
      },
    ]);

    const wrapper = mountView();
    await searchByName(wrapper, 'Заявкин Пётр');
    await wrapper.find('[data-testid="pds-collect"]').trigger('click');
    await flushPromises();

    // Записи реестра у него нет вовсе - таких людей на стенде 22, и запрос
    // государственного органа может прийти именно о них.
    expect(fetchSubjectReport).toHaveBeenCalledWith({ registryId: 0, employeeId: 42 });
  });

  it('однофамильцы различаются по документу и организации', async () => {
    findSubjectCandidates.mockResolvedValue([
      {
        full_name: 'Мякотных Сергей',
        registry_id: 4,
        registry_rows: 1,
        application_rows: 5,
        has_document: true,
        organization: 'Отдел контроля доступа',
        document_tail: '2135',
      },
      {
        full_name: 'Мякотных Сергей',
        registry_id: 6,
        registry_rows: 1,
        application_rows: 1,
        has_document: true,
        organization: 'Бюро пропусков',
        document_tail: '7788',
      },
    ]);

    const wrapper = mountView();
    await searchByName(wrapper, 'Мякотных Сергей');

    // Три одинаковые строки «Мякотных С.» - это то, на чём споткнулась ручная
    // проверка: выбрать было не из чего.
    const text = wrapper.text();
    expect(text).toContain('документ …2135');
    expect(text).toContain('документ …7788');
    expect(text).toContain('Отдел контроля доступа');
  });

  it('поиск по документу уходит номером, а не именем', async () => {
    findSubjectCandidates.mockResolvedValue([]);

    const wrapper = mountView();
    await wrapper.find('[data-testid="pds-mode-document"]').trigger('click');
    await wrapper.find('[data-testid="pds-document"]').setValue('4510 123456');
    await wrapper.find('form').trigger('submit');
    await flushPromises();

    // В запросе государственного органа номер документа есть чаще, чем верное
    // написание фамилии, и он однозначен - находит ровно того, о ком спрашивают.
    expect(findSubjectCandidates).toHaveBeenCalledWith({ fio: '', document: '4510 123456' });
  });

  it('памятка видна на пустом экране и уходит после поиска', async () => {
    const wrapper = mountView();
    // Раздел открывают несколько раз в год: порядок с реквизитами запроса
    // к следующему разу забывается.
    expect(wrapper.find('.pds__steps').exists()).toBe(true);

    await searchByName(wrapper);
    expect(wrapper.find('.pds__steps').exists()).toBe(false);
  });
});

describe('PdSubjectExportModal', () => {
  const stubs = { BaseModal: { template: '<div><slot /><slot name="actions" /></div>' } };

  it('содержимое лежит в блоке со своими отступами', () => {
    const wrapper = mount(PdSubjectExportModal, { props: { show: true }, global: { stubs } });

    // base-modal__body идёт без padding: отступы несёт содержимое. Без обёртки поля
    // упирались в края окна и начинались левее заголовка.
    const form = wrapper.find('.pdse__form');
    expect(form.exists()).toBe(true);
    expect(form.find('[data-testid="pdse-recipient"]').exists()).toBe(true);
  });

  it('кнопка выгрузки гаснет без получателя и реквизитов запроса', async () => {
    const wrapper = mount(PdSubjectExportModal, { props: { show: true }, global: { stubs } });

    const submit = wrapper.find('[data-testid="pdse-submit"]');
    expect(submit.attributes('disabled')).toBeDefined();

    await wrapper.find('[data-testid="pdse-recipient"]').setValue('УМВД');
    await wrapper.find('[data-testid="pdse-request"]').setValue('исх. 1');
    expect(submit.attributes('disabled')).toBeUndefined();
  });
});
