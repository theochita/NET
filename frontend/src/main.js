import {createApp} from 'vue'
import ElementPlus from 'element-plus'
import App from './App.vue'
import { createRouter, createWebHashHistory } from 'vue-router'
import 'element-plus/dist/index.css'
import './styles/tokens.css'
import './styles/element-overrides.css'
import './styles/base.css'
import DHCPDashboard from './components/dhcp/DHCPDashboard.vue'
import TFTPDashboard from './components/tftp/TFTPDashboard.vue'
import SyslogDashboard from './components/syslog/SyslogDashboard.vue'



const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', redirect: '/dhcp' },
    { path: '/dhcp', component: DHCPDashboard },
    { path: '/tftp', component: TFTPDashboard },
    { path: '/syslog', component: SyslogDashboard },
  ],
});


export default router


const app = createApp(App)


app.use(router)
app.use(ElementPlus)
app.mount('#app')