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
	
	
	export class BepInExUninstallResult {
	    gameDir: string;
	    removed: boolean;
	    skipped: boolean;
	    message: string;
	    removedPaths: string[];
	    clearedRecords: boolean;
	    status: BepInExStatus;
	
	    static createFrom(source: any = {}) {
	        return new BepInExUninstallResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameDir = source["gameDir"];
	        this.removed = source["removed"];
	        this.skipped = source["skipped"];
	        this.message = source["message"];
	        this.removedPaths = source["removedPaths"];
	        this.clearedRecords = source["clearedRecords"];
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
	export class DependencyInstallResult {
	    id: number;
	    name: string;
	    version: string;
	    pluginDir: string;
	    zipPath: string;
	    installed: boolean;
	    removed: boolean;
	    skipped: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new DependencyInstallResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.pluginDir = source["pluginDir"];
	        this.zipPath = source["zipPath"];
	        this.installed = source["installed"];
	        this.removed = source["removed"];
	        this.skipped = source["skipped"];
	        this.message = source["message"];
	    }
	}
	export class HeadlessDependencyStatus {
	    id: string;
	    name: string;
	    folder: string;
	    installedVersion: string;
	    description: string;
	    function: string;
	    source: string;
	    pluginDir: string;
	    manifestPath: string;
	    installed: boolean;
	    missingFiles: string[];
	
	    static createFrom(source: any = {}) {
	        return new HeadlessDependencyStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.folder = source["folder"];
	        this.installedVersion = source["installedVersion"];
	        this.description = source["description"];
	        this.function = source["function"];
	        this.source = source["source"];
	        this.pluginDir = source["pluginDir"];
	        this.manifestPath = source["manifestPath"];
	        this.installed = source["installed"];
	        this.missingFiles = source["missingFiles"];
	    }
	}
	export class HeadlessNetworkSample {
	    label: string;
	    capturedAtUnix: number;
	    uploadMBps: number;
	    downloadMBps: number;
	
	    static createFrom(source: any = {}) {
	        return new HeadlessNetworkSample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.capturedAtUnix = source["capturedAtUnix"];
	        this.uploadMBps = source["uploadMBps"];
	        this.downloadMBps = source["downloadMBps"];
	    }
	}
	export class HeadlessNetworkStatus {
	    valid: boolean;
	    uploadBytesPerSecond: number;
	    downloadBytesPerSecond: number;
	    uploadMBps: number;
	    downloadMBps: number;
	    uploadPeakMBps: number;
	    downloadPeakMBps: number;
	    sampleWindowSeconds: number;
	    samples: HeadlessNetworkSample[];
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new HeadlessNetworkStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.valid = source["valid"];
	        this.uploadBytesPerSecond = source["uploadBytesPerSecond"];
	        this.downloadBytesPerSecond = source["downloadBytesPerSecond"];
	        this.uploadMBps = source["uploadMBps"];
	        this.downloadMBps = source["downloadMBps"];
	        this.uploadPeakMBps = source["uploadPeakMBps"];
	        this.downloadPeakMBps = source["downloadPeakMBps"];
	        this.sampleWindowSeconds = source["sampleWindowSeconds"];
	        this.samples = this.convertValues(source["samples"], HeadlessNetworkSample);
	        this.error = source["error"];
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
	export class HeadlessEnvironmentStatus {
	    gameDir: string;
	    pluginRoot: string;
	    bepInExStatus: BepInExStatus;
	    dependencies: HeadlessDependencyStatus[];
	    nebulaReady: boolean;
	    steamAppIdPath: string;
	    steamAppIdExists: boolean;
	    steamAppIdValid: boolean;
	    hostPort: number;
	    nebulaConfigPath: string;
	    processRunning: boolean;
	    processId: number;
	    tcpListening: boolean;
	    udpListening: boolean;
	    portListening: boolean;
	    startCommand: string[];
	    startCommandText: string;
	    bepInExLogPath: string;
	    outputLogPath: string;
	    network: HeadlessNetworkStatus;
	    issues: string[];
	    ready: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HeadlessEnvironmentStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameDir = source["gameDir"];
	        this.pluginRoot = source["pluginRoot"];
	        this.bepInExStatus = this.convertValues(source["bepInExStatus"], BepInExStatus);
	        this.dependencies = this.convertValues(source["dependencies"], HeadlessDependencyStatus);
	        this.nebulaReady = source["nebulaReady"];
	        this.steamAppIdPath = source["steamAppIdPath"];
	        this.steamAppIdExists = source["steamAppIdExists"];
	        this.steamAppIdValid = source["steamAppIdValid"];
	        this.hostPort = source["hostPort"];
	        this.nebulaConfigPath = source["nebulaConfigPath"];
	        this.processRunning = source["processRunning"];
	        this.processId = source["processId"];
	        this.tcpListening = source["tcpListening"];
	        this.udpListening = source["udpListening"];
	        this.portListening = source["portListening"];
	        this.startCommand = source["startCommand"];
	        this.startCommandText = source["startCommandText"];
	        this.bepInExLogPath = source["bepInExLogPath"];
	        this.outputLogPath = source["outputLogPath"];
	        this.network = this.convertValues(source["network"], HeadlessNetworkStatus);
	        this.issues = source["issues"];
	        this.ready = source["ready"];
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
	
	
	export class HeadlessPrepareResult {
	    gameDir: string;
	    changed: boolean;
	    message: string;
	    writtenPath: string[];
	    status: HeadlessEnvironmentStatus;
	
	    static createFrom(source: any = {}) {
	        return new HeadlessPrepareResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameDir = source["gameDir"];
	        this.changed = source["changed"];
	        this.message = source["message"];
	        this.writtenPath = source["writtenPath"];
	        this.status = this.convertValues(source["status"], HeadlessEnvironmentStatus);
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
	export class HeadlessStartResult {
	    gameDir: string;
	    started: boolean;
	    skipped: boolean;
	    processId: number;
	    message: string;
	    startCommandText: string;
	    status: HeadlessEnvironmentStatus;
	
	    static createFrom(source: any = {}) {
	        return new HeadlessStartResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameDir = source["gameDir"];
	        this.started = source["started"];
	        this.skipped = source["skipped"];
	        this.processId = source["processId"];
	        this.message = source["message"];
	        this.startCommandText = source["startCommandText"];
	        this.status = this.convertValues(source["status"], HeadlessEnvironmentStatus);
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
	export class HeadlessStopResult {
	    gameDir: string;
	    stopped: boolean;
	    skipped: boolean;
	    processId: number;
	    message: string;
	    status: HeadlessEnvironmentStatus;
	
	    static createFrom(source: any = {}) {
	        return new HeadlessStopResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameDir = source["gameDir"];
	        this.stopped = source["stopped"];
	        this.skipped = source["skipped"];
	        this.processId = source["processId"];
	        this.message = source["message"];
	        this.status = this.convertValues(source["status"], HeadlessEnvironmentStatus);
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
	export class PackageInstallResult {
	    packageId: number;
	    packageName: string;
	    gameDir: string;
	    pluginRoot: string;
	    installed: boolean;
	    uninstalled: boolean;
	    skipped: boolean;
	    message: string;
	    dependencies: DependencyInstallResult[];
	    bepInExStatus: BepInExStatus;
	
	    static createFrom(source: any = {}) {
	        return new PackageInstallResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packageId = source["packageId"];
	        this.packageName = source["packageName"];
	        this.gameDir = source["gameDir"];
	        this.pluginRoot = source["pluginRoot"];
	        this.installed = source["installed"];
	        this.uninstalled = source["uninstalled"];
	        this.skipped = source["skipped"];
	        this.message = source["message"];
	        this.dependencies = this.convertValues(source["dependencies"], DependencyInstallResult);
	        this.bepInExStatus = this.convertValues(source["bepInExStatus"], BepInExStatus);
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

export namespace response {
	
	export class ModDependencyDTO {
	    id: number;
	    modName?: string;
	    modVersion?: string;
	    modDescription?: string;
	    gameVersions: any;
	    updateTime?: string;
	    source?: string;
	    url?: string;
	    trustworthiness?: number;
	    config?: string;
	    ex?: string;
	
	    static createFrom(source: any = {}) {
	        return new ModDependencyDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.modName = source["modName"];
	        this.modVersion = source["modVersion"];
	        this.modDescription = source["modDescription"];
	        this.gameVersions = source["gameVersions"];
	        this.updateTime = source["updateTime"];
	        this.source = source["source"];
	        this.url = source["url"];
	        this.trustworthiness = source["trustworthiness"];
	        this.config = source["config"];
	        this.ex = source["ex"];
	    }
	}
	export class ModDependencyPackageDTO {
	    id: number;
	    packageName?: string;
	    packageVersion?: string;
	    packageDescription?: string;
	    packageSize?: number;
	    downloadCount?: number;
	    dependencyCount?: number;
	    gameVersions: any;
	    tags: any;
	    source?: string;
	    url?: string;
	    coverUrl?: string;
	    author?: string;
	    status?: number;
	    trustworthiness?: number;
	    updateTime?: string;
	    config?: string;
	    ex?: string;
	    dependencies: ModDependencyDTO[];
	
	    static createFrom(source: any = {}) {
	        return new ModDependencyPackageDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.packageName = source["packageName"];
	        this.packageVersion = source["packageVersion"];
	        this.packageDescription = source["packageDescription"];
	        this.packageSize = source["packageSize"];
	        this.downloadCount = source["downloadCount"];
	        this.dependencyCount = source["dependencyCount"];
	        this.gameVersions = source["gameVersions"];
	        this.tags = source["tags"];
	        this.source = source["source"];
	        this.url = source["url"];
	        this.coverUrl = source["coverUrl"];
	        this.author = source["author"];
	        this.status = source["status"];
	        this.trustworthiness = source["trustworthiness"];
	        this.updateTime = source["updateTime"];
	        this.config = source["config"];
	        this.ex = source["ex"];
	        this.dependencies = this.convertValues(source["dependencies"], ModDependencyDTO);
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
	export class PackageListData {
	    list: ModDependencyPackageDTO[];
	    total: number;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new PackageListData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], ModDependencyPackageDTO);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
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
	export class CreateLogRequest {
	    namespace: string;
	    module: string;
	    event: string;
	    level: number;
	    type: number;
	    content: string;
	    metadata: Record<string, any>;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateLogRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.namespace = source["namespace"];
	        this.module = source["module"];
	        this.event = source["event"];
	        this.level = source["level"];
	        this.type = source["type"];
	        this.content = source["content"];
	        this.metadata = source["metadata"];
	        this.error = source["error"];
	    }
	}
	export class LogDTO {
	    id: number;
	    namespace: string;
	    module: string;
	    event: string;
	    level: number;
	    type: number;
	    content: string;
	    metadata: Record<string, any>;
	    error: string;
	    dateTime: string;
	
	    static createFrom(source: any = {}) {
	        return new LogDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.namespace = source["namespace"];
	        this.module = source["module"];
	        this.event = source["event"];
	        this.level = source["level"];
	        this.type = source["type"];
	        this.content = source["content"];
	        this.metadata = source["metadata"];
	        this.error = source["error"];
	        this.dateTime = source["dateTime"];
	    }
	}
	export class LogListResult {
	    items: LogDTO[];
	    total: number;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new LogListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], LogDTO);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
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
	export class LogQuery {
	    namespace: string;
	    module: string;
	    event: string;
	    level: number;
	    type: number;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new LogQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.namespace = source["namespace"];
	        this.module = source["module"];
	        this.event = source["event"];
	        this.level = source["level"];
	        this.type = source["type"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	    }
	}

}

export namespace time {
	
	export class Time {
	
	
	    static createFrom(source: any = {}) {
	        return new Time(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace updater {
	
	export class CheckResult {
	    currentVersion: string;
	    latestVersion: string;
	    hasUpdate: boolean;
	    assetName: string;
	    assetUrl: string;
	    releaseName: string;
	    releaseNotes: string;
	    releaseUrl: string;
	    publishedAt: string;
	    os: string;
	    arch: string;
	
	    static createFrom(source: any = {}) {
	        return new CheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.hasUpdate = source["hasUpdate"];
	        this.assetName = source["assetName"];
	        this.assetUrl = source["assetUrl"];
	        this.releaseName = source["releaseName"];
	        this.releaseNotes = source["releaseNotes"];
	        this.releaseUrl = source["releaseUrl"];
	        this.publishedAt = source["publishedAt"];
	        this.os = source["os"];
	        this.arch = source["arch"];
	    }
	}
	export class UpdateResult {
	    updated: boolean;
	    targetVersion: string;
	    message: string;
	    check: CheckResult;
	
	    static createFrom(source: any = {}) {
	        return new UpdateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.updated = source["updated"];
	        this.targetVersion = source["targetVersion"];
	        this.message = source["message"];
	        this.check = this.convertValues(source["check"], CheckResult);
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

