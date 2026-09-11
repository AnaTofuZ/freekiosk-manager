import { defineConfig } from "vite";
import { barefoot } from "@barefootjs/go-template/vite";

export default defineConfig({
  base: "/static/generated/",
  build: {
    outDir: "web/static/generated",
    target: "es2022",
    rollupOptions: {
      input: { main: "web/src/components/Page.tsx" },
      output: { entryFileNames: "[name].js" },
    },
  },
  plugins: barefoot({
    components: ["web/src/components"],
    templates: "web/generated",
    packageName: "views",
    typesOutputFile: "web/views/components.go",
  }),
});
