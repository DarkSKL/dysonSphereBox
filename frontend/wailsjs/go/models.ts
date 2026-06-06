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

