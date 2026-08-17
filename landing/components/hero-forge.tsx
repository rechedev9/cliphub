import Image from "next/image";

// Static server-rendered backdrop; the animated centerpiece lives in HeroReel.
export default function HeroForge() {
  return (
    <div className="absolute inset-0 overflow-hidden bg-[#050812]" data-testid="hero-forge">
      <Image
        src="/images/hero-replay-forge.webp"
        alt=""
        fill
        priority
        sizes="100vw"
        data-testid="hero-art"
        className="scale-[1.02] object-cover object-[67%_center] opacity-90 saturate-[1.08] contrast-[1.04] motion-safe:transition-transform motion-safe:duration-[1800ms] lg:object-center"
      />
      <div aria-hidden="true" className="absolute inset-0 bg-[linear-gradient(90deg,rgba(5,8,18,0.99)_0%,rgba(5,8,18,0.96)_24%,rgba(5,8,18,0.58)_55%,rgba(5,8,18,0.14)_78%,rgba(5,8,18,0.28)_100%)]" />
      <div aria-hidden="true" className="absolute inset-0 bg-gradient-to-b from-[#050812]/35 via-transparent to-[#050812]/95" />
      <div aria-hidden="true" className="absolute -right-32 top-1/4 size-[540px] animate-pulse rounded-full bg-orange-400/10 blur-[100px] motion-reduce:animate-none" />
      <div aria-hidden="true" className="absolute right-[10%] top-[15%] size-3 animate-ping bg-violet-500 shadow-[0_0_28px_rgba(139,92,246,0.8)] motion-reduce:animate-none" />
      <div aria-hidden="true" className="absolute inset-0 opacity-[0.055] mix-blend-screen [background-image:linear-gradient(rgba(255,255,255,0.16)_1px,transparent_1px)] [background-size:100%_4px]" />
    </div>
  );
}
