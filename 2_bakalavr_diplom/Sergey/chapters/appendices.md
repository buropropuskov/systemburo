# ПРИЛОЖЕНИЯ

Приложение А -- Диаграммы бизнес-процессов и проектирования

Приложение Б -- Полный перечень требований к пользовательскому интерфейсу

Приложение В -- Листинги кода

Приложение Г -- Экраны интерфейса сервиса Dockee

## Приложение А

Диаграммы бизнес-процессов и проектирования

Рисунок А.1 -- Бизнес-процесс «Регистрация и онбординг пользователя»

Рисунок А.2 -- Бизнес-процесс «Загрузка и анализ документа»

Рисунок А.3 -- Бизнес-процесс «Работа с результатами анализа»

Рисунок А.4 -- Диаграмма активности «Загрузка и анализ документа»

## Приложение Б

Полный перечень требований к пользовательскому интерфейсу

### Б.1 Функциональные требования

Таблица Б.1 -- Функциональные требования к пользовательскому интерфейсу

| ID | Требование | Зачем | Как проверить |
| ФТ-01 | Загрузка документов PDF, DOCX, TXT через диалог выбора файла | Поддержка основных форматов юридических документов | Загрузить файл каждого формата, убедиться в корректном отображении |
| ФТ-02 | Загрузка документов через перетаскивание (drag-and-drop) | Ускорение рабочего процесса для опытных пользователей | Перетащить файл в область загрузки, убедиться в успешном приёме |
| ФТ-03 | Ограничение размера загружаемого файла (50 МБ) | Защита сервера от перегрузки | Попытаться загрузить файл более 50 МБ, убедиться в отклонении с сообщением |
| ФТ-04 | Отображение загруженного документа во встроенном просмотрщике | Работа с документом без переключения между приложениями | Загрузить PDF и DOCX, убедиться в корректном рендеринге |
| ФТ-05 | Пролистывание страниц документа | Навигация по многостраничным документам | Загрузить документ на 20+ страниц, пролистать все страницы |
| ФТ-06 | Изменение масштаба текста документа (zoom) | Адаптация под разные размеры экранов и зрение пользователя | Увеличить и уменьшить масштаб, убедиться в корректном рендеринге |
| ФТ-07 | Подсветка рисков в тексте документа цветовыми маркерами | Мгновенное визуальное определение проблемных мест | Загрузить документ с известными рисками, проверить наличие подсветки |
| ФТ-08 | Три уровня цветовой маркировки (критический, сомнительный, информационный) | Приоритизация рисков по степени важности | Проверить, что каждый уровень имеет свой цвет и текстовую метку |
| ФТ-09 | Пояснение при клике на подсвеченный фрагмент | Объяснение, почему фрагмент помечен как рискованный | Кликнуть на подсвеченный фрагмент, убедиться в появлении пояснения |
| ФТ-10 | Список рисков в панели инспектора с группировкой по типу и важности | Обзорная картина всех выявленных проблем | Проверить наличие группировки и корректность сортировки |
| ФТ-11 | Переход от элемента списка рисков к фрагменту в документе | Быстрая навигация между списком и текстом | Кликнуть на риск в панели, убедиться в прокрутке документа |
| ФТ-12 | Переход от подсвеченного фрагмента к элементу в панели рисков | Двунаправленная навигация | Кликнуть на фрагмент в тексте, убедиться в прокрутке панели |
| ФТ-13 | ИИ-помощник в виде чата (выдвигающаяся панель) | Возможность задавать вопросы по документу | Открыть чат, отправить вопрос, получить ответ |
| ФТ-14 | Вопрос ИИ по выделенному фрагменту текста | Контекстный анализ конкретного участка документа | Выделить текст, нажать «Спросить ИИ», проверить релевантность ответа |
| ФТ-15 | Свободный вопрос ИИ без привязки к тексту | Общие вопросы по содержанию документа | Ввести вопрос в чат, убедиться в получении ответа |
| ФТ-16 | Заметки, привязанные к конкретному риску | Фиксация решений и комментариев к каждому риску | Создать заметку к риску, закрыть документ, открыть повторно, проверить сохранение |
| ФТ-17 | Общие заметки по документу | Фиксация промежуточных выводов по документу | Создать общую заметку, закрыть, открыть, проверить |
| ФТ-18 | Изменение статуса риска (принять, отклонить, уточнить) | Отслеживание прогресса обработки рисков | Изменить статус, обновить страницу, убедиться в сохранении |
| ФТ-19 | Экспорт отчёта в формате PDF | Получение документа для передачи коллегам | Скачать отчёт, открыть в PDF-ридере, проверить содержимое |
| ФТ-20 | Экспорт отчёта в формате DOCX | Возможность редактирования отчёта в текстовом редакторе | Скачать отчёт, открыть в Word, проверить форматирование |
| ФТ-21 | Отчёт включает сводку, описания рисков, цитаты, рекомендации, заметки | Полнота информации в отчёте | Проверить наличие всех разделов в скачанном файле |
| ФТ-22 | Просмотр списка загруженных документов | Управление файлами пользователя | Загрузить несколько файлов, открыть страницу «Документы», проверить список |
| ФТ-23 | Поиск документов по названию | Быстрый доступ к нужному файлу | Ввести часть названия, убедиться в фильтрации |
| ФТ-24 | Сортировка документов по дате и имени | Упорядочивание списка файлов | Переключить сортировку, проверить корректность порядка |
| ФТ-25 | Создание папок | Организация файлов по категориям | Создать папку, переместить файл, проверить |
| ФТ-26 | Удаление документов с подтверждением | Предотвращение случайного удаления | Нажать «Удалить», убедиться в появлении диалога подтверждения |
| ФТ-27 | Переименование документов (inline-редактирование) | Исправление названий файлов | Двойной клик на название, ввести новое, проверить сохранение |
| ФТ-28 | Отображение использования дискового пространства | Контроль лимита хранилища | Проверить наличие индикатора «X МБ / Y ГБ» |
| ФТ-29 | Корзина для удалённых документов | Возможность восстановления ошибочно удалённых файлов | Удалить файл, открыть корзину, восстановить |
| ФТ-30 | Страница аналитики с фильтрами (период, тип, контрагент) | Анализ статистики по проверенным документам | Выбрать фильтры, убедиться в обновлении данных |
| ФТ-31 | Карточки метрик (документы, качество, контрагенты, риски, отклонения) | Быстрый обзор ключевых показателей | Проверить наличие и корректность значений |
| ФТ-32 | Экспорт аналитики в Excel | Передача данных для внешнего анализа | Скачать Excel, открыть, проверить данные |
| ФТ-33 | Личный кабинет (профиль, настройки, подписка, устройства) | Управление аккаунтом пользователя | Пройти по каждой вкладке, проверить функционал |

