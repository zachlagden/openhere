import type { ComponentProps, JSX } from "react";
import { cn } from "@/lib/cn";

export function Skeleton({ className, ...props }: ComponentProps<"div">): JSX.Element {
  return <div aria-hidden="true" className={cn("skeleton rounded-md", className)} {...props} />;
}
