# Port Perilaku tcToken Baileys → whatsmeow — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bawa empat perilaku tcToken baileys ke whatsmeow — gate AB-prop (config flag), filter "regular user" saat menyimpan token, dan pemilihan issuance JID (LID/PN) yang migration-aware — tanpa migrasi DB.

**Architecture:** Tambah tiga flag in-memory di `Client` (default menjaga perilaku lama). Pasang gate di call-site `send.go` & `user.go`. Pisahkan "storage JID" (tetap LID) dari "issuance JID" (mengikuti flag ATAU status migrasi LID) lewat helper baru di `tctoken.go`. Pakai ulang `shouldSendTCTokenInChatAction` sebagai filter storable-user di `notification.go` & `message.go`. Logika keputusan diekstrak ke fungsi murni agar bisa di-unit-test (package root whatsmeow tidak punya infra mock store).

**Tech Stack:** Go, package `go.mau.fi/whatsmeow`. Test: `go test` (testing standard library, table-driven). Spec sumber: `docs/superpowers/specs/2026-06-14-tctoken-baileys-port-design.md`.

---

## File Structure

| File | Tanggung jawab perubahan |
|---|---|
| `client.go` | Deklarasi 3 field flag pada struct `Client` + set default di `NewClient` |
| `tctoken.go` | Helper murni `shouldIssueTCTokenToLID`, alias `isTCTokenStorableUser`, method `resolveTCTokenIssuanceJID`; wiring issuance JID ke `issuePrivacyTokenAndSave` |
| `tctoken_internal_test.go` (baru) | Unit test table-driven untuk helper murni (`package whatsmeow`) |
| `send.go` | Gate attach `<tctoken>` 1:1 + gate issuance pasca-kirim dengan `PrivacyTokenOn1to1` |
| `user.go` | Gate attach `<tctoken>` foto profil dengan `ProfilePicPrivacyToken` |
| `notification.go` | Guard storable-user di `handlePrivacyTokenNotification` |
| `message.go` | Filter storable-user di `storeHistoricalMessageSecrets` |

**Catatan urutan kompilasi:** Task 1 (fields) mendahului Task 2/3 (memakai `cli.LIDTrustedTokenIssueToLID`), Task 4/5 (memakai `cli.PrivacyTokenOn1to1`/`ProfilePicPrivacyToken`), dan Task 6/7 (memakai `isTCTokenStorableUser`). Tiap task meninggalkan tree dalam keadaan build OK (fungsi/method tak terpakai diperbolehkan Go).

**Catatan commit:** Pesan commit TANPA co-author Claude (preferensi user).

---

## Task 1: Tambah config flag + default

**Files:**
- Modify: `client.go:181-184` (deklarasi field, dekat `ErrorOnSubscribePresenceWithoutToken`)
- Modify: `client.go:280-283` (struct literal di `NewClient`)

- [ ] **Step 1: Tambah deklarasi field**

Di `client.go`, ganti blok di sekitar baris 181-184:

```go
	// Should SubscribePresence return an error if no privacy token is stored for the user?
	ErrorOnSubscribePresenceWithoutToken bool

	// PrivacyTokenOn1to1 controls whether a <tctoken> is attached to outgoing 1:1 messages.
	// Mirrors baileys AB prop 10518. Default: true.
	PrivacyTokenOn1to1 bool
	// ProfilePicPrivacyToken controls whether a <tctoken> is attached to profile picture IQs.
	// Mirrors baileys AB prop 9666. Default: true.
	ProfilePicPrivacyToken bool
	// LIDTrustedTokenIssueToLID forces privacy token issuance to the LID even when the account
	// is not LID-migrated. When false, issuance follows the migration state (LID if migrated,
	// otherwise PN). Mirrors baileys AB prop 14303. Default: false.
	LIDTrustedTokenIssueToLID bool

	SendReportingTokens bool
```

(Blok di atas menyisipkan tiga field baru di antara `ErrorOnSubscribePresenceWithoutToken` dan `SendReportingTokens` yang sudah ada.)

- [ ] **Step 2: Set default di NewClient**

Di `client.go`, dalam struct literal `cli := &Client{ ... }` (sekitar baris 280-283), ganti:

```go
		EnableAutoReconnect: true,
		AutoTrustIdentity:   true,
```

menjadi:

```go
		EnableAutoReconnect: true,
		AutoTrustIdentity:   true,

		PrivacyTokenOn1to1:     true,
		ProfilePicPrivacyToken: true,
		// LIDTrustedTokenIssueToLID defaults to false (zero value).
```

- [ ] **Step 3: Build untuk verifikasi**

