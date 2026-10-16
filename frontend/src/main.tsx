import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { Providers } from "./app/providers";
import { AppRoutes } from "./app/router";
import { ErrorBoundary } from "./components/ErrorBoundary";
import "@fontsource-variable/inter/wght.css";
import "./styles/index.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ErrorBoundary>
      <Providers>
        <BrowserRouter
          future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
        >
          <AppRoutes />
        </BrowserRouter>
      </Providers>
    </ErrorBoundary>
  </StrictMode>,
);
