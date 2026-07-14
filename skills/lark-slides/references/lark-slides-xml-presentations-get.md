# lark-slides xml_presentations get

## Purpose

Read the full XML content of a Feishu Slides (PPT) presentation.

## Underlying Native Command Form

```bash
lark-cli slides xml_presentations get --as user --params '<json_params>'
```

## Parameter Description

| Parameter | Type | Required | Description |
|------|------|------|------|
| `--params` | JSON string | Yes | Path and query parameters; the structure follows the schema |

### params JSON Structure

```json
{
  "xml_presentation_id": "slides_example_presentation_id",
  "revision_id": -1
}
```

| Field | Type | Required | Description |
|------|------|------|------|
| `xml_presentation_id` | string | Yes | Unique identifier of the presentation |
| `revision_id` | integer | No | Revision number, `-1` means the latest revision |

## Usage Examples

### Basic Example

```bash
lark-cli slides xml_presentations get --as user \
  --params '{"xml_presentation_id":"slides_example_presentation_id","revision_id":-1}'
```

### Read a Specific Revision

```bash
lark-cli slides xml_presentations get --as user \
  --params '{"xml_presentation_id":"slides_example_presentation_id","revision_id":10}'
```

### Read with XML id Attributes Removed

```bash
lark-cli slides xml_presentations get --as user \
  --params '{"xml_presentation_id":"slides_example_presentation_id","revision_id":-1,"remove_attr_id":true}'
```

## Return Value

On success, returns the full presentation information:

```json
{
  "ok": true,
  "identity": "user",
  "data": {
    "xml_presentation": {
      "presentation_id": "slides_example_presentation_id",
      "revision_id": 1,
      "content": "<presentation xmlns=\"http://www.larkoffice.com/sml/2.0\" height=\"540\" width=\"960\">...</presentation>"
    }
  }
}
```

### Return Field Description

| Field | Type | Description |
|------|------|------|
| `data.xml_presentation.presentation_id` | string | Unique identifier of the presentation |
| `data.xml_presentation.revision_id` | integer | Revision number |
| `data.xml_presentation.content` | string | Full content in XML format |

## Common Errors

| Error Code | Meaning | Solution |
|--------|------|----------|
| 404 | Presentation does not exist | Check whether `xml_presentation_id` is correct |
| 403 | Insufficient permissions | Check whether you have the `slides:presentation:read` scope, or whether you have access permission |
| 400 | Malformed parameters | Make sure `--params` is a valid JSON string |

## Notes

1. Before calling the underlying API directly, use `lark-cli schema slides.xml_presentations.get` to check the latest parameter structure
2. The returned XML is in the `data.xml_presentation.content` field
3. If you only need part of the information, you can filter the result with tools like `jq`

## Related Commands

- [slides +create](lark-slides-create.md) - Create a PPT / add slides
- [xml_presentation.slide delete](lark-slides-xml-presentation-slide-delete.md) - Delete a slide
