import { unref } from 'vue'

/**
 * Хелперы потребления настройки полей выбранного шаблона вложения (#529).
 *
 * Источник конфига - ref / reactive / геттер / простой объект вида
 * `{ [fieldKey]: { visible, required } }`. Передавать лучше геттером
 * (`() => props.fieldConfig`), чтобы сохранить реактивность пропса.
 *
 * Семантика дефолта: нет строки конфига для ключа -> поле видимо и обязательно.
 * Это повторяет текущее поведение форм подачи, поэтому шаблоны без явной
 * настройки не меняются.
 *
 * Единая точка потребления field-config для всех секций формы подачи
 * (DateRangeSection + люди/машины/предметы) - чтобы H-6/7/8 не разъехались
 * в разные способы интеграции.
 *
 * @param {object | import('vue').Ref<object> | (() => object)} source - источник конфига
 * @returns {{ fieldVisible: (key: string) => boolean, fieldRequired: (key: string) => boolean }}
 */
export function useFieldConfig(source) {
  const resolve = (key) => {
    const cfg = typeof source === 'function' ? source() : unref(source)
    return cfg ? cfg[key] : undefined
  }

  const fieldVisible = (key) => {
    const c = resolve(key)
    return c ? c.visible : true
  }

  const fieldRequired = (key) => {
    const c = resolve(key)
    return c ? c.required : true
  }

  return { fieldVisible, fieldRequired }
}
