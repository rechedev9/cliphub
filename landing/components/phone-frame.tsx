interface PhoneFrameProps {
  children: React.ReactNode;
}

/** CSS-only bezel: the phone presents a vertical video without borrowing a device mockup image. */
export default function PhoneFrame({ children }: PhoneFrameProps) {
  return (
    <div className="mx-auto w-full max-w-[18rem]">
      <div className="relative overflow-hidden rounded-[2.4rem] border-[5px] border-surface-2 bg-surface-2 p-1.5 shadow-2xl shadow-black/50">
        <div
          aria-hidden="true"
          className="absolute left-1/2 top-3 z-10 h-1.5 w-14 -translate-x-1/2 rounded-full bg-black/60"
        />
        <div className="relative aspect-[9/16] overflow-hidden rounded-[2rem] bg-black">{children}</div>
      </div>
    </div>
  );
}
