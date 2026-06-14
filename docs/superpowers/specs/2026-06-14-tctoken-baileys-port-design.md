# Spec: Port Perilaku tcToken Baileys → whatsmeow

- **Tanggal:** 2026-06-14
- **Status:** Disetujui (siap masuk tahap rencana implementasi)
- **Branch:** fork-13-06-2026

## Latar Belakang

whatsmeow sudah memiliki implementasi tcToken (privacy token) yang cukup lengkap:
penyimpanan di tabel SQL `whatsmeow_privacy_tokens`, bucket window 7 hari × 4
(~28 hari), cache sender-timestamp in-memory, fallback `cstoken` (HMAC-SHA256 + NCT
salt), serta issuance & re-issuance saat identity change.

Dibanding baileys, ada beberapa perilaku yang belum ada di whatsmeow. Spec ini
memilih sebagian perbedaan tersebut untuk di-port, **tanpa** mengubah arsitektur
inti whatsmeow yang sudah lebih unggul (SQL vs sentinel `__index` baileys).

## Tujuan (Scope)

1. **Tiga config flag** di `Client` untuk meng-gate perilaku tcToken (padanan AB props baileys),
   dikontrol manual oleh pengguna library (whatsmeow tidak punya fetch server-props).
2. **Gate** attach `<tctoken>` pada pesan 1:1 dan IQ foto profil berdasarkan flag.
3. **Pemisahan issuance JID vs storage JID** dengan flag (issue ke LID vs PN).
4. **Filter "regular user"** saat menyimpan token (notifikasi & history sync).

## Non-Tujuan (Out of Scope)

- Membangun infrastruktur fetch server-props/AB props dari server (whatsmeow tidak punya;
  effort besar, subsistem terpisah). Flag dikontrol manual.
- Penanganan error 463 (MessageAccountRestriction) / 479 (SmaxInvalid). Sengaja dilewati.
- Mengadopsi sentinel `__index` baileys (SQL whatsmeow sudah lebih baik).
- Mengubah/menghapus `cstoken` / NCT salt (fitur khusus whatsmeow, dipertahankan).
- Mengubah cache sender-timestamp in-memory (dipertahankan).

## Desain Rinci

### A. Tiga config flag di `Client` (`client.go`)

Tambah field datar mengikuti pola `ErrorOnSubscribePresenceWithoutToken`:

```go
// PrivacyTokenOn1to1 controls whether a <tctoken> is attached to 1:1 messages.
// Mirrors baileys AB prop 10518. Default: true.
PrivacyTokenOn1to1 bool
// ProfilePicPrivacyToken controls whether a <tctoken> is attached to profile picture IQs.
// Mirrors baileys AB prop 9666. Default: true.
ProfilePicPrivacyToken bool
// LIDTrustedTokenIssueToLID controls whether privacy tokens are issued to the LID (true)
// or the phone number / PN (false). Mirrors baileys AB prop 14303. Default: false.
LIDTrustedTokenIssueToLID bool
```

Karena dua flag default `true`, nilai diinisialisasi di `NewClient` (bukan zero-value Go).
`LIDTrustedTokenIssueToLID` default `false` (zero-value, tetap di-set eksplisit untuk kejelasan).

### B. Gate attach `<tctoken>` pada pesan 1:1 (`send.go` sekitar baris 876–902)

Struktur baru:

```go
tcTokenBytes, tcErr := cli.ensureTCToken(ctx, to)
if tcErr != nil {
    cli.Log.Warnf("Failed to get privacy token for %s: %v", to, tcErr)
}
if cli.PrivacyTokenOn1to1 && len(tcTokenBytes) > 0 {
    node.Content = append(node.GetChildren(), waBinary.Node{Tag: "tctoken", Content: tcTokenBytes})
} else if csToken := cli.generateCsToken(ctx, to); len(csToken) > 0 {
    node.Content = append(node.GetChildren(), waBinary.Node{Tag: "cstoken", Content: csToken})
}
```

- Saat `PrivacyTokenOn1to1 == false`, attach `<tctoken>` dimatikan, namun fallback
  `<cstoken>` **tetap jalan** (independen — keputusan disepakati).
- Issuance pasca-kirim (`send.go:899-902`) juga di-gate `PrivacyTokenOn1to1` agar konsisten
  dengan baileys (baileys hanya issue saat `privacyTokenOn1to1` aktif):

```go
if cli.PrivacyTokenOn1to1 {
    storageJID := cli.resolveTCTokenStorageLID(ctx, to)
    if shouldSendTCTokenInChatAction(to) && shouldSendNewTCToken(cli.getTCTokenSenderTS(storageJID)) {
        go cli.issuePrivacyTokenAndSave(storageJID, time.Now())
    }
}
```

