package httpapi

import "github.com/rechedev9/cliphub/internal/library"

// cleanTitle trims a sent title in place, so an empty one reaches the store as a clear.
func (req *patchLibraryDemoRequest) cleanTitle() error {
	if req.Title == nil {
		return nil
	}
	title, err := library.CleanDemoTitle(*req.Title)
	req.Title = &title
	return err
}
