<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import ContainerSidebar from '@/components/ContainerSidebar.vue'
import { useConfigStore } from '@/stores/config'

const route = useRoute()
const configStore = useConfigStore()
configStore.load() // fire-and-forget: fetch before any terminal mounts
</script>

<template>
  <div class="app-layout">
    <aside class="sidebar">
      <ContainerSidebar />
    </aside>
    <main class="main-panel">
      <RouterView :key="route.fullPath" />
    </main>
  </div>
</template>

<style>
/* Global styles are in src/assets/main.css */
.app-layout {
  display: flex;
  width: 100%;
  height: 100%;
}

.sidebar {
  width: 260px;
  flex-shrink: 0;
  background: #181825;
  border-right: 1px solid #313244;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.main-panel {
  flex: 1;
  overflow: hidden;
  min-width: 0;
}
</style>
