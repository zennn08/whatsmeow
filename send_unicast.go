// Copyright (c) 2024 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// SendGroupUnicast sends message addressed to a group but delivered ONLY to the
// devices of the given recipients, via the participant attribute — the same
// delivery path WhatsApp uses for retry responses. The content is pairwise
// (1:1) encrypted per device; there is NO sender-key (skmsg) node, so other
// group members receive nothing at all (no "waiting for this message"
// placeholder, no decoy needed). Each target device gets its own stanza under a
// shared message ID.
//
// recipients must be given in the group's own addressing (LID in a LID group)
// so the participant attribute matches how the group routes them. All target
// devices share the returned message ID.
//
// EXPERIMENTAL: sending a participant-addressed stanza proactively (not as a
// response to a retry receipt) is not a documented WhatsApp flow. The server may
// drop it, and whether a target client renders it as a group message is
// client-dependent. Verify on a device before relying on it.
func (cli *Client) SendGroupUnicast(ctx context.Context, group types.JID, recipients []types.JID, message *waE2E.Message) (types.MessageID, error) {
	id := cli.GenerateMessageID()
	plaintext, err := proto.Marshal(message)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}
	devices, err := cli.GetUserDevices(ctx, recipients)
	if err != nil {
		return "", fmt.Errorf("failed to get device list: %w", err)
	}

	ownJID := cli.getOwnID()
	ownLID := cli.getOwnLID()

	// Prefetch prekey bundles for devices without an existing session.
	var needPreKeys []types.JID
	for _, d := range devices {
		if d == ownJID || d == ownLID {
			continue
		}
		if exists, cErr := cli.Store.ContainsSession(ctx, d.SignalAddress()); cErr == nil && !exists {
			needPreKeys = append(needPreKeys, d)
		}
	}
	bundles := cli.fetchPreKeysNoError(ctx, needPreKeys)

	msgType := getTypeFromMessage(message)
	var sent int
	for _, d := range devices {
		if d == ownJID || d == ownLID {
			continue
		}
		enc, includeIdentity, encErr := cli.encryptMessageForDevice(ctx, plaintext, d, bundles[d], waBinary.Attrs{}, nil)
		if encErr != nil {
			cli.Log.Warnf("SendGroupUnicast: failed to encrypt %s for %s: %v", id, d, encErr)
			continue
		}
		attrs := waBinary.Attrs{
			"to":          group,
			"type":        msgType,
			"id":          id,
			"participant": d,
		}
		if d.Server == types.HiddenUserServer {
			attrs["addressing_mode"] = string(types.AddressingModeLID)
		}
		content := cli.getMessageContent(*enc, message, attrs, includeIdentity, nodeExtraParams{})
		if sErr := cli.sendNode(ctx, waBinary.Node{Tag: "message", Attrs: attrs, Content: content}); sErr != nil {
			cli.Log.Warnf("SendGroupUnicast: failed to send %s to %s: %v", id, d, sErr)
			continue
		}
		sent++
	}
	if sent == 0 {
		return "", fmt.Errorf("SendGroupUnicast: no stanza delivered to any target device")
	}
	return id, nil
}
