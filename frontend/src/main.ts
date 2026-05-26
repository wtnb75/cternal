import './assets/main.css'

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'

import App from './App.vue'
import router from './router'
import ja from './locales/ja.json'
import en from './locales/en.json'

const pinia = createPinia()

const savedLang = (() => {
  try { return JSON.parse(localStorage.getItem('cternal.language') ?? '"ja"') } catch { return 'ja' }
})()

const i18n = createI18n({
  legacy: false,
  locale: savedLang,
  fallbackLocale: 'en',
  messages: { ja, en },
})

const app = createApp(App)
app.use(pinia)
app.use(router)
app.use(i18n)
app.mount('#app')
