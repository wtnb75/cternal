import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import { createI18n } from 'vue-i18n'
import ContainerSidebar from '../ContainerSidebar.vue'
import { useConfigStore } from '@/stores/config'
import en from '@/locales/en.json'

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'welcome', component: { template: '<div/>' } },
      { path: '/sessions/:id', name: 'terminal', component: { template: '<div/>' } },
      { path: '/panes', name: 'panes', component: { template: '<div/>' } },
    ],
  })
}

function makeI18n() {
  return createI18n({ legacy: false, locale: 'en', messages: { en } })
}

describe('ContainerSidebar', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify([]))))
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  function mountSidebar() {
    return mount(ContainerSidebar, {
      global: { plugins: [makeRouter(), pinia, makeI18n()] },
    })
  }

  it('does not show username or logout link when not configured', async () => {
    const wrapper = mountSidebar()
    await flushPromises()
    expect(wrapper.find('.sidebar-user').exists()).toBe(false)
  })

  it('shows username and logout link when configured', async () => {
    const configStore = useConfigStore()
    configStore.username = 'alice'
    configStore.logoutUrl = '/oauth2/sign_out'

    const wrapper = mountSidebar()
    await flushPromises()
    expect(wrapper.find('.sidebar-user').text()).toContain('alice')
    const logoutLink = wrapper.find('.sidebar-user a')
    expect(logoutLink.exists()).toBe(true)
    expect(logoutLink.attributes('href')).toBe('/oauth2/sign_out')
    expect(logoutLink.text()).toBe('Log out')
  })

  it('shows username without a logout link when logoutUrl is not configured', async () => {
    const configStore = useConfigStore()
    configStore.username = 'alice'

    const wrapper = mountSidebar()
    await flushPromises()
    expect(wrapper.find('.sidebar-user').text()).toContain('alice')
    expect(wrapper.find('.sidebar-user a').exists()).toBe(false)
  })
})
