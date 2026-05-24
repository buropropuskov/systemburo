<template>
  <div
    class="xlsx-viewer"
    :class="{ loading }"
  >
    <div
      v-if="loading"
      class="xv-loading"
    >
      <span>Загрузка превью...</span>
    </div>
    <div
      v-else-if="error"
      class="xv-error"
    >
      {{ error }}
    </div>
    <div
      v-else-if="sheets.length"
      class="xv-content"
    >
      <div
        v-if="sheets.length > 1"
        class="xv-tabs"
      >
        <button
          v-for="(sheet, idx) in sheets"
          :key="idx"
          class="xv-tab"
          :class="{ active: activeSheet === idx }"
          @click="activeSheet = idx"
        >
          {{ sheet.name }}
        </button>
      </div>
      <div class="xv-table-wrap">
        <table class="xv-table">
          <thead>
            <tr>
              <th class="xv-corner" />
              <th
                v-for="col in currentSheet.cols"
                :key="col"
                class="xv-col-header"
              >
                {{ col }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in currentSheet.rows"
              :key="row.num"
            >
              <td class="xv-row-header">
                {{ row.num }}
              </td>
              <td
                v-for="cell in row.cells"
                :key="cell.ref"
                :data-cell-ref="cell.ref"
                class="xv-cell"
                :class="{
                  selected: selectedCell === cell.ref,
                  mapped: mappedCells.has(cell.ref),
                  'has-value': cell.value,
                }"
                :style="cell.style"
                :colspan="cell.colspan || 1"
                :rowspan="cell.rowspan || 1"
                @click="onCellClick(cell)"
              >
                <span class="xv-cell-text">{{ cell.display }}</span>
                <span
                  v-if="mappedCells.has(cell.ref)"
                  class="xv-mapped-badge"
                >
                  {{ mappedCells.get(cell.ref) }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <div
      v-else
      class="xv-empty"
    >
      Файл не загружен
    </div>
  </div>
</template>

<script>
import ExcelJS from 'exceljs';

export default {
  name: 'XlsxViewer',
  props: {
    fileBuffer: { type: ArrayBuffer, default: null },
    mappings: { type: Array, default: () => [] },
    selectedCell: { type: String, default: '' },
  },
  emits: ['cell-click'],
  data() {
    return {
      sheets: [],
      activeSheet: 0,
      loading: false,
      error: '',
    };
  },
  computed: {
    currentSheet() {
      return this.sheets[this.activeSheet] || { cols: [], rows: [] };
    },
    mappedCells() {
      const map = new Map();
      for (const m of this.mappings) {
        if (m.cell_ref) {
          const label = m.fieldLabel || m.field_path || '';
          map.set(m.cell_ref.toUpperCase(), label);
        }
      }
      return map;
    },
  },
  watch: {
    fileBuffer: {
      immediate: true,
      handler(buf) {
        if (buf) this.parseWorkbook(buf);
        else this.sheets = [];
      },
    },
  },
  methods: {
    async parseWorkbook(buffer) {
      this.loading = true;
      this.error = '';
      try {
        const workbook = new ExcelJS.Workbook();
        await workbook.xlsx.load(buffer);
        this.sheets = workbook.worksheets.map(ws => this.parseSheet(ws));
        this.activeSheet = 0;
      } catch (e) {
        this.error = 'Не удалось прочитать файл: ' + (e.message || e);
        this.sheets = [];
      } finally {
        this.loading = false;
      }
    },
    parseSheet(ws) {
      const colCount = Math.min(ws.columnCount || 0, 30);
      const rowCount = Math.min(ws.rowCount || 0, 100);
      const cols = [];
      for (let c = 1; c <= colCount; c++) {
        cols.push(this.colLetter(c));
      }
      const rows = [];
      for (let r = 1; r <= rowCount; r++) {
        const row = ws.getRow(r);
        const cells = [];
        for (let c = 1; c <= colCount; c++) {
          const cell = row.getCell(c);
          const ref = this.colLetter(c) + r;
          cells.push({
            ref,
            value: cell.value,
            display: this.formatCellValue(cell),
            style: this.extractStyle(cell),
            colspan: 1,
            rowspan: 1,
          });
        }
        rows.push({ num: r, cells });
      }
      return { name: ws.name, cols, rows };
    },
    colLetter(num) {
      let s = '';
      while (num > 0) {
        num--;
        s = String.fromCharCode(65 + (num % 26)) + s;
        num = Math.floor(num / 26);
      }
      return s;
    },
    formatCellValue(cell) {
      if (cell.value === null || cell.value === undefined) return '';
      if (typeof cell.value === 'object') {
        if (cell.value.richText) {
          return cell.value.richText.map(r => r.text).join('');
        }
        if (cell.value.text) return cell.value.text;
        if (cell.value.result !== undefined) return String(cell.value.result);
        return '';
      }
      return String(cell.value);
    },
    extractStyle(cell) {
      const style = {};
      if (cell.font) {
        if (cell.font.bold) style.fontWeight = 'bold';
        if (cell.font.italic) style.fontStyle = 'italic';
        if (cell.font.size) style.fontSize = Math.min(cell.font.size, 14) + 'px';
        if (cell.font.color && cell.font.color.argb) {
          style.color = '#' + cell.font.color.argb.slice(2);
        }
      }
      if (cell.fill && cell.fill.fgColor && cell.fill.fgColor.argb) {
        const hex = '#' + cell.fill.fgColor.argb.slice(2);
        if (hex !== '#000000') style.backgroundColor = hex;
      }
      if (cell.alignment) {
        if (cell.alignment.horizontal) style.textAlign = cell.alignment.horizontal;
      }
      return style;
    },
    onCellClick(cell) {
      this.$emit('cell-click', cell.ref);
    },
  },
};
</script>

<style scoped>
.xlsx-viewer {
  border: 1px solid var(--color-border);
  border-radius: 8px;
  overflow: hidden;
  background: #fff;
  min-height: 200px;
  display: flex;
  flex-direction: column;
}

.xlsx-viewer.loading {
  opacity: 0.7;
}

.xv-loading,
.xv-error,
.xv-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  color: #888;
  font-size: 13px;
}

