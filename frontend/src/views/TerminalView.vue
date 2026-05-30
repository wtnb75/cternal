<template>
  <div class="terminal-view">
    <div class="toolbar">
      <span class="session-info">{{ t('session') }}: {{ sessionId }} · {{ mode }}</span>
      <span :class="['status', connected ? 'connected' : 'disconnected']">
        {{ connected ? t('connected') : t('reconnecting') }}
      </span>
      <button @click="router.push(`/sessions/${sessionId}/replay`)" class="btn btn-sm">{{ t('replay') }}</button>
      <button @click="downloadCast" class="btn btn-sm btn-download">{{ t('downloadCast') }}</button>
    </div>
    <div class="terminal-wrapper">
      <div ref="termEl" class="terminal-container" />
      <div v-if="showSearch" class="search-bar">
        <input
          ref="searchInputEl"
          v-model="searchQuery"
          placeholder="Search…"
          class="search-input"
          @input="doSearch"
          @keydown.enter.exact="doSearch"
          @keydown.shift.enter.prevent="doSearchPrev"
          @keydown.escape.prevent="closeSearch"
        />
        <button @click="doSearchPrev" class="btn-search-nav" title="Previous (Shift+Enter)">▲</button>
        <button @click="doSearch" class="btn-search-nav" title="Next (Enter)">▼</button>
        <button @click="closeSearch" class="btn-search-close" title="Close (Esc)">✕</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useTerminal } from '@/composables/useTerminal'
import { useWebSocket } from '@/composables/useWebSocket'
import { useConfigStore } from '@/stores/config'
import { apiUrl, wsUrl } from '@/lib/api'
import { useSettingsStore } from '@/stores/settings'
import type { WSMessage } from '@/types'

const route = useRoute()
const router = useRouter()
const sessionId = route.params.id as string

const termEl = ref<HTMLElement | null>(null)
const mode = ref('')
const showSearch = ref(false)
const searchQuery = ref('')
const searchInputEl = ref<HTMLInputElement | null>(null)

const { t } = useI18n()
const configStore = useConfigStore()
const settingsStore = useSettingsStore()
const { init, write, fit, onData, search, searchPrev, setFontSize, setTheme, terminal } = useTerminal()

const { connected, send } = useWebSocket(wsUrl(sessionId), (msg: WSMessage) => {
  if (msg.type === 'output') {
    write(msg.data)
  } else if (msg.type === 'exit') {
    write('\r\n\x1b[2m\x1b[33m── process exited ──\x1b[0m\r\n')
  }
})

function handleResize() {
  const size = fit()
  if (size) send({ type: 'resize', ...size })
}

async function downloadCast() {
  const res = await fetch(apiUrl(`/api/v1/sessions/${sessionId}/cast`))
  const blob = await res.blob()
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `${sessionId}.cast`
  a.click()
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

function doSearch() {
  if (searchQuery.value) search(searchQuery.value)
}

function doSearchPrev() {
  if (searchQuery.value) searchPrev(searchQuery.value)
}

onMounted(async () => {
  if (!termEl.value) return

  init(termEl.value, configStore.scrollback, settingsStore.fontSize, settingsStore.theme)

  // Keep terminal appearance in sync with settings while this session is open.
  watch(() => settingsStore.fontSize, (size) => {
    setFontSize(size)
    handleResize()
  })
  watch(() => settingsStore.theme, (theme) => {
    setTheme(theme)
  })
  onData?.((data: string) => send({ type: 'input', data }))
  handleResize()

  // Intercept Ctrl+F / Cmd+F before xterm consumes it.
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

  window.addEventListener('resize', handleResize)

  try {
    const res = await fetch(apiUrl(`/api/v1/sessions/${sessionId}`))
    const sess = await res.json()
    mode.value = sess.mode
  } catch { /* ignore */ }
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped>
.terminal-view {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: var(--bg-base);
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.5rem 1rem;
  background: var(--bg-surface);
  border-bottom: 1px solid var(--bg-surface-alt);
  flex-shrink: 0;
}

.session-info {
  color: var(--text-secondary);
  font-size: 0.85rem;
  flex: 1;
}

.status {
  font-size: 0.8rem;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
}

.connected { background: var(--green); color: var(--bg-base); }
.disconnected { background: var(--red); color: var(--bg-base); }

.terminal-wrapper {
  position: relative;
  flex: 1;
  overflow: hidden;
}

.terminal-container {
  width: 100%;
  height: 100%;
  padding: 4px;
}

.search-bar {
  position: absolute;
  top: 8px;
  right: 8px;
  display: flex;
  align-items: center;
  gap: 0.25rem;
  background: var(--bg-surface);
  border: 1px solid var(--bg-surface-alt);
  border-radius: 6px;
  padding: 0.3rem 0.4rem;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
  z-index: 10;
}

.search-input {
  width: 200px;
  padding: 0.25rem 0.4rem;
  background: var(--bg-base);
  border: 1px solid var(--bg-surface-alt);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 0.85rem;
  outline: none;
}
.search-input:focus { border-color: var(--accent); }
.search-input::placeholder { color: var(--text-muted); }

.btn-search-nav {
  padding: 0.2rem 0.4rem;
  background: var(--bg-surface-alt);
  border: none;
  border-radius: 3px;
  color: var(--text-primary);
  cursor: pointer;
  font-size: 0.75rem;
  line-height: 1;
}
.btn-search-nav:hover { background: var(--text-muted); }

.btn-search-close {
  padding: 0.2rem 0.4rem;
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.85rem;
  line-height: 1;
  border-radius: 3px;
}
.btn-search-close:hover { color: var(--text-primary); background: var(--bg-surface-alt); }

.btn {
  padding: 0.3rem 0.7rem;
  border: none;
  border-radius: 4px;
  background: var(--accent);
  color: var(--bg-base);
  cursor: pointer;
  font-size: 0.8rem;
}

.btn-sm { padding: 0.2rem 0.5rem; }
</style>
