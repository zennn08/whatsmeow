// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestShouldIssueTCTokenToLID(t *testing.T) {
	cases := []struct {
		name string
		flag bool
		ts   int64
		want bool
	}{
		{"flag off, not migrated", false, 0, false},
		{"flag off, migrated", false, 1700000000, true},
		{"flag on, not migrated", true, 0, true},
		{"flag on, migrated", true, 1700000000, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldIssueTCTokenToLID(c.flag, c.ts); got != c.want {
				t.Errorf("shouldIssueTCTokenToLID(%v, %d) = %v, want %v", c.flag, c.ts, got, c.want)
			}
		})
	}
}

func TestIsTCTokenStorableUser(t *testing.T) {
	cases := []struct {
		name string
		jid  types.JID
		want bool
	}{
		{"normal PN", types.NewJID("12345", types.DefaultUserServer), true},
		{"normal LID", types.NewJID("67890", types.HiddenUserServer), true},
		{"PSA", types.PSAJID, false},
		{"bot 1313555xxxx", types.NewJID("13135550001", types.DefaultUserServer), false},
		{"MetaAI", types.MetaAIJID, false},
		{"group", types.NewJID("12345", types.GroupServer), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isTCTokenStorableUser(c.jid); got != c.want {
				t.Errorf("isTCTokenStorableUser(%s) = %v, want %v", c.jid, got, c.want)
			}
		})
	}
}
