package whatsmeow

import (
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestKeepDevice(t *testing.T) {
	dev := func(user string, d uint16) types.JID {
		return types.JID{User: user, Server: types.HiddenUserServer, Device: d}
	}
	cases := []struct {
		mode GroupDeviceMode
		jid  types.JID
		want bool
	}{
		{GroupDevicePrimary, dev("9", 0), true},
		{GroupDevicePrimary, dev("9", 3), false},
		{GroupDeviceSecondary, dev("9", 0), false},
		{GroupDeviceSecondary, dev("9", 3), true},
		{GroupDeviceAll, dev("9", 3), true},
		{GroupDeviceSecondary, dev("own", 0), true}, // own devices untouched
	}
	for _, c := range cases {
		if got := keepDevice(c.mode, c.jid, "ownpn", "own"); got != c.want {
			t.Errorf("keepDevice(%q, %v) = %v, want %v", c.mode, c.jid, got, c.want)
		}
	}
}
