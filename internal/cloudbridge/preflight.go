package cloudbridge

import (
	"context"
	"fmt"
)

// Worker block codes, as the portal's operator panel understands them.
const (
	blockToolsMissing     = "tools_missing"
	blockSteamUnavailable = "steam_unavailable"
	blockCS2Running       = "cs2_running"
	blockDiskFull         = "disk_full"
)

// DefaultMinFreeBytes is the free space a worker keeps before claiming: a
// raw capture of one match is several gigabytes.
const DefaultMinFreeBytes = 30 << 30

// preflight decides whether this machine can take a capture right now. It
// returns the health it measured and, when the machine cannot, why. A check
// that cannot be run blocks: claiming blind would fail the user's job.
func preflight(ctx context.Context, machine Machine, minFreeBytes uint64) (Health, *Block) {
	health := machine.Health(ctx, false)
	if !health.RecordEnabled {
		return health, &Block{Code: blockToolsMissing, Detail: "the recorder, HLAE or CS2 is not installed or not found"}
	}
	if !health.SteamRunning {
		return health, &Block{Code: blockSteamUnavailable, Detail: "steam.exe is not running in this session"}
	}
	cs2Running, err := machine.CS2Running(ctx)
	if err != nil {
		return health, &Block{Code: blockCS2Running, Detail: "could not check for a running cs2.exe: " + err.Error()}
	}
	if cs2Running {
		return health, &Block{Code: blockCS2Running, Detail: "cs2.exe is already running"}
	}
	if health.DiskFreeBytes < minFreeBytes {
		return health, &Block{
			Code:   blockDiskFull,
			Detail: fmt.Sprintf("%d bytes free of %d, below the floor of %d", health.DiskFreeBytes, health.DiskTotalBytes, minFreeBytes),
		}
	}
	return health, nil
}
