package editor

import (
	"context"
	"fmt"
	"image"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/rooted"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/ast"
	"github.com/yuin/gopher-lua/parse"

	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/keydropbanner"
	"github.com/rechedev9/cliphub/internal/mediafont"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

var effectsEvaluationTimeout = 500 * time.Millisecond

const (
	maxEffectsSourceBytes = 256 << 10
	maxEffectCallbacks    = 64
	maxCallbackExecutions = 512
	maxEffectsPerShort    = 2048
	maxEffectStringBytes  = 4096
	maxEffectASTNodes     = 4096
	effectsRegistrySize   = 1024
	effectsRegistryMax    = 8192
)
