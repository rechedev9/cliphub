package workers

import (
	"context"
	"encoding/json"
	"io"
	"path"
	"reflect"
	"sort"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

func fullDemoCachedArtifacts(ctx context.Context, store storage.Storage, state renderplan.RenderVariantState, short editor.ShortResult) ([]string, bool, error) {
	if short.FullDemo == nil || !renderplan.SameFullDemoRequest(state.FullDemo, &short.FullDemo.Approved) {
		return nil, false, nil
	}
	ref, err := renderplan.NewRenderVariantArtifactRefForState(state, renderplan.RenderVariantArtifactVideo, short.SegmentID)
	if err != nil {
		return nil, false, err
	}
	if err := mediaassets.VerifyContent(ctx, store, ref.Key, short.FullDemo.Delivery.ContentSHA256, 64<<30); err != nil {
		return nil, false, ctx.Err()
	}
	prefix := path.Dir(path.Dir(ref.Key))
	var keys []string
	for name, expected := range editor.FullDemoDocumentFiles(*short.FullDemo) {
		key := path.Join(prefix, name)
		reader, err := store.Open(key)
		if err != nil {
			if storage.IsNotExist(err) {
				return nil, false, nil
			}
			return nil, false, err
		}
		body, readErr := io.ReadAll(io.LimitReader(reader, (4<<20)+1))
		closeErr := reader.Close()
		if readErr != nil {
			return nil, false, readErr
		}
		if closeErr != nil {
			return nil, false, closeErr
		}
		if len(body) > 4<<20 {
			return nil, false, nil
		}
		want, err := json.Marshal(expected)
		if err != nil {
			return nil, false, err
		}
		var actualValue, expectedValue any
		if json.Unmarshal(body, &actualValue) != nil {
			return nil, false, nil
		}
		if err := json.Unmarshal(want, &expectedValue); err != nil {
			return nil, false, err
		}
		if !reflect.DeepEqual(actualValue, expectedValue) {
			return nil, false, nil
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, true, nil
}
