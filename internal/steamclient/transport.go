package steamclient

import (
	"context"
	"fmt"
	"io"
	"time"

	steam "github.com/Philipp15b/go-steam/v3"
	"github.com/Philipp15b/go-steam/v3/protocol"
	"github.com/Philipp15b/go-steam/v3/protocol/gamecoordinator"
	"github.com/Philipp15b/go-steam/v3/protocol/steamlang"
	"google.golang.org/protobuf/proto"

	"github.com/rechedev9/cliphub/internal/steamgc"
	"github.com/rechedev9/cliphub/internal/steamresolve"
)

const csAppID = 730

const steamSessionTimeout = 60 * time.Second

// Transport opens a short-lived Steam session, asks the CS2 Game Coordinator
// for one match list, and disconnects. It cannot be exercised by the test
// suite: it needs a live account.
type Transport struct {
	session steamresolve.Session
}

// New returns the go-steam backed transport. session must be Complete().
func New(session steamresolve.Session) *Transport {
	return &Transport{session: session}
}

type rawGCMsg struct {
	appID  uint32
	header *steamlang.MsgGCHdrProtoBuf
	body   []byte
}

func newRawGCMsg(appID uint32, msgType steamgc.MsgID, body []byte) *rawGCMsg {
	header := steamlang.NewMsgGCHdrProtoBuf()
	header.Msg = uint32(msgType)
	return &rawGCMsg{appID: appID, header: header, body: body}
}

func (m *rawGCMsg) Serialize(w io.Writer) error {
	if err := m.header.Serialize(w); err != nil {
		return err
	}
	_, err := w.Write(m.body)
	return err
}

func (m *rawGCMsg) IsProto() bool      { return true }
func (m *rawGCMsg) GetAppId() uint32   { return m.appID }
func (m *rawGCMsg) GetMsgType() uint32 { return m.header.Msg }

func (m *rawGCMsg) GetTargetJobId() protocol.JobId {
	return protocol.JobId(m.header.Proto.GetJobidTarget())
}

func (m *rawGCMsg) SetTargetJobId(job protocol.JobId) {
	m.header.Proto.JobidTarget = proto.Uint64(uint64(job))
}

func (m *rawGCMsg) GetSourceJobId() protocol.JobId {
	return protocol.JobId(m.header.Proto.GetJobidSource())
}

func (m *rawGCMsg) SetSourceJobId(job protocol.JobId) {
	m.header.Proto.JobidSource = proto.Uint64(uint64(job))
}

type matchListResult struct {
	matches []steamgc.Match
	err     error
}

type matchListHandler struct {
	results chan matchListResult
}

func (h *matchListHandler) HandleGCPacket(packet *gamecoordinator.GCPacket) {
	if packet.AppId != csAppID || packet.MsgType != uint32(steamgc.MsgMatchList) {
		return
	}
	matches, err := steamgc.DecodeMatchList(packet.Body)
	select {
	case h.results <- matchListResult{matches: matches, err: err}:
	default:
	}
}

func (t *Transport) RequestMatch(ctx context.Context, req steamgc.Request) ([]steamgc.Match, error) {
	ctx, cancel := context.WithTimeout(ctx, steamSessionTimeout)
	defer cancel()

	client := steam.NewClient()
	defer client.Disconnect()

	handler := &matchListHandler{results: make(chan matchListResult, 1)}
	client.GC.RegisterPacketHandler(handler)

	if _, err := client.Connect(); err != nil {
		return nil, fmt.Errorf("connect to steam: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("steam session: %w", ctx.Err())
		case result := <-handler.results:
			if result.err != nil {
				return nil, fmt.Errorf("decode match list: %w", result.err)
			}
			return result.matches, nil
		case event, ok := <-client.Events():
			if !ok {
				return nil, fmt.Errorf("steam event stream closed before the match list arrived")
			}
			switch e := event.(type) {
			case *steam.ConnectedEvent:
				client.Auth.LogOn(&steam.LogOnDetails{
					Username:      t.session.Username,
					Password:      t.session.Password,
					TwoFactorCode: t.session.Guard,
				})
			case *steam.LoggedOnEvent:
				client.GC.SetGamesPlayed(csAppID)
				client.GC.Write(newRawGCMsg(csAppID, steamgc.MsgMatchListRequestFullGameInfo, steamgc.EncodeRequest(req)))
			case *steam.LogOnFailedEvent:
				return nil, fmt.Errorf("steam logon failed: result %v", e.Result)
			case *steam.DisconnectedEvent:
				return nil, fmt.Errorf("steam disconnected before the match list arrived")
			case steam.FatalErrorEvent:
				return nil, fmt.Errorf("steam session fatal error: %w", error(e))
			}
		}
	}
}
