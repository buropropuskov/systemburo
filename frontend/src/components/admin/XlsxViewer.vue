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
      <!-- data-theme="light" - светлый островок на область листа: цвета шрифта,
           заливок и рамок приходят из самого xlsx литералами (чёрные рамки,
           тёмный текст), они рассчитаны на белую бумагу. Рамка вьюера и
           закладки листов остаются в выбранной теме. -->
      <div
        class="xv-table-wrap"
        data-theme="light"
      >
        <table
          class="xv-table"
          :style="{ tableLayout: 'fixed', width: currentSheet.tableWidth + 'px' }"
        >
          <colgroup>
            <col class="xv-col-num">
            <col
              v-for="cw in currentSheet.colWidths"
              :key="cw.letter"
              :style="{ width: cw.width + 'px' }"
            >
          </colgroup>
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
              :style="row.height ? { height: row.height + 'px' } : {}"
            >
              <td class="xv-row-header">
                {{ row.num }}
              </td>
              <td
                v-for="cell in row.cells"
                v-show="!cell.hidden"
                :key="cell.ref"
                :data-cell-ref="cell.ref"
                :data-tooltip="cellTooltip(cell.ref)"
                class="xv-cell"
                :class="{
                  selected: selectedCell === cell.ref,
                  mapped: mappedCells.has(cell.ref),
                  'has-value': cell.value,
                }"
                :style="cellStyle(cell)"
                :colspan="cell.colspan || 1"
                :rowspan="cell.rowspan || 1"
                @click="onCellClick(cell)"
                @mouseenter="$emit('cell-hover', cell.ref)"
                @mouseleave="$emit('cell-hover', '')"
              >
                <span class="xv-cell-text">{{ cell.display }}</span>
                <img
                  v-for="img in cell.images"
                  :key="img.src"
                  :src="img.src"
                  class="xv-cell-image"
                  :style="img.style"
                >
                <span
                  v-if="mappedCells.has(cell.ref)"
                  class="xv-mapped-badge"
                  :class="{ 'xv-mapped-badge--multi': mappedCells.get(cell.ref).length > 1 }"
                >
                  <span
                    v-for="(label, i) in mappedCells.get(cell.ref)"
                    :key="i"
                    class="xv-mapped-part"
                  >
                    <span
                      v-if="mappedCells.get(cell.ref).length > 1"
                      class="xv-mapped-order"
                    >{{ i + 1 }}</span>{{ label }}
                  </span>
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

const DEFAULT_COL_WIDTH = 64;
const DEFAULT_ROW_HEIGHT = 20;
const COL_WIDTH_FACTOR = 7.5;
const ROW_HEIGHT_FACTOR = 1.33;
const ROW_NUM_COL_WIDTH = 32;

