// Canonical and legacy field names are both accepted by existing card callers.
// Keep these computed properties together: blacklist actions use the same
// identity as the label shown in confirmations and result notifications.
export const vehicleDetailsIdentityComputed = {
  vehicleNumber() {
    return (this.vehicle?.plateNumber || this.vehicle?.car_number || '').trim();
  },
  vehicleMark() {
    return (this.vehicle?.mark || this.vehicle?.car_brand || '').trim();
  },
  vehicleLabel() {
    return [this.vehicleNumber, this.vehicleMark].filter(Boolean).join(' ');
  },
  // Добавить в ЧС можно только реальную машину с номером и маркой (не "по факту").
  hasVehicleIdentity() {
    const n = this.vehicleNumber.toLowerCase();
    return !!this.vehicleNumber && !!this.vehicleMark && n !== 'по факту';
  },
};
