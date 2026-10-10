import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { reportsFolder } from "@/test/fixtures";
import { apiError, mockApi } from "@/test/mock-api";
import { renderWithApp } from "@/test/render";
import { CreateFolderDialog, DeleteFolderDialog, RenameFolderDialog } from "./folder-dialogs";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("folder dialogs", () => {
  it("creates a folder with a trimmed name and closes", async () => {
    const fetchMock = mockApi({ "POST /api/v1/workspaces/personal/folders": { status: 201, body: { folder: reportsFolder } } });
    const onOpenChange = vi.fn();
    renderWithApp(<CreateFolderDialog workspace="personal" open onOpenChange={onOpenChange} />);
    await userEvent.type(await screen.findByLabelText("Name"), "  Reports ");
    await userEvent.click(screen.getByRole("button", { name: "Create folder" }));
    await vi.waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
    const call = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(JSON.parse(String(call[1].body))).toEqual({ name: "Reports" });
  });

  it("shows a duplicate name error and stays open", async () => {
    mockApi({ "POST /api/v1/workspaces/personal/folders": apiError(409, "folder_name_taken", "A folder with that name already exists.") });
    const onOpenChange = vi.fn();
    renderWithApp(<CreateFolderDialog workspace="personal" open onOpenChange={onOpenChange} />);
    await userEvent.type(await screen.findByLabelText("Name"), "Reports");
    await userEvent.click(screen.getByRole("button", { name: "Create folder" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("A folder with that name already exists.");
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });

  it("renames a folder", async () => {
    const fetchMock = mockApi({ "PATCH /api/v1/workspaces/personal/folders/f1": { body: { folder: { ...reportsFolder, name: "Archive" } } } });
    const onOpenChange = vi.fn();
    renderWithApp(<RenameFolderDialog workspace="personal" folder={reportsFolder} open onOpenChange={onOpenChange} />);
    const field = await screen.findByLabelText("Name");
    expect(field).toHaveValue("Reports");
    await userEvent.clear(field);
    await userEvent.type(field, "Archive");
    await userEvent.click(screen.getByRole("button", { name: "Save name" }));
    await vi.waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
    expect(JSON.parse(String((fetchMock.mock.calls[0] as [string, RequestInit])[1].body))).toEqual({ name: "Archive" });
  });

  it("confirms before deleting and reports the result", async () => {
    mockApi({ "DELETE /api/v1/workspaces/personal/folders/f1": { status: 204 } });
    const onDeleted = vi.fn();
    renderWithApp(<DeleteFolderDialog workspace="personal" folder={reportsFolder} open onOpenChange={() => undefined} onDeleted={onDeleted} />);
    expect(await screen.findByText(/become unfiled/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Delete folder" }));
    await vi.waitFor(() => expect(onDeleted).toHaveBeenCalledOnce());
  });

  it("explains a refused delete and offers to delete anyway", async () => {
    const fetchMock = mockApi({
      "DELETE /api/v1/workspaces/personal/folders/f1": (request) =>
        request.search.get("force") === "true"
          ? { status: 204 }
          : apiError(409, "folder_has_grants", "People still have access to this folder. Remove their access first, or delete the folder anyway."),
    });
    const onDeleted = vi.fn();
    renderWithApp(<DeleteFolderDialog workspace="personal" folder={reportsFolder} open onOpenChange={() => undefined} onDeleted={onDeleted} />);
    await userEvent.click(await screen.findByRole("button", { name: "Delete folder" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("People still have access to this folder.");
    expect(onDeleted).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "Delete anyway" }));
    await vi.waitFor(() => expect(onDeleted).toHaveBeenCalledOnce());
    const urls = fetchMock.mock.calls.map((call) => String(call[0]));
    expect(urls[0]).toMatch(/folders\/f1$/);
    expect(urls[1]).toMatch(/folders\/f1\?force=true$/);
  });
});
