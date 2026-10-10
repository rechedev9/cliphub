"use client";

import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";

export interface ConfirmButtonProps {
  label: string;
  confirmLabel: string;
  onConfirm: () => void;
  disabled?: boolean;
  busy?: boolean;
  size?: "xs" | "sm";
}

// Two-step confirm: the first click arms the button, the second one acts.
export function ConfirmButton(props: ConfirmButtonProps) {
  const [armed, setArmed] = useState(false);
  const size = props.size ?? "xs";

  // An armed button left alone disarms, so a stray click later cannot act.
  useEffect(() => {
    if (!armed) return;
    const timer = setTimeout(() => setArmed(false), 5000);
    return () => clearTimeout(timer);
  }, [armed]);

  if (!armed) {
    return (
      <Button
        type="button"
        variant="outline-destructive"
        size={size}
        disabled={props.disabled || props.busy}
        onClick={() => setArmed(true)}
      >
        {props.busy ? "Un momento" : props.label}
      </Button>
    );
  }

  return (
    <span className="inline-flex flex-wrap items-center gap-2">
      <Button
        type="button"
        variant="destructive"
        size={size}
        disabled={props.disabled}
        onClick={() => {
          setArmed(false);
          props.onConfirm();
        }}
      >
        {props.confirmLabel}
      </Button>
      <Button type="button" variant="outline" size={size} onClick={() => setArmed(false)}>
        No
      </Button>
    </span>
  );
}
