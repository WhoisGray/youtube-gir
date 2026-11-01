package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.EnsureAdmin(context.Background(), "admin-token-0123456789"); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestUsersAndDownloads(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()

	admin, err := st.UserByToken(ctx, "admin-token-0123456789")
	if err != nil || admin.Role != RoleAdmin || admin.Username != "admin" {
		t.Fatalf("admin: %+v %v", admin, err)
	}
	if err := st.EnsureAdmin(ctx, "rotated-admin-token-012345"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserByToken(ctx, "admin-token-0123456789"); !errors.Is(err, ErrNotFound) {
		t.Fatal("old admin token should stop working")
	}

	alice, err := st.CreateUser(ctx, "alice", "token-alice-0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(ctx, "Alice", "token-other-0123456789abcdef"); err == nil {
		t.Fatal("usernames are case-insensitive")
	}

	d := &Download{UserID: alice.ID, URL: "https://www.youtube.com/watch?v=abcdefghijk", VideoID: "abcdefghijk", Title: "Hello", Kind: "video", Quality: "1080"}
	if err := st.CreateDownload(ctx, d); err != nil {
		t.Fatal(err)
	}
	if len(d.ID) != 32 || len(d.FileToken) != 64 {
		t.Fatalf("ids: %s %s", d.ID, d.FileToken)
	}

	active, err := st.FindActive(ctx, alice.ID, d.URL, "video", "1080")
	if err != nil || active.ID != d.ID {
		t.Fatalf("active: %+v %v", active, err)
	}

	claimed, err := st.ClaimNext(ctx)
	if err != nil || claimed.ID != d.ID || claimed.Status != StatusDownloading || claimed.StartedAt == nil {
		t.Fatalf("claim: %+v %v", claimed, err)
	}
	again, err := st.ClaimNext(ctx)
	if err != nil || again != nil {
		t.Fatalf("second claim: %+v %v", again, err)
	}
	if err := st.UpdateMeta(ctx, d.ID, "Real title", "abcdefghijk"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateProgress(ctx, d.ID, ProgressUpdate{Percent: 40, BytesDownloaded: 40, TotalBytes: 100, SpeedBPS: 10, ETA: "00:03"}); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkCompleted(ctx, d.ID, "/data/downloads/1/file.mp4", "Real title.mp4", 100); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetByFileToken(ctx, d.FileToken)
	if err != nil || got.Status != StatusCompleted || got.Title != "Real title" || got.FileSize != 100 || got.Progress != 100 {
		t.Fatalf("completed: %+v %v", got, err)
	}
	stats, err := st.Stats(ctx, alice.ID, false)
	if err != nil || stats.DownloadsCompleted != 1 || stats.BytesDownloaded != 100 {
		t.Fatalf("stats: %+v %v", stats, err)
	}
	all, err := st.Stats(ctx, 0, true)
	if err != nil || all.Users != 1 || all.DownloadsTotal != 1 {
		t.Fatalf("all stats: %+v %v", all, err)
	}

	mine, err := st.ListDownloads(ctx, alice.ID, false, 10)
	if err != nil || len(mine) != 1 {
		t.Fatal(mine, err)
	}
	adminList, err := st.ListDownloads(ctx, admin.ID, false, 10)
	if err != nil || len(adminList) != 0 {
		t.Fatalf("admin personal list: %+v", adminList)
	}
}

func TestDownloadProxySetting(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()
	value, ok, err := st.GetSetting(ctx, SettingDownloadProxy)
	if err != nil || ok || value != "" {
		t.Fatalf("missing setting: %q %v %v", value, ok, err)
	}
	if err := st.SetSetting(ctx, SettingDownloadProxy, "socks5://127.0.0.1:1080"); err != nil {
		t.Fatal(err)
	}
	value, ok, err = st.GetSetting(ctx, SettingDownloadProxy)
	if err != nil || !ok || value != "socks5://127.0.0.1:1080" {
		t.Fatalf("saved: %q %v %v", value, ok, err)
	}
	if err := st.SetSetting(ctx, SettingDownloadProxy, ""); err != nil {
		t.Fatal(err)
	}
	value, ok, err = st.GetSetting(ctx, SettingDownloadProxy)
	if err != nil || !ok || value != "" {
		t.Fatalf("cleared: %q %v %v", value, ok, err)
	}
}

func TestRecoverAndPause(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()
	user, err := st.CreateUser(ctx, "bob", "token-bob-0123456789abcdef12")
	if err != nil {
		t.Fatal(err)
	}
	d := &Download{UserID: user.ID, URL: "https://www.youtube.com/watch?v=abcdefghijk", VideoID: "abcdefghijk", Title: "T", Kind: "audio", Quality: "best"}
	if err := st.CreateDownload(ctx, d); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNext(ctx); err != nil {
		t.Fatal(err)
	}
	n, err := st.RecoverInterrupted(ctx)
	if err != nil || n != 1 {
		t.Fatalf("recover %d %v", n, err)
	}
	got, err := st.GetDownload(ctx, d.ID)
	if err != nil || got.Status != StatusQueued {
		t.Fatalf("%+v", got)
	}
	if err := st.SetStatus(ctx, d.ID, StatusPaused, "", []string{StatusQueued}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNext(ctx); err != nil {
		t.Fatal(err)
	}
	empty, err := st.ClaimNext(ctx)
	if err != nil || empty != nil {
		t.Fatalf("paused item was claimed: %+v", empty)
	}
	if err := st.SetStatus(ctx, d.ID, StatusQueued, "", []string{StatusPaused}); err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimNext(ctx)
	if err != nil || claimed == nil || claimed.ID != d.ID {
		t.Fatalf("resume claim: %+v %v", claimed, err)
	}
}
