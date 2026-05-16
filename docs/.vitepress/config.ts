import { defineConfig } from "vitepress";

const SITE_URL = "https://turbine.yakir.io";
const SITE_NAME = "Turbine";
const SITE_DESCRIPTION = "SQLite-based durable workflow engine for Go";
const OG_IMAGE = `${SITE_URL}/og.png`;

export default defineConfig({
  lang: "en-US",
  title: SITE_NAME,
  description: SITE_DESCRIPTION,
  cleanUrls: true,
  lastUpdated: true,
  sitemap: {
    hostname: SITE_URL,
  },
  head: [
    ["link", { rel: "icon", type: "image/svg+xml", href: "/favicon.svg" }],
    ["meta", { property: "og:type", content: "website" }],
    ["meta", { property: "og:site_name", content: SITE_NAME }],
    ["meta", { property: "og:image", content: OG_IMAGE }],
    ["meta", { property: "og:image:width", content: "2994" }],
    ["meta", { property: "og:image:height", content: "1642" }],
    ["meta", { property: "og:image:alt", content: "Turbine dashboard showing workflow steps and logs" }],
    ["meta", { name: "twitter:card", content: "summary_large_image" }],
    ["meta", { name: "twitter:image", content: OG_IMAGE }],
  ],
  transformPageData(pageData) {
    const pageTitle = pageData.title || SITE_NAME;
    const fullTitle = `${pageTitle} | ${SITE_NAME}`;
    const description =
      (pageData.frontmatter.description as string | undefined) ?? SITE_DESCRIPTION;

    const path = pageData.relativePath
      .replace(/(^|\/)index\.md$/, "$1")
      .replace(/\.md$/, "");
    const url = `${SITE_URL}/${path}`;

    pageData.frontmatter.head ??= [];
    pageData.frontmatter.head.push(
      ["meta", { name: "description", content: description }],
      ["meta", { property: "og:title", content: fullTitle }],
      ["meta", { property: "og:description", content: description }],
      ["meta", { property: "og:url", content: url }],
      ["meta", { name: "twitter:title", content: fullTitle }],
      ["meta", { name: "twitter:description", content: description }],
      ["link", { rel: "canonical", href: url }],
    );
  },
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
