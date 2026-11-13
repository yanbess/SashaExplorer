export namespace files {
	
	export class Details {
	    name: string;
	    path: string;
	    dir: boolean;
	    size: number;
	    modified: number;
	    created: number;
	    ext: string;
	    hidden: boolean;
	    system: boolean;
	    link: boolean;
	    original?: string;
	    deleted?: number;
	    accessed: number;
	    readOnly: boolean;
	    archive: boolean;
	    items: number;
	
	    static createFrom(source: any = {}) {
	        return new Details(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.dir = source["dir"];
	        this.size = source["size"];
	        this.modified = source["modified"];
	        this.created = source["created"];
	        this.ext = source["ext"];
	        this.hidden = source["hidden"];
	        this.system = source["system"];
	        this.link = source["link"];
	        this.original = source["original"];
	        this.deleted = source["deleted"];
	        this.accessed = source["accessed"];
	        this.readOnly = source["readOnly"];
	        this.archive = source["archive"];
	        this.items = source["items"];
	    }
	}
	export class Drive {
	    path: string;
	    label: string;
	    kind: string;
	    total: number;
	    free: number;
	    ready: boolean;
	    fileSystem: string;
	    serial: string;
	
	    static createFrom(source: any = {}) {
	        return new Drive(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.label = source["label"];
	        this.kind = source["kind"];
	        this.total = source["total"];
	        this.free = source["free"];
	        this.ready = source["ready"];
	        this.fileSystem = source["fileSystem"];
	        this.serial = source["serial"];
	    }
	}
	export class Entry {
	    name: string;
	    path: string;
	    dir: boolean;
	    size: number;
	    modified: number;
	    created: number;
	    ext: string;
	    hidden: boolean;
	    system: boolean;
	    link: boolean;
	    original?: string;
	    deleted?: number;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.dir = source["dir"];
	        this.size = source["size"];
	        this.modified = source["modified"];
	        this.created = source["created"];
	        this.ext = source["ext"];
	        this.hidden = source["hidden"];
	        this.system = source["system"];
	        this.link = source["link"];
	        this.original = source["original"];
	        this.deleted = source["deleted"];
	    }
	}
	export class Size {
	    bytes: number;
	    files: number;
	    folders: number;
	    unreadable: number;
	
	    static createFrom(source: any = {}) {
	        return new Size(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bytes = source["bytes"];
	        this.files = source["files"];
	        this.folders = source["folders"];
	        this.unreadable = source["unreadable"];
	    }
	}
	export class TextPreview {
	    text: string;
	    truncated: boolean;
	    binary: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TextPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.truncated = source["truncated"];
	        this.binary = source["binary"];
	    }
	}

}

export namespace main {
	
	export class ClipboardState {
	    hasFiles: boolean;
	    cut: string[];
	
	    static createFrom(source: any = {}) {
	        return new ClipboardState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasFiles = source["hasFiles"];
	        this.cut = source["cut"];
	    }
	}
	export class Folder {
	    path: string;
	    entries: files.Entry[];
	    unreadable: number;
	    types: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Folder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.entries = this.convertValues(source["entries"], files.Entry);
	        this.unreadable = source["unreadable"];
	        this.types = source["types"];
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
	export class Location {
	    folder: string;
	    select: string;
	
	    static createFrom(source: any = {}) {
	        return new Location(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folder = source["folder"];
	        this.select = source["select"];
	    }
	}
	export class Modifiers {
	    ctrl: boolean;
	    shift: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Modifiers(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ctrl = source["ctrl"];
	        this.shift = source["shift"];
	    }
	}
	export class SearchDone {
	    count: number;
	    truncated: boolean;
	    cancelled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SearchDone(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.count = source["count"];
	        this.truncated = source["truncated"];
	        this.cancelled = source["cancelled"];
	    }
	}
	export class Startup {
	    state: state.State;
	    known: Record<string, string>;
	    drives: files.Drive[];
	    paths: string[];
	    terminals: terminals.Terminal[];
	    version: string;
	    defaultTerminal: string;
	
	    static createFrom(source: any = {}) {
	        return new Startup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = this.convertValues(source["state"], state.State);
	        this.known = source["known"];
	        this.drives = this.convertValues(source["drives"], files.Drive);
	        this.paths = source["paths"];
	        this.terminals = this.convertValues(source["terminals"], terminals.Terminal);
	        this.version = source["version"];
	        this.defaultTerminal = source["defaultTerminal"];
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

export namespace media {
	
	export class Audio {
	    codec: string;
	    profile: string;
	    channels: number;
	    layout: string;
	    sampleRate: number;
	    bitrate: number;
	    bitDepth: number;
	    language: string;
	    title: string;
	
	    static createFrom(source: any = {}) {
	        return new Audio(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.codec = source["codec"];
	        this.profile = source["profile"];
	        this.channels = source["channels"];
	        this.layout = source["layout"];
	        this.sampleRate = source["sampleRate"];
	        this.bitrate = source["bitrate"];
	        this.bitDepth = source["bitDepth"];
	        this.language = source["language"];
	        this.title = source["title"];
	    }
	}
	export class Photo {
	    width: number;
	    height: number;
	    bitDepth: number;
	    dpi: number;
	    camera: string;
	    lens: string;
	    taken: number;
	    exposure: number;
	    fNumber: number;
	    iso: number;
	    focalLength: number;
	    flash: boolean;
	    colorProfile: string;
	
	    static createFrom(source: any = {}) {
	        return new Photo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.bitDepth = source["bitDepth"];
	        this.dpi = source["dpi"];
	        this.camera = source["camera"];
	        this.lens = source["lens"];
	        this.taken = source["taken"];
	        this.exposure = source["exposure"];
	        this.fNumber = source["fNumber"];
	        this.iso = source["iso"];
	        this.focalLength = source["focalLength"];
	        this.flash = source["flash"];
	        this.colorProfile = source["colorProfile"];
	    }
	}
	export class Subtitle {
	    codec: string;
	    language: string;
	    title: string;
	
	    static createFrom(source: any = {}) {
	        return new Subtitle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.codec = source["codec"];
	        this.language = source["language"];
	        this.title = source["title"];
	    }
	}
	export class Video {
	    codec: string;
	    profile: string;
	    width: number;
	    height: number;
	    fps: number;
	    bitrate: number;
	    pixelFormat: string;
	    bitDepth: number;
	    hdr: string;
	    aspect: string;
	    scan: string;
	    rotation: number;
	    language: string;
	    title: string;
	
	    static createFrom(source: any = {}) {
	        return new Video(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.codec = source["codec"];
	        this.profile = source["profile"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.fps = source["fps"];
	        this.bitrate = source["bitrate"];
	        this.pixelFormat = source["pixelFormat"];
	        this.bitDepth = source["bitDepth"];
	        this.hdr = source["hdr"];
	        this.aspect = source["aspect"];
	        this.scan = source["scan"];
	        this.rotation = source["rotation"];
	        this.language = source["language"];
	        this.title = source["title"];
	    }
	}
	export class Info {
	    source: string;
	    format: string;
	    duration: number;
	    bitrate: number;
	    video: Video[];
	    audio: Audio[];
	    subtitles: Subtitle[];
	    tags: Record<string, string>;
	    photo?: Photo;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.format = source["format"];
	        this.duration = source["duration"];
	        this.bitrate = source["bitrate"];
	        this.video = this.convertValues(source["video"], Video);
	        this.audio = this.convertValues(source["audio"], Audio);
	        this.subtitles = this.convertValues(source["subtitles"], Subtitle);
	        this.tags = source["tags"];
	        this.photo = this.convertValues(source["photo"], Photo);
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

export namespace state {
	
	export class FolderView {
	    view: string;
	    sortKey: string;
	    sortDesc: boolean;
	    used: number;
	
	    static createFrom(source: any = {}) {
	        return new FolderView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = source["view"];
	        this.sortKey = source["sortKey"];
	        this.sortDesc = source["sortDesc"];
	        this.used = source["used"];
	    }
	}
	export class Tab {
	    path: string;
	    pinned: boolean;
	    view?: string;
	    sortKey?: string;
	    sortDesc?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Tab(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.pinned = source["pinned"];
	        this.view = source["view"];
	        this.sortKey = source["sortKey"];
	        this.sortDesc = source["sortDesc"];
	    }
	}
	export class Pane {
	    tabs: Tab[];
	    activeIndex: number;
	
	    static createFrom(source: any = {}) {
	        return new Pane(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tabs = this.convertValues(source["tabs"], Tab);
	        this.activeIndex = source["activeIndex"];
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
	export class Terminal {
	    id: string;
	    name: string;
	    path: string;
	    args: string;
	
	    static createFrom(source: any = {}) {
	        return new Terminal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.args = source["args"];
	    }
	}
	export class Window {
	    left: number;
	    top: number;
	    right: number;
	    bottom: number;
	    maximized: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Window(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.left = source["left"];
	        this.top = source["top"];
	        this.right = source["right"];
	        this.bottom = source["bottom"];
	        this.maximized = source["maximized"];
	    }
	}
	export class State {
	    panes: Pane[];
	    focusedPane: number;
	    splitRatio: number;
	    tabs?: Tab[];
	    activeIndex?: number;
	    windowsIcons: boolean;
	    places: string[];
	    removedDrives: string[];
	    placesReady: boolean;
	    recent: string[];
	    showHidden: boolean;
	    sidebarWidth: number;
	    previewWidth: number;
	    previewOpen: boolean;
	    tileSize: number;
	    columns: Record<string, number>;
	    folderViews: Record<string, FolderView>;
	    recentOpen: boolean;
	    recentHeight: number;
	    theme: string;
	    keepWindowSize: boolean;
	    keepMaximized: boolean;
	    window?: Window;
	    terminals: Terminal[];
	    defaultTerminal: string;
	    hiddenTerminals: string[];
	    checkUpdates: boolean;
	    lastUpdateCheck: number;
	    skippedVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.panes = this.convertValues(source["panes"], Pane);
	        this.focusedPane = source["focusedPane"];
	        this.splitRatio = source["splitRatio"];
	        this.tabs = this.convertValues(source["tabs"], Tab);
	        this.activeIndex = source["activeIndex"];
	        this.windowsIcons = source["windowsIcons"];
	        this.places = source["places"];
	        this.removedDrives = source["removedDrives"];
	        this.placesReady = source["placesReady"];
	        this.recent = source["recent"];
	        this.showHidden = source["showHidden"];
	        this.sidebarWidth = source["sidebarWidth"];
	        this.previewWidth = source["previewWidth"];
	        this.previewOpen = source["previewOpen"];
	        this.tileSize = source["tileSize"];
	        this.columns = source["columns"];
	        this.folderViews = this.convertValues(source["folderViews"], FolderView, true);
	        this.recentOpen = source["recentOpen"];
	        this.recentHeight = source["recentHeight"];
	        this.theme = source["theme"];
	        this.keepWindowSize = source["keepWindowSize"];
	        this.keepMaximized = source["keepMaximized"];
	        this.window = this.convertValues(source["window"], Window);
	        this.terminals = this.convertValues(source["terminals"], Terminal);
	        this.defaultTerminal = source["defaultTerminal"];
	        this.hiddenTerminals = source["hiddenTerminals"];
	        this.checkUpdates = source["checkUpdates"];
	        this.lastUpdateCheck = source["lastUpdateCheck"];
	        this.skippedVersion = source["skippedVersion"];
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

export namespace terminals {
	
	export class Terminal {
	    id: string;
	    name: string;
	    custom: boolean;
	    path: string;
	    args: string;
	
	    static createFrom(source: any = {}) {
	        return new Terminal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.custom = source["custom"];
	        this.path = source["path"];
	        this.args = source["args"];
	    }
	}

}

export namespace undo {
	
	export class State {
	    canUndo: boolean;
	    canRedo: boolean;
	    undoLabel: string;
	    redoLabel: string;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.canUndo = source["canUndo"];
	        this.canRedo = source["canRedo"];
	        this.undoLabel = source["undoLabel"];
	        this.redoLabel = source["redoLabel"];
	    }
	}

}

export namespace update {
	
	export class Release {
	    version: string;
	    name: string;
	    url: string;
	    notes: string;
	    published: string;
	    newer: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Release(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.notes = source["notes"];
	        this.published = source["published"];
	        this.newer = source["newer"];
	    }
	}

}

export namespace winshell {
	
	export class RecycleBin {
	    items: number;
	    bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new RecycleBin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = source["items"];
	        this.bytes = source["bytes"];
	    }
	}

}

