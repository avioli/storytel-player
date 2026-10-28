import {StorageType} from "../interfaces/helper";

declare global {
    interface Window {
        electronStore?: StorageType;
    }
}

// Persistent key/value store backed by the Go side (server/store.go), via the
// bridge installed by platform/wails.ts.
const storage: StorageType = {
    get: (key: string) => window.electronStore!.get(key),
    set: (key: string, value: any) => window.electronStore!.set(key, value),
    remove: (key: string) => window.electronStore!.remove(key),
};

export default storage;
