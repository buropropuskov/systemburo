export function singleMomentCalendarPosition(anchor, zoom) {
  const popup = document.querySelector('.datepicker');
  if (!popup) return null;
  const r = anchor.getBoundingClientRect();
  const width = popup.getBoundingClientRect().width / zoom;
  const height = popup.getBoundingClientRect().height / zoom;
  const viewportWidth = window.innerWidth / zoom, viewportHeight = window.innerHeight / zoom;
  const below = r.bottom / zoom + 8;
  return {
    top: `${Math.max(8, Math.min(below + height <= viewportHeight - 8 ? below : r.top / zoom - height - 8, viewportHeight - height - 8))}px`,
    left: `${Math.max(8, Math.min(r.left / zoom, viewportWidth - width - 8))}px`,
  };
}

// Measure before showing so the first visible frame is already inside the viewport.
export function prepareSingleMomentCalendar(vm, anchor, zoom) {
  vm.datepickerStyle = { ...vm.datepickerStyle, visibility: 'hidden' };
  vm.$nextTick(() => {
    if (!vm.showSingleDatepicker) return;
    vm.datepickerStyle = { ...vm.datepickerStyle, ...singleMomentCalendarPosition(anchor, zoom), visibility: 'visible' };
  });
}
