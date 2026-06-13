<template>
  <div class="modal-backdrop" @click.self="$emit('close')">
    <div class="modal" role="dialog" aria-modal="true">
      <div class="modal-header">
        <span class="modal-title">{{ t('settings') }}</span>
        <button class="btn-close" @click="$emit('close')">✕</button>
      </div>

      <div class="modal-body">
        <!-- Login user -->
        <div v-if="configStore.username" class="setting-row settings-user">
          <label class="setting-label">{{ t('user') }}</label>
          <div class="settings-user-row">
            <span class="settings-user-name">{{ configStore.username }}</span>
            <a v-if="configStore.logoutUrl" :href="configStore.logoutUrl" class="settings-user-logout">{{ t('logout') }}</a>
          </div>
        </div>

        <!-- Theme -->
        <div class="setting-row">
          <label class="setting-label">{{ t('theme') }}</label>
          <div class="radio-group">
            <label class="radio-option">
              <input type="radio" v-model="settings.theme" value="dark" />
              {{ t('dark') }}
            </label>
            <label class="radio-option">
              <input type="radio" v-model="settings.theme" value="light" />
              {{ t('light') }}
            </label>
          </div>
        </div>

        <!-- Language -->
        <div class="setting-row">
          <label class="setting-label">{{ t('language') }}</label>
          <div class="radio-group">
            <label class="radio-option">
              <input type="radio" v-model="settings.language" value="ja" />
              {{ t('japanese') }}
            </label>
            <label class="radio-option">
              <input type="radio" v-model="settings.language" value="en" />
              {{ t('english') }}
            </label>
          </div>
        </div>

        <!-- Font Size -->
        <div class="setting-row">
          <label class="setting-label">{{ t('fontSize') }}</label>
          <div class="font-size-control">
            <input
              type="range"
              min="10"
              max="24"
              step="1"
              v-model.number="settings.fontSize"
              class="font-range"
            />
            <span class="font-value">{{ settings.fontSize }}px</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings'
import { useConfigStore } from '@/stores/config'

defineEmits<{ close: [] }>()

const { t } = useI18n()
const settings = useSettingsStore()
const configStore = useConfigStore()
</script>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal {
  background: var(--bg-surface);
  border: 1px solid var(--bg-surface-alt);
  border-radius: 8px;
  width: 340px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid var(--bg-surface-alt);
}

.modal-title {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
}

.btn-close {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.9rem;
  padding: 0.2rem 0.4rem;
  border-radius: 3px;
  line-height: 1;
}
.btn-close:hover { color: var(--text-primary); background: var(--bg-surface-alt); }

.modal-body {
  padding: 1rem;
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.setting-row {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.setting-label {
  font-size: 0.78rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-secondary);
}

.radio-group {
  display: flex;
  gap: 1rem;
}

.radio-option {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.875rem;
  color: var(--text-primary);
  cursor: pointer;
}
.radio-option input { accent-color: var(--accent); cursor: pointer; }

.font-size-control {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.font-range {
  flex: 1;
  accent-color: var(--accent);
}

.font-value {
  font-size: 0.85rem;
  color: var(--text-primary);
  min-width: 36px;
  text-align: right;
}

.settings-user-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.settings-user-name {
  font-size: 0.875rem;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.settings-user-logout {
  color: var(--accent);
  text-decoration: none;
  font-size: 0.8rem;
  flex-shrink: 0;
}
.settings-user-logout:hover { text-decoration: underline; }
</style>
