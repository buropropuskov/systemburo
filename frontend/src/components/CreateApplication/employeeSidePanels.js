import { useDeletionsStore } from '@/stores/deletions';

export default {
  methods: {
        showTableDetails(tableId) {
            const tableData = this.allTables.find(t => (t.table && t.table.id === tableId) || t.id === tableId);
            if (!tableData) {
                useDeletionsStore().notify({ bold: 'Информация о месте прохода недоступна', type: 'error' });
                return;
            }
            this.selectedTable = {
                table: tableData.table || tableData,
                time_slots: tableData.time_slots || [],
                photos: tableData.photos || [],
                current_status: tableData.current_status || 'closed'
            };
            if (this.shiftTimer) clearTimeout(this.shiftTimer);
            this.isMainShifted = true;
            this.shiftTimer = setTimeout(() => {
                if (this.show && this.selectedTable) this.showPlaceModal = true;
                this.shiftTimer = null;
            }, 300);
        },

        closeTableDetails() {
            if (this.shiftTimer) { clearTimeout(this.shiftTimer); this.shiftTimer = null; }
            this.showPlaceModal = false;
        },

        onPlaceLeave() {
            if (this.showPlaceModal || this.shiftTimer) return;
            this.isMainShifted = false;
            this.selectedTable = null;
        },

  },
};
