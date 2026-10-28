// Wails desktop bridge. Recreates the window.* APIs the Electron preload used
// to expose, on top of the Go backend's same-origin /native/* routes. Events
// from Go arrive as DOM CustomEvents named "native:<name>" (see main.go).

const call = async (method: string, path: string, body?: unknown): Promise<any> => {
    const res = await fetch(`/native${path}`, {
        method,
        headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
        body: body === undefined ? undefined : JSON.stringify(body),
    });
    return res.json();
};

const onNative = (name: string, callback: (detail: any) => void): void => {
    window.addEventListener(`native:${name}`, (e) => callback((e as CustomEvent).detail));
};

window.electronStore = {
    get: async (key: string) => (await call('POST', '/store/get', { key })).value,
    set: async (key: string, value: any) => { await call('POST', '/store/set', { key, value }); },
    remove: async (key: string) => { await call('POST', '/store/remove', { key }); },
};

window.electronLocale = {
    getLocale: () => call('GET', '/locale'),
    getState: () => call('GET', '/locale/state'),
    setLocale: (locale: string) => call('POST', '/locale', { locale }),
};

window.electronWindow = {
    setAlwaysOnTop: async (value: boolean) => { await call('POST', '/window/always-on-top', { value }); },
    isAlwaysOnTop: () => call('GET', '/window/always-on-top'),
};

window.electronLogs = {
    openLogsFolder: async () => { await call('POST', '/logs/open-folder'); },
    onOpenLogsModal: (callback: () => void) => onNative('open-logs-modal', callback),
};

window.trayControls = {
    onPlayPause: (callback: () => void) => onNative('tray-play-pause', callback),
    onSetSpeed: (callback: (event: any, speed: number) => void) =>
        onNative('tray-set-speed', (speed) => callback(null, speed)),
    onLogout: (callback: () => void) => onNative('tray-logout', callback),
    updatePlayingState: (isPlaying: boolean, bookTitle: string | null) => {
        void call('POST', '/tray/playing', { isPlaying, bookTitle });
    },
    updateAuthState: (isAuthenticated: boolean) => {
        void call('POST', '/tray/auth', { isAuthenticated });
    },
};

export {};
