import { describe, it, expect, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';

import ApplicationAttachmentDetail from '../ApplicationAttachmentDetail.vue';

function car(over = {}) {
  return {
    id: 1,
    car_number: 'А123ВС',
    car_brand: 'Toyota',
    unload_places: [],
    ...over,
  };
}

function employee(over = {}) {
  return {
    id: 1,
    last_name: 'Иванов',
    first_name: 'Иван',
    middle_name: 'Иванович',
    position: 'Водитель',
    target_tables: [],
    ...over,
  };
}

function flag(over = {}) {
  return {
    flag_id: 10,
    matched_value: 'А124ВС Toyota',
    matched_reason: 'похожий номер',
    similarity: 0.85,
    overridden: false,
    ...over,
  };
}

function mountCars(cars) {
  return mount(ApplicationAttachmentDetail, {
    props: {
      attachment: { id: 1, attachment_type: 'cars', attachment_display_name: 'Машины' },
      cars,
    },
  });
}

function mountEmployees(employees) {
  return mount(ApplicationAttachmentDetail, {
    props: {
      attachment: { id: 1, attachment_type: 'people', attachment_display_name: 'Люди' },
      employees,
    },
  });
}

function mountItems(items) {
  return mount(ApplicationAttachmentDetail, {
    props: {
      attachment: { id: 1, attachment_type: 'items', attachment_display_name: 'Имущество' },
      items,
    },
  });
}

describe('ApplicationAttachmentDetail — подсветка возможного обхода ЧС (#481)', () => {
  it('помеченная машина: красноватый модификатор + бейдж "похоже на ЧС"', () => {
    const wrapper = mountCars([car({ blacklist_similar: flag() })]);
    const item = wrapper.find('[data-testid="attachment-element-row"]');
    expect(item.classes()).toContain('el-row--flagged');

    const badge = item.find('.blacklist-badge');
    expect(badge.exists()).toBe(true);
    expect(badge.text()).toBe('похоже на ЧС');
    expect(badge.classes()).toContain('badge--danger');
  });

  it('чистая машина: нет модификатора и нет бейджа', () => {
    const wrapper = mountCars([car()]);
    const item = wrapper.find('[data-testid="attachment-element-row"]');
    expect(item.classes()).not.toContain('el-row--flagged');
    expect(item.find('.blacklist-badge').exists()).toBe(false);
  });

  it('подтверждённый пропуск (overridden): без красной подсветки, нейтральный бейдж', () => {
    const wrapper = mountCars([car({ blacklist_similar: flag({ overridden: true }) })]);
    const item = wrapper.find('[data-testid="attachment-element-row"]');
    expect(item.classes()).not.toContain('el-row--flagged');

    const badge = item.find('.blacklist-badge');
    expect(badge.exists()).toBe(true);
    expect(badge.text()).toBe('ЧС снят');
    expect(badge.classes()).toContain('badge--neutral');
  });

  it('подсказка ЧС идёт через data-hint (тёмный пузырёк проекта), а не через title', () => {
    const wrapper = mountCars([car({ blacklist_similar: flag() })]);
    const badge = wrapper.find('.blacklist-badge');
    expect(badge.attributes('data-hint')).toBe('Возможный обход чёрного списка. Похоже на: А124ВС Toyota (похожий номер)');
    expect(badge.attributes('title')).toBeUndefined();
  });

  it('помеченный сотрудник: красноватый модификатор + бейдж', () => {
    const wrapper = mountEmployees([
      employee({ blacklist_similar: flag({ matched_value: 'Иваноф Иван Иванович', matched_reason: 'опечатка в фамилии' }) }),
    ]);
    const item = wrapper.find('[data-testid="attachment-element-row"]');
    expect(item.classes()).toContain('el-row--flagged');

    const badge = item.find('.blacklist-badge');
    expect(badge.exists()).toBe(true);
    expect(badge.text()).toBe('похоже на ЧС');
    expect(badge.attributes('data-hint')).toContain('Иваноф Иван Иванович');
  });

  it('подпись бейджа короткая: длинная не помещалась в колонку действий и резалась', () => {
    const wrapper = mountCars([car({ blacklist_similar: flag({ overridden: true }) })]);
    const badge = wrapper.find('.blacklist-badge');
    expect(badge.text()).toBe('ЧС снят');
    expect(badge.attributes('data-hint')).toContain('Пропуск подтверждён');
  });

  it('в футере видна сводка по количеству помеченных строк', () => {
    const wrapper = mountCars([
      car({ id: 1, blacklist_similar: flag() }),
      car({ id: 2 }),
      car({ id: 3, blacklist_similar: flag() }),
    ]);
    expect(wrapper.find('[data-testid="attachment-flagged-summary"]').text()).toBe('2 похоже на ЧС');
  });
});

describe('ApplicationAttachmentDetail — кнопка "Пропустить" override (#481, срез 6a)', () => {
  // Меню действий строки телепортируется в body - стабим Teleport, иначе find его не видит.
  function mountCarsWith(cars, props = {}) {
    return mount(ApplicationAttachmentDetail, {
      props: {
        attachment: { id: 1, attachment_type: 'cars', attachment_display_name: 'Машины' },
        cars,
        ...props,
      },
      global: { stubs: { Teleport: true } },
    });
  }

  it('canOverride+canRemove: на помеченной строке кнопка «Принять» и стрелка меню', () => {
    const wrapper = mountCarsWith([car({ blacklist_similar: flag() })], { canOverride: true, canRemove: true });
    const btn = wrapper.find('[data-testid="blacklist-override-btn"]');
    expect(btn.exists()).toBe(true);
    expect(btn.text()).toContain('Принять');
    expect(wrapper.find('[data-testid="row-actions-toggle"]').exists()).toBe(true);
  });

  it('canOverride без canRemove: стрелки нет - в меню остался бы дубль «Принять»', () => {
    const wrapper = mountCarsWith([car({ blacklist_similar: flag() })], { canOverride: true });
    expect(wrapper.find('[data-testid="blacklist-override-btn"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="row-actions-toggle"]').exists()).toBe(false);
    // Кнопка остаётся цельной: половинку скругления обнуляет только сдвоенный вид.
    expect(wrapper.find('.split-btn').classes()).toContain('split-btn--single');
  });

  it('canOverride=false (дефолт): кнопки действия нет даже на помеченной строке', () => {
    const wrapper = mountCarsWith([car({ blacklist_similar: flag() })]);
    expect(wrapper.find('[data-testid="blacklist-override-btn"]').exists()).toBe(false);
  });

  it('overridden строка: кнопки нет даже при canOverride=true', () => {
    const wrapper = mountCarsWith([car({ blacklist_similar: flag({ overridden: true }) })], { canOverride: true });
    expect(wrapper.find('[data-testid="blacklist-override-btn"]').exists()).toBe(false);
  });

  it('клик по «Принять» сразу эмитит override-element, но НЕ open-vehicle', async () => {
    const f = flag();
    const wrapper = mountCarsWith([car({ car_number: 'А123ВС', blacklist_similar: f })], { canOverride: true });
    await wrapper.find('[data-testid="blacklist-override-btn"]').trigger('click');

    const emitted = wrapper.emitted('override-element');
    expect(emitted).toHaveLength(1);
    expect(emitted[0][0]).toEqual({ label: 'А123ВС', flag: f });
    // @click.stop не должен пробросить клик на строку (открытие модалки машины)
    expect(wrapper.emitted('open-vehicle')).toBeUndefined();
  });

  it('сотрудник: override-element несёт ФИО как label', async () => {
    const f = flag({ matched_value: 'Иванов И.И.' });
    const wrapper = mount(ApplicationAttachmentDetail, {
      props: {
        attachment: { id: 1, attachment_type: 'people', attachment_display_name: 'Люди' },
        employees: [employee({ last_name: 'Иваноф', first_name: 'Иван', blacklist_similar: f })],
        canOverride: true,
      },
      global: { stubs: { Teleport: true } },
    });
    await wrapper.find('[data-testid="blacklist-override-btn"]').trigger('click');
    expect(wrapper.emitted('override-element')[0][0]).toEqual({ label: 'Иваноф Иван Иванович', flag: f });
  });
});

describe('ApplicationAttachmentDetail — колонки и подписи (#1392)', () => {
  function headerLabels(wrapper) {
    return wrapper.findAll('.el-head .el-cell').map(cell => cell.text());
  }

  it('машины: колонки названы, места разгрузки и проезд разделены', () => {
    const wrapper = mountCars([car()]);
    expect(headerLabels(wrapper)).toEqual(['Гос. номер', 'Марка', 'Места разгрузки', 'Проезд']);
  });

  it('сотрудники: ФИО, должность и места прохода — отдельные колонки', () => {
    const wrapper = mountEmployees([employee()]);
    expect(headerLabels(wrapper)).toEqual(['ФИО', 'Должность', 'Места прохода']);
  });

  it('ТМЦ: наименование и количество, в футере сумма единиц', () => {
    const wrapper = mountItems([
      { id: 1, name: 'Ноутбук', count: 2 },
      { id: 2, name: 'Стеллаж', count: 12 },
    ]);
    expect(headerLabels(wrapper)).toEqual(['Наименование', 'Количество']);
    expect(wrapper.find('.qty').text()).toBe('2 шт');
    expect(wrapper.find('[data-testid="attachment-elements-total"]').text()).toBe('Всего позиций: 2, единиц: 14');
  });

  it('каждая ячейка несёт data-label — на телефоне из него берётся подпись поля', () => {
    const wrapper = mountCars([car()]);
    const labels = wrapper.findAll('[data-testid="attachment-element-row"] .el-cell')
      .map(cell => cell.attributes('data-label'));
    expect(labels).toEqual(['Гос. номер', 'Марка', 'Места разгрузки', 'Проезд']);
  });

  it('ключевое значение выводится отдельным полем и не склеено с маркой', () => {
    const wrapper = mountCars([car({ car_number: 'А123ВС 750', car_brand: 'Volvo FH' })]);
    expect(wrapper.find('[data-testid="attachment-element-key"]').text()).toBe('А123ВС 750');
  });
});

describe('ApplicationAttachmentDetail — чипы мест (#1392)', () => {
  function chipTexts(wrapper) {
    return wrapper.findAll('[data-testid="attachment-chip"], [data-testid="attachment-chip-more"]')
      .map(chip => chip.text());
  }

  it('места разгрузки выводятся чипами по названию', () => {
    const wrapper = mountCars([car({
      unload_places: [{ id: 1, name: 'Склад №1' }, { id: 2, name: 'Рампа А' }],
    })]);
    expect(chipTexts(wrapper)).toEqual(['Склад №1', 'Рампа А']);
  });

  it('лишние места сворачиваются в "+N", полный список — в подсказке data-hint', () => {
    const wrapper = mountCars([car({
      unload_places: [
        { id: 1, name: 'Склад №1' },
        { id: 2, name: 'Рампа А' },
        { id: 3, name: 'Рампа Б' },
        { id: 4, name: 'Северный въезд' },
      ],
    })]);
    expect(chipTexts(wrapper)).toEqual(['Склад №1', 'Рампа А', '+2']);

    const more = wrapper.find('[data-testid="attachment-chip-more"]');
    expect(more.attributes('data-hint')).toBe('Склад №1, Рампа А, Рампа Б, Северный въезд');
  });

  it('таблицы проезда берут display_name', () => {
    const wrapper = mountCars([car({
      target_tables: [{ id: 7, name: 'post72', display_name: 'ПОСТ №72 (АВТО)' }],
    })]);
    expect(chipTexts(wrapper)).toEqual(['ПОСТ №72 (АВТО)']);
  });

  it('пустой список мест показывает прочерк, а не пустую ячейку', () => {
    const wrapper = mountCars([car({ unload_places: [] })]);
    expect(wrapper.find('.chip--empty').text()).toBe('—');
  });

  it('когда в колонку не влезает ни одно название, вместо чипов встаёт счётчик', () => {
    const wrapper = mountCars([car()]);
    const col = wrapper.vm.allColumns.find(c => c.key === 'tables');
    // Ширины колонок снимаются с DOM (в jsdom нет раскладки) - задаём напрямую.
    wrapper.vm.chipColumnWidths = { tables: 80 };
    const row = {
      target_tables: [
        { id: 1, display_name: 'ПОСТ №15 (ГРУЗОВОЙ)' },
        { id: 2, display_name: 'ПОСТ №72 (АВТО)' },
      ],
    };
    expect(wrapper.vm.visibleChips(row, col)).toEqual([{
      key: 'summary',
      text: '2 поста',
      hint: 'ПОСТ №15 (ГРУЗОВОЙ), ПОСТ №72 (АВТО)',
      isMore: true,
    }]);
  });

  it('в широкой колонке помещаются оба названия целиком', () => {
    const wrapper = mountCars([car()]);
    const col = wrapper.vm.allColumns.find(c => c.key === 'tables');
    wrapper.vm.chipColumnWidths = { tables: 420 };
    const row = {
      target_tables: [
        { id: 1, display_name: 'ПОСТ №15' },
        { id: 2, display_name: 'ПОСТ №72' },
      ],
    };
    expect(wrapper.vm.visibleChips(row, col).map(c => c.text)).toEqual(['ПОСТ №15', 'ПОСТ №72']);
  });

  it('единственное длинное название остаётся чипом: счётчик «1 место» ничего не сказал бы', () => {
    const wrapper = mountCars([car()]);
    const col = wrapper.vm.allColumns.find(c => c.key === 'places');
    wrapper.vm.chipColumnWidths = { places: 60 };
    const row = { unload_places: [{ id: 1, name: 'Северный въезд' }] };
    expect(wrapper.vm.visibleChips(row, col).map(c => c.text)).toEqual(['Северный въезд']);
  });

  it('счётчик склоняется по числу', () => {
    const wrapper = mountCars([car()]);
    const forms = ['пост', 'поста', 'постов'];
    expect(wrapper.vm.plural(1, forms)).toBe('пост');
    expect(wrapper.vm.plural(3, forms)).toBe('поста');
    expect(wrapper.vm.plural(5, forms)).toBe('постов');
    expect(wrapper.vm.plural(11, forms)).toBe('постов');
    expect(wrapper.vm.plural(21, forms)).toBe('пост');
  });
});

describe('ApplicationAttachmentDetail — клик по строке (#1392)', () => {
  it('по умолчанию строка кликабельна и открывает карточку машины', async () => {
    const c = car();
    const wrapper = mountCars([c]);
    const row = wrapper.find('[data-testid="attachment-element-row"]');
    expect(row.classes()).toContain('el-row--clickable');

    await row.trigger('click');
    expect(wrapper.emitted('open-vehicle')[0][0]).toEqual(c);
  });

  it('сотрудник открывает свою карточку', async () => {
    const e = employee();
    const wrapper = mountEmployees([e]);
    await wrapper.find('[data-testid="attachment-element-row"]').trigger('click');
    expect(wrapper.emitted('open-employee')[0][0]).toEqual(e);
  });

  it('interactive=false («Доступные мне»): строка не кликабельна и ничего не эмитит', async () => {
    const wrapper = mount(ApplicationAttachmentDetail, {
      props: {
        attachment: { id: 1, attachment_type: 'cars', attachment_display_name: 'Машины' },
        cars: [car()],
        interactive: false,
      },
    });
    const row = wrapper.find('[data-testid="attachment-element-row"]');
    expect(row.classes()).not.toContain('el-row--clickable');

    await row.trigger('click');
    expect(wrapper.emitted('open-vehicle')).toBeUndefined();
  });

  it('ТМЦ карточку не открывают', async () => {
    const wrapper = mountItems([{ id: 1, name: 'Ноутбук', count: 2 }]);
    const row = wrapper.find('[data-testid="attachment-element-row"]');
    expect(row.classes()).not.toContain('el-row--clickable');

    await row.trigger('click');
    expect(wrapper.emitted('open-vehicle')).toBeUndefined();
  });
});

describe('ApplicationAttachmentDetail — состояния списка (#1392)', () => {
  it('загрузка: спиннер вместо таблицы', () => {
    const wrapper = mount(ApplicationAttachmentDetail, {
      props: {
        attachment: { id: 1, attachment_type: 'cars', attachment_display_name: 'Машины' },
        cars: [],
        loading: true,
      },
    });
    expect(wrapper.find('.loading-spinner').exists()).toBe(true);
    expect(wrapper.find('[data-testid="attachment-elements"]').exists()).toBe(false);
  });

  it('пустой список: текст по типу вложения, без шапки колонок', () => {
    const wrapper = mountEmployees([]);
    expect(wrapper.find('.no-data').text()).toBe('Нет данных о сотрудниках');
    expect(wrapper.find('[data-testid="attachment-elements"]').exists()).toBe(false);
  });

  it('счётчик в заголовке равен числу строк', () => {
    const wrapper = mountCars([car({ id: 1 }), car({ id: 2 })]);
    expect(wrapper.find('[data-testid="attachment-elements-count"]').text()).toBe('2');
    expect(wrapper.find('[data-testid="attachment-elements-total"]').text()).toBe('Всего: 2');
  });
});

describe('ApplicationAttachmentDetail — поиск по списку (#1392)', () => {
  const fleet = [
    car({ id: 1, car_number: 'К 050 УА 902', car_brand: 'BMW X5', unload_places: [{ id: 1, name: 'Склад №1' }] }),
    car({ id: 2, car_number: 'М 234 ОО 123', car_brand: 'Шкода', unload_places: [{ id: 2, name: 'Рампа А' }] }),
    car({ id: 3, car_number: 'У 456 АУ 964', car_brand: 'Мерседес', target_tables: [{ id: 7, display_name: 'ПОСТ №72 (АВТО)' }] }),
  ];

  async function search(wrapper, query) {
    await wrapper.find('[data-testid="attachment-elements-search"] input').setValue(query);
    return wrapper;
  }

  function numbers(wrapper) {
    return wrapper.findAll('[data-testid="attachment-element-key"]').map(el => el.text());
  }

  it('поле поиска стоит в шапке рядом с заголовком списка', () => {
    const wrapper = mountCars(fleet);
    const head = wrapper.find('.el-section__head');
    expect(head.find('[data-testid="attachment-elements-search"]').exists()).toBe(true);
    expect(head.find('h5').text()).toBe('Автомобили');
  });

  it('ищет по гос. номеру, в том числе без пробелов', async () => {
    const wrapper = mountCars(fleet);
    await search(wrapper, 'м234оо');
    expect(numbers(wrapper)).toEqual(['М 234 ОО 123']);
  });

  it('ищет по марке независимо от регистра', async () => {
    const wrapper = mountCars(fleet);
    await search(wrapper, 'мерс');
    expect(numbers(wrapper)).toEqual(['У 456 АУ 964']);
  });

  it('ищет по месту разгрузки — не только по своим колонкам', async () => {
    const wrapper = mountCars(fleet);
    await search(wrapper, 'рампа');
    expect(numbers(wrapper)).toEqual(['М 234 ОО 123']);
  });

  it('ищет по посту проезда', async () => {
    const wrapper = mountCars(fleet);
    await search(wrapper, 'пост №72');
    expect(numbers(wrapper)).toEqual(['У 456 АУ 964']);
  });

  it('переживает неправильную раскладку клавиатуры', async () => {
    const wrapper = mountCars(fleet);
    // "irjlf" на английской раскладке даёт "шкода"
    await search(wrapper, 'irjlf');
    expect(numbers(wrapper)).toEqual(['М 234 ОО 123']);
  });

  it('счётчик и футер показывают, сколько нашлось из скольких', async () => {
    const wrapper = mountCars(fleet);
    await search(wrapper, 'рампа');
    expect(wrapper.find('[data-testid="attachment-elements-count"]').text()).toBe('1');
    expect(wrapper.find('[data-testid="attachment-elements-total"]').text()).toBe('Найдено: 1 из 3');
  });

  it('опечатка в номере не мешает: «942» находит «У 952 ЕУ 935»', async () => {
    const wrapper = mountCars([
      car({ id: 1, car_number: 'У 952 ЕУ 935', car_brand: 'BMW X5' }),
      car({ id: 2, car_number: 'У 465 КУ 423', car_brand: 'BMW X5' }),
    ]);
    await search(wrapper, '942');
    // «У 465 КУ 423» похоже только на склейке («у42» из «ку423»), пословно - нет
    expect(numbers(wrapper)).toEqual(['У 952 ЕУ 935']);
  });

  it('номер слитно с опечаткой: «у953» находит «У 952 ЕУ 935»', async () => {
    const wrapper = mountCars([
      car({ id: 1, car_number: 'У 952 ЕУ 935', car_brand: 'BMW X5' }),
      car({ id: 2, car_number: 'У 465 КУ 423', car_brand: 'BMW X5' }),
    ]);
    await search(wrapper, 'у953');
    expect(numbers(wrapper)).toEqual(['У 952 ЕУ 935']);
  });

  it('перепутанный порядок цифр: «359» находит «У 952 ЕУ 935»', async () => {
    const wrapper = mountCars([
      car({ id: 1, car_number: 'У 952 ЕУ 935', car_brand: 'BMW X5' }),
      car({ id: 2, car_number: 'У 465 КУ 423', car_brand: 'BMW X5' }),
    ]);
    await search(wrapper, '359');
    expect(numbers(wrapper)).toEqual(['У 952 ЕУ 935']);
  });

  it('опечатка в марке: «мерсдес» находит «Мерседес»', async () => {
    const wrapper = mountCars([
      car({ id: 1, car_number: 'У 952 ЕУ 935', car_brand: 'Мерседес' }),
      car({ id: 2, car_number: 'М 234 ОО 123', car_brand: 'Шкода' }),
    ]);
    await search(wrapper, 'мерсдес');
    expect(numbers(wrapper)).toEqual(['У 952 ЕУ 935']);
  });

  it('пустой запрос возвращает весь список', async () => {
    const wrapper = mountCars(fleet);
    await search(wrapper, 'рампа');
    await search(wrapper, '');
    expect(numbers(wrapper)).toHaveLength(3);
    expect(wrapper.find('[data-testid="attachment-elements-total"]').text()).toBe('Всего: 3');
  });

  it('ничего не нашлось — понятное сообщение вместо пустой таблицы', async () => {
    const wrapper = mountCars(fleet);
    await search(wrapper, 'камаз');
    expect(wrapper.find('[data-testid="attachment-elements"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="attachment-elements-nothing-found"]').text()).toContain('камаз');
  });

  it('сотрудники ищутся по ФИО и должности', async () => {
    const wrapper = mountEmployees([
      employee({ id: 1, last_name: 'Иванов', first_name: 'Иван', position: 'Водитель' }),
      employee({ id: 2, last_name: 'Сидоров', first_name: 'Алексей', middle_name: 'Сергеевич', position: 'Инженер' }),
    ]);
    await search(wrapper, 'сидор');
    expect(numbers(wrapper)).toEqual(['Сидоров Алексей Сергеевич']);
    await search(wrapper, 'водитель');
    expect(numbers(wrapper)).toEqual(['Иванов Иван Иванович']);
  });

  it('ТМЦ ищутся по наименованию, счётчик единиц считает только найденное', async () => {
    const wrapper = mountItems([
      { id: 1, name: 'Ноутбук Lenovo', count: 2 },
      { id: 2, name: 'Стеллаж металлический', count: 12 },
    ]);
    await search(wrapper, 'ноут');
    expect(numbers(wrapper)).toEqual(['Ноутбук Lenovo']);
    expect(wrapper.find('[data-testid="attachment-elements-total"]').text()).toContain('Всего позиций: 1, единиц: 2');
  });

  it('счётчик ЧС в футере считает только найденные строки', async () => {
    const wrapper = mountCars([
      car({ id: 1, car_number: 'К 050 УА 902', blacklist_similar: flag() }),
      car({ id: 2, car_number: 'М 234 ОО 123', blacklist_similar: flag() }),
    ]);
    expect(wrapper.find('[data-testid="attachment-flagged-summary"]').text()).toBe('2 похоже на ЧС');
    await search(wrapper, 'к050');
    expect(wrapper.find('[data-testid="attachment-flagged-summary"]').text()).toBe('1 похоже на ЧС');
  });
});

describe('ApplicationAttachmentDetail — кнопка "Убрать" элемента из заявки', () => {
  // Меню действий строки телепортируется в body - стабим Teleport, иначе find его не видит.
  function mountCarsWith(cars, props = {}) {
    return mount(ApplicationAttachmentDetail, {
      props: {
        attachment: { id: 1, attachment_type: 'cars', attachment_display_name: 'Машины' },
        cars,
        ...props,
      },
      global: { stubs: { Teleport: true } },
    });
  }

  it('canRemove=true: у непомеченной строки отдельная кнопка «Убрать»', () => {
    const wrapper = mountCarsWith([car()], { canRemove: true });
    expect(wrapper.find('[data-testid="element-remove-btn"]').exists()).toBe(true);
  });

  it('на помеченной строке отдельной кнопки нет - «Убрать» лежит в меню под стрелкой', async () => {
    const wrapper = mountCarsWith([car({ blacklist_similar: flag() })], { canOverride: true, canRemove: true });

    expect(wrapper.find('[data-testid="element-remove-btn"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="row-actions-menu"]').exists()).toBe(false);

    await wrapper.find('[data-testid="row-actions-toggle"]').trigger('click');
    expect(wrapper.find('[data-testid="row-actions-menu"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="row-action-remove"]').exists()).toBe(true);
  });

  it('пункт «Убрать из заявки» эмитит remove-element и закрывает меню', async () => {
    const wrapper = mountCarsWith([car({ id: 9, car_number: 'А123ВС', blacklist_similar: flag() })], { canOverride: true, canRemove: true });
    await wrapper.find('[data-testid="row-actions-toggle"]').trigger('click');
    await wrapper.find('[data-testid="row-action-remove"]').trigger('click');

    const removed = wrapper.emitted('remove-element');
    expect(removed).toHaveLength(1);
    expect(removed[0][0].id).toBe(9);
    expect(wrapper.find('[data-testid="row-actions-menu"]').exists()).toBe(false);
    expect(wrapper.emitted('open-vehicle')).toBeUndefined();
  });

  it('canRemove=false (дефолт): кнопки нет', () => {
    const wrapper = mountCarsWith([car({ blacklist_similar: flag() })]);
    expect(wrapper.find('[data-testid="element-remove-btn"]').exists()).toBe(false);
  });

  it('клик по «Убрать» на непомеченной строке эмитит remove-element, но не открывает карточку', async () => {
    const wrapper = mountCarsWith([car({ id: 7, car_number: 'А123ВС' })], { canRemove: true });
    await wrapper.find('[data-testid="element-remove-btn"]').trigger('click');

    const removed = wrapper.emitted('remove-element');
    expect(removed).toHaveLength(1);
    expect(removed[0][0].id).toBe(7);
    expect(removed[0][0].label).toContain('А123ВС');
    expect(wrapper.emitted('open-vehicle')).toBeUndefined();
  });
});

describe('ApplicationAttachmentDetail — строка, попавшая в чёрный список после подачи', () => {
  it('is_blacklisted: строка перечёркнута, но остаётся в списке', () => {
    const wrapper = mount(ApplicationAttachmentDetail, {
      props: {
        attachment: { id: 1, attachment_type: 'cars', attachment_display_name: 'Машины' },
        cars: [car({ id: 3, car_number: 'Ч 001 СС 777', is_blacklisted: true })],
      },
    });

    const rows = wrapper.findAll('[data-testid="attachment-element-row"]');
    expect(rows).toHaveLength(1);
    expect(rows[0].classes()).toContain('el-row--blacklisted');
    expect(rows[0].text()).toContain('Ч 001 СС 777');
  });

  it('чистая строка не перечёркивается', () => {
    const wrapper = mount(ApplicationAttachmentDetail, {
      props: {
        attachment: { id: 1, attachment_type: 'cars', attachment_display_name: 'Машины' },
        cars: [car({ id: 4 })],
      },
    });

    expect(wrapper.find('[data-testid="attachment-element-row"]').classes()).not.toContain('el-row--blacklisted');
  });
});

describe('ApplicationAttachmentDetail — обрезка длинного значения в чипе', () => {
  it('единственное место разгрузки лежит в отдельном элементе под многоточие и помечено chip--solo', () => {
    const wrapper = mount(ApplicationAttachmentDetail, {
      props: {
        attachment: { id: 1, attachment_type: 'cars', attachment_display_name: 'Машины' },
        cars: [car({ unload_places: [{ id: 1, name: 'Дебаркадер №1' }] })],
      },
      global: { stubs: { Teleport: true } },
    });

    const chip = wrapper.find('[data-testid="attachment-chip"]');
    expect(chip.exists()).toBe(true);
    // Многоточие рисует CSS, а проверить в jsdom можно структуру: текст обязан лежать
    // в своём элементе - на самом чипе (inline-flex) text-overflow не работает.
    expect(chip.classes()).toContain('chip--solo');
    expect(chip.find('.chip__text').text()).toBe('Дебаркадер №1');
    // Полное название остаётся в подсказке.
    expect(chip.attributes('data-hint')).toBe('Дебаркадер №1');
  });

  it('несколько мест схлопываются в счётчик, а не обрезаются', () => {
    const wrapper = mount(ApplicationAttachmentDetail, {
      props: {
        attachment: { id: 1, attachment_type: 'cars', attachment_display_name: 'Машины' },
        cars: [car({
          unload_places: [
            { id: 1, name: 'Дебаркадер №1' },
            { id: 2, name: 'Дебаркадер №2' },
            { id: 3, name: 'Дебаркадер №3' },
          ],
        })],
      },
      global: { stubs: { Teleport: true } },
    });

    const solo = wrapper.findAll('[data-testid="attachment-chip"]').filter((c) => c.classes().includes('chip--solo'));
    expect(solo).toHaveLength(0);
  });
});

describe('ApplicationAttachmentDetail — куда раскрывается меню действий строки', () => {
  const VIEWPORT = { width: 1200, height: 800 };
  const MENU_WIDTH = 170;
  const MARGIN = 8;
  const GAP = 4;

  function setViewport({ width, height }) {
    Object.defineProperty(window, 'innerWidth', { value: width, writable: true, configurable: true });
    Object.defineProperty(window, 'innerHeight', { value: height, writable: true, configurable: true });
  }

  function mountFlagged() {
    return mount(ApplicationAttachmentDetail, {
      props: {
        attachment: { id: 1, attachment_type: 'cars', attachment_display_name: 'Машины' },
        cars: [car({ blacklist_similar: flag() })],
        canOverride: true,
        canRemove: true,
      },
      global: { stubs: { Teleport: true } },
    });
  }

  /** Кнопка в jsdom не имеет размеров - подставляем те, что нужны расчёту. */
  function openMenuAt(wrapper, rect) {
    const toggle = wrapper.find('[data-testid="row-actions-toggle"]');
    toggle.element.getBoundingClientRect = () => rect;
    return toggle.trigger('click');
  }

  beforeEach(() => setViewport(VIEWPORT));

  it('снизу есть место - меню встаёт под кнопкой', async () => {
    const wrapper = mountFlagged();
    await openMenuAt(wrapper, { top: 200, bottom: 220, right: 1000 });

    expect(wrapper.vm.rowMenuOpenUp).toBe(false);
    expect(wrapper.vm.rowMenuStyle.top).toBe(`${220 + GAP}px`);
    expect(wrapper.vm.rowMenuStyle.bottom).toBe('auto');
  });

  it('строка у нижнего края - меню раскрывается вверх, а не уходит под край карточки', async () => {
    const wrapper = mountFlagged();
    await openMenuAt(wrapper, { top: 760, bottom: 780, right: 1000 });

    expect(wrapper.vm.rowMenuOpenUp).toBe(true);
    expect(wrapper.vm.rowMenuStyle.bottom).toBe(`${VIEWPORT.height - 760 + GAP}px`);
    // top:'auto' обязателен - иначе обе координаты заданы и меню растягивается.
    expect(wrapper.vm.rowMenuStyle.top).toBe('auto');
  });

  it('кнопка у правого края - меню прижато к полю, а не вылезает за окно', async () => {
    const wrapper = mountFlagged();
    await openMenuAt(wrapper, { top: 200, bottom: 220, right: VIEWPORT.width - 1 });

    expect(wrapper.vm.rowMenuStyle.right).toBe(`${MARGIN}px`);
  });

  it('кнопка у левого края - левый край меню остаётся в окне', async () => {
    const wrapper = mountFlagged();
    await openMenuAt(wrapper, { top: 200, bottom: 220, right: 100 });

    const right = Number.parseInt(wrapper.vm.rowMenuStyle.right, 10);
    expect(VIEWPORT.width - right - MENU_WIDTH).toBeGreaterThanOrEqual(MARGIN);
  });

  it('прокрутка списка двигает меню за строкой, а не оставляет висеть', async () => {
    const wrapper = mountFlagged();
    const toggle = wrapper.find('[data-testid="row-actions-toggle"]');
    toggle.element.getBoundingClientRect = () => ({ top: 200, bottom: 220, right: 1000 });
    await toggle.trigger('click');
    expect(wrapper.vm.rowMenuStyle.top).toBe(`${220 + GAP}px`);

    toggle.element.getBoundingClientRect = () => ({ top: 100, bottom: 120, right: 1000 });
    window.dispatchEvent(new Event('scroll'));
    expect(wrapper.vm.rowMenuStyle.top).toBe(`${120 + GAP}px`);
  });
});
