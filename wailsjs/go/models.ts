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

}

