package drive

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/platform/platformtest"
)

func TestCloudDriveFolderUploadDownloadDelete(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(NewRepository(client), store)
	ctx := context.Background()
	teacher := platformtest.TeacherPrincipal(client)

	folder, err := service.CreateFolder(ctx, teacher, CreateFolderInput{Name: "高一资料"})
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	if _, err := service.CreateFolder(ctx, teacher, CreateFolderInput{Name: "高一资料"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}

	pdf := []byte("%PDF-1.7\ncloud drive test\n")
	file, err := service.Upload(ctx, teacher, folder.ID, "期中试卷.pdf", pdf)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if file.Kind != "pdf" || file.SizeBytes != int64(len(pdf)) {
		t.Fatalf("file = %#v", file)
	}

	rootList, err := service.List(ctx, teacher, "")
	if err != nil || len(rootList.Items) != 1 || !rootList.Items[0].IsFolder {
		t.Fatalf("root list = %#v, err = %v", rootList, err)
	}
	folderList, err := service.List(ctx, teacher, folder.ID)
	if err != nil {
		t.Fatalf("List folder: %v", err)
	}
	if len(folderList.Items) != 1 || len(folderList.Breadcrumbs) != 1 || folderList.Breadcrumbs[0].Name != "高一资料" {
		t.Fatalf("folder list = %#v", folderList)
	}

	item, reader, err := service.OpenFile(ctx, teacher, uuid.MustParse(file.ID))
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	content, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil || string(content) != string(pdf) || item.Name != "期中试卷.pdf" {
		t.Fatalf("download mismatch: %v", err)
	}

	otherSchool := auth.Principal{
		UserID: platformtest.TeacherB, SchoolID: platformtest.SchoolB,
		Role: auth.RoleTeacher, Subject: "数学", SessionVersion: 1,
	}
	if _, _, err := service.OpenFile(ctx, otherSchool, uuid.MustParse(file.ID)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-school access should be hidden, got %v", err)
	}

	deleted, err := service.Delete(ctx, teacher, uuid.MustParse(folder.ID))
	if err != nil || deleted != 2 {
		t.Fatalf("Delete = %d, err = %v", deleted, err)
	}
	list, err := service.List(ctx, teacher, "")
	if err != nil || len(list.Items) != 0 {
		t.Fatalf("list after delete = %#v, err = %v", list, err)
	}
	if _, _, err := service.OpenFile(ctx, teacher, uuid.MustParse(file.ID)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted file should be hidden, got %v", err)
	}
}

func TestCloudDriveImportsAIAttachmentWithDedup(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(NewRepository(client), store)
	ctx := context.Background()
	teacher := platformtest.TeacherPrincipal(client)
	content := []byte("1. 云盘去重测试\n答案：A\n")

	if err := service.ImportFromAI(ctx, teacher, "课堂练习.txt", content); err != nil {
		t.Fatalf("ImportFromAI: %v", err)
	}
	if err := service.ImportFromAI(ctx, teacher, "课堂练习-副本.txt", content); err != nil {
		t.Fatalf("ImportFromAI duplicate: %v", err)
	}
	list, err := service.List(ctx, teacher, "")
	if err != nil || len(list.Items) != 1 {
		t.Fatalf("expected one deduplicated item, got %#v, err = %v", list.Items, err)
	}
	if !strings.HasSuffix(list.Items[0].Name, ".txt") {
		t.Fatalf("unexpected name %q", list.Items[0].Name)
	}
}

func TestCloudDriveBatchDelete(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(NewRepository(client), store)
	ctx := context.Background()
	teacher := platformtest.TeacherPrincipal(client)

	folder, err := service.CreateFolder(ctx, teacher, CreateFolderInput{Name: "批量删除文件夹"})
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	nested, err := service.Upload(ctx, teacher, folder.ID, "嵌套文件.pdf", []byte("%PDF-1.7\nbatch delete test\n"))
	if err != nil {
		t.Fatalf("Upload nested: %v", err)
	}
	fileA, err := service.Upload(ctx, teacher, "", "试卷A.pdf", []byte("%PDF-1.7\nfile a\n"))
	if err != nil {
		t.Fatalf("Upload A: %v", err)
	}
	fileB, err := service.Upload(ctx, teacher, "", "试卷B.pdf", []byte("%PDF-1.7\nfile b\n"))
	if err != nil {
		t.Fatalf("Upload B: %v", err)
	}

	if _, err := service.DeleteMany(ctx, teacher, nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("DeleteMany empty should fail validation, got %v", err)
	}
	if _, err := service.DeleteMany(ctx, teacher, make([]uuid.UUID, maxBatchDeleteItems+1)); !errors.Is(err, ErrValidation) {
		t.Fatalf("DeleteMany over limit should fail validation, got %v", err)
	}

	missing := uuid.New()
	deleted, err := service.DeleteMany(ctx, teacher, []uuid.UUID{
		uuid.MustParse(fileA.ID),
		uuid.MustParse(folder.ID),
		missing,
		missing,
	})
	if err != nil {
		t.Fatalf("DeleteMany: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("DeleteMany = %d, want 3 (fileA + folder + nested)", deleted)
	}

	list, err := service.List(ctx, teacher, "")
	if err != nil || len(list.Items) != 1 || list.Items[0].Name != "试卷B.pdf" {
		t.Fatalf("list after batch delete = %#v, err = %v", list, err)
	}
	if _, _, err := service.OpenFile(ctx, teacher, uuid.MustParse(fileA.ID)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted file A should be hidden, got %v", err)
	}
	if _, _, err := service.OpenFile(ctx, teacher, uuid.MustParse(nested.ID)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted nested file should be hidden, got %v", err)
	}

	otherSchool := auth.Principal{
		UserID: platformtest.TeacherB, SchoolID: platformtest.SchoolB,
		Role: auth.RoleTeacher, Subject: "数学", SessionVersion: 1,
	}
	if deleted, err := service.DeleteMany(ctx, otherSchool, []uuid.UUID{uuid.MustParse(fileB.ID)}); err != nil || deleted != 0 {
		t.Fatalf("cross-school DeleteMany = %d, err = %v", deleted, err)
	}
}
