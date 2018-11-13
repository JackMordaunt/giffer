import Vue from "vue"
import App from "./App.vue"
import SuiVue from "semantic-ui-vue"
import "semantic-ui-css/semantic.min.css"
import "normalize.css"

Vue.use(SuiVue)
Vue.config.productionTip = false

new Vue({
    render: h => h(App),
}).$mount('#app')
