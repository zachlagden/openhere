import { CircleAlert, CircleCheck } from "lucide-react";
import { createContext, use, useCallback, useMemo, useRef, useState, type JSX, type ReactNode } from "react";
import { cn } from "@/lib/cn";

type ToastTone = "success" | "error";

interface ToastItem {
  id: number;
  tone: ToastTone;
  message: string;
}

interface ToastApi {
  success: (message: string) => void;
  error: (message: string) => void;
}

const ToastContext = createContext<ToastApi | null>(null);

const LIFETIME_MS: Record<ToastTone, number> = { success: 3500, error: 7000 };

export function ToastProvider({ children }: { children: ReactNode }): JSX.Element {
  const [items, setItems] = useState<ToastItem[]>([]);
  const nextId = useRef(1);

  const push = useCallback((tone: ToastTone, message: string): void => {
    const id = nextId.current++;
    setItems((current) => [...current.slice(-3), { id, tone, message }]);
    setTimeout(() => setItems((current) => current.filter((item) => item.id !== id)), LIFETIME_MS[tone]);
  }, []);

  const api = useMemo<ToastApi>(
    () => ({ success: (message) => push("success", message), error: (message) => push("error", message) }),
    [push],
  );

  return (
    <ToastContext value={api}>
      {children}
      <div
        aria-live="polite"
        className="pointer-events-none fixed inset-x-0 bottom-0 z-50 flex flex-col items-center gap-2 p-4 sm:items-end"
      >
        {items.map((item) => (
          <div
            key={item.id}
            role={item.tone === "error" ? "alert" : "status"}
            className={cn(
              "rise pointer-events-auto flex max-w-sm items-start gap-2.5 rounded-lg border bg-surface px-3.5 py-2.5 text-sm shadow-dialog",
              item.tone === "error" ? "border-danger text-foreground" : "border-line text-foreground",
            )}
          >
            {item.tone === "error" ? (
              <CircleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-danger" />
            ) : (
              <CircleCheck aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-ok" />
            )}
            <span>{item.message}</span>
          </div>
        ))}
      </div>
    </ToastContext>
  );
}

export function useToast(): ToastApi {
  const api = use(ToastContext);
  if (!api) {
    throw new Error("useToast needs a ToastProvider.");
  }
  return api;
}
