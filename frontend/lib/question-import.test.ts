import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./api";
import { downloadQuestionImportTemplate, importQuestions, importValidationResultSchema, validateCsvFile, validateQuestionImport } from "./question-import";

const previewRow = {
  line: 2,
  subject: "数学",
  grade: "七年级",
  question_type: "single_choice",
  difficulty: 1,
  tags: ["有理数"],
  content: "<p>1+1 等于多少？</p>",
  options: [{ key: "A", content: "1" }, { key: "B", content: "2" }],
  answer: "B",
  explanation: "<p>基础加法</p>",
};

afterEach(() => vi.unstubAllGlobals());

describe("CSV 文件与响应校验", () => {
  it("检查扩展名、空文件与大小", () => {
    expect(validateCsvFile(new File(["a"], "questions.txt", { type: "text/plain" }))).toBe("请选择 CSV 格式文件");
    expect(validateCsvFile(new File([], "questions.csv", { type: "text/csv" }))).toBe("CSV 文件内容为空，请重新选择");
    expect(validateCsvFile(new File(["a,b"], "questions.csv", { type: "text/csv" }))).toBeNull();
  });

  it("校验逐行错误并兼容后端的 type 字段", () => {
    const { question_type, ...rowWithoutQuestionType } = previewRow;
    const result = importValidationResultSchema.parse({
      valid_count: 1,
      preview: [{ ...rowWithoutQuestionType, type: question_type }],
      errors: [{ line: 3, field: "难度", message: "难度须为 1 至 5" }],
    });
    expect(result.preview[0].question_type).toBe("single_choice");
    expect(result.errors[0].line).toBe(3);
  });
});

describe("CSV API", () => {
  it("校验与导入分别重新上传同一个文件", async () => {
    const file = new File(["header\nrow"], "questions.csv", { type: "text/csv" });
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ valid_count: 1, preview: [previewRow], errors: [] }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ imported: 1 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    await validateQuestionImport(file);
    await expect(importQuestions(file)).resolves.toEqual({ imported: 1 });

    const validateBody = fetchMock.mock.calls[0][1].body as FormData;
    const importBody = fetchMock.mock.calls[1][1].body as FormData;
    expect(validateBody).not.toBe(importBody);
    expect(validateBody.get("file")).toBe(file);
    expect(importBody.get("file")).toBe(file);
    expect(fetchMock.mock.calls[0][1].headers.get("Content-Type")).toBeNull();
  });

  it("保留 422 响应中的预览和逐行错误", async () => {
    const file = new File(["invalid"], "questions.csv", { type: "text/csv" });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({
      valid_count: 0,
      preview: [],
      errors: [{ line: 2, field: "学科", message: "学科须与教师账号一致" }],
    }), { status: 422 })));

    const error = await importQuestions(file).catch((value: unknown) => value);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 422, details: { errors: [{ line: 2, field: "学科" }] } });
  });

  it("模板下载携带会话 Cookie", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("题型,年级", { status: 200, headers: { "Content-Type": "text/csv" } }));
    vi.stubGlobal("fetch", fetchMock);
    await expect((await downloadQuestionImportTemplate()).text()).resolves.toBe("题型,年级");
    expect(fetchMock.mock.calls[0][0]).toBe("http://localhost:8080/api/v1/questions/import/template");
    expect(fetchMock.mock.calls[0][1]).toMatchObject({ credentials: "include" });
  });
});
