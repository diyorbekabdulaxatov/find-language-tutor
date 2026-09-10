"use client";

import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import type { Question, QuestionKind } from "@/features/resources/types";

const uid = () =>
  typeof crypto !== "undefined" && crypto.randomUUID
    ? crypto.randomUUID()
    : Math.random().toString(36).slice(2);

export function newQuestion(): Question {
  return {
    id: uid(),
    prompt: "",
    kind: "single",
    choices: [
      { id: uid(), text: "" },
      { id: uid(), text: "" },
    ],
    correct: [],
    points: 1,
  };
}

export function QuestionBuilder({
  questions,
  onChange,
}: {
  questions: Question[];
  onChange: (next: Question[]) => void;
}) {
  const update = (i: number, patch: Partial<Question>) =>
    onChange(questions.map((q, idx) => (idx === i ? { ...q, ...patch } : q)));

  return (
    <div className="flex flex-col gap-4">
      {questions.map((q, i) => (
        <QuestionCard
          key={q.id}
          index={i}
          question={q}
          onChange={(patch) => update(i, patch)}
          onRemove={() => onChange(questions.filter((_, idx) => idx !== i))}
        />
      ))}
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="self-start"
        onClick={() => onChange([...questions, newQuestion()])}
      >
        <Plus className="size-4" />
        Add question
      </Button>
    </div>
  );
}

function QuestionCard({
  index,
  question: q,
  onChange,
  onRemove,
}: {
  index: number;
  question: Question;
  onChange: (patch: Partial<Question>) => void;
  onRemove: () => void;
}) {
  function setKind(kind: QuestionKind) {
    if (kind === "text") {
      onChange({ kind, choices: [], correct: [] });
    } else {
      const choices = q.choices.length >= 2 ? q.choices : [{ id: uid(), text: "" }, { id: uid(), text: "" }];
      onChange({ kind, choices, correct: kind === "single" ? q.correct.slice(0, 1) : q.correct });
    }
  }

  function toggleCorrect(choiceId: string) {
    if (q.kind === "single") {
      onChange({ correct: [choiceId] });
    } else {
      onChange({
        correct: q.correct.includes(choiceId)
          ? q.correct.filter((c) => c !== choiceId)
          : [...q.correct, choiceId],
      });
    }
  }

  return (
    <div className="rounded-xl border border-border bg-card p-4">
      <div className="flex items-start gap-2">
        <span className="mt-2 text-xs font-semibold text-muted-foreground">Q{index + 1}</span>
        <Input
          value={q.prompt}
          placeholder="Question prompt"
          onChange={(e) => onChange({ prompt: e.target.value })}
        />
        <button
          type="button"
          aria-label="Remove question"
          className="mt-1.5 rounded p-1 text-muted-foreground hover:bg-muted hover:text-destructive"
          onClick={onRemove}
        >
          <Trash2 className="size-4" />
        </button>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-2 text-sm">
        <select
          value={q.kind}
          onChange={(e) => setKind(e.target.value as QuestionKind)}
          className="rounded-lg border border-input bg-transparent px-2 py-1 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <option value="single">Single choice</option>
          <option value="multi">Multiple choice</option>
          <option value="text">Text answer</option>
        </select>
        <label className="flex items-center gap-1.5 text-muted-foreground">
          Points
          <Input
            type="number"
            min={1}
            value={q.points}
            onChange={(e) => onChange({ points: Math.max(1, Number(e.target.value) || 1) })}
            className="h-8 w-16"
          />
        </label>
      </div>

      {q.kind === "text" ? (
        <TextAnswers
          accepted={q.correct}
          onChange={(correct) => onChange({ correct })}
        />
      ) : (
        <div className="mt-3 flex flex-col gap-2">
          {q.choices.map((c) => (
            <div key={c.id} className="flex items-center gap-2">
              <button
                type="button"
                aria-label={q.correct.includes(c.id) ? "Correct answer" : "Mark correct"}
                onClick={() => toggleCorrect(c.id)}
                className={cn(
                  "grid size-5 shrink-0 place-items-center border text-xs",
                  q.kind === "single" ? "rounded-full" : "rounded",
                  q.correct.includes(c.id)
                    ? "border-primary bg-primary text-primary-foreground"
                    : "border-input",
                )}
              >
                {q.correct.includes(c.id) ? "✓" : ""}
              </button>
              <Input
                value={c.text}
                placeholder="Choice"
                onChange={(e) =>
                  onChange({
                    choices: q.choices.map((x) => (x.id === c.id ? { ...x, text: e.target.value } : x)),
                  })
                }
              />
              {q.choices.length > 2 && (
                <button
                  type="button"
                  aria-label="Remove choice"
                  className="rounded p-1 text-muted-foreground hover:bg-muted hover:text-destructive"
                  onClick={() =>
                    onChange({
                      choices: q.choices.filter((x) => x.id !== c.id),
                      correct: q.correct.filter((id) => id !== c.id),
                    })
                  }
                >
                  <Trash2 className="size-3.5" />
                </button>
              )}
            </div>
          ))}
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="self-start"
            onClick={() => onChange({ choices: [...q.choices, { id: uid(), text: "" }] })}
          >
            <Plus className="size-3.5" />
            Add choice
          </Button>
        </div>
      )}
    </div>
  );
}

function TextAnswers({
  accepted,
  onChange,
}: {
  accepted: string[];
  onChange: (next: string[]) => void;
}) {
  const rows = accepted.length ? accepted : [""];
  return (
    <div className="mt-3 flex flex-col gap-2">
      <span className="text-xs text-muted-foreground">Accepted answers (case-sensitive)</span>
      {rows.map((a, i) => (
        <div key={i} className="flex items-center gap-2">
          <Input
            value={a}
            placeholder="Accepted answer"
            onChange={(e) => {
              const next = [...rows];
              next[i] = e.target.value;
              onChange(next);
            }}
          />
          {rows.length > 1 && (
            <button
              type="button"
              aria-label="Remove answer"
              className="rounded p-1 text-muted-foreground hover:bg-muted hover:text-destructive"
              onClick={() => onChange(rows.filter((_, idx) => idx !== i))}
            >
              <Trash2 className="size-3.5" />
            </button>
          )}
        </div>
      ))}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="self-start"
        onClick={() => onChange([...rows, ""])}
      >
        <Plus className="size-3.5" />
        Add accepted answer
      </Button>
    </div>
  );
}
