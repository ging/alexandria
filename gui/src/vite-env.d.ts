// Ambient declarations for build-time constants injected by vite.config.ts.
// They expose the proxied upstream targets so the UI can display them.
/// <reference types="vite/client" />
declare const __HUB_TARGET__: string
declare const __CLIENT_TARGET__: string
declare const __IDP_TARGET__: string
