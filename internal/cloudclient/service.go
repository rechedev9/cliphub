package cloudclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/rechedev9/cliphub/internal/rooted"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/rules"
)

// Portal job statuses the client reacts to.
const (
	portalAwaitingDemo = "awaiting_demo"
	portalQueued       = "queued"
	portalDone         = "done"
	portalFailed       = "failed"
	portalCanceled     = "canceled"

	demoUploadRequired = "required"
	artifactVideo      = "video"

	accountRefreshTimeout = 5 * time.Second

	// codeUploadInProgress is the portal still reading an earlier upload of the
	// same demo, usually one whose connection this PC already lost.
	codeUploadInProgress = "upload_in_progress"

	// maxDemoUploads is how many demo uploads the portal takes from one user
	// at once.
	maxDemoUploads = 2
	// maxDownloadMismatches is how often a result may arrive with the wrong
	// sha256 before the job fails on this PC.
	maxDownloadMismatches = 3
	// retryWatchWindow is how long a failed or canceled job is checked for an
	// operator retry. The portal lists finished jobs for 14 days.
	retryWatchWindow = 14 * 24 * time.Hour
	// abandonedJobLifetime bounds the cancel of an abandoned job: the portal
	// deletes a job that never got its demo after 24 h.
	abandonedJobLifetime = 25 * time.Hour
)

// Local statuses and failure codes: phases only this PC knows about.
const (
	StatusUploadingDemo = "uploading_demo"
	StatusDownloading   = "downloading"

	// FailureDemoUpload means the demo never reached the portal.
	FailureDemoUpload = "demo_upload_failed"
	// FailureResultsExpired means the job finished but the portal deleted
	// its videos before this Studio downloaded them.
	FailureResultsExpired = "results_expired"
	// FailureDownload means the result arrived damaged every time.
	FailureDownload = "download_failed"
	// Refusals of the demo upload that the UI words on their own.
	FailureLimitUploads = "limit_uploads"
	FailureLimitStorage = "limit_storage"
	FailureStorageFull  = "cloud_storage_full"

	// Why a job that has not finished is not advancing (LocalJob.Stalled).
	StalledPortal     = "portal_unreachable"
	StalledDemoUpload = "demo_upload"
	StalledDownload   = "download"

	errorPortalUnreachable = "portal_unreachable"
	errorUnauthorized      = "unauthorized"
)

// localFailureMessages is what the user reads for a failure the portal does
// not know about.
var localFailureMessages = map[string]string{
	FailureDemoUpload:     "No se pudo subir la demo a la nube.",
	FailureResultsExpired: "Los vídeos de este trabajo ya no están disponibles en la nube.",
	FailureDownload:       "El vídeo llegó dañado a este PC varias veces y no se pudo guardar.",
	FailureLimitUploads:   "Ya hay dos demos tuyas subiendo a la nube.",
	FailureLimitStorage:   "Tus demos ocupan todo tu espacio en la nube.",
	FailureStorageFull:    "La nube se ha quedado sin espacio por ahora.",
}
