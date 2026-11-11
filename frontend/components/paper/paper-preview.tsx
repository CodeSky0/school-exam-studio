"use client";

import type { Paper, PaperQuestion } from "@/lib/paper-schemas";
import { questionTypeLabels, type QuestionType } from "@/lib/question-schemas";
import { sumScores, toCents } from "@/lib/score";
import { RichTextView } from "@/components/question/rich-text-view";
import { usePaperStore } from "./paper-store-provider";

export interface PaperPreviewData {
  school_name: string;
  title: string;
  subject: string;
  grade: string;
  total_score: string;
  duration_minutes: number;
  questions: PaperQuestion[];
}

const sectionNumbers = ["一", "二", "三", "四", "五"];
const typeOrder = Object.keys(questionTypeLabels) as QuestionType[];

function safeTotal(scores: string[]): string {
  try {
    return sumScores(scores);
  } catch {
    return "—";
  }
}

function displayScore(score: string): string {
  try {
    toCents(score);
    return score;
  } catch {
    return "待设置";
  }
}

export function PaperPreview({ paper }: { paper: PaperPreviewData }) {
  const sortedQuestions = [...paper.questions].sort((left, right) => left.order_index - right.order_index);
  const groups = typeOrder.flatMap((type) => {
    const questions = sortedQuestions.filter((item) => item.question.question_type === type);
    return questions.length ? [{ type, questions }] : [];
  });
  const questionNumbers = new Map(groups.flatMap((group) => group.questions).map((item, index) => [item.question_id, index + 1]));

  return (
    <article className="paper-sheet" aria-label={`${paper.title || "未命名试卷"}预览`}>
      <div className="paper-candidate-info">
        <span>班级：__________</span><span>姓名：__________</span><span>考号：____________</span>
      </div>
      <div className="paper-seal" aria-label="密封线"><span>密 封 线 内 请 勿 答 题</span></div>
      <div className="paper-document-header">
        <p className="paper-school-name">{paper.school_name}</p>
        <h2 className="paper-title">{paper.title || "未命名试卷"}</h2>
        <p className="paper-metadata">
          <span>学科：{paper.subject || "未设置"}</span>
          <span>年级：{paper.grade || "未设置"}</span>
          <span>时长：{paper.duration_minutes > 0 ? `${paper.duration_minutes} 分钟` : "未设置"}</span>
          <span>总分：{paper.total_score} 分</span>
        </p>
      </div>
      {groups.length === 0 ? <p className="paper-empty">尚未添加题目</p> : (
        <div className="paper-sections">
          {groups.map((group, groupIndex) => (
            <section key={group.type} className="paper-section" aria-labelledby={`paper-section-${group.type}`}>
              <h3 id={`paper-section-${group.type}`} className="paper-section-heading">
                {sectionNumbers[groupIndex]}、{questionTypeLabels[group.type]}
                <span>（共 {group.questions.length} 题，合计 {safeTotal(group.questions.map((item) => item.assigned_score))} 分）</span>
              </h3>
              <div className="paper-questions">
                {group.questions.map((item) => {
                  const questionNumber = questionNumbers.get(item.question_id) ?? 0;
                  const question = item.question;
                  const needsAnswerBracket = question.question_type === "single_choice"
                    || question.question_type === "multiple_choice"
                    || question.question_type === "true_false";
                  return (
                    <section key={item.question_id} className={`paper-question paper-question--${question.question_type}`}>
                      <div className="paper-question-heading">
                        <span className="paper-question-number">{questionNumber}.</span>
                        {needsAnswerBracket ? <span className="paper-answer-bracket">（　　）</span> : null}
                        <span className="paper-question-score">（{displayScore(item.assigned_score)} 分）</span>
                      </div>
                      <RichTextView content={question.content} className="paper-rich-text" />
                      {question.options.length > 0 ? (
                        <div className="paper-options">
                          {question.options.map((option) => (
                            <div key={option.key} className="paper-option">
                              <span className="paper-option-key">{option.key}.</span>
                              <RichTextView content={option.content} className="paper-rich-text" />
                            </div>
                          ))}
                        </div>
                      ) : null}
                      {question.question_type === "fill_blank" ? <div className="paper-answer-space paper-answer-space--short" aria-label="填空答题留白" /> : null}
                      {question.question_type === "essay" ? (
                        <div className="paper-answer-space paper-answer-space--long" aria-label="简答答题留白">
                          <span /><span /><span />
                        </div>
                      ) : null}
                    </section>
                  );
                })}
              </div>
            </section>
          ))}
        </div>
      )}
    </article>
  );
}

export function LivePaperPreview({ paper }: { paper: Paper }) {
  const title = usePaperStore((state) => state.title);
  const subject = usePaperStore((state) => state.subject);
  const grade = usePaperStore((state) => state.grade);
  const durationMinutes = usePaperStore((state) => state.durationMinutes);
  const items = usePaperStore((state) => state.items);
  const questions = items.map((item, index) => ({
    question_id: item.question.id,
    order_index: index,
    assigned_score: item.assignedScore,
    question: item.question,
  }));
  return (
    <PaperPreview
      paper={{
        school_name: paper.school_name,
        title,
        subject,
        grade,
        total_score: safeTotal(items.map((item) => item.assignedScore)),
        duration_minutes: /^\d+$/.test(durationMinutes) ? Number(durationMinutes) : 0,
        questions,
      }}
    />
  );
}
