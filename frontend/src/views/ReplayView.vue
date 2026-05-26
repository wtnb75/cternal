<template>
  <div class="replay-view">
    <div class="toolbar">
      <button @click="router.push(`/sessions/${sessionId}`)" class="btn btn-back">← Terminal</button>
      <span class="session-info">Replay: {{ sessionId }}</span>
      <button @click="togglePlay" class="btn">{{ playing ? 'Pause' : 'Play' }}</button>
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
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTerminal } from '@/composables/useTerminal'

interface RecordedEvent {
  Time: number
  Type: string
  Data: string
}

const route = useRoute()
const router = useRouter()
const sessionId = route.params.id as string

const termEl = ref<HTMLElement | null>(null)
const events = ref<RecordedEvent[]>([])
const seekPos = ref(0)
const playing = ref(false)
const speed = ref(1)

const { init, write, fit } = useTerminal()

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
  // Re-render from start up to seekPos
  const { init: reinit } = useTerminal()
  if (termEl.value) {
    reinit(termEl.value)
    fit()
    const outputEvents = events.value.slice(0, seekPos.value).filter(e => e.Type === 'o')
    for (const ev of outputEvents) {
      write(ev.Data)
    }
  }
}

onMounted(async () => {
  if (!termEl.value) return
  init(termEl.value)
  fit()

  try {
    const res = await fetch(`/api/v1/sessions/${sessionId}/events`)
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
  background: #1e1e2e;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  background: #313244;
  border-bottom: 1px solid #45475a;
  flex-shrink: 0;
  flex-wrap: wrap;
}

.session-info {
  color: #a6adc8;
  font-size: 0.85rem;
  flex: 1;
}

.seek-bar { flex: 2; min-width: 100px; }

.speed-select {
  padding: 0.2rem 0.4rem;
  background: #45475a;
  color: #cdd6f4;
  border: none;
  border-radius: 4px;
}

.pos-label {
  font-size: 0.8rem;
  color: #a6adc8;
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
  background: #cba6f7;
  color: #1e1e2e;
  cursor: pointer;
  font-size: 0.8rem;
}

.btn-back { background: #45475a; color: #cdd6f4; }
</style>
