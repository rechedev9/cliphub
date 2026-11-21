package tuiclient

import "context"

type SharePackageImport struct {
	Kind            string `json:"kind"`
	Title           string `json:"title"`
	Notes           string `json:"notes"`
	ReportID        string `json:"report_id,omitempty"`
	Duplicate       bool   `json:"duplicate"`
	MatchesImported int    `json:"matches_imported"`
	MatchesExisting int    `json:"matches_existing"`
	Annotations     int    `json:"annotations"`
}

func (c *Client) ImportSharePackage(ctx context.Context, filePath string) (SharePackageImport, error) {
	var out SharePackageImport
	err := c.uploadMultipart(ctx, upload{Path: "/api/library/import-package", FileField: "package", FilePath: filePath}, &out)
	return out, err
}
