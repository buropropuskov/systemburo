import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

const STORAGE_KEY = 'sound-prefs'

function loadPrefs() {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY)) || {}
  } catch {
    return {}
  }
}

export const useSoundStore = defineStore('sound', () => {
  const saved = loadPrefs()

  const enabled = ref(saved.enabled ?? false)
  const selectedPreset = ref(saved.selectedPreset ?? 'soft')
  const volume = ref(typeof saved.volume === 'number' ? saved.volume : 0.6)

  watch([enabled, selectedPreset, volume], ([en, preset, vol]) => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ enabled: en, selectedPreset: preset, volume: vol }))
    } catch {
      // localStorage недоступен (приватный режим) - персист best-effort, не критично.
    }
  })

  function setEnabled(value) {
    enabled.value = value
  }

  function setPreset(preset) {
    selectedPreset.value = preset
  }

  function setVolume(value) {
    volume.value = Math.max(0, Math.min(1, Number(value)))
  }

  return {
    enabled,
    selectedPreset,
    volume,
    setEnabled,
    setPreset,
    setVolume,
  }
})
