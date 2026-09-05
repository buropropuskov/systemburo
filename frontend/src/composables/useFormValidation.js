import { ref, computed } from 'vue'

export function useFormValidation(rules) {
  const showTooltip = ref(false)
  const fieldErrors = ref({})
  const touched = ref({})

  const missingFields = computed(() => {
    return rules().filter(r => !r.check).map(r => r.message)
  })

  const isValid = computed(() => missingFields.value.length === 0)

  /**
   * Причины, по которым кнопка недоступна.
   *
   * Заголовок нейтральный: в списке лежат не только незаполненные поля, но и
   * запреты - «Машина в чёрном списке», «На этот автомобиль уже есть активная
   * заявка», «в заявке уже есть машина „По факту“». С «Заполните поля» они
   * читались как требование что-то ввести (#2320).
   */
  const tooltipMessage = computed(() => {
    if (isValid.value) return ''
    const fields = missingFields.value
    if (fields.length === 1) return `Не хватает: ${fields[0]}`
    return `Не хватает:\n${fields.map(f => `• ${f}`).join('\n')}`
  })

  function validateField(fieldName) {
    touched.value[fieldName] = true
    const rule = rules().find(r => r.field === fieldName)
    if (rule && !rule.check) {
      fieldErrors.value[fieldName] = rule.message
    } else {
      delete fieldErrors.value[fieldName]
    }
  }

  function validateAll() {
    rules().forEach(r => {
      if (r.field) {
        touched.value[r.field] = true
        if (!r.check) {
          fieldErrors.value[r.field] = r.message
        } else {
          delete fieldErrors.value[r.field]
        }
      }
    })
    return isValid.value
  }

  function getFieldError(fieldName) {
    return touched.value[fieldName] ? (fieldErrors.value[fieldName] || '') : ''
  }

  function resetValidation() {
    fieldErrors.value = {}
    touched.value = {}
  }

  return {
    isValid,
    missingFields,
    tooltipMessage,
    showTooltip,
    fieldErrors,
    touched,
    validateField,
    validateAll,
    getFieldError,
    resetValidation,
  }
}
