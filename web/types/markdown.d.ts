// next.config.mjs loads `.md` files as their source text.
declare module '*.md' {
  const source: string;
  export default source;
}
