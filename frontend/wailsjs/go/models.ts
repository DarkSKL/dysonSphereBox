export namespace contract {
	
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
	    key: string;
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
	        this.key = source["key"];
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
	export class HeadlessArchiveOption {
	    name: string;
	    sizeBytes: number;
	    lastModifiedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new HeadlessArchiveOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.sizeBytes = source["sizeBytes"];
	        this.lastModifiedAt = source["lastModifiedAt"];
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
	    uptimeSeconds: number;
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
	    serverArchive: string;
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
	        this.uptimeSeconds = source["uptimeSeconds"];
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
	        this.serverArchive = source["serverArchive"];
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
	export class HeadlessLogLine {
	    level: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new HeadlessLogLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.content = source["content"];
	    }
	}
	export class HeadlessLogSnapshot {
	    path: string;
	    exists: boolean;
	    updatedAt: string;
	    truncated: boolean;
	    lines: HeadlessLogLine[];
	
	    static createFrom(source: any = {}) {
	        return new HeadlessLogSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.exists = source["exists"];
	        this.updatedAt = source["updatedAt"];
	        this.truncated = source["truncated"];
	        this.lines = this.convertValues(source["lines"], HeadlessLogLine);
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
	export class HeadlessSettings {
	    hostPort: number;
	    serverArchive: string;
	    nebulaConfigPath: string;
	    status: HeadlessEnvironmentStatus;
	
	    static createFrom(source: any = {}) {
	        return new HeadlessSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hostPort = source["hostPort"];
	        this.serverArchive = source["serverArchive"];
	        this.nebulaConfigPath = source["nebulaConfigPath"];
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
	    hostPort: number;
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
	        this.hostPort = source["hostPort"];
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
	export class InstallConflictDTO {
	    packageKey: string;
	    componentKey: string;
	    componentName: string;
	    componentVersion: string;
	    targetPath: string;
	    relativePath: string;
	    incomingSha256: string;
	    existingSha256: string;
	    existingPackageKey: string;
	    existingPackageName: string;
	    existingComponentKey: string;
	    existingComponentName: string;
	    existingComponentVersion: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new InstallConflictDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packageKey = source["packageKey"];
	        this.componentKey = source["componentKey"];
	        this.componentName = source["componentName"];
	        this.componentVersion = source["componentVersion"];
	        this.targetPath = source["targetPath"];
	        this.relativePath = source["relativePath"];
	        this.incomingSha256 = source["incomingSha256"];
	        this.existingSha256 = source["existingSha256"];
	        this.existingPackageKey = source["existingPackageKey"];
	        this.existingPackageName = source["existingPackageName"];
	        this.existingComponentKey = source["existingComponentKey"];
	        this.existingComponentName = source["existingComponentName"];
	        this.existingComponentVersion = source["existingComponentVersion"];
	        this.message = source["message"];
	    }
	}
	export class InstallProgressSnapshot {
	    packageKey: string;
	    stage: string;
	    status: string;
	    percent: number;
	    current: number;
	    total: number;
	    bytesPerSecond: number;
	    message: string;
	    state: string;
	    errorMessage: string;
	    conflict?: InstallConflictDTO;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new InstallProgressSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packageKey = source["packageKey"];
	        this.stage = source["stage"];
	        this.status = source["status"];
	        this.percent = source["percent"];
	        this.current = source["current"];
	        this.total = source["total"];
	        this.bytesPerSecond = source["bytesPerSecond"];
	        this.message = source["message"];
	        this.state = source["state"];
	        this.errorMessage = source["errorMessage"];
	        this.conflict = this.convertValues(source["conflict"], InstallConflictDTO);
	        this.updatedAt = source["updatedAt"];
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
	export class InstalledComponentSummary {
	    componentKey: string;
	    namespace: string;
	    name: string;
	    displayName: string;
	    version: string;
	    fileCount: number;
	
	    static createFrom(source: any = {}) {
	        return new InstalledComponentSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.componentKey = source["componentKey"];
	        this.namespace = source["namespace"];
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.version = source["version"];
	        this.fileCount = source["fileCount"];
	    }
	}
	export class InstalledPackageSummary {
	    packageKey: string;
	    source: string;
	    namespace: string;
	    name: string;
	    displayName: string;
	    version: string;
	    author: string;
	    description: string;
	    packageUrl: string;
	    installedAt: string;
	    fileCount: number;
	    sizeBytes: number;
	    pluginDirs: string[];
	    components: InstalledComponentSummary[];
	
	    static createFrom(source: any = {}) {
	        return new InstalledPackageSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packageKey = source["packageKey"];
	        this.source = source["source"];
	        this.namespace = source["namespace"];
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.version = source["version"];
	        this.author = source["author"];
	        this.description = source["description"];
	        this.packageUrl = source["packageUrl"];
	        this.installedAt = source["installedAt"];
	        this.fileCount = source["fileCount"];
	        this.sizeBytes = source["sizeBytes"];
	        this.pluginDirs = source["pluginDirs"];
	        this.components = this.convertValues(source["components"], InstalledComponentSummary);
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
	    packageKey: string;
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
	        this.packageKey = source["packageKey"];
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

export namespace diagnostics {
	
	export class Health {
	    directory: string;
	    sessionId: string;
	    fileError: string;
	    fallback: boolean;
	    memoryCount: number;
	    dropped: number;
	    noiseSuppressed: number;
	
	    static createFrom(source: any = {}) {
	        return new Health(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.directory = source["directory"];
	        this.sessionId = source["sessionId"];
	        this.fileError = source["fileError"];
	        this.fallback = source["fallback"];
	        this.memoryCount = source["memoryCount"];
	        this.dropped = source["dropped"];
	        this.noiseSuppressed = source["noiseSuppressed"];
	    }
	}

}

export namespace environment {
	
	export class BlueprintArea {
	    index: number;
	    parentIndex: number;
	    tropicAnchor: number;
	    areaSegments: number;
	    anchorLocalOffsetX: number;
	    anchorLocalOffsetY: number;
	    width: number;
	    height: number;
	
	    static createFrom(source: any = {}) {
	        return new BlueprintArea(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.parentIndex = source["parentIndex"];
	        this.tropicAnchor = source["tropicAnchor"];
	        this.areaSegments = source["areaSegments"];
	        this.anchorLocalOffsetX = source["anchorLocalOffsetX"];
	        this.anchorLocalOffsetY = source["anchorLocalOffsetY"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}
	export class BlueprintPointFloat {
	    x: number;
	    y: number;
	    z: number;
	
	    static createFrom(source: any = {}) {
	        return new BlueprintPointFloat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.z = source["z"];
	    }
	}
	export class BlueprintBuilding {
	    num: number;
	    index: number;
	    itemId: number;
	    itemName: string;
	    modelIndex: number;
	    areaIndex: number;
	    localOffset: BlueprintPointFloat;
	    yaw: number;
	    tilt?: number;
	    pitch?: number;
	    localOffset2?: BlueprintPointFloat;
	    yaw2?: number;
	    tilt2?: number;
	    pitch2?: number;
	    outputObjIdx: number;
	    inputObjIdx: number;
	    outputToSlot: number;
	    inputFromSlot: number;
	    outputFromSlot: number;
	    inputToSlot: number;
	    outputOffset: number;
	    inputOffset: number;
	    recipeId: number;
	    filterId: number;
	    parameters: number[];
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new BlueprintBuilding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.num = source["num"];
	        this.index = source["index"];
	        this.itemId = source["itemId"];
	        this.itemName = source["itemName"];
	        this.modelIndex = source["modelIndex"];
	        this.areaIndex = source["areaIndex"];
	        this.localOffset = this.convertValues(source["localOffset"], BlueprintPointFloat);
	        this.yaw = source["yaw"];
	        this.tilt = source["tilt"];
	        this.pitch = source["pitch"];
	        this.localOffset2 = this.convertValues(source["localOffset2"], BlueprintPointFloat);
	        this.yaw2 = source["yaw2"];
	        this.tilt2 = source["tilt2"];
	        this.pitch2 = source["pitch2"];
	        this.outputObjIdx = source["outputObjIdx"];
	        this.inputObjIdx = source["inputObjIdx"];
	        this.outputToSlot = source["outputToSlot"];
	        this.inputFromSlot = source["inputFromSlot"];
	        this.outputFromSlot = source["outputFromSlot"];
	        this.inputToSlot = source["inputToSlot"];
	        this.outputOffset = source["outputOffset"];
	        this.inputOffset = source["inputOffset"];
	        this.recipeId = source["recipeId"];
	        this.filterId = source["filterId"];
	        this.parameters = source["parameters"];
	        this.content = source["content"];
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
	export class BlueprintPointInt {
	    x: number;
	    y: number;
	
	    static createFrom(source: any = {}) {
	        return new BlueprintPointInt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	    }
	}
	export class BlueprintBinary {
	    version: number;
	    cursorOffset: BlueprintPointInt;
	    cursorTargetArea: number;
	    dragBoxSize: BlueprintPointInt;
	    primaryAreaIdx: number;
	    areas: BlueprintArea[];
	    buildings: BlueprintBuilding[];
	    trailingBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new BlueprintBinary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.cursorOffset = this.convertValues(source["cursorOffset"], BlueprintPointInt);
	        this.cursorTargetArea = source["cursorTargetArea"];
	        this.dragBoxSize = this.convertValues(source["dragBoxSize"], BlueprintPointInt);
	        this.primaryAreaIdx = source["primaryAreaIdx"];
	        this.areas = this.convertValues(source["areas"], BlueprintArea);
	        this.buildings = this.convertValues(source["buildings"], BlueprintBuilding);
	        this.trailingBytes = source["trailingBytes"];
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
	
	export class BlueprintItemGroup {
	    itemId: number;
	    itemName: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new BlueprintItemGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.itemId = source["itemId"];
	        this.itemName = source["itemName"];
	        this.count = source["count"];
	    }
	}
	export class BlueprintSummary {
	    areaCount: number;
	    buildingCount: number;
	    connectionCount: number;
	    parameterizedCount: number;
	    contentCount: number;
	    itemGroups: BlueprintItemGroup[];
	
	    static createFrom(source: any = {}) {
	        return new BlueprintSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.areaCount = source["areaCount"];
	        this.buildingCount = source["buildingCount"];
	        this.connectionCount = source["connectionCount"];
	        this.parameterizedCount = source["parameterizedCount"];
	        this.contentCount = source["contentCount"];
	        this.itemGroups = this.convertValues(source["itemGroups"], BlueprintItemGroup);
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
	export class BlueprintMeta {
	    rawFields: string[];
	    title: string;
	    description: string;
	    gameVersion: string;
	    createdAt: string;
	    checksum: string;
	    icons: string[];
	
	    static createFrom(source: any = {}) {
	        return new BlueprintMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rawFields = source["rawFields"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.gameVersion = source["gameVersion"];
	        this.createdAt = source["createdAt"];
	        this.checksum = source["checksum"];
	        this.icons = source["icons"];
	    }
	}
	export class BlueprintDecodeResult {
	    meta: BlueprintMeta;
	    binary: BlueprintBinary;
	    summary: BlueprintSummary;
	
	    static createFrom(source: any = {}) {
	        return new BlueprintDecodeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.meta = this.convertValues(source["meta"], BlueprintMeta);
	        this.binary = this.convertValues(source["binary"], BlueprintBinary);
	        this.summary = this.convertValues(source["summary"], BlueprintSummary);
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
	
	
	
	
	
	export class EnvironmentBlueprintBreadcrumb {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentBlueprintBreadcrumb(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class EnvironmentBlueprintItem {
	    id: string;
	    parentId: string;
	    type: string;
	    name: string;
	    state: string;
	    sizeBytes: number;
	    updatedAt: string;
	    relativePath: string;
	    extension: string;
	    childCount: number;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentBlueprintItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.parentId = source["parentId"];
	        this.type = source["type"];
	        this.name = source["name"];
	        this.state = source["state"];
	        this.sizeBytes = source["sizeBytes"];
	        this.updatedAt = source["updatedAt"];
	        this.relativePath = source["relativePath"];
	        this.extension = source["extension"];
	        this.childCount = source["childCount"];
	    }
	}
	export class EnvironmentBlueprintDetail {
	    item: EnvironmentBlueprintItem;
	    libraryPath: string;
	    code: string;
	    parsed: BlueprintDecodeResult;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentBlueprintDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.item = this.convertValues(source["item"], EnvironmentBlueprintItem);
	        this.libraryPath = source["libraryPath"];
	        this.code = source["code"];
	        this.parsed = this.convertValues(source["parsed"], BlueprintDecodeResult);
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
	
	export class EnvironmentBlueprintMigrationEntry {
	    itemId: string;
	    name: string;
	    oldRelativePath: string;
	    newRelativePath: string;
	    oldTargetPath: string;
	    newTargetPath: string;
	    conflict: boolean;
	    conflictReason: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentBlueprintMigrationEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.itemId = source["itemId"];
	        this.name = source["name"];
	        this.oldRelativePath = source["oldRelativePath"];
	        this.newRelativePath = source["newRelativePath"];
	        this.oldTargetPath = source["oldTargetPath"];
	        this.newTargetPath = source["newTargetPath"];
	        this.conflict = source["conflict"];
	        this.conflictReason = source["conflictReason"];
	    }
	}
	export class EnvironmentBlueprintMigrationPreview {
	    itemId: string;
	    operation: string;
	    targetParentId: string;
	    targetName: string;
	    requiresConfirmation: boolean;
	    enabledBlueprints: number;
	    conflicts: number;
	    entries: EnvironmentBlueprintMigrationEntry[];
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentBlueprintMigrationPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.itemId = source["itemId"];
	        this.operation = source["operation"];
	        this.targetParentId = source["targetParentId"];
	        this.targetName = source["targetName"];
	        this.requiresConfirmation = source["requiresConfirmation"];
	        this.enabledBlueprints = source["enabledBlueprints"];
	        this.conflicts = source["conflicts"];
	        this.entries = this.convertValues(source["entries"], EnvironmentBlueprintMigrationEntry);
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
	export class EnvironmentBlueprintMigrationRequest {
	    itemId: string;
	    operation: string;
	    targetParentId: string;
	    targetName: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentBlueprintMigrationRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.itemId = source["itemId"];
	        this.operation = source["operation"];
	        this.targetParentId = source["targetParentId"];
	        this.targetName = source["targetName"];
	    }
	}
	export class EnvironmentBlueprintPage {
	    parentId: string;
	    breadcrumbs: EnvironmentBlueprintBreadcrumb[];
	    items: EnvironmentBlueprintItem[];
	    totalBlueprints: number;
	    enabledBlueprints: number;
	    totalFolders: number;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentBlueprintPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.parentId = source["parentId"];
	        this.breadcrumbs = this.convertValues(source["breadcrumbs"], EnvironmentBlueprintBreadcrumb);
	        this.items = this.convertValues(source["items"], EnvironmentBlueprintItem);
	        this.totalBlueprints = source["totalBlueprints"];
	        this.enabledBlueprints = source["enabledBlueprints"];
	        this.totalFolders = source["totalFolders"];
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
	export class EnvironmentInitialSyncPreview {
	    kind: string;
	    mods: number;
	    blueprints: number;
	    saves: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentInitialSyncPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.mods = source["mods"];
	        this.blueprints = source["blueprints"];
	        this.saves = source["saves"];
	        this.message = source["message"];
	    }
	}
	export class EnvironmentInitialSyncRequest {
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentInitialSyncRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	    }
	}
	export class EnvironmentLibraryCounts {
	    mods: number;
	    blueprints: number;
	    blueprintFolders: number;
	    saves: number;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentLibraryCounts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mods = source["mods"];
	        this.blueprints = source["blueprints"];
	        this.blueprintFolders = source["blueprintFolders"];
	        this.saves = source["saves"];
	    }
	}
	export class RootResolution {
	    path: string;
	    source: string;
	    confidence: string;
	    needConfirm: boolean;
	    exists: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new RootResolution(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.source = source["source"];
	        this.confidence = source["confidence"];
	        this.needConfirm = source["needConfirm"];
	        this.exists = source["exists"];
	        this.message = source["message"];
	    }
	}
	export class EnvironmentOverview {
	    appDataRoot: string;
	    modRoot: RootResolution;
	    blueprintRoot: RootResolution;
	    saveRoot: RootResolution;
	    libraryCounts: EnvironmentLibraryCounts;
	    initialSync: EnvironmentInitialSyncPreview;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appDataRoot = source["appDataRoot"];
	        this.modRoot = this.convertValues(source["modRoot"], RootResolution);
	        this.blueprintRoot = this.convertValues(source["blueprintRoot"], RootResolution);
	        this.saveRoot = this.convertValues(source["saveRoot"], RootResolution);
	        this.libraryCounts = this.convertValues(source["libraryCounts"], EnvironmentLibraryCounts);
	        this.initialSync = this.convertValues(source["initialSync"], EnvironmentInitialSyncPreview);
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
	export class EnvironmentInitialSyncResult {
	    kind: string;
	    syncedMods: number;
	    syncedBlueprints: number;
	    indexedSaves: number;
	    failures: string[];
	    overview: EnvironmentOverview;
	    operationId: number;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentInitialSyncResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.syncedMods = source["syncedMods"];
	        this.syncedBlueprints = source["syncedBlueprints"];
	        this.indexedSaves = source["indexedSaves"];
	        this.failures = source["failures"];
	        this.overview = this.convertValues(source["overview"], EnvironmentOverview);
	        this.operationId = source["operationId"];
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
	
	export class EnvironmentManagedStateIssue {
	    itemId: string;
	    kind: string;
	    state: string;
	    reason: string;
	    message: string;
	    relativePath: string;
	    targetPath: string;
	    expectedRoot: string;
	    recordedRoot: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentManagedStateIssue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.itemId = source["itemId"];
	        this.kind = source["kind"];
	        this.state = source["state"];
	        this.reason = source["reason"];
	        this.message = source["message"];
	        this.relativePath = source["relativePath"];
	        this.targetPath = source["targetPath"];
	        this.expectedRoot = source["expectedRoot"];
	        this.recordedRoot = source["recordedRoot"];
	    }
	}
	export class EnvironmentManagedStateSummary {
	    kind: string;
	    checkedRecords: number;
	    checkedItems: number;
	    changedItems: number;
	    conflictItems: number;
	    missingItems: number;
	    issues: EnvironmentManagedStateIssue[];
	    operationId: number;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentManagedStateSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.checkedRecords = source["checkedRecords"];
	        this.checkedItems = source["checkedItems"];
	        this.changedItems = source["changedItems"];
	        this.conflictItems = source["conflictItems"];
	        this.missingItems = source["missingItems"];
	        this.issues = this.convertValues(source["issues"], EnvironmentManagedStateIssue);
	        this.operationId = source["operationId"];
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
	export class EnvironmentModDependency {
	    name: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentModDependency(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	    }
	}
	export class EnvironmentModDetail {
	    id: string;
	    name: string;
	    version: string;
	    author: string;
	    description: string;
	    websiteUrl: string;
	    dependencies: EnvironmentModDependency[];
	    libraryPath: string;
	    supported: boolean;
	    warning: string;
	    rawManifest: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentModDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.author = source["author"];
	        this.description = source["description"];
	        this.websiteUrl = source["websiteUrl"];
	        this.dependencies = this.convertValues(source["dependencies"], EnvironmentModDependency);
	        this.libraryPath = source["libraryPath"];
	        this.supported = source["supported"];
	        this.warning = source["warning"];
	        this.rawManifest = source["rawManifest"];
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
	export class EnvironmentModItem {
	    id: string;
	    name: string;
	    version: string;
	    author: string;
	    description: string;
	    state: string;
	    sizeBytes: number;
	    updatedAt: string;
	    libraryPath: string;
	    supported: boolean;
	    warning: string;
	    installerManaged: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentModItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.author = source["author"];
	        this.description = source["description"];
	        this.state = source["state"];
	        this.sizeBytes = source["sizeBytes"];
	        this.updatedAt = source["updatedAt"];
	        this.libraryPath = source["libraryPath"];
	        this.supported = source["supported"];
	        this.warning = source["warning"];
	        this.installerManaged = source["installerManaged"];
	    }
	}
	export class EnvironmentOperation {
	    id: number;
	    operation: string;
	    kind: string;
	    itemId: string;
	    status: string;
	    message: string;
	    detail: number[];
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentOperation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.operation = source["operation"];
	        this.kind = source["kind"];
	        this.itemId = source["itemId"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.detail = source["detail"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class EnvironmentSaveItem {
	    id: string;
	    name: string;
	    fileName: string;
	    relativePath: string;
	    sizeBytes: number;
	    lastModifiedAt: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentSaveItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.fileName = source["fileName"];
	        this.relativePath = source["relativePath"];
	        this.sizeBytes = source["sizeBytes"];
	        this.lastModifiedAt = source["lastModifiedAt"];
	        this.status = source["status"];
	    }
	}
	export class EnvironmentOperationResult {
	    message: string;
	    operationId: number;
	    mod?: EnvironmentModItem;
	    mods?: EnvironmentModItem[];
	    blueprintPage?: EnvironmentBlueprintPage;
	    saves?: EnvironmentSaveItem[];
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentOperationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	        this.operationId = source["operationId"];
	        this.mod = this.convertValues(source["mod"], EnvironmentModItem);
	        this.mods = this.convertValues(source["mods"], EnvironmentModItem);
	        this.blueprintPage = this.convertValues(source["blueprintPage"], EnvironmentBlueprintPage);
	        this.saves = this.convertValues(source["saves"], EnvironmentSaveItem);
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
	
	export class EnvironmentPathSelection {
	    paths: string[];
	    canceled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentPathSelection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.paths = source["paths"];
	        this.canceled = source["canceled"];
	    }
	}
	export class EnvironmentRecoverySummary {
	    pendingOperations: number;
	    recoveredOperations: number;
	    failedOperations: number;
	    removedPaths: string[];
	    failedPaths: string[];
	    managedState?: EnvironmentManagedStateSummary;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentRecoverySummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pendingOperations = source["pendingOperations"];
	        this.recoveredOperations = source["recoveredOperations"];
	        this.failedOperations = source["failedOperations"];
	        this.removedPaths = source["removedPaths"];
	        this.failedPaths = source["failedPaths"];
	        this.managedState = this.convertValues(source["managedState"], EnvironmentManagedStateSummary);
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
	export class EnvironmentRootDirectoryResult {
	    kind: string;
	    root: RootResolution;
	    overview: EnvironmentOverview;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentRootDirectoryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.root = this.convertValues(source["root"], RootResolution);
	        this.overview = this.convertValues(source["overview"], EnvironmentOverview);
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

export namespace forward {
	
	export class LocalPort {
	    label: string;
	    proto: string;
	    address: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalPort(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.proto = source["proto"];
	        this.address = source["address"];
	    }
	}
	export class TargetState {
	    index: number;
	    proto: string;
	    port: number;
	    address: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new TargetState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.proto = source["proto"];
	        this.port = source["port"];
	        this.address = source["address"];
	        this.status = source["status"];
	    }
	}

}

export namespace logging {
	
	export class Entry {
	    id: string;
	    schemaVersion: number;
	    action: string;
	    level: string;
	    source: string;
	    reason: string;
	    namespace: string;
	    data: Record<string, any>;
	    time: string;
	    handler: string;
	    sessionId: string;
	    operationId: string;
	    appVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.schemaVersion = source["schemaVersion"];
	        this.action = source["action"];
	        this.level = source["level"];
	        this.source = source["source"];
	        this.reason = source["reason"];
	        this.namespace = source["namespace"];
	        this.data = source["data"];
	        this.time = source["time"];
	        this.handler = source["handler"];
	        this.sessionId = source["sessionId"];
	        this.operationId = source["operationId"];
	        this.appVersion = source["appVersion"];
	    }
	}

}

export namespace modDependence {
	
	export class RemoteModDependencyDTO {
	    key: string;
	    source: string;
	    sourceDependencyId: string;
	    namespace: string;
	    name: string;
	    displayName: string;
	    version: string;
	    description: string;
	    downloadUrl: string;
	    packageUrl: string;
	    artifactKind: string;
	    localPath: string;
	    raw: string;
	    deprecated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RemoteModDependencyDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.source = source["source"];
	        this.sourceDependencyId = source["sourceDependencyId"];
	        this.namespace = source["namespace"];
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.downloadUrl = source["downloadUrl"];
	        this.packageUrl = source["packageUrl"];
	        this.artifactKind = source["artifactKind"];
	        this.localPath = source["localPath"];
	        this.raw = source["raw"];
	        this.deprecated = source["deprecated"];
	    }
	}
	export class RemoteModPackageDTO {
	    key: string;
	    source: string;
	    sourcePackageId: string;
	    namespace: string;
	    name: string;
	    displayName: string;
	    version: string;
	    description: string;
	    author: string;
	    downloadUrl: string;
	    packageUrl: string;
	    coverUrl: string;
	    downloadCount: number;
	    dependencyCount: number;
	    packageSize: number;
	    tags: string[];
	    gameVersions: any;
	    trustworthiness: number;
	    updateTime: string;
	    deprecated: boolean;
	    artifactKind: string;
	    localPath: string;
	    dependencies: RemoteModDependencyDTO[];
	    rawDependencySource: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteModPackageDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.source = source["source"];
	        this.sourcePackageId = source["sourcePackageId"];
	        this.namespace = source["namespace"];
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.author = source["author"];
	        this.downloadUrl = source["downloadUrl"];
	        this.packageUrl = source["packageUrl"];
	        this.coverUrl = source["coverUrl"];
	        this.downloadCount = source["downloadCount"];
	        this.dependencyCount = source["dependencyCount"];
	        this.packageSize = source["packageSize"];
	        this.tags = source["tags"];
	        this.gameVersions = source["gameVersions"];
	        this.trustworthiness = source["trustworthiness"];
	        this.updateTime = source["updateTime"];
	        this.deprecated = source["deprecated"];
	        this.artifactKind = source["artifactKind"];
	        this.localPath = source["localPath"];
	        this.dependencies = this.convertValues(source["dependencies"], RemoteModDependencyDTO);
	        this.rawDependencySource = source["rawDependencySource"];
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
	export class RemotePackageListData {
	    list: RemoteModPackageDTO[];
	    total: number;
	    page: number;
	    pageSize: number;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new RemotePackageListData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], RemoteModPackageDTO);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.source = source["source"];
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

export namespace multiplayer {
	
	export class AccountView {
	    loggedIn: boolean;
	    userId: number;
	    nickname: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.loggedIn = source["loggedIn"];
	        this.userId = source["userId"];
	        this.nickname = source["nickname"];
	    }
	}
	export class ActionResult {
	    code: number;
	    msg: string;
	    data: any;
	
	    static createFrom(source: any = {}) {
	        return new ActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.msg = source["msg"];
	        this.data = source["data"];
	    }
	}
	export class Capabilities {
	    nativeDcutr: boolean;
	    relayV2: boolean;
	    relayFallback: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Capabilities(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nativeDcutr = source["nativeDcutr"];
	        this.relayV2 = source["relayV2"];
	        this.relayFallback = source["relayFallback"];
	    }
	}
	export class ConnectionView {
	    state: string;
	    lastErrorCode: number;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.lastErrorCode = source["lastErrorCode"];
	    }
	}
	export class InviteView {
	    code: string;
	    link: string;
	    expiresAtMs: number;
	
	    static createFrom(source: any = {}) {
	        return new InviteView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.link = source["link"];
	        this.expiresAtMs = source["expiresAtMs"];
	    }
	}
	export class LinkState {
	    state: string;
	    path: string;
	    ipFamily: string;
	    rttMs: number;
	    error: number;
	
	    static createFrom(source: any = {}) {
	        return new LinkState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.path = source["path"];
	        this.ipFamily = source["ipFamily"];
	        this.rttMs = source["rttMs"];
	        this.error = source["error"];
	    }
	}
	export class LinkView {
	    userId: number;
	    state: string;
	    path: string;
	    ipFamily: string;
	    rttMs: number;
	    error: number;
	    detail: string;
	    natBlocked: string;
	
	    static createFrom(source: any = {}) {
	        return new LinkView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.userId = source["userId"];
	        this.state = source["state"];
	        this.path = source["path"];
	        this.ipFamily = source["ipFamily"];
	        this.rttMs = source["rttMs"];
	        this.error = source["error"];
	        this.detail = source["detail"];
	        this.natBlocked = source["natBlocked"];
	    }
	}
	export class LobbyListRequest {
	    page: number;
	    pageSize: number;
	    keyword: string;
	    sort: string;
	    order: string;
	
	    static createFrom(source: any = {}) {
	        return new LobbyListRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.keyword = source["keyword"];
	        this.sort = source["sort"];
	        this.order = source["order"];
	    }
	}
	export class PortDecl {
	    proto: string;
	    port: number;
	    count: number;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new PortDecl(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proto = source["proto"];
	        this.port = source["port"];
	        this.count = source["count"];
	        this.label = source["label"];
	    }
	}
	export class RoomSummary {
	    roomId: string;
	    name: string;
	    hostNickname: string;
	    memberCount: number;
	    maxMembers: number;
	    hasPassword: boolean;
	    ports: PortDecl[];
	    createdAtMs: number;
	
	    static createFrom(source: any = {}) {
	        return new RoomSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.roomId = source["roomId"];
	        this.name = source["name"];
	        this.hostNickname = source["hostNickname"];
	        this.memberCount = source["memberCount"];
	        this.maxMembers = source["maxMembers"];
	        this.hasPassword = source["hasPassword"];
	        this.ports = this.convertValues(source["ports"], PortDecl);
	        this.createdAtMs = source["createdAtMs"];
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
	export class LobbyView {
	    watching: boolean;
	    total: number;
	    list: RoomSummary[];
	
	    static createFrom(source: any = {}) {
	        return new LobbyView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.watching = source["watching"];
	        this.total = source["total"];
	        this.list = this.convertValues(source["list"], RoomSummary);
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
	export class Member {
	    userId: number;
	    nickname: string;
	    peerId: string;
	    joinedAtMs: number;
	    link?: LinkState;
	
	    static createFrom(source: any = {}) {
	        return new Member(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.userId = source["userId"];
	        this.nickname = source["nickname"];
	        this.peerId = source["peerId"];
	        this.joinedAtMs = source["joinedAtMs"];
	        this.link = this.convertValues(source["link"], LinkState);
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
	export class NetworkView {
	    upnp: boolean;
	    ipv6: boolean;
	    natConsistent?: boolean;
	    natMappings: string[];
	    observedAddrs: string[];
	    reportedAddrs: p2p.ClassifiedAddr[];
	    observed: p2p.ClassifiedAddr[];
	    observeErrors: string[];
	    hostAddrs: p2p.ClassifiedAddr[];
	    hostNoPublic: boolean;
	    firewallRule: boolean;
	    targets: forward.TargetState[];
	
	    static createFrom(source: any = {}) {
	        return new NetworkView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.upnp = source["upnp"];
	        this.ipv6 = source["ipv6"];
	        this.natConsistent = source["natConsistent"];
	        this.natMappings = source["natMappings"];
	        this.observedAddrs = source["observedAddrs"];
	        this.reportedAddrs = this.convertValues(source["reportedAddrs"], p2p.ClassifiedAddr);
	        this.observed = this.convertValues(source["observed"], p2p.ClassifiedAddr);
	        this.observeErrors = source["observeErrors"];
	        this.hostAddrs = this.convertValues(source["hostAddrs"], p2p.ClassifiedAddr);
	        this.hostNoPublic = source["hostNoPublic"];
	        this.firewallRule = source["firewallRule"];
	        this.targets = this.convertValues(source["targets"], forward.TargetState);
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
	
	export class RoomCreateRequest {
	    name: string;
	    visibility: string;
	    password: string;
	    maxMembers: number;
	    ports: PortDecl[];
	
	    static createFrom(source: any = {}) {
	        return new RoomCreateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.visibility = source["visibility"];
	        this.password = source["password"];
	        this.maxMembers = source["maxMembers"];
	        this.ports = this.convertValues(source["ports"], PortDecl);
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
	export class RoomHost {
	    userId: number;
	    nickname: string;
	    peerId: string;
	
	    static createFrom(source: any = {}) {
	        return new RoomHost(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.userId = source["userId"];
	        this.nickname = source["nickname"];
	        this.peerId = source["peerId"];
	    }
	}
	export class RoomJoinRequest {
	    roomId: string;
	    inviteCode: string;
	    password: string;
	
	    static createFrom(source: any = {}) {
	        return new RoomJoinRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.roomId = source["roomId"];
	        this.inviteCode = source["inviteCode"];
	        this.password = source["password"];
	    }
	}
	export class RoomResolveRequest {
	    roomId: string;
	    inviteCode: string;
	
	    static createFrom(source: any = {}) {
	        return new RoomResolveRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.roomId = source["roomId"];
	        this.inviteCode = source["inviteCode"];
	    }
	}
	export class RoomSnapshot {
	    roomId: string;
	    revision: number;
	    name: string;
	    visibility: string;
	    hasPassword: boolean;
	    maxMembers: number;
	    ports: PortDecl[];
	    createdAtMs: number;
	    host: RoomHost;
	    members: Member[];
	
	    static createFrom(source: any = {}) {
	        return new RoomSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.roomId = source["roomId"];
	        this.revision = source["revision"];
	        this.name = source["name"];
	        this.visibility = source["visibility"];
	        this.hasPassword = source["hasPassword"];
	        this.maxMembers = source["maxMembers"];
	        this.ports = this.convertValues(source["ports"], PortDecl);
	        this.createdAtMs = source["createdAtMs"];
	        this.host = this.convertValues(source["host"], RoomHost);
	        this.members = this.convertValues(source["members"], Member);
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
	
	export class RoomView {
	    snapshot: RoomSnapshot;
	    role: string;
	    invite?: InviteView;
	
	    static createFrom(source: any = {}) {
	        return new RoomView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.snapshot = this.convertValues(source["snapshot"], RoomSnapshot);
	        this.role = source["role"];
	        this.invite = this.convertValues(source["invite"], InviteView);
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
	export class SessionLimits {
	    maxMembers: number;
	    maxPorts: number;
	    punchRounds: number;
	
	    static createFrom(source: any = {}) {
	        return new SessionLimits(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.maxMembers = source["maxMembers"];
	        this.maxPorts = source["maxPorts"];
	        this.punchRounds = source["punchRounds"];
	    }
	}
	export class SessionView {
	    sessionId: string;
	    peerId: string;
	    listenPort: number;
	    limits: SessionLimits;
	    capabilities: Capabilities;
	    relays: p2p.RelayState[];
	
	    static createFrom(source: any = {}) {
	        return new SessionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.peerId = source["peerId"];
	        this.listenPort = source["listenPort"];
	        this.limits = this.convertValues(source["limits"], SessionLimits);
	        this.capabilities = this.convertValues(source["capabilities"], Capabilities);
	        this.relays = this.convertValues(source["relays"], p2p.RelayState);
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
	export class State {
	    stateRevision: number;
	    connection: ConnectionView;
	    account: AccountView;
	    session?: SessionView;
	    lobby: LobbyView;
	    room?: RoomView;
	    links: LinkView[];
	    localPorts: forward.LocalPort[];
	    network: NetworkView;
	    pending: string[];
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stateRevision = source["stateRevision"];
	        this.connection = this.convertValues(source["connection"], ConnectionView);
	        this.account = this.convertValues(source["account"], AccountView);
	        this.session = this.convertValues(source["session"], SessionView);
	        this.lobby = this.convertValues(source["lobby"], LobbyView);
	        this.room = this.convertValues(source["room"], RoomView);
	        this.links = this.convertValues(source["links"], LinkView);
	        this.localPorts = this.convertValues(source["localPorts"], forward.LocalPort);
	        this.network = this.convertValues(source["network"], NetworkView);
	        this.pending = source["pending"];
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

export namespace p2p {
	
	export class ClassifiedAddr {
	    addr: string;
	    display: string;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new ClassifiedAddr(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.addr = source["addr"];
	        this.display = source["display"];
	        this.kind = source["kind"];
	    }
	}
	export class RelayState {
	    peerId: string;
	    expiresAtMs: number;
	    limitDurationSeconds: number;
	    limitDataBytes: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new RelayState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.peerId = source["peerId"];
	        this.expiresAtMs = source["expiresAtMs"];
	        this.limitDurationSeconds = source["limitDurationSeconds"];
	        this.limitDataBytes = source["limitDataBytes"];
	        this.error = source["error"];
	    }
	}

}

export namespace server {
	
	export class ProbeResult {
	    ok: boolean;
	    latencyMs: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.latencyMs = source["latencyMs"];
	        this.message = source["message"];
	    }
	}

}

export namespace system {
	
	export class AccountCaptcha {
	    captchaId: string;
	    picPath: string;
	    captchaLength: number;
	    openCaptcha: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AccountCaptcha(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.captchaId = source["captchaId"];
	        this.picPath = source["picPath"];
	        this.captchaLength = source["captchaLength"];
	        this.openCaptcha = source["openCaptcha"];
	    }
	}
	export class AccountChangePasswordRequest {
	    password: string;
	    newPassword: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountChangePasswordRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.password = source["password"];
	        this.newPassword = source["newPassword"];
	    }
	}
	export class AccountLoginRequest {
	    username: string;
	    password: string;
	    captcha?: string;
	    captchaId?: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountLoginRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.password = source["password"];
	        this.captcha = source["captcha"];
	        this.captchaId = source["captchaId"];
	    }
	}
	export class AccountUser {
	    ID: number;
	    uuid: string;
	    userName: string;
	    nickName: string;
	    headerImg: string;
	    phone: string;
	    email: string;
	    authorityId: number;
	    enable: number;
	    originSetting: Record<string, any>;
	    passwordUpdatedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountUser(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.uuid = source["uuid"];
	        this.userName = source["userName"];
	        this.nickName = source["nickName"];
	        this.headerImg = source["headerImg"];
	        this.phone = source["phone"];
	        this.email = source["email"];
	        this.authorityId = source["authorityId"];
	        this.enable = source["enable"];
	        this.originSetting = source["originSetting"];
	        this.passwordUpdatedAt = source["passwordUpdatedAt"];
	    }
	}
	export class AccountLoginResult {
	    user: AccountUser;
	    expiresAt: number;
	    needChangePassword: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AccountLoginResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.user = this.convertValues(source["user"], AccountUser);
	        this.expiresAt = source["expiresAt"];
	        this.needChangePassword = source["needChangePassword"];
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
	export class AccountRegisterRequest {
	    username: string;
	    password: string;
	    captcha: string;
	    captchaId: string;
	    nickName?: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountRegisterRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.password = source["password"];
	        this.captcha = source["captcha"];
	        this.captchaId = source["captchaId"];
	        this.nickName = source["nickName"];
	    }
	}
	export class AccountSelfInfoRequest {
	    nickName?: string;
	    headerImg?: string;
	    phone?: string;
	    email?: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountSelfInfoRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nickName = source["nickName"];
	        this.headerImg = source["headerImg"];
	        this.phone = source["phone"];
	        this.email = source["email"];
	    }
	}
	export class AccountSessionState {
	    authenticated: boolean;
	    needChangePassword: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AccountSessionState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.authenticated = source["authenticated"];
	        this.needChangePassword = source["needChangePassword"];
	    }
	}
	
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
	export class LogHealth {
	    file: diagnostics.Health;
	    indexError: string;
	    pending: number;
	
	    static createFrom(source: any = {}) {
	        return new LogHealth(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = this.convertValues(source["file"], diagnostics.Health);
	        this.indexError = source["indexError"];
	        this.pending = source["pending"];
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
	export class LogListResult {
	    items: logging.Entry[];
	    total: number;
	    page: number;
	    pageSize: number;
	    warning: string;
	
	    static createFrom(source: any = {}) {
	        return new LogListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], logging.Entry);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.warning = source["warning"];
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
	    namespaces: string[];
	    levels: string[];
	    handlers: string[];
	    namespace: string;
	    level: string;
	    source: string;
	    handler: string;
	    keyword: string;
	    sessionId: string;
	    operationId: string;
	    from: string;
	    to: string;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new LogQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.namespaces = source["namespaces"];
	        this.levels = source["levels"];
	        this.handlers = source["handlers"];
	        this.namespace = source["namespace"];
	        this.level = source["level"];
	        this.source = source["source"];
	        this.handler = source["handler"];
	        this.keyword = source["keyword"];
	        this.sessionId = source["sessionId"];
	        this.operationId = source["operationId"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	    }
	}
	export class TutorialProgressDTO {
	    pageKey: string;
	    tutorialVersion: number;
	    status: string;
	    completed: boolean;
	    skipped: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TutorialProgressDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pageKey = source["pageKey"];
	        this.tutorialVersion = source["tutorialVersion"];
	        this.status = source["status"];
	        this.completed = source["completed"];
	        this.skipped = source["skipped"];
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

