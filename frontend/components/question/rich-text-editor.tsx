"use client";

import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { EditorContent, useEditor, type Editor } from "@tiptap/react";
import {
  FontBoldIcon, FontItalicIcon, ListBulletIcon, QuoteIcon, ResetIcon, StrikethroughIcon, UnderlineIcon,
} from "@radix-ui/react-icons";
import { sanitizeRichText } from "@/lib/rich-text";
import { cn } from "@/lib/utils";
import { questionEditorExtensions } from "./math-extension";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface RichTextEditorProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  onActivate?: () => void;
  error?: string;
  compact?: boolean;
}

function ToolbarButton({ label, active = false, disabled = false, onPress, children }: {
  label: string;
  active?: boolean;
  disabled?: boolean;
  onPress: () => void;
  children: ReactNode;
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      aria-label={label}
      aria-pressed={active}
      title={label}
      disabled={disabled}
      onMouseDown={(event) => event.preventDefault()}
      onClick={onPress}
      className={cn("h-8 min-w-8 px-2", active && "bg-accent-soft text-accent-strong hover:bg-accent-soft-strong")}
    >
      {children}
    </Button>
  );
}

function EditorToolbar({ editor, onMath }: { editor: Editor; onMath: (mode: "inline" | "block") => void }) {
  return (
    <div className="flex flex-wrap items-center gap-0.5 border-b border-border bg-neutral-2/80 px-2 py-1.5" role="toolbar" aria-label="富文本排版工具">
      <ToolbarButton label="加粗" active={editor.isActive("bold")} onPress={() => editor.chain().focus().toggleBold().run()}><FontBoldIcon /></ToolbarButton>
      <ToolbarButton label="斜体" active={editor.isActive("italic")} onPress={() => editor.chain().focus().toggleItalic().run()}><FontItalicIcon /></ToolbarButton>
      <ToolbarButton label="下划线" active={editor.isActive("underline")} onPress={() => editor.chain().focus().toggleUnderline().run()}><UnderlineIcon /></ToolbarButton>
      <ToolbarButton label="删除线" active={editor.isActive("strike")} onPress={() => editor.chain().focus().toggleStrike().run()}><StrikethroughIcon /></ToolbarButton>
      <span className="mx-1 h-5 w-px bg-border" aria-hidden="true" />
      <ToolbarButton label="二级标题" active={editor.isActive("heading", { level: 2 })} onPress={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}><span className="text-label-12 font-medium">H2</span></ToolbarButton>
      <ToolbarButton label="无序列表" active={editor.isActive("bulletList")} onPress={() => editor.chain().focus().toggleBulletList().run()}><ListBulletIcon /></ToolbarButton>
      <ToolbarButton label="有序列表" active={editor.isActive("orderedList")} onPress={() => editor.chain().focus().toggleOrderedList().run()}><span className="font-mono text-label-12">1.</span></ToolbarButton>
      <ToolbarButton label="引用" active={editor.isActive("blockquote")} onPress={() => editor.chain().focus().toggleBlockquote().run()}><QuoteIcon /></ToolbarButton>
      <span className="mx-1 h-5 w-px bg-border" aria-hidden="true" />
      <ToolbarButton label="插入行内公式" onPress={() => onMath("inline")}><span className="font-mono text-label-12">$x$</span></ToolbarButton>
      <ToolbarButton label="插入独立公式" onPress={() => onMath("block")}><span className="font-mono text-label-12">$$x$$</span></ToolbarButton>
      <span className="mx-1 h-5 w-px bg-border" aria-hidden="true" />
      <ToolbarButton label="撤销" disabled={!editor.can().undo()} onPress={() => editor.chain().focus().undo().run()}><ResetIcon /></ToolbarButton>
      <ToolbarButton label="重做" disabled={!editor.can().redo()} onPress={() => editor.chain().focus().redo().run()}><ResetIcon className="-scale-x-100" /></ToolbarButton>
    </div>
  );
}

