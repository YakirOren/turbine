import path from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

const isDev = process.env.NODE_ENV !== "production";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: isDev ? "/" : "/_/turbine/",
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 5174,
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8090",
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "../dashboard/dist",
    emptyOutDir: true,
    target: "es2022",
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) return;
          if (id.includes("@codemirror") || id.includes("/codemirror/")) return "codemirror";
          if (id.includes("recharts") || id.includes("d3-")) return "charts";
          if (id.includes("@xyflow") || id.includes("dagre")) return "flow";
          if (id.includes("@refinedev") || id.includes("refine-pocketbase")) return "refine";
          if (id.includes("radix-ui") || id.includes("@base-ui")) return "ui-primitives";
        },
      },
    },
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
    passWithNoTests: true,
  },
});
