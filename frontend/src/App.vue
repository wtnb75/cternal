<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import ContainerSidebar from '@/components/ContainerSidebar.vue'
import { useConfigStore } from '@/stores/config'
import { useSettingsStore } from '@/stores/settings'

const route = useRoute()
const configStore = useConfigStore()
const settings = useSettingsStore()
const { locale, t } = useI18n()

configStore.load()

settings.applyTheme()

watch(() => settings.language, (lang) => { locale.value = lang }, { immediate: true })

const SIDEBAR_KEY = 'sidebar_collapsed'
const sidebarCollapsed = ref(localStorage.getItem(SIDEBAR_KEY) === '1')
watch(sidebarCollapsed, (v) => { try { localStorage.setItem(SIDEBAR_KEY, v ? '1' : '0') } catch { /* storage unavailable */ } })

function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value
  nextTick(() => {
    const el =
      document.querySelector<HTMLElement>('.terminal-pane.active .xterm-helper-textarea') ??
      document.querySelector<HTMLElement>('.xterm-helper-textarea')
    el?.focus()
  })
}
</script>

<template>
  <div class="app-layout">
    <aside id="main-sidebar" class="sidebar" :class="{ collapsed: sidebarCollapsed }">
      <ContainerSidebar />
    </aside>
    <button
      class="sidebar-toggle"
      :title="sidebarCollapsed ? t('expandSidebar') : t('collapseSidebar')"
      :aria-expanded="!sidebarCollapsed"
      aria-controls="main-sidebar"
      @click="toggleSidebar"
    >{{ sidebarCollapsed ? '›' : '‹' }}</button>
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
  border-right: none;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  transition: width 0.2s ease;
}
.sidebar.collapsed { width: 0; }

.sidebar-toggle {
  width: 14px;
  flex-shrink: 0;
  background: var(--bg-overlay);
  border: none;
  border-left: 1px solid var(--border);
  border-right: 1px solid var(--border);
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.8rem;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.1s, color 0.1s;
}
.sidebar-toggle:hover {
  background: var(--bg-surface);
  color: var(--text-primary);
}

.main-panel {
  flex: 1;
  overflow: hidden;
  min-width: 0;
}
</style>
