<template>
  <div class="container-list">
    <h1>Containers</h1>
    <div class="filters">
      <input v-model="nameFilter" placeholder="Filter by name" class="filter-input" />
      <select v-model="statusFilter" class="filter-select">
        <option value="">All statuses</option>
        <option value="running">Running</option>
        <option value="exited">Exited</option>
      </select>
      <button @click="load" class="btn">Refresh</button>
    </div>

    <div v-if="error" class="error">{{ error }}</div>

    <div v-if="loading" class="loading">Loading...</div>

    <table v-else-if="containers.length" class="container-table">
      <thead>
        <tr>
          <th>Name</th>
          <th>Image</th>
          <th>Status</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="c in containers" :key="c.id">
          <td>{{ c.name || c.id.slice(0, 12) }}</td>
          <td>{{ c.image }}</td>
          <td :class="c.running ? 'running' : 'stopped'">{{ c.status }}</td>
          <td class="actions">
            <button @click="connect(c, 'exec')" :disabled="!c.running" class="btn btn-sm">
              Exec
            </button>
            <button @click="connect(c, 'attach')" :disabled="!c.running" class="btn btn-sm">
              Attach
            </button>
            <button @click="connect(c, 'logs')" class="btn btn-sm">Logs</button>
          </td>
        </tr>
      </tbody>
    </table>

    <div v-else class="empty">No containers found.</div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import type { Container } from '@/types'

const router = useRouter()
const store = useSessionStore()

const containers = ref<Container[]>([])
const nameFilter = ref('')
const statusFilter = ref('')
const loading = ref(false)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams()
    if (nameFilter.value) params.set('name', nameFilter.value)
    if (statusFilter.value) params.set('status', statusFilter.value)
    const res = await fetch('/api/v1/containers?' + params.toString())
    containers.value = await res.json()
  } catch {
    error.value = 'Failed to fetch containers'
  } finally {
    loading.value = false
  }
}

async function connect(container: Container, mode: 'exec' | 'attach' | 'logs') {
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

watch([nameFilter, statusFilter], () => load())
onMounted(load)
</script>

<style scoped>
.container-list {
  padding: 1.5rem;
  max-width: 1200px;
  margin: 0 auto;
}

h1 {
  margin-bottom: 1rem;
  font-size: 1.5rem;
}

.filters {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1rem;
  flex-wrap: wrap;
}

.filter-input,
.filter-select {
  padding: 0.4rem 0.6rem;
  border: 1px solid #45475a;
  border-radius: 4px;
  background: #313244;
  color: #cdd6f4;
  font-size: 0.9rem;
}

.container-table {
  width: 100%;
  border-collapse: collapse;
}

.container-table th,
.container-table td {
  text-align: left;
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid #313244;
}

.container-table th {
  color: #a6adc8;
  font-size: 0.8rem;
  text-transform: uppercase;
}

.running { color: #a6e3a1; }
.stopped { color: #f38ba8; }

.actions { display: flex; gap: 0.4rem; }

.btn {
  padding: 0.4rem 0.8rem;
  border: none;
  border-radius: 4px;
  background: #cba6f7;
  color: #1e1e2e;
  cursor: pointer;
  font-size: 0.85rem;
}

.btn:disabled { opacity: 0.4; cursor: not-allowed; }
.btn-sm { padding: 0.25rem 0.5rem; font-size: 0.8rem; }

.error { color: #f38ba8; margin-bottom: 1rem; }
.loading { color: #a6adc8; }
.empty { color: #a6adc8; padding: 1rem 0; }
</style>