Run: `go build ./...`
Expected: sukses tanpa error (exit 0).

- [ ] **Step 4: Commit**

```bash
git add client.go
git commit -m "feat(tctoken): add PrivacyTokenOn1to1/ProfilePicPrivacyToken/LIDTrustedTokenIssueToLID flags"
```

---

## Task 2: Helper murni + resolver issuance JID (TDD)

**Files:**
- Create: `tctoken_internal_test.go`
- Modify: `tctoken.go` (tambah helper di akhir file, setelah `issuePrivacyToken`)

- [ ] **Step 1: Tulis test yang gagal**

Buat file `tctoken_internal_test.go`:

```go
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
```

- [ ] **Step 2: Jalankan test untuk memastikan GAGAL (belum compile)**

Run: `go test ./ -run 'TestShouldIssueTCTokenToLID|TestIsTCTokenStorableUser' -v`
Expected: GAGAL compile — `undefined: shouldIssueTCTokenToLID` dan `undefined: isTCTokenStorableUser`.

- [ ] **Step 3: Implementasi helper minimal**

Tambah di akhir `tctoken.go` (setelah fungsi `issuePrivacyToken`):

```go

// shouldIssueTCTokenToLID decides whether a privacy token should be issued to the LID.
// Issuance prefers the LID when the explicit flag is set OR the account is already
// LID-migrated; otherwise it prefers the PN.
func shouldIssueTCTokenToLID(issueToLIDFlag bool, lidMigrationTimestamp int64) bool {
	return issueToLIDFlag || lidMigrationTimestamp > 0
}

// isTCTokenStorableUser reports whether a privacy token received for the given JID should
// be stored. Mirrors baileys' isRegularUser: rejects PSA, bots (incl. MetaAI) and non-user
// servers. Delegates to the same predicate used for issuing tokens in chat actions.
func isTCTokenStorableUser(jid types.JID) bool {
	return shouldSendTCTokenInChatAction(jid)
}

// resolveTCTokenIssuanceJID returns the JID to put in the <token jid="..."> attribute when
// issuing a privacy token. Storage keying is unaffected (still by LID via
// resolveTCTokenStorageLID); this only chooses the wire identifier for the issuance IQ.
func (cli *Client) resolveTCTokenIssuanceJID(ctx context.Context, jid types.JID) types.JID {
	issueJID := jid.ToNonAD()
	if cli.Store == nil || cli.Store.LIDs == nil {
		return issueJID
	}
	if shouldIssueTCTokenToLID(cli.LIDTrustedTokenIssueToLID, cli.Store.LIDMigrationTimestamp) {
		if issueJID.Server == types.HiddenUserServer {
			return issueJID
		}
		lid, err := cli.Store.LIDs.GetLIDForPN(ctx, issueJID)
		if err != nil {
			cli.Log.Debugf("Failed to resolve LID for tctoken issuance JID %s: %v", issueJID, err)
			return issueJID
		}
		if lid.IsEmpty() {
			return issueJID
		}
		return lid.ToNonAD()
	}
	if issueJID.Server == types.HiddenUserServer {
		pn, err := cli.Store.LIDs.GetPNForLID(ctx, issueJID)
		if err != nil {
			cli.Log.Debugf("Failed to resolve PN for tctoken issuance JID %s: %v", issueJID, err)
			return issueJID
		}
		if pn.IsEmpty() {
			return issueJID
		}
		return pn.ToNonAD()
	}
	return issueJID
}
```

(`context` dan `types` sudah di-import di `tctoken.go`.)

- [ ] **Step 4: Jalankan test untuk memastikan LULUS**

Run: `go test ./ -run 'TestShouldIssueTCTokenToLID|TestIsTCTokenStorableUser' -v`
Expected: PASS untuk semua sub-test.

- [ ] **Step 5: Build penuh**

Run: `go build ./...`
Expected: sukses (method `resolveTCTokenIssuanceJID` belum dipanggil — diperbolehkan).

- [ ] **Step 6: Commit**

```bash
git add tctoken.go tctoken_internal_test.go
git commit -m "feat(tctoken): add storable-user filter and migration-aware issuance JID resolver"
```

---

## Task 3: Wiring issuance JID ke issuePrivacyTokenAndSave

**Files:**
- Modify: `tctoken.go:158-180` (fungsi `issuePrivacyTokenAndSave`)

- [ ] **Step 1: Gunakan issuance JID untuk IQ, pertahankan storage JID untuk DB**

Di `tctoken.go`, dalam `issuePrivacyTokenAndSave`, ganti dua baris pertama setelah `ctx := cli.BackgroundEventCtx`:

