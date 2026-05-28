<template>
  <div class="sidebar-inner">
    <div class="sidebar-header">
      <span class="title">Containers</span>
      <button @click="load" class="btn-refresh" title="Refresh">↻</button>
    </div>

    <div class="sidebar-filter">
      <input v-model="nameFilter" placeholder="Filter by name…" class="filter-input" />
    </div>

    <div v-if="error" class="sidebar-error">{{ error }}</div>
    <div v-if="loading && !containers.length" class="sidebar-empty">Loading…</div>
    <div v-else-if="!containers.length" class="sidebar-empty">No containers found.</div>

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
          <button @click="connect(c, 'exec')" :disabled="!c.running" class="btn-action">Exec</button>
          <button @click="connect(c, 'attach')" :disabled="!c.running" class="btn-action">Attach</button>
          <button @click="connect(c, 'logs')" class="btn-action">Logs</button>
        </div>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import type { Container } from '@/types'

const router = useRouter()
const route = useRoute()
const store = useSessionStore()

const containers = ref<Container[]>([])
const nameFilter = ref('')
const loading = ref(false)
const error = ref('')

function isActive(containerId: string): boolean {
  const id = route.params.id as string | undefined
  if (!id) return false
  return store.sessions.some(s => s.id === id && s.containerId === containerId)
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
    error.value = 'Failed to fetch containers'
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
    router.push({ name: 'terminal', params: { id: sess.id } })
  } catch (err: unknown) {
    error.value = err instanceof Error ? err.message : String(err)
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
  border-bottom: 1px solid #313244;
  flex-shrink: 0;
}

.title {
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: #a6adc8;
}

.btn-refresh {
  background: none;
  border: none;
  color: #6c7086;
  cursor: pointer;
  font-size: 1rem;
  padding: 0.1rem 0.3rem;
  border-radius: 3px;
  line-height: 1;
}
.btn-refresh:hover { color: #cdd6f4; background: #313244; }

.sidebar-filter {
  padding: 0.5rem 0.75rem;
  flex-shrink: 0;
}

.filter-input {
  width: 100%;
  padding: 0.35rem 0.5rem;
  background: #313244;
  border: 1px solid #45475a;
  border-radius: 4px;
  color: #cdd6f4;
  font-size: 0.8rem;
}
.filter-input::placeholder { color: #6c7086; }

.sidebar-error {
  padding: 0.5rem 0.75rem;
  font-size: 0.8rem;
  color: #f38ba8;
}

.sidebar-empty {
  padding: 1rem 0.75rem;
  font-size: 0.8rem;
  color: #6c7086;
}

.container-list {
  list-style: none;
  overflow-y: auto;
  flex: 1;
}

.container-item {
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid #1e1e2e;
  cursor: default;
  transition: background 0.1s;
}
.container-item:hover { background: #25253a; }
.container-item.active { background: #2a2a45; border-left: 2px solid #cba6f7; }

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
.status-dot.running { background: #a6e3a1; }
.status-dot.stopped { background: #6c7086; }

.container-name {
  font-size: 0.85rem;
  color: #cdd6f4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.container-image {
  font-size: 0.72rem;
  color: #6c7086;
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
  border: 1px solid #45475a;
  border-radius: 3px;
  background: #313244;
  color: #cdd6f4;
  cursor: pointer;
  transition: background 0.1s;
}
.btn-action:hover:not(:disabled) { background: #45475a; }
.btn-action:disabled { opacity: 0.35; cursor: not-allowed; }
</style>
