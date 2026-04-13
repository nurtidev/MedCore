import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import router from './router'
import App from './App.vue'
import ru from './locales/ru.json'
import kk from './locales/kk.json'
import './style.css'

const i18n = createI18n({
  legacy: false,
  locale: localStorage.getItem('locale') ?? 'ru',
  fallbackLocale: 'ru',
  messages: { ru, kk },
})

createApp(App)
  .use(createPinia())
  .use(router)
  .use(i18n)
  .mount('#app')