```go
	storageJID := jid.ToNonAD()
	_, err := cli.issuePrivacyToken(ctx, storageJID, senderTimestamp)
```

menjadi:

```go
	storageJID := jid.ToNonAD()
	issuanceJID := cli.resolveTCTokenIssuanceJID(ctx, storageJID)
	_, err := cli.issuePrivacyToken(ctx, issuanceJID, senderTimestamp)
```

Sisa fungsi (yang memakai `storageJID` untuk `setTCTokenSenderTS`, `GetPrivacyToken`, `PutPrivacyTokens`) TIDAK berubah — penyimpanan tetap by LID.

- [ ] **Step 2: Build untuk verifikasi**

Run: `go build ./...`
Expected: sukses.

- [ ] **Step 3: Jalankan ulang test helper (regresi cepat)**

Run: `go test ./ -run 'TestShouldIssueTCTokenToLID|TestIsTCTokenStorableUser' -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add tctoken.go
git commit -m "feat(tctoken): issue privacy token to migration-aware JID, keep storage by LID"
```

---

## Task 4: Gate tctoken pada pesan 1:1 (send.go)

**Files:**
- Modify: `send.go:876-902` (dalam `sendDM`)

- [ ] **Step 1: Gate attach `<tctoken>`**

Di `send.go`, ganti blok:

```go
	if len(tcTokenBytes) > 0 {
		node.Content = append(node.GetChildren(), waBinary.Node{
			Tag:     "tctoken",
			Content: tcTokenBytes,
		})
	} else if csToken := cli.generateCsToken(ctx, to); len(csToken) > 0 {
		node.Content = append(node.GetChildren(), waBinary.Node{
			Tag:     "cstoken",
			Content: csToken,
		})
	}
```

menjadi:

```go
	if cli.PrivacyTokenOn1to1 && len(tcTokenBytes) > 0 {
		node.Content = append(node.GetChildren(), waBinary.Node{
			Tag:     "tctoken",
			Content: tcTokenBytes,
		})
	} else if csToken := cli.generateCsToken(ctx, to); len(csToken) > 0 {
		node.Content = append(node.GetChildren(), waBinary.Node{
			Tag:     "cstoken",
			Content: csToken,
		})
	}
```

(Saat flag off, attach `<tctoken>` dimatikan namun fallback `<cstoken>` tetap dievaluasi & jalan — sesuai keputusan desain.)

- [ ] **Step 2: Gate issuance pasca-kirim**

Di `send.go`, ganti blok:

```go
	storageJID := cli.resolveTCTokenStorageLID(ctx, to)
	if shouldSendTCTokenInChatAction(to) && shouldSendNewTCToken(cli.getTCTokenSenderTS(storageJID)) {
		go cli.issuePrivacyTokenAndSave(storageJID, time.Now())
	}
```

menjadi:

```go
	if cli.PrivacyTokenOn1to1 {
		storageJID := cli.resolveTCTokenStorageLID(ctx, to)
		if shouldSendTCTokenInChatAction(to) && shouldSendNewTCToken(cli.getTCTokenSenderTS(storageJID)) {
			go cli.issuePrivacyTokenAndSave(storageJID, time.Now())
		}
	}
```

- [ ] **Step 3: Build untuk verifikasi**

Run: `go build ./...`
Expected: sukses.

- [ ] **Step 4: Commit**

```bash
git add send.go
git commit -m "feat(tctoken): gate 1:1 tctoken attach and issuance behind PrivacyTokenOn1to1"
```

---

## Task 5: Gate tctoken pada foto profil (user.go)

**Files:**
- Modify: `user.go:581-587` (dalam pembentukan `pictureContent`)

- [ ] **Step 1: Bungkus attach dengan flag**

Di `user.go`, ganti blok:

```go
		var pictureContent []waBinary.Node
		if token, _ := cli.Store.PrivacyTokens.GetPrivacyToken(ctx, jid); token != nil {
			pictureContent = []waBinary.Node{{
				Tag:     "tctoken",
				Content: token.Token,
			}}
		}
```

menjadi:

```go
		var pictureContent []waBinary.Node
		if cli.ProfilePicPrivacyToken {
			if token, _ := cli.Store.PrivacyTokens.GetPrivacyToken(ctx, jid); token != nil {
				pictureContent = []waBinary.Node{{
					Tag:     "tctoken",
					Content: token.Token,
				}}
			}
		}
```

- [ ] **Step 2: Build untuk verifikasi**

Run: `go build ./...`
Expected: sukses.

- [ ] **Step 3: Commit**

```bash
git add user.go
git commit -m "feat(tctoken): gate profile picture tctoken attach behind ProfilePicPrivacyToken"
```

