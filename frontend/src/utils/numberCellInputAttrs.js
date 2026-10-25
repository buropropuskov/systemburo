export function numberCellInputAttrs(cell) {
  return {
    maxlength: cell.max_length,
    placeholder: (cell.cell_type === 'numbers' ? '0' : 'A').repeat(cell.max_length),
    style: { width: `${Math.max(50, cell.max_length * 25)}px` },
    inputmode: cell.cell_type === 'numbers' ? 'numeric' : 'text',
    autocapitalize: 'characters',
    autocorrect: 'off',
    spellcheck: false,
  };
}
