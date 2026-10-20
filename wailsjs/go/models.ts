export namespace backend {
	
	export class Album {
	    id: string;
	    name: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new Album(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.count = source["count"];
	    }
	}
	export class LocalFile {
	    path: string;
	    filename: string;
	    size: number;
	    modTime: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.filename = source["filename"];
	        this.size = source["size"];
	        this.modTime = source["modTime"];
	    }
	}
	export class Conflict {
	    photo: Photo;
	    localPath: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new Conflict(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.photo = this.convertValues(source["photo"], Photo);
	        this.localPath = source["localPath"];
	        this.reason = source["reason"];
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
	export class Photo {
	    id: string;
	    filename: string;
	    date: string;
	    size: number;
	    format: string;
	    width: number;
	    height: number;
	    albumId: string;
	    thumbUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new Photo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.filename = source["filename"];
	        this.date = source["date"];
	        this.size = source["size"];
	        this.format = source["format"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.albumId = source["albumId"];
	        this.thumbUrl = source["thumbUrl"];
	    }
	}
	export class CompareResult {
	    newOnDevice: Photo[];
	    alreadyOnPc: Photo[];
	    conflicts: Conflict[];
	    missingOnPc: LocalFile[];
	    scannedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new CompareResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.newOnDevice = this.convertValues(source["newOnDevice"], Photo);
	        this.alreadyOnPc = this.convertValues(source["alreadyOnPc"], Photo);
	        this.conflicts = this.convertValues(source["conflicts"], Conflict);
	        this.missingOnPc = this.convertValues(source["missingOnPc"], LocalFile);
	        this.scannedAt = source["scannedAt"];
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
	
	export class Device {
	    id: string;
	    name: string;
	    ip: string;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.ip = source["ip"];
	        this.model = source["model"];
	    }
	}
	export class DiscoveryResult {
	    devices: Device[];
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new DiscoveryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.devices = this.convertValues(source["devices"], Device);
	        this.status = source["status"];
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
	
	
	export class TransferError {
	    photoId: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new TransferError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.photoId = source["photoId"];
	        this.reason = source["reason"];
	    }
	}
	export class TransferOptions {
	    saveOriginal: boolean;
	    saveJpg: boolean;
	    jpgQuality: number;
	    destPath: string;
	    overwritePaths: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new TransferOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.saveOriginal = source["saveOriginal"];
	        this.saveJpg = source["saveJpg"];
	        this.jpgQuality = source["jpgQuality"];
	        this.destPath = source["destPath"];
	        this.overwritePaths = source["overwritePaths"];
	    }
	}
	export class TransferResult {
	    succeeded: string[];
	    failed: TransferError[];
	    bytesCopied: number;
	    durationMs: number;
	    cancelled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TransferResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.succeeded = source["succeeded"];
	        this.failed = this.convertValues(source["failed"], TransferError);
	        this.bytesCopied = source["bytesCopied"];
	        this.durationMs = source["durationMs"];
	        this.cancelled = source["cancelled"];
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

