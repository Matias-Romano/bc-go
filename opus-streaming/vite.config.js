import { defineConfig } from 'vite';

export default defineConfig({
    server: {
        port: 42068
    },
    build: {
        outDir: 'dist',
        rollupOptions: {
            input: './client.js'
        }
    }
});
