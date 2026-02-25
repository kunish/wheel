import { useCallback, useRef, useState } from "react"
import { useLocation, useNavigate, useSearchParams } from "react-router"
import { buildFilterSearchParams, parseLogFilters } from "@/pages/logs/log-filters"

export function useLogFilters() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const { pathname } = useLocation()

  const filters = parseLogFilters(searchParams)
  const { page, model, status, channelId, keyword, pageSize, startTime, endTime } = filters

  const [keywordInput, setKeywordInput] = useState(keyword)

  // Sync local input state when URL changes externally
  const prevKeywordRef = useRef(keyword)
  if (prevKeywordRef.current !== keyword) {
    prevKeywordRef.current = keyword
    setKeywordInput(keyword)
  }

  const updateFilter = useCallback(
    (updates: Record<string, string | number | undefined | null>) => {
      const params = buildFilterSearchParams(searchParams, updates)
      const query = params.toString()
      navigate(query ? `${pathname}?${query}` : pathname, { replace: true })
    },
    [searchParams, pathname, navigate],
  )

  const debounceRef = useRef<ReturnType<typeof setTimeout>>(null)
  const debouncedUpdateFilter = useCallback(
    (key: string, value: string) => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
      debounceRef.current = setTimeout(() => {
        updateFilter({ [key]: value || undefined })
      }, 300)
    },
    [updateFilter],
  )

  const hasFilters =
    model !== "" ||
    status !== "all" ||
    keyword !== "" ||
    channelId !== undefined ||
    startTime !== undefined
  const isFirstPage = page === 1

  return {
    filters,
    page,
    model,
    status,
    channelId,
    keyword,
    pageSize,
    startTime,
    endTime,
    hasFilters,
    isFirstPage,
    keywordInput,
    setKeywordInput,
    updateFilter,
    debouncedUpdateFilter,
    pathname,
    navigate,
  }
}
