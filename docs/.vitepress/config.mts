import { defineConfig } from "vitepress";

export default defineConfig({
  title: "Go Feather Route",
  description: "A fast, featherweight OpenAI-compatible model router written in Go.",
  base: "/go-feather-route/",
  cleanUrls: true,
  themeConfig: {
    nav: [
      { text: "Guide", link: "/getting-started" },
      { text: "API", link: "/api" },
      { text: "GitHub", link: "https://github.com/sayanmohsin/go-feather-route" },
      { text: "Docker Hub", link: "https://hub.docker.com/r/sayanmohsin/go-feather-route" },
    ],
    sidebar: [
      {
        text: "Guide",
        items: [
          { text: "Overview", link: "/" },
          { text: "Getting started", link: "/getting-started" },
          { text: "Configuration", link: "/configuration" },
          { text: "Environment", link: "/environment" },
          { text: "Providers and routing", link: "/providers" },
          { text: "Streaming", link: "/streaming" },
          { text: "Authentication", link: "/authentication" },
          { text: "Docker", link: "/docker" },
          { text: "Deployment", link: "/deployment" },
        ],
      },
      {
        text: "Operations",
        items: [
          { text: "Health", link: "/health" },
          { text: "Performance", link: "/performance" },
          { text: "Benchmarks", link: "/benchmarks" },
          { text: "Security", link: "/security" },
          { text: "Troubleshooting", link: "/troubleshooting" },
        ],
      },
      {
        text: "Reference",
        items: [
          { text: "API reference", link: "/api" },
          { text: "OpenAPI", link: "/api/openapi" },
          { text: "Thingd MCP", link: "/thingd-mcp" },
          { text: "Testing", link: "/testing" },
          { text: "Roadmap", link: "/roadmap" },
          { text: "Contributing", link: "/contributing" },
        ],
      },
    ],
    socialLinks: [{ icon: "github", link: "https://github.com/sayanmohsin/go-feather-route" }],
  },
});
