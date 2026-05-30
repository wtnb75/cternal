<template>
  <div class="terminal-pane" :class="{ active: isActive }" @mousedown="emit('activate')">
    <div class="pane-header">
      <span class="pane-info">{{ sessionId.slice(0, 8) }} · {{ mode || '…' }}</span>
      <span :class="['pane-status', connected ? 'connected' : 'disconnected']">
        {{ connected ? t('connected') : t('reconnecting') }}
      </span>
      <button @click.stop="emit('close')" class="btn-close-pane" :title="t('close')">✕</button>
    </div>
    <div class="pane-body">
      <div ref="termEl" class="pane-terminal" />
      <div v-if="showSearch" class="search-bar">
        <input
          ref="searchInputEl"
          v-model="searchQuery"
          :placeholder="t('search') + '…'"
          class="search-input"
          @input="doSearch"
          @keydown.enter.exact="doSearch"
          @keydown.shift.enter.prevent="doSearchPrev"
          @keydown.escape.prevent="closeSearch"
        />
        <button @click="doSearchPrev" class="btn-search-nav">▲</button>
        <button @click="doSearch" class="btn-search-nav">▼</button>
        <button @click="closeSearch" class="btn-search-close">✕</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTerminal } from '@/composables/useTerminal'
import { useWebSocket } from '@/composables/useWebSocket'
import { useConfigStore } from '@/stores/config'
import { useSettingsStore } from '@/stores/settings'
import { apiUrl, wsUrl } from '@/lib/api'
import type { WSMessage } from '@/types'

const props = defineProps<{
  sessionId: string
  paneIndex: number
  isActive: boolean
}>()
const emit = defineEmits<{
  activate: []
  close: []
}>()

const { t } = useI18n()
const configStore = useConfigStore()
const settingsStore = useSettingsStore()

const termEl = ref<HTMLElement | null>(null)
const mode = ref('')
const showSearch = ref(false)
const searchQuery = ref('')
const searchInputEl = ref<HTMLInputElement | null>(null)

const { init, write, fit, onData, search, searchPrev, setFontSize, setTheme, terminal } = useTerminal()

const { connected, send } = useWebSocket(wsUrl(props.sessionId), (msg: WSMessage) => {
  if (msg.type === 'output') write(msg.data)
  else if (msg.type === 'exit') write('\r\n\x1b[2m\x1b[33m── process exited ──\x1b[0m\r\n')
})

function handleResize() {
  const size = fit()
  if (size) send({ type: 'resize', ...size })
}

function openSearch() {
  showSearch.value = true
  nextTick(() => searchInputEl.value?.focus())
}

function closeSearch() {
  showSearch.value = false
  searchQuery.value = ''
  terminal()?.focus()
}

function doSearch() { if (searchQuery.value) search(searchQuery.value) }
function doSearchPrev() { if (searchQuery.value) searchPrev(searchQuery.value) }

let resizeObserver: ResizeObserver | null = null

onMounted(async () => {
  if (!termEl.value) return

  init(termEl.value, configStore.scrollback, settingsStore.fontSize, settingsStore.theme)
  onData?.((data: string) => send({ type: 'input', data }))
  handleResize()

  terminal()?.attachCustomKeyEventHandler((e: KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'f' && e.type === 'keydown') {
      openSearch()
      return false
    }
    if (e.key === 'Escape' && showSearch.value && e.type === 'keydown') {
      closeSearch()
      return false
    }
    return true
  })

  // ResizeObserver handles both window resize and pane layout changes.
  resizeObserver = new ResizeObserver(() => handleResize())
  resizeObserver.observe(termEl.value)

  try {
    const res = await fetch(apiUrl(`/api/v1/sessions/${props.sessionId}`))
    const sess = await res.json()
    mode.value = sess.mode
  } catch { /* ignore */ }
})

onUnmounted(() => {
  resizeObserver?.disconnect()
})

// Expose for parent-triggered theme/font updates (via watch in PaneView)
defineExpose({ setFontSize, setTheme })
</script>

<style scoped>
.terminal-pane {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 2px solid transparent;
  transition: border-color 0.15s;
}
.terminal-pane.active { border-color: var(--accent); }

.pane-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.3rem 0.6rem;
  background: var(--bg-surface);
  border-bottom: 1px solid var(--bg-surface-alt);
  flex-shrink: 0;
  min-height: 0;
}

.pane-info {
  font-size: 0.75rem;
  color: var(--text-secondary);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pane-status {
  font-size: 0.7rem;
  padding: 0.1rem 0.35rem;
  border-radius: 3px;
  flex-shrink: 0;
}
.connected { background: var(--green); color: var(--bg-base); }
.disconnected { background: var(--red); color: var(--bg-base); }

.btn-close-pane {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.8rem;
  padding: 0.1rem 0.3rem;
  border-radius: 3px;
  line-height: 1;
  flex-shrink: 0;
}
.btn-close-pane:hover { color: var(--red); background: var(--bg-surface-alt); }

.pane-body {
  position: relative;
  flex: 1;
  overflow: hidden;
}

.pane-terminal {
  width: 100%;
  height: 100%;
  padding: 2px;
}

.search-bar {
  position: absolute;
  top: 6px;
  right: 6px;
  display: flex;
  align-items: center;
  gap: 0.2rem;
  background: var(--bg-surface);
  border: 1px solid var(--bg-surface-alt);
  border-radius: 5px;
  padding: 0.25rem 0.35rem;
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.4);
  z-index: 10;
}

.search-input {
  width: 160px;
  padding: 0.2rem 0.35rem;
  background: var(--bg-base);
  border: 1px solid var(--bg-surface-alt);
  border-radius: 3px;
  color: var(--text-primary);
  font-size: 0.8rem;
  outline: none;
}
.search-input:focus { border-color: var(--accent); }
.search-input::placeholder { color: var(--text-muted); }

.btn-search-nav {
  padding: 0.15rem 0.35rem;
  background: var(--bg-surface-alt);
  border: none;
  border-radius: 3px;
  color: var(--text-primary);
  cursor: pointer;
  font-size: 0.7rem;
}
.btn-search-nav:hover { background: var(--text-muted); }

.btn-search-close {
  padding: 0.15rem 0.35rem;
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.8rem;
  border-radius: 3px;
}
.btn-search-close:hover { color: var(--text-primary); background: var(--bg-surface-alt); }
</style>
