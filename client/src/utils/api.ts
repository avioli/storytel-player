import storage from "./storage";
import axios from "axios";

// The Go backend serves /api/* on the app's own origin (see server/api.go).
const api = axios.create({
    baseURL: "/api",
    withCredentials: true,
    // Downloads hold the request open until the file is complete, so there
    // is no timeout (the request never leaves the process).
    timeout: 0
});

api.interceptors.request.use(async (config) => {
    const token = await storage.get("token");
    if (token) {
        config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
});

api.interceptors.response.use(
    (response) => response,
    (error) => {
        const hadToken = !!error.config?.headers?.Authorization;
        if (error.response?.status === 401 && hadToken && !error.config?.url?.includes('/login')) {
            window.dispatchEvent(new Event('unauthorized'));
        }
        return Promise.reject(error);
    }
);

export default api;


export const trackAction = async (message: string, data?: any) => {
    try {
        await api.post('/logs/action', { message, data });
    } catch (e) {
        // Silently ignore tracking errors
        console.error('Failed to track action', e);
    }
};
