import { defineConfig } from "vitepress";

export default defineConfig({
  title: "Turbine",
  description: "Durable workflow engine for PocketBase",
  cleanUrls: true,
  sitemap: {
    hostname: "https://turbine.yakir.io",
  },
  head: [
    ["link", { rel: "icon", type: "image/svg+xml", href: "/favicon.svg" }],
    ["meta", { property: "og:type", content: "website" }],
    ["meta", { property: "og:title", content: "Turbine" }],
    ["meta", { property: "og:description", content: "Durable workflow engine for PocketBase" }],
    ["meta", { property: "og:url", content: "https://turbine.yakir.io/" }],
    ["meta", { name: "twitter:card", content: "summary" }],
    ["meta", { name: "twitter:title", content: "Turbine" }],
    ["meta", { name: "twitter:description", content: "Durable workflow engine for PocketBase" }],
  ],
  themeConfig: {
    logo: "/favicon.svg",
    nav: [
      { text: "Guide", link: "/getting-started" },
      { text: "API", link: "/api/workflows" },
      { text: "Examples", link: "/examples/basic" },
      { text: "GitHub", link: "https://github.com/YakirOren/turbine" },
    ],
    sidebar: [
      {
        text: "Get Started",
        items: [
          { text: "What is Turbine?", link: "/" },
          { text: "Getting Started", link: "/getting-started" },
        ],
      },
      {
        text: "Core Concepts",
        items: [
          { text: "Workflows", link: "/concepts/workflows" },
          { text: "Steps", link: "/concepts/steps" },
          { text: "Checkpoints", link: "/concepts/checkpoints" },
          { text: "Context", link: "/concepts/context" },
        ],
      },
      {
        text: "Execution",
        items: [
          { text: "Queues", link: "/concepts/queues" },
          { text: "Scheduling", link: "/concepts/scheduling" },
          { text: "App Status", link: "/concepts/app-status" },
          { text: "Error Handling", link: "/concepts/errors" },
        ],
      },
      {
        text: "Communication",
        items: [
          { text: "Send / Recv", link: "/concepts/communication" },
          { text: "Approvals", link: "/concepts/approvals" },
          { text: "Webhooks", link: "/concepts/webhooks" },
          { text: "Notifications", link: "/concepts/notifications" },
        ],
      },
      {
        text: "Data & Storage",
        items: [
          { text: "Products", link: "/concepts/products" },
          { text: "KV Store", link: "/concepts/kv-store" },
        ],
      },
      {
        text: "Operations",
        items: [
          { text: "Lifecycle", link: "/concepts/lifecycle" },
          { text: "Configuration", link: "/api/configuration" },
        ],
      },
      {
        text: "API Reference",
        collapsed: true,
        items: [
          { text: "Workflows", link: "/api/workflows" },
          { text: "Steps", link: "/api/steps" },
          { text: "Queues", link: "/api/queues" },
        ],
      },
      {
        text: "Examples",
        collapsed: true,
        items: [
          { text: "Basic", link: "/examples/basic" },
          { text: "Steps", link: "/examples/steps" },
          { text: "Concurrent Steps", link: "/examples/concurrent" },
          { text: "Retry", link: "/examples/retry" },
          { text: "Sleep", link: "/examples/sleep" },
          { text: "Queue", link: "/examples/queue" },
          { text: "Scheduled", link: "/examples/scheduled" },
          { text: "Events", link: "/examples/events" },
          { text: "Lifecycle", link: "/examples/lifecycle" },
          { text: "App Access", link: "/examples/app-access" },
          { text: "Connector", link: "/examples/connector" },
          { text: "Products", link: "/examples/products" },
          { text: "Dashboard", link: "/examples/dashboard" },
        ],
      },
    ],
    socialLinks: [
      { icon: "github", link: "https://github.com/YakirOren/turbine" },
    ],
    search: {
      provider: "local",
    },
  },
});
