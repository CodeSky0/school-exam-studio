import type { CurrentUser } from "./schemas";
import { normalizeChoiceAnswer, type Question, type QuestionInput, type QuestionOption, type QuestionType } from "./question-schemas";
import { sanitizeRichText } from "./rich-text";

export type EditableQuestionField = "content" | "answer" | "explanation";

export interface QuestionFormState {
  subject: string;
  grade: string;
  question_type: QuestionType;
  difficulty: number;
  tagsText: string;
  category?: string;
  knowledgePointsText?: string;
  content: string;
  options: QuestionOption[];
  answer: string;
  explanation: string;
}

export function parseTags(value: string): string[] {
  return Array.from(new Set(value.split(/[;,；，]/).map((tag) => tag.trim()).filter(Boolean)));
}

export function formatTags(tags: string[]): string {
  return tags.join("；");
}

export function isChoiceType(type: QuestionType): boolean {
  return type === "single_choice" || type === "multiple_choice";
}

export function createChoiceOptions(): QuestionOption[] {
  return [{ key: "A", content: "" }, { key: "B", content: "" }];
}

export function createQuestionFormState(user: CurrentUser, question?: Question): QuestionFormState {
  if (question) {
    return {
      subject: question.subject,
      grade: question.grade,
      question_type: question.question_type,
      difficulty: question.difficulty,
      tagsText: formatTags(question.tags),
      category: question.category,
      knowledgePointsText: formatTags(question.knowledge_points),
      content: question.content,
      options: question.options.map((option) => ({ ...option })),
      answer: question.answer,
      explanation: question.explanation,
    };
  }
  return {
    subject: user.subject,
    grade: "",
    question_type: "single_choice",
    difficulty: 3,
    tagsText: "",
    category: "",
    knowledgePointsText: "",
    content: "",
    options: createChoiceOptions(),
    answer: "",
    explanation: "",
  };
}

export function toQuestionInput(state: QuestionFormState): QuestionInput {
  return {
    subject: state.subject.trim(),
    grade: state.grade.trim(),
    question_type: state.question_type,
    difficulty: state.difficulty,
    tags: parseTags(state.tagsText),
    category: (state.category ?? "").trim(),
    knowledge_points: parseTags(state.knowledgePointsText ?? ""),
    content: sanitizeRichText(state.content),
    options: isChoiceType(state.question_type)
      ? state.options.map((option) => ({ key: option.key, content: sanitizeRichText(option.content) }))
      : [],
    answer: state.question_type === "multiple_choice"
      ? normalizeChoiceAnswer(state.answer).split("").join(",")
      : isChoiceType(state.question_type)
        ? normalizeChoiceAnswer(state.answer)
        : state.question_type === "true_false" ? state.answer : sanitizeRichText(state.answer),
    explanation: sanitizeRichText(state.explanation),
  };
}
