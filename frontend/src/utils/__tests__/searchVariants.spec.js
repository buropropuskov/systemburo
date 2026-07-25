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

  it('номер, набранный слитно с буквой: «у953» находит «У 952 ЕУ 935»', () => {
    // «у» и «952» - разные слова, поэтому сравниваем и склейки соседних
    expect(find('У 952 ЕУ 935 BMW X5', 'у953')).toBe(true);
    expect(find('У 952 ЕУ 935 BMW X5', 'еу935')).toBe(true);
  });

  it('склейка не превращается в поиск по произвольному куску строки', () => {
    // на «У 465 КУ 423» кусок «у42» отличается от «942» одной заменой,
    // но словом или склейкой слов он не является
    expect(find('У 465 КУ 423 BMW X5', '942')).toBe(false);
    expect(find('У 465 КУ 423 BMW X5', 'у953')).toBe(false);
  });

  it('те же цифры в другом порядке: «359» находит «У 952 ЕУ 935»', () => {
    // номер запоминают набором цифр, порядок путают
    expect(find('У 952 ЕУ 935 BMW X5', '359')).toBe(true);
    expect(find('У 952 ЕУ 935 BMW X5', '529')).toBe(true);
  });

  it('перестановка соседних символов стоит одну правку, а не две', () => {
    expect(find('У 952 ЕУ 935 BMW X5', '395')).toBe(true);
    expect(find('Дебаркадер №1', 'дебарквдер')).toBe(true);
  });

  it('чужие цифры не притягиваются перестановкой', () => {
    // {5,3,2} - цифры другой машины, состав не совпадает
    expect(find('У 952 ЕУ 935 BMW X5', '532')).toBe(false);
    expect(find('Н 859 УТ 532 Газель', '532')).toBe(true);
  });

  it('перестановка не применяется к буквам: «ток» не должен находить «кот»', () => {
    expect(find('кот на крыше', 'ток')).toBe(false);
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
