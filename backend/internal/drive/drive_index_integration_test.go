package drive

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/embedding"
	"school-exam-studio/backend/internal/platform/platformtest"
)

func TestCloudIndexerIndexesUploadedTextAndSearches(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(client)
	service := NewService(repository, store)
	indexer := NewIndexer(repository, store, embedding.NewLocal(256), nil)
	service.SetIndexer(indexer)
	ctx := context.Background()
	teacher := platformtest.TeacherPrincipal(client)

	content := []byte("一元一次方程的解法：移项、合并同类项、系数化为1。\n例题：解方程 2x+3=7。\n答案：x=2。")
	file, err := service.Upload(ctx, teacher, "", "一元一次方程教案.txt", content)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	itemID := uuid.MustParse(file.ID)
	outcome, err := indexer.Process(ctx, teacher.SchoolID, itemID)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if outcome.Status != IndexReady || outcome.ChunkCount == 0 {
		t.Fatalf("unexpected index outcome: %#v", outcome)
	}
	item, err := repository.Get(ctx, teacher.SchoolID, itemID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if item.IndexStatus != IndexReady || item.ChunkCount != outcome.ChunkCount || item.IndexProvider != embedding.LocalName {
		t.Fatalf("unexpected item index fields: %#v", item)
	}

	result, err := service.Search(ctx, teacher, "一元一次方程怎么解", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !result.IndexAvailable || len(result.Hits) == 0 {
		t.Fatalf("expected search hits, got %#v", result)
	}
	if result.Hits[0].Name != "一元一次方程教案.txt" {
		t.Fatalf("unexpected first hit: %#v", result.Hits[0])
	}

	if _, err := service.Delete(ctx, teacher, itemID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	matches, err := repository.ListSearchableChunks(ctx, teacher.SchoolID, embedding.LocalName, searchChunkLimit)
	if err != nil {
		t.Fatalf("ListSearchableChunks: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected chunks to be removed with the file, got %d", len(matches))
	}
}

func TestCloudIndexerMarksUnsupportedFormat(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(client)
	service := NewService(repository, store)
	indexer := NewIndexer(repository, store, embedding.NewLocal(256), nil)
	ctx := context.Background()
	teacher := platformtest.TeacherPrincipal(client)

	file, err := service.Upload(ctx, teacher, "", "课件.pptx", []byte("not-a-real-pptx"))
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	outcome, err := indexer.Process(ctx, teacher.SchoolID, uuid.MustParse(file.ID))
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if outcome.Status != IndexUnsupported || outcome.Message == "" {
		t.Fatalf("expected unsupported outcome, got %#v", outcome)
	}
}