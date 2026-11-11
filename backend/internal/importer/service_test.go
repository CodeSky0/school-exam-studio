package importer_test

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"testing"

	entquestion "school-exam-studio/backend/ent/question"
	"school-exam-studio/backend/internal/auth"
	"school-exam-studio/backend/internal/importer"
	"school-exam-studio/backend/internal/platform/platformtest"
	"school-exam-studio/backend/internal/question"
)

func TestImportOneHundredRowsAndRejectLastInvalidRowAtomically(t *testing.T) {
	client := platformtest.Open(t)
	platformtest.EnsureFixtures(t, client)
	service := importer.NewService(importer.NewRepository(client), question.NewSanitizer())
	ctx := context.Background()
	validCSV := hundredRowCSV(t, 100, 0)

	result, err := service.ImportCSV(ctx, platformtest.TeacherPrincipal(client), bytes.NewReader(validCSV))
	if err != nil || result.ValidCount != 100 || len(result.Errors) != 0 || len(result.Preview) != 50 {
		t.Fatalf("100-row import failed: %+v %v", result, err)
	}
	if got := platformtest.QuestionCount(t, client, platformtest.SchoolA); got != 100 {
		t.Fatalf("imported question count=%d, want 100", got)
	}
	entities, err := client.Question.Query().Where(entquestion.SchoolIDEQ(platformtest.SchoolA)).All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, entity := range entities {
		if entity.CreatorID != platformtest.TeacherA || entity.Status != question.StatusDraft || entity.Subject != "数学" {
			t.Fatalf("import ignored authenticated school/creator/draft constraints: %+v", entity)
		}
	}

	invalidCSV := hundredRowCSV(t, 100, 6)
	invalidResult, err := service.ImportCSV(ctx, platformtest.TeacherBPrincipal(), bytes.NewReader(invalidCSV))
	if err != nil {
		t.Fatal(err)
	}
	if invalidResult.ValidCount != 99 || len(invalidResult.Preview) != 50 || len(invalidResult.Errors) != 1 ||
		invalidResult.Errors[0].Line != 101 || invalidResult.Errors[0].Field != "difficulty" {
		t.Fatalf("last-row validation is incorrect: %+v", invalidResult)
	}
	if got := platformtest.QuestionCount(t, client, platformtest.SchoolB); got != 0 {
		t.Fatalf("invalid import partially wrote %d rows", got)
	}

	validationResult, err := service.Validate(ctx, platformtest.TeacherPrincipal(client), bytes.NewReader(invalidCSV))
	if err != nil || validationResult.Errors[0].Line != 101 {
		t.Fatalf("validate changed its line reporting: %+v %v", validationResult, err)
	}
	if got := platformtest.QuestionCount(t, client, platformtest.SchoolA); got != 100 {
		t.Fatalf("validate wrote database rows: %d", got)
	}

	otherTeacher := auth.Principal{UserID: platformtest.OtherTeacherA, SchoolID: platformtest.SchoolA, Role: auth.RoleTeacher, SessionVersion: 1}
	mismatch, err := service.Validate(ctx, otherTeacher, bytes.NewReader(hundredRowCSV(t, 1, 0)))
	if err != nil || len(mismatch.Errors) != 1 || mismatch.Errors[0].Field != "subject" || mismatch.ValidCount != 0 {
		t.Fatalf("teacher subject was not enforced: %+v %v", mismatch, err)
	}
	adminResult, err := service.ImportCSV(ctx, platformtest.AdminPrincipal(client), bytes.NewReader(csvWithSubject(t, "语文")))
	if err != nil || adminResult.ValidCount != 1 {
		t.Fatalf("admin should not be restricted to a teacher subject: %+v %v", adminResult, err)
	}

	rows := []importer.ImportRow{validDirectImportRow(2), validDirectImportRow(3)}
	rows[1].Difficulty = 6
	count, err := service.Import(ctx, platformtest.TeacherPrincipal(client), rows)
	var validation *importer.RowValidationError
	if count != 0 || !errors.As(err, &validation) || len(validation.RowErrors) != 1 || validation.RowErrors[0].Line != 3 {
		t.Fatalf("direct import was not fully revalidated: count=%d err=%v", count, err)
	}
}

func hundredRowCSV(t *testing.T, count, lastDifficulty int) []byte {
	t.Helper()
	records := make([][]string, count)
	for index := range records {
		records[index] = validCSVRecord("单选题")
		records[index][5] = "第" + strconv.Itoa(index+1) + "题：$x+1=2$"
		if index == count-1 && lastDifficulty != 0 {
			records[index][3] = strconv.Itoa(lastDifficulty)
		}
	}
	return csvBytes(t, records, count == 100)
}

func csvWithSubject(t *testing.T, subject string) []byte {
	t.Helper()
	record := validCSVRecord("单选题")
	record[2] = subject
	return csvBytes(t, [][]string{record}, false)
}

func validDirectImportRow(line int) importer.ImportRow {
	return importer.ImportRow{
		Line: line, Type: question.TypeSingleChoice, Grade: "七年级", Subject: "数学", Difficulty: 1,
		Tags: []string{"代数"}, Content: "<p>1+1=?</p>",
		Options: []question.Option{{Key: "A", Content: "1"}, {Key: "B", Content: "2"}},
		Answer: "B", Explanation: "<p>基础加法</p>",
	}
}
