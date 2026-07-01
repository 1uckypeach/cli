# VC CLI E2E Coverage

## Summary
- TestVCMeetingMessageSendDryRun: dry-run coverage for `vc +meeting-message-send`; asserts CLI flag parsing, validation, and dry-run request shape for both text and reaction messages.
- Live coverage for `vc +meeting-message-send` is intentionally not included here because it requires an active meeting, a joined user or bot identity, and meeting-message permission setup.

## Command Table

| Status | Cmd | Type | Testcase | Key parameter shapes | Notes / uncovered reason |
| --- | --- | --- | --- | --- | --- |
| dry-run ✓ / live ✕ | vc +meeting-message-send | shortcut | vc/vc_meeting_message_send_dryrun_test.go::TestVCMeetingMessageSendDryRun | `--meeting-id`; `--text`; `--msg-type reaction`; `--emoji-type`; `--uuid` | live E2E requires active VC meeting and message-enabled in-meeting identity |