### Б.2 Нефункциональные требования

Таблица Б.2 -- Нефункциональные требования

| ID | Требование | Значение | Метод проверки |
| НФТ-01 | Время отклика интерфейса на действие пользователя | Менее 200 мс | Измерение через DevTools Performance |
| НФТ-02 | Время от загрузки до отображения результатов анализа | Менее 30 секунд | Таймер от нажатия «Анализировать» до появления результатов |
| НФТ-03 | Адаптивная вёрстка (минимальная ширина) | 320 пикселей | Проверка в DevTools при ширине 320px |
| НФТ-04 | Адаптивная вёрстка (максимальная ширина) | 2560 пикселей | Проверка на мониторе 2560px |
| НФТ-05 | Поддержка браузеров | Chrome 90+, Firefox 90+, Safari 14+, Edge 90+ | Запуск на каждом браузере, визуальная проверка |
| НФТ-06 | Протокол передачи данных | HTTPS | Проверка через DevTools Network |
| НФТ-07 | Хранение сессионных данных | HttpOnly + Secure cookies | Проверка через DevTools Application |
| НФТ-08 | Экранирование пользовательского ввода | Все выводимые данные экранируются | Попытка ввести HTML-тег в поле, проверить отсутствие рендеринга |
| НФТ-09 | Обновление сторонних библиотек | Мониторинг уязвимостей через npm audit | Запуск npm audit, проверка отсутствия critical |
| НФТ-10 | Доступность интерфейса (Uptime) | 99.5% | Мониторинг через внешний сервис |
| НФТ-11 | Поведение при потере соединения | Сообщение + кнопка повтора | Отключить сеть, проверить появление баннера |
| НФТ-12 | Доступность с клавиатуры (Tab, Enter, Пробел) | Все интерактивные элементы | Пройти весь интерфейс без мыши |
| НФТ-13 | Контрастность текста | Минимум 4.5:1 (WCAG AA) | Проверка через Lighthouse Accessibility |
| НФТ-14 | LCP (Largest Contentful Paint) | Менее 2.5 секунд | Lighthouse Performance |
| НФТ-15 | FID (First Input Delay) | Менее 100 мс | Lighthouse Performance |
| НФТ-16 | CLS (Cumulative Layout Shift) | Менее 0.1 | Lighthouse Performance |
| НФТ-17 | Размер JavaScript-бандла (gzip) | Менее 300 КБ (начальная загрузка) | Проверка через DevTools Network |
| НФТ-18 | Кэширование статических ресурсов | Cache-Control: max-age=31536000 | Проверка заголовков через DevTools |

