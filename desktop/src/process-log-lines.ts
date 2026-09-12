import { StringDecoder } from 'node:string_decoder';

/** Frame pipe chunks before adding source tags or filtering credentials. */
export class ProcessLogLines {
  private readonly decoder = new StringDecoder('utf8');
  private pending = '';
  private discarding = false;
  private privateKey = false;
  private readonly emit: (text: string) => void;

  constructor(emit: (text: string) => void) { this.emit = emit; }

  write(chunk: Buffer): void { this.accept(this.decoder.write(chunk)); }

  end(): void {
    this.accept(this.decoder.end());
    if (this.privateKey) this.emit('[diagnostic-gap] unterminated private-key block omitted\n');
    if (this.pending && !this.discarding && !this.privateKey) this.emit(this.pending + '\n');
    this.pending = '';
    this.privateKey = false;
  }

  private accept(text: string): void {
    for (const part of text.match(/[^\n]*\n|[^\n]+$/g) ?? []) {
      const complete = part.endsWith('\n');
      if (!this.discarding) {
        if (this.pending.length + part.length > 64 * 1024) {
          this.pending = '';
          this.discarding = true;
          this.emit('[diagnostic-gap] subprocess line exceeded 64 KiB; omitted\n');
        } else this.pending += part;
      }
      if (complete) {
        if (/-----BEGIN [A-Z ]*PRIVATE KEY-----/.test(this.pending)) {
          this.privateKey = true;
          this.emit('[credential]\n');
        }
        if (!this.privateKey && !this.discarding) this.emit(this.pending);
        if (/-----END [A-Z ]*PRIVATE KEY-----/.test(this.pending)) this.privateKey = false;
        this.pending = '';
        this.discarding = false;
      }
    }
  }
}
