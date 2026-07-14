
# slides +media-upload (Upload a Local Image to Feishu Slides)

> **Prerequisite:** Read [`../lark-shared/SKILL.md`](../../lark-shared/SKILL.md) first to understand authentication, global parameters, and safety rules.

Uploads a local image to the drive media library of the specified presentation and returns a `file_token`. **Put the returned token into `<img src="...">` in the slide XML to display the image.**

## Command

```bash
# Pass the xml_presentation_id directly
lark-cli slides +media-upload --as user \
  --file ./pic.png \
  --presentation slidesXXXXXXXXXXXXXXXXXXXXXX

# A slides URL also works
lark-cli slides +media-upload --as user \
  --file ./chart.png \
  --presentation "https://xxx.feishu.cn/slides/slidesXXXXXXXXXXXXXXXXXXXXXX"

# A wiki URL works too (the CLI automatically resolves it to the real token via wiki.spaces.get_node, verifying obj_type=slides)
lark-cli slides +media-upload --as user \
  --file ./pic.png \
  --presentation "https://xxx.feishu.cn/wiki/wikcnXXXXXX"

# Preview (does not actually upload)
lark-cli slides +media-upload --file ./pic.png --presentation $PRES_ID --dry-run
```

## Return Value

```json
{
  "file_token": "boxcnXXXXXXXXXXXXXXXXXXXXXX",
  "file_name": "pic.png",
  "size": 12345,
  "presentation_id": "slidesXXXXXXXXXXXXXXXXXXXXXX"
}
```

- **`file_token`**: write it into `<img src="...">`
- **`file_name` / `size`**: metadata of the uploaded file
- **`presentation_id`**: the resolved real `xml_presentation_id` (changes after resolving a wiki URL)

## Parameters

| Parameter | Required | Description |
|------|------|------|
| `--file` | Yes | Local image path; **must be a relative path inside the CWD** (e.g. `./pic.png`). **Max 20 MB** (the slides upload API does not support chunked upload) |
| `--presentation` | Yes | `xml_presentation_id`, a `/slides/<token>` URL, or a `/wiki/<token>` URL |

> [!IMPORTANT]
> **Paths must be inside the CWD**: `--file /abs/path/x.png` or `--file ../up/x.png` is rejected by the CLI (with an `unsafe file path` error). If the assets live in another directory, `cd` there first, then run the command.

## Usage Workflows

### Adding a New Page with an Image to an Existing Presentation

```bash
# 1) Upload the image
TOKEN=$(lark-cli slides +media-upload --as user \
  --file ./pic.png \
  --presentation $PRES_ID | jq -r .data.file_token)

# 2) Create the new page with the image using the file_token
lark-cli slides xml_presentation.slide create --as user \
  --params "{\"xml_presentation_id\":\"$PRES_ID\"}" \
  --data "{\"slide\":{\"content\":\"<slide xmlns=\\\"http://www.larkoffice.com/sml/2.0\\\"><data><img src=\\\"$TOKEN\\\" topLeftX=\\\"100\\\" topLeftY=\\\"100\\\" width=\\\"320\\\" height=\\\"180\\\"/></data></slide>\"}}"
```

### Creating a New Presentation with Images (recommended: use the `@` placeholder of `+create --slides` for a one-step flow)

```bash
# No separate +media-upload needed; just write src="@<local path>"
lark-cli slides +create --as user --title "Image Test" --slides '[
  "<slide xmlns=\"http://www.larkoffice.com/sml/2.0\"><data><img src=\"@./pic.png\" topLeftX=\"100\" topLeftY=\"100\" width=\"320\" height=\"180\"/></data></slide>"
]'
```

See the [+create documentation](lark-slides-create.md#local-images-path-placeholders) for details.

### Adding an Image to an Existing Page of an Existing Presentation

After getting the `file_token`, use `block_insert` via [`+replace-slide`](lark-slides-replace-slide.md); no need to move the original XML, change the `slide_id`, or disturb the page order:

```bash
PRES_ID=xxx
SID=yyy       # the page to add the image to

# 1) Upload the image to get the file_token
TOKEN=$(lark-cli slides +media-upload --as user \
  --file ./pic.png --presentation $PRES_ID | jq -r '.data.file_token')

# 2) block_insert at the end of the page (or use insert_before_block_id to specify the insertion position)
lark-cli slides +replace-slide --as user \
  --presentation "$PRES_ID" --slide-id "$SID" \
  --parts "$(jq -n --arg token "$TOKEN" \
    '[{action:"block_insert",insertion:("<img src=\""+$token+"\" topLeftX=\"500\" topLeftY=\"100\" width=\"200\" height=\"150\"/>")}]')"
```

Notes:

1. **Keep `<img>` coordinates clear of existing elements** — read the bounding boxes of existing elements first and pick an empty area; if there is not enough space, use `block_replace` to move/shrink existing elements first, then place the image
2. **Match the `<img>` `width:height` to the original image's aspect ratio** — a mismatched ratio gets cropped; see the `<img>` notes in [xml-schema-quick-ref.md](xml-schema-quick-ref.md)

## How It Works

`+media-upload` internally calls `POST /open-apis/drive/v1/medias/upload_all` (single-shot upload, max 20 MB), always using:

- `parent_type=slide_file` (the only value the slides backend accepts, verified in practice)
- `parent_node=<xml_presentation_id>`

**Do not try `slides_image`, `slide_image`, or other parent_type values** — the backend returns 1061001 / 1061002 errors. This is a slides-specific convention.

## Common Errors

| Error Code | Meaning | Solution |
|--------|------|----------|
| 1061002 | params error / unsupported parent_type | Do not assemble the parent_type yourself with the raw API; just use `+media-upload` |
| 1061004 | forbidden: the current identity has no edit permission on the presentation | Confirm that the current identity (user or bot) has edit permission on the target presentation. Common cause in bot mode: the presentation was not created by that bot — create a new one with `+create --as bot`, or grant the bot access as the user via `lark-cli drive permission.members create --as user ...` |
| 1061044 | parent node not exist | The token given to `--presentation` is wrong, or is not a slides resource |
| 403 | Insufficient permissions | Check the `docs:document.media:upload` scope; wiki URLs additionally require `wiki:node:read` |

## Related Commands

- [+create](lark-slides-create.md) — create a new presentation (supports `@` placeholders for automatic image upload)
- [+replace-slide](lark-slides-replace-slide.md) — add / swap an image on an existing page (`block_insert` / `block_replace`)
