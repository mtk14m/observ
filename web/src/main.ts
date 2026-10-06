import { createApp } from 'vue'
import '@fontsource-variable/inter'
import '@fontsource/jetbrains-mono/400.css'
import './styles/tokens.css'
import './styles/base.css'
import App from './App.vue'
import { createAppRouter } from './router'

createApp(App).use(createAppRouter()).mount('#app')
