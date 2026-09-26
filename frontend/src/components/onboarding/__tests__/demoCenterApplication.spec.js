import { describe, it, expect, afterEach } from 'vitest';
import { createCenterDemoResponder, syncDemoBackend } from '../demoBackend';
import { DEMO_CENTER_APPLICATION_ID, DEMO_CENTER_ATTACHMENT_ID, DEMO_CENTER_PEOPLE_ID } from '../demoCenterApplication';
import { interceptRead } from '@/api/readInterceptor';
import { acceptOnboardingSteps } from '../acceptOnboardingSteps';

/**
 * Примерная заявка нужна ради одного: чтобы шаги тура было на чём показать.
 * Поэтому замок сверяет её не саму по себе, а против состава тура - каждый шаг,
 * которому нужны данные заявки, обязан их получить. Иначе мы снова придём к
 * «двадцать шагов обещано, тринадцать показано» (#2580, #2592).
 */

const ответ = (path, method = 'GET') => createCenterDemoResponder()(path, method);
const заявка = () => ответ(`/applications/${DEMO_CENTER_APPLICATION_ID}/details`).data;

describe('примерная заявка Центра', () => {
  afterEach(() => syncDemoBackend(false));

  it('список Центра отдаёт ровно её', () => {
    const res = ответ('/applications?page=1');
    expect(res.data).toHaveLength(1);
    expect(res.data[0].id).toBe(DEMO_CENTER_APPLICATION_ID);
    expect(res.meta.total).toBe(1);
  });

  it('в ней есть машины с местами разгрузки и проездом', () => {
    // Шаги «Места и проезд в строке» и «Назначить по одной строке» показывают
    // именно чипы: у заявки с товарами таких колонок нет вовсе.
    const cars = ответ(`/attachments/${DEMO_CENTER_ATTACHMENT_ID}/cars`).data;
    expect(cars.length).toBeGreaterThan(1);
    expect(cars[0].unload_places[0]).toHaveProperty('name');
    expect(cars[0].target_tables[0]).toHaveProperty('display_name');
    expect(cars.some((c) => !c.unload_places.length), 'пустая строка для доназначения').toBe(true);
  });

  it('вложение - именно машины, а не товары', () => {
    const attachments = ответ(`/applications/${DEMO_CENTER_APPLICATION_ID}/attachments`).data;
    expect(attachments[0].attachment_type).toBe('cars');
  });

  it('признаки для шагов про тег, дополнение, разбор наименования и бланки', () => {
    const a = заявка();
    expect(a.blacklist_flags_count, 'тег «похоже на ЧС» в строке списка').toBeGreaterThan(0);
    expect(a.supplements_count, 'блок дополнения в карточке').toBeGreaterThan(0);
    expect(a.organization_moderation_status, 'плашка разбора наименования').toBe('pending');
    expect(a.organization_id, 'без id плашка не рисуется').toBeGreaterThan(0);
    expect(a.has_blank_template, 'кнопка «Скачать»').toBe(true);
  });

  it('статус рабочий - иначе нет ни приёма, ни назначения мест', () => {
    expect(['Непрочитано', 'В обработке', 'В работе']).toContain(заявка().status);
  });

  it('дополнение приходит отдельным запросом', () => {
    const s = ответ(`/applications/${DEMO_CENTER_APPLICATION_ID}/supplements`).data;
    expect(s).toHaveLength(1);
    expect(s[0].status).toBe('pending');
  });

  it('запись по примерной заявке наружу не уходит', () => {
    // Такой заявки на сервере нет: отметка о прочтении возвращалась бы отказом
    // посреди обучения.
    expect(ответ(`/applications/${DEMO_CENTER_APPLICATION_ID}/read`, 'POST')).toEqual({ success: true, data: null });
    expect(ответ('/applications/123/read', 'POST'), 'чужое не трогаем').toBe(null);
  });

  it('во вложении с людьми есть места прохода и пустая строка', () => {
    const люди = ответ(`/attachments/${DEMO_CENTER_PEOPLE_ID}/employees`).data;
    expect(люди.length).toBeGreaterThan(1);
    expect(люди[0].target_tables[0]).toHaveProperty('display_name');
    expect(люди.some((e) => !e.target_tables.length), 'строка для доназначения').toBe(true);
    expect(люди[0].position, 'должность вместо марки').toBeTruthy();
  });

  it('вложений два: машины и люди', () => {
    const виды = ответ(`/applications/${DEMO_CENTER_APPLICATION_ID}/attachments`).data
      .map((a) => a.attachment_type);
    expect(виды).toEqual(['cars', 'people']);
  });

  it('согласующие есть - иначе блок согласования пустая рамка', () => {
    const люди = ответ(`/applications/${DEMO_CENTER_APPLICATION_ID}/responsible-users`).data;
    expect(люди.length).toBeGreaterThan(1);
    expect(люди[0].approval_status).toBe('approved');
    expect(люди[0].last_name).toBeTruthy();
    expect(заявка().confirmation, 'колонка «Подтверждение» в списке').toBeTruthy();
  });

  it('каждый шаг про содержимое карточки обеспечен данными', () => {
    const данные = {
      'attachment-elements': ответ(`/attachments/${DEMO_CENTER_ATTACHMENT_ID}/cars`).data.length > 0,
      'attachment-chip': ответ(`/attachments/${DEMO_CENTER_ATTACHMENT_ID}/cars`).data[0].unload_places.length > 0,
      'attachment-assign-open': true,
      'attachment-assign-all-open': true,
      'supplement-panel': заявка().supplements_count > 0,
      'ob-org-moderation': заявка().organization_moderation_status === 'pending',
      'app-detail-button-download': заявка().has_blank_template,
      'ob-center-blacklist-tag': заявка().blacklist_flags_count > 0,
    };
    const необеспеченные = acceptOnboardingSteps
      .filter((s) => s.element)
      .map((s) => s.element.match(/^\[data-testid="([^"]+)"\]$/)[1])
      .filter((id) => id in данные && !данные[id]);
    expect(необеспеченные).toEqual([]);
  });
});

describe('включение подмены', () => {
  afterEach(() => syncDemoBackend(false));

  it('в туре принимающего пример поднимается даже при своих заявках', () => {
    syncDemoBackend(true, true, 'accept');
    expect(interceptRead('/applications?page=1')).not.toBe(null);
  });

  it('в туре заявителя Центр не подменяется', () => {
    syncDemoBackend(true, false, 'user');
    expect(interceptRead('/applications?page=1')).toBe(null);
  });

  it('по окончании тура подмена снимается', () => {
    syncDemoBackend(true, true, 'accept');
    syncDemoBackend(false);
    expect(interceptRead('/applications?page=1')).toBe(null);
  });
});
