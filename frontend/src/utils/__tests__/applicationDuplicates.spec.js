import { describe, it, expect } from 'vitest';
import {
    isSameEmployee,
    isSameVehicle,
    findDuplicateEmployee,
    findDuplicateVehicle,
    findFirstDuplicate,
    employeeFromCatalog,
    vehicleFromCatalog,
} from '@/utils/applicationDuplicates';

// Одного человека/машину нельзя положить в список вложения дважды. Ключ совпадения общий
// для ручного ввода, выбора существующих и блокировки строк в модалках выбора.

describe('isSameEmployee', () => {
    it('совпадение по паспорту без учёта регистра и пробелов', () => {
        expect(isSameEmployee({ passportSeriesNumber: '4510 123456' }, { passportSeriesNumber: '4510123456' })).toBe(true);
        expect(isSameEmployee({ passportSeriesNumber: ' ab 123456 ' }, { passportSeriesNumber: 'AB123456' })).toBe(true);
        expect(isSameEmployee({ passportSeriesNumber: '4510 123456' }, { passportSeriesNumber: '4510 654321' })).toBe(false);
    });

    it('разные паспорта у полных тёзок - не дубль', () => {
        const a = { lastName: 'Иванов', firstName: 'Иван', middleName: 'Иванович', passportSeriesNumber: '4510 111111' };
        const b = { lastName: 'Иванов', firstName: 'Иван', middleName: 'Иванович', passportSeriesNumber: '4510 222222' };
        expect(isSameEmployee(a, b)).toBe(false);
    });

    it('паспорт не заполнен (скрыт шаблоном) - падаем на ФИО', () => {
        const a = { lastName: ' Иванов ', firstName: 'Иван', middleName: 'Иванович' };
        const b = { lastName: 'иванов', firstName: 'ИВАН', middleName: 'Иванович' };
        expect(isSameEmployee(a, b)).toBe(true);
        expect(isSameEmployee(a, { lastName: 'Иванов', firstName: 'Пётр', middleName: 'Иванович' })).toBe(false);
    });

    it('паспорт есть только у одного - сравниваем ФИО, а не считаем разными', () => {
        const added = { lastName: 'Иванов', firstName: 'Иван', middleName: '', passportSeriesNumber: '4510 111111' };
        const candidate = { lastName: 'Иванов', firstName: 'Иван', middleName: '' };
        expect(isSameEmployee(added, candidate)).toBe(true);
    });

    it('пустые паспорт и ФИО не склеивают разные записи', () => {
        expect(isSameEmployee({}, {})).toBe(false);
        expect(isSameEmployee({ passportSeriesNumber: '' }, { passportSeriesNumber: '' })).toBe(false);
    });

    it('одна и та же запись каталога - дубль даже при разном заполнении полей', () => {
        expect(isSameEmployee({ existingEmployeeId: 5 }, { existingEmployeeId: 5 })).toBe(true);
        expect(isSameEmployee(
            { existingEmployeeId: 5, lastName: 'Иванов', firstName: 'Иван', passportSeriesNumber: '4510 111111' },
            { existingEmployeeId: 7, lastName: 'Петров', firstName: 'Пётр', passportSeriesNumber: '4510 222222' },
        )).toBe(false);
    });

    it('существующий из каталога против введённого руками - по паспорту', () => {
        const manual = { isExisting: false, lastName: 'Петров', firstName: 'Пётр', passportSeriesNumber: '4510 333333' };
        const catalog = employeeFromCatalog({ id: 9, last_name: 'Петров', first_name: 'Пётр', middle_name: '', passport_series_number: '4510333333' });
        expect(isSameEmployee(manual, catalog)).toBe(true);
    });
});

