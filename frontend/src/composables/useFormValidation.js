import { ref, computed } from 'vue'

export function useFormValidation(rules) {
  const showTooltip = ref(false)
  const fieldErrors = ref({})
  const touched = ref({})

  const missingFields = computed(() => {
    return rules().filter(r => !r.check).map(r => r.message)
  })

  const isValid = computed(() => missingFields.value.length === 0)

  const tooltipMessage = computed(() => {
    if (isValid.value) return ''
    const fields = missingFields.value
    if (fields.length === 1) return `Заполните поле: ${fields[0]}`
    return `Заполните поля:\n${fields.map(f => `• ${f}`).join('\n')}`
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
