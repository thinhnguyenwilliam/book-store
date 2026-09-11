import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from './App.vue'
import router from './app/router'
import './assets/styles/main.css'
import { installGoogleAnalytics } from './features/analytics/lib/google-analytics'

installGoogleAnalytics(router)
createApp(App).use(createPinia()).use(router).mount('#app')
