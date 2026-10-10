import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { personal, reportsFolder } from "@/test/fixtures";
import { mockApi } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { FolderNav } from "./folder-nav";

afterEach(() => {
  vi.unstubAllGlobals();
});

function renderNav(workspace: typeof personal): void {
  mockApi({ "GET /api/v1/workspaces/personal/folders": { body: { folders: [reportsFolder] } } });
  renderWithApp(<FolderNav workspace={workspace} activeFolder="" />);
}

describe("FolderNav", () => {
  it("offers rename and delete to a workspace admin", async () => {
    renderNav(personal);
    expect(await screen.findByRole("button", { name: "Options for folder Reports" })).toBeInTheDocument();
  });

  it("hides the folder menu from someone with only a grant on the folder", async () => {
    renderNav({ ...personal, role: "read", scopedOnly: true });
    await screen.findByRole("link", { name: "Reports" });
    expect(screen.queryByRole("button", { name: "Options for folder Reports" })).not.toBeInTheDocument();
  });

  it("hides the folder menu from a folder-scoped writer", async () => {
    renderNav({ ...personal, role: "write", scopedOnly: true });
    await screen.findByRole("link", { name: "Reports" });
    expect(screen.queryByRole("button", { name: "Options for folder Reports" })).not.toBeInTheDocument();
  });

  it("offers only rename to a workspace writer", async () => {
    renderNav({ ...personal, role: "write" });
    const menu = await screen.findByRole("button", { name: "Options for folder Reports" });
    await userEvent.click(menu);
    expect(await screen.findByRole("menuitem", { name: /Rename/ })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: /Delete/ })).not.toBeInTheDocument();
  });
});
