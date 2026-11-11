import { describe, expect, it } from "vitest";
import { formatTags, parseTags, toQuestionInput } from "./question-form";

describe("题目标签输入", () => {
  it("支持中英文逗号、分号并去重", () => {
    expect(parseTags(" 方程；函数,几何，方程; 代数 ")).toEqual(["方程", "函数", "几何", "代数"]);
  });

  it("将数组转换为便于编辑的输入文本", () => {
    expect(formatTags(["方程", "函数"])).toBe("方程；函数");
  });
});

describe("题目 API 输入序列化", () => {
  it("多选答案使用逗号分隔并消毒富文本答案", () => {
    const multiple = toQuestionInput({
      subject: "数学",
      grade: "八年级",
      question_type: "multiple_choice",
      difficulty: 2,
      tagsText: "方程",
      content: "<p>题干</p>",
      options: [{ key: "A", content: "2" }, { key: "B", content: "4" }],
      answer: "BA",
      explanation: "",
    });
    expect(multiple.answer).toBe("A,B");
    const essay = toQuestionInput({
      subject: "数学",
      grade: "八年级",
      question_type: "essay",
      difficulty: 2,
      tagsText: "",
      content: "<p>题干</p>",
      options: [],
      answer: '<p>正确<script>alert(1)</script>答案</p>',
      explanation: "",
    });
    expect(essay.answer).toBe("<p>正确答案</p>");
  });
});
