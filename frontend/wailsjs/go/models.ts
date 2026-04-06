export namespace config {
	
	export class APIConfig {
	    BaseURL: string;
	    EmbeddingModel: string;
	    TimeoutSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new APIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.BaseURL = source["BaseURL"];
	        this.EmbeddingModel = source["EmbeddingModel"];
	        this.TimeoutSeconds = source["TimeoutSeconds"];
	    }
	}
	export class LlamaConfig {
	    binary_path: string;
	    port: number;
	    embedding_port: number;
	    ctx_size: number;
	    models_dir: string;
	
	    static createFrom(source: any = {}) {
	        return new LlamaConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.binary_path = source["binary_path"];
	        this.port = source["port"];
	        this.embedding_port = source["embedding_port"];
	        this.ctx_size = source["ctx_size"];
	        this.models_dir = source["models_dir"];
	    }
	}
	export class RemoteAccessConfig {
	    enabled: boolean;
	    port: number;
	
	    static createFrom(source: any = {}) {
	        return new RemoteAccessConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.port = source["port"];
	    }
	}
	export class MemoryConfig {
	    PersistDir: string;
	    TopK: number;
	    MinSimilarity: number;
	
	    static createFrom(source: any = {}) {
	        return new MemoryConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.PersistDir = source["PersistDir"];
	        this.TopK = source["TopK"];
	        this.MinSimilarity = source["MinSimilarity"];
	    }
	}
	export class IdentityConfig {
	    UserName: string;
	    AssistantName: string;
	    Style: string;
	    SystemRole: string;
	    IncognitoPrompt: string;
	
	    static createFrom(source: any = {}) {
	        return new IdentityConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.UserName = source["UserName"];
	        this.AssistantName = source["AssistantName"];
	        this.Style = source["Style"];
	        this.SystemRole = source["SystemRole"];
	        this.IncognitoPrompt = source["IncognitoPrompt"];
	    }
	}
	export class AppConfig {
	    API: APIConfig;
	    Identity: IdentityConfig;
	    Memory: MemoryConfig;
	    RemoteAccess: RemoteAccessConfig;
	    Llama: LlamaConfig;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.API = this.convertValues(source["API"], APIConfig);
	        this.Identity = this.convertValues(source["Identity"], IdentityConfig);
	        this.Memory = this.convertValues(source["Memory"], MemoryConfig);
	        this.RemoteAccess = this.convertValues(source["RemoteAccess"], RemoteAccessConfig);
	        this.Llama = this.convertValues(source["Llama"], LlamaConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	

}

export namespace embed {
	
	export class FS {
	
	
	    static createFrom(source: any = {}) {
	        return new FS(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace llama {
	
	export class GPUInfo {
	    type: string;
	    name: string;
	    vram_mb: number;
	    recommended_layers: number;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new GPUInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.name = source["name"];
	        this.vram_mb = source["vram_mb"];
	        this.recommended_layers = source["recommended_layers"];
	        this.description = source["description"];
	    }
	}
	export class ServerStatus {
	    running: boolean;
	    model_path: string;
	    model_name: string;
	    port: number;
	    pid: number;
	    gpu: GPUInfo;
	
	    static createFrom(source: any = {}) {
	        return new ServerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.model_path = source["model_path"];
	        this.model_name = source["model_name"];
	        this.port = source["port"];
	        this.pid = source["pid"];
	        this.gpu = this.convertValues(source["gpu"], GPUInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace main {
	
	export class ConnectionStatus {
	    connected: boolean;
	    models: string[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.models = source["models"];
	        this.error = source["error"];
	    }
	}
	export class RemoteAccessStatus {
	    enabled: boolean;
	    port: number;
	    running: boolean;
	    addresses: string[];
	
	    static createFrom(source: any = {}) {
	        return new RemoteAccessStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.port = source["port"];
	        this.running = source["running"];
	        this.addresses = source["addresses"];
	    }
	}

}

export namespace memory {
	
	export class GobFileInfo {
	    path: string;
	    name: string;
	    size_kb: number;
	    modified: string;
	
	    static createFrom(source: any = {}) {
	        return new GobFileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size_kb = source["size_kb"];
	        this.modified = source["modified"];
	    }
	}

}

export namespace modelstore {
	
	export class DownloadProgress {
	    active: boolean;
	    repo_id: string;
	    filename: string;
	    total_bytes: number;
	    downloaded: number;
	    percent: number;
	    speed: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.repo_id = source["repo_id"];
	        this.filename = source["filename"];
	        this.total_bytes = source["total_bytes"];
	        this.downloaded = source["downloaded"];
	        this.percent = source["percent"];
	        this.speed = source["speed"];
	    }
	}
	export class GGUFFile {
	    filename: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new GGUFFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filename = source["filename"];
	        this.size = source["size"];
	    }
	}
	export class HFModelResult {
	    id: string;
	    author: string;
	    downloads: number;
	    likes: number;
	    tags: string[];
	
	    static createFrom(source: any = {}) {
	        return new HFModelResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.author = source["author"];
	        this.downloads = source["downloads"];
	        this.likes = source["likes"];
	        this.tags = source["tags"];
	    }
	}
	export class LocalModel {
	    repo_id: string;
	    filename: string;
	    size: number;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalModel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repo_id = source["repo_id"];
	        this.filename = source["filename"];
	        this.size = source["size"];
	        this.path = source["path"];
	    }
	}

}

export namespace sessions {
	
	export class ChatMessage {
	    role: string;
	    content: string;
	    image_path?: string;
	    file_path?: string;
	    timestamp: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	        this.image_path = source["image_path"];
	        this.file_path = source["file_path"];
	        this.timestamp = source["timestamp"];
	    }
	}
	export class SessionInfo {
	    id: string;
	    title: string;
	    created_at: string;
	    updated_at: string;
	    msg_count: number;
	
	    static createFrom(source: any = {}) {
	        return new SessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.msg_count = source["msg_count"];
	    }
	}

}

