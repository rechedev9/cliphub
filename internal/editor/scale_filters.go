package editor

import "fmt"

func baseGeometryFilters(short ShortEdit, height string, singleCrop, fullFrame bool) []string {
	width, outHeight := outputDimensions(short)
	if !singleCrop && isPlainIntegerHeight(height) {
		return []string{
			fmt.Sprintf("crop='trunc(min(iw\\,ih*%d/%d)/2)*2':'trunc(min(ih\\,iw*%d/%d)/2)*2':(iw-ow)/2:(ih-oh)/2", width, outHeight, outHeight, width),
			exactScaleFilter(width, outHeight, short),
		}
	}
	filters := []string{scaleFilter(height, short)}
	if fullFrame {
		filters = []string{fullFrameBackgroundScaleFilter(short, height)}
	}
	if !singleCrop {
		filters = append(filters, fmt.Sprintf("crop=%d:%d:(iw-ow)/2:(ih-oh)/2", width, outHeight))
	}
	return filters
}

func scaleFilter(height string, short ShortEdit) string {
	filter := fmt.Sprintf("scale=w=-2:h=%s:%s", height, scaleEvalMode(height))
	if short.HQFilters {
		filter += ":flags=" + hqScaleFlags(short)
	}
	return filter
}

func scaleEvalMode(height string) string {
	if isPlainIntegerHeight(height) {
		return "eval=init"
	}
	return "eval=frame"
}

func isPlainIntegerHeight(height string) bool {
	if height == "" {
		return false
	}
	for _, r := range height {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func fullFrameBackgroundScaleFilter(short ShortEdit, heightExpr string) string {
	width, _ := outputDimensions(short)
	filter := fmt.Sprintf("scale=w=%d:h=%s:force_original_aspect_ratio=increase:%s", width, heightExpr, scaleEvalMode(heightExpr))
	if short.HQFilters {
		filter += ":flags=" + hqScaleFlags(short)
	}
	return filter
}

func exactScaleFilter(width, height int, short ShortEdit) string {
	filter := fmt.Sprintf("scale=w=%d:h=%d:eval=init", width, height)
	if short.HQFilters {
		filter += ":flags=" + hqScaleFlags(short)
	}
	return filter
}

func hqScaleFlags(short ShortEdit) string {
	return "lanczos+accurate_rnd+full_chroma_int"
}
