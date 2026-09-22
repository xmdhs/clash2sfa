async function compressString(str) {
    const stream = new ReadableStream({
        start(c) {
            c.enqueue(new TextEncoder().encode(str));
            c.close();
        },
    }).pipeThrough(new CompressionStream("deflate"));
    return new Uint8Array(await new Response(stream).arrayBuffer());
}

async function decompressString(compressedData) {
    const stream = new ReadableStream({
        start(c) {
            c.enqueue(compressedData);
            c.close();
        },
    }).pipeThrough(new DecompressionStream("deflate"));
    return new TextDecoder().decode(await new Response(stream).arrayBuffer());
}

// 原生 Uint8Array base64 还是新特性，旧浏览器不支持时才动态加载兜底
const hasNativeBase64 = typeof Uint8Array.fromBase64 === "function" &&
    typeof Uint8Array.prototype.toBase64 === "function";

let jsBase64Promise = null;
function loadJsBase64() {
    if (!jsBase64Promise) {
        jsBase64Promise = import("https://cdn.jsdelivr.net/npm/js-base64@3.9.4/+esm").then(
            (mod) => mod.Base64 ?? mod.default?.Base64 ?? mod.default ?? mod,
        );
    }
    return jsBase64Promise;
}

async function decodeBase64Url(value) {
    if (hasNativeBase64) {
        return Uint8Array.fromBase64(value, { alphabet: "base64url" });
    }
    const Base64 = await loadJsBase64();
    return Base64.toUint8Array(value);
}

async function encodeBase64Url(value) {
    if (hasNativeBase64) {
        return value.toBase64({ alphabet: "base64url", omitPadding: true });
    }
    const Base64 = await loadJsBase64();
    return Base64.fromUint8Array(value, true);
}

const CONFIG_TEMPLATES = {
    0: { configurl: "config.json.template", outFields: "1" },
    1: { configurl: "config.json-1.11.0+.template", outFields: "0" },
    4: { configurl: "config.json-1.12.0+.template", outFields: "0" },
    5: { configurl: "config.json-1.14.0+.template", outFields: "0" },
};

class Clash2SfaApp extends HTMLElement {
    oldConfig = "";
    abortController = null;

    connectedCallback() {
        if (this.initialized) return;
        this.initialized = true;

        this.sub = this.querySelector('[data-ref="sub"]');
        this.include = this.querySelector('[data-ref="include"]');
        this.exclude = this.querySelector('[data-ref="exclude"]');
        this.config = this.querySelector('[data-ref="config"]');
        this.configurl = this.querySelector('[data-ref="config-url"]');
        this.configType = this.querySelector('[data-ref="config-type"]');
        this.disableUrlTest = this.querySelector('[data-ref="disable-url-test"]');
        this.addTag = this.querySelector('[data-ref="add-tag"]');
        this.outFields = this.querySelector('[data-ref="out-fields"]');
        this.fetchProgress = this.querySelector('[data-ref="in-fetch"]');
        this.newSub = this.querySelector('[data-ref="new-sub"]');
        this.convert = this.querySelector('[data-ref="convert"]');

        this.abortController = new AbortController();
        const { signal } = this.abortController;
        this.convert.addEventListener("click", this.handleClick, { signal });
        this.configType.addEventListener("change", this.onConfigTypeChange, { signal });
        document.addEventListener("paste", this.handlePaste, { signal });

        this.updateConfigVisibility();
        this.loadDefaultConfig();
    }

    disconnectedCallback() {
        if (!this.initialized) return;
        this.abortController?.abort();
        this.abortController = null;
        this.initialized = false;
    }

    async loadDefaultConfig() {
        try {
            const version = document.querySelector('meta[name="app-version"]')?.content ?? "";
            const response = await fetch("/config/config.json-1.14.0+.template?" + version);
            this.config.value = await response.text();
            this.oldConfig = this.config.value;
        } catch (error) {
            this.config.value = "";
            console.warn(error);
        }
    }

