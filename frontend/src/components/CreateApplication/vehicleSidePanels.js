import { useDeletionsStore } from '@/stores/deletions';

export default {
  methods: {
    onSidePanelLeave() { this.onPlaceLeave(); this.onTableLeave(); },
        showUnloadPlaceDetails(placeId) {
            const place = this.allUnloadingPlaces.find(p => p.id === placeId);
            if (!place) return;

            // Взаимоисключение с drill-down таблицы: слот place-modal-container один,
            // поэтому гасим подмодалку таблицы, если открыта.
            if (this.tableShiftTimer) {
                clearTimeout(this.tableShiftTimer);
                this.tableShiftTimer = null;
            }
            this.showTableModal = false;
            this.selectedTable = null;

            this.selectedSidePanel = 'place';
            this.selectedUnloadPlace = place;

            if (this.shiftTimer) {
                clearTimeout(this.shiftTimer);
                this.shiftTimer = null;
            }

            this.isMainShifted = true;

            this.shiftTimer = setTimeout(() => {
                if (this.show && this.selectedSidePanel === 'place') this.showPlaceModal = true;
                this.shiftTimer = null;
            }, 300);
        },

        closeUnloadPlaceDetails() {
            if (this.selectedSidePanel === 'place') this.selectedSidePanel = null;
            if (this.shiftTimer) {
                clearTimeout(this.shiftTimer);
                this.shiftTimer = null;
            }
            this.showPlaceModal = false;
        },

        onPlaceLeave() {
            if (this.selectedSidePanel === 'place') return;
            this.isMainShifted = !!this.selectedSidePanel;
            this.selectedUnloadPlace = null;
        },

        showTableDetails(tableId) {
            const tableData = this.allTables.find(t => (t.table && t.table.id === tableId) || t.id === tableId);
            if (!tableData) {
                useDeletionsStore().notify({ bold: 'Информация о месте проезда недоступна', type: 'error' });
                return;
            }

            // Взаимоисключение с drill-down места разгрузки (общий слот).
            if (this.shiftTimer) {
                clearTimeout(this.shiftTimer);
                this.shiftTimer = null;
            }
            this.showPlaceModal = false;
            this.selectedUnloadPlace = null;

            this.selectedSidePanel = 'table';
            this.selectedTable = {
                table: tableData.table || tableData,
                time_slots: tableData.time_slots || [],
                photos: tableData.photos || [],
                current_status: tableData.current_status || 'closed'
            };

            if (this.tableShiftTimer) {
                clearTimeout(this.tableShiftTimer);
                this.tableShiftTimer = null;
            }

            this.isMainShifted = true;

            this.tableShiftTimer = setTimeout(() => {
                if (this.show && this.selectedSidePanel === 'table') this.showTableModal = true;
                this.tableShiftTimer = null;
            }, 300);
        },

        closeTableDetails() {
            if (this.selectedSidePanel === 'table') this.selectedSidePanel = null;
            if (this.tableShiftTimer) {
                clearTimeout(this.tableShiftTimer);
                this.tableShiftTimer = null;
            }
            this.showTableModal = false;
        },

        onTableLeave() {
            if (this.selectedSidePanel === 'table') return;
            this.isMainShifted = !!this.selectedSidePanel;
            this.selectedTable = null;
        },

  },
};
