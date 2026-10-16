import { Component } from "react";
import type { ErrorInfo, ReactNode } from "react";
import { TriangleAlert } from "lucide-react";
import { Button } from "./ui/button";
export class ErrorBoundary extends Component<
  { children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  componentDidCatch(error: Error, info: ErrorInfo) {
    if (import.meta.env.DEV)
      console.error("Render failure", error, info.componentStack);
  }
  render() {
    if (this.state.failed)
      return (
        <main className="boundary-fallback">
          <TriangleAlert size={36} />
          <h1>Something went wrong</h1>
          <p>
            This page could not be displayed. Reload it or return to your
            dashboard.
          </p>
          <div className="flex flex-wrap justify-center gap-3">
            <Button onClick={() => window.location.reload()}>
              Reload page
            </Button>
            <Button variant="outline" asChild>
              <a href="/dashboard">Return to dashboard</a>
            </Button>
          </div>
        </main>
      );
    return this.props.children;
  }
}
