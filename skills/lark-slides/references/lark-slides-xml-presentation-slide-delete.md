# lark-slides xml_presentation.slide delete

## Purpose

Delete a slide from the specified XML presentation.

## Command

```bash
lark-cli slides xml_presentation.slide delete --as user --params '<json_params>'
```

## Parameter Description

| Parameter | Type | Required | Description |
|------|------|------|------|
| `--params` | JSON string | Yes | Path and query parameters |

### params JSON Structure

```json
{
  "xml_presentation_id": "slides_example_presentation_id",
  "slide_id": "slide_example_id",
  "revision_id": -1,
  "tid": "idMock"
}
```

| Field | Type | Required | Description |
|------|------|------|------|
| `xml_presentation_id` | string | Yes | Unique identifier of the presentation |
| `slide_id` | string | Yes | Unique identifier of the slide to delete |
| `revision_id` | integer | No | Presentation revision number, `-1` means the latest revision |
| `tid` | string | No | Transaction ID of the lock |

## Usage Examples

### Delete a Specific Slide

```bash
lark-cli slides xml_presentation.slide delete --as user --params '{
  "xml_presentation_id": "slides_example_presentation_id",
  "slide_id": "slide_example_id"
}'
```

### Delete After Inspection (with jq)

```bash
# First read the XML content to confirm the slide to delete
lark-cli slides +xml-get --as user \
  --presentation "slides_example_presentation_id" \
  --output .lark-slides/plan/slides_example_presentation_id/readback.xml \
  --json

# Then delete by the known slide_id
lark-cli slides xml_presentation.slide delete --as user --params '{"xml_presentation_id":"slides_example_presentation_id","slide_id":"slide_example_id"}'
```

## Return Value

On success, returns a deletion confirmation:

```json
{
  "ok": true,
  "identity": "user",
  "data": {
    "revision_id": 100
  }
}
```

### Return Field Description

| Field | Type | Description |
|------|------|------|
| `data.revision_id` | integer | The latest revision number after deletion |

## Common Errors

| Error Code | Meaning | Solution |
|--------|------|----------|
| 404 | Presentation does not exist | Check whether `xml_presentation_id` is correct |
| 404 | Slide does not exist | Check whether `slide_id` is correct, or the slide may have already been deleted |
| 400 | Cannot delete the only slide | The presentation must keep at least one slide |
| 403 | Insufficient permissions | Check whether you have the `slides:presentation:update` or `slides:presentation:write_only` scope |

## Notes

1. **Do this before executing**: use `lark-cli schema slides.xml_presentation.slide.delete` to check the latest parameter structure
2. **Deletion is irreversible**: the delete operation cannot be undone; make sure important content is backed up
3. **Keep at least one slide**: the presentation must keep at least one slide; deleting the last slide returns an error
4. **Version control**: if you rely on revision numbers for concurrency control, confirm the `revision_id` before deleting
5. **Getting slide_id**: save the return value when creating slides; the server-side short ID cannot be derived directly from the XML returned by `get` alone

## How to Get slide_id

### Method 1: Save at Creation Time

```bash
lark-cli slides xml_presentation.slide create --as user --params '{"xml_presentation_id":"slides_example_presentation_id"}' --data '{
  "slide": {
    "content": "<slide xmlns=\"http://www.larkoffice.com/sml/2.0\"><data><shape type=\"text\" topLeftX=\"80\" topLeftY=\"80\" width=\"800\" height=\"120\"><content textType=\"title\"><p>New Slide</p></content></shape></data></slide>"
  }
}'
```

The `slide_id` in the response is the value needed for later deletion.

## Batch Deletion Suggestions

If you need to delete multiple slides, prepare the list of `slide_id`s to delete first, then delete them one by one:

```bash
for slide_id in sld_a sld_b sld_c; do
  lark-cli slides xml_presentation.slide delete --as user --params "{\"xml_presentation_id\":\"slides_example_presentation_id\",\"slide_id\":\"$slide_id\"}"
done
```

## Related Commands

- [slides +create](lark-slides-create.md) - Create a PPT / add slides
- [slides +xml-get](lark-slides-xml-get.md) - Read PPT content and save it to a local file
