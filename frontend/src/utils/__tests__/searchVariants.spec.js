import { describe, it, expect } from 'vitest';
import { buildSearchVariants, matchesSearch } from '../searchVariants';

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
});
