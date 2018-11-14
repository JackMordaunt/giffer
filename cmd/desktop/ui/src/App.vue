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
                                    <input name="start" type="number" placeholder="120" min="0" required v-model="form.start">
                                </sui-form-field>
                                <sui-form-field>
                                    <label>end</label> 
                                    <input name="end" type="number" placeholder="170" min="0" required v-model="form.end">
                                </sui-form-field>
                            </sui-form-fields>
                            <sui-form-fields unstackable>
                                <sui-form-field>
                                    <label>width</label> 
                                    <input name="width" type="number" placeholder="400" min="0" v-model="form.width">
                                </sui-form-field>
                                <sui-form-field>
                                    <label>height</label> 
                                    <input name="height" type="number" placeholder="250" min="0" v-model="form.height">
                                </sui-form-field>
                            </sui-form-fields>
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
                    <pre>{{msg.description}}</pre>
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
                start: 120,
                end: 130,
                width: 350,
                height: 0,
                fps: 24,
                quality: 0,
            },
            loading: false,
            messages: {
                active: [
                    {
                        name: "Test Error",
                        description: "This is an error.",
                        isError: true,
                        id: 0,
                    }
                ],
                id: 0,
            },
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
                this.pushMessage({
                    name: "Form Validation",
                    description: "form values are not valid numbers",
                    isError: true,
                })
                return 
            }
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
                    let updates = new WebSocket(`ws://localhost:8081${info}`)
                    updates.onmessage = (msg) => {
                        this.loading = false
                        updates.close()
                        let data = JSON.parse(msg.data)
                        // console.log(data)
                        if (data.error !== undefined) {
                            this.pushMessage({
                                name: "Websocket Error",
                                description: data.error,
                                isError: true,
                            })
                            return
                        }
                        // console.log("ready to download!")
                        this.link = `http://localhost:8081${file}`
                    }
                })
                .catch(err => {
                    // console.log("catching error")
                    // console.log(err.response.data)
                    this.pushMessage({
                        name: "API Error",
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
</style>
