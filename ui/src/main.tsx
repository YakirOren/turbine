import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Toaster } from "sonner";
import { useTheme } from "next-themes";
import "./index.css";
import App from "./App";
import { ThemeProvider } from "@/components/theme-provider";

function ThemedToaster() {
  const { resolvedTheme } = useTheme();
  return (
    <Toaster
      richColors
      position="bottom-right"
      theme={resolvedTheme === "dark" ? "dark" : "light"}
    />
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ThemeProvider>
      <App />
      <ThemedToaster />
    </ThemeProvider>
  </StrictMode>
);
