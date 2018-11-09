<template>
<div id="app">
    <div class="container center">
        <div class="row">
            <h1 class="branding">Giffer</h1>
        </div>
        <div class="row">
            <sui-form fluid :loading="loading" @submit.prevent="submit">
                <sui-form-field>
                    <label>url</label>
                    <input id="url" name="url" type="url" placeholder="https://youtube.com/..." required v-model="form.url">
                </sui-form-field>
                <sui-form-field>
                    <label>start</label> 
                    <input name="start" type="number" placeholder="120" min="0" required v-model="form.start">
                </sui-form-field>
                <sui-form-field>
                    <label>end</label> 
                    <input name="end" type="number" placeholder="170" min="0" required v-model="form.end">
                </sui-form-field>
                <sui-form-field>
                    <label>width</label> 
                    <input name="width" type="number" placeholder="400" min="0" v-model="form.width">
                </sui-form-field>
                <sui-form-field>
                    <label>height</label> 
                    <input name="height" type="number" placeholder="250" min="0" v-model="form.height">
                </sui-form-field>
                <sui-form-field>
                    <label>fps</label> 
                    <input name="fps" type="number" placeholder="24" min="0" v-model="form.fps">
                </sui-form-field>
                <sui-form-field>
                    <label>quality</label> 
                    <sui-dropdown 
                        name="quality"
                        fluid
                        placeholder="Select quality"
                        selection
                        search
                        :options="qualities"
                        v-model="form.quality"
                    >
                    </sui-dropdown>
                </sui-form-field>
                <sui-form-field class="form-group">
                    <sui-button primary type="submit">Submit</sui-button>
                </sui-form-field>
            </sui-form>
        </div>
        <div v-if="link.length > 0" class="row">
            <sui-image :src="link" size="medium" bordered />
        </div>
        <div class="row">
            <pre v-for="(err, ii) in errors" :key="ii" class="errors">
                {{err}}
            </pre>
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
            qualities: [
                {key: "low", value: 0, text: "Low"},
                {key: "medium", value: 1, text: "Medium"},
                {key: "high", value: 2, text: "High"},
                {key: "Best", value: 3, text: "Best"},
            ],
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
                        this.loading = false
                        updates.close()
                        let data = JSON.parse(msg.data)
                        console.log(data)
                        if (data.error !== undefined) {
                            this.errors.push(data.error)
                            return
                        }
                        console.log("ready to download!")
                        this.link = `http://localhost:8081${file}`
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

.row {
    width: 100%;
    display: flex;
    justify-content: center;
    margin: auto;
}

.branding {
    margin: auto;
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
