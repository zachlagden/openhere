export type Inline =
  | { kind: "text"; text: string }
  | { kind: "code"; text: string }
  | { kind: "bold"; text: string }
  | { kind: "link"; text: string; href: string };

export type Block =
  | { kind: "heading"; level: 1 | 2 | 3 | 4; text: string; id: string }
  | { kind: "paragraph"; content: Inline[] }
  | { kind: "code"; language: string; text: string }
  | { kind: "list"; ordered: boolean; items: Inline[][] }
  | { kind: "table"; header: Inline[][]; rows: Inline[][][] };

const INLINE_PATTERN = /`([^`]+)`|\*\*([^*]+)\*\*|\[([^\]]+)\]\(([^)\s]+)\)/g;

export function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export function isSafeHref(href: string): boolean {
  return /^(https?:\/\/|\/|#|mailto:)/i.test(href);
}

export function parseInline(source: string): Inline[] {
  const nodes: Inline[] = [];
  let cursor = 0;
  for (const match of source.matchAll(INLINE_PATTERN)) {
    const start = match.index ?? 0;
    if (start > cursor) {
      nodes.push({ kind: "text", text: source.slice(cursor, start) });
    }
    if (match[1] !== undefined) {
      nodes.push({ kind: "code", text: match[1] });
    } else if (match[2] !== undefined) {
      nodes.push({ kind: "bold", text: match[2] });
    } else if (match[3] !== undefined && match[4] !== undefined) {
      nodes.push(isSafeHref(match[4]) ? { kind: "link", text: match[3], href: match[4] } : { kind: "text", text: match[3] });
    }
    cursor = start + match[0].length;
  }
  if (cursor < source.length) {
    nodes.push({ kind: "text", text: source.slice(cursor) });
  }
  return nodes;
}

function splitRow(line: string): string[] {
  return line
    .trim()
    .replace(/^\|/, "")
    .replace(/\|$/, "")
    .split("|")
    .map((cell) => cell.trim());
}

function isTableSeparator(line: string): boolean {
  return /^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$/.test(line);
}

const LIST_ITEM = /^\s*(?:(\d+)\.|[-*])\s+(.*)$/;

export function parseMarkdown(source: string): Block[] {
  const lines = source.replace(/\r\n/g, "\n").split("\n");
  const blocks: Block[] = [];
  let index = 0;

  while (index < lines.length) {
    const line = lines[index] ?? "";

    if (line.trim() === "") {
      index += 1;
      continue;
    }

    const fence = /^```(\S*)\s*$/.exec(line);
    if (fence) {
      const body: string[] = [];
      index += 1;
      while (index < lines.length && !/^```\s*$/.test(lines[index] ?? "")) {
        body.push(lines[index] ?? "");
        index += 1;
      }
      index += 1;
      blocks.push({ kind: "code", language: fence[1] ?? "", text: body.join("\n") });
      continue;
    }

    const heading = /^(#{1,4})\s+(.*)$/.exec(line);
    if (heading) {
      const text = (heading[2] ?? "").trim();
      blocks.push({ kind: "heading", level: (heading[1]?.length ?? 1) as 1 | 2 | 3 | 4, text, id: slugify(text) });
      index += 1;
      continue;
    }

    if (line.includes("|") && isTableSeparator(lines[index + 1] ?? "")) {
      const header = splitRow(line).map(parseInline);
      const rows: Inline[][][] = [];
      index += 2;
      while (index < lines.length && (lines[index] ?? "").includes("|") && (lines[index] ?? "").trim() !== "") {
        rows.push(splitRow(lines[index] ?? "").map(parseInline));
        index += 1;
      }
      blocks.push({ kind: "table", header, rows });
      continue;
    }

    const first = LIST_ITEM.exec(line);
    if (first) {
      const ordered = first[1] !== undefined;
      const items: Inline[][] = [];
      while (index < lines.length) {
        const item = LIST_ITEM.exec(lines[index] ?? "");
        if (!item || (item[1] !== undefined) !== ordered) {
          break;
        }
        items.push(parseInline(item[2] ?? ""));
        index += 1;
      }
      blocks.push({ kind: "list", ordered, items });
      continue;
    }

    const paragraph: string[] = [];
    while (index < lines.length) {
      const current = lines[index] ?? "";
      if (current.trim() === "" || /^```/.test(current) || /^#{1,4}\s/.test(current) || LIST_ITEM.test(current)) {
        break;
      }
      paragraph.push(current.trim());
      index += 1;
    }
    blocks.push({ kind: "paragraph", content: parseInline(paragraph.join(" ")) });
  }
  return blocks;
}
