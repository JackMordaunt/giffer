<template>
<div id="app">
    <div v-if="loading" class="overlay">
        <div class="dimmer"></div>
        <div class="center">
            <icon name="sync" scale="2" spin></icon>
        </div>
    </div>
    <div class="container center">
        <div class="row">
            <h1 class="branding">Giffer</h1>
        </div>
        <div class="row">
            <form id="#form" name="main-form" class="form" @submit.prevent="submit">
                <span class="form-group">
                    <label>url</label>
                    <input id="url" name="url" type="url" placeholder="https://youtube.com/..." required v-model="form.url">
                </span>
                <span class="form-group">
                    <label>start</label> 
                    <input name="start" type="number" placeholder="120" min="0" required v-model="form.start">
                </span>
                <span class="form-group">
                    <label>end</label> 
                    <input name="end" type="number" placeholder="170" min="0" required v-model="form.end">
                </span>
                <span class="form-group">
                    <label>width</label> 
                    <input name="width" type="number" placeholder="400" min="0" v-model="form.width">
                </span>
                <span class="form-group">
                    <label>height</label> 
                    <input name="height" type="number" placeholder="250" min="0" v-model="form.height">
                </span>
                <span class="form-group">
                    <label>fps</label> 
                    <input name="fps" type="number" placeholder="24" min="0" v-model="form.fps">
                </span>
                <span class="form-group">
                    <label>quality</label> 
                    <select name="quality" v-model="form.quality">
                        <option value="0">Low</option>
                        <option value="1">Medium</option>
                        <option value="2">High</option>
                        <option value="3">Best</option>
                    </select>
                </span>
                <span class="form-group">
                    <button type="submit">Submit</button>
                </span>
            </form>
        </div>
        <div v-if="link.length > 0" class="row">
            <!-- <a download="memer.gif" :href="link">Download!</a> -->
            <img :src="link"/>
        </div>
        <div class="row">
            <pre v-for="(err, ii) in errors" :key="ii" class="errors">
                {{err}}
            </pre>
        </div>
        <div class="row">
        </div>
    </div>
</div>
</template>

<script>
import axios from "axios"
import "vue-awesome/icons"
import Icon from "vue-awesome/components/Icon"

export default {
    name: "app",
    data() {
        return {
            form: {
                url: "",
                start: 120,
                end: 130,
                width: 350,
                height: 0,
                fps: 24,
                quality: 0,
            },
            loading: false,
            errors: [],
            link: "",
        }
    },
    methods: {
        submit() {
            this.loading = true
            // FIXME(jfm): Validate form data the Vue-idiomatic way.
            try {
                this.form.start = Number(this.form.start)
                this.form.end = Number(this.form.end)
                this.form.width = Number(this.form.width)
                this.form.height = Number(this.form.height)
                this.form.fps = Number(this.form.fps)
                this.form.quality = Number(this.form.quality)
            } catch(e) {
                this.errors.push("form values are not valid numbers")
                return 
            }
            axios.post("gifify", this.form)
                .then(resp => {
                    console.log(resp)
                    if (resp.data.error) {
                        this.errors.push(resp.data.error)
                        return
                    }
                    console.log("waiting for download to be ready...")
                    let { file, info } = resp.data
                    let updates = new WebSocket(`ws://localhost:8081${info}`)
                    updates.onmessage = (msg) => {
                        if (msg.error !== undefined) {
                            this.errors.push(msg.error)
                            return
                        }
                        console.log("ready to download!")
                        this.link = `http://localhost:8081${file}`
                        updates.close()
                        this.loading = false
                    }
                   
                })
                .catch(err => {
                    console.log("catching error")
                    this.errors.push(err.response.data)
                })
        }
    },
    components: {
        Icon,
    }
}
</script>

<style>
html {
    padding: 0;
    margin: 0;
}

body {
    padding: 0;
    margin: 0;
    position: fixed;
    min-width: 100vw;
    min-height: 100vh;
    top: 0;
    bottom: 0;
    left: 0;
    right: 0;
}

.container {
    width: 100%;
    height: 100%;
    display: flex;
    flex-wrap: wrap;
    flex-direction: column;
    justify-content: center;
}

.overlay {
    position: absolute;
    width: 100%;
    height: 100%; 
    top: 0;
    left: 0;
    bottom: 0;
    right: 0;
    display: flex;
    z-index: 2;
}

.overlay .dimmer {
    position: absolute;
    top: 0;
    left: 0;
    bottom: 0;
    right: 0;
    background-color: #4e4e4e;
    opacity: 0.7;
    width: 100%;
    height: 100%;
}

.overlay .loader {
    z-index: 4;
    margin: auto;
    width: 100%;
    height: 100%;
}

.row {
    width: 100%;
    display: flex;
    justify-content: center;
    margin: auto;
}

.branding {
    margin: auto;
}

.form-group {
    width: 100%;
    display: flex;
    flex-direction: row;
    padding: 10px 0px 10px 0;
}

.form-group > * {
    width: 100%;
    margin: auto;
}

.form-group label {
    width: 25%;
}

.errors {
    display: flex;
    max-height: 20vh;
    overflow: scroll;
}

.center {
    margin: 0;
    position: absolute;
    top: 50%;
    left: 50%;
    -ms-transform: translate(-50%, -50%);
    transform: translate(-50%, -50%);
}
</style>
