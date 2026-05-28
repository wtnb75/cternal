import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useConfigStore = defineStore('config', () => {
  const scrollback = ref(5000)

  async function load() {
    try {
      const res = await fetch('/api/v1/config')
      if (res.ok) {
        const cfg = await res.json()
        if (typeof cfg.scrollback === 'number' && cfg.scrollback > 0) {
          scrollback.value = cfg.scrollback
        }
      }
    } catch {}
  }

  return { scrollback, load }
})
