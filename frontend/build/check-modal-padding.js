/*
 * Окно без внутренних отступов - повторяющийся дефект: BaseModal отбивает
 * шапку и кнопки, а тело оставляет на нуле, и каждое окно вспоминает про
 * отступ само. Кто забыл - у того текст упирается в рамку, и ловит это
 * человек на экране, а не тесты.
 *
 * Проверка требует, чтобы у корневого элемента окна был свой отступ. Окна со
 * своей раскладкой (таблица во всю ширину, свой скролл, тулбар) перечислены
 * в FLUSH с причиной.
 */
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const SRC = path.join(ROOT, 'src')

/** Окна, которые рисуют раскладку сами: ключ - файл и метка окна, значение - почему. */
const FLUSH = {
  'components/UserControl.vue::user-edit-modal': 'карточка пользователя - свой тулбар и вкладки',
  'components/admin/AttachmentTemplateEditor.vue::te-modal-rounded': 'редактор бланка со своим скроллом и панелями',
  'components/ui/SelectionModal.vue::title': 'список с липкой строкой поиска - отступы у секций',
}

/** Классы с отступом, объявленные не в компоненте, а в общих стилях. */
const GLOBAL_PADDED = new Set(['reassign-body'])

function walk(dir, acc = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) walk(full, acc)
    else if (entry.name.endsWith('.vue') && !full.includes('__tests__')) acc.push(full)
  }
  return acc
}

/** Первый элемент дефолтного слота: именованные слоты и комментарии не в счёт. */
function slotRoot(slot) {
  const body = slot
    .replace(/<template\s+#[\s\S]*?<\/template>/g, '')
    .replace(/<!--[\s\S]*?-->/g, '')
    .trim()
  return /<([\w-]+)\b([^>]*)>/.exec(body)
}

function hasPadding(style, classes) {
  return classes.some((cls) => {
    if (GLOBAL_PADDED.has(cls)) return true
    const rule = new RegExp(`\\.${cls.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*(?:,[^{]*)?\\{([^}]*)\\}`).exec(style)
    return !!rule && /\bpadding/.test(rule[1])
  })
}

const failures = []

for (const file of walk(SRC)) {
  const source = fs.readFileSync(file, 'utf8')
  if (!source.includes('<BaseModal')) continue

  const rel = path.relative(SRC, file).split(path.sep).join('/')
  const style = (source.match(/<style[^>]*>[\s\S]*?<\/style>/g) || []).join('')

  for (const match of source.matchAll(/<BaseModal\b([^>]*?)>([\s\S]*?)<\/BaseModal>/g)) {
    const [, attrs, slot] = match
    const mark = /(?:title|content-class|content-testid|data-testid)="([^"]{0,40})"/.exec(attrs)
    const key = `${rel}::${mark ? mark[1] : ''}`
    if (FLUSH[key]) continue

    const root = slotRoot(slot)
    if (!root) continue
    const classAttr = /class="([^"]+)"/.exec(root[2])
    const classes = (classAttr ? classAttr[1].split(/\s+/) : []).filter((c) => !c.startsWith('{') && !c.startsWith('['))

    if (!hasPadding(style, classes)) {
      failures.push(`${rel}: окно «${mark ? mark[1] : root[1]}» - у корня <${root[1]}> нет внутреннего отступа.`)
    }
  }
}

if (failures.length > 0) {
  console.error(`Отступы в окнах: ${failures.length} нарушений.`)
  for (const line of failures) console.error(`  ${line}`)
  console.error('')
  console.error('Тело BaseModal идёт без отступов - их задаёт корневой элемент окна.')
  console.error('Обычное значение 16px 20px, как у шапки и кнопок окна.')
  console.error('Окну со своей раскладкой - строка в FLUSH в build/check-modal-padding.js с причиной.')
  process.exit(1)
}

console.log('Отступы в окнах: порядок, у всех есть внутренний отступ.')