---

## Task 6: Filter storable-user di handler notifikasi (notification.go)

**Files:**
- Modify: `notification.go:299-305` (dalam `handlePrivacyTokenNotification`, setelah resolusi `senderLID`)

- [ ] **Step 1: Skip notifikasi dari non-storable user**

Di `notification.go`, setelah blok:

```go
	if senderLID.IsEmpty() {
		senderLID = cli.resolveTCTokenStorageLID(ctx, sender)
	}
	if !parentAG.OK() {
		cli.Log.Warnf("privacy_token notification didn't have a sender (%v)", parentAG.Error())
		return
	}
```

sisipkan guard berikut (sebelum `for _, child := range tokens.GetChildren() {`):

```go
	if !isTCTokenStorableUser(senderLID) {
		cli.Log.Debugf("Ignoring privacy token notification from non-storable user %s", senderLID)
		return
	}
```

- [ ] **Step 2: Build untuk verifikasi**

Run: `go build ./...`
Expected: sukses.

- [ ] **Step 3: Commit**

```bash
git add notification.go
git commit -m "feat(tctoken): skip storing privacy tokens from non-storable users"
```

---

## Task 7: Filter storable-user di history sync (message.go)

**Files:**
- Modify: `message.go:954` (dalam `storeHistoricalMessageSecrets`)

- [ ] **Step 1: Tambah predikat storable-user pada kondisi simpan**

Di `message.go`, ganti baris:

```go
		if !chatPN.IsEmpty() && conv.GetTcToken() != nil {
```

menjadi:

```go
		if !chatPN.IsEmpty() && conv.GetTcToken() != nil && isTCTokenStorableUser(chatPN) {
```

- [ ] **Step 2: Build untuk verifikasi**

Run: `go build ./...`
Expected: sukses.

- [ ] **Step 3: Commit**

```bash
git add message.go
git commit -m "feat(tctoken): filter non-storable users when storing history-sync privacy tokens"
```

---

## Task 8: Verifikasi akhir menyeluruh

**Files:** (tidak ada perubahan kode)

- [ ] **Step 1: Build seluruh modul**

Run: `go build ./...`
Expected: sukses tanpa error.

- [ ] **Step 2: Vet**

Run: `go vet ./...`
Expected: tanpa temuan baru terkait file yang diubah.

- [ ] **Step 3: Jalankan test package root**

Run: `go test ./ -v`
Expected: PASS (termasuk `TestShouldIssueTCTokenToLID`, `TestIsTCTokenStorableUser`, dan `Example` yang sudah ada tidak rusak).

- [ ] **Step 4: Tinjauan manual hal yang tak ter-unit-test**

Periksa secara manual (package root whatsmeow tidak punya mock store untuk LID, jadi resolusi store-dependent diverifikasi lewat pembacaan kode):
- `resolveTCTokenIssuanceJID`: untuk akun belum-migrasi + flag off → mengembalikan PN; akun migrasi/flag on → mengembalikan LID; fallback ke input saat mapping kosong/error.
- `send.go`: saat `PrivacyTokenOn1to1=false`, `<tctoken>` tidak di-attach tapi `<cstoken>` masih bisa; issuance pasca-kirim tidak dipanggil.
- `user.go`: saat `ProfilePicPrivacyToken=false`, tidak ada `<tctoken>` di IQ foto profil.

---

## Self-Review (diisi penulis plan)

**Spec coverage:**
- A (3 flag + default) → Task 1. ✅
- B (gate 1:1 + issuance) → Task 4. ✅
- C (gate foto profil) → Task 5. ✅
- D (issuance JID migration-aware: `shouldIssueTCTokenToLID`, `resolveTCTokenIssuanceJID`, wiring) → Task 2 + Task 3. ✅
- E (filter storable-user di notifikasi & history sync) → Task 2 (helper) + Task 6 + Task 7. ✅
- F (table-test helper murni) → Task 2 Step 1-4. ✅
- Migrasi & kompat mundur (tanpa perubahan skema/DELETE) → terjaga: tidak ada task yang menyentuh SQL/skema. ✅

**Type consistency:** Nama dipakai konsisten — `shouldIssueTCTokenToLID(bool, int64) bool`, `isTCTokenStorableUser(types.JID) bool`, `resolveTCTokenIssuanceJID(context.Context, types.JID) types.JID`, field `PrivacyTokenOn1to1`/`ProfilePicPrivacyToken`/`LIDTrustedTokenIssueToLID`. Cocok di seluruh task.

**Placeholder scan:** Tidak ada TBD/TODO; semua step memuat kode/command konkret.
