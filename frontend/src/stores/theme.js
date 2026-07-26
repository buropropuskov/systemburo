import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  DEFAULT_THEME,
  applyTheme,
  isValidTheme,
  readStoredTheme,
  storeTheme,
} from '@/utils/theme'
import { revealThemeChange } from '@/utils/themeTransition'
import { getTheme, saveTheme } from '@/api/theme'
import { useDeletionsStore } from '@/stores/deletions'

/**
 * Тема оформления (#1415).
 *
 * Два хранилища с разными ролями: localStorage применяет тему мгновенно (до
 * первого кадра, из bootstrap-скрипта index.html), профиль на бэке - источник
 * правды, чтобы выбор ехал за человеком между устройствами.
 */
export const useThemeStore = defineStore('theme', () => {
  // Стартуем с того, что уже применил bootstrap-скрипт: стор и DOM не расходятся
  // даже если хранилище недоступно (там и здесь фолбэк на светлую).
  const current = ref(readStoredTheme() || DEFAULT_THEME)
  applyTheme(current.value)

  // Счётчик выбора: ответ /users/me/theme, приехавший ПОСЛЕ клика юзера по теме,
  // не должен откатывать его выбор (last-resolve-wins на общий ref).
  let choiceSeq = 0

  /**
   * Применяет тему к DOM + localStorage, без обращения к бэку.
   *
   * @param {string} id
   * @param {{x: number, y: number}|null} [origin] точка клика: с ней тема
   *   заливает экран от курсора, без неё применяется мгновенно (загрузка,
   *   синхронизация профиля - анимировать там нечего).
   */
  function applyLocal(id, origin) {
    return revealThemeChange(() => {
      current.value = applyTheme(id)
      storeTheme(current.value)
    }, origin)
  }

  /**
   * Выбор пользователя: применяем сразу, потом сохраняем в профиль. Ошибку
   * сохранения показываем (тема на этом устройстве осталась, но на другое
   * не переедет) и НЕ откатываем выбор - переключение уже видно на экране.
   *
   * Заливку намеренно не ждём: запрос в профиль уходит параллельно анимации.
   * Поэтому и сохраняем ЯВНЫЙ id, а не `current.value`: с заливкой тему ставит
   * коллбэк View Transitions, который браузер зовёт после снятия кадра, и на
   * момент запроса в `current` лежала бы ещё прошлая тема (id уже проверен
   * `isValidTheme`, так что `applyTheme` его не переписывает).
   *
   * @param {string} id
   * @param {{x: number, y: number}|null} [origin] точка нажатия по пункту темы
   */
  async function setTheme(id, origin) {
    if (!isValidTheme(id) || id === current.value) return
    choiceSeq += 1
    applyLocal(id, origin)
    try {
      await saveTheme(id)
    } catch (e) {
      useDeletionsStore().notify({
        type: 'error',
        title: 'Тема не сохранена',
        prefix: 'Оформление применено на этом устройстве, но не сохранилось в профиле: ',
        bold: e?.message || 'ошибка сети',
      })
    }
  }

  /**
   * Подтягивает тему из профиля после входа/восстановления сессии. Пустая тема в
   * профиле = юзер не выбирал -> светлая: иначе на общем компьютере следующий
   * человек унаследовал бы чужую тему из localStorage.
   *
   * Сетевую ошибку глушим намеренно: остаётся уже применённая локальная тема,
   * ронять из-за оформления вход не за чем.
   */
  async function syncFromServer() {
    const seq = choiceSeq
    try {
      const { theme } = await getTheme()
      if (seq !== choiceSeq) return
      applyLocal(isValidTheme(theme) ? theme : DEFAULT_THEME)
    } catch {
      // Профиль недоступен - живём на локальной теме до следующей загрузки.
    }
  }

  return { current, setTheme, syncFromServer }
})