describe('isSameVehicle', () => {
    it('совпадение по госномеру без учёта регистра и пробелов', () => {
        expect(isSameVehicle({ plateNumber: 'A777AA 777' }, { plateNumber: 'a777aa777' })).toBe(true);
        expect(isSameVehicle({ plateNumber: 'A777AA 777' }, { plateNumber: 'B111BB 77' })).toBe(false);
    });

    it('номер решает, марка - нет: та же машина с другой маркой всё равно дубль', () => {
        expect(isSameVehicle({ plateNumber: 'A777AA 777', mark: 'BMW' }, { plateNumber: 'A777AA 777', mark: 'Toyota' })).toBe(true);
    });

    it('"По факту" не опознаёт машину - несколько таких строк допустимы', () => {
        expect(isSameVehicle({ plateNumber: 'По факту' }, { plateNumber: 'По факту' })).toBe(false);
        expect(isSameVehicle({ plateNumber: 'По факту', mark: 'BMW' }, { plateNumber: 'По факту', mark: 'BMW' })).toBe(false);
        expect(isSameVehicle({ plateNumber: 'A777AA 777' }, { plateNumber: 'По факту' })).toBe(false);
    });

    it('пустой номер не склеивает записи', () => {
        expect(isSameVehicle({ plateNumber: '' }, { plateNumber: '' })).toBe(false);
        expect(isSameVehicle({}, {})).toBe(false);
    });

    it('одна и та же запись каталога - дубль', () => {
        expect(isSameVehicle({ existingCarId: 3 }, { existingCarId: 3 })).toBe(true);
    });

    it('существующая из каталога против введённой руками - по номеру', () => {
        const manual = { isExisting: false, plateNumber: 'a777aa 777', mark: 'BMW' };
        const catalog = vehicleFromCatalog({ id: 4, number: 'A777AA777', mark: 'Toyota' });
        expect(isSameVehicle(manual, catalog)).toBe(true);
    });
});

describe('findDuplicate*', () => {
    const employees = [
        { id: 1, lastName: 'Иванов', firstName: 'Иван', passportSeriesNumber: '4510 111111' },
        { id: 2, lastName: 'Петров', firstName: 'Пётр', passportSeriesNumber: '4510 222222' },
    ];

    it('возвращает совпавшую строку списка', () => {
        expect(findDuplicateEmployee(employees, { passportSeriesNumber: '4510222222' })).toEqual(employees[1]);
        expect(findDuplicateEmployee(employees, { passportSeriesNumber: '4510 999999' })).toBeNull();
    });

    it('excludeId пропускает саму себя - правка строки не считается дублем', () => {
        expect(findDuplicateEmployee(employees, { passportSeriesNumber: '4510 111111' }, 1)).toBeNull();
        expect(findDuplicateEmployee(employees, { passportSeriesNumber: '4510 111111' }, 2)).toEqual(employees[0]);
    });

    it('правка строки в дубль другой строки ловится', () => {
        expect(findDuplicateEmployee(employees, { passportSeriesNumber: '4510 222222' }, 1)).toEqual(employees[1]);
    });

    it('пустой список и undefined безопасны', () => {
        expect(findDuplicateEmployee(undefined, { passportSeriesNumber: '1' })).toBeNull();
        expect(findDuplicateVehicle([], { plateNumber: 'A777AA 777' })).toBeNull();
    });

    it('машины: excludeId и совпадение по номеру', () => {
        const vehicles = [{ id: 1, plateNumber: 'A777AA 777' }, { id: 2, plateNumber: 'B111BB 77' }];
        expect(findDuplicateVehicle(vehicles, { plateNumber: 'a777aa777' })).toEqual(vehicles[0]);
        expect(findDuplicateVehicle(vehicles, { plateNumber: 'a777aa777' }, 1)).toBeNull();
    });
});

describe('findFirstDuplicate', () => {
    it('возвращает вторую из совпавших строк готового списка', () => {
        const list = [
            { id: 1, plateNumber: 'A777AA 777' },
            { id: 2, plateNumber: 'B111BB 77' },
            { id: 3, plateNumber: 'a777aa777' },
        ];
        expect(findFirstDuplicate(list, isSameVehicle)).toEqual(list[2]);
    });

    it('список без повторов и пустой вход дают null', () => {
        expect(findFirstDuplicate([{ id: 1, plateNumber: 'A777AA 777' }], isSameVehicle)).toBeNull();
        expect(findFirstDuplicate([], isSameVehicle)).toBeNull();
        expect(findFirstDuplicate(undefined, isSameEmployee)).toBeNull();
    });

    it('несколько строк "По факту" повтором не считаются', () => {
        const list = [{ id: 1, plateNumber: 'По факту' }, { id: 2, plateNumber: 'По факту' }];
        expect(findFirstDuplicate(list, isSameVehicle)).toBeNull();
    });
});
