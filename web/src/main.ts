import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { VueQueryPlugin } from '@tanstack/vue-query'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'element-plus/dist/index.css'
import './styles/tokens.css'
import './styles/global.css'
import './styles/light.css'
import App from './App.vue'
import { router } from './app/router'
import { queryClient } from './app/query-client'
import { initializeTheme } from './stores/theme'

initializeTheme()

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(ElementPlus, { locale: zhCn })
app.use(VueQueryPlugin, { queryClient })

app.mount('#app')
