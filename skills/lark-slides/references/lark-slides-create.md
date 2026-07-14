
# slides +create (Create a Feishu Slides Presentation)

> **Prerequisite:** Read [`../lark-shared/SKILL.md`](../../lark-shared/SKILL.md) first to understand authentication, global parameters, and safety rules.

Creates a new Feishu Slides presentation, with the option to add page content in one step.

- Forbidden: generating the submission payload by parsing/splitting/re-serializing a full <presentation> XML.
- Recommended: the submission source should be single-page <slide> XML directly; +create --slides only accepts slide arrays that were produced directly by hand or by a program, not arrays
      dynamically split out of a presentation.

- Most reliable: for complex decks, default to an empty deck + per-page slide create, submitting only one <slide> at a time.

- Note: complex XML is not suitable for inlining directly on the command line. When there is a lot of non-ASCII text, quotes, or special characters, concatenating `--slides` directly can easily cause shell escaping errors or truncation. Save each page's XML as a separate file and assemble the JSON array with `jq --rawfile` to avoid handling XML quotes and newlines manually.

## Command

```bash
# Create a blank presentation
lark-cli slides +create --title "Project Report"

# Create a presentation + add slide pages
lark-cli slides +create --title "Project Report" --slides '[
  "<slide xmlns=\"http://www.larkoffice.com/sml/2.0\"><data><shape type=\"text\" topLeftX=\"80\" topLeftY=\"80\" width=\"800\" height=\"120\"><content textType=\"title\"><p>Cover</p></content></shape></data></slide>",
  "<slide xmlns=\"http://www.larkoffice.com/sml/2.0\"><data><shape type=\"text\" topLeftX=\"80\" topLeftY=\"80\" width=\"800\" height=\"120\"><content textType=\"title\"><p>Page 2</p></content></shape></data></slide>"
]'

# Create as the app identity (automatically grants access to the current user)
lark-cli slides +create --title "Project Report" --as bot

# Preview (does not execute)
lark-cli slides +create --title "Project Report" --slides '[...]' --dry-run
```

For complex content, save the XML per page and assemble the `--slides` argument with `jq --rawfile`:

```bash
lark-cli slides +create --as user --title "Project Report" \
  --slides "$(jq -n \
    --rawfile s1 .lark-slides/plan/project/slide-01.xml \
    --rawfile s2 .lark-slides/plan/project/slide-02.xml \
    '[$s1, $s2]')"
```

`--rawfile` reads the file content into JSON as a string, automatically handling quotes and newlines inside the XML; do not manually concatenate JSON strings full of escape characters.

## Return Value

On success, the tool returns a JSON object with the following fields:

- **`xml_presentation_id`** (string): the unique identifier of the presentation; this ID is required when adding pages later
- **`title`** (string): the presentation title
- **`url`** (string, optional): the online link to the presentation. If returned, always show it to the user (requires drive-related permissions; if retrieval fails, this field is not returned)
- **`revision_id`** (integer): the presentation revision number
- **`slide_ids`** (string[], optional): returned only when `--slides` is passed; the list of successfully added page IDs
- **`slides_added`** (integer, optional): returned only when `--slides` is passed; the number of successfully added pages
- **`images_uploaded`** (integer, optional): returned only when `--slides` contains `@<local path>` placeholders; the number of deduplicated images uploaded
- **`permission_grant`** (object, optional): returned only with `--as bot`; indicates whether manage permission was automatically granted to the current CLI user

> [!IMPORTANT]
> Without `--slides`, `slides +create` only creates a blank presentation. After creation, use `xml_presentation.slide create` to add slide content page by page.
>
> With `--slides`, the CLI first creates a blank presentation, then calls `xml_presentation.slide create` page by page to add pages. If adding any page fails, the CLI stops and reports an error; the created presentation and already-added pages are kept.
>
> If the presentation is **created as the app identity (bot)**, e.g. `lark-cli slides +create --as bot`, the CLI will **attempt to automatically grant the current CLI user `full_access` (manage permission) on the presentation**.
>
> When creating as the app identity, the result additionally includes a `permission_grant` field that explicitly states the grant outcome:
> - `status = granted`: the current CLI user has been granted manage permission on the presentation
> - `status = skipped`: no usable current-user `open_id` is available locally, so no automatic grant is performed
> - `status = failed`: the presentation was created successfully, but automatically granting the user failed
>
> **Never transfer ownership on your own initiative.** If the user wants ownership transferred to themselves, that must be confirmed separately.

