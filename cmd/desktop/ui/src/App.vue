<template>
<div id="app" :class="mode">
    <div class="header top-level">
        <h1>Giffer</h1>
    </div>
    <div class="container top-level">
        <div class="flex-grid">
            <div class="col">
                <sui-form 
                    id="input-form"
                    class="top-level"
                    :loading="loading"
                    @submit.prevent="submit"
                >
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
                    <sui-form-field>
                        <sui-button
                            id="input-form-button"
                            primary
                            type="submit"
                        >
                            Create
                        </sui-button>
                    </sui-form-field>
                </sui-form>
            </div>
            <div class="col">
                <div v-if="link.length > 0">
                    <sui-image :src="link" size="medium" bordered />
                </div>
            </div>
            <div class="col">
                <div v-for="(err, ii) in errors" :key="ii">
                    <sui-message error>
                        <sui-message-header>Error</sui-message-header>
                        <pre>{{err}}</pre>
                    </sui-message>
                </div>
            </div>
        </div>
    </div>
</div>
</template>

<script>
import config from "@/config.js"
import axios from "axios"
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
                    // console.log(resp)
                    if (resp.data.error) {
                        this.errors.push(resp.data.error)
                        return
                    }
                    // console.log("waiting for download to be ready...")
                    let { file, info } = resp.data
                    let updates = new WebSocket(`ws://localhost:8081${info}`)
                    updates.onmessage = (msg) => {
                        this.loading = false
                        updates.close()
                        let data = JSON.parse(msg.data)
                        // console.log(data)
                        if (data.error !== undefined) {
                            this.errors.push(data.error)
                            return
                        }
                        // console.log("ready to download!")
                        this.link = `http://localhost:8081${file}`
                    }
                })
                .catch(err => {
                    // console.log("catching error")
                    // console.log(err.response.data)
                    this.errors.push(err.response.data)
                })
        }
    },
    computed: {
        mode() {
            return config.debug ? "wireframe" : ""
        }
    },
    components: {
        Icon,
    }
}
</script>

<style>
/* .wireframe outlines all child elements for debugging. */
.wireframe * {
    outline: 0.5px dotted black;
}

body {
    height: 100vh;
}

#app {
    height: 100vh;
}

#input-form {
    /* height: calc(100vh - 35px);
    overflow-y: auto; */
    padding: 20px 20px 0 20px;
}

#input-form-button {
    margin-bottom: 20px;
}

.flex-grid {
    display: flex;
}

.col {
    flex: 1
}

.flex-grid-thirds {
  display: flex;
  justify-content: space-between;
}

.flex-grid-thirds .col {
  width: 32%;
}

/*  top-level specifies that the element's height is derived from the window
    size and header.
*/
.top-level {
    height: calc(100vh - 35px);
    overflow-y: auto;
}

.top-level.header {
    height: 35px;
    overflow: hidden;
}

@media (max-width: 400px) {
    .flex-grid {
        display: block;
    }
}
</style>
