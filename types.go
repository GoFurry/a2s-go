package a2s

import "fmt"

// ServerType identifies the server host mode reported by A2S_INFO.
type ServerType byte

const (
	ServerTypeDedicated    ServerType = 'd'
	ServerTypeNonDedicated ServerType = 'l'
	ServerTypeSourceTV     ServerType = 'p'
)

func (s ServerType) String() string {
	switch s {
	case ServerTypeDedicated:
		return "dedicated"
	case ServerTypeNonDedicated:
		return "non_dedicated"
	case ServerTypeSourceTV:
		return "source_tv"
	default:
		return fmt.Sprintf("unknown(0x%X)", byte(s))
	}
}

// Environment identifies the server OS reported by A2S_INFO.
type Environment byte

const (
	EnvironmentLinux   Environment = 'l'
	EnvironmentWindows Environment = 'w'
	EnvironmentMacOS   Environment = 'm'
	EnvironmentMacOSX  Environment = 'o'
)

func (e Environment) String() string {
	switch e {
	case EnvironmentLinux:
		return "linux"
	case EnvironmentWindows:
		return "windows"
	case EnvironmentMacOS, EnvironmentMacOSX:
		return "macos"
	default:
		return fmt.Sprintf("unknown(0x%X)", byte(e))
	}
}

// TheShipMode is The Ship-specific game mode metadata from A2S_INFO.
type TheShipMode byte

const (
	TheShipModeHunt            TheShipMode = 0
	TheShipModeElimination     TheShipMode = 1
	TheShipModeDuel            TheShipMode = 2
	TheShipModeDeathmatch      TheShipMode = 3
	TheShipModeTeamVIP         TheShipMode = 4
	TheShipModeTeamElimination TheShipMode = 5
	TheShipModeUnknown         TheShipMode = 255
)

func (m TheShipMode) String() string {
	switch m {
	case TheShipModeHunt:
		return "hunt"
	case TheShipModeElimination:
		return "elimination"
	case TheShipModeDuel:
		return "duel"
	case TheShipModeDeathmatch:
		return "deathmatch"
	case TheShipModeTeamVIP:
		return "team_vip"
	case TheShipModeTeamElimination:
		return "team_elimination"
	default:
		return "unknown"
	}
}

// TheShipInfo carries The Ship-only metadata from A2S_INFO.
type TheShipInfo struct {
	Mode      TheShipMode
	Witnesses uint8
	Duration  uint8
}

// Info matches the public A2S_INFO contract.
type Info struct {
	Protocol    uint8
	Name        string
	Map         string
	Folder      string
	Game        string
	AppID       uint16
	Players     uint8
	MaxPlayers  uint8
	Bots        uint8
	ServerType  ServerType
	Environment Environment
	Visibility  bool
	VAC         bool
	Version     string
	EDF         uint8
	Port        uint16
	SteamID     uint64
	Keywords    string
	GameID      uint64
	TVPort      uint16
	TVName      string
	TheShip     *TheShipInfo
}

// Players matches the public A2S_PLAYER contract.
type Players struct {
	Count   uint8
	Players []Player
}

// TheShipPlayer carries The Ship-only metadata from A2S_PLAYER.
type TheShipPlayer struct {
	Deaths uint32
	Money  uint32
}

// Player matches one A2S_PLAYER entry.
type Player struct {
	Index    uint8
	Name     string
	Score    int32
	Duration float32
	TheShip  *TheShipPlayer
}

// Rules matches the public A2S_RULES contract.
type Rules struct {
	Count         uint16
	ReportedCount uint16
	Truncated     bool
	Items         map[string]string
}
