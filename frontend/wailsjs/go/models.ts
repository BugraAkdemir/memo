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
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.API = this.convertValues(source["API"], APIConfig);
	        this.Identity = this.convertValues(source["Identity"], IdentityConfig);
	        this.Memory = this.convertValues(source["Memory"], MemoryConfig);
	        this.RemoteAccess = this.convertValues(source["RemoteAccess"], RemoteAccessConfig);
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

