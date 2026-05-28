<template>
  <div class="sidebar-inner">
    <div class="sidebar-header">
      <span class="title">{{ t('containers') }}</span>
      <div class="header-actions">
        <div class="pane-toggle">
          <button
            v-for="n in ([1, 2, 4] as const)"
            :key="n"
            @click="switchPaneCount(n)"
            :class="['btn-pane', { active: paneStore.count === n }]"
          >{{ n }}</button>
        </div>
        <button @click="load" class="btn-icon" :title="t('refresh')">↻</button>
        <button @click="showSettings = true" class="btn-icon" :title="t('settings')">⚙</button>
      </div>
    </div>

    <div class="sidebar-filter">
      <input v-model="nameFilter" :placeholder="t('filterByName') + '…'" class="filter-input" />
    </div>

    <div v-if="error" class="sidebar-error">{{ error }}</div>
    <div v-if="loading && !containers.length" class="sidebar-empty">{{ t('loading') }}</div>
    <div v-else-if="!containers.length" class="sidebar-empty">{{ t('noContainers') }}</div>

    <ul v-else class="container-list">
      <li
        v-for="c in containers"
        :key="c.id"
        :class="['container-item', { active: isActive(c.id) }]"
      >
        <div class="container-info">
          <span :class="['status-dot', c.running ? 'running' : 'stopped']" />
          <span class="container-name" :title="c.name || c.id">{{ c.name || c.id.slice(0, 12) }}</span>
        </div>
        <div class="container-image">{{ c.image }}</div>
        <div class="container-actions">
          <button @click="connect(c, 'exec')" :disabled="!c.running" class="btn-action">{{ t('exec') }}</button>
          <button @click="connect(c, 'attach')" :disabled="!c.running" class="btn-action">{{ t('attach') }}</button>
          <button @click="connect(c, 'logs')" class="btn-action">{{ t('logs') }}</button>
        </div>
      </li>
    </ul>

    <SettingsModal v-if="showSettings" @close="showSettings = false" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useSessionStore } from '@/stores/session'
import { usePaneStore, type PaneCount } from '@/stores/pane'
import type { Container } from '@/types'
import SettingsModal from './SettingsModal.vue'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const store = useSessionStore()
const paneStore = usePaneStore()

const containers = ref<Container[]>([])
const nameFilter = ref('')
const loading = ref(false)
const error = ref('')
const showSettings = ref(false)

function isActive(containerId: string): boolean {
  if (paneStore.count > 1 || route.name === 'panes') {
    return paneStore.sessionIds.some(id => {
      if (!id) return false
      return store.sessions.some(s => s.id === id && s.containerId === containerId)
    })
  }
  const id = route.params.id as string | undefined
  if (!id) return false
  return store.sessions.some(s => s.id === id && s.containerId === containerId)
}

function switchPaneCount(n: PaneCount) {
  paneStore.setCount(n)
  if (n === 1) {
    // Navigate to the active pane's session, or welcome
    const activeId = paneStore.sessionIds[paneStore.activeIndex]
    if (activeId) router.push({ name: 'terminal', params: { id: activeId } })
    else router.push({ name: 'welcome' })
  } else {
    // Transfer current single-terminal session to pane 0 if applicable
    if (route.name === 'terminal' && route.params.id) {
      const currentId = route.params.id as string
      if (!paneStore.sessionIds.some(id => id === currentId)) {
        paneStore.sessionIds[0] = currentId
        paneStore.activeIndex = 0
      }
    }
    router.push({ name: 'panes' })
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams()
    if (nameFilter.value) params.set('name', nameFilter.value)
    const res = await fetch('/api/v1/containers?' + params.toString())
    containers.value = await res.json()
  } catch {
    error.value = t('failedFetch')
  } finally {
    loading.value = false
  }
}

async function connect(container: Container, mode: 'exec' | 'attach' | 'logs') {
  error.value = ''
  try {
    const sess = await store.createSession({
      containerId: container.id,
      containerName: container.name,
      mode,
    })
    if (paneStore.count > 1) {
      paneStore.assignSession(sess.id)
      router.push({ name: 'panes' })
    } else {
      router.push({ name: 'terminal', params: { id: sess.id } })
    }
  } catch (err: unknown) {
    error.value = err instanceof Error ? err.message : t('failedConnect')
  }
}

watch(nameFilter, () => load())
onMounted(load)
</script>

<style scoped>
.sidebar-inner {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem 1rem 0.5rem;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.title {
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-secondary);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 0.2rem;
}

.pane-toggle {
  display: flex;
  border: 1px solid var(--bg-surface-alt);
  border-radius: 4px;
  overflow: hidden;
  margin-right: 0.2rem;
}

.btn-pane {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.7rem;
  padding: 0.1rem 0.35rem;
  line-height: 1.4;
  transition: background 0.1s, color 0.1s;
}
.btn-pane:not(:last-child) { border-right: 1px solid var(--bg-surface-alt); }
.btn-pane:hover { background: var(--bg-surface); color: var(--text-primary); }
.btn-pane.active { background: var(--accent); color: var(--bg-base); }

.btn-icon {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 1rem;
  padding: 0.1rem 0.3rem;
  border-radius: 3px;
  line-height: 1;
}
.btn-icon:hover { color: var(--text-primary); background: var(--bg-surface); }

.sidebar-filter {
  padding: 0.5rem 0.75rem;
  flex-shrink: 0;
}

.filter-input {
  width: 100%;
  padding: 0.35rem 0.5rem;
  background: var(--bg-surface);
  border: 1px solid var(--bg-surface-alt);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 0.8rem;
}
.filter-input::placeholder { color: var(--text-muted); }

.sidebar-error {
  padding: 0.5rem 0.75rem;
  font-size: 0.8rem;
  color: var(--red);
}

.sidebar-empty {
  padding: 1rem 0.75rem;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.container-list {
  list-style: none;
  overflow-y: auto;
  flex: 1;
}

.container-item {
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--bg-base);
  cursor: default;
  transition: background 0.1s;
}
.container-item:hover { background: var(--bg-surface); }
.container-item.active { background: var(--bg-surface); border-left: 2px solid var(--accent); }

.container-info {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  margin-bottom: 0.15rem;
}

.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}
.status-dot.running { background: var(--green); }
.status-dot.stopped { background: var(--text-muted); }

.container-name {
  font-size: 0.85rem;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.container-image {
  font-size: 0.72rem;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-bottom: 0.35rem;
}

.container-actions {
  display: flex;
  gap: 0.3rem;
}

.btn-action {
  padding: 0.18rem 0.45rem;
  font-size: 0.72rem;
  border: 1px solid var(--bg-surface-alt);
  border-radius: 3px;
  background: var(--bg-surface);
  color: var(--text-primary);
  cursor: pointer;
  transition: background 0.1s;
}
.btn-action:hover:not(:disabled) { background: var(--bg-surface-alt); }
.btn-action:disabled { opacity: 0.35; cursor: not-allowed; }
</style>
