import { createApp } from 'vue'
import { ElDrawer, ElInput, ElSelect, ElOption, ElInputNumber, ElSwitch } from 'element-plus'
import 'element-plus/dist/index.css'
import './styles/app.css'
import App from './App.vue'
createApp(App).use(ElDrawer).use(ElInput).use(ElSelect).use(ElOption).use(ElInputNumber).use(ElSwitch).mount('#app')
