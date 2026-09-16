import { describe, expect, it } from "vitest";
import { parse, TYPE, type MessageFormatElement } from "@formatjs/icu-messageformat-parser";
import en from "../../messages/en.json";
import ru from "../../messages/ru.json";
import uz from "../../messages/uz.json";

type Tree = { [key: string]: string | Tree };

function flatten(tree: Tree, prefix = ""): Map<string, string> {
  const out = new Map<string, string>();
  for (const [k, v] of Object.entries(tree)) {
    if (typeof v === "string") out.set(prefix + k, v);
    else for (const [ck, cv] of flatten(v, `${prefix}${k}.`)) out.set(ck, cv);
  }
  return out;
}

const catalogues = { en: flatten(en), ru: flatten(ru), uz: flatten(uz) };

/** ICU argument names used in a message, e.g. {count} / {name, plural, …}. */
function argNames(msg: string): string[] {
  const names = new Set<string>();
  const walk = (els: MessageFormatElement[]) => {
    for (const el of els) {
      switch (el.type) {
        case TYPE.literal:
        case TYPE.pound:
          break;
        case TYPE.select:
        case TYPE.plural:
          names.add(el.value);
          for (const opt of Object.values(el.options)) walk(opt.value);
          break;
        case TYPE.tag:
          names.add(el.value);
          walk(el.children);
          break;
        default:
          names.add(el.value);
      }
    }
  };
  walk(parse(msg));
  return [...names].sort();
}

describe("message catalogues", () => {
  it("ru and uz carry exactly the keys en has", () => {
    const enKeys = [...catalogues.en.keys()].sort();
    expect([...catalogues.ru.keys()].sort()).toEqual(enKeys);
    expect([...catalogues.uz.keys()].sort()).toEqual(enKeys);
  });

  it("every message parses as ICU", () => {
    for (const [loc, cat] of Object.entries(catalogues)) {
      for (const [key, msg] of cat) {
        expect(() => parse(msg), `${loc}:${key}`).not.toThrow();
      }
    }
  });

  it("no message is empty", () => {
    for (const [loc, cat] of Object.entries(catalogues)) {
      for (const [key, msg] of cat) {
        expect(msg.trim(), `${loc}:${key}`).not.toBe("");
      }
    }
  });

  it("translations use the same ICU arguments and tags as English", () => {
    for (const loc of ["ru", "uz"] as const) {
      for (const [key, msg] of catalogues[loc]) {
        const source = catalogues.en.get(key)!;
        expect(argNames(msg), `${loc}:${key}`).toEqual(argNames(source));
      }
    }
  });
});
