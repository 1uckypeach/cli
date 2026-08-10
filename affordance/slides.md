# slides

## +update-slide

### Tips

- Read the page first with `slides +xml-get --slide-id <id>`, edit that XML, and hand it back whole.
- Anything left out of `--content` is removed from the page; pass the full page, not a fragment.
- Editing one element is cheaper with `slides +replace-slide`.
- In shell loops, assign the computed path to a variable, reject empty or missing files before invoking the command, and prefer `--content - < "$file"` over constructing an `@file` argument inline.

### Skills

- lark-slides/references/lark-slides-update-slide.md
