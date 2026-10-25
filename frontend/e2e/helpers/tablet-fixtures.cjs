const CLOCK = '2030-05-15T09:00:00Z';
const templates = [
  { id: 9901, title: 'Автомобили', name: 'Автозаявка', display_name: 'Автозаявка', attachment_type: 'cars' },
  { id: 9902, title: 'Сотрудники', name: 'Сотрудники', display_name: 'Сотрудники', attachment_type: 'people' },
  { id: 9903, title: 'Материальные ценности', name: 'Ввоз', display_name: 'Ввоз', attachment_type: 'items' },
  { id: 9904, title: 'Разовые пропуска', name: 'Пропуск', display_name: 'Пропуск', attachment_type: 'people' },
];
const tableFields = kind => (kind === 'people'
  ? ['last_name', 'first_name', 'middle_name', 'organization', 'company', 'application_id', 'valid_until', 'pass_time', 'position', 'status']
  : ['car_number', 'car_brand', 'organization', 'company', 'application_id', 'unload_place', 'valid_until', 'time_range', 'status']
).map((field_name, i) => ({ field_name, is_visible: true, display_order: i + 1, priority: i + 1 }));

async function tabletFixtures(page, options = {}) {
  await page.clock.setFixedTime(new Date(CLOCK));
  const token = `${Buffer.from('{}').toString('base64url')}.${Buffer.from(JSON.stringify({ user_id: 1, username: 'layout-fixture', is_super_admin: !options.consent, exp: Date.parse(CLOCK) / 1000 + 86400 })).toString('base64url')}.fixture`;
  const requests = [];
  const cars = Array.from({ length: 90 }, (_, i) => ({ id: i + 1, license_plate: `А${100 + i}АА 77`, mark: `Учебная машина ${i + 1}`, status: true, user_id: 1, organization_id: 1 }));
  const people = Array.from({ length: 90 }, (_, i) => ({ id: i + 1, last_name: 'Учебный', first_name: `Сотрудник ${i + 1}`, position: 'Учебная должность', status: true, user_id: 1, organization_id: 1 }));
  await page.route(url => url.pathname.startsWith('/api/'), async route => {
    const url = new URL(route.request().url());
    const path = url.pathname.slice(4);
    const send = (data, meta) => route.fulfill({ json: { success: true, data, ...(meta ? { meta } : {}) } });
    if (path === '/refresh-token') return send({ token });
    if (path === '/users/me') return send({ id: 1, username: 'layout-fixture', type_code: 'admin', is_super_admin: !options.consent, created_at: '2020-01-01T00:00:00Z' });
    if (path === '/permissions/my') return send({ mode: options.consent ? 'admin' : 'super', permissions: [], denied: [] });
    if (path === '/consents/gate') return send({ required: !!options.consent, version: 1, version_at: CLOCK, text: options.consent ? '<h3>Учебный документ</h3>' + '<p>Учебный текст для проверки прокрутки и кнопок на телефоне.</p>'.repeat(40) : '', document: null });
    if (path === '/user-data' && options.password) return route.fulfill({ status: 403, headers: { 'X-Password-Change-Required': '1' }, json: { success: false, code: 'PASSWORD_CHANGE_REQUIRED', error: 'Учебное требование смены пароля' } });
    if (path === '/onboarding') return send({ completed: { user: 999, guard: 999, approve: 999, accept: 999, admin: 999 }, finished: ['user', 'guard', 'approve', 'accept', 'admin'] });
    if (path === '/user-data') return send({ id: 1, last_name: 'Учебный', first_name: 'Заявитель', organization: 'Учебная организация', organization_id: 1, phone: '79990000000' });
    if (path === '/attachments') return send(templates);
    if (/^\/attachments\/\d+\/field-config$/.test(path)) return send({});
    if (path === '/application-approvers/recipients') return send(Array.from({ length: 5 }, (_, i) => ({ user_id: i + 2, name: `Учебный получатель ${i + 1}` })));
    if (path.endsWith('/ownership-info')) return send({ user_id: 1, organization_id: 1, company_id: 1 });
    if (path.startsWith('/system-tables/name/')) return send({ table: { id: 991, name: path.split('/').at(-1), display_name: 'Учебный КПП', table_type: path.includes('people') ? 'people' : 'cars', font_size: 14 }, fields: tableFields(path.includes('people') ? 'people' : 'cars') });
    if (path === '/system-tables/991') return send({ table: { id: 991, table_type: 'cars', font_size: 14 }, fields: tableFields('cars') });
    if (path === '/employees/active-for-table/991') return send(people.slice(0, 45).map(p => ({ ...p, organization: 'Учебная организация', application_id: 1, application_number: '20300515/001', entry_date_to: '2030-05-30', pass_time: '09:00-18:00', pd_consent: true, territory_status: 0 })));
    if (path === '/cars/active-for-table/991') return send(cars.slice(0, 45).map(c => ({ ...c, car_number: c.license_plate, car_brand: c.mark, organization: 'Учебная организация', application_id: 1, application_number: '20300515/001', entry_date_to: '2030-05-30', territory_status: 0 })));
    if (/^\/(cars|employees)\/fact-for-table\/991$/.test(path)) return send(cars.slice(0, 8).map(c => ({ ...c, car_number: 'По факту', car_brand: c.mark, organization: 'Учебная организация', entry_date_to: '2030-05-30', territory_status: 0 })));
    if (path === '/unique-cars' || path === '/unique-employees') {
      const items = path === '/unique-cars' ? cars : people;
      const pageNumber = Number(url.searchParams.get('page') || 1);
      const perPage = Number(url.searchParams.get('per_page') || 30);
      requests.push({ path, page: pageNumber });
      return send(items.slice((pageNumber - 1) * perPage, pageNumber * perPage), { total: items.length, page: pageNumber, per_page: perPage });
    }
    if (path === '/applications/files/drafts') return send({ items: [], usage: { saved_count: 0, reserved_count: 0, saved_bytes: 0, reserved_bytes: 0 }, limits: { max_count: 50, max_bytes: 209715200 } }, { total: 0, page: 1, per_page: 20 });
    if (path === '/license-plate-formats') return send([{ format: { id: 1, name: 'Российский', is_default: true }, cells: [
      { cell_type: 'letters', alphabet_type: 'cyrillic', max_length: 1 }, { cell_type: 'numbers', max_length: 3 },
      { cell_type: 'letters', alphabet_type: 'cyrillic', max_length: 2 }, { cell_type: 'numbers', max_length: 2 },
    ] }]);
    if (path === '/settings/contacts') return send({ email: 'layout@example.org', phone: '+7 (000) 000-00-00' });
    if (path === '/settings/password-policy') return send({ min_length: 8, require_letter: true, require_digit: true, require_uppercase: false, require_lowercase: false, require_special: false });
    return send([]);
  });
  return requests;
}

module.exports = { tabletFixtures, CLOCK };
