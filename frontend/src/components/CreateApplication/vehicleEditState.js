import { matchNumberToFormat } from '@/composables/useNumberFormat';

// A new edit/cancel invalidates a previous edit still waiting for formats.
export async function restoreEditedVehicleNumber(vm, vehicle, current = () => true) {
    if (!vm.availableFormats.length && vm.formatsReady) await vm.formatsReady;
    if (!current()) return false;
    const known = vehicle.formatId ? vm.availableFormats.find(item => item.format.id === vehicle.formatId) : null;
    const match = known && matchNumberToFormat(vehicle.plateNumber, [known]);
    const guessed = match || matchNumberToFormat(vehicle.plateNumber, vm.availableFormats);
    if (guessed) {
        vm.selectedFormat = match ? known : guessed.format;
        vm.numberParts = guessed.parts;
        return true;
    }
    vm.selectedFormat = null;
    vm.numberParts = [];
    return false;
}

export async function restoreVehicleEdit(vm, vehicle) {
    const epoch = ++vm.vehicleEditEpoch;
    const current = () => vm.vehicleEditEpoch === epoch && !!vm.editingVehicle;
    vm.vehicleEditPending = true;
    try {
        vm.editingVehicle = vehicle;
        vm.selectedExistingCars = [];
        vm.activeCarInfo = null;
        vm.isNumberByFact = vehicle.plateNumber === 'По факту';
        if (!vm.isNumberByFact) {
            await vm.applyEditedVehicleNumber(vehicle, current);
            if (!current()) return;
        }
        vm.isMarkByFact = vehicle.mark === 'По факту';
        vm.selectedMark = vm.isMarkByFact ? '' : vehicle.mark || '';
        vm.selectedMarkId = vm.isMarkByFact ? null : vehicle.markId || vm.marks.find(mark => mark.name === vehicle.mark)?.id || null;
        vm.selectedUnloadingPlaces = vehicle.unloadPlaces || [];
        vm.selectedPassageTables = vehicle.passage_tables || [];
        vm.checkBlacklist();
    } finally {
        if (vm.vehicleEditEpoch === epoch) vm.vehicleEditPending = false;
    }
}
