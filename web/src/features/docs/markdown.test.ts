import { describe, expect, it } from "vitest";
import { parseInline, parseMarkdown } from "./markdown";

describe("parseInline", () => {
  it("parses code, bold and safe links", () => {
    expect(parseInline("Use `GET /me` and **care** with [docs](/docs)")).toEqual([
      { kind: "text", text: "Use " },
      { kind: "code", text: "GET /me" },
      { kind: "text", text: " and " },
      { kind: "bold", text: "care" },
      { kind: "text", text: " with " },
      { kind: "link", text: "docs", href: "/docs" },
    ]);
  });

  it("drops unsafe link targets", () => {
    expect(parseInline("[x](javascript:void)")).toEqual([{ kind: "text", text: "x" }]);
  });
});

describe("parseMarkdown", () => {
  it("parses headings, lists, code and tables", () => {
    const blocks = parseMarkdown("# Title\n\nText line one\nline two\n\n1. a\n2. b\n\n```json\n{}\n```\n\n| A | B |\n|---|---|\n| 1 | 2 |\n");
    expect(blocks.map((block) => block.kind)).toEqual(["heading", "paragraph", "list", "code", "table"]);
    expect(blocks[0]).toMatchObject({ level: 1, id: "title" });
    expect(blocks[2]).toMatchObject({ ordered: true });
    expect(blocks[3]).toMatchObject({ language: "json", text: "{}" });
  });

  it("never produces markup from raw HTML", () => {
    const [block] = parseMarkdown("<script>alert(1)</script>");
    expect(block).toMatchObject({ kind: "paragraph", content: [{ kind: "text", text: "<script>alert(1)</script>" }] });
  });
});
