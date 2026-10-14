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
// keyID/seed. Members still holding the old cached sender key can no longer
// derive the new one, and the one-way hash chain means they cannot read the
// message sent with the new key even if they later receive it via a normal
// message. Call this before a restricted (GroupParticipants) send to make the
// exclusion hold against members who already cached the sender key.
//
// Storing an empty record is enough: LoadSenderKey returns it as IsEmpty(), so
// Create treats it as absent and regenerates.
func (cli *Client) ResetGroupSenderKey(ctx context.Context, group types.JID) error {
	senderKeyName := protocol.NewSenderKeyName(group.String(), cli.getOwnLID().SignalAddress())
	empty := groupRecord.NewSenderKey(pbSerializer.SenderKeyRecord, pbSerializer.SenderKeyState)
	return cli.Store.StoreSenderKey(ctx, senderKeyName, empty)
}
