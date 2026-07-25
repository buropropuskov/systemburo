import { describe, it, expect } from 'vitest';
import { buildSearchVariants, matchesSearch, matchesSearchFuzzy } from '../searchVariants';

describe('buildSearchVariants', () => {
    it('пустой ввод -> пустой набор', () => {
        expect(buildSearchVariants('')).toEqual([]);
        expect(buildSearchVariants('   ')).toEqual([]);
        expect(buildSearchVariants(null)).toEqual([]);
    });

    it('включает оригинал в нижнем регистре', () => {
        expect(buildSearchVariants('Иванов')).toContain('иванов');
    });

    it('раскладка EN->RU: набрано на латинице вместо кириллицы', () => {
        // "bdfyjd" на физических клавишах = "иванов"
        expect(buildSearchVariants('bdfyjd')).toContain('иванов');
    });

    it('раскладка RU->EN: набрано на кириллице вместо латиницы', () => {
        // "фвьшт" на физических клавишах = "admin"
        expect(buildSearchVariants('фвьшт')).toContain('admin');
    });

    it('фонетический транслит кириллица->латиница', () => {
        expect(buildSearchVariants('иванов')).toContain('ivanov');
    });

    it('фонетический транслит латиница->кириллица', () => {
        expect(buildSearchVariants('ivanov')).toContain('иванов');
    });

    it('добавляет вариант без пробелов', () => {
        expect(buildSearchVariants('а 123 вс')).toContain('а123вс');
    });

    it('варианты уникальны', () => {
        const v = buildSearchVariants('test');
        expect(new Set(v).size).toBe(v.length);
    });
});

describe('matchesSearch', () => {
    const variants = buildSearchVariants('иванов');

    it('пустые варианты -> совпадает всё', () => {
        expect(matchesSearch('что угодно', [])).toBe(true);
        expect(matchesSearch('что угодно', null)).toBe(true);
    });

    it('прямое вхождение', () => {
        expect(matchesSearch('Иванов Иван Иванович', variants)).toBe(true);
    });

    it('находит при вводе в неверной раскладке', () => {
        expect(matchesSearch('Иванов Пётр', buildSearchVariants('bdfyjd'))).toBe(true);
    });

    it('находит транслит', () => {
        expect(matchesSearch('Ivanov Ivan', buildSearchVariants('иванов'))).toBe(true);
    });

    it('номер слитно находит раздельный', () => {
        expect(matchesSearch('А 123 ВС 77', buildSearchVariants('а123вс'))).toBe(true);
    });

    it('номер раздельно находит слитный', () => {
        expect(matchesSearch('А123ВС77', buildSearchVariants('а 123 вс'))).toBe(true);
    });

    it('не матчит несвязанное', () => {
        expect(matchesSearch('Петров Сидор', variants)).toBe(false);
    });

    it('матчит конкатенацию полей (имя код id), как в справочниках/admin-разделах', () => {
        // "fhtylfnjh" на физических клавишах = "арендатор" - роль ищется даже
        // если пользователь не переключил раскладку клавиатуры.
        const haystack = 'Арендатор tenant 1';
        expect(matchesSearch(haystack, buildSearchVariants('fhtylfnjh'))).toBe(true);
    });
});

describe('matchesSearchFuzzy — поиск с опечаткой', () => {
  const plate = 'У 952 ЕУ 935 Мерседес Ворота Черепашки';

  function find(haystack, query) {
    return matchesSearchFuzzy(haystack, buildSearchVariants(query));
  }

  it('опечатка в цифре номера не мешает: 942 находит 952', () => {
    expect(find(plate, '942')).toBe(true);
  });

  it('точное вхождение работает как раньше', () => {
    expect(find(plate, '952')).toBe(true);
    expect(find(plate, 'черепашки')).toBe(true);
  });

  it('опечатка в слове: «мерсдес» находит «Мерседес»', () => {
    expect(find(plate, 'мерсдес')).toBe(true);
  });

  it('пропущенный и лишний символ тоже прощаются', () => {
    expect(find(plate, 'черпашки')).toBe(true);
    expect(find(plate, 'череппашки')).toBe(true);
  });

  it('непохожее не находится', () => {
    expect(find(plate, 'камаз')).toBe(false);
    expect(find(plate, '777')).toBe(false);
  });

  it('запрос короче трёх символов сверяется строго', () => {
    // на двух символах «похожей» оказалась бы почти любая строка
    expect(find(plate, '94')).toBe(false);
    expect(find(plate, '95')).toBe(true);
  });

  it('пустой запрос пропускает всё, как и строгий поиск', () => {
    expect(matchesSearchFuzzy(plate, [])).toBe(true);
  });

  it('чем длиннее запрос, тем больше правок прощается', () => {
    // 0.65 порога: на 9 символах это три правки
    expect(find('Северный въезд', 'сеаерныы')).toBe(true);
    expect(find('Северный въезд', 'абвгдежз')).toBe(false);
  });
});