### Б.3 UX-требования

Таблица Б.3 -- UX-требования

| ID | Требование | Значение | Метод проверки |
| UX-01 | Время выполнения основных сценариев новым пользователем | Менее 3 минут (загрузка, анализ, отчёт) | Юзабилити-тестирование с 5 участниками |
| UX-02 | Время проверки типового договора опытным пользователем | Менее 5 минут | Замер времени при повторном использовании |
| UX-03 | Текстовые подписи на всех кнопках | 100% кнопок с текстом или подсказкой | Визуальная проверка каждого экрана |
| UX-04 | Всплывающие подсказки для иконок | Все иконки без текста имеют tooltip | Навести курсор на каждую иконку |
| UX-05 | Подтверждение необратимых действий (удаление) | Диалог подтверждения | Попытаться удалить файл, проверить диалог |
| UX-06 | Понятные сообщения об ошибках | Указание причины + предложение действия | Загрузить файл неподдерживаемого формата, проверить сообщение |
| UX-07 | Встроенная справка или руководство | Иконка «?» с доступом к справке | Нажать на иконку, убедиться в открытии справки |
| UX-08 | Отсутствие обучения при первом использовании | Интерфейс понятен без инструкций | Наблюдение за новым пользователем (zero instruction test) |

## Приложение В

Листинги кода

*Листинг В.1 -- Компонент главной секции лендинга*

```vue
<template>
  <section id="service" class="service" :class="{ animate: isAnimated }">
    <div class="service__description">
      <div class="service__badge">
        <img src="@/assets/icons/service_stars.png" class="icon__stars" alt="Stars" />
        ИИ-технологии нового поколения
      </div>
      <h1 class="service__title">
        Сервис анализа и проверки документов
        <span class="color-blue">с помощью ИИ</span>
      </h1>
      <h2 class="service__subtitle">
        Мгновенная проверка документов с минимальными затратами.
        Сосредоточьтесь на росте бизнеса, а безопасность оставьте нам
      </h2>
      <div class="service__bottom">
        <button class="service__button">
          <img src="@/assets/icons/service_button.png" class="icon" alt="Start" />
          Начать пользоваться
        </button>
        <p class="service__hint">
          Попробуйте <strong class="free">бесплатно</strong>
        </p>
      </div>
    </div>
    <img src="@/assets/service_photo.svg" class="service__photo" alt="Documents" />
  </section>
</template>

<script setup>
import { ref, onMounted } from 'vue'

const isAnimated = ref(false)

onMounted(() => {
  setTimeout(() => {
    isAnimated.value = true
  }, 50)
})
</script>
```

*Листинг В.2 -- Медиа-запросы для адаптивной вёрстки герой-секции*

```css
@media (max-width: 1200px) {
  .service {
    width: 90%;
    flex-direction: column;
    text-align: center;
  }
  .service__description {
    width: 100%;
  }
  .service__badge {
    justify-content: center;
  }
  .service__bottom {
    justify-content: center;
  }
  .service__photo {
    margin-top: 40px;
    max-width: 300px;
  }
}

@media (max-width: 768px) {
  .service__title {
    font-size: 28px;
  }
  .service__subtitle {
    font-size: 16px;
  }
  .service__button {
    padding: 12px 24px;
    font-size: 14px;
  }
}
```

