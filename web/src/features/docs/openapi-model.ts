export interface SchemaProperty {
  name: string;
  type: string;
  required: boolean;
  description: string;
}

export interface Parameter {
  name: string;
  location: string;
  required: boolean;
  description: string;
}

export interface ResponseSummary {
  status: string;
  description: string;
}

export interface Operation {
  id: string;
  method: string;
  path: string;
  summary: string;
  description: string;
  public: boolean;
  parameters: Parameter[];
  requestSchema: string | null;
  responses: ResponseSummary[];
}

export interface TagGroup {
  name: string;
  operations: Operation[];
}

export interface ApiReference {
  title: string;
  version: string;
  description: string;
  groups: TagGroup[];
  schemas: Record<string, SchemaProperty[]>;
}

type Json = Record<string, unknown>;

const METHODS = ["get", "post", "put", "patch", "delete"] as const;

function asRecord(value: unknown): Json {
  return typeof value === "object" && value !== null && !Array.isArray(value) ? (value as Json) : {};
}

function asString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

export function refName(value: unknown): string | null {
  const ref = asString(asRecord(value)["$ref"]);
  const marker = "#/components/schemas/";
  return ref.startsWith(marker) ? ref.slice(marker.length) : null;
}

function describeType(schema: unknown): string {
  const record = asRecord(schema);
  const named = refName(record);
  if (named) {
    return named;
  }
  const type = record["type"];
  const base = Array.isArray(type) ? type.map(String).join(" or ") : asString(type);
  if (base === "array") {
    const items = describeType(record["items"]);
    return items === "" ? "array" : `${items}[]`;
  }
  const options = record["enum"];
  if (Array.isArray(options) && options.length > 0 && options.length <= 6) {
    return options.map(String).join(" | ");
  }
  return base;
}

function readProperties(schema: unknown): SchemaProperty[] {
  const record = asRecord(schema);
  const required = new Set(Array.isArray(record["required"]) ? record["required"].map(String) : []);
  return Object.entries(asRecord(record["properties"])).map(([name, value]) => ({
    name,
    type: describeType(value),
    required: required.has(name),
    description: asString(asRecord(value)["description"]),
  }));
}

function resolveParameter(value: unknown, components: Json): Parameter | null {
  const record = asRecord(value);
  const ref = asString(record["$ref"]);
  const marker = "#/components/parameters/";
  const resolved = ref.startsWith(marker) ? asRecord(asRecord(components["parameters"])[ref.slice(marker.length)]) : record;
  const name = asString(resolved["name"]);
  if (name === "") {
    return null;
  }
  return {
    name,
    location: asString(resolved["in"]),
    required: resolved["required"] === true,
    description: asString(resolved["description"]),
  };
}

function readOperation(path: string, method: string, value: unknown, shared: unknown[], components: Json): Operation {
  const record = asRecord(value);
  const body = asRecord(asRecord(asRecord(asRecord(record["requestBody"])["content"])["application/json"])["schema"]);
  const parameters = [...shared, ...(Array.isArray(record["parameters"]) ? record["parameters"] : [])]
    .map((item) => resolveParameter(item, components))
    .filter((item): item is Parameter => item !== null);
  const responses = Object.entries(asRecord(record["responses"])).map(([status, response]) => {
    const resolved = asRecord(response);
    const ref = asString(resolved["$ref"]);
    const target = ref.startsWith("#/components/responses/") ? asRecord(asRecord(components["responses"])[ref.split("/").pop() ?? ""]) : resolved;
    return { status, description: asString(target["description"]).trim() };
  });
  return {
    id: asString(record["operationId"]) || `${method}-${path}`,
    method: method.toUpperCase(),
    path,
    summary: asString(record["summary"]),
    description: asString(record["description"]).trim(),
    public: Array.isArray(record["security"]) && record["security"].length === 0,
    parameters,
    requestSchema: refName(body),
    responses,
  };
}

export function buildApiReference(spec: unknown): ApiReference {
  const root = asRecord(spec);
  const info = asRecord(root["info"]);
  const components = asRecord(root["components"]);
  const groups = new Map<string, Operation[]>();
  for (const tag of Array.isArray(root["tags"]) ? root["tags"] : []) {
    groups.set(asString(asRecord(tag)["name"]), []);
  }
  for (const [path, item] of Object.entries(asRecord(root["paths"]))) {
    const pathItem = asRecord(item);
    const shared = Array.isArray(pathItem["parameters"]) ? pathItem["parameters"] : [];
    for (const method of METHODS) {
      if (pathItem[method] === undefined) {
        continue;
      }
      const operation = readOperation(path, method, pathItem[method], shared, components);
      const tags = asRecord(pathItem[method])["tags"];
      const tag = Array.isArray(tags) && typeof tags[0] === "string" ? tags[0] : "Other";
      groups.set(tag, [...(groups.get(tag) ?? []), operation]);
    }
  }
  const schemas: Record<string, SchemaProperty[]> = {};
  for (const [name, schema] of Object.entries(asRecord(components["schemas"]))) {
    schemas[name] = readProperties(schema);
  }
  return {
    title: asString(info["title"]),
    version: asString(info["version"]),
    description: asString(info["description"]).trim(),
    groups: [...groups.entries()].filter(([, operations]) => operations.length > 0).map(([name, operations]) => ({ name, operations })),
    schemas,
  };
}

export function filterReference(reference: ApiReference, query: string): TagGroup[] {
  const needle = query.trim().toLowerCase();
  if (needle === "") {
    return reference.groups;
  }
  return reference.groups
    .map((group) => ({
      name: group.name,
      operations: group.operations.filter((operation) =>
        `${operation.method} ${operation.path} ${operation.summary} ${operation.id}`.toLowerCase().includes(needle),
      ),
    }))
    .filter((group) => group.operations.length > 0);
}
