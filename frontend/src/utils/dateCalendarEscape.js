import { markEscapeHandled } from './modalStack';

// Vue привязывает метод к компоненту. Перехват происходит до обработчиков окон.
export function handleDateCalendarEscape(event) {
  if (event.key !== 'Escape') return;
  if (!this.showStartDatepicker && !this.showEndDatepicker && !this.showSingleDatepicker) return;
  markEscapeHandled(event);
  event.preventDefault();
  event.stopPropagation();
  this.closeDatepicker();
}
