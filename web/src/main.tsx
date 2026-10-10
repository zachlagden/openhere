import { QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import { App } from "./app";
import { ToastProvider } from "./components/ui/toast";
import { ErrorBoundary } from "./layout/error-boundary";
import { ThemeProvider } from "./lib/theme";
import { createQueryClient } from "./queries/client";
import "./index.css";

const queryClient = createQueryClient();
const container = document.getElementById("root");
if (!container) {
  throw new Error("The page has no root element.");
}

createRoot(container).render(
  <StrictMode>
    <ErrorBoundary>
      <ThemeProvider>
        <QueryClientProvider client={queryClient}>
          <BrowserRouter>
            <ToastProvider>
              <App />
            </ToastProvider>
          </BrowserRouter>
        </QueryClientProvider>
      </ThemeProvider>
    </ErrorBoundary>
  </StrictMode>,
);
