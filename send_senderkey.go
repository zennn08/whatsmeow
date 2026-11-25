// Copyright (c) 2024 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"

	groupRecord "go.mau.fi/libsignal/groups/state/record"
	"go.mau.fi/libsignal/protocol"

	"go.mau.fi/whatsmeow/types"
)

// ResetGroupSenderKey discards our own sender key for the group, so the NEXT
// group send (sendGroup → builder.Create) generates and distributes a fresh
// keyID/seed. Call this before a restricted (GroupParticipants) send.
//
// Storing an empty record is enough: LoadSenderKey returns it as IsEmpty(), so
// Create treats it as absent and regenerates.
func (cli *Client) ResetGroupSenderKey(ctx context.Context, group types.JID) error {
	senderKeyName := protocol.NewSenderKeyName(group.String(), cli.getOwnLID().SignalAddress())
	empty := groupRecord.NewSenderKey(pbSerializer.SenderKeyRecord, pbSerializer.SenderKeyState)
	return cli.Store.StoreSenderKey(ctx, senderKeyName, empty)
}

// keepDevice reports whether a restricted group send in the given mode goes to
// jid. Our own devices are always kept here (GroupIncludeOwnDevices decides
// whether they are recipients at all).
func keepDevice(mode GroupDeviceMode, jid types.JID, ownPN, ownLID string) bool {
	if jid.User == ownPN || jid.User == ownLID {
		return true
	}
	switch mode {
	case GroupDevicePrimary:
		return jid.Device == 0
	case GroupDeviceSecondary:
		return jid.Device != 0
	}
	return true
}
