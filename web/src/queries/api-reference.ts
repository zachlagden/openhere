import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import * as api from "@/api/endpoints";
import { buildApiReference, type ApiReference } from "@/features/docs/openapi-model";
import { queryKeys } from "./keys";

export function useApiReference(): UseQueryResult<ApiReference> {
  return useQuery({
    queryKey: [...queryKeys.openApi, "parsed"],
    staleTime: 10 * 60_000,
    queryFn: async ({ signal }) => {
      const [text, yaml] = await Promise.all([api.getOpenApiDocument({ signal }), import("yaml")]);
      return buildApiReference(yaml.parse(text));
    },
  });
}
