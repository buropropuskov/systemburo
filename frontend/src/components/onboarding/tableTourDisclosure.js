import { toRef } from 'vue';
import { useTourDisclosure } from './tourDisclosure';

/** Use the same responsive controls and permissions as the table itself. */
export function useTableTourDisclosure(table) {
  useTourDisclosure({
    requested: () => table.onboardingStore.revealOpen === 'table-actions',
    enabled: () => table.isNarrow && table.hasSheetActions,
    open: toRef(table, 'showActionsSheet'),
  });
  useTourDisclosure({
    requested: () => table.onboardingStore.revealOpen === 'pass-report',
    enabled: () => table.canReport,
    open: toRef(table, 'showPassReport'),
  });
}
