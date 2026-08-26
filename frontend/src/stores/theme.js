import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  DEFAULT_THEME,
  applyTheme,
  isValidTheme,
  readStoredTheme,
  storeTheme,
} from '@/utils/theme'
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

  /** Применяет тему к DOM + localStorage, без обращения к бэку. */
  function applyLocal(id) {
    current.value = applyTheme(id)
    storeTheme(current.value)
  }

  /**
   * Выбор пользователя: применяем сразу, потом сохраняем в профиль. Ошибку
   * сохранения показываем (тема на этом устройстве осталась, но на другое
   * не переедет) и НЕ откатываем выбор - переключение уже видно на экране.
   *
   * Сохраняем ЯВНЫЙ id, а не `current.value`: id уже проверен `isValidTheme`,
   * так что `applyTheme` его не переписывает, и запрос не зависит от того, что
   * успело попасть в общий ref.
   *
   * @param {string} id
   */
  async function setTheme(id) {
    if (!isValidTheme(id) || id === current.value) return
    choiceSeq += 1
    applyLocal(id)
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