    updateConfigVisibility() {
        this.config.hidden = this.configType.value !== "2";
        this.configurl.hidden = this.configType.value !== "3";
    }

    setFetching(value) {
        this.fetchProgress.hidden = !value;
        this.convert.hidden = value;
    }

    async saveParameter() {
        const subUrl = new URL(location.origin);
        subUrl.pathname = "/sub";
        const config = this.config.value !== this.oldConfig ? this.config.value : "";
        if (config !== "") {
            subUrl.searchParams.set("config", await encodeBase64Url(await compressString(config)));
        }
        if (this.configurl.value) subUrl.searchParams.set("configurl", this.configurl.value);
        if (this.include.value) subUrl.searchParams.set("include", this.include.value);
        if (this.exclude.value) subUrl.searchParams.set("exclude", this.exclude.value);
        if (this.addTag.checked) subUrl.searchParams.set("addTag", "true");
        if (this.disableUrlTest.checked) subUrl.searchParams.set("disableUrlTest", "true");
        if (this.outFields.value) subUrl.searchParams.set("outFields", this.outFields.value);
        subUrl.searchParams.set("sub", this.sub.value.trim());
        return subUrl.toString();
    }

    handleClick = async () => {
        if (this.sub.value.trim() === "" || !this.fetchProgress.hidden) return "";
        this.newSub.value = "";
        this.newSub.hidden = true;
        this.setFetching(true);
        try {
            const subURL = await this.saveParameter();
            const response = await fetch(subURL);
            if (!response.ok) {
                const message = await response.text();
                this.newSub.value = message;
                this.newSub.hidden = false;
                console.warn(message);
                alert("错误 " + message);
                return;
            }
            this.newSub.value = subURL;
            this.newSub.hidden = false;
            this.newSub.scrollIntoView({ behavior: "smooth" });
            this.newSub.select();
            try {
                await navigator.clipboard.writeText(subURL);
            } catch (error) {
                console.warn(error);
            }
            const sing = new URL("sing-box://import-remote-profile");
            sing.searchParams.set("url", subURL);
            window.location.href = sing.toString();
        } catch (error) {
            console.warn(error);
            alert(String(error));
        } finally {
            this.setFetching(false);
        }
    };

    handlePaste = async (event) => {
        const text = event.clipboardData?.getData("text")?.trim();
        if (!text) return;
        let url;
        try {
            url = new URL(text);
        } catch {
            return;
        }
        if (url.pathname !== "/sub") return;
        if (!confirm("解析粘贴的订阅链接？")) return;
        try {
            const config = url.searchParams.get("config");
            if (config) {
                this.configType.value = "2";
                this.config.value = await decompressString(await decodeBase64Url(config));
            }
            const configurl = url.searchParams.get("configurl");
            if (configurl) {
                this.configurl.value = configurl;
                this.config.value = this.oldConfig;
                this.configType.value = "3";
            } else {
                this.configurl.value = "";
            }
            this.include.value = url.searchParams.get("include") || this.include.value;
            this.exclude.value = url.searchParams.get("exclude") || this.exclude.value;
            this.sub.value = url.searchParams.get("sub") || this.sub.value;
            this.addTag.checked = url.searchParams.get("addTag") === "true";
            this.disableUrlTest.checked = url.searchParams.get("disableUrlTest") === "true";
            this.outFields.value = url.searchParams.get("outFields") || this.outFields.value;
            this.updateConfigVisibility();
        } catch (error) {
            console.log(error);
        }
    };

    onConfigTypeChange = () => {
        const { value } = this.configType;
        this.outFields.value = "";
        if (value !== "2") this.config.value = "";
        if (value !== "3") this.configurl.value = "";
        const preset = CONFIG_TEMPLATES[value];
        if (preset) {
            this.configurl.value = preset.configurl;
            this.outFields.value = preset.outFields;
        }
        if (value === "2" && this.config.value === "") this.config.value = this.oldConfig;
        this.updateConfigVisibility();
    };
}

customElements.define("clash2sfa-app", Clash2SfaApp);
