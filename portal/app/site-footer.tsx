const LINK = "text-fg-2 transition-colors duration-(--dur-fast) hover:text-primary";

export function SiteFooter() {
  return (
    <footer className="mt-14 flex flex-wrap items-center gap-3 border-t border-border-subtle pt-5 text-body-sm text-fg-3">
      <span>ClipHub</span>
      <span aria-hidden="true">·</span>
      <a className={LINK} href="/">
        Inicio
      </a>
      <a className={LINK} href="/link">
        Vincular Studio
      </a>
      <a className={LINK} href="/privacy">
        Privacidad
      </a>
      <a className={LINK} href="/terms">
        Condiciones
      </a>
    </footer>
  );
}
