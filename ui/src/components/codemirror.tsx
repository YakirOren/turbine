import { useEffect, useRef } from "react";
import { EditorView, placeholder as cmPlaceholder, keymap } from "@codemirror/view";
import { EditorState } from "@codemirror/state";
import { json } from "@codemirror/lang-json";
import { oneDark } from "@codemirror/theme-one-dark";
import { defaultHighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { useTheme } from "next-themes";
import { cn } from "@/lib/utils";

const baseTheme = EditorView.theme({
  "&": {
    fontSize: "13px",
    backgroundColor: "transparent",
  },
  "&.cm-focused": {
    outline: "none",
  },
  ".cm-scroller": {
    fontFamily: "var(--font-mono)",
  },
  ".cm-content": {
    padding: "8px 0",
  },
  ".cm-line": {
    padding: "0 12px",
  },
  ".cm-gutters": {
    backgroundColor: "transparent",
    border: "none",
    color: "hsl(var(--muted-foreground))",
  },
  ".cm-placeholder": {
    color: "hsl(var(--muted-foreground))",
  },
});

interface CodeMirrorEditorProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  readOnly?: boolean;
  minHeight?: string;
  maxHeight?: string;
  className?: string;
}

export function CodeMirrorEditor({
  value,
  onChange,
  placeholder = "",
  readOnly = false,
  minHeight = "80px",
  maxHeight = "300px",
  className,
}: CodeMirrorEditorProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  const { resolvedTheme } = useTheme();
  const isDark = resolvedTheme === "dark";

  useEffect(() => {
    if (!containerRef.current) return;

    const extensions = [
      baseTheme,
      json(),
      EditorView.lineWrapping,
      history(),
      keymap.of([...defaultKeymap, ...historyKeymap]),
      EditorView.updateListener.of((update) => {
        if (update.docChanged && update.transactions.some((t) => t.isUserEvent("input") || t.isUserEvent("delete"))) {
          onChangeRef.current(update.state.doc.toString());
        }
      }),
    ];

    if (placeholder) {
      extensions.push(cmPlaceholder(placeholder));
    }
    if (readOnly) {
      extensions.push(EditorState.readOnly.of(true));
    }
    if (isDark) {
      extensions.push(oneDark);
    } else {
      extensions.push(syntaxHighlighting(defaultHighlightStyle));
    }

    const state = EditorState.create({
      doc: value,
      extensions,
    });

    const view = new EditorView({
      state,
      parent: containerRef.current,
    });

    viewRef.current = view;

    return () => {
      view.destroy();
      viewRef.current = null;
    };
    // Recreate on theme change
  }, [isDark, readOnly, placeholder]);

  // Reconcile external value changes
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const current = view.state.doc.toString();
    if (current !== value) {
      view.dispatch({
        changes: { from: 0, to: current.length, insert: value },
      });
    }
  }, [value]);

  return (
    <div
      ref={containerRef}
      className={cn(
        "overflow-auto rounded-md border border-input bg-background ring-offset-background focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2",
        className,
      )}
      style={{ minHeight, maxHeight }}
    />
  );
}
