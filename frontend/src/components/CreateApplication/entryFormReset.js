/**
 * Возврат форм вложения к виду новой записи после отмены правки.
 *
 * Отдельным модулем, а не методами компонентов: script-блоки VehicleForm и
 * EmployeeForm давно за порогом гейта размеров и расти им нельзя.
 *
 * Раньше это делал родитель, меняя :key формы, то есть пересоздавая компонент. Ремаунт
 * заодно заново тянул справочники, повторял тост про автовыбор места разгрузки и дёргал
 * страницу по высоте, поэтому состояние восстанавливаем по уже загруженным данным.
 */

/**
 * Машина: дефолтный формат номера, места разгрузки заявки (либо автовыбор по
 * организации), закрытые дропдауны, снятые отложенные проверки.
 */
export function resetVehicleFormState(vm) {
    vm.clearVehicleForm();

    const defaultFormat = vm.availableFormats.find((item) => item.format.is_default);
    vm.selectedFormat = defaultFormat || vm.availableFormats[0] || null;
    vm.initializeNumberParts();

    vm.selectedUnloadingPlaces = vm.applicationUnloadPlaces.length > 0
        ? [...vm.applicationUnloadPlaces]
        : vm.activeAttachedIds(vm.attachedUnloadingPlaces);

    vm.isFormatDropdownOpen = false;
    vm.isMarkDropdownOpen = false;
    vm.markSearch = '';
    vm.showAllExistingCars = false;

    // Снимают отложенные проверки предыдущей машины и гасят баннеры.
    vm.checkVehicleActive();
    vm.checkBlacklist();
}

/** Сотрудник: дефолтное гражданство, пустые поля, погашенный баннер ЧС. */
export function resetEmployeeFormState(vm) {
    vm.clearEmployeeForm();

    const defaultCitizenship = vm.availableCitizenships.find((item) => item.is_default);
    vm.selectedCitizenship = defaultCitizenship || vm.availableCitizenships[0] || null;

    vm.isCitizenshipDropdownOpen = false;
    vm.isPermissionDropdownOpen = false;

    // Снимает отложенную проверку ЧС предыдущего человека и гасит баннер.
    vm.checkBlacklist();
}
