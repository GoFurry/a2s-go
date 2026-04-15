package a2s

import (
	ierrors "github.com/GoFurry/a2s-go/internal/errors"
	"github.com/GoFurry/a2s-go/internal/protocol"
)

func parseInfo(packet []byte) (*Info, error) {
	r := protocol.NewReader(packet)
	if header, ok := r.Int32(); !ok || header != -1 {
		return nil, ierrors.ErrPacketHeader
	}
	if kind, ok := r.Uint8(); !ok || kind != protocol.HeaderInfo {
		return nil, ierrors.ErrUnsupported
	}

	info := &Info{}
	var ok bool

	if info.Protocol, ok = r.Uint8(); !ok {
		return nil, ierrors.ErrDecode
	}
	if info.Name, ok = r.String(); !ok {
		return nil, ierrors.ErrDecode
	}
	if info.Map, ok = r.String(); !ok {
		return nil, ierrors.ErrDecode
	}
	if info.Folder, ok = r.String(); !ok {
		return nil, ierrors.ErrDecode
	}
	if info.Game, ok = r.String(); !ok {
		return nil, ierrors.ErrDecode
	}
	if info.AppID, ok = r.Uint16(); !ok {
		return nil, ierrors.ErrDecode
	}
	if info.Players, ok = r.Uint8(); !ok {
		return nil, ierrors.ErrDecode
	}
	if info.MaxPlayers, ok = r.Uint8(); !ok {
		return nil, ierrors.ErrDecode
	}
	if info.Bots, ok = r.Uint8(); !ok {
		return nil, ierrors.ErrDecode
	}
	serverType, ok := r.Uint8()
	if !ok {
		return nil, ierrors.ErrDecode
	}
	info.ServerType = ServerType(serverType)
	environment, ok := r.Uint8()
	if !ok {
		return nil, ierrors.ErrDecode
	}
	info.Environment = Environment(environment)
	visibility, ok := r.Uint8()
	if !ok {
		return nil, ierrors.ErrDecode
	}
	info.Visibility = visibility == 1
	vac, ok := r.Uint8()
	if !ok {
		return nil, ierrors.ErrDecode
	}
	info.VAC = vac == 1
	if info.AppID == 2400 {
		mode, ok := r.Uint8()
		if !ok {
			return nil, ierrors.ErrDecode
		}
		witnesses, ok := r.Uint8()
		if !ok {
			return nil, ierrors.ErrDecode
		}
		duration, ok := r.Uint8()
		if !ok {
			return nil, ierrors.ErrDecode
		}
		info.TheShip = &TheShipInfo{
			Mode:      TheShipMode(mode),
			Witnesses: witnesses,
			Duration:  duration,
		}
	}
	if info.Version, ok = r.String(); !ok {
		return nil, ierrors.ErrDecode
	}
	if !r.Remaining() {
		return info, nil
	}

	if info.EDF, ok = r.Uint8(); !ok {
		return nil, ierrors.ErrDecode
	}
	if info.EDF&0x80 != 0 {
		if info.Port, ok = r.Uint16(); !ok {
			return nil, ierrors.ErrDecode
		}
	}
	if info.EDF&0x10 != 0 {
		if info.SteamID, ok = r.Uint64(); !ok {
			return nil, ierrors.ErrDecode
		}
	}
	if info.EDF&0x40 != 0 {
		if info.TVPort, ok = r.Uint16(); !ok {
			return nil, ierrors.ErrDecode
		}
		if info.TVName, ok = r.String(); !ok {
			return nil, ierrors.ErrDecode
		}
	}
	if info.EDF&0x20 != 0 {
		if info.Keywords, ok = r.String(); !ok {
			return nil, ierrors.ErrDecode
		}
	}
	if info.EDF&0x01 != 0 {
		if info.GameID, ok = r.Uint64(); !ok {
			return nil, ierrors.ErrDecode
		}
	}
	return info, nil
}

func parsePlayers(packet []byte) (*Players, error) {
	r := protocol.NewReader(packet)
	if header, ok := r.Int32(); !ok || header != -1 {
		return nil, ierrors.ErrPacketHeader
	}
	if kind, ok := r.Uint8(); !ok || kind != protocol.HeaderPlayers {
		return nil, ierrors.ErrUnsupported
	}

	count, ok := r.Uint8()
	if !ok {
		return nil, ierrors.ErrDecode
	}
	out := &Players{
		Count:   count,
		Players: make([]Player, 0, count),
	}
	for i := 0; i < int(count); i++ {
		var player Player
		if player.Index, ok = r.Uint8(); !ok {
			return nil, ierrors.ErrDecode
		}
		if player.Name, ok = r.String(); !ok {
			return nil, ierrors.ErrDecode
		}
		if player.Score, ok = r.Int32(); !ok {
			return nil, ierrors.ErrDecode
		}
		if player.Duration, ok = r.Float32(); !ok {
			return nil, ierrors.ErrDecode
		}
		out.Players = append(out.Players, player)
	}
	if count > 0 && len(packet)-r.Pos() == int(count)*8 {
		for i := range out.Players {
			deaths, ok := r.Uint32()
			if !ok {
				return nil, ierrors.ErrDecode
			}
			money, ok := r.Uint32()
			if !ok {
				return nil, ierrors.ErrDecode
			}
			out.Players[i].TheShip = &TheShipPlayer{
				Deaths: deaths,
				Money:  money,
			}
		}
	}
	return out, nil
}

func parseRules(packet []byte) (*Rules, error) {
	r := protocol.NewReader(packet)
	if header, ok := r.Int32(); !ok || header != -1 {
		return nil, ierrors.ErrPacketHeader
	}
	if kind, ok := r.Uint8(); !ok || kind != protocol.HeaderRules {
		return nil, ierrors.ErrUnsupported
	}

	count, ok := r.Uint16()
	if !ok {
		return nil, ierrors.ErrDecode
	}
	out := &Rules{
		ReportedCount: count,
		Items:         make(map[string]string, count),
	}
	for i := 0; i < int(count); i++ {
		if !r.Remaining() {
			out.Truncated = true
			break
		}
		key, ok := r.String()
		if !ok {
			if len(out.Items) > 0 {
				out.Truncated = true
				break
			}
			return nil, ierrors.ErrDecode
		}
		value, ok := r.String()
		if !ok {
			if len(out.Items) > 0 {
				out.Truncated = true
				break
			}
			return nil, ierrors.ErrDecode
		}
		out.Items[key] = value
	}
	out.Count = uint16(len(out.Items))
	if out.Count < out.ReportedCount {
		out.Truncated = true
	}
	return out, nil
}