.xv-error {
  color: #d73a3a;
}

.xv-content {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.xv-tabs {
  display: flex;
  gap: 2px;
  padding: 4px 8px 0;
  background: #f5f5f5;
  border-bottom: 1px solid var(--color-border);
}

.xv-tab {
  padding: 4px 12px;
  border: 1px solid var(--color-border);
  border-bottom: none;
  border-radius: 4px 4px 0 0;
  background: #fff;
  font-size: 12px;
  cursor: pointer;
}

.xv-tab.active {
  background: var(--color-primary);
  color: #fff;
  border-color: var(--color-primary);
}

.xv-table-wrap {
  overflow: auto;
  flex: 1;
}

.xv-table {
  border-collapse: collapse;
  font-size: 11px;
  min-width: 100%;
  table-layout: auto;
}

.xv-table th,
.xv-table td {
  border: 1px solid #ddd;
  padding: 2px 4px;
  white-space: nowrap;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.xv-corner {
  width: 32px;
  min-width: 32px;
  background: #f0f0f0;
}

.xv-col-header {
  background: #f0f0f0;
  text-align: center;
  font-weight: 600;
  font-size: 10px;
  color: #666;
  position: sticky;
  top: 0;
  z-index: 2;
}

.xv-row-header {
  background: #f0f0f0;
  text-align: center;
  font-weight: 600;
  font-size: 10px;
  color: #666;
  position: sticky;
  left: 0;
  z-index: 1;
}

.xv-cell {
  cursor: pointer;
  position: relative;
  transition: background-color 0.1s;
  min-width: 40px;
}

.xv-cell:hover {
  background-color: #e8f4fd !important;
  outline: 2px solid var(--color-primary);
  outline-offset: -2px;
}

.xv-cell.selected {
  background-color: #d0e8ff !important;
  outline: 2px solid var(--color-primary);
  outline-offset: -2px;
}

.xv-cell.mapped {
  background-color: #e8f8e8 !important;
}

.xv-cell.mapped:hover {
  background-color: #d0f0d0 !important;
}

.xv-cell-text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
}

.xv-mapped-badge {
  position: absolute;
  top: 0;
  right: 0;
  background: var(--color-primary);
  color: #fff;
  font-size: 8px;
  padding: 1px 3px;
  border-radius: 0 0 0 4px;
  max-width: 60px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