*Листинг В.3 -- Компонент формы обратной связи*

```vue
<script setup>
import { ref, reactive } from 'vue'

const isModalOpen = ref(false)
const isSubmitting = ref(false)
const error = ref('')
const success = ref(false)

const formData = reactive({
  name: '',
  phone: '',
  email: '',
  subject: ''
})

function openModal() {
  isModalOpen.value = true
  error.value = ''
  success.value = false
}

function closeModal() {
  isModalOpen.value = false
}

async function submitForm() {
  if (!formData.email.includes('@')) {
    error.value = 'Укажите корректный email'
    return
  }
  if (!formData.name.trim()) {
    error.value = 'Укажите ваше имя'
    return
  }

  isSubmitting.value = true
  try {
    const response = await fetch('/api/feedback', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(formData)
    })
    if (response.ok) {
      success.value = true
      setTimeout(closeModal, 2000)
    } else {
      error.value = 'Ошибка отправки. Попробуйте позже.'
    }
  } catch (e) {
    error.value = 'Нет подключения к серверу'
  } finally {
    isSubmitting.value = false
  }
}
</script>
```

### Листинги кода сервиса

*Листинг В.1 -- Хранилище авторизации (Pinia store)*

```javascript
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const accessToken = ref(null)
  const user = ref(null)

  const isAuthenticated = computed(() => !!accessToken.value)

  async function login(email, password) {
    const response = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ email, password })
    })
    if (!response.ok) throw new Error('Неверные учётные данные')
    const data = await response.json()
    accessToken.value = data.accessToken
    user.value = data.user
  }

  async function refreshToken() {
    const response = await fetch('/api/auth/refresh', {
      method: 'POST',
      credentials: 'include'
    })
    if (!response.ok) {
      logout()
      throw new Error('Сессия истекла')
    }
    const data = await response.json()
    accessToken.value = data.accessToken
  }

  function logout() {
    accessToken.value = null
    user.value = null
  }

  return { accessToken, user, isAuthenticated, login, refreshToken, logout }
})
```

*Листинг В.2 -- Обёртка для HTTP-запросов (apiClient)*

```javascript
import { useAuthStore } from '@/stores/auth'
import router from '@/router'

export async function apiClient(url, options = {}) {
  const auth = useAuthStore()

  const headers = {
    ...options.headers
  }
  if (auth.accessToken) {
    headers['Authorization'] = `Bearer ${auth.accessToken}`
  }

  let response = await fetch(url, { ...options, headers, credentials: 'include' })

  if (response.status === 401) {
    try {
      await auth.refreshToken()
      headers['Authorization'] = `Bearer ${auth.accessToken}`
      response = await fetch(url, { ...options, headers, credentials: 'include' })
    } catch {
      router.push('/login')
      throw new Error('Требуется авторизация')
    }
  }

  return response
}
```

*Листинг В.3 -- Компонент загрузки документа*

