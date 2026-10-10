import type { JSX } from "react";
import { Link } from "react-router";
import { BRAND_NAME } from "@/brand";

export function Logo(): JSX.Element {
  return (
    <Link to="/" className="inline-flex items-center gap-2.5 rounded-md font-display text-xl font-semibold tracking-tight">
      <span aria-hidden="true" className="flex size-7 flex-col justify-center gap-[3px] rounded-[7px] bg-accent px-[6px]">
        <span className="h-[2.5px] rounded-full bg-accent-foreground" />
        <span className="h-[2.5px] rounded-full bg-accent-foreground" />
        <span className="h-[2.5px] w-2/3 rounded-full bg-accent-foreground" />
      </span>
      {BRAND_NAME}
    </Link>
  );
}
