<template>
  <span
    class="file-type-icon"
    v-html="svgMarkup"
    :style="{ display: 'inline-flex', flexShrink: 0 }"
  />
</template>

<script>
/**
 * Цветные SVG-иконки типов файлов (#39).
 * Геометрия: лист с загнутым углом + цветная плашка с расширением.
 * Цвета по семейству формата (как в офисных пакетах).
 *
 * Разметка SVG - статические доверенные константы, не пользовательский ввод,
 * поэтому v-html здесь безопасен.
 */

const COLOR_MAP = {
  pdf:  ['#E5252A', 'PDF'],
  doc:  ['#2B579A', 'DOC'],
  docx: ['#2B579A', 'DOCX'],
  xls:  ['#1D7044', 'XLS'],
  xlsx: ['#1D7044', 'XLSX'],
  ppt:  ['#C13B1B', 'PPT'],
  pptx: ['#C13B1B', 'PPTX'],
};

const FALLBACK = ['#8A8F9E', 'FILE'];

function buildSvg(ext, size) {
  const [color, label] = COLOR_MAP[ext] || FALLBACK;
  const h = Math.round(size * 1.2);
  // Более длинные метки (4 символа: DOCX, XLSX, PPTX) — меньший шрифт
  const fs = label.length > 3 ? 6.2 : 7.4;
  return `<svg width="${size}" height="${h}" viewBox="0 0 36 44" fill="none" xmlns="http://www.w3.org/2000/svg">
  <path d="M5 3h18l8 8v28a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z" fill="#fff" stroke="#E2E4EE" stroke-width="1.6"/>
  <path d="M23 3l8 8h-6a2 2 0 0 1-2-2V3z" fill="#EDEFF7"/>
  <rect x="3" y="24" width="22" height="12" rx="2.6" fill="${color}"/>
  <text x="14" y="32.6" font-size="${fs}" font-weight="800" fill="#fff" text-anchor="middle" font-family="Montserrat,Arial,sans-serif">${label}</text>
</svg>`;
}

export default {
  name: 'FileTypeIcon',
  props: {
    ext: {
      type: String,
      required: true,
    },
    size: {
      type: Number,
      default: 30,
    },
  },
  computed: {
    svgMarkup() {
      // Бэкенд отдаёт расширение с ведущей точкой (".pdf"), COLOR_MAP — без неё.
      const ext = this.ext.toLowerCase().replace(/^\./, '');
      return buildSvg(ext, this.size);
    },
  },
};
</script>