export default {
  name: 'XlsxViewer',
  props: {
    fileBuffer: { type: ArrayBuffer, default: null },
    mappings: { type: Array, default: () => [] },
    selectedCell: { type: String, default: '' },
    cellColors: { type: Map, default: () => new Map() },
    // Разделитель совмещённых полей: подсказка показывает, как склеится ячейка.
    concatSeparator: { type: String, default: ', ' },
  },
  emits: ['cell-click', 'cell-hover'],
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
      return this.sheets[this.activeSheet] || { cols: [], rows: [], colWidths: [], tableWidth: 0 };
    },
    // Ячейка -> подписи ВСЕХ привязанных к ней полей в том порядке, в котором бланк
    // их склеит. Раньше в Map лежала одна подпись, и у совмещённой ячейки было видно
    // только последнее поле - сколько их и в каком порядке, из превью не читалось.
    mappedCells() {
      const map = new Map();
      for (const m of this.mappings) {
        if (!m.cell_ref) continue;
        const ref = m.cell_ref.toUpperCase();
        const label = m.fieldLabel || m.field_path || '';
        if (!map.has(ref)) map.set(ref, []);
        map.get(ref).push(label);
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
    // Тултип ячейки: адрес, а у совмещённой - сколько полей и как они склеятся.
    // Переиспользуем существующий тултип, свой пузырёк на бейдже обрезала бы ячейка.
    cellTooltip(ref) {
      const labels = this.mappedCells.get(ref) || [];
      if (labels.length < 2) return ref;
      return `${ref}: полей ${labels.length}, склеятся так - ${labels.join(this.concatSeparator)}`;
    },
    async parseWorkbook(buffer) {
      this.loading = true;
      this.error = '';
      try {
        const workbook = new ExcelJS.Workbook();
        await workbook.xlsx.load(buffer);
        this.sheets = workbook.worksheets.map(ws => this.parseSheet(ws, workbook));
        this.activeSheet = 0;
      } catch (e) {
        this.error = 'Не удалось прочитать файл: ' + (e.message || e);
        this.sheets = [];
      } finally {
        this.loading = false;
      }
    },
    parseSheet(ws, workbook) {
      const colCount = Math.min(ws.columnCount || 0, 30);
      const rowCount = Math.min(ws.rowCount || 0, 100);

      const merges = this.parseMerges(ws);
      const imageMap = this.parseImages(ws, workbook);

      const cols = [];
      const colWidths = [];
      for (let c = 1; c <= colCount; c++) {
        const letter = this.colLetter(c);
        cols.push(letter);
        const col = ws.getColumn(c);
        const w = col.width ? Math.round(col.width * COL_WIDTH_FACTOR) : DEFAULT_COL_WIDTH;
        colWidths.push({ letter, width: w });
      }

      const tableWidth = ROW_NUM_COL_WIDTH + colWidths.reduce((sum, cw) => sum + cw.width, 0);

      const rows = [];
      for (let r = 1; r <= rowCount; r++) {
        const row = ws.getRow(r);
        const rowHeight = row.height ? Math.round(row.height * ROW_HEIGHT_FACTOR) : DEFAULT_ROW_HEIGHT;
        const cells = [];
        for (let c = 1; c <= colCount; c++) {
          const cell = row.getCell(c);
          const ref = this.colLetter(c) + r;

          const mergeInfo = merges.get(ref);
          const hidden = mergeInfo && mergeInfo.hidden;
          const colspan = mergeInfo ? mergeInfo.colspan : 1;
          const rowspan = mergeInfo ? mergeInfo.rowspan : 1;

          const cellImages = imageMap.get(ref) || [];

          cells.push({
            ref,
            value: cell.value,
            display: hidden ? '' : this.formatCellValue(cell),
            style: this.extractStyle(cell, mergeInfo),
            colspan,
            rowspan,
            hidden,
            images: cellImages,
          });
        }
        rows.push({ num: r, cells, height: rowHeight });
      }
      return { name: ws.name, cols, rows, colWidths, tableWidth };
    },
    parseMerges(ws) {
      const map = new Map();
      if (!ws.model || !ws.model.merges) return map;
      for (const mergeRange of ws.model.merges) {
        const parts = mergeRange.split(':');
        if (parts.length !== 2) continue;
        const tl = this.parseRef(parts[0]);
        const br = this.parseRef(parts[1]);
        if (!tl || !br) continue;

        const colspan = br.col - tl.col + 1;
        const rowspan = br.row - tl.row + 1;

        const tlRef = parts[0].toUpperCase();
        map.set(tlRef, { colspan, rowspan, hidden: false });

        for (let r = tl.row; r <= br.row; r++) {
          for (let c = tl.col; c <= br.col; c++) {
            const ref = this.colLetter(c) + r;
            if (ref !== tlRef) {
              map.set(ref, { colspan: 1, rowspan: 1, hidden: true });
            }
          }
        }
      }
      return map;
    },
    parseRef(ref) {
      const m = ref.toUpperCase().match(/^([A-Z]+)(\d+)$/);
      if (!m) return null;
      let col = 0;
      for (const ch of m[1]) {
        col = col * 26 + (ch.charCodeAt(0) - 64);
      }
      return { col, row: parseInt(m[2], 10) };
    },
    parseImages(ws, workbook) {
      const map = new Map();
      if (!ws.getImages) return map;
      const wsImages = ws.getImages();
      for (const img of wsImages) {
        const mediaImage = workbook.model.media.find(m => m.index === img.imageId);
        if (!mediaImage || !mediaImage.buffer) continue;

        const ext = mediaImage.extension || 'png';
        const mime = ext === 'jpeg' || ext === 'jpg' ? 'image/jpeg' : `image/${ext}`;
        const b64 = this.bufferToBase64(mediaImage.buffer);
        const src = `data:${mime};base64,${b64}`;

        let cellRef = '';
        const style = {};
        if (img.range) {
          const tl = img.range.tl;
          cellRef = this.colLetter(Math.floor(tl.col) + 1) + (Math.floor(tl.row) + 1);
          if (img.range.ext) {
            style.maxWidth = Math.round(img.range.ext.width / 9525) + 'px';
            style.maxHeight = Math.round(img.range.ext.height / 9525) + 'px';
          }
        }
        if (!cellRef) continue;

        if (!map.has(cellRef)) map.set(cellRef, []);
        map.get(cellRef).push({ src, style });
      }
      return map;
    },
    bufferToBase64(buffer) {
      const bytes = buffer instanceof Uint8Array ? buffer : new Uint8Array(buffer);
      let binary = '';
      for (let i = 0; i < bytes.length; i++) {
        binary += String.fromCharCode(bytes[i]);
      }
      return btoa(binary);
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
    extractStyle(cell, mergeInfo) {
      const style = {};

      if (cell.font) {
        if (cell.font.bold) style.fontWeight = 'bold';
        if (cell.font.italic) style.fontStyle = 'italic';
        if (cell.font.underline) style.textDecoration = 'underline';
        if (cell.font.size) style.fontSize = cell.font.size + 'px';
        if (cell.font.name) style.fontFamily = cell.font.name + ', sans-serif';
        if (cell.font.color && cell.font.color.argb) {
          style.color = '#' + cell.font.color.argb.slice(2);
        }
      }

      if (cell.fill && cell.fill.fgColor && cell.fill.fgColor.argb) {
        const hex = '#' + cell.fill.fgColor.argb.slice(2);
        if (hex.toLowerCase() !== '#000000') style.backgroundColor = hex;
      }

      if (cell.alignment) {
        if (cell.alignment.horizontal) {
          const hmap = { centerContinuous: 'center', distributed: 'justify', fill: 'left' };
          style.textAlign = hmap[cell.alignment.horizontal] || cell.alignment.horizontal;
        }
        if (cell.alignment.vertical) {
          const vmap = { top: 'top', middle: 'middle', bottom: 'bottom' };
          style.verticalAlign = vmap[cell.alignment.vertical] || 'middle';
        }
        if (cell.alignment.wrapText) {
          style.whiteSpace = 'pre-wrap';
          style.wordBreak = 'break-word';
        }
      }

      if (cell.border) {
        const toBorder = (b) => {
          if (!b || !b.style) return '';
          const styleMap = {
            thin: '1px solid',
            medium: '2px solid',
            thick: '3px solid',
            dotted: '1px dotted',
            dashed: '1px dashed',
            double: '3px double',
            hair: '1px solid',
          };
          const bStyle = styleMap[b.style] || '1px solid';
          let color = '#000';
          if (b.color && b.color.argb) {
            color = '#' + b.color.argb.slice(2);
          }
          return `${bStyle} ${color}`;
        };
        if (cell.border.top) style.borderTop = toBorder(cell.border.top);
        if (cell.border.bottom) style.borderBottom = toBorder(cell.border.bottom);
        if (cell.border.left) style.borderLeft = toBorder(cell.border.left);
        if (cell.border.right) style.borderRight = toBorder(cell.border.right);
      }

      if (mergeInfo && !mergeInfo.hidden && (mergeInfo.colspan > 1 || mergeInfo.rowspan > 1)) {
        style.overflow = 'visible';
      }

      return style;
    },
    cellStyle(cell) {
      const base = { ...cell.style };
      const color = this.cellColors.get(cell.ref);
      if (color) {
        base.outline = `2px solid ${color}`;
        base.outlineOffset = '-2px';
      }
      return base;
    },
    onCellClick(cell) {
      if (cell.hidden) return;
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
  background: var(--surface);
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
  color: var(--text-muted);
  font-size: 13px;
}

.xv-error {
  color: var(--danger-text);
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
  background: var(--surface-2);
  border-bottom: 1px solid var(--color-border);
}

.xv-tab {
  padding: 4px 12px;
  border: 1px solid var(--color-border);
  border-bottom: none;
  border-radius: 4px 4px 0 0;
  background: var(--surface);
  font-size: 12px;
  cursor: pointer;
}

.xv-tab.active {
  background: var(--color-primary);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

.xv-table-wrap {
  overflow: auto;
  flex: 1;
  /* Бумага под ячейками: без своей заливки островок не виден - ячейки
     прозрачны, и сквозь них смотрел бы тёмный фон вьюера. */
  background: var(--surface);
}

.xv-col-num {
  width: 32px;
}

.xv-table {
  border-collapse: collapse;
}

.xv-table th,
.xv-table td {
  border: 1px solid var(--border);
  padding: 2px 4px;
}

.xv-corner {
  width: 32px;
  min-width: 32px;
  background: var(--border);
}

.xv-col-header {
  background: var(--border);
  text-align: center;
  font-weight: 600;
  font-size: 10px;
  color: var(--text-muted);
  position: sticky;
  top: 0;
  z-index: 2;
}

.xv-row-header {
  background: var(--border);
  text-align: center;
  font-weight: 600;
  font-size: 10px;
  color: var(--text-muted);
  position: sticky;
  left: 0;
  z-index: 1;
  width: 32px;
  min-width: 32px;
}

.xv-cell {
  cursor: pointer;
  position: relative;
  transition: background-color 0.1s;
}

.xv-cell::after {
  content: attr(data-tooltip);
  position: absolute;
  bottom: calc(100% + 4px);
  left: 50%;
  transform: translateX(-50%) scale(0.9);
  background: var(--hint-bg);
  color: var(--hint-text);
  padding: 3px 7px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  white-space: nowrap;
  pointer-events: none;
  opacity: 0;
  z-index: 20;
  transition: opacity 0.15s ease, transform 0.15s ease;
  transition-delay: 0s;
}

.xv-cell:hover::after {
  opacity: 1;
  transform: translateX(-50%) scale(1);
  transition-delay: 0.5s;
}

.xv-cell:hover {
  background-color: var(--accent-tint) !important;
  outline: 2px solid var(--color-primary);
  outline-offset: -2px;
}

.xv-cell.selected {
  background-color: var(--accent-tint) !important;
  outline: 2px solid var(--color-primary);
  outline-offset: -2px;
}

.xv-cell.mapped {
  background-color: var(--success-bg) !important;
}

.xv-cell.mapped:hover {
  background-color: var(--success-bg) !important;
}

.xv-cell-text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
}

.xv-cell-image {
  display: block;
  max-width: 100%;
  height: auto;
  object-fit: contain;
}

.xv-mapped-badge {
  position: absolute;
  top: 0;
  right: 0;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 1px;
  background: var(--color-primary);
  color: var(--accent-contrast);
  font-size: 8px;
  padding: 1px 3px;
  border-radius: 0 0 0 4px;
  max-width: 60px;
  overflow: hidden;
  white-space: nowrap;
}

.xv-mapped-part {
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
}

/* Номер = порядок склейки в ячейке: первым в бланк пойдёт поле с единицей. */
.xv-mapped-order {
  display: inline-block;
  min-width: 8px;
  margin-right: 2px;
  padding: 0 1px;
  border-radius: 2px;
  background: var(--accent-contrast);
  color: var(--color-primary);
  font-weight: 700;
  text-align: center;
}

</style>
