"use client"

import * as React from "react"
import { type VariantProps } from "class-variance-authority"
import { ToggleGroup as ToggleGroupPrimitive } from "radix-ui"

import { cn } from "@/lib/utils"
import { toggleVariants } from "@/components/ui/toggle"

/**
 * `style` only ever carries the gap scalar, so the custom property is part of
 * the declared type instead of being smuggled past the checker with
 * `as React.CSSProperties`.
 */
type ToggleGroupStyle = React.CSSProperties & {
  "--toggle-group-gap"?: string
}

const ToggleGroupContext = React.createContext<
  VariantProps<typeof toggleVariants> & {
    spacing?: number
  }
>({
  size: "default",
  variant: "default",
  spacing: 0,
})

function ToggleGroup({
  className,
  variant,
  size,
  spacing = 0,
  style,
  children,
  ...props
}: React.ComponentProps<typeof ToggleGroupPrimitive.Root> &
  VariantProps<typeof toggleVariants> & {
    spacing?: number
  }) {
  // `gap-[--spacing(var(--gap))]` was not valid Tailwind v4 arbitrary-value
  // syntax and compiled to nothing, so `spacing` had never had any effect.
  // `gap-(--toggle-group-gap)` is the supported var shorthand; the value is
  // resolved here against the theme spacing unit.
  const gapStyle: ToggleGroupStyle = {
    ...style,
    "--toggle-group-gap": `calc(var(--spacing) * ${spacing})`,
  }

  return (
    <ToggleGroupPrimitive.Root
      data-slot="toggle-group"
      data-variant={variant}
      data-size={size}
      data-spacing={spacing}
      style={gapStyle}
      className={cn(
        "group/toggle-group flex w-fit items-center gap-(--toggle-group-gap) rounded-md",
        className
      )}
      {...props}
    >
      <ToggleGroupContext.Provider value={{ variant, size, spacing }}>
        {children}
      </ToggleGroupContext.Provider>
    </ToggleGroupPrimitive.Root>
  )
}

function ToggleGroupItem({
  className,
  children,
  variant,
  size,
  ...props
}: React.ComponentProps<typeof ToggleGroupPrimitive.Item> &
  VariantProps<typeof toggleVariants>) {
  const context = React.useContext(ToggleGroupContext)

  return (
    <ToggleGroupPrimitive.Item
      data-slot="toggle-group-item"
      data-variant={context.variant || variant}
      data-size={context.size || size}
      data-spacing={context.spacing}
      className={cn(
        toggleVariants({
          variant: context.variant || variant,
          size: context.size || size,
        }),
        "w-auto min-w-0 shrink-0 focus-visible:z-10",
        // Joined mode: one continuous plate, so only the outer corners round
        // and adjacent bordered items share a single hairline.
        "data-[spacing=0]:rounded-none data-[spacing=0]:first:rounded-l-md data-[spacing=0]:last:rounded-r-md data-[spacing=0]:shadow-none",
        "data-[spacing=0]:data-[variant=outline]:border-l-0 data-[spacing=0]:data-[variant=outline]:first:border-l",
        "data-[spacing=0]:data-[variant=filter]:border-l-0 data-[spacing=0]:data-[variant=filter]:first:border-l",
        className
      )}
      {...props}
    >
      {children}
    </ToggleGroupPrimitive.Item>
  )
}

export { ToggleGroup, ToggleGroupItem }
