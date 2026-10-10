import * as Primitive from "@radix-ui/react-dropdown-menu";
import type { ComponentProps, JSX } from "react";
import { cn } from "@/lib/cn";

export function DropdownMenu(props: Omit<ComponentProps<typeof Primitive.Root>, "modal">): JSX.Element {
  return <Primitive.Root modal={false} {...props} />;
}

export const DropdownMenuTrigger = Primitive.Trigger;
export const DropdownMenuRadioGroup = Primitive.RadioGroup;

export function DropdownMenuContent({ className, sideOffset = 6, ...props }: ComponentProps<typeof Primitive.Content>): JSX.Element {
  return (
    <Primitive.Portal>
      <Primitive.Content
        sideOffset={sideOffset}
        collisionPadding={12}
        className={cn(
          "z-50 max-h-[min(24rem,var(--radix-dropdown-menu-content-available-height))] min-w-52 max-w-[calc(100vw-1.5rem)] overflow-y-auto rounded-lg border border-line bg-surface p-1 text-sm text-foreground shadow-dialog",
          className,
        )}
        {...props}
      />
    </Primitive.Portal>
  );
}

const itemClasses =
  "relative flex cursor-default select-none items-center gap-2 rounded-md px-2.5 py-2 outline-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50 data-[highlighted]:bg-sunken [&_svg]:size-4 [&_svg]:shrink-0";

export function DropdownMenuItem({ className, ...props }: ComponentProps<typeof Primitive.Item>): JSX.Element {
  return <Primitive.Item className={cn(itemClasses, className)} {...props} />;
}

export function DropdownMenuRadioItem({ className, children, ...props }: ComponentProps<typeof Primitive.RadioItem>): JSX.Element {
  return (
    <Primitive.RadioItem className={cn(itemClasses, "data-[state=checked]:font-medium", className)} {...props}>
      <span className="size-4 shrink-0">
        <Primitive.ItemIndicator>
          <span aria-hidden="true" className="block text-accent">
            &#10003;
          </span>
        </Primitive.ItemIndicator>
      </span>
      {children}
    </Primitive.RadioItem>
  );
}

export function DropdownMenuLabel({ className, ...props }: ComponentProps<typeof Primitive.Label>): JSX.Element {
  return <Primitive.Label className={cn("px-2.5 py-1.5 text-xs font-medium uppercase tracking-wider text-muted", className)} {...props} />;
}

export function DropdownMenuSeparator({ className, ...props }: ComponentProps<typeof Primitive.Separator>): JSX.Element {
  return <Primitive.Separator className={cn("-mx-1 my-1 h-px bg-line", className)} {...props} />;
}
