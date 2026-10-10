import type { JSX } from "react";
import { cn } from "@/lib/cn";
import { parseMarkdown, type Block, type Inline } from "./markdown";

export function InlineView({ nodes }: { nodes: Inline[] }): JSX.Element {
  return (
    <>
      {nodes.map((node, index) => {
        switch (node.kind) {
          case "code":
            return (
              <code key={index} className="rounded bg-sunken px-1.5 py-0.5 font-mono text-[0.85em]">
                {node.text}
              </code>
            );
          case "bold":
            return <strong key={index}>{node.text}</strong>;
          case "link":
            return (
              <a key={index} href={node.href} className="text-accent underline underline-offset-4">
                {node.text}
              </a>
            );
          default:
            return <span key={index}>{node.text}</span>;
        }
      })}
    </>
  );
}

const HEADING_CLASSES: Record<1 | 2 | 3 | 4, string> = {
  1: "mb-4 font-display text-3xl font-medium tracking-tight",
  2: "mb-3 mt-10 border-t border-line pt-6 font-display text-2xl font-medium tracking-tight",
  3: "mb-2 mt-7 text-lg font-semibold",
  4: "mb-2 mt-5 font-semibold",
};

function BlockView({ block }: { block: Block }): JSX.Element {
  switch (block.kind) {
    case "heading": {
      const Tag = `h${block.level}` as const;
      return (
        <Tag id={block.id} className={cn("scroll-mt-20", HEADING_CLASSES[block.level])}>
          {block.text}
        </Tag>
      );
    }
    case "paragraph":
      return (
        <p className="my-3 leading-relaxed">
          <InlineView nodes={block.content} />
        </p>
      );
    case "code":
      return (
        <pre className="my-4 overflow-x-auto rounded-lg border border-line bg-sunken p-4 font-mono text-[0.8125rem] leading-relaxed">
          <code>{block.text}</code>
        </pre>
      );
    case "list": {
      const Tag = block.ordered ? "ol" : "ul";
      return (
        <Tag className={cn("my-3 space-y-1.5 pl-6", block.ordered ? "list-decimal" : "list-disc")}>
          {block.items.map((item, index) => (
            <li key={index} className="pl-1 leading-relaxed">
              <InlineView nodes={item} />
            </li>
          ))}
        </Tag>
      );
    }
    case "table":
      return (
        <div className="my-4 overflow-x-auto rounded-lg border border-line">
          <table className="w-full min-w-[32rem] text-left text-sm">
            <thead className="bg-sunken text-xs uppercase tracking-wider text-muted">
              <tr>
                {block.header.map((cell, index) => (
                  <th key={index} scope="col" className="px-3 py-2 font-medium">
                    <InlineView nodes={cell} />
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {block.rows.map((row, rowIndex) => (
                <tr key={rowIndex} className="align-top">
                  {row.map((cell, index) => (
                    <td key={index} className="px-3 py-2">
                      <InlineView nodes={cell} />
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      );
  }
}

export function MarkdownView({ source }: { source: string }): JSX.Element {
  const blocks = parseMarkdown(source);
  return (
    <div className="max-w-3xl">
      {blocks.map((block, index) => (
        <BlockView key={index} block={block} />
      ))}
    </div>
  );
}
