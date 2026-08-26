import { describe, it, expect, beforeEach, vi } from 'vitest';
import { shallowMount, flushPromises } from '@vue/test-utils';
import { setActivePinia, createPinia } from 'pinia';

const createSupplement = vi.fn();
vi.mock('@/api/applications', () => ({ createSupplement: (...a) => createSupplement(...a) }));

const getFieldConfig = vi.fn().mockResolvedValue({ base: [], custom: [] });
vi.mock('@/api/attachment-templates', () => ({ getFieldConfig: (...a) => getFieldConfig(...a) }));

import { useDeletionsStore } from '@/stores/deletions';
import SupplementModal from '../SupplementModal.vue';

const APPLICATION = {
  id: 42,
  application_number: '№ 118',
  organization_id: 3,
  organization_name: 'ООО «Ромашка»',
  company_id: 9,
  company_name: 'Отдел монтажа',
};

// Срок заведомо в будущем: вложение с истёкшим entry_date_to модалка не предлагает
// (бэк такое отклоняет), и фикстура с прошлой датой давала бы пустой дропдаун.
const FUTURE = '2099-12-31';

const CARS_ATTACHMENT = {
  id: 12,
  attachment_type: 'cars',
  attachment_name: 'Пропуск на автотранспорт',
  attachment_display_name: 'Автотранспорт',
  unique_attachment_id: 101,
  entry_date_from: '2026-01-10',
  entry_date_to: FUTURE,
  entry_time_from: '08:00:00',
  entry_time_to: '20:00:00',
};

const PEOPLE_ATTACHMENT = {
  id: 13,
  attachment_type: 'people',
  attachment_name: 'Пропуск на людей',
  attachment_display_name: 'Сотрудники',
  unique_attachment_id: 102,
  entry_date_from: '2026-01-10',
  entry_date_to: FUTURE,
  entry_time_from: '09:00:00',
  entry_time_to: '18:00:00',
};

const ITEMS_ATTACHMENT = {
  id: 14,
  attachment_type: 'items',
  attachment_name: 'Пропуск на ТМЦ',
  attachment_display_name: 'ТМЦ',
  unique_attachment_id: 103,
  entry_date_from: '2026-01-10',
  entry_date_to: FUTURE,
};

const VEHICLE_ROW = {
  plateNumber: 'А123ВС77',
  mark: 'ГАЗель',
  markId: 4,
  unloadingPlace: 'Ворота 2',
  unloadPlaces: [7, 8],
  passage_tables: [21],
};

const EMPLOYEE_ROW = {
  lastName: 'Иванов',
  firstName: 'Пётр',
  middleName: 'Сергеевич',
  citizenshipId: 1,
  position: 'Монтажник',
  passportSeriesNumber: '4510 123456',
  patentNumber: null,
  otherPermission: null,
  targetTables: [33],
};

const ITEM_ROW = { itemName: 'Перфоратор', quantity: 2 };

async function mountModal(attachments = [CARS_ATTACHMENT]) {
  const wrapper = shallowMount(SupplementModal, {
    props: { show: true, application: APPLICATION, attachments },
    // Содержимое окна телепортируется в body - без стаба оно уезжает из поддерева обёртки.
    global: { stubs: { teleport: true } },
  });
  await wrapper.vm.$nextTick();
  return wrapper;
}

/** Шлёт строку так, как её эмитит реальная форма - через событие, не присваиванием. */
async function emitRow(wrapper, componentName, event, row) {
  wrapper.findComponent({ name: componentName }).vm.$emit(event, row);
  await wrapper.vm.$nextTick();
}

