# Troubleshooting

This file covers XML troubleshooting and common failure handling for lark-slides.

## Failure Order

When you hit `invalid param`, a page fails to create, a page is blank, or the layout is broken, handle it in order:

1. First determine whether a usable `xml_presentation_id` already exists: get it from a successful stdout, an error hint, a user-provided link, or saved context; without an ID, do not read back — handle the current error directly.
2. If you have an `xml_presentation_id`, try reading back with `slides +xml-get` to confirm whether the presentation exists, whether some pages were partially written, or whether it is just an empty presentation.
3. Check whether the failing page contains unescaped characters: `Q&A -> Q&amp;A`, textual `<` / `>` written as `&lt;` / `&gt;`, attribute URLs `a=1&b=2 -> a=1&amp;b=2`.
4. Check tag closure, attribute quoting, `<content>` structure, and the direct children of `<slide>`.
5. If using `--slides '[...]'` and you suspect shell truncation, switch directly to two-step creation: `slides +create` first, then add pages one by one with `xml_presentation.slide create`.

## Symptom Fixes

| Observed problem | Fix |
|-----------|----------|
| Text truncated / not fully visible | Increase the shape's `width` or `height`, or reduce the amount of text |
| Elements overlap | Adjust `topLeftX` / `topLeftY` to open up spacing |
| Large blank areas on the page | Read back to confirm content was written; if content exists, tighten spacing or add primary elements |
| Text color too close to background | Use light text on dark backgrounds, dark text on light backgrounds |
| Unreasonable table column widths | Adjust the `width` values of `col` in `colgroup` |
| Chart not displayed | Check that both `chartPlotArea` and `chartData` are present, and that `dim1` / `dim2` data counts match |
| Image partially cropped | The `<img>` `width` / `height` are post-crop dimensions; to show the full image, match `width:height` to the original aspect ratio |
| Image not displayed / `<img src>` is still `@path` | The `@` placeholder is only substituted in `+create --slides`; direct `xml_presentation.slide create` calls must first obtain a `file_token` via `+media-upload` |
| Newly inserted `<img>` covers existing elements | Read the original page with `slide.get`, pick an empty spot against existing block coordinates; if space is tight, move/shrink existing blocks in the same `--parts` batch before inserting the image |
| Gradient background turns white | Gradients must use the `rgba()` format + percentage stops, e.g. `linear-gradient(135deg,rgba(30,60,114,1) 0%,rgba(59,130,246,1) 100%)` |

## Common Errors

| Error code / signal | Meaning | Solution |
|--------------|------|----------|
| 400 XML format error | XML syntax error | Check tag closure, attribute quoting, special-character escaping |
| 400 request wrapping error | `--data` not wrapped per schema | Check whether `xml_presentation.content` or `slide.content` was passed |
| Creation succeeded but pages blank / content missing / layout broken | Typically shell escaping or long-argument issues with `--slides '[...]'` | Switch to two-step creation, and read the XML immediately after creation to verify |
| 403 insufficient permission | Identity or scope mismatch | First check whether the bot identity was used by mistake, then confirm scopes and document permissions; when unauthorized, guide the user per the error response |
| 404 presentation not found | `xml_presentation_id` incorrect or no permission | Check the token; wiki URLs must first be resolved to the real `obj_token` |
| 404 slide not found | `slide_id` incorrect | Re-read the presentation or slide to confirm the latest ID |
| 400 cannot delete the only slide | The presentation must keep at least one page | Create the new page first, then delete the old one |
| 1061002 media upload params error | Slides media upload parameters do not meet the contract | Use `slides +media-upload`; do not hand-build the native `medias/upload_all`; the only valid `parent_type` for slides is `slide_file` |
| 1061004 forbidden | The current identity has no edit permission on the presentation | Confirm the user/bot has edit permission on the target PPT; for bots this commonly means the PPT was not created by that bot |
| 3350001 | XML not well-formed, XML structure not accepted by the server, or a bad replace fragment | Check unescaped characters first; in replace scenarios also check `block_id` and `<content/>` |
| 3350002 | `revision_id` greater than the current version | Use `-1` for the current version, or fetch the latest `revision_id` again with `slides +xml-get` |
| validation: unsafe file path | `--file` was given an absolute or parent path | `--file` must be a relative path inside the CWD; `cd` into the asset directory first, then run the command |