export function RichTextEditor({ label, value, onChange, onActivate, error, compact = false }: RichTextEditorProps) {
  const editorId = useId();
  const messageId = `${editorId}-message`;
  const onChangeRef = useRef(onChange);
  const onActivateRef = useRef(onActivate);
  const [mathMode, setMathMode] = useState<"inline" | "block" | null>(null);
  const [latex, setLatex] = useState("");

  useEffect(() => { onChangeRef.current = onChange; }, [onChange]);
  useEffect(() => { onActivateRef.current = onActivate; }, [onActivate]);

  const editor = useEditor({
    extensions: questionEditorExtensions,
    content: sanitizeRichText(value) || "<p></p>",
    immediatelyRender: false,
    shouldRerenderOnTransaction: true,
    editorProps: {
      attributes: {
        id: editorId,
        "aria-label": label,
        "aria-describedby": messageId,
        "aria-multiline": "true",
        class: cn("question-prosemirror", compact ? "min-h-20" : "min-h-36"),
      },
    },
    onUpdate: ({ editor: currentEditor }) => onChangeRef.current(sanitizeRichText(currentEditor.getHTML())),
    onFocus: () => onActivateRef.current?.(),
  }, []);

  useEffect(() => {
    if (!editor) return;
    const nextValue = sanitizeRichText(value);
    if (nextValue !== sanitizeRichText(editor.getHTML())) {
      editor.commands.setContent(nextValue || "<p></p>", { emitUpdate: false });
    }
  }, [editor, value]);

  function insertMath() {
    const normalized = latex.trim();
    if (!editor || !normalized) return;
    const chain = editor.chain().focus();
    if (mathMode === "inline") chain.insertInlineMath({ latex: normalized }).run();
    if (mathMode === "block") chain.insertBlockMath({ latex: normalized }).run();
    setLatex("");
    setMathMode(null);
  }

  return (
    <div className="grid gap-2">
      <Label htmlFor={editorId}>{label}</Label>
      <div className={cn("overflow-hidden rounded-lg border border-border-strong bg-neutral-1 shadow-sm transition-[border-color,box-shadow] focus-within:border-accent focus-within:ring-3 focus-within:ring-accent/15", error && "border-error")}>
        {editor ? <EditorToolbar editor={editor} onMath={(mode) => setMathMode(mode)} /> : null}
        <EditorContent editor={editor} />
        {!editor ? <div className={cn("animate-pulse bg-neutral-2", compact ? "h-20" : "h-36")} aria-label="正在加载编辑器" /> : null}
        {mathMode && editor ? (
          <div className="grid gap-2 border-t border-border bg-accent-soft/45 p-3 sm:grid-cols-[minmax(0,1fr)_auto_auto] sm:items-end">
            <div className="grid gap-1.5">
              <Label htmlFor={`${editorId}-latex`} className="text-label-12">{mathMode === "inline" ? "行内 LaTeX 公式" : "独立 LaTeX 公式"}</Label>
              <Input
                id={`${editorId}-latex`}
                value={latex}
                autoFocus
                onChange={(event) => setLatex(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") { event.preventDefault(); insertMath(); }
                  if (event.key === "Escape") setMathMode(null);
                }}
                placeholder="例如 \\frac{a+b}{2}"
                className="h-9 bg-neutral-1 font-mono text-label-12"
              />
            </div>
            <Button type="button" size="sm" className="h-9" disabled={!latex.trim()} onClick={insertMath}>插入公式</Button>
            <Button type="button" size="sm" variant="ghost" className="h-9" onClick={() => setMathMode(null)}>取消</Button>
          </div>
        ) : null}
      </div>
      <p id={messageId} className={cn("text-label-12 leading-5", error ? "text-error" : "text-neutral-7")}>
        {error ?? "支持基础排版、$…$ 行内公式与 $$…$$ 独立公式；图片不会被保存。"}
      </p>
    </div>
  );
}
