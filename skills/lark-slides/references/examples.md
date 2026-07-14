# Complete Operation Examples

This document provides invocation examples consistent with the CLI schema; all XML content follows [slides_xml_schema_definition.xml](slides_xml_schema_definition.xml).

> **Important**: to create a new PPT, use `slides +create --slides` with a JSON array of `<slide>` XML strings; each element must be one complete `<slide>` page. For complex content, create an empty PPT first, then add pages one by one via `xml_presentation.slide.create`. A complete `<presentation>` XML can be used for local lint or reading, but cannot be submitted directly as the `+create` argument.

## Table of Contents

- [Example 1: Reliably create a 6-page PPT](#example-1-reliably-create-a-6-page-ppt)
- [Example 7: +replace-slide + block_insert to add an image to an existing page](#example-7-replace-slide--block_insert-to-add-an-image-to-an-existing-page)
- [Example 8: +replace-slide + block_replace to replace one block](#example-8-replace-slide--block_replace-to-replace-one-block)

## Example 1: Reliably create a 6-page PPT

### 1. Write the plan file

```bash
DECK_DIR=".lark-slides/plan/reliable-six-page-ppt"
mkdir -p "$DECK_DIR"

# Write "$DECK_DIR/slide_plan.json" per planning-layer.md,
# recording at least the order and titles of the 6 pages.
```

### 2. Save a standalone XML file per page

Each file is a complete `<slide>`. The loop below generates 6 standalone XML files; in a real project, replace each page body with the planned content.

```bash
titles=("Topic and Conclusion" "Problem Background" "Core Method" "Key Data" "Execution Plan" "Summary and Actions")
for i in {1..6}; do
  printf -v page '%02d' "$i"
  cat > "$DECK_DIR/slide-$page.xml" <<XML
<slide xmlns="http://www.larkoffice.com/sml/2.0"><style><fill><fillColor color="rgb(248,250,252)"/></fill></style><data><shape type="rect" topLeftX="56" topLeftY="56" width="12" height="428"><fill><fillColor color="rgb(37,99,235)"/></fill></shape><shape type="text" topLeftX="100" topLeftY="160" width="760" height="90"><content textType="title" autoFit="normal-auto-fit"><p>${titles[$((i-1))]}</p></content></shape><shape type="text" topLeftX="100" topLeftY="290" width="700" height="70"><content textType="body" autoFit="normal-auto-fit"><p>Page body content.</p></content></shape></data></slide>
XML
done
```

### 3. Run lint on each page

Check every standalone XML before submission. `summary.error_count` must be `0`; otherwise fix the XML or layout issues first.

```bash
for slide_xml in "$DECK_DIR"/slide-0{1,2,3,4,5,6}.xml; do
  python3 skills/lark-slides/scripts/xml_text_overlap_lint.py \
    --input "$slide_xml" | tee "${slide_xml%.xml}.lint.json"
done

test "$(jq -s 'map(.summary.error_count) | add' "$DECK_DIR"/slide-0{1,2,3,4,5,6}.lint.json)" = "0"
```

### 4. Create the 6-page PPT with `+create`

`--slides` receives a JSON array of 6 complete `<slide>` XML strings; use `jq --rawfile` to avoid manually handling XML quotes and newlines.

```bash
lark-cli slides +create --as user \
  --title "Reliably created 6-page PPT" \
  --slides "$(jq -n \
    --rawfile s1 "$DECK_DIR/slide-01.xml" \
    --rawfile s2 "$DECK_DIR/slide-02.xml" \
    --rawfile s3 "$DECK_DIR/slide-03.xml" \
    --rawfile s4 "$DECK_DIR/slide-04.xml" \
    --rawfile s5 "$DECK_DIR/slide-05.xml" \
    --rawfile s6 "$DECK_DIR/slide-06.xml" \
    '[$s1, $s2, $s3, $s4, $s5, $s6]')" \
  > "$DECK_DIR/create.json"
create_status=$?

if [ "$create_status" -ne 0 ]; then
  exit "$create_status"
fi

if ! PRESENTATION_ID=$(jq -er '.data.xml_presentation_id | strings | select(length > 0)' "$DECK_DIR/create.json"); then
  echo "missing non-empty data.xml_presentation_id in $DECK_DIR/create.json" >&2
  exit 1
fi
echo "$PRESENTATION_ID" > "$DECK_DIR/xml_presentation_id"
```

If creation fails midway, save the already-returned `xml_presentation_id` first, then read back to confirm how many pages were actually created.

### 5. Read back the full XML with `+xml-get`

```bash
lark-cli slides +xml-get --as user \
  --presentation "$PRESENTATION_ID" \
  --output "$DECK_DIR/readback.xml" \
  --json | tee "$DECK_DIR/readback.json"
```
