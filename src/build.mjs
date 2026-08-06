// Сборка PDF из markdown-исходников: markdown-it -> HTML -> печать в Chromium.
//
// Запуск:  node src/build.mjs           собрать все документы
//          node src/build.mjs 04        собрать только тот, чей файл начинается с 04
//
// Зависимости (markdown-it, playwright) не дублируются в этой ветке: скрипт ищет
// их в соседнем чекауте либо в каталоге из переменной ANALYST_NODE_MODULES.

import { createRequire } from 'node:module'
import { readFileSync, writeFileSync, mkdirSync, readdirSync } from 'node:fs'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { dirname, join, resolve } from 'node:path'

const HERE = dirname(fileURLToPath(import.meta.url))
const ROOT = resolve(HERE, '..')
const OUT = join(ROOT, 'pdf')

const CANDIDATES = [
  process.env.ANALYST_NODE_MODULES,
  resolve(ROOT, '../systemburo/frontend/node_modules'),
  resolve(ROOT, '../node_modules'),
].filter(Boolean)

function loadDep(name) {
  for (const dir of CANDIDATES) {
    try {
      const req = createRequire(pathToFileURL(join(dir, 'noop.js')))
      return req(name)
    } catch { /* следующий кандидат */ }
  }
  throw new Error(
    `не найден пакет ${name}. Укажи каталог с node_modules:\n` +
    `  ANALYST_NODE_MODULES=/путь/к/node_modules node src/build.mjs`
  )
}

const MarkdownIt = loadDep('markdown-it')
const { chromium } = loadDep('playwright')

const md = new MarkdownIt({ html: false, linkify: true, typographer: false })

// --- нумерация и якоря заголовков -----------------------------------------
// Оглавление собирается из h2/h3. Номер раздела проставляется здесь, а не в
// тексте: при вставке нового раздела нумерация в документе и в оглавлении
// разъезжается, если её ведут руками.
function slug(text, seen) {
  const base = text.toLowerCase()
    .replace(/[^\p{L}\p{N}]+/gu, '-')
    .replace(/^-+|-+$/g, '') || 'section'
  let s = base
  let i = 2
  while (seen.has(s)) s = `${base}-${i++}`
  seen.add(s)
  return s
}

function renderBody(source) {
  const tokens = md.parse(source, {})
  const seen = new Set()
  const toc = []
  let h2 = 0
  let h3 = 0

  for (let i = 0; i < tokens.length; i++) {
    const t = tokens[i]
    if (t.type !== 'heading_open') continue
    const inline = tokens[i + 1]
    const text = inline.content
    if (t.tag === 'h2' || t.tag === 'h3') {
      if (t.tag === 'h2') { h2 += 1; h3 = 0 } else { h3 += 1 }
      const num = t.tag === 'h2' ? `${h2}.` : `${h2}.${h3}`
      const id = slug(text, seen)
      t.attrSet('id', id)
      inline.children.unshift(Object.assign(new inline.constructor('html_inline', '', 0), {
        content: `<span class="num">${num}</span> `,
      }))
      toc.push({ level: t.tag, num, text, id })
    }
  }
  const html = md.renderer.render(tokens, md.options, {})
    // Короткие листинги не рвём по страницам, длинные разрешаем разорвать.
    .replace(/<pre>(<code[\s\S]*?<\/code>)<\/pre>/g, (m, inner) =>
      inner.split('\n').length <= 14 ? `<pre class="nobreak">${inner}</pre>` : m)
  return { html, toc }
}

function tocHtml(toc) {
  if (!toc.length) return ''
  const items = toc.map((e) =>
    `<li class="toc-${e.level}"><a href="#${e.id}"><span class="toc-num">${e.num}</span>` +
    `<span class="toc-text">${md.utils.escapeHtml(e.text)}</span></a></li>`
  ).join('\n')
  return `<nav class="toc"><h2 class="toc-title">Содержание</h2><ul>${items}</ul></nav>`
}

// --- шаблон страницы -------------------------------------------------------
function page(meta, bodyHtml, toc) {
  const css = readFileSync(join(HERE, 'style.css'), 'utf8')
  return `<!doctype html><html lang="ru"><head><meta charset="utf-8">
<title>${md.utils.escapeHtml(meta.title)}</title><style>${css}</style></head><body>
<header class="cover">
  <div class="cover__kicker">${md.utils.escapeHtml(meta.kicker)}</div>
  <h1 class="cover__title">${md.utils.escapeHtml(meta.title)}</h1>
  <p class="cover__lead">${md.utils.escapeHtml(meta.lead)}</p>
  <div class="cover__meta">Редакция от ${meta.date}</div>
</header>
${tocHtml(toc)}
<main>${bodyHtml}</main>
</body></html>`
}

// --- документы -------------------------------------------------------------
const DATE = '06.08.2026'
const DOCS = readdirSync(join(HERE, 'docs')).filter((f) => f.endsWith('.md')).sort()

const META = JSON.parse(readFileSync(join(HERE, 'docs', 'meta.json'), 'utf8'))

const filter = process.argv[2]

mkdirSync(OUT, { recursive: true })

const browser = await chromium.launch()
const ctx = await browser.newContext()
const results = []

for (const file of DOCS) {
  if (filter && !file.startsWith(filter)) continue
  const key = file.replace(/\.md$/, '')
  const meta = META[key]
  if (!meta) throw new Error(`нет описания для ${file} в docs/meta.json`)

  const source = readFileSync(join(HERE, 'docs', file), 'utf8')
  const { html, toc } = renderBody(source)
  const full = page({ ...meta, date: DATE }, html, toc)

  const tmp = join(OUT, `.${key}.html`)
  writeFileSync(tmp, full)

  const p = await ctx.newPage()
  await p.goto(pathToFileURL(tmp).href, { waitUntil: 'load' })
  const pdfPath = join(OUT, `${key}.pdf`)
  await p.pdf({
    path: pdfPath,
    format: 'A4',
    printBackground: true,
    margin: { top: '16mm', bottom: '16mm', left: '18mm', right: '18mm' },
    displayHeaderFooter: true,
    headerTemplate: '<div></div>',
    footerTemplate:
      `<div style="width:100%;font-family:Inter,sans-serif;font-size:8pt;color:#8a8a8a;` +
      `padding:0 18mm;display:flex;justify-content:space-between;">` +
      `<span>${meta.title}</span><span class="pageNumber"></span></div>`,
    outline: true,
  })
  await p.close()

  const bytes = readFileSync(pdfPath).length
  results.push({ key, title: meta.title, kb: Math.round(bytes / 1024), sections: toc.length })
}

await ctx.close()
await browser.close()

for (const r of results) {
  console.log(`${r.key.padEnd(28)} ${String(r.kb).padStart(5)} КБ  разделов: ${r.sections}  ${r.title}`)
}
console.log(`\nсобрано документов: ${results.length}, каталог: pdf/`)
