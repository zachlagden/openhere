import { useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import * as api from "@/api/admin";
import type { AdminUser, EmailTestResult, HostingStatus, ResetLink, Settings, SettingsPatch, UpdateSettingsEnvelope, WildcardCheckResult } from "@/api/types";
import { queryKeys } from "./keys";

export function useHostingStatus(enabled = true): UseQueryResult<HostingStatus> {
  return useQuery({
    queryKey: queryKeys.hostingStatus,
    enabled,
    queryFn: ({ signal }) => api.getHostingStatus({ signal }),
  });
}

export function useAdminSettings(): UseQueryResult<Settings> {
  return useQuery({
    queryKey: queryKeys.adminSettings,
    queryFn: ({ signal }) => api.getSettings({ signal }),
  });
}

export function useUpdateSettings(): UseMutationResult<UpdateSettingsEnvelope, Error, SettingsPatch> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: api.patchSettings,
    onSuccess: async (result) => {
      client.setQueryData(queryKeys.adminSettings, result.settings);
      await Promise.all([
        client.invalidateQueries({ queryKey: queryKeys.hostingStatus }),
        client.invalidateQueries({ queryKey: queryKeys.instance }),
        client.invalidateQueries({ queryKey: queryKeys.sites }),
      ]);
    },
  });
}

export function useCheckWildcard(): UseMutationResult<WildcardCheckResult, Error, void> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: api.checkWildcard,
    onSuccess: async (result) => {
      client.setQueryData(queryKeys.adminSettings, result.settings);
      await client.invalidateQueries({ queryKey: queryKeys.hostingStatus });
    },
  });
}

export function useSendTestEmail(): UseMutationResult<EmailTestResult, Error, void> {
  return useMutation({ mutationFn: api.sendTestEmail });
}

export function useAdminUsers(): UseQueryResult<AdminUser[]> {
  return useQuery({
    queryKey: queryKeys.adminUsers,
    queryFn: ({ signal }) => api.listUsers({ signal }),
  });
}

export function useSetUserDisabled(): UseMutationResult<AdminUser, Error, { id: string; disabled: boolean }> {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ id, disabled }) => api.setUserDisabled(id, disabled),
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.adminUsers }),
  });
}

export function useCreateResetLink(): UseMutationResult<ResetLink, Error, string> {
  return useMutation({ mutationFn: api.createResetLink });
}