describe('SupplementModal - сборка дополнения (#1685)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    createSupplement.mockReset().mockResolvedValue({
      supplement_id: 1,
      number: 1,
      status: 'pending',
      counts: { vehicles: 1, employees: 0, items: 0 },
    });
    getFieldConfig.mockClear();
  });

  it('машины: additions[] несут attachment_id и vehicles в форме подачи', async () => {
    const wrapper = await mountModal();
    await emitRow(wrapper, 'VehicleForm', 'vehicle-added', VEHICLE_ROW);
    wrapper.vm.comment = 'Подрядчик прислал ещё одну машину';

    await wrapper.vm.submit();

    expect(createSupplement).toHaveBeenCalledTimes(1);
    const [applicationId, payload] = createSupplement.mock.calls[0];
    expect(applicationId).toBe(42);
    expect(payload.comment).toBe('Подрядчик прислал ещё одну машину');
    expect(payload.additions).toEqual([
      {
        attachment_id: 12,
        vehicles: [{
          car_number: 'А123ВС77',
          car_brand: 'ГАЗель',
          mark_id: 4,
          // mark_name бэкенд не читает (в VehicleInput такого поля нет), но подача его
          // шлёт с самого начала. Маппинг у подачи и дополнения общий, поэтому поле
          // едет в обоих - расхождение форм дороже лишнего ключа в JSON.
          mark_name: 'ГАЗель',
          unload_place: 'Ворота 2',
          unload_places: [7, 8],
          passage_tables: [21],
          pd_consent: false,
        }],
      },
    ]);
  });

  it('люди: employees собираются в форме подачи (target_tables, а не passage_tables)', async () => {
    const wrapper = await mountModal([PEOPLE_ATTACHMENT]);
    await emitRow(wrapper, 'EmployeeForm', 'employee-added', EMPLOYEE_ROW);

    await wrapper.vm.submit();

    const [, payload] = createSupplement.mock.calls[0];
    expect(payload.additions).toEqual([
      {
        attachment_id: 13,
        employees: [{
          last_name: 'Иванов',
          first_name: 'Пётр',
          middle_name: 'Сергеевич',
          citizenship_id: 1,
          position: 'Монтажник',
          passport_series_number: '4510 123456',
          pd_consent: false,
          patent_number: null,
          other_permission: null,
          target_tables: [33],
        }],
      },
    ]);
  });

  it('ТМЦ: items получают сквозной order_index', async () => {
    const wrapper = await mountModal([ITEMS_ATTACHMENT]);
    await emitRow(wrapper, 'ItemsForm', 'items-added', [ITEM_ROW, { itemName: 'Стремянка', quantity: 1 }]);

    await wrapper.vm.submit();

    const [, payload] = createSupplement.mock.calls[0];
    expect(payload.additions).toEqual([
      {
        attachment_id: 14,
        items: [
          { name: 'Перфоратор', count: 2, order_index: 1 },
          { name: 'Стремянка', count: 1, order_index: 2 },
        ],
      },
    ]);
  });

  it('строки, набранные в разных вложениях, уходят одним раундом', async () => {
    const wrapper = await mountModal([CARS_ATTACHMENT, PEOPLE_ATTACHMENT]);
    await emitRow(wrapper, 'VehicleForm', 'vehicle-added', VEHICLE_ROW);

    // Переключение вложения не должно терять уже набранное: additions[] на то и массив.
    wrapper.vm.onAttachmentChange(PEOPLE_ATTACHMENT.id);
    await wrapper.vm.$nextTick();
    await emitRow(wrapper, 'EmployeeForm', 'employee-added', EMPLOYEE_ROW);

    await wrapper.vm.submit();

    const [, payload] = createSupplement.mock.calls[0];
    expect(payload.additions.map(a => a.attachment_id)).toEqual([12, 13]);
    expect(payload.additions[0].vehicles).toHaveLength(1);
    expect(payload.additions[1].employees).toHaveLength(1);
  });

  it('пустой комментарий уходит как null, а не пустой строкой', async () => {
    const wrapper = await mountModal();
    await emitRow(wrapper, 'VehicleForm', 'vehicle-added', VEHICLE_ROW);
    wrapper.vm.comment = '   ';

    await wrapper.vm.submit();

    expect(createSupplement.mock.calls[0][1].comment).toBeNull();
  });

  it('без добавленных строк отправка заблокирована и запроса нет', async () => {
    const wrapper = await mountModal();
    expect(wrapper.vm.canSubmit).toBe(false);
    await wrapper.vm.submit();
    expect(createSupplement).not.toHaveBeenCalled();
  });

  it('вложение с истёкшим сроком не предлагается', async () => {
    const wrapper = await mountModal([{ ...CARS_ATTACHMENT, entry_date_to: '2020-01-01' }]);
    expect(wrapper.vm.attachmentOptions).toEqual([]);
    expect(wrapper.vm.selectedAttachmentId).toBeNull();
  });

  it('конфиг полей грузится по unique_attachment_id выбранного вложения', async () => {
    const wrapper = await mountModal([CARS_ATTACHMENT, PEOPLE_ATTACHMENT]);
    // Конфиг едет через динамический import - одного nextTick мало.
    await flushPromises();
    expect(getFieldConfig).toHaveBeenCalledWith(101);

    wrapper.vm.onAttachmentChange(PEOPLE_ATTACHMENT.id);
    await flushPromises();
    expect(getFieldConfig).toHaveBeenCalledWith(102);
  });
});

