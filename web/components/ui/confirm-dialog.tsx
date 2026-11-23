"use client"

import * as React from "react"
import { AlertDialog as AlertDialogPrimitive } from "radix-ui"

import { Button } from "@/components/ui/button"

type Opening =
  | { trigger: React.ReactElement; open?: never; onOpenChange?: never }
  | { trigger?: never; open: boolean; onOpenChange: (open: boolean) => void }

export type ConfirmDialogProps = Opening & {
  title: string
  description: React.ReactNode
  confirmLabel: string
  pendingLabel?: string
  cancelLabel?: string
  tone?: "destructive" | "default"
  onConfirm: () => Promise<void> | void
  onCloseAutoFocus?: (event: Event) => void
}

function ConfirmDialog({
  trigger,
  open: controlledOpen,
  onOpenChange,
  title,
  description,
  confirmLabel,
  pendingLabel,
  cancelLabel = "Cancelar",
  tone = "destructive",
  onConfirm,
  onCloseAutoFocus,
}: ConfirmDialogProps): React.JSX.Element {
  const [ownOpen, setOwnOpen] = React.useState(false)
  const [pending, setPending] = React.useState(false)
  const open = controlledOpen ?? ownOpen
  const setOpen = onOpenChange ?? setOwnOpen

  async function confirm(): Promise<void> {
    setPending(true)
    try {
      await onConfirm()
    } finally {
      setPending(false)
      setOpen(false)
    }
  }

  return (
    <AlertDialogPrimitive.Root open={open} onOpenChange={(next) => (pending ? undefined : setOpen(next))}>
      {trigger ? <AlertDialogPrimitive.Trigger asChild>{trigger}</AlertDialogPrimitive.Trigger> : null}
      <AlertDialogPrimitive.Portal>
        <AlertDialogPrimitive.Overlay className="fixed inset-0 z-50 bg-black/60 duration-(--dur-base) ease-standard data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0" />
        <AlertDialogPrimitive.Content
          data-slot="confirm-dialog"
          onCloseAutoFocus={onCloseAutoFocus}
          className="fixed top-[50%] left-[50%] z-50 grid w-full min-w-0 max-w-[calc(100%-2rem)] translate-x-[-50%] translate-y-[-50%] gap-4 rounded-lg border border-border-strong bg-popover p-5 text-popover-foreground outline-none data-[state=closed]:animate-dialog-out data-[state=open]:animate-dialog-in sm:max-w-md"
        >
          <div className="flex min-w-0 flex-col gap-2">
            <AlertDialogPrimitive.Title className="min-w-0 break-words font-display text-title font-bold text-fg-1">
              {title}
            </AlertDialogPrimitive.Title>
            <AlertDialogPrimitive.Description className="text-body-sm text-fg-2">
              {description}
            </AlertDialogPrimitive.Description>
          </div>
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <AlertDialogPrimitive.Cancel asChild>
              <Button type="button" variant="outline" disabled={pending}>
                {cancelLabel}
              </Button>
            </AlertDialogPrimitive.Cancel>
            <Button
              type="button"
              variant={tone === "destructive" ? "destructive" : "default"}
              loading={pending}
              loadingText={pendingLabel}
              onClick={() => void confirm()}
            >
              {confirmLabel}
            </Button>
          </div>
        </AlertDialogPrimitive.Content>
      </AlertDialogPrimitive.Portal>
    </AlertDialogPrimitive.Root>
  )
}

export { ConfirmDialog }
