/** The API row and the detail/pass controls share one normalized fact record. */
export function normalizeFactTableRows(factCars, organizationsMap, passageFields) {
  const nameToIdMap = {};
  Object.keys(organizationsMap).forEach(id => {
    nameToIdMap[organizationsMap[id]] = id;
  });
  return factCars.map(car => {
    const orgName = car.organization || '';
    const orgId = nameToIdMap[orgName] || car.organization_id;
    return {
      id: car.id,
      car_number: car.car_number || 'по факту',
      car_brand: car.car_brand || '',
      organization_id: orgId,
      organization_name: orgName || 'Не указана',
      company: car.company || null,
      company_id: car.company_id,
      unload_place: car.unload_place || '-',
      unload_place_ids: car.unload_place_ids || [],
      entry_date_to: car.entry_date_to || '',
      entry_time_from: car.entry_time_from || '',
      entry_time_to: car.entry_time_to || '',
      status: 'В работе',
      entry_checked: false,
      exit_checked: false,
      applicationId: car.application_id,
      plateNumber: car.car_number,
      mark: car.car_brand,
      formatId: null,
      unloadPlaces: car.unload_place_ids || [],
      ...passageFields(car)
    };
  });
}
