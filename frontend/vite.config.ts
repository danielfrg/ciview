import { fileURLToPath, URL } from "node:url";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [svelte()],
  // Kit UI ships Svelte/TypeScript source; compile it with the app.
  optimizeDeps: { exclude: ["@kenn-io/kit-ui"] },
  resolve: {
    alias: { $lib: fileURLToPath(new URL("./src/lib", import.meta.url)) },
  },
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
    proxy: {
      "/api": process.env.CIVIEW_API_TARGET ?? "http://127.0.0.1:8080",
    },
  },
});
