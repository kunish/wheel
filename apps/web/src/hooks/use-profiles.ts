import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { createProfile, deleteProfile, listProfiles, updateProfile } from "@/lib/api"

export function useProfilesQuery() {
  return useQuery({
    queryKey: ["model-profiles"],
    queryFn: listProfiles,
    staleTime: 30_000,
  })
}

export function useCreateProfile() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: createProfile,
    onSuccess: () => qc.invalidateQueries({ queryKey: ["model-profiles"] }),
    onError: (err: Error) => toast.error(err.message),
  })
}

export function useUpdateProfile() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: updateProfile,
    onSuccess: () => qc.invalidateQueries({ queryKey: ["model-profiles"] }),
    onError: (err: Error) => toast.error(err.message),
  })
}

export function useDeleteProfile() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => deleteProfile(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["model-profiles"] }),
    onError: (err: Error) => toast.error(err.message),
  })
}
