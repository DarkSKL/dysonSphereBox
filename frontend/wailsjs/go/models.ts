export namespace modDependence {
	
	export class BepInExStatus {
	    gameDir: string;
	    installed: boolean;
	    bepInExDir: string;
	    missingFiles: string[];
	    coreExists: boolean;
	    preloaderExists: boolean;
	    winHTTPExists: boolean;
	    doorstopExists: boolean;
	    doorstopValid: boolean;
	    installedVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new BepInExStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameDir = source["gameDir"];
	        this.installed = source["installed"];
	        this.bepInExDir = source["bepInExDir"];
	        this.missingFiles = source["missingFiles"];
	        this.coreExists = source["coreExists"];
	        this.preloaderExists = source["preloaderExists"];
	        this.winHTTPExists = source["winHTTPExists"];
	        this.doorstopExists = source["doorstopExists"];
	        this.doorstopValid = source["doorstopValid"];
	        this.installedVersion = source["installedVersion"];
	    }
	}
	export class BepInExPackage {
	    owner: string;
	    name: string;
	    fullName: string;
	    version: string;
	    downloadUrl: string;
	    dependencies: string[];
	
	    static createFrom(source: any = {}) {
	        return new BepInExPackage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.owner = source["owner"];
	        this.name = source["name"];
	        this.fullName = source["fullName"];
	        this.version = source["version"];
	        this.downloadUrl = source["downloadUrl"];
	        this.dependencies = source["dependencies"];
	    }
	}
	export class BepInExInstallResult {
	    gameDir: string;
	    bufferDir: string;
	    installed: boolean;
	    skipped: boolean;
	    message: string;
	    package?: BepInExPackage;
	    zipPath?: string;
	    extractPath?: string;
	    status: BepInExStatus;
	
	    static createFrom(source: any = {}) {
	        return new BepInExInstallResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameDir = source["gameDir"];
	        this.bufferDir = source["bufferDir"];
	        this.installed = source["installed"];
	        this.skipped = source["skipped"];
	        this.message = source["message"];
	        this.package = this.convertValues(source["package"], BepInExPackage);
	        this.zipPath = source["zipPath"];
	        this.extractPath = source["extractPath"];
	        this.status = this.convertValues(source["status"], BepInExStatus);
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

export namespace system {
	
	export class ConfigUnitInfo {
	    name: string;
	    group: string;
	    key: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigUnitInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.group = source["group"];
	        this.key = source["key"];
	    }
	}

}