describe('SupplementModal - результат отправки (#1685)', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    createSupplement.mockReset();
    getFieldConfig.mockClear();
  });

  async function readyModal() {
    const wrapper = await mountModal();
    await emitRow(wrapper, 'VehicleForm', 'vehicle-added', VEHICLE_ROW);
    return wrapper;
  }

  it('pending: «отправлено на согласование», окно закрывается, наверх уходит результат', async () => {
    createSupplement.mockResolvedValue({ supplement_id: 5, number: 2, status: 'pending', counts: {} });
    const wrapper = await readyModal();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await wrapper.vm.submit();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      prefix: 'Дополнение отправлено на согласование по заявке ',
      bold: '№ 118',
      type: 'success',
    }));
    expect(wrapper.emitted('submitted')[0][0]).toMatchObject({ supplement_id: 5 });
    expect(wrapper.emitted('close')).toBeTruthy();
  });

  it('merged: «строки добавлены» - добавка влилась в текущий круг, отдельного раунда нет', async () => {
    createSupplement.mockResolvedValue({ supplement_id: 6, number: 1, status: 'merged', counts: {} });
    const wrapper = await readyModal();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await wrapper.vm.submit();

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      prefix: 'Строки добавлены в заявку ',
      type: 'success',
    }));
  });

  it('409: текст бэка показывается как есть, окно НЕ закрывается, строки целы', async () => {
    const conflict = new Error('У заявки уже есть дополнение на рассмотрении - дождитесь решения по нему');
    conflict.status = 409;
    createSupplement.mockRejectedValue(conflict);
    const wrapper = await readyModal();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await wrapper.vm.submit();

    expect(notify).toHaveBeenCalledWith({
      bold: 'У заявки уже есть дополнение на рассмотрении - дождитесь решения по нему',
      type: 'error',
    });
    expect(wrapper.emitted('close')).toBeFalsy();
    expect(wrapper.emitted('submitted')).toBeFalsy();
    // Набранное не теряем: человек дожидается решения и жмёт отправку снова.
    expect(wrapper.vm.totalRows).toBe(1);
    expect(wrapper.vm.submitting).toBe(false);
  });

  it('ошибка без кода: показываем сообщение бэка, а не сырой объект', async () => {
    createSupplement.mockRejectedValue(new Error('Вложение не принадлежит этой заявке'));
    const wrapper = await readyModal();
    const notify = vi.spyOn(useDeletionsStore(), 'notify');

    await wrapper.vm.submit();

    expect(notify).toHaveBeenCalledWith({ bold: 'Вложение не принадлежит этой заявке', type: 'error' });
    expect(wrapper.emitted('close')).toBeFalsy();
  });
});
