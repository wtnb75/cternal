<template>
  <div class="pane-view" :class="`grid-${paneStore.count}`">
    <template v-for="i in paneStore.count" :key="i">
      <TerminalPane
        v-if="paneStore.sessionIds[i - 1]"
        :key="paneStore.sessionIds[i - 1] ?? i"
        :session-id="paneStore.sessionIds[i - 1]!"
        :pane-index="i - 1"
        :is-active="paneStore.activeIndex === i - 1"
        @activate="paneStore.setActive(i - 1)"
        @close="paneStore.closePane(i - 1)"
      />
      <div
        v-else
        class="pane-empty"
        :class="{ active: paneStore.activeIndex === i - 1 }"
        @click="paneStore.setActive(i - 1)"
      >
        <span class="empty-hint">{{ t('selectContainer') }}</span>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { usePaneStore } from '@/stores/pane'
import TerminalPane from '@/components/TerminalPane.vue'

const { t } = useI18n()
const paneStore = usePaneStore()
</script>

<style scoped>
.pane-view {
  display: grid;
  width: 100%;
  height: 100%;
  background: var(--bg-base);
  gap: 2px;
}

.grid-1 { grid-template-columns: 1fr; }
.grid-2 { grid-template-columns: 1fr 1fr; }
.grid-4 { grid-template-columns: 1fr 1fr; grid-template-rows: 1fr 1fr; }

.pane-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-base);
  border: 2px dashed var(--bg-surface-alt);
  cursor: pointer;
  transition: border-color 0.15s;
}
.pane-empty:hover { border-color: var(--text-muted); }
.pane-empty.active { border-color: var(--accent); }

.empty-hint {
  color: var(--text-muted);
  font-size: 0.85rem;
}
</style>
