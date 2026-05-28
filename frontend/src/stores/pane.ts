import { defineStore } from 'pinia'
import { ref } from 'vue'

export type PaneCount = 1 | 2 | 4

function loadCount(): PaneCount {
  const v = Number(localStorage.getItem('cternal.paneCount'))
  return (v === 2 || v === 4) ? v : 1
}

export const usePaneStore = defineStore('pane', () => {
  const count = ref<PaneCount>(loadCount())
  const activeIndex = ref(0)
  const sessionIds = ref<(string | null)[]>([null, null, null, null])

  function setCount(n: PaneCount) {
    count.value = n
    localStorage.setItem('cternal.paneCount', String(n))
    if (activeIndex.value >= n) activeIndex.value = 0
  }

  function setActive(index: number) {
    activeIndex.value = index
  }

  function assignSession(sessionId: string) {
    const ids = [...sessionIds.value]
    ids[activeIndex.value] = sessionId
    sessionIds.value = ids
  }

  function closePane(index: number) {
    const ids = [...sessionIds.value]
    ids[index] = null
    sessionIds.value = ids
    if (activeIndex.value === index) {
      const next = ids.findIndex((id, i) => i !== index && id !== null)
      activeIndex.value = next >= 0 ? next : (index > 0 ? index - 1 : 0)
    }
  }

  return { count, activeIndex, sessionIds, setCount, setActive, assignSession, closePane }
})
