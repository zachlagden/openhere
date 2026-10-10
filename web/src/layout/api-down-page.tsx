import { CloudOff, ServerCrash } from "lucide-react";
import type { JSX } from "react";
import { describeError } from "@/api/errors";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";

interface ApiDownPageProps {
  error: unknown;
  onRetry: () => void;
}

export function ApiDownPage({ error, onRetry }: ApiDownPageProps): JSX.Element {
  const description = describeError(error);
  const offline = description.kind === "network";
  return (
    <div className="mx-auto flex min-h-dvh max-w-xl items-center px-4">
      <div className="w-full">
        <EmptyState
          icon={offline ? <CloudOff /> : <ServerCrash />}
          title={offline ? "Cannot reach the server" : "The server had a problem"}
          actions={
            <Button variant="primary" onClick={onRetry}>
              Try again
            </Button>
          }
        >
          <p>{offline ? "The server may be restarting, or your connection may be down. Your work on the server is safe." : description.message}</p>
        </EmptyState>
      </div>
    </div>
  );
}
