import { ref, computed } from 'vue'

export function useFormValidation(rules) {
  const showTooltip = ref(false)

  const missingFields = computed(() => {
    return rules().filter(r => !r.check).map(r => r.message)
  })

  const isValid = computed(() => missingFields.value.length === 0)

  const tooltipMessage = computed(() => {
    if (isValid.value) return ''
    const fields = missingFields.value
    if (fields.length === 1) return `Заполните поле: ${fields[0]}`
    return `Заполните поля:\n${fields.map(f => `- ${f}`).join('\n')}`
  })

  return { isValid, missingFields, tooltipMessage, showTooltip }
}