## Parameters

| Parameter | Required | Description |
|------|------|------|
| `--title` | No | Presentation title (defaults to "Untitled" if omitted) |
| `--slides` | No | JSON array of slide content, each element a `<slide>` XML string (max 10; for more than 10 pages, first create a blank presentation with `+create`, then add pages one by one with `xml_presentation.slide create`) |

## `--slides` Parameter Format

```json
[
  "<slide xmlns=\"http://www.larkoffice.com/sml/2.0\">...page 1 XML...</slide>",
  "<slide xmlns=\"http://www.larkoffice.com/sml/2.0\">...page 2 XML...</slide>"
]
```

A JSON string array where each element is the complete XML of one slide page. The CLI internally wraps each into the `{"slide": {"content": "..."}}` format required by the API and calls it page by page.

### Local Images: `@<path>` Placeholders

If the `src` attribute of an `<img>` element starts with `@`, the CLI treats it as a local file path, automatically uploads it to the current presentation, and replaces the placeholder with the returned `file_token`.

```bash
lark-cli slides +create --as user --title "Image Test" --slides '[
  "<slide xmlns=\"http://www.larkoffice.com/sml/2.0\"><data><img src=\"@./assets/chart.png\" topLeftX=\"100\" topLeftY=\"100\" width=\"320\" height=\"180\"/></data></slide>"
]'
```

Behavior:

- Paths are resolved relative to the **current working directory** (CWD); they **must be relative paths inside the CWD** (e.g. `./pic.png`, `./assets/x.png`)
- The same image referenced multiple times is **uploaded only once** (deduplicated by path)
- `src` values not starting with `@` are kept as-is, but **only `file_token` values obtained from `slides +media-upload` are allowed**; **http(s) external URLs are forbidden**: the Feishu slides renderer does not proxy external images, so external `src` values usually render as broken images. To use a web image, download it into the CWD first and go through the upload workflow
- Maximum 20 MB per image (the slides upload API does not support chunked upload)
- All placeholder files are checked for existence and size during the validation phase; a missing or oversized file fails immediately, without creating a blank placeholder presentation
- Execution order: create the blank presentation → upload all images → replace tokens → create slides page by page

> [!IMPORTANT]
> **Paths must be inside the CWD**: forms like `@/abs/path/x.png` or `@../up/x.png` are rejected by the CLI (with an `unsafe file path` error). If the assets live in another directory, `cd` there first, then run the command.

### Adding a New Page with an Image to an Existing Presentation

`+create --slides` only supports `@` placeholders when creating a new presentation. Adding a new page with an image to an existing presentation takes two steps (the CLI does not wrap this combination):

```bash
# 1) Upload the image
TOKEN=$(lark-cli slides +media-upload --as user \
  --file ./pic.png --presentation $PRES_ID | jq -r .data.file_token)

# 2) Create the new page with the image using the returned file_token
lark-cli slides xml_presentation.slide create --as user \
  --params "{\"xml_presentation_id\":\"$PRES_ID\"}" \
  --data "{\"slide\":{\"content\":\"<slide xmlns=\\\"http://www.larkoffice.com/sml/2.0\\\"><data><img src=\\\"$TOKEN\\\" topLeftX=\\\"100\\\" topLeftY=\\\"100\\\" width=\\\"200\\\" height=\\\"200\\\"/></data></slide>\"}}"
```

## Next Steps After Creation

If `--slides` was not used, the `xml_presentation_id` returned by `slides +create` is used for subsequent operations:

```bash
# Step 1: create a blank presentation
PRES_ID=$(lark-cli slides +create --title "Project Report" | jq -r '.data.xml_presentation_id')

# Step 2: add a page (using the returned xml_presentation_id)
lark-cli slides xml_presentation.slide create --as user \
  --params "{\"xml_presentation_id\":\"$PRES_ID\"}" \
  --data '{
    "slide": {
      "content": "<slide xmlns=\"http://www.larkoffice.com/sml/2.0\">...</slide>"
    }
  }'
```

## Common Errors

| Error Code | Meaning | Solution |
|--------|------|----------|
| 400 | Invalid parameters | Check that the parameter format is correct |
| 403 | Insufficient permissions | Check that you have the `slides:presentation:create` and `slides:presentation:write_only` scopes |

## Related Commands

- [slides +xml-get](lark-slides-xml-get.md) — read presentation content and save it to a local file
