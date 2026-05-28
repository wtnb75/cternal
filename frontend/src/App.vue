<script setup lang="ts">
import { watch } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import ContainerSidebar from '@/components/ContainerSidebar.vue'
import { useConfigStore } from '@/stores/config'
import { useSettingsStore } from '@/stores/settings'

const route = useRoute()
const configStore = useConfigStore()
const settings = useSettingsStore()
const { locale } = useI18n()

configStore.load() // fire-and-forget: fetch before any terminal mounts

// Apply saved theme immediately, then keep in sync on changes.
settings.applyTheme()

// Keep i18n locale in sync with settings store.
watch(() => settings.language, (lang) => { locale.value = lang }, { immediate: true })
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
  background: var(--bg-overlay);
  border-right: 1px solid var(--border);
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
