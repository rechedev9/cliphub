// Classic Web Worker for the browser demo importer. It loads the Go runtime,
// instantiates the scanner, and runs one demo per message so FileReaderSync
// stays off the main thread.
let readyPromise = null;

function ensureReady() {
  readyPromise ??= (async () => {
    importScripts('/wasm/wasm_exec.js');
    const go = new Go();
    const { instance } = await WebAssembly.instantiateStreaming(
      fetch('/wasm/zv-scan.wasm'),
      go.importObject,
    );
    go.run(instance);
  })();
  return readyPromise;
}

self.onmessage = async (event) => {
  const { id, file } = event.data;
  try {
    await ensureReady();
    const result = zvScanDemo(file, (read, total) => {
      postMessage({ id, type: 'progress', read, total });
    });
    postMessage({ id, type: 'result', result }, result.ok ? [result.positions.buffer] : []);
  } catch (error) {
    postMessage({ id, type: 'result', result: { ok: false, error: error.message } });
  }
};
