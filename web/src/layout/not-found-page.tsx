import { Compass } from "lucide-react";
import type { JSX } from "react";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";

interface NotFoundPageProps {
  title?: string;
  message?: string;
}

export function NotFoundPage({ title = "Page not found", message = "The page you asked for does not exist." }: NotFoundPageProps): JSX.Element {
  return (
    <div className="mx-auto max-w-xl py-10">
      <EmptyState
        icon={<Compass />}
        title={title}
        actions={
          <Button asChild variant="primary">
            <Link to="/">Back to sites</Link>
          </Button>
        }
      >
        <p>{message}</p>
      </EmptyState>
    </div>
  );
}
