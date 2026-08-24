// Сборка PDF резюме из resume/resume.md.
//
// Запуск:  node src/build-resume.mjs
//
// Контакты в исходнике не хранятся: ветка лежит в репозитории продукта, и телефон
// с почтой оттуда уже не убрать. Подстановка идёт из resume/contacts.local.json,
// который не коммитится. Есть файл - собирается pdf/resume-personal.pdf с
// контактами, нет файла - pdf/resume.pdf с заглушками.

import { createRequire } from 'node:module'
import { readFileSync, writeFileSync, mkdirSync, existsSync } from 'node:fs'
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
    `  ANALYST_NODE_MODULES=/путь/к/node_modules node src/build-resume.mjs`
  )
}

const MarkdownIt = loadDep('markdown-it')
const { chromium } = loadDep('playwright')
// linkify выключен: он делает ссылкой каждое draw.io и hh.ru в тексте навыков.
const md = new MarkdownIt({ html: false, linkify: false, typographer: false })

const CONTACTS_FILE = join(ROOT, 'resume', 'contacts.local.json')
const personal = existsSync(CONTACTS_FILE)
  ? JSON.parse(readFileSync(CONTACTS_FILE, 'utf8'))
  : null

function contactsHtml() {
  if (!personal) {
    return '<span class="gap">телефон</span> · <span class="gap">почта</span> · ' +
      '<span class="gap">telegram</span> · <span class="gap">портфолио</span>'
  }
  const parts = []
  if (personal.phone) parts.push(md.utils.escapeHtml(personal.phone))
  if (personal.email) parts.push(`<a href="mailto:${personal.email}">${md.utils.escapeHtml(personal.email)}</a>`)
  if (personal.telegram) parts.push(md.utils.escapeHtml(personal.telegram))
  if (personal.portfolio) parts.push(`<a href="${personal.portfolio}">${md.utils.escapeHtml(personal.portfolio)}</a>`)
  return parts.join(' · ')
}

const source = readFileSync(join(ROOT, 'resume', 'resume.md'), 'utf8')
let body = md.render(source)

// Первый h2 идёт сразу за именем и держит целевую должность, а не раздел.
body = body.replace(/<h2>([\s\S]*?)<\/h2>/, (m, inner) => `<p class="role">${inner}</p>`)
// Строка контактов и подсветка незаполненных мест.
body = body.replace(/<p>\{CONTACTS\}<\/p>/, `<p class="contacts">${contactsHtml()}</p>`)
body = body.replace(/⟨([^⟩]*)⟩/g, (m, inner) => `<span class="gap">${inner}</span>`)
// Строка «город · формат · занятость» стоит первой и набирается мельче.
body = body.replace(/<p>(Москва[\s\S]*?)<\/p>/, '<p class="meta">$1</p>')
// Разделитель в заголовках мест работы: точки-посредники приглушаются.
body = body.replace(/<h3>([\s\S]*?)<\/h3>/g, (m, inner) =>
  `<h3>${inner.replace(/ · /g, ' <span class="sep">·</span> ')}</h3>`)

function plural(n, one, few, many) {
  const d10 = n % 10
  const d100 = n % 100
  if (d10 === 1 && d100 !== 11) return one
  if (d10 >= 2 && d10 <= 4 && (d100 < 12 || d100 > 14)) return few
  return many
}

const gaps = (source.match(/⟨/g) || []).length
const draftNote = gaps
  ? `<p class="draft">Черновик: не заполнено ${gaps} ${plural(gaps, 'место', 'места', 'мест')}. ` +
    `Подсвеченные фрагменты нужно закрыть или удалить до отправки.</p>`
  : ''

const css = readFileSync(join(HERE, 'resume.css'), 'utf8')
const html = `<!doctype html><html lang="ru"><head><meta charset="utf-8">
<title>Резюме</title><style>${css}</style></head><body>${draftNote}${body}</body></html>`

mkdirSync(OUT, { recursive: true })
const name = personal ? 'resume-personal' : 'resume'
const tmp = join(OUT, `.${name}.html`)
writeFileSync(tmp, html)

const browser = await chromium.launch()
const page = await browser.newPage()
await page.goto(pathToFileURL(tmp).href, { waitUntil: 'load' })
const pdfPath = join(OUT, `${name}.pdf`)
await page.pdf({
  path: pdfPath,
  format: 'A4',
  printBackground: true,
  margin: { top: '13mm', bottom: '13mm', left: '15mm', right: '15mm' },
})
const pages = await page.evaluate(() => Math.ceil(document.body.scrollHeight / 1122))
await browser.close()

console.log(
  `${pdfPath}\n` +
  `контакты: ${personal ? 'подставлены из contacts.local.json' : 'заглушки, файла contacts.local.json нет'}\n` +
  `незаполненных мест: ${gaps}\n` +
  `ориентировочно страниц: ${pages}`
)
