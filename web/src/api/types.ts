import type { components, operations } from "./schema";

type Schemas = components["schemas"];

export type Site = Schemas["Site"];
export type SiteHosting = Schemas["SiteHosting"];
export type Notice = Schemas["Notice"];
export type Workspace = Schemas["Workspace"];
export type Folder = Schemas["Folder"];
export type User = Schemas["User"];
export type Instance = Schemas["Instance"];
export type Role = Schemas["Role"];
export type AccessMode = Schemas["AccessMode"];
export type InvitePreview = Schemas["InvitePreview"];
export type SetupRequest = Schemas["SetupRequest"];
export type LoginRequest = Schemas["LoginRequest"];
export type AcceptInviteRequest = Schemas["AcceptInviteRequest"];
export type PasswordResetConfirm = Schemas["PasswordResetConfirm"];
export type SiteList = Schemas["SiteList"];
export type UpdateSiteRequest = Schemas["UpdateSiteRequest"];
export type ListSitesQuery = NonNullable<operations["listSites"]["parameters"]["query"]>;
