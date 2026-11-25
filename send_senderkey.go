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

// oneShotSenderKeyStore holds a single sender key record in memory for one
// restricted group send (SendRequestExtra.GroupParticipants). It starts empty,
// so SessionBuilder.Create generates a fresh keyID/seed/signing key at
// iteration 0, and it is dropped after the send: the key never reaches the
// persistent store, so no other send can distribute or reuse it.
type oneShotSenderKeyStore struct {
	rec *groupRecord.SenderKey
}

func (s *oneShotSenderKeyStore) StoreSenderKey(_ context.Context, _ *protocol.SenderKeyName, rec *groupRecord.SenderKey) error {
	s.rec = rec
	return nil
}

func (s *oneShotSenderKeyStore) LoadSenderKey(_ context.Context, _ *protocol.SenderKeyName) (*groupRecord.SenderKey, error) {
	if s.rec == nil {
		return groupRecord.NewSenderKey(pbSerializer.SenderKeyRecord, pbSerializer.SenderKeyState), nil
	}
	return s.rec, nil
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
