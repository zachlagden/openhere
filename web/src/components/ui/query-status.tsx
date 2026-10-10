import type { JSX } from "react";
import { describeError } from "@/api/errors";
import { Alert } from "./alert";
import { Button } from "./button";
import { Skeleton } from "./skeleton";

export function LoadError({ error, onRetry }: { error: unknown; onRetry: () => void }): JSX.Element {
  return (
    <Alert tone="error">
      <p>{describeError(error).message}</p>
      <Button variant="secondary" size="sm" className="mt-2" onClick={onRetry}>
        Try again
      </Button>
    </Alert>
  );
}

export function BlockSkeleton({ label, rows = 3 }: { label: string; rows?: number }): JSX.Element {
  return (
    <div role="status" aria-label={label} className="flex flex-col gap-3 rounded-lg border border-line bg-surface p-5">
      {Array.from({ length: rows }, (_, index) => (
        <Skeleton key={index} className={index === 0 ? "h-5 w-1/3" : "h-4 w-full max-w-md"} />
      ))}
    </div>
  );
}