### C. Gate attach `<tctoken>` pada foto profil (`user.go` sekitar baris 581–587)

```go
var pictureContent []waBinary.Node
if cli.ProfilePicPrivacyToken {
    if token, _ := cli.Store.PrivacyTokens.GetPrivacyToken(ctx, jid); token != nil {
        pictureContent = []waBinary.Node{{Tag: "tctoken", Content: token.Token}}
    }
}
```

- Branch community (`w:g2`) tidak terpengaruh.
- Group JID natural aman: `GetPrivacyToken` mengembalikan `nil` → tidak ada token di-attach.

### D. Pisahkan storage JID vs issuance JID (flag 14303)

Tambah helper di `tctoken.go`:

```go
// resolveTCTokenIssuanceJID returns the JID to put in the <token jid="..."> attribute
// when issuing a privacy token. Issuance prefers the LID when either the
// LIDTrustedTokenIssueToLID flag is set OR the account is already LID-migrated
// (Store.LIDMigrationTimestamp > 0); otherwise it prefers the PN. Falls back to the
// input JID if the required mapping is unavailable.
func (cli *Client) resolveTCTokenIssuanceJID(ctx context.Context, jid types.JID) types.JID
```

**Keputusan "issue ke LID" bersifat migration-aware:**

```go
issueToLID := cli.LIDTrustedTokenIssueToLID || cli.Store.LIDMigrationTimestamp > 0
```

Alasan: saat `Store.LIDMigrationTimestamp > 0`, whatsmeow memaksa tujuan pesan PN→LID
(`send.go:331`), sehingga seluruh stanza dikirim sebagai LID. Jika token tetap di-issue ke
PN, terjadi inkonsistensi (kontak diperlakukan LID di mana-mana kecuali issuance). Dengan
menjadikan keputusan migration-aware, issuance mengikuti addressing pesan yang sebenarnya.
baileys tidak punya konsep forced-LID-migration per-akun, jadi default statisnya (`false` → PN)
tidak cukup untuk whatsmeow.

Aturan resolusi:
- `issueToLID == true`: jika input PN → resolve ke LID (`Store.LIDs.GetLIDForPN`);
  jika sudah LID → pakai apa adanya. Fallback ke input jika mapping kosong.
- `issueToLID == false`: jika input LID → resolve ke PN
  (`Store.LIDs.GetPNForLID`); jika sudah PN → pakai apa adanya. Fallback ke input jika mapping kosong.

Matriks hasil:
| Kondisi | Hasil issuance JID |
|---|---|
| Belum migrasi, flag `false` (default) | **PN** (parity baileys) |
| Sudah migrasi (`LIDMigrationTimestamp > 0`) | **LID** (konsisten dgn tujuan pesan) |
| Flag `true` (override eksplisit) | **LID** |

Perubahan `issuePrivacyToken` / `issuePrivacyTokenAndSave`:
- **Storage & lookup** tetap memakai LID (`resolveTCTokenStorageLID`) — tidak berubah.
- **Atribut `<token jid=...>`** pada IQ memakai hasil `resolveTCTokenIssuanceJID`.
- Caranya: `issuePrivacyToken` menerima `issuanceJID` terpisah dari `storageJID`, atau
  `issuePrivacyTokenAndSave` menghitung `issuanceJID` lalu meneruskannya ke `issuePrivacyToken`.

**Perubahan perilaku default (disengaja):** saat ini whatsmeow selalu issue ke LID
(`storageJID` = LID). Setelah perubahan: akun yang **belum** migrasi akan issue ke **PN**
(sesuai default baileys), sedangkan akun yang **sudah** migrasi tetap issue ke **LID**.

### E. Filter "regular user" saat menyimpan token

Padanan `isRegularUser` baileys sudah tersedia di whatsmeow sebagai
`shouldSendTCTokenInChatAction` (cek `(DefaultUserServer || HiddenUserServer) && user != PSA && !IsBot()`).
Catatan: `botUserRegex` whatsmeow (`^1313555\d{4}$|^131655500\d{2}$`) **identik** dengan
`BOT_PHONE_REGEX` baileys dan sudah mencakup MetaAI `13135550002`.

Pakai ulang sebagai guard sebelum menyimpan (opsional dibungkus alias `isTCTokenStorableUser(jid)`
demi kejelasan maksud):

- `notification.go` `handlePrivacyTokenNotification`: skip simpan jika `senderLID` bukan storable user.
- `message.go` `storeHistoricalMessageSecrets`: filter tiap kandidat token sebelum di-append ke
  slice `privacyTokens`.

