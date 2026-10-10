import { TriangleAlert } from "lucide-react";
import { Component, type ErrorInfo, type JSX, type ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";

interface ErrorBoundaryProps {
  children: ReactNode;
}

interface ErrorBoundaryState {
  failed: boolean;
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  override state: ErrorBoundaryState = { failed: false };

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { failed: true };
  }

  override componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error("Dashboard error", error, info.componentStack);
  }

  override render(): JSX.Element | ReactNode {
    if (!this.state.failed) {
      return this.props.children;
    }
    return (
      <div className="mx-auto flex min-h-dvh max-w-xl items-center px-4">
        <div className="w-full">
          <EmptyState
            icon={<TriangleAlert />}
            title="Something went wrong"
            actions={
              <Button variant="primary" onClick={() => window.location.reload()}>
                Reload the page
              </Button>
            }
          >
            <p>The dashboard hit an unexpected problem. Reloading usually fixes it.</p>
          </EmptyState>
        </div>
      </div>
    );
  }
}
