import StarterKit from "@tiptap/starter-kit";
import { mergeAttributes } from "@tiptap/core";
import { BlockMath, InlineMath } from "@tiptap/extension-mathematics";
import type { KatexOptions } from "katex";

const katexOptions: KatexOptions = {
  throwOnError: false,
  strict: "error",
  trust: false,
  output: "html",
};

const StorageInlineMath = InlineMath.extend({
  parseHTML() {
    return [{ tag: 'span[data-type="inline-math"]' }, { tag: 'span[data-type="inlineMath"]' }];
  },
  renderHTML({ HTMLAttributes }) {
    return ["span", mergeAttributes(HTMLAttributes, { "data-type": "inlineMath" })];
  },
});

const StorageBlockMath = BlockMath.extend({
  parseHTML() {
    return [{ tag: 'div[data-type="block-math"]' }, { tag: 'div[data-type="blockMath"]' }];
  },
  renderHTML({ HTMLAttributes }) {
    return ["div", mergeAttributes(HTMLAttributes, { "data-type": "blockMath" })];
  },
});

export const questionEditorExtensions = [
  StarterKit.configure({
    heading: { levels: [2, 3] },
    link: false,
    code: false,
    codeBlock: false,
    horizontalRule: false,
  }),
  StorageInlineMath.configure({ katexOptions: { ...katexOptions, displayMode: false } }),
  StorageBlockMath.configure({ katexOptions: { ...katexOptions, displayMode: true } }),
];
