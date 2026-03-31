import { ref, computed } from 'vue'

export function useEntitySelection(idKey = 'id') {
  const tempSelected = ref([])
  const confirmed = ref([])

  function toggle(item) {
    const id = item[idKey]
    const idx = tempSelected.value.findIndex(i => i[idKey] === id)
    if (idx >= 0) tempSelected.value.splice(idx, 1)
    else tempSelected.value.push(item)
  }

  function isSelected(item) {
    return tempSelected.value.some(i => i[idKey] === item[idKey])
  }

  function confirm() {
    confirmed.value = [...tempSelected.value]
  }

  function syncFromConfirmed() {
    tempSelected.value = [...confirmed.value]
  }

  function reset() {
    tempSelected.value = []
    confirmed.value = []
  }

  const count = computed(() => tempSelected.value.length)
  const confirmedCount = computed(() => confirmed.value.length)

  return { tempSelected, confirmed, toggle, isSelected, confirm, syncFromConfirmed, reset, count, confirmedCount }
}
