"use client"

import * as React from "react"

import { cn } from "@/lib/utils"
import { Label } from "@/components/ui/label"

/** Control wiring: id, describedby, invalid. Spread onto Input/Select/textarea. */
export interface FieldControlProps {
  id: string
  "aria-describedby": string | undefined
  "aria-invalid": true | undefined
}

export interface FieldProps {
  /** Visible label text. Always rendered — a field without one is a defect. */
  label: React.ReactNode
  /** Receives the wiring for the control it renders. */
  children: (control: FieldControlProps) => React.ReactNode
  /** Persistent helper copy, shown under the control. */
  hint?: React.ReactNode
  /** Validation message. Its presence also marks the control `aria-invalid`. */
  error?: React.ReactNode
  required?: boolean
  className?: string
}

function Field({
  label,
  children,
  hint,
  error,
  required = false,
  className,
}: FieldProps): React.JSX.Element {
  const id = React.useId()
  const hintId = `${id}-hint`
  const errorId = `${id}-error`

  const hasHint = Boolean(hint)
  const hasError = Boolean(error)
  const describedBy = [hasHint ? hintId : "", hasError ? errorId : ""]
    .filter((token) => token !== "")
    .join(" ")

  return (
    <div
      data-slot="field"
      className={cn("flex w-full flex-col gap-2", hasError && "studio-shake", className)}
    >
      <Label htmlFor={id} className="text-label tracking-wide text-fg-2 uppercase">
        {label}
        {required ? (
          <span aria-hidden className="text-destructive">
            *
          </span>
        ) : null}
      </Label>

      {children({
        id,
        "aria-describedby": describedBy === "" ? undefined : describedBy,
        "aria-invalid": hasError ? true : undefined,
      })}

      {hasHint ? (
        <p id={hintId} data-slot="field-hint" className="text-body-sm text-fg-3">
          {hint}
        </p>
      ) : null}

      {hasError ? (
        <p
          id={errorId}
          data-slot="field-error"
          role="alert"
          className="text-body-sm text-destructive"
        >
          {error}
        </p>
      ) : null}
    </div>
  )
}

export { Field }
