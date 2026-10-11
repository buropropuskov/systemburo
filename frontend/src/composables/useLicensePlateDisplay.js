import { ref, onMounted } from 'vue';
import { apiRequest } from '@/api/client';
import { formatNumberForDisplay } from '@/composables/useNumberFormat';

// A failed directory read preserves the stored number verbatim.
export function useLicensePlateDisplay() {
  const licensePlateFormats = ref([]);
  onMounted(async () => {
    try {
      const response = await apiRequest('/license-plate-formats');
      if (response?.ok) {
        const rows = await response.json();
        if (Array.isArray(rows)) licensePlateFormats.value = rows;
      }
    } catch { /* The card/history remains usable without number formatting. */ }
  });
  return { licensePlateFormats, displayPlate: value => formatNumberForDisplay(value || '', licensePlateFormats.value) };
}