### F. Testing

Table-test untuk logika murni / mudah di-mock:
- `isTCTokenStorableUser` / `shouldSendTCTokenInChatAction`: kasus PSA (`0`), bot (`13135550xxx`),
  MetaAI, user normal PN, user LID, server lain.
- `resolveTCTokenIssuanceJID`: matriks (flag on/off) × (`LIDMigrationTimestamp` >0 / =0) ×
  (ada/tidak ada mapping LID↔PN), dengan mock `Store.LIDs`. Pastikan: belum-migrasi+flag-off → PN;
  sudah-migrasi → LID; flag-on → LID.
- Bila perlu, ekstrak keputusan gate ke fungsi kecil agar bisa diuji tanpa koneksi nyata.

## Ringkasan File yang Disentuh

| File | Perubahan |
|---|---|
| `client.go` | 3 field flag + default di `NewClient` |
| `send.go` | Gate attach `<tctoken>` + gate issuance pasca-kirim |
| `user.go` | Gate attach `<tctoken>` pada foto profil |
| `tctoken.go` | Helper `resolveTCTokenIssuanceJID`; ubah `issuePrivacyToken`/`issuePrivacyTokenAndSave` |
| `notification.go` | Guard storable-user di `handlePrivacyTokenNotification` |
| `message.go` | Guard storable-user di `storeHistoricalMessageSecrets` |
| `*_test.go` | Table-test untuk helper murni |

## Risiko & Catatan

- **Perubahan default issuance** bersifat migration-aware: akun **belum** migrasi + flag `false`
  kini issue ke **PN** (selaras baileys); akun **sudah** migrasi (`LIDMigrationTimestamp > 0`) tetap
  issue ke **LID** (konsisten dengan tujuan pesan yang dipaksa LID di `send.go:331`). Pengguna yang
  ingin selalu LID dapat set `LIDTrustedTokenIssueToLID=true`.
- Gate default `true` untuk `PrivacyTokenOn1to1` & `ProfilePicPrivacyToken` menjaga perilaku
  attach tetap seperti sekarang; hanya menambah kemampuan mematikannya.
- Tidak ada migrasi DB (skema tidak berubah).

## Migrasi & Kompatibilitas Mundur

Sasaran: aman diterapkan pada session yang **sudah jalan** dengan tabel
`whatsmeow_privacy_tokens` yang **sudah terisi**. Tidak ada migrasi DB, rewrite, maupun delete.

| Perubahan | Dampak ke data existing | Aman? |
|---|---|---|
| 3 config flag di `Client` | Murni in-memory; default di `NewClient`. Tidak ada kolom/tabel baru | ✅ |
| Gate `<tctoken>` 1:1 (default `true`) | Tidak ada — perilaku attach sama | ✅ |
| Gate foto profil (default `true`) | Tidak ada — perilaku attach sama | ✅ |
| `resolveTCTokenIssuanceJID` migration-aware | Keying storage tetap by LID; hanya ubah attr `<token jid>` di IQ runtime | ✅ |
| Filter storable-user saat simpan | Hanya menyaring **write baru**; baris lama tidak disentuh/dihapus | ✅ |

**Invariannya:**

1. **Tidak ada file upgrade SQL baru / `ALTER TABLE` / bump versi skema.** Tabel
   `whatsmeow_privacy_tokens` (termasuk kolom `sender_timestamp` dari upgrade V12) tetap apa adanya.
2. **Keying penyimpanan tidak berubah.** Baris lama — tersimpan by LID maupun by PN (untuk yang dulu
   belum punya mapping) — tetap ketemu, karena `GetPrivacyToken` punya `CASE` cross-lookup PN↔LID via
   `whatsmeow_lid_map`. Data lama tetap terbaca tanpa perubahan.
3. **Filter hanya gate write, bukan read/delete.** Baris "tidak valid" yang terlanjur tersimpan tetap
   dibaca & di-attach seperti sebelumnya — tanpa penghapusan retroaktif.
4. **Default flag (`true/true/false`) menjaga perilaku lama** untuk attach; hanya menambah opsi setel.

**Satu perubahan perilaku (bukan bahaya data):** untuk akun yang **belum** migrasi LID, issuance
default bergeser **LID → PN** di tengah session berjalan. Disengaja (selaras baileys), tidak merusak
data (paling banter terbit token tambahan ke PN; token yang sudah tersimpan tak terpengaruh karena
jalur berbeda). Untuk rollout dengan **nol** perubahan perilaku, set `LIDTrustedTokenIssueToLID=true`
agar tetap "selalu issue ke LID" seperti sekarang.
