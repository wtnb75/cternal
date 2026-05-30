<template>
  <div class="replay-view">
    <div class="toolbar">
      <button @click="router.push(`/sessions/${sessionId}`)" class="btn btn-back">← Terminal</button>
      <span class="session-info">{{ t('replay') }}: {{ sessionId }}</span>
      <button @click="togglePlay" class="btn">{{ playing ? t('pause') : t('play') }}</button>
      <select v-model="speed" class="speed-select">
        <option :value="0.5">0.5×</option>
        <option :value="1">1×</option>
        <option :value="2">2×</option>
        <option :value="5">5×</option>
      </select>
      <input
        type="range"
        :min="0"
        :max="events.length"
        v-model.number="seekPos"
        @input="onSeek"
        class="seek-bar"
      />
      <span class="pos-label">{{ seekPos }}/{{ events.length }}</span>
    </div>
    <div ref="termEl" class="terminal-container" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useTerminal } from '@/composables/useTerminal'
import { useSettingsStore } from '@/stores/settings'
import { apiUrl } from '@/lib/api'

interface RecordedEvent {
  Time: number
  Type: string
  Data: string
}

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const sessionId = route.params.id as string
const settingsStore = useSettingsStore()

const termEl = ref<HTMLElement | null>(null)
const events = ref<RecordedEvent[]>([])
const seekPos = ref(0)
const playing = ref(false)
const speed = ref(1)

const { init, write, fit, setTheme, dispose } = useTerminal()

let playTimer: ReturnType<typeof setTimeout> | null = null
let playIndex = 0

function stopPlay() {
  if (playTimer) {
    clearTimeout(playTimer)
    playTimer = null
  }
  playing.value = false
}

function playFrom(index: number) {
  stopPlay()
  playIndex = index
  playing.value = true
  stepPlay()
}

function stepPlay() {
  if (!playing.value) return

  while (playIndex < events.value.length && events.value[playIndex]?.Type !== 'o') {
    playIndex++
  }

  if (playIndex >= events.value.length) {
    stopPlay()
    return
  }

  const ev = events.value[playIndex]
  if (!ev) { stopPlay(); return }

  const next = events.value[playIndex + 1]
  write(ev.Data)
  seekPos.value = playIndex
  playIndex++

  if (next) {
    const delay = (next.Time - ev.Time) / speed.value / 1e6 // nanoseconds to ms
    playTimer = setTimeout(stepPlay, Math.max(0, delay))
  } else {
    stopPlay()
  }
}

function togglePlay() {
  if (playing.value) {
    stopPlay()
  } else {
    playFrom(seekPos.value)
  }
}

function onSeek() {
  stopPlay()
  if (!termEl.value) return
  // Dispose the current terminal and reinitialise on the same element so that
  // accumulated output is cleared before replaying up to the seek position.
  dispose()
  init(termEl.value)
  fit()
  const outputEvents = events.value.slice(0, seekPos.value).filter(e => e.Type === 'o')
  for (const ev of outputEvents) {
    write(ev.Data)
  }
}

onMounted(async () => {
  if (!termEl.value) return
  init(termEl.value, undefined, undefined, settingsStore.theme)
  fit()
  watch(() => settingsStore.theme, (theme) => setTheme(theme))

  try {
    const res = await fetch(apiUrl(`/api/v1/sessions/${sessionId}/events`))
    events.value = await res.json() ?? []
  } catch { /* ignore */ }
})

onUnmounted(stopPlay)
</script>

<style scoped>
.replay-view {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: var(--bg-base);
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  background: var(--bg-surface);
  border-bottom: 1px solid var(--bg-surface-alt);
  flex-shrink: 0;
  flex-wrap: wrap;
}

.session-info {
  color: var(--text-secondary);
  font-size: 0.85rem;
  flex: 1;
}

.seek-bar { flex: 2; min-width: 100px; accent-color: var(--accent); }

.speed-select {
  padding: 0.2rem 0.4rem;
  background: var(--bg-surface-alt);
  color: var(--text-primary);
  border: none;
  border-radius: 4px;
}

.pos-label {
  font-size: 0.8rem;
  color: var(--text-secondary);
  white-space: nowrap;
}

.terminal-container {
  flex: 1;
  overflow: hidden;
  padding: 4px;
}

.btn {
  padding: 0.3rem 0.7rem;
  border: none;
  border-radius: 4px;
  background: var(--accent);
  color: var(--bg-base);
  cursor: pointer;
  font-size: 0.8rem;
}

.btn-back { background: var(--bg-surface-alt); color: var(--text-primary); }
</style>
