import { toast } from "sonner"

export const defaultMutationCallbacks = {
  onError: (err: Error) => toast.error(err.message),
}