```vue
<template>
  <div
    class="upload-zone"
    :class="{ 'upload-zone--active': isDragging }"
    @dragover.prevent="isDragging = true"
    @dragleave="isDragging = false"
    @drop.prevent="handleDrop"
    @click="openFileDialog"
  >
    <input
      ref="fileInput"
      type="file"
      accept=".pdf,.docx,.txt"
      hidden
      @change="handleFileSelect"
    />
    <div v-if="!isUploading" class="upload-zone__content">
      <img src="@/assets/icons/upload.svg" alt="Upload" />
      <p>Перетащите файл или нажмите для выбора</p>
      <span class="upload-zone__hint">PDF, DOCX, TXT (до 50 МБ)</span>
    </div>
    <div v-else class="upload-zone__progress">
      <div class="progress-bar" :style="{ width: progress + '%' }"></div>
      <span>{{ progress }}%</span>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useDocumentStore } from '@/stores/document'

const fileInput = ref(null)
const isDragging = ref(false)
const isUploading = ref(false)
const progress = ref(0)

const ALLOWED_TYPES = ['application/pdf', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'text/plain']
const MAX_SIZE = 50 * 1024 * 1024

const documentStore = useDocumentStore()

function openFileDialog() {
  fileInput.value.click()
}

function handleDrop(event) {
  isDragging.value = false
  const file = event.dataTransfer.files[0]
  if (file) uploadFile(file)
}

function handleFileSelect(event) {
  const file = event.target.files[0]
  if (file) uploadFile(file)
}

async function uploadFile(file) {
  if (!ALLOWED_TYPES.includes(file.type)) {
    alert('Неподдерживаемый формат. Используйте PDF, DOCX или TXT.')
    return
  }
  if (file.size > MAX_SIZE) {
    alert('Файл слишком большой. Максимальный размер: 50 МБ.')
    return
  }

  isUploading.value = true
  progress.value = 0

  const formData = new FormData()
  formData.append('document', file)

  const xhr = new XMLHttpRequest()
  xhr.upload.onprogress = (e) => {
    if (e.lengthComputable) progress.value = Math.round((e.loaded / e.total) * 100)
  }
  xhr.onload = () => {
    isUploading.value = false
    if (xhr.status === 200) {
      const { taskId } = JSON.parse(xhr.responseText)
      documentStore.startAnalysis(taskId)
    }
  }
  xhr.onerror = () => {
    isUploading.value = false
    alert('Ошибка загрузки. Проверьте подключение.')
  }
  xhr.open('POST', '/api/documents/upload')
  xhr.send(formData)
}
</script>
```

*Листинг В.4 -- Polling результатов анализа*

```javascript
export async function pollAnalysisResult(taskId, onResult, onError, interval = 2000) {
  const poll = async () => {
    try {
      const response = await apiClient(`/api/documents/result/${taskId}`)
      if (response.status === 202) {
        setTimeout(poll, interval)
        return
      }
      if (response.ok) {
        const result = await response.json()
        onResult(result)
        return
      }
      onError(new Error(`Сервер вернул статус ${response.status}`))
    } catch (err) {
      onError(err)
    }
  }
  poll()
}
```

*Листинг В.5 -- Функция экранирования HTML*

```javascript
export function escapeHtml(str) {
  if (typeof str !== 'string') return ''
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}
```

*Листинг В.6 -- Обёртка для localStorage*

```javascript
export const settingsStorage = {
  set(key, value) {
    localStorage.setItem(key, JSON.stringify(value))
  },
  get(key, defaultValue = null) {
    const value = localStorage.getItem(key)
    return value ? JSON.parse(value) : defaultValue
  },
  remove(key) {
    localStorage.removeItem(key)
  }
}
```

*Листинг В.7 -- Ленивая загрузка маршрутов*

```javascript
import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes = [
  {
    path: '/',
    name: 'Landing',
    component: () => import('@/views/LandingPage.vue')
  },
  {
    path: '/main',
    name: 'Main',
    component: () => import('@/views/MainPage.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/documents',
    name: 'Documents',
    component: () => import('@/views/DocumentsPage.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/analytics',
    name: 'Analytics',
    component: () => import('@/views/AnalyticsPage.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/account',
    name: 'Account',
    component: () => import('@/views/AccountPage.vue'),
    meta: { requiresAuth: true }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to) => {
  if (to.meta.requiresAuth) {
    const auth = useAuthStore()
    if (!auth.isAuthenticated) {
      return { path: '/login' }
    }
  }
})

export default router
```

## Приложение Г

Экраны интерфейса сервиса Dockee

Рисунок Г.1 -- Лендинг: первый экран (герой-секция)

Рисунок Г.2 -- Лендинг: блок тарифов

Рисунок Г.3 -- Лендинг: FAQ и форма обратной связи

Рисунок Г.4 -- Главная страница: загрузка документа

Рисунок Г.5 -- Главная страница: результаты анализа с подсветкой рисков

Рисунок Г.6 -- Главная страница: ИИ-помощник (чат)

Рисунок Г.7 -- Страница «Документы»: файловое хранилище

Рисунок Г.8 -- Страница «Аналитика»: статистика и фильтры

Рисунок Г.9 -- Личный кабинет: настройки и подписка
