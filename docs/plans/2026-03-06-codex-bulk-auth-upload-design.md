# Codex Bulk Auth Upload Design

**Goal:** Let users select and upload many Codex auth JSON files in one action, with partial-success handling and clear per-file feedback.

## Current State

- `apps/web/src/pages/model/codex-channel-detail.tsx` only selects one file from a hidden file input.
- `apps/web/src/pages/model/channel-dialog.tsx` only selects one file from a hidden file input.
- `apps/web/src/lib/api/codex.ts` only sends one multipart `file` field.
- `apps/worker/internal/handler/codex.go` only reads one uploaded file via `c.FormFile("file")`.

## Desired UX

- Users can select multiple `.json` auth files in one picker interaction.
- Upload runs as one logical batch, not a stream of unrelated single-file toasts.
- Invalid files or duplicate filenames fail independently without blocking valid files.
- The UI shows a compact batch result summary: total, success count, failed count, and a short failure list.
- On any successful uploads, the page still refreshes auth files, quota, and channels so the Codex card updates immediately.

## API Design

- Keep the endpoint path as `POST /api/v1/channel/:id/codex/auth-files`.
- Extend multipart parsing to accept:
  - `files` repeated multipart entries for batch upload
  - `file` as a compatibility fallback for existing single-file callers
- Return a structured batch response for both single and multiple uploads:
  - `total`
  - `successCount`
  - `failedCount`
  - `results[]` with `name`, `status`, and optional `error`

Example response:

```json
{
  "success": true,
  "data": {
    "total": 3,
    "successCount": 2,
    "failedCount": 1,
    "results": [
      { "name": "a.json", "status": "ok" },
      { "name": "b.json", "status": "error", "error": "invalid auth file json" },
      { "name": "c.json", "status": "ok" }
    ]
  }
}
```

## Backend Behavior

- Parse all incoming multipart files first.
- For each file:
  - require `.json` filename
  - read content
  - validate with existing `parseCodexAuthContent`
  - insert DB record
  - materialize runtime file
- Record per-file success or failure instead of aborting the whole batch on first error.
- Run `bestEffortSyncCodexChannelModels` once after the batch if at least one file succeeded.
- Preserve existing duplicate-name behavior: same-name uploads fail per file instead of overwriting.

## Frontend Behavior

- Add `multiple` to both Codex upload inputs.
- Change upload helpers to accept `FileList` or `File[]`.
- Send all selected files in one `FormData` payload using repeated `files` entries.
- Show one completion toast:
  - all success: success toast with uploaded count
  - partial success: warning/success style summary with failed count
  - all failed: error toast
- Keep detailed failure text short in toast; full per-file detail can stay in the response object for future UI expansion.

## Error Handling Rules

- Non-JSON files: rejected per file with a clear error.
- Invalid JSON/auth schema: rejected per file.
- Duplicate filename: rejected per file using existing DB error path.
- Mixed batches: overall HTTP response stays 200 if request parsing succeeds, even when some files fail.
- Malformed multipart with no files at all: still returns 400.

## Testing Strategy

- Backend handler test for mixed-result batch upload.
- Backend handler test for compatibility single-file upload still working.
- Frontend unit test for upload query-key invalidation can remain unchanged if mutation success path still invalidates once.
- Add focused frontend tests for batch summary formatting only if there is extracted helper logic.

## Non-Goals

- No zip archive upload.
- No overwrite/replace mode.
- No drag-and-drop zone in this pass.
- No per-file progress bars in this pass.
