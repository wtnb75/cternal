import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import SettingsModal from '../SettingsModal.vue'
import { useConfigStore } from '@/stores/config'
import en from '@/locales/en.json'

function makeI18n() {
  return createI18n({ legacy: false, locale: 'en', messages: { en } })
}

describe('SettingsModal', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  function mountModal() {
    return mount(SettingsModal, {
      global: { plugins: [pinia, makeI18n()] },
    })
  }

  it('does not show a user section when username is not configured', () => {
    const wrapper = mountModal()
    expect(wrapper.find('.settings-user').exists()).toBe(false)
  })

  it('shows username and logout link when configured', () => {
    const configStore = useConfigStore()
    configStore.username = 'alice'
    configStore.logoutUrl = '/oauth2/sign_out'

    const wrapper = mountModal()
    expect(wrapper.find('.settings-user').text()).toContain('alice')
    const logoutLink = wrapper.find('.settings-user a')
    expect(logoutLink.attributes('href')).toBe('/oauth2/sign_out')
    expect(logoutLink.text()).toBe('Log out')
  })

  it('shows username without a logout link when logoutUrl is not configured', () => {
    const configStore = useConfigStore()
    configStore.username = 'alice'

    const wrapper = mountModal()
    expect(wrapper.find('.settings-user').text()).toContain('alice')
    expect(wrapper.find('.settings-user a').exists()).toBe(false)
  })
})
