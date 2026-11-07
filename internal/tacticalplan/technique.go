package tacticalplan

import "math"

// Speeds in world units per second that split a throw's movement. CS2 walks
// at about 130 with a grenade out and runs at about 245.
const (
	standingMaxSpeed = 40
	walkingMaxSpeed  = 175
)

// Movement words match library.Movement*.
const (
	techniqueStanding  = "standing"
	techniqueWalking   = "walking"
	techniqueRunning   = "running"
	techniqueCrouching = "crouching"
)

// Technique is how a throw was made, as far as 8 Hz samples tell it.
type Technique struct {
	Movement string
	Jump     bool
}

// SetangPitch brings a demo pitch into setang's -90 (up) to 90 (down): demos
// write looking up as 270 to 360.
func SetangPitch(deg float64) float64 {
	if math.IsNaN(deg) || math.IsInf(deg, 0) {
		return 0
	}
	deg = math.Mod(deg, 360)
	if deg > 180 {
		deg -= 360
	} else if deg < -180 {
		deg += 360
	}
	return min(max(deg, -90), 90)
}

// SetangYaw brings a yaw into -180 to 180.
func SetangYaw(deg float64) float64 {
	if math.IsNaN(deg) || math.IsInf(deg, 0) {
		return 0
	}
	deg = math.Mod(deg, 360)
	if deg > 180 {
		deg -= 360
	} else if deg <= -180 {
		deg += 360
	}
	return deg
}

// EstimateTechnique reads the thrower's samples just before and at the throw:
// airborne means a jump throw, ducking a crouched one, and the speed between
// the two samples splits standing, walking and running.
func EstimateTechnique(before, at Sample, seconds float64) Technique {
	t := Technique{Jump: at.Flags&FlagAirborne != 0}
	if at.Flags&FlagDucking != 0 {
		t.Movement = techniqueCrouching
		return t
	}
	speed := 0.0
	if seconds > 0 {
		speed = math.Hypot(at.X-before.X, at.Y-before.Y) / seconds
	}
	switch {
	case speed <= standingMaxSpeed:
		t.Movement = techniqueStanding
	case speed <= walkingMaxSpeed:
		t.Movement = techniqueWalking
	default:
		t.Movement = techniqueRunning
	}
	return t
}
