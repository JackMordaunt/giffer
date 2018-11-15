<template>
<div id="app" :class="mode">
    <div id="header" class="top-level">
        <h1>Giffer</h1>
    </div>
    <div class="flex-grid">
        <div class="col top-level">
            <div class="flex-container">
                <div class="row">
                    <div class="flex-item">
                        <sui-form 
                            id="input-form"
                            :loading="loading"
                            @submit.prevent="submit"
                        >
                            <sui-form-field>
                                <label>url</label>
                                <input id="url" name="url" type="url" placeholder="https://youtube.com/watch..." required v-model="form.url">
                            </sui-form-field>
                            <sui-form-fields unstackable>
                                <sui-form-field>
                                    <label>start</label> 
                                    <input type="number" step="0.1" name="start"  :min="0" required v-model="form.start" />
                                </sui-form-field>
                                <sui-form-field>
                                    <label>end</label> 
                                    <input type="number" step="0.1" name="end"  :min="0" required v-model="form.end" />
                                </sui-form-field>
                            </sui-form-fields>
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
                            <sui-form-fields unstackable>
                                <sui-form-field>
                                    <label>width</label> 
                                    <input type="number" step="0.1" name="width" :min="0" v-model="form.width" />
                                </sui-form-field>
                                <sui-form-field>
                                    <label>height</label> 
                                    <input type="number" step="0.1" name="height" :min="0" v-model="form.height" />
                                </sui-form-field>
                            </sui-form-fields>
                            <sui-form-field>
                                <label>fps</label> 
                                <input type="number" step="0.1" name="fps" :min="0" v-model="form.fps" />
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
                </div>
            </div>
        </div>
        <div class="col">
            <div class="flex-container">
                <div v-if="link.length > 0" class="row">
                    <div class="flex-item">
                        <sui-image :src="link" bordered/>
                    </div>
                </div>
            </div>
        </div>
        <div class="snackbar" v-if="messages.active.length > 0">
            <div v-for="(msg, ii) in messages.active" :key="msg.id" class="row">
                <sui-message :error="msg.isError" dismissable @dismiss="deleteMessage(ii)">
                    <sui-message-header>{{msg.name}}</sui-message-header>
                    <pre class="message-content">{{msg.description}}</pre>
                </sui-message>
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
                start: 0,
                end: 10,
                fps: 15,
                width: 350,
                height: 0,
                quality: 0,
            },
            loading: false,
            messages: {
                active: [],
                id: 0,
            },
            link: "",
        }
    },
    methods: {
        submit() {
            this.loading = true
            // FIXME(jfm): Validate form data the Vue-idiomatic way.
            this.form.keys().forEach(v => {
                try {
                    this.$set(this.form, v, Number(this.form[v]))
                } catch(err) {
                    this.$set(this.form, v, 0)
                    this.pushMessage({
                        name: "Form Validation",
                        description: `${v} should be a number, got ${typeof this.form[v]}`,
                        isError: true,
                    })
                }
            })
            axios.post("gifify", this.form)
                .then(resp => {
                    // console.log(resp)
                    if (resp.data.error) {
                        this.pushMessage({
                            name: "Application Error",
                            description: resp.data.error,
                            isError: true,
                        })
                        return
                    }
                    // console.log("waiting for download to be ready...")
                    let { file, info } = resp.data
                    let updates = new WebSocket(((window.location.protocol === "https:") ? "wss://" : "ws://") + window.location.host + info)

                    updates.onmessage = (msg) => {
                        this.loading = false
                        updates.close()
                        let data = JSON.parse(msg.data)
                        // console.log(data)
                        if (data.error !== undefined) {
                            this.pushMessage({
                                name: "Making Gif",
                                description: data.error,
                                isError: true,
                            })
                            return
                        }
                        // console.log("ready to download!")
                        this.link = `${file}`
                    }
                })
                .catch(err => {
                    // console.log("catching error")
                    // console.log(err.response.data)
                    this.pushMessage({
                        name: "Calling API",
                        description: err.response.data,
                        isError: true,
                    })
                })
        },
        deleteMessage(ii) {
            this.messages.active.splice(ii, 1)
        },
        pushMessage(msg) {
            msg.id = this.messages.id++
            this.messages.active.push(msg)
            if (msg.isErr) {
                throw msg
            }
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
    margin: 20px 20px 0 20px;
    text-align: left;
}

#input-form .inline.fields {
    padding: 0;
    display: flex;
}

#input-form .inline.fields .field {
    padding: 0;
    flex: 1;
}

#input-form-button {
    margin-bottom: 20px;
}

#header {
    background-color: black;
    color: white;
}

#header > * {
    margin: 0 20px;
}

.flex-container {
    height: 100%;
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
}

.row {
    width: auto;
}

.flex-item {
    text-align: center;
}

.flex-grid {
    display: flex;
}

.col {
    flex: 1
}

/*  top-level specifies that the element's height is derived from the window
    size and header.
*/
.top-level {
    height: calc(100vh - 35px);
    overflow-y: auto;
}

.top-level#header {
    height: 35px;
    overflow: hidden;
}

@media (max-width: 400px) {
    .flex-grid {
        display: block;
    }
    .top-level {
        display: block;
        padding-top: 35px;
        height: 100%;
    }
    .top-level#header {
        padding-top: 0;
        top: 0;
        position: fixed;
        height: 35px;
        width: 100vw;
        z-index: 200;
    }
}

.snackbar {
    position: absolute;
    bottom: 35px;
    margin: auto;
    right: 25%;
    left: 25%;
    z-index: 201;
}

.message-content {
    overflow: auto;
    white-space: pre-wrap;
    max-height: 100px;
}
</style>
