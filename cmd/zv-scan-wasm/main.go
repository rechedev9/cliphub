//go:build js && wasm

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"syscall/js"

	"github.com/rechedev9/cliphub/internal/tactical"
)

// sliceBytes is the size of each FileReaderSync read. The demo is streamed in
// slices this size, so the whole file is never copied into Go memory at once.
const sliceBytes = 4 * 1024 * 1024

func main() {
	js.Global().Set("zvScanDemo", js.FuncOf(zvScanDemo))
	select {}
}

// zvScanDemo scans one .dem, .dem.zst or .dem.gz File inside a Web Worker (FileReaderSync is worker only)
// and returns the tactical artifacts, the decompressed size and the name Studio would store.
func zvScanDemo(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return failResult("zvScanDemo espera un archivo y un callback de progreso")
	}
	blob := args[0]
	onProgress := args[1]
	if blob.Type() != js.TypeObject || onProgress.Type() != js.TypeFunction {
		return failResult("zvScanDemo recibió argumentos no válidos")
	}
	reader := js.Global().Get("FileReaderSync")
	if reader.IsUndefined() {
		return failResult("el escaneo en el navegador solo funciona dentro de un Web Worker")
	}

	fileName := ""
	if name := blob.Get("name"); name.Type() == js.TypeString {
		fileName = name.String()
	}

	source := &blobReader{blob: blob, fr: reader.New(), size: blob.Get("size").Int(), onProgress: onProgress}
	stream, err := openDemo(source)
	if err != nil {
		return failResult(err.Error())
	}
	defer stream.Close()
	result, err := scanDemo(stream)
	if err := stream.finish(err); err != nil {
		return failResult(err.Error())
	}
	if !result.Document.Geometry.Calibration.Valid() {
		return failResult(fmt.Sprintf("el mapa %q no tiene una calibración de radar válida", result.Document.Demo.Map))
	}

	document, err := json.MarshalIndent(result.Document, "", "  ")
	if err != nil {
		return failResult(err.Error())
	}
	positions := js.Global().Get("Uint8Array").New(len(result.Positions.Data))
	js.CopyBytesToJS(positions, result.Positions.Data)

	return map[string]any{
		"ok":        true,
		"sha256":    result.Document.Demo.SHA256,
		"map":       result.Document.Demo.Map,
		"rounds":    len(result.Document.Rounds),
		"document":  string(document) + "\n",
		"positions": positions,
		"cs2Build":  result.Header.BuildNum,
		"sizeBytes": stream.n,
		"fileName":  uploadName(fileName),
	}
}

// scanDemo runs the tactical scan and turns a panic into the failure shape.
func scanDemo(r io.Reader) (result tactical.Result, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("error interno del escáner: %v", rec)
		}
	}()
	return tactical.ScanReader(context.Background(), r, tactical.Options{})
}

func failResult(msg string) map[string]any {
	return map[string]any{"ok": false, "error": msg}
}

// blobReader streams a Blob into io.Reader one slice at a time through a
// synchronous FileReaderSync, reporting progress after every slice.
type blobReader struct {
	blob       js.Value
	fr         js.Value
	size       int
	next       int
	buf        []byte
	onProgress js.Value
}

func (r *blobReader) Read(p []byte) (int, error) {
	if len(r.buf) == 0 {
		if r.next >= r.size {
			return 0, io.EOF
		}
		if err := r.fill(); err != nil {
			return 0, err
		}
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

func (r *blobReader) fill() error {
	end := r.next + sliceBytes
	if end > r.size {
		end = r.size
	}
	arrayBuffer := r.fr.Call("readAsArrayBuffer", r.blob.Call("slice", r.next, end))
	view := js.Global().Get("Uint8Array").New(arrayBuffer)
	n := view.Length()
	if cap(r.buf) < n {
		r.buf = make([]byte, n)
	}
	r.buf = r.buf[:n]
	js.CopyBytesToGo(r.buf, view)
	r.next = end
	r.onProgress.Invoke(r.next, r.size)
	return nil
}
